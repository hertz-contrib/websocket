// Copyright 2017 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// This file may have been modified by CloudWeGo authors. All CloudWeGo
// Modifications are Copyright 2022 CloudWeGo Authors.

package websocket

import (
	"bytes"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

const badHandshake = "websocket: the client is not using the websocket protocol: "

var strPermessageDeflate = []byte("permessage-deflate")

// HandshakeError describes an error with the handshake from the peer.
type HandshakeError struct {
	message string
}

func (e HandshakeError) Error() string { return e.message }

var poolWriteBuffer = sync.Pool{
	New: func() interface{} {
		var buf []byte
		return buf
	},
}

// HertzHandler receives a websocket connection after the handshake has been
// completed. This must be provided.
type HertzHandler func(*Conn)

// HertzUpgrader specifies parameters for upgrading an HTTP connection to a
// WebSocket connection.
type HertzUpgrader struct {
	// HandshakeTimeout specifies the duration for the handshake to complete.
	HandshakeTimeout time.Duration

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

	// Subprotocols specifies the server's supported protocols in order of
	// preference. If this field is not nil, then the Upgrade method negotiates a
	// subprotocol by selecting the first match in this list with a protocol
	// requested by the client. If there's no match, then no protocol is
	// negotiated (the Sec-Websocket-Protocol header is not included in the
	// handshake response).
	Subprotocols []string

	// Error specifies the function for generating HTTP error responses. If Error
	// is nil, then http.Error is used to generate the HTTP response.
	Error func(ctx *app.RequestContext, status int, reason error)

	// CheckOrigin returns true if the request Origin header is acceptable. If
	// CheckOrigin is nil, then a safe default is used: return false if the
	// Origin request header is present and the origin host is not equal to
	// request Host header.
	//
	// A CheckOrigin function should carefully validate the request origin to
	// prevent cross-site request forgery.
	CheckOrigin func(ctx *app.RequestContext) bool

	// EnableCompression specify if the server should attempt to negotiate per
	// message compression (RFC 7692). Setting this value to true does not
	// guarantee that compression will be supported. Currently only "no context
	// takeover" modes are supported.
	EnableCompression bool
}

func (u *HertzUpgrader) returnError(ctx *app.RequestContext, status int, reason string) error {
	err := HandshakeError{reason}
	if u.Error != nil {
		u.Error(ctx, status, err)
	} else {
		ctx.Response.Header.Set("Sec-Websocket-Version", "13")
		ctx.AbortWithMsg(consts.StatusMessage(status), status)
	}

	return err
}

func (u *HertzUpgrader) selectSubprotocol(ctx *app.RequestContext) []byte {
	if u.Subprotocols != nil {
		clientProtocols := parseDataHeader(ctx.Request.Header.Peek("Sec-Websocket-Protocol"))

		for _, serverProtocol := range u.Subprotocols {
			for _, clientProtocol := range clientProtocols {
				if b2s(clientProtocol) == serverProtocol {
					return clientProtocol
				}
			}
		}
	} else if ctx.Response.Header.Len() > 0 {
		return ctx.Response.Header.Peek("Sec-Websocket-Protocol")
	}

	return nil
}

func (u *HertzUpgrader) isCompressionEnable(ctx *app.RequestContext) bool {
	extensions := parseDataHeader(ctx.Request.Header.Peek("Sec-WebSocket-Extensions"))

	// Negotiate PMCE
	if u.EnableCompression {
		for _, ext := range extensions {
			if bytes.HasPrefix(ext, strPermessageDeflate) {
				return true
			}
		}
	}

	return false
}

// Upgrade upgrades the HTTP server connection to the WebSocket protocol.
//
// The responseHeader is included in the response to the client's upgrade
// request. Use the responseHeader to specify cookies (Set-Cookie) and the
// application negotiated subprotocol (Sec-WebSocket-Protocol).
//
// If the upgrade fails, then Upgrade replies to the client with an HTTP error
// response.
func (u *HertzUpgrader) Upgrade(ctx *app.RequestContext, handler HertzHandler) error {
	if !ctx.IsGet() {
		return u.returnError(ctx, consts.StatusMethodNotAllowed, fmt.Sprintf("%s request method is not GET", badHandshake))
	}

	if !tokenContainsValue(b2s(ctx.Request.Header.Peek("Connection")), "Upgrade") {
		return u.returnError(ctx, consts.StatusBadRequest, fmt.Sprintf("%s 'upgrade' token not found in 'Connection' header", badHandshake))
	}

	if !tokenContainsValue(b2s(ctx.Request.Header.Peek("Upgrade")), "Websocket") {
		return u.returnError(ctx, consts.StatusBadRequest, fmt.Sprintf("%s 'websocket' token not found in 'Upgrade' header", badHandshake))
	}

	if !tokenContainsValue(b2s(ctx.Request.Header.Peek("Sec-Websocket-Version")), "13") {
		return u.returnError(ctx, consts.StatusBadRequest, "websocket: unsupported version: 13 not found in 'Sec-Websocket-Version' header")
	}

	if len(ctx.Response.Header.Peek("Sec-Websocket-Extensions")) > 0 {
		return u.returnError(ctx, consts.StatusInternalServerError, "websocket: application specific 'Sec-WebSocket-Extensions' headers are unsupported")
	}

	checkOrigin := u.CheckOrigin
	if checkOrigin == nil {
		checkOrigin = fastHTTPCheckSameOrigin
	}
	if !checkOrigin(ctx) {
		return u.returnError(ctx, consts.StatusForbidden, "websocket: request origin not allowed by HertzUpgrader.CheckOrigin")
	}

	challengeKey := ctx.Request.Header.Peek("Sec-Websocket-Key")
	if len(challengeKey) == 0 {
		return u.returnError(ctx, consts.StatusBadRequest, "websocket: not a websocket handshake: `Sec-WebSocket-Key' header is missing or blank")
	}

	subprotocol := u.selectSubprotocol(ctx)
	compress := u.isCompressionEnable(ctx)

	ctx.SetStatusCode(consts.StatusSwitchingProtocols)
	ctx.Response.Header.Set("Upgrade", "websocket")
	ctx.Response.Header.Set("Connection", "Upgrade")
	ctx.Response.Header.Set("Sec-WebSocket-Accept", computeAcceptKeyBytes(challengeKey))
	if compress {
		ctx.Response.Header.Set("Sec-WebSocket-Extensions", "permessage-deflate; server_no_context_takeover; client_no_context_takeover")
	}
	if subprotocol != nil {
		ctx.Response.Header.SetBytesV("Sec-WebSocket-Protocol", subprotocol)
	}

	ctx.Hijack(func(netConn network.Conn) {
		writeBuf := poolWriteBuffer.Get().([]byte)
		c := newConn(netConn, true, u.ReadBufferSize, u.WriteBufferSize, u.WriteBufferPool, nil, writeBuf)
		if subprotocol != nil {
			c.subprotocol = b2s(subprotocol)
		}

		if compress {
			c.newCompressionWriter = compressNoContextTakeover
			c.newDecompressionReader = decompressNoContextTakeover
		}

		// Clear deadlines set by HTTP server.
		netConn.SetDeadline(time.Time{})

		handler(c)

		writeBuf = writeBuf[0:0]
		poolWriteBuffer.Put(writeBuf)
	})

	return nil
}

// fastHTTPCheckSameOrigin returns true if the origin is not set or is equal to the request host.
func fastHTTPCheckSameOrigin(ctx *app.RequestContext) bool {
	origin := ctx.Request.Header.Peek("Origin")
	if len(origin) == 0 {
		return true
	}
	u, err := url.Parse(b2s(origin))
	if err != nil {
		return false
	}
	return equalASCIIFold(u.Host, b2s(ctx.Host()))
}
