package main

import (
	// "bufio"
	// "bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os/exec"
	"time"

	"github.com/gorilla/websocket"
	// "github.com/pion/example-webrtc-applications/blob/v3.0.5/internal/gstreamer-src"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	// "github.com/pion/webrtc/v4/pkg/media"
	// "github.com/pion/example-webrtc-applications/v3/internal/gstreamer-src"
	// "github.com/pion/webrtc/v3/examples/internal/gstreamer-src"
	// "github.com/edgeimpulse/linux-sdk-go/image/gstreamer"
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

// func sendTrackFromFFmpeg(peerConnection *webrtc.PeerConnection) error {
// 	// Start FFmpeg process
// 	cmd := exec.Command("ffmpeg", "-i", "input.mp4", "-f", "rtp", "-")
// 	stdout, err := cmd.StdoutPipe()
// 	if err != nil {
// 		return err
// 	}

// 	// Start FFmpeg process
// 	if err := cmd.Start(); err != nil {
// 		return err
// 	}

// 	var track webrtc.TrackLocal
// 	// Create a new RTPSender
// 	rtpSender, err := peerConnection.AddTrack(track)
// 	if err != nil {
// 		return err
// 	}

// 	// Read FFmpeg output and send it through RTPSender
// 	go func() {
// 		buf := make([]byte, 4096)
// 		for {
// 			n, err := stdout.Read(buf)
// 			if err != nil {
// 				break
// 			}

// 			// Send the data through RTPSender
// 			err = rtpSender.Send(webrtc.RTPSendParameters{
// 				Payload: buf[:n],
// 			})
// 			if err != nil {
// 				break
// 			}
// 		}
// 	}()

// 	return nil
// }

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer ws.Close()

	//log.Println("New client connected")

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		log.Printf("error creating peer connection: %v", err)

	}

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

		// if TYPE == Initiation then create and send offer
		if msg.Type == "Initiation" {
			// Example: Creating a data channel to simulate adding a "track"
			// In practice, here you would start your FFmpeg process and prepare your RTPSender

			// _, err := peerConnection.CreateDataChannel("audio", nil)
			// if err != nil {
			// 	log.Printf("Error creating data channel: %v", err)
			// 	continue
			// }
			audioTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: "audio/opus"}, "audio", "pion")
			// audioTrack, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: "audio/opus"}, "audio", "pion")
			if err != nil {
				log.Printf("error creating audio track: %v", err)
				continue
			}

			// Add the audio track to the peer connection
			_, err = peerConnection.AddTrack(audioTrack)
			if err != nil {
				log.Printf("error adding audio track: %v", err)
				continue
			}

			log.Println("11", audioTrack)

			// Create an offer
			offer, err := peerConnection.CreateOffer(nil)
			if err != nil {
				log.Printf("error creating offer: %v", err)
				continue
			}
			//log.Println("11", offer)

			// Set the local description to the offer
			err = peerConnection.SetLocalDescription(offer)
			// //log.Println(peerConnection.LocalDescription())
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

			// When Connected send audio data
			peerConnection.OnConnectionStateChange(func(connectionState webrtc.PeerConnectionState) {
				if connectionState == webrtc.PeerConnectionStateConnected {
					log.Printf("Connection State has changed %s \n", connectionState.String())

					// cmd := exec.Command("ffmpeg", "-stream_loop", "-1", "-i", "file.mp3", "-acodec", "libopus", "-b:a", "128k", "-f", "opus", "-")
					cmd := exec.Command("ffmpeg", "-stream_loop", "-1", "-i", "file.mp3", "-acodec", "pcm_s16le", "-b:a", "128k", "-f", "s16le", "-")
					stdoutPipe, err := cmd.StdoutPipe()
					if err != nil {
						log.Printf("error creating FFmpeg stdout pipe: %v", err)
					}

					if err := cmd.Start(); err != nil {
						log.Printf("error starting FFmpeg process: %v", err)
					}
					// Use a buffer to read the data in chunks
					buf := make([]byte, 4096) // Adjust the size as needed
					// buf2 := make([]byte, 1024) // Adjust the size as needed
					for {
						n, err := stdoutPipe.Read(buf)
						if err != nil {
							if err == io.EOF {
								break // End of file is expected for some commands
							}
							log.Printf("Error reading from stdout pipe: %v\n", err)
							break
						}
						audioTrack.WriteSample(media.Sample{Data: buf[:n], Duration: time.Millisecond * 256})
					}

					// Wait for the command to finish
					if err := cmd.Wait(); err != nil {
						log.Printf("Command finished with error: %v\n", err)
					}
				}
			})

		} else if msg.Type == "answer" {
			// Declare a variable to hold the unmarshaled session description
			var answer webrtc.SessionDescription

			// Unmarshal the JSON string into the SessionDescription struct
			err := json.Unmarshal([]byte(msg.Data), &answer)
			if err != nil {
				log.Printf("error unmarshaling answer: %v", err)
				continue
			}
			// //log.Println(peerConnection.LocalDescription())
			//log.Println("111", answer)
			// Use the unmarshaled session description
			err = peerConnection.SetRemoteDescription(answer)
			if err != nil {
				log.Printf("error setting remote description: %v", err)
				continue
			}

			msgJSON, err := json.Marshal(Message{
				Type: "reqice",
				Data: "Start Sending Ice",
			})
			if err != nil {
				log.Printf("error marshalling ICEreq message to JSON: %v", err)
				return
			}

			if err := ws.WriteMessage(websocket.TextMessage, msgJSON); err != nil {
				log.Printf("error sending ICEreq message over WebSocket: %v", err)
			}

		} else if msg.Type == "candidate" {
			var candidate webrtc.ICECandidateInit
			//log.Println("Received ICE candidate:", msg.Data)
			if err := json.Unmarshal([]byte(msg.Data), &candidate); err != nil {
				log.Printf("error unmarshalling candidate: %v", err)
				return
			}

			// Add the ICE candidate to the peer connection
			if err := peerConnection.AddICECandidate(candidate); err != nil {
				log.Printf("error adding ICE candidate: %v", err)
				return
			}

			//log.Println("Added ICE candidate")
		} else if msg.Type == "reqice" {
			// Handle ICE candidates
			//log.Println(1111, peerConnection.LocalDescription())
			//log.Println("\n\n\n", peerConnection.RemoteDescription())
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
		}

	}
}

func main() {

	// Configure websocket route
	http.HandleFunc("/ws", handleConnections)

	// Start the server on localhost port 8000 and log any errors
	//log.Println("http server started on :6080")
	err := http.ListenAndServe(":6080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
