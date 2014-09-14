// eISCP communications intermediary
// The communications intermediary does the work of finding devices, then providing a simpler interface with
// which other programs and systems may interact.

// @author: Thaddeus Bond (http://thaddeusbond.com, @thaddeusbond)
// @date: September 9th, 2014
// @version: 1.2.3

// The protocol is based off of version 1.24 of the "Integra Serial Communication Protocol for AV Receiver"
// from June 8th, 2012. Testing was performed with an Onkyo TX-NR616, other models have not been tested.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/gorilla/mux"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Map used to store the Device objects by the broadcaster.
var devices = make(map[string]*deviceInfo)

// Map used to store keepAlive Devices (the ones we want to talk to)
var keepAlive = make(map[string]*keepAliveDevice)

// Command line flags
var debug bool
var defaultDevice string
var devicePort = 60128

// Locate devices in our network and record them
func findDevices() {
	// Create socket, connecting using UDP4, any available local address, to our broadcast IP with the ISCP default port
	socket, err := net.ListenUDP("udp4", &net.UDPAddr{
		IP:   net.IPv4zero, // net.IPv4zero is 0.0.0.0 (bind on all addresses)
		Port: devicePort,
	})
	if err != nil {
		fmt.Println("Error creating UDP socket")
	}

	// Keep looping infinitely
	for {
		// Try all known ISCP endings to make sure we don't miss anybody.
		for _, broadcastEndBytes := range endBytes {
			if debug {
				fmt.Printf("Attempting broadcast using end 0x%X\n", broadcastEndBytes)
			}

			// If the socket creates successfully - write the question out using the current ending
			if err == nil {
				socket.WriteToUDP(packageISCP("!xECNQSTN", broadcastEndBytes), &net.UDPAddr{
					IP:   net.IPv4bcast, // net.IPv4bcast is 255.255.255.255 (send to all available addresses)
					Port: devicePort,
				})
			}

			// Run listen in parallel
			go func() {
				timeout := time.Now().Add(time.Millisecond * 50)

				for time.Now().Before(timeout) {
					data := make([]byte, 1024)

					// We should get a response within 50ms
					socket.SetReadDeadline(time.Now().Add(time.Millisecond * 51))
					read, remoteAddr, err := socket.ReadFromUDP(data)
					if err != nil {
						// We timed out or died, whatevs.
					}

					packet, valid := processISCP(data[:read])
					// This isn't a broadcast question from ourselves or another controller
					if valid && packet[0:5] == "!1ECN" && packet[5:9] != "QSTN" {
						// Get the meta data about the device from its response
						data := packet[5:]
						if debug {
							fmt.Println("DEBUG: Received device response:", data, "from", remoteAddr.String())
						}
						go foundDevice(data, remoteAddr)
					}
				}
			}()

			// We don't want to do this too much, so we'll sleep for five seconds before trying again.
			time.Sleep(time.Second * 5)
		}
	}
}

// Every time a device responds, verify it
func foundDevice(devicePacket string, remoteAddr *net.UDPAddr) {
	fragments := strings.Split(devicePacket, "/")
	port, _ := strconv.ParseInt(fragments[1], 10, 0)
	// Create and store the device response according to its MAC address
	device := &deviceInfo{
		Model:   fragments[0],                                            // Model
		Port:    int(port),                                               // Port
		IP:      net.ParseIP(strings.Split(remoteAddr.String(), ":")[0]), // IP Address
		Region:  fragments[2],                                            // Region
		Macaddr: fragments[3],                                            // MAC Address
	}
	if _, present := devices[device.Macaddr]; !present {
		devices[device.Macaddr] = device
		if debug {
			fmt.Println("DEBUG: Added device", fragments[0], "with MAC addr", fragments[3], "to collection")
		}
	}
	if _, keepMeAlive := keepAlive[device.Macaddr]; keepMeAlive {
		// The user loves me. I'm important.
		if keepAlive[device.Macaddr].device != devices[device.Macaddr] {
			// We need to update our device info
			keepAliveTmp := keepAlive[device.Macaddr]
			keepAliveTmp.device = devices[device.Macaddr]
			keepAlive[device.Macaddr] = keepAliveTmp
			if debug {
				fmt.Println("DEBUG: Updating device", device.Macaddr, "info")
			}
		}

		if keepAlive[device.Macaddr].messenger == nil {
			// We need to set up a messenger
			if debug {
				fmt.Println("DEBUG: Generating messenger for", device.Macaddr)
			}
			go connectDevice(device.Macaddr)
		}
	}
}

func connectDevice(theTarget string) {
	socket, err := net.DialTCP("tcp4", nil, &net.TCPAddr{
		IP:   devices[theTarget].IP,
		Port: devices[theTarget].Port,
	})
	if err != nil {
		fmt.Println("Error connecting to device", theTarget)
	}

	messenger := &deviceMessenger{
		device:     devices[theTarget],      // General info found during broadcast
		connection: socket,                  // Connection to device, we die if this dies
		endBytes:   nil,                     // The end bytes it'll respond to
		properties: make(map[string]string), // Interface for setting/getting/caching device properties
		killFlag:   false,                   // Flag we can trip if we need to exit
	}

	keepAliveTmp := keepAlive[theTarget]
	keepAliveTmp.messenger = messenger
	keepAlive[theTarget] = keepAliveTmp
	if keepAlive[theTarget].messenger == nil {
		fmt.Println("Nil messenger")
	}
	if debug {
		fmt.Println("DEBUG: Updating device", theTarget, "messenger")
	}
	go messenger.mediate()
	messenger.message("!1MVLQSTN")
}

func main() {
	fmt.Println("Starting eISCP (ethernet Integra Serial Communication Protocol) Intermediary")
	// Command line options
	flag.BoolVar(&debug, "debug", false, "enable verbose debugging")
	flag.StringVar(&defaultDevice, "device", "000000000000", "mac address of auto-connect device")
	flag.IntVar(&devicePort, "port", 60128, "port on devices to commmunicate with")
	// Now that we've defined our flags, parse them
	flag.Parse()

	if debug {
		fmt.Println("Displaying debug output.")
	}

	fmt.Println("Searching for devices on port", devicePort)

	if defaultDevice != "000000000000" {
		fmt.Println("Searching for a device with MAC address of", defaultDevice)
		keepAlive[defaultDevice] = &keepAliveDevice{messenger: nil, device: nil}
	}

	// Start finding devices
	go findDevices()

	r := mux.NewRouter()
	r.HandleFunc("/kill", HandleKill) //Debug Function
	r.HandleFunc("/devices", GetDevices).Methods("GET")
	r.HandleFunc("/devices/{id}", PutDevices).Methods("PUT")
	r.HandleFunc("/devices/{id}", DeleteDevices).Methods("DELETE")
	http.Handle("/", r)

	http.ListenAndServe(":3000", nil)
}

func GetDevices(w http.ResponseWriter, r *http.Request) {
	if len(devices) > 0 {
		// Create a slice of devices to properly format our output
		deviceSlice := make([]deviceInfo, 0)
		for _, device := range devices {
			deviceSlice = append(deviceSlice, *device)
			fmt.Println(device)
		}
		resObject := GetDevicesResponse{len(devices), deviceSlice}
		deviceResponse, _ := json.Marshal(resObject)
		w.Write([]byte(deviceResponse))
	} else {
		w.WriteHeader(http.StatusNoContent)
	}
}

func PutDevices(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	macAddr := vars["id"]
	if _, present := keepAlive[macAddr]; !present {
		keepAlive[macAddr] = &keepAliveDevice{messenger: nil, device: nil}
		if debug {
			fmt.Println("DEBUG: Added device with MAC addrress of", macAddr, "to the Keep-Alive list")
		}
	} else if debug {
		fmt.Println("DEBUG: Device with MAC addrress of", macAddr, "already existed on the Keep-Alive list")
	}
	w.WriteHeader(200)
}

func DeleteDevices(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	macAddr := vars["id"]
	if _, present := keepAlive[macAddr]; present {
		keepAlive[macAddr].messenger.kill()
		delete(keepAlive, macAddr)
		if debug {
			fmt.Println("DEBUG: Remove device with MAC addrress of", macAddr, "from the Keep-Alive list")
		}
	} else if debug {
		fmt.Println("DEBUG: Device with MAC addrress of", macAddr, "did not exist on the Keep-Alive list")
	}
	w.WriteHeader(200)
}

func HandleKill(w http.ResponseWriter, r *http.Request) { // Debug function
	w.Write([]byte("Goodbye"))
	fmt.Println("REST Request Shutdown")
	go os.Exit(0)
}

// Device type, record of facts learned through broadcast
type deviceInfo struct {
	Model   string
	Port    int
	IP      net.IP
	Region  string
	Macaddr string
}

// Used to keep track of user requested devices
type keepAliveDevice struct {
	messenger *deviceMessenger // Messenger, if we're alive.
	device    *deviceInfo      //Device Info, if available
}

// Used for JSON response - probably nuke this later
type GetDevicesResponse struct {
	Total   int
	Devices []deviceInfo
}
