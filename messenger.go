package main

import (
	"fmt"
	"net"
	"time"
)

// Used for active connections and caching
type deviceMessenger struct {
	device     *deviceInfo       // General info found during broadcast
	connection *net.TCPConn      // Connection to device, we die if this dies
	endBytes   []byte            // The end packet bytes the device will respond to
	properties map[string]string // Interface for setting/getting/caching device properties
	killFlag   bool              // Do we need to stop mediating this device?
	recvCount  int               // Number of received packets
	sendCount  int               // Number of sent packets
}

// Device communication loop
func (d *deviceMessenger) mediate() {
	defer eraseMessenger(d)
	for !d.killFlag {
		data := make([]byte, 1024)
		d.connection.SetReadDeadline(time.Now().Add(time.Second))
		read, err := d.connection.Read(data)
		switch err := err.(type) {
		case net.Error:
			if err.Timeout() {
				// Timeout error, we expect this.
			} else {
				// Unexpected net error
			}
		default:
			// No error?
		}
		packet, valid := processISCP(data[:read])

		if valid {
			packetType := packet[2:5]
			packetData := packet[5:]
			d.properties[packetType] = packetData
			if debug {
				fmt.Println("DEBUG: Packet:", packet, "received from", d.connection.RemoteAddr().String(), "(", d.device.Macaddr, ")")
			}
		}
		d.recvCount++
	}
}

func (d *deviceMessenger) message(command string) {
	// This block is used to make sure we're using end bytes that the device understands
	if d.endBytes == nil {
		for _, broadcastEndBytes := range endBytes {
			curCount := d.recvCount
			count, err := d.connection.Write(packageISCP("!1PWRQSTN", broadcastEndBytes))
			if err != nil {
				fmt.Println("Write failed", count, "with", d.device.Macaddr)
			}
			for d.recvCount <= curCount {
				time.Sleep(time.Millisecond * 25)
			}
			if d.properties["PWR"] != "N/A" {
				d.endBytes = broadcastEndBytes
			}
		}
	}
	count, err := d.connection.Write(packageISCP(command, d.endBytes))
	if err != nil {
		fmt.Println("Write failed", count, "with", d.device.Macaddr)
	}
	d.sendCount++
}

func (d *deviceMessenger) getProperty(property string) string {
	if _, present := d.properties[property]; !present {
		timeout := time.Now().Add(time.Second)
		d.message("!1" + property + "QSTN")
		valid := false
		for !valid && time.Now().Before(timeout) {
			time.Sleep(time.Millisecond * 25)
			_, present := d.properties[property]
			valid = present
		}
		if valid {
			return d.properties[property]
		}
	} else {
		return d.properties[property]
	}
	return ""
}

func (d *deviceMessenger) setProperty() {

}

func (d *deviceMessenger) kill() {
	d.killFlag = true
	fmt.Println("Killing connection with", d.device.Macaddr)
}

func eraseMessenger(d *deviceMessenger) {
	d = nil
}
