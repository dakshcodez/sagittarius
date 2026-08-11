// Package quicconn adapts a QUIC stream + connection pair to the net.Conn
// interface, so the existing TCP-oriented network/transfer/tracker code
// (which only ever asks for a net.Conn) can run unmodified over QUIC.
package quicconn

import (
	"net"
	"time"

	"github.com/quic-go/quic-go"
)

// Conn wraps a quic.Stream (for Read/Write/Close/deadlines) and its parent
// quic.Conn (for addressing) to satisfy net.Conn.
type Conn struct {
	stream *quic.Stream
	qconn  *quic.Conn
}

// Wrap adapts stream (opened or accepted on qconn) into a net.Conn.
func Wrap(qconn *quic.Conn, stream *quic.Stream) *Conn {
	return &Conn{stream: stream, qconn: qconn}
}

func (c *Conn) Read(b []byte) (int, error)  { return c.stream.Read(b) }
func (c *Conn) Write(b []byte) (int, error) { return c.stream.Write(b) }

// Close closes this stream only, not the underlying QUIC connection, so
// callers that intend to close the whole connection should call
// Conn.CloseConnection instead.
func (c *Conn) Close() error { return c.stream.Close() }

// CloseConnection closes the underlying QUIC connection (all streams).
func (c *Conn) CloseConnection() error {
	return c.qconn.CloseWithError(0, "closed")
}

func (c *Conn) LocalAddr() net.Addr  { return c.qconn.LocalAddr() }
func (c *Conn) RemoteAddr() net.Addr { return c.qconn.RemoteAddr() }

func (c *Conn) SetDeadline(t time.Time) error      { return c.stream.SetDeadline(t) }
func (c *Conn) SetReadDeadline(t time.Time) error  { return c.stream.SetReadDeadline(t) }
func (c *Conn) SetWriteDeadline(t time.Time) error { return c.stream.SetWriteDeadline(t) }

// OpenStream opens a new bidirectional stream on qconn and wraps it.
func OpenStream(qconn *quic.Conn) (*Conn, error) {
	stream, err := qconn.OpenStreamSync(qconn.Context())
	if err != nil {
		return nil, err
	}
	return Wrap(qconn, stream), nil
}

// AcceptStream blocks until the peer opens a stream on qconn, then wraps it.
func AcceptStream(qconn *quic.Conn) (*Conn, error) {
	stream, err := qconn.AcceptStream(qconn.Context())
	if err != nil {
		return nil, err
	}
	return Wrap(qconn, stream), nil
}
