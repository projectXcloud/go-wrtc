// pkg/webrtc/webrtc.go
package webrtc

import (
	"context"
	"io"
	"log"
	"net"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

// NewPeerConnection creates a new WebRTC PeerConnection with a configured MediaEngine.
func NewPeerConnection() (*webrtc.PeerConnection, error) {
	turnURL := "turn:freestun.net:3478" // Changed to lowercase 'turn:'
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
		ICETransportPolicy: webrtc.ICETransportPolicyRelay,
		ICEServers:         iceServers,
	})
}

// ReadRTPPackets reads RTP packets from the listener and writes them to the audio track.
func ReadRTPPackets(ctx context.Context, listener net.PacketConn, audioTrack *webrtc.TrackLocalStaticRTP) {
	buffer := make([]byte, 2048) // Increased buffer size for safety
	for {
		select {
		case <-ctx.Done():
			log.Println("ReadRTPPackets: Context canceled, stopping RTP reading")
			return
		default:
			n, addr, err := listener.ReadFrom(buffer)
			if err != nil {
				log.Println("ReadRTPPackets: ReadFrom Error:", err)
				return
			}

			// Log the source of the RTP packet
			log.Printf("Received RTP packet from %s, size=%d bytes", addr.String(), n)

			// Unmarshal the RTP packet.
			packet := &rtp.Packet{}
			if err := packet.Unmarshal(buffer[:n]); err != nil {
				log.Println("ReadRTPPackets: RTP Packet Unmarshal Error:", err)
				continue
			}

			// Write the RTP packet to the audio track.
			if err := audioTrack.WriteRTP(packet); err != nil {
				log.Println("ReadRTPPackets: Audio Track Write Error:", err)
				return
			}

			log.Printf("Sent RTP packet Seq=%d Timestamp=%d Size=%d bytes", packet.SequenceNumber, packet.Timestamp, len(packet.Payload))
		}
	}
}

// ReadRTCPPackets reads RTCP packets from the RTP sender.
func ReadRTCPPackets(ctx context.Context, rtpSender *webrtc.RTPSender) {
	rtcpBuf := make([]byte, 1500)
	for {
		select {
		case <-ctx.Done():
			log.Println("ReadRTCPPackets: Context canceled, stopping RTCP reading")
			return
		default:
			n, _, err := rtpSender.Read(rtcpBuf)
			if err != nil {
				if err != io.EOF {
					log.Println("ReadRTCPPackets: RTCP Read Error:", err)
				}
				return
			}
			log.Printf("ReadRTCPPackets: Received RTCP packet of size %d bytes", n)
			// Handle RTCP packets as needed
		}
	}
}
