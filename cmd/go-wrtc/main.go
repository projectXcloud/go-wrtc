package main

import (
	// "bufio"
	// "bytes"
	"encoding/json"

	// "errors"
	// "io"
	"log"
	"net"
	"net/http"
	// "os/exec"

	"github.com/gorilla/websocket"
	// "github.com/pion/example-webrtc-applications/blob/v3.0.5/internal/gstreamer-src"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
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

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer ws.Close()

	//log.Println("New client connected")

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs:           []string{"turn:freeturn.net:3478"},
				Username:       "free",
				Credential:     "free",
				CredentialType: webrtc.ICECredentialTypePassword,
			},
		},
		
	})
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
			audioTrack, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus}, "audio", "pion")
			// audioTrack, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: "audio/opus"}, "audio", "pion")
			if err != nil {
				log.Printf("error creating audio track: %v", err)
				continue
			}

			// Add the audio track to the peer connection
			rtpSender, err := peerConnection.AddTrack(audioTrack)
			// _, err = peerConnection.AddTrack(audioTrack)
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
					// cmd := exec.Command("ffmpeg", "-stream_loop", "-1", "-i", "file.mp3", "-acodec", "pcm_s16le", "-b:a", "128k", "-f", "s16le", "-")
					// cmd := exec.Command("ffmpeg", "-stream_loop", "-1", "-i", "file.mp3", "-acodec", "libopus", "-b:a", "128k", "-f", "rtp", "rtp://127.0.0.1:12345")
					// cmd := exec.Command("ffmpeg", "-stream_loop", "-1", "-i", "file2.mp3", "-acodec", "libopus", "-b:a", "128k", "-f", "rtp", "rtp://127.0.0.1:12345")
					// cmd := exec.Command("ffmpeg", "-re", "-stream_loop", "-1", "-i", "file2.mp3", "-acodec", "libopus", "-b:a", "128k", "-f", "rtp", "rtp://127.0.0.1:12345", "-tune", "zerolatency")


					// // Start FFmpeg process
					// if err := cmd.Start(); err != nil {
					// 	log.Printf("error starting FFmpeg process: %v", err)
					// 	return
					// }

					// Open a UDP Listener for RTP Packets on port 12345
					// listener, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345})
					listener, err := net.ListenPacket("udp", "localhost:12345")

					if err != nil {
						panic(err)
					}

					// Increase the UDP receive buffer size
					// Default UDP buffer sizes vary on different operating systems
					// bufferSize := 40960 // 40KB
					// err = listener.SetReadBuffer(bufferSize)
					// if err != nil {
					// panic(err)
					// }

					defer func() {
						if err = listener.Close(); err != nil {
							panic(err)
						}
					}()

					// Read incoming RTCP packets
					// Before these packets are returned they are processed by interceptors. For things
					// like NACK this needs to be called.
					go func() {
						rtcpBufRead := make([]byte, 1500)
						for {
							if _, _, rtcpErr := rtpSender.Read(rtcpBufRead); rtcpErr != nil {
								return
							}
						}
					}()

					go func() {
						// Read incoming RTP packets
						rtpBuf := make([]byte, 15000)
						for {
							n, _, rtpErr := listener.ReadFrom(rtpBuf)
							if rtpErr != nil {
								log.Fatal(err)
							}

							// // Write the RTP packet to the peer
							// if _, writeErr := audioTrack.Write(rtpBuf); writeErr != nil {
							// 	return
							// }
							packet := &rtp.Packet{}
							err = packet.Unmarshal(rtpBuf[:n])
							if err != nil {
								log.Println("Failed to parse RTP packet:", err)
								continue
							}
							// log.Println("Packet Timestamp", packet.Timestamp)
							audioTrack.WriteRTP(packet)

						}
					}()

					// Wait for the command to finish
					// if err := cmd.Wait(); err != nil {
					// 	log.Printf("Command finished with error: %v\n", err)
					// }
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
