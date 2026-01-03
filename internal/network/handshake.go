package network

import (
	"encoding/json"
	"errors"
)

type HandshakePayload struct {
	ProtocolVersion string `json:"protocol_version"`
}

const ProtocolVersion = "sag/1.0"

func SendHandshake(c *Conn, selfID string) error {
	payload, _ := json.Marshal(HandshakePayload{
		ProtocolVersion: ProtocolVersion,
	})

	return c.Send(Message{
		Type: "HANDSHAKE",
		SenderID: selfID,
		Payload: payload,
	})
}

func ReceiveHandshake(c *Conn) (string, error) {
	msg, err := c.Receive()
	if err != nil {
		return "", err
	}

	if msg.Type != "HANDSHAKE" {
		return "", errors.New("expected HANDSHAKE")
	}

	var payload HandshakePayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return "", err
	}

	if payload.ProtocolVersion != ProtocolVersion {
		return "", errors.New("protocol version mismatch")
	}

	return msg.SenderID, nil
}