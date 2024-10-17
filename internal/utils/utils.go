// pkg/utils/utils.go
package utils

import (
	"encoding/json"
	"log"

	"github.com/gorilla/websocket"
)

// Message represents the signaling messages exchanged over WebSocket.
type Message struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

// SendWebSocketMessage sends a JSON message over WebSocket.
func SendWebSocketMessage(ws *websocket.Conn, msg Message) error {
	msgJSON, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	log.Printf("Sending WebSocket message: Type=%s, Data=%s", msg.Type, msg.Data)
	return ws.WriteMessage(websocket.TextMessage, msgJSON)
}
