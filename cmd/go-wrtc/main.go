package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

// Message represents the signaling messages exchanged over WebSocket.
type Message struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

// upgrader upgrades HTTP connections to WebSocket connections.
var upgrader = websocket.Upgrader{
	// Implement proper origin checks to enhance security.
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		allowedOrigins := []string{
			"http://yourdomain.com", // Replace with your actual domain
			"http://localhost:8000", // Allow localhost (default port from web.go)
			"http://127.0.0.1:8000", // Allow localhost via IP
		}

		for _, ao := range allowedOrigins {
			if origin == ao {
				return true
			}
		}
		log.Printf("Rejected connection from origin: %s", origin)
		return false
	},
}

func main() {
	// Define the test mode flag.
	testMode := flag.Bool("test", false, "Run server in test mode with MP3 file playback")
	flag.BoolVar(testMode, "t", false, "Run server in test mode with MP3 file playback (shorthand)")
	flag.Parse()

	// Configure the WebSocket route.
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handleConnections(w, r, *testMode)
	})

	// Start the server on localhost port 6080 and log any errors.
	log.Println("Server starting on :6080")
	if err := http.ListenAndServe(":6080", nil); err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

// handleConnections handles incoming WebSocket connections.
func handleConnections(w http.ResponseWriter, r *http.Request, testMode bool) {
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
	ffmpegCmd, listener, err := startFFmpeg(testMode)
	if err != nil {
		log.Println("FFmpeg Start Error:", err)
		return
	}
	defer func() {
		if err := ffmpegCmd.Process.Kill(); err != nil {
			log.Println("FFmpeg Process Kill Error:", err)
		}
		listener.Close()
	}()

	// Create a new PeerConnection per client.
	peerConnection, err := newPeerConnection()
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

	// Use a WaitGroup to wait for goroutines to finish.
	var wg sync.WaitGroup

	// Start reading RTP packets and sending them to the audio track.
	wg.Add(1)
	go func() {
		defer wg.Done()
		readRTPPackets(ctx, listener, audioTrack)
	}()

	// Start reading RTCP packets (for feedback like NACK).
	wg.Add(1)
	go func() {
		defer wg.Done()
		readRTCPPackets(ctx, rtpSender)
	}()

	// Handle ICE candidates.
	peerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			// All ICE candidates have been sent.
			return
		}
		// Send the ICE candidate to the client.
		candidate, err := json.Marshal(c.ToJSON())
		if err != nil {
			log.Println("ICE Candidate Marshal Error:", err)
			return
		}
		msg := Message{
			Type: "candidate",
			Data: string(candidate),
		}
		sendWebSocketMessage(ws, msg)
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
		var msg Message
		if err := json.Unmarshal(p, &msg); err != nil {
			log.Println("Message Unmarshal Error:", err)
			continue
		}

		// Handle the signaling messages.
		switch msg.Type {
		case "Initiation":
			// Create and send the offer.
			if err := handleInitiation(ws, peerConnection); err != nil {
				log.Println("Handle Initiation Error:", err)
				return
			}
		case "answer":
			// Set the remote description with the answer.
			if err := handleAnswer(peerConnection, msg.Data); err != nil {
				log.Println("Handle Answer Error:", err)
				return
			}
		case "candidate":
			// Add the ICE candidate.
			if err := handleCandidate(peerConnection, msg.Data); err != nil {
				log.Println("Handle Candidate Error:", err)
				return
			}
		default:
			log.Println("Unknown message type:", msg.Type)
		}
	}

	// Wait for goroutines to finish.
	wg.Wait()
}

// startFFmpeg starts the FFmpeg process and returns the command and listener.
func startFFmpeg(testMode bool) (*exec.Cmd, net.PacketConn, error) {
	// Create a UDP listener on a random port.
	listener, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, err
	}

	// Get the address to which FFmpeg should stream.
	address := listener.LocalAddr().String()

	// Construct the FFmpeg command based on the mode.
	var cmd *exec.Cmd
	if testMode {
		// FFmpeg command to stream an MP3 file in real-time.
		cmd = exec.Command(
			"ffmpeg", "-re", "-stream_loop", "-1", "-i", "file2.mp3",
			"-acodec", "libopus", "-b:a", "128k",
			"-f", "rtp", "rtp://"+address, "-tune", "zerolatency",
		)
	} else {
		// FFmpeg command to start RTP stream from PulseAudio.
		cmd = exec.Command(
			"ffmpeg", "-re", "-f", "pulse", "-i", "default",
			"-acodec", "libopus", "-b:a", "128k",
			"-f", "rtp", "rtp://"+address, "-tune", "zerolatency",
		)
	}

	// Start the FFmpeg process.
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}

	return cmd, listener, nil
}

// newPeerConnection creates a new WebRTC PeerConnection.
func newPeerConnection() (*webrtc.PeerConnection, error) {
	// Load TURN server credentials from environment variables.
	turnURL := os.Getenv("TURN_URL") // e.g., "turn:yourturnserver.com:3478"
	turnUsername := os.Getenv("TURN_USERNAME")
	turnCredential := os.Getenv("TURN_CREDENTIAL")

	// Create the ICE servers configuration.
	iceServers := []webrtc.ICEServer{
		{
			URLs: []string{"stun:stun.l.google.com:19302"},
		},
	}

	if turnURL != "" {
		iceServers = append(iceServers, webrtc.ICEServer{
			URLs:           []string{turnURL},
			Username:       turnUsername,
			Credential:     turnCredential,
			CredentialType: webrtc.ICECredentialTypePassword,
		})
	}

	// Create a new PeerConnection configuration.
	config := webrtc.Configuration{
		ICEServers: iceServers,
	}

	// Create the PeerConnection.
	return webrtc.NewPeerConnection(config)
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

	msg := Message{
		Type: "offer",
		Data: string(offerSDP),
	}

	return sendWebSocketMessage(ws, msg)
}

// handleAnswer sets the remote description with the client's answer.
func handleAnswer(peerConnection *webrtc.PeerConnection, data string) error {
	var answer webrtc.SessionDescription
	if err := json.Unmarshal([]byte(data), &answer); err != nil {
		return err
	}
	return peerConnection.SetRemoteDescription(answer)
}

// handleCandidate adds an ICE candidate to the peer connection.
func handleCandidate(peerConnection *webrtc.PeerConnection, data string) error {
	var candidate webrtc.ICECandidateInit
	if err := json.Unmarshal([]byte(data), &candidate); err != nil {
		return err
	}
	return peerConnection.AddICECandidate(candidate)
}

// sendWebSocketMessage sends a JSON message over WebSocket.
func sendWebSocketMessage(ws *websocket.Conn, msg Message) error {
	msgJSON, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return ws.WriteMessage(websocket.TextMessage, msgJSON)
}

// readRTPPackets reads RTP packets from the listener and writes them to the audio track.
func readRTPPackets(ctx context.Context, listener net.PacketConn, audioTrack *webrtc.TrackLocalStaticRTP) {
	rtpBuf := make([]byte, 1500)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			n, _, err := listener.ReadFrom(rtpBuf)
			if err != nil {
				log.Println("RTP Read Error:", err)
				return
			}

			// Unmarshal the RTP packet.
			packet := &rtp.Packet{}
			if err := packet.Unmarshal(rtpBuf[:n]); err != nil {
				log.Println("RTP Packet Unmarshal Error:", err)
				continue
			}

			// Write the RTP packet to the audio track.
			if err := audioTrack.WriteRTP(packet); err != nil {
				log.Println("Audio Track Write Error:", err)
				return
			}
		}
	}
}

// readRTCPPackets reads RTCP packets from the RTP sender.
func readRTCPPackets(ctx context.Context, rtpSender *webrtc.RTPSender) {
	rtcpBuf := make([]byte, 1500)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if _, _, err := rtpSender.Read(rtcpBuf); err != nil {
				if err != io.EOF {
					log.Println("RTCP Read Error:", err)
				}
				return
			}
		}
	}
}
