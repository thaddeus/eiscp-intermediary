package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"reflect"
)

// eISCP sucks at telling us which endBytes we should expect. So have fun with these.
var EOF = []byte{0x1a}
var CR = []byte{0x0d}
var LF = []byte{0x0a}
var EOF_CR = []byte{0x1a, 0x0d}
var CR_LF = []byte{0x0d, 0x0a}
var EM_CR_LF = []byte{0x19, 0x0d, 0x0a}
var EOF_CR_LF = []byte{0x1a, 0x0d, 0x0a}

// Collection of end bytes for convencience, from biggest to smallest.
var endBytes = [][]byte{EOF_CR_LF, EM_CR_LF, CR_LF, EOF_CR, LF, CR, EOF}

// Take a command and encapsulate it according to the ISCP guidelines
func packageISCP(command string, endBytes []byte) []byte {

	// Start forming header
	packageISCP := make([]byte, 0)
	// Beginning of every message
	bytesISCP := []byte("ISCP")
	// Append ISCP to the end (the beginning of our empty byte array) of header
	packageISCP = append(packageISCP, bytesISCP...)

	// eISCP header size (hex decode string is probably the wrong way to do this)
	bytesHeaderSize, _ := hex.DecodeString("00000010")
	// Append to the header after ISCP bytes
	packageISCP = append(packageISCP, bytesHeaderSize...)

	// Get command bytes (we need the size), we'll also use this later
	bytesCommand := []byte(command)
	// Createa a new byte array of length 4 for the command size
	commandSize := make([]byte, 4)
	// Take the length of the command and put it in the previous byte array. We add 3 for the data end characters.
	binary.BigEndian.PutUint32(commandSize, uint32(len(bytesCommand)+len(endBytes)))
	// Append the command length to the current end of the header
	packageISCP = append(packageISCP, commandSize...)

	// eISCP version (again, hex decode string is probably inefficient)
	byteVersion, _ := hex.DecodeString("01")
	// Append to the header after ISCP bytes
	packageISCP = append(packageISCP, byteVersion...)

	// eISCP reserved bytes (again, inefficient)
	bytesReserved, _ := hex.DecodeString("000000")
	// Append to the header after ISCP bytes
	packageISCP = append(packageISCP, bytesReserved...)

	// We add the bytes of the command that we processed before.
	packageISCP = append(packageISCP, bytesCommand...)

	// Add the end bytes to the packet
	packageISCP = append(packageISCP, endBytes...)

	// Add the end characters
	return packageISCP
}

// Deconstructs an ISCP packet
func processISCP(packet []byte) (string, bool) {
	// Check header for valid ISCP format
	if reflect.DeepEqual(packet[0:4], []byte("ISCP")) && // First four characters ISCP
		reflect.DeepEqual(packet[4:8], []byte{0x00, 0x00, 0x00, 0x10}) && // Header size is 16
		reflect.DeepEqual(packet[12:13], []byte{0x01}) && // Version is 01
		reflect.DeepEqual(packet[13:16], []byte{0x00, 0x00, 0x00}) { // Reserved bytes are 000000
		// Get the size of the data from the header
		dataSizeBuf := bytes.NewBuffer(packet[8:12])
		var dataSize int32
		binary.Read(dataSizeBuf, binary.BigEndian, &dataSize)

		var endSize int32

		for _, endCharacter := range endBytes {
			if reflect.DeepEqual(packet[16-int32(len(endCharacter))+dataSize:16+dataSize], endCharacter) {
				if debug {
					fmt.Printf("DEBUG: Received packet with 0x%X ending\n", endCharacter)
				}
				endSize = int32(len(endCharacter))
				break
			}
		}

		// Get the actual data in a buffer, then convert to a string
		dataBuf := bytes.NewBuffer(packet[16 : 16+dataSize-endSize])
		data := dataBuf.String()

		return data, true
	}
	return "", false
}
