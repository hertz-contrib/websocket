package websocket

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/cloudwego/hertz/pkg/protocol"
)

// ErrBadHandshake is returned when the server response to opening handshake is
// invalid.
var ErrBadHandshake = errors.New("websocket: bad handshake")

// ClientUpgrader is a helper for upgrading hertz http response to websocket conn.
// See ExampleClient for usage
type ClientUpgrader struct {
	// ReadBufferSize and WriteBufferSize specify I/O buffer sizes in bytes. If a buffer
	// size is zero, then buffers allocated by the HTTP server are used. The
	// I/O buffer sizes do not limit the size of the messages that can be sent
	// or received.
	ReadBufferSize, WriteBufferSize int

	// WriteBufferPool is a pool of buffers for write operations. If the value
	// is not set, then write buffers are allocated to the connection for the
	// lifetime of the connection.
	//
	// A pool is most useful when the application has a modest volume of writes
	// across a large number of connections.
	//
	// Applications should use a single pool for each unique value of
	// WriteBufferSize.
	WriteBufferPool BufferPool

	// EnableCompression specify if the server should attempt to negotiate per
	// message compression (RFC 7692). Setting this value to true does not
	// guarantee that compression will be supported. Currently only "no context
	// takeover" modes are supported.
	EnableCompression bool
}

// PrepareRequest prepares request for websocket
//
// It adds headers for websocket,
// and it must be called BEFORE sending http request via cli.DoXXX
func (p *ClientUpgrader) PrepareRequest(req *protocol.Request) {
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", generateChallengeKey())
	if p.EnableCompression {
		req.Header.Set("Sec-WebSocket-Extensions", "permessage-deflate; server_no_context_takeover; client_no_context_takeover")
	}
}

// UpgradeResponse upgrades a response to websocket conn
//
// It returns Conn if success. ErrBadHandshake is returned if headers go wrong.
// This method must be called after PrepareRequest and (*.Client).DoXXX
func (p *ClientUpgrader) UpgradeResponse(req *protocol.Request, resp *protocol.Response) (*Conn, error) {
	if resp.StatusCode() != 101 ||
		!tokenContainsValue(resp.Header.Get("Upgrade"), "websocket") ||
		!tokenContainsValue(resp.Header.Get("Connection"), "Upgrade") ||
		resp.Header.Get("Sec-Websocket-Accept") != computeAcceptKeyBytes(req.Header.Peek("Sec-Websocket-Key")) {
		return nil, ErrBadHandshake
	}

	c, err := resp.Hijack()
	if err != nil {
		return nil, fmt.Errorf("Hijack response connection err: %w", err)
	}

	c.SetDeadline(time.Time{})
	conn := newConn(c, false, p.ReadBufferSize, p.WriteBufferSize, p.WriteBufferPool, nil, nil)

	// can not use p.EnableCompression, always follow ext returned from server
	compress := false
	extensions := parseDataHeader(resp.Header.Peek("Sec-WebSocket-Extensions"))
	for _, ext := range extensions {
		if bytes.HasPrefix(ext, strPermessageDeflate) {
			compress = true
		}
	}
	if compress {
		conn.newCompressionWriter = compressNoContextTakeover
		conn.newDecompressionReader = decompressNoContextTakeover
	}
	return conn, nil
}
