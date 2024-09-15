package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net"
	"net/http"
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
	// Allow all origins (not recommended for production)
	CheckOrigin: func(r *http.Request) bool {
		return true
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

	log.Println("Audio track added to PeerConnection")

	// Use a WaitGroup to wait for goroutines to finish.
	var wg sync.WaitGroup

	// Start reading RTP packets from FFmpeg and sending them to the audio track.
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("Starting RTP packet reading")
		readRTPPackets(ctx, listener, audioTrack)
		log.Println("RTP packet reading stopped")
	}()

	// Start reading RTCP packets (for feedback like NACK).
	wg.Add(1)
	go func() {
		defer wg.Done()
		readRTCPPackets(ctx, rtpSender)
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
			msg := Message{
				Type: "candidate",
				Data: string(candidate),
			}
			log.Printf("Sending ICE candidate: %s", string(candidate))
			sendWebSocketMessage(ws, msg)
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
		var msg Message
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
				msg := Message{
					Type: "candidate",
					Data: string(candidateJSON),
				}
				log.Printf("Sending buffered ICE candidate: %s", string(candidateJSON))
				sendWebSocketMessage(ws, msg)
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

// startFFmpeg starts the FFmpeg process and returns the command and UDP listener.
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
		// FFmpeg command to stream an MP3 file in real-time via RTP.
		cmd = exec.Command(
			"ffmpeg",
			"-re",
			"-stream_loop", "-1",
			"-i", "file2.mp3",
			"-acodec", "libopus",
			"-ar", "48000", // Set sample rate to 48kHz
			"-ac", "2", // Set number of channels to 2 (stereo)
			"-b:a", "128k",
			"-application", "audio", // Ensures Opus is in audio mode
			"-payload_type", "111", // Explicitly set payload type to 111
			"-f", "rtp",
			"rtp://"+address,
			"-tune", "zerolatency",
		)
	} else {
		// FFmpeg command to capture audio from PulseAudio and stream Opus via RTP.
		cmd = exec.Command(
			"ffmpeg",
			"-re",
			"-f", "pulse",
			"-i", "default",
			"-acodec", "libopus",
			"-ar", "48000", // Set sample rate to 48kHz
			"-ac", "2", // Set number of channels to 2 (stereo)
			"-b:a", "128k",
			"-application", "audio", // Ensures Opus is in audio mode
			"-payload_type", "111", // Explicitly set payload type to 111
			"-f", "rtp",
			"rtp://"+address,
			"-tune", "zerolatency",
		)
	}

	// Get the stderr pipe for logging.
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, err
	}

	// Start the FFmpeg process.
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}

	// Log FFmpeg's stderr in a separate goroutine.
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			log.Printf("FFmpeg STDERR: %s", scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			log.Printf("Error reading FFmpeg stderr: %v", err)
		}
	}()

	return cmd, listener, nil
}

// newPeerConnection creates a new WebRTC PeerConnection with a configured MediaEngine.
func newPeerConnection() (*webrtc.PeerConnection, error) {

	turnURL := "turn:freeturn.net:3478" // Changed to lowercase 'turn:'
	turnUsername := "free"
	turnCredential := "free"

	// Initialize MediaEngine and register Opus codec with payload type 111.
	mediaEngine := webrtc.MediaEngine{}
	err := mediaEngine.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:    webrtc.MimeTypeOpus,
			ClockRate:   48000,
			Channels:    2,
			SDPFmtpLine: "minptime=10; useinbandfec=1",
		},
		PayloadType: 111, // Ensure this matches FFmpeg's payload type for Opus
	}, webrtc.RTPCodecTypeAudio)
	if err != nil {
		return nil, err
	}

	// Create a SettingEngine to configure advanced settings.
	settingEngine := webrtc.SettingEngine{}

	// Set the UDP port range to a single port: 50000.
	err = settingEngine.SetEphemeralUDPPortRange(50000, 50000)
	if err != nil {
		return nil, err
	}

	// Create a new API with the MediaEngine and SettingEngine.
	api := webrtc.NewAPI(
		webrtc.WithMediaEngine(&mediaEngine),
		webrtc.WithSettingEngine(settingEngine),
	)

	// Configure ICE servers.
	iceServers := []webrtc.ICEServer{
		{
			URLs: []string{"stun:stun.l.google.com:19302"},
		},
	}

	// Hardcoded TURN server configuration
	if turnURL != "" {
		iceServers = append(iceServers, webrtc.ICEServer{
			URLs:       []string{turnURL},
			Username:   turnUsername,
			Credential: turnCredential,
		})
	}

	// Log ICE server configurations
	for _, server := range iceServers {
		log.Printf("Configured ICE Server: %v", server.URLs)
	}

	// Create the PeerConnection using the API.
	return api.NewPeerConnection(webrtc.Configuration{
		ICETransportPolicy: webrtc.ICETransportPolicyAll,
		ICEServers:         iceServers,
	})
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

	log.Println("Sending SDP offer to client")
	if err := sendWebSocketMessage(ws, msg); err != nil {
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
	msg := Message{
		Type: "reqice",
		Data: "",
	}
	log.Println("Sending reqice message to client")
	return sendWebSocketMessage(ws, msg)
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

// sendWebSocketMessage sends a JSON message over WebSocket.
func sendWebSocketMessage(ws *websocket.Conn, msg Message) error {
	msgJSON, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	log.Printf("Sending WebSocket message: Type=%s, Data=%s", msg.Type, msg.Data)
	return ws.WriteMessage(websocket.TextMessage, msgJSON)
}

// readRTPPackets reads RTP packets from the listener and writes them to the audio track.
func readRTPPackets(ctx context.Context, listener net.PacketConn, audioTrack *webrtc.TrackLocalStaticRTP) {
	buffer := make([]byte, 2048) // Increased buffer size for safety
	for {
		select {
		case <-ctx.Done():
			log.Println("readRTPPackets: Context canceled, stopping RTP reading")
			return
		default:
			n, addr, err := listener.ReadFrom(buffer)
			if err != nil {
				log.Println("readRTPPackets: ReadFrom Error:", err)
				return
			}

			// Log the source of the RTP packet
			log.Printf("Received RTP packet from %s, size=%d bytes", addr.String(), n)

			// Unmarshal the RTP packet.
			packet := &rtp.Packet{}
			if err := packet.Unmarshal(buffer[:n]); err != nil {
				log.Println("readRTPPackets: RTP Packet Unmarshal Error:", err)
				continue
			}

			// Write the RTP packet to the audio track.
			if err := audioTrack.WriteRTP(packet); err != nil {
				log.Println("readRTPPackets: Audio Track Write Error:", err)
				return
			}

			log.Printf("Sent RTP packet Seq=%d Timestamp=%d Size=%d bytes", packet.SequenceNumber, packet.Timestamp, len(packet.Payload))
		}
	}
}

// readRTCPPackets reads RTCP packets from the RTP sender.
func readRTCPPackets(ctx context.Context, rtpSender *webrtc.RTPSender) {
	rtcpBuf := make([]byte, 1500)
	for {
		select {
		case <-ctx.Done():
			log.Println("readRTCPPackets: Context canceled, stopping RTCP reading")
			return
		default:
			n, _, err := rtpSender.Read(rtcpBuf)
			if err != nil {
				if err != io.EOF {
					log.Println("readRTCPPackets: RTCP Read Error:", err)
				}
				return
			}
			log.Printf("readRTCPPackets: Received RTCP packet of size %d bytes", n)
			// Handle RTCP packets as needed
		}
	}
}
