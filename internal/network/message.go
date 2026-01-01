package network

import "encoding/json"

type Message struct {
	Type     string          `json:"type"`
	SenderID string          `json:"sender_id"`
	Payload  json.RawMessage `json:"payload"`
}
