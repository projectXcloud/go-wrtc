// cmd/go-wrtc/main.go
package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/ProjectXcloud/go-wrtc/internal/signaling"
)

func main() {
	// Define the test mode flag.
	testMode := flag.Bool("test", false, "Run server in test mode with MP3 file playback")
	flag.BoolVar(testMode, "t", false, "Run server in test mode with MP3 file playback (shorthand)")
	flag.Parse()

	// Configure the WebSocket route.
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		signaling.HandleConnections(w, r, *testMode)
	})

	// Start the server on localhost port 6080 and log any errors.
	log.Println("Server starting on :6080")
	if err := http.ListenAndServe(":6080", nil); err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
