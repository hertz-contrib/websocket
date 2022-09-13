// Copyright 2017 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// This file may have been modified by CloudWeGo authors. All CloudWeGo
// Modifications are Copyright 2022 CloudWeGo Authors.

package main

import (
	"context"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/hertz-contrib/websocket"
)

var upgrader = websocket.HertzUpgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var addr = ":8080"

func serveHome(_ context.Context, c *app.RequestContext) {
	if string(c.URI().Path()) != "/" {
		hlog.Error("Not found", http.StatusNotFound)
		return
	}
	if !c.IsGet() {
		hlog.Error("Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	c.HTML(http.StatusOK, "home.html", nil)
}

func main() {
	hub := newHub()
	go hub.run()
	// server.Default() creates a Hertz with recovery middleware.
	// If you need a pure hertz, you can use server.New()
	h := server.Default(server.WithHostPorts(addr))
	h.LoadHTMLGlob("home.html")

	h.GET("/", serveHome)
	h.GET("/ws", func(c context.Context, ctx *app.RequestContext) {
		serveWs(ctx, hub)
	})

	h.Spin()
}
