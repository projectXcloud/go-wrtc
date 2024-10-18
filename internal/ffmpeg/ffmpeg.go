// pkg/ffmpeg/ffmpeg.go
package ffmpeg

import (
	"bufio"
	"log"
	"net"
	"os/exec"
)

// StartFFmpeg starts the FFmpeg process and returns the command and UDP listener.
func StartFFmpeg(testMode bool) (*exec.Cmd, net.PacketConn, error) {
	// Create a UDP listener on a random port.
	listener, err := net.ListenPacket("udp", "127.0.0.1:40000")
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
			"-c:a", "libopus",
			"-frame_duration", "40",
			"-application", "lowdelay", // Ensures Opus is in lowdelay mode
			"-ar", "48000", // Set sample rate to 48kHz
			"-ac", "2", // Set number of channels to 2 (stereo)
			"-b:a", "128k",
			"-payload_type", "111", // Explicitly set payload type to 111
			"-f", "rtp",
			"rtp://"+address,
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
