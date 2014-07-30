//eISCP communications intermediary
//The communications intermediary does the work of finding devices, then providing a simpler interface with
//which other programs and systems may interact.

//@author: Thaddeus Bond (http://thaddeusbond.com, @thaddeusbond)
//@date: July 9th, 2014
//@version: 1.0.3 (July 10th, 2014)

//The protocol is based off of version 1.24 of the "Integra Serial Communication Protocol for AV Receiver"
//from June 8th, 2012. Testing was performed with an Onkyo TX-NR616, other models have not been tested.

package main

import "fmt"
import "net"
import "encoding/hex"
import "encoding/binary"
import "time"
import "bytes"
import "reflect"
import "strings"
import "strconv"
import "github.com/gorilla/mux"
import "os"

//import "io"
import "net/http"

//import "flag"
import "encoding/json"

//The three different packet endings specified by the eISCP protocol. There are three more specified by the RS-232C connection protocol.
//For some reason someone decided that the eISCP protocol could end in any of the SIX. So we have to check/try all of them.
var EOF = []byte{0x1a}
var CR = []byte{0x0d}
var LF = []byte{0x0a}
var EOF_CR = []byte{0x1a, 0x0d}
var CR_LF = []byte{0x0d, 0x0a}
var EM_CR_LF = []byte{0x19, 0x0d, 0x0a}
var EOF_CR_LF = []byte{0x1a, 0x0d, 0x0a}

//Map used to store the Device objects by the broadcaster.
var devices = make(map[string]Device)

//Port to check for devices on - default is 60128
var devicePort = 60128

//Take a command and encapsulate it according to the ISCP guidelines
func packageISCP(command string, endBytes []byte) []byte {

	//Start forming header
	packageISCP := make([]byte, 0)
	//Beginning of every message
	bytesISCP := []byte("ISCP")
	//Append ISCP to the end (the beginning of our empty byte array) of header
	packageISCP = append(packageISCP, bytesISCP...)

	//eISCP header size (hex decode string is probably the wrong way to do this)
	bytesHeaderSize, _ := hex.DecodeString("00000010")
	//Append to the header after ISCP bytes
	packageISCP = append(packageISCP, bytesHeaderSize...)

	//Get command bytes (we need the size), we'll also use this later
	bytesCommand := []byte(command)
	//Createa a new byte array of length 4 for the command size
	commandSize := make([]byte, 4)
	//Take the length of the command and put it in the previous byte array. We add 3 for the data end characters.
	binary.BigEndian.PutUint32(commandSize, uint32(len(bytesCommand)+len(endBytes)))
	//Append the command length to the current end of the header
	packageISCP = append(packageISCP, commandSize...)

	//eISCP version (again, hex decode string is probably inefficient)
	byteVersion, _ := hex.DecodeString("01")
	//Append to the header after ISCP bytes
	packageISCP = append(packageISCP, byteVersion...)

	//eISCP reserved bytes (again, inefficient)
	bytesReserved, _ := hex.DecodeString("000000")
	//Append to the header after ISCP bytes
	packageISCP = append(packageISCP, bytesReserved...)

	//We add the bytes of the command that we processed before.
	packageISCP = append(packageISCP, bytesCommand...)

	//Add the end bytes to the packet
	packageISCP = append(packageISCP, endBytes...)

	//Add the end characters
	return packageISCP
}

func processISCP(packet []byte) (string, bool) {
	//Check header for valid ISCP format
	if reflect.DeepEqual(packet[0:4], []byte("ISCP")) && //First four characters ISCP
		reflect.DeepEqual(packet[4:8], []byte{0x00, 0x00, 0x00, 0x10}) && //Header size is 16
		reflect.DeepEqual(packet[12:13], []byte{0x01}) && //Version is 01
		reflect.DeepEqual(packet[13:16], []byte{0x00, 0x00, 0x00}) { //Reserved bytes are 000000
		//Get the size of the data from the header
		dataSizeBuf := bytes.NewBuffer(packet[8:12])
		var dataSize int32
		binary.Read(dataSizeBuf, binary.BigEndian, &dataSize)

		var endSize int32
		//And this wonderful block of code is brought to you by the intelligent people that designed the ISCP protocol to use one of many possible end combinations.
		if reflect.DeepEqual(packet[13+dataSize:16+dataSize], EOF_CR_LF) {
			endSize = 3
		} else if reflect.DeepEqual(packet[13+dataSize:16+dataSize], EM_CR_LF) {
			endSize = 3
		} else if reflect.DeepEqual(packet[14+dataSize:16+dataSize], EOF_CR) {
			endSize = 2
		} else if reflect.DeepEqual(packet[14+dataSize:16+dataSize], CR_LF) {
			endSize = 2
		} else if reflect.DeepEqual(packet[15+dataSize:16+dataSize], EOF) {
			endSize = 1
		} else if reflect.DeepEqual(packet[15+dataSize:16+dataSize], CR) {
			endSize = 1
		} else if reflect.DeepEqual(packet[15+dataSize:16+dataSize], LF) {
			endSize = 1
		}

		//Get the actual data in a buffer, then convert to a string
		dataBuf := bytes.NewBuffer(packet[16 : 16+dataSize-endSize])
		data := dataBuf.String()

		return data, true
	}
	return "", false
}

func findDevices() {
	//Keep track of which ending we're currently using
	currentEnd := EOF

	//Create socket, connecting using UDP4, any available local address, to our broadcast IP with the ISCP default port
	socket, err := net.ListenUDP("udp4", &net.UDPAddr{
		IP:   net.IPv4zero, //net.IPv4zero is 0.0.0.0 (bind on all addresses)
		Port: devicePort,
	})

	if err != nil {
		fmt.Println("Error creating UDP socket")
	}

	//Keep looping infinitely
	for {
		//We can constantly generate a UDP socket which uses 60128 since we can run it concurrently with a TCP socket.
		//Here, we're going to broadcast a message, then immediately switch to listening for responses within the next 50ms

		//We'll do this for each type of packet ending, since apparently unknown models can take unknown endings.
		if bytes.Equal(currentEnd, EOF_CR_LF) {
			currentEnd = EOF_CR
		} else if bytes.Equal(currentEnd, EOF_CR) {
			currentEnd = EOF
		} else {
			currentEnd = EOF_CR_LF
		}
		//fmt.Printf("Attempting broadcast using end %X\n", currentEnd)

		//If the socket creates successfully - write the question out using the current ending
		if err == nil {
			socket.WriteToUDP(packageISCP("!xECNQSTN", currentEnd), &net.UDPAddr{
				IP:   net.IPv4bcast, //net.IPv4bcast is 255.255.255.255 (send to all available addresses)
				Port: devicePort,
			})
		}

		//Now we need to close out outbound socket, and open an inbound quickly --
		socketError := make(chan error, 1)

		//Run listen in parallel
		go func() {
			for {
				data := make([]byte, 1024)
				read, remoteAddr, err := socket.ReadFromUDP(data)
				if err != nil {
					socketError <- err
				}
				packet, valid := processISCP(data[:read])

				//This isn't a broadcast question from ourselves or another controller
				if valid && packet[0:5] == "!1ECN" && packet[5:9] != "QSTN" {
					//Get the meta data about the device from its response
					data := packet[5:]
					fragments := strings.Split(data, "/")
					port, _ := strconv.ParseInt(fragments[1], 10, 0)
					//Create and store the device response according to its MAC address8
					if _, present := devices[fragments[3]]; !present {
						devices[fragments[3]] = Device{fragments[0], //Model
							int(port), //Port
							net.ParseIP(strings.Split(remoteAddr.String(), ":")[0]), //IP Address
							fragments[2],       //Region
							fragments[3],       //MAC Address
							new(net.TCPConn),   //And we don't need a TCPConn yet.
							make(chan bool, 1), //Connection open/close statuses
							currentEnd}         //The proper byte ending
					}
					//fmt.Printf("Broadcast response from %s: %s\n", remoteAddr.String(), data)
				}
			}
		}()
		select {
		//Problem reading from socket
		case res := <-socketError:
			fmt.Println(res)
		//Finished the 50ms response limit imposed by ISCP protocol
		case <-time.After(time.Millisecond * 51):
			//fmt.Printf("Finished polling port %d using end %X\n\n", devicePort, currentEnd)
		}

		//We don't want to do this too much, so we'll sleep for five seconds before trying again.
		time.Sleep(time.Second * 1)
	}
}

func main() {
	fmt.Println("Starting eISCP (ethernet Integra Serial Communication Protocol) Intermediary")

	//Start finding devices
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
		//Create a slice of devices to properly format our output
		deviceSlice := make([]Device, 0)
		for _, device := range devices {
			deviceSlice = append(deviceSlice, device)
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
	go deviceConnection(macAddr)
	w.WriteHeader(200)
}

func deviceConnection(id string) {
	socket, err := net.DialTCP("tcp4", nil, &net.TCPAddr{
		IP:   devices[id].IP,
		Port: devices[id].Port,
	})
	if err != nil {
		fmt.Println("Error connecting to device")
		devices[id].statusChan <- false
	} else {
		devices[id].statusChan <- true
	}

	//In parallel, read from socket
	go func() {
		for {
			data := make([]byte, 1024)
			read, err := socket.Read(data)
			if err != nil {
				fmt.Println("Error reading from device")
			}
			packet, valid := processISCP(data[:read])

			if valid {
				fmt.Println(packet)
			}
		}
	}()

	time.Sleep(time.Millisecond * 51)

	fmt.Println()
	count, err := socket.Write(packageISCP("!1MVLQSTN", CR_LF))
	if err != nil {
		fmt.Println("Write failed ", count)
	}
}

func DeleteDevices(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Hello "))
}

func HandleKill(w http.ResponseWriter, r *http.Request) { //Debug function
	w.Write([]byte("Goodbye"))
	go os.Exit(1)
}

//Device type, record of facts learned through broadcast
type Device struct {
	Model      string
	Port       int
	IP         net.IP
	Region     string
	Macaddr    string
	connection *net.TCPConn
	statusChan chan bool
	currentEnd []byte
}

type GetDevicesResponse struct {
	Total   int
	Devices []Device
}
