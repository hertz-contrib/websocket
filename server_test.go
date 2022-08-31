// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package websocket

import (
	"bufio"
	"bytes"
	"net"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

var subprotocolTests = []struct {
	h         string
	protocols []string
}{
	{"", nil},
	{"foo", []string{"foo"}},
	{"foo,bar", []string{"foo", "bar"}},
	{"foo, bar", []string{"foo", "bar"}},
	{" foo, bar", []string{"foo", "bar"}},
	{" foo, bar ", []string{"foo", "bar"}},
}

func TestSubprotocols(t *testing.T) {
	for _, st := range subprotocolTests {
		r := protocol.NewRequest(consts.MethodGet, "", nil)
		r.Header.Set("Sec-Websocket-Protocol", st.h)
		protocols := Subprotocols(r)
		if !reflect.DeepEqual(st.protocols, protocols) {
			t.Errorf("SubProtocols(%q) returned %#v, want %#v", st.h, protocols, st.protocols)
		}
	}
}

type reuseTestResponseWriter struct {
	brw *bufio.ReadWriter
	http.ResponseWriter
}

func (resp *reuseTestResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return fakeNetConn{strings.NewReader(""), &bytes.Buffer{}}, resp.brw, nil
}

var bufioReuseTests = []struct {
	n     int
	reuse bool
}{
	{4096, true},
	{128, false},
}
