package network

import (
	"encoding/json"
	"net"
)

type Conn struct {
	net.Conn
}

func NewConn(c net.Conn) *Conn {
	return &Conn{Conn: c}
}

func (c *Conn) Send(msg Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return WriteFrame(c.Conn, data)
}

func (c *Conn) Receive() (*Message, error) {
	data, err := ReadFrame(c.Conn)
	if err != nil {
		return nil, err
	}

	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}