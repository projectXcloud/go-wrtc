package main

import (
	"fmt"
	"log"
	"net"

	"github.com/pion/rtp"
)

func main() {
	// Create a UDP connection to receive RTP packets
	conn, err := net.ListenPacket("udp", "localhost:12345")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	// Create a buffer to hold the received RTP packets
	buffer := make([]byte, 1500)

	// Loop to continuously receive and process RTP packets
	for {
		// Read an RTP packet from the UDP connection
		n, _, err := conn.ReadFrom(buffer)
		if err != nil {
			log.Fatal(err)
		}
		if err != nil {
			log.Fatal(err)
		}

		// Parse the received RTP packet
		packet := &rtp.Packet{}
		err = packet.Unmarshal(buffer[:n])
		if err != nil {
			log.Println("Failed to parse RTP packet:", err)
			continue
		}

		// Process the RTP packet
		// TODO: Add your code here to handle the RTP packet data

		// Print the sequence number of the RTP packet
		fmt.Println("Received RTP packet with sequence number:", packet.SequenceNumber, "and timestamp:", packet.PayloadType)
	}
}
