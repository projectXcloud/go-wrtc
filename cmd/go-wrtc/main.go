package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
)

type Message struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Adjust the origin check as needed
	},
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer ws.Close()

	log.Println("New client connected")

	for {
		_, p, err := ws.ReadMessage()
		if err != nil {
			log.Printf("error: %v", err)
			break
		}

		// Temporarily log the raw message for debugging
		// log.Printf("Raw message: %s", string(p))

		// Unmarshal the JSON into the Message struct
		var msg Message
		err = json.Unmarshal(p, &msg)
		if err != nil {
			log.Printf("error unmarshalling message: %v", err)
			continue
		}

		// Log the unmarshalled Message
		log.Printf("Received message: %+v", msg)

		var peerConnection *webrtc.PeerConnection

		// if TYPE == Initiation then create and send offer
		if msg.Type == "Initiation" {
			// Create a new RTCPeerConnection
			peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{})
			if err != nil {
				log.Printf("error creating peer connection: %v", err)
				continue
			}
			defer peerConnection.Close()

			// Handle ICE candidates
			peerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
				if c == nil {
					// All ICE candidates have been gathered
					return
				}

				candidate, err := json.Marshal(c.ToJSON())
				if err != nil {
					log.Printf("error marshalling ICE candidate to JSON: %v", err)
					return
				}

				iceCandidateMsg := Message{
					Type: "candidate",
					Data: string(candidate),
				}

				msgJSON, err := json.Marshal(iceCandidateMsg)
				if err != nil {
					log.Printf("error marshalling ICE candidate message to JSON: %v", err)
					return
				}

				if err := ws.WriteMessage(websocket.TextMessage, msgJSON); err != nil {
					log.Printf("error sending ICE candidate message over WebSocket: %v", err)
				}
			})

			// Create an offer
			offer, err := peerConnection.CreateOffer(nil)
			if err != nil {
				log.Printf("error creating offer: %v", err)
				continue
			}

			// Set the local description to the offer
			err = peerConnection.SetLocalDescription(offer)
			if err != nil {
				log.Printf("error setting local description: %v", err)
				continue
			}

			// Convert the offer to JSON to get a string representation
			offerSDP, err := json.Marshal(offer)
			if err != nil {
				log.Printf("error marshalling offer to JSON: %v", err)
				continue
			}

			// Wrap the offer in the Message struct
			msg := Message{
				Type: "offer",
				Data: string(offerSDP),
			}

			// Marshal the Message struct to JSON for sending
			msgJSON, err := json.Marshal(msg)
			if err != nil {
				log.Printf("error marshalling message to JSON: %v", err)
				continue
			}

			// Send the Message struct as a JSON string over WebSocket
			if err := ws.WriteMessage(websocket.TextMessage, msgJSON); err != nil {
				log.Printf("error sending message over WebSocket: %v", err)
				continue
			}
		} else if msg.Type == "candidate" {
			var candidate webrtc.ICECandidateInit
			if err := json.Unmarshal([]byte(msg.Data), &candidate); err != nil {
				log.Printf("error unmarshalling candidate: %v", err)
				return
			}

			// Add the ICE candidate to the peer connection
			if err := peerConnection.AddICECandidate(candidate); err != nil {
				log.Printf("error adding ICE candidate: %v", err)
				return
			}

			log.Println("Added ICE candidate")
		}

	}
}

func main() {

	// Configure websocket route
	http.HandleFunc("/ws", handleConnections)

	// Start the server on localhost port 8000 and log any errors
	log.Println("http server started on :6080")
	err := http.ListenAndServe(":6080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
