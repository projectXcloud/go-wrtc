// pkg/signaling/signaling.go
package signaling

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/ProjectXcloud/go-wrtc/internal/ffmpeg"
	"github.com/ProjectXcloud/go-wrtc/internal/utils"
	webrtcpkg "github.com/ProjectXcloud/go-wrtc/internal/webrtc"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
)

// upgrader upgrades HTTP connections to WebSocket connections.
var upgrader = websocket.Upgrader{
	// Allow all origins (not recommended for production)
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// HandleConnections handles incoming WebSocket connections.
func HandleConnections(w http.ResponseWriter, r *http.Request, testMode bool) {
	// Upgrade the HTTP connection to a WebSocket connection.
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket Upgrade Error:", err)
		return
	}
	defer ws.Close()

	// Log the new client connection.
	log.Println("New client connected")

	// Create a context to manage goroutine lifecycles.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the FFmpeg process for this client.
	ffmpegCmd, listener, err := ffmpeg.StartFFmpeg(testMode)
	if err != nil {
		log.Println("FFmpeg Start Error:", err)
		return
	}
	defer func() {
		if err := ffmpegCmd.Process.Kill(); err != nil {
			log.Println("FFmpeg Process Kill Error:", err)
		}
	}()

	// Create a new PeerConnection per client.
	peerConnection, err := webrtcpkg.NewPeerConnection()
	if err != nil {
		log.Println("PeerConnection Error:", err)
		return
	}
	defer peerConnection.Close()

	// Create a new audio track for this client.
	audioTrack, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus},
		"audio", "pion",
	)
	if err != nil {
		log.Println("Audio Track Error:", err)
		return
	}

	// Add the audio track to the peer connection.
	rtpSender, err := peerConnection.AddTrack(audioTrack)
	if err != nil {
		log.Println("AddTrack Error:", err)
		return
	}

	log.Println("Audio track added to PeerConnection")

	// Use a WaitGroup to wait for goroutines to finish.
	var wg sync.WaitGroup

	// Start reading RTP packets from FFmpeg and sending them to the audio track.
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("Starting RTP packet reading")
		webrtcpkg.ReadRTPPackets(ctx, listener, audioTrack)
		log.Println("RTP packet reading stopped")
	}()

	// Start reading RTCP packets (for feedback like NACK).
	wg.Add(1)
	go func() {
		defer wg.Done()
		webrtcpkg.ReadRTCPPackets(ctx, rtpSender)
	}()

	// Buffered ICE candidates.
	var iceCandidates []webrtc.ICECandidateInit
	var sendCandidates bool = false // Flag indicating whether we have received "reqice" from client

	// Handle ICE candidates.
	peerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			// All ICE candidates have been sent.
			return
		}
		// Marshal the candidate.
		candidate, err := json.Marshal(c.ToJSON())
		if err != nil {
			log.Println("ICE Candidate Marshal Error:", err)
			return
		}

		// If we have received "reqice", send the candidate immediately.
		if sendCandidates {
			msg := utils.Message{
				Type: "candidate",
				Data: string(candidate),
			}
			log.Printf("Sending ICE candidate: %s", string(candidate))
			utils.SendWebSocketMessage(ws, msg)
		} else {
			// Buffer the ICE candidate.
			log.Printf("Buffering ICE candidate: %s", string(candidate))
			var init webrtc.ICECandidateInit
			if err := json.Unmarshal(candidate, &init); err != nil {
				log.Println("ICE Candidate Unmarshal Error:", err)
				return
			}
			iceCandidates = append(iceCandidates, init)
		}
	})

	// Handle the peer connection state changes.
	peerConnection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("Connection State has changed: %s\n", state.String())
		if state == webrtc.PeerConnectionStateFailed ||
			state == webrtc.PeerConnectionStateClosed {
			// Cancel the context to stop goroutines.
			cancel()
		}
	})

	// Main loop to handle incoming WebSocket messages.
	for {
		_, p, err := ws.ReadMessage()
		if err != nil {
			log.Println("WebSocket Read Error:", err)
			break
		}

		// Unmarshal the JSON message.
		var msg utils.Message
		if err := json.Unmarshal(p, &msg); err != nil {
			log.Println("Message Unmarshal Error:", err)
			continue
		}

		log.Printf("Received WebSocket message: Type=%s, Data=%s", msg.Type, msg.Data)

		// Handle the signaling messages.
		switch msg.Type {
		case "Initiation":
			// Create and send the offer.
			log.Println("Handling Initiation message")
			if err := handleInitiation(ws, peerConnection); err != nil {
				log.Println("Handle Initiation Error:", err)
				return
			}
		case "answer":
			// Set the remote description with the answer.
			log.Println("Handling answer message")
			if err := handleAnswer(peerConnection, msg.Data, ws); err != nil {
				log.Println("Handle Answer Error:", err)
				return
			}
		case "candidate":
			// Add the ICE candidate.
			log.Println("Handling candidate message")
			if err := handleCandidate(peerConnection, msg.Data); err != nil {
				log.Println("Handle Candidate Error:", err)
				return
			}
		case "reqice":
			// Start sending ICE candidates
			log.Println("Handling reqice message")
			sendCandidates = true
			// Send all buffered ICE candidates
			for _, candidate := range iceCandidates {
				candidateJSON, err := json.Marshal(candidate)
				if err != nil {
					log.Println("ICE Candidate Marshal Error:", err)
					continue
				}
				msg := utils.Message{
					Type: "candidate",
					Data: string(candidateJSON),
				}
				log.Printf("Sending buffered ICE candidate: %s", string(candidateJSON))
				utils.SendWebSocketMessage(ws, msg)
			}
			// Clear the buffer
			iceCandidates = nil
		default:
			log.Println("Unknown message type:", msg.Type)
		}
	}

	// Wait for goroutines to finish.
	wg.Wait()
	log.Println("Connection handling completed")
}

// handleInitiation handles the initiation message and sends an offer.
func handleInitiation(ws *websocket.Conn, peerConnection *webrtc.PeerConnection) error {
	// Create an offer.
	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		return err
	}

	// Set the local description.
	if err := peerConnection.SetLocalDescription(offer); err != nil {
		return err
	}

	// Wait until ICE gathering is complete.
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)
	<-gatherComplete

	// Send the offer to the client.
	offerSDP, err := json.Marshal(peerConnection.LocalDescription())
	if err != nil {
		return err
	}

	msg := utils.Message{
		Type: "offer",
		Data: string(offerSDP),
	}

	log.Println("Sending SDP offer to client")
	if err := utils.SendWebSocketMessage(ws, msg); err != nil {
		return err
	}

	return nil
}

// handleAnswer sets the remote description with the client's answer.
func handleAnswer(peerConnection *webrtc.PeerConnection, data string, ws *websocket.Conn) error {
	var answer webrtc.SessionDescription
	if err := json.Unmarshal([]byte(data), &answer); err != nil {
		return err
	}
	if err := peerConnection.SetRemoteDescription(answer); err != nil {
		return err
	}

	// Send "reqice" message to the client indicating we are ready to receive ICE candidates
	msg := utils.Message{
		Type: "reqice",
		Data: "",
	}
	log.Println("Sending reqice message to client")
	return utils.SendWebSocketMessage(ws, msg)
}

// handleCandidate adds an ICE candidate to the peer connection.
func handleCandidate(peerConnection *webrtc.PeerConnection, data string) error {
	var candidate webrtc.ICECandidateInit
	if err := json.Unmarshal([]byte(data), &candidate); err != nil {
		return err
	}
	log.Printf("Adding ICE candidate: %+v", candidate)
	return peerConnection.AddICECandidate(candidate)
}
