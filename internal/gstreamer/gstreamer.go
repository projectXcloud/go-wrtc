// internal/gstreamer/gstreamer.go
package gstreamer

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os/exec"
)

// StartGStreamer starts the GStreamer process and returns the command and UDP listener.
func StartGStreamer(testMode bool) (*exec.Cmd, net.PacketConn, error) {
	// Create a UDP listener on a random port.
	listener, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, err
	}

	// Get the address to which GStreamer should stream.
	address := listener.LocalAddr().(*net.UDPAddr)
	port := address.Port

	var cmd *exec.Cmd
	if testMode {
		// GStreamer command to stream an MP3 file in real-time via RTP.
		cmd = exec.Command("gst-launch-1.0", "-v", "-e",
			"filesrc", "location=file2.mp3", "!",
			"decodebin", "!",
			"audioconvert", "!",
			"audioresample", "!",
			"opusenc", "bitrate=128000", "!",
			"rtpopuspay", "pt=111", "!",
			"udpsink", "host=127.0.0.1", fmt.Sprintf("port=%d", port),
		)
	} else {
		// GStreamer command to capture audio from PulseAudio and stream Opus via RTP.
		cmd = exec.Command("gst-launch-1.0", "-v", "-e",
			"pulsesrc", "!",
			"audioconvert", "!",
			"audioresample", "!",
			"opusenc", "bitrate=128000", "!",
			"rtpopuspay", "pt=111", "!",
			"udpsink", "host=127.0.0.1", fmt.Sprintf("port=%d", port),
		)
	}

	// Get the stderr pipe for logging.
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, err
	}

	// Start the GStreamer process.
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}

	// Log GStreamer's stderr in a separate goroutine.
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			log.Printf("GStreamer STDERR: %s", scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			log.Printf("Error reading GStreamer stderr: %v", err)
		}
	}()

	// Optionally, log the GStreamer command for debugging.
	log.Printf("Started GStreamer with PID %d", cmd.Process.Pid)

	return cmd, listener, nil
}
