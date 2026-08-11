package network

import (
	"encoding/binary"
	"fmt"
	"io"
)

// MaxFrameSize bounds how large a single frame's declared length may be.
// Without a cap, a corrupted or hostile 4-byte length prefix would make
// ReadFrame allocate an attacker-controlled amount of memory before ever
// validating the data. 16MiB comfortably covers a base64-encoded 1MB
// chunk (filemeta.DefaultChunkSize) plus JSON/message overhead.
const MaxFrameSize = 16 * 1024 * 1024

// length-prefix framing of 4 bytes
func WriteFrame(w io.Writer, data []byte) error {
	length := uint32(len(data))
	if err := binary.Write(w, binary.BigEndian, length); err != nil {
		return err
	}
	_, err := w.Write(data)
	return err
}

func ReadFrame(r io.Reader) ([]byte, error) {
	var length uint32
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return nil, err
	}

	if length > MaxFrameSize {
		return nil, fmt.Errorf("frame length %d exceeds max %d", length, MaxFrameSize)
	}

	buf := make([]byte, length)
	_, err := io.ReadFull(r, buf)
	return buf, err
}
