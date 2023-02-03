// Copyright 2017 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// This file may have been modified by CloudWeGo authors. All CloudWeGo
// Modifications are Copyright 2022 CloudWeGo Authors.

package main

import (
	"sync"
)

// Hub maintains the set of active clients and broadcasts messages to the
// clients.
type Hub struct {
	// Registered clients.
	clients     map[*Client]struct{}
	clientsLock sync.RWMutex

	// Inbound messages from the clients.
	broadcast chan []byte

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client
}

var hub = newHub()

func newHub() *Hub {
	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]struct{}),
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.Register(client)
		case client := <-h.unregister:
			h.Unregister(client)
		case message := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}

func (h *Hub) Register(client *Client) {
	h.AddClient(client)
}

func (h *Hub) AddClient(client *Client) {
	h.clientsLock.Lock()
	defer h.clientsLock.Unlock()
	h.clients[client] = struct{}{}
}

func (h *Hub) Unregister(client *Client) {
	h.DelClient(client)
}

func (h *Hub) DelClient(client *Client) {
	h.clientsLock.Lock()
	defer h.clientsLock.Unlock()
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		close(client.send)
	}
}
