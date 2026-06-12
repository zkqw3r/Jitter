package signaling

import (
	"errors"
	"log"
	"sync"
	"time"
)

const idleRoomTTL = 5 * time.Minute

var (
	peerJoinedMsg = []byte(`{"type":"peer-joined"}`)
	peerLeftMsg   = []byte(`{"type":"peer-left"}`)
	roomTimeout   = []byte(`{"type":"room-timeout"}`)
)

type Hub struct {
	mu        sync.Mutex
	rooms     map[string]map[*Client]struct{}
	timers    map[string]chan struct{}
	callbacks map[string]func()
}

func NewHub() *Hub {
	return &Hub{
		rooms:     make(map[string]map[*Client]struct{}),
		timers:    make(map[string]chan struct{}),
		callbacks: make(map[string]func()),
	}
}

func (h *Hub) stopTimer(roomID string) {
	if done, ok := h.timers[roomID]; ok {
		close(done)
		delete(h.timers, roomID)
	}
}

func (h *Hub) Join(roomID string, c *Client, fn func()) error {
	h.mu.Lock()

	if len(h.rooms[roomID]) >= 2 {
		h.mu.Unlock()
		return errors.New("room is full")
	}

	if h.rooms[roomID] == nil {
		h.rooms[roomID] = make(map[*Client]struct{})
	}
	h.rooms[roomID][c] = struct{}{}

	switch len(h.rooms[roomID]) {
	case 1:
		if _, exists := h.callbacks[roomID]; !exists {
			h.callbacks[roomID] = fn
		}
		h.startTimer(roomID)
	case 2:
		h.stopTimer(roomID)
	}
	h.mu.Unlock()
	return nil
}

func (h *Hub) Leave(roomID string, c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	clients, ok := h.rooms[roomID]
	if !ok {
		return
	}
	if _, present := clients[c]; !present {
		return
	}
	delete(clients, c)
	close(c.send)

	var cb func()
	var remaining []*Client
	switch len(clients) {
	case 0:
		delete(h.rooms, roomID)
		h.stopTimer(roomID)
		cb = h.callbacks[roomID]
		delete(h.callbacks, roomID)
	case 1:
		for other := range clients {
			remaining = append(remaining, other)
		}
		h.startTimer(roomID)
	}
	h.mu.Unlock()

	for _, other := range remaining {
		trySend(other, peerLeftMsg)
	}
	if cb != nil {
		safeCallback(cb)
	}
}

func (h *Hub) Broadcast(roomID string, sender *Client, msg []byte) {
	h.mu.Lock()
	targets := make([]*Client, 0, len(h.rooms[roomID]))
	for c := range h.rooms[roomID] {
		if c != sender {
			targets = append(targets, c)
		}
	}
	h.mu.Unlock()

	for _, c := range targets {
		trySend(c, msg)
	}
}

func (h *Hub) BroadcastPeerJoined(roomID string, newcomer *Client) {
	h.Broadcast(roomID, newcomer, peerJoinedMsg)
}

func (h *Hub) Count(roomID string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.rooms[roomID])
}

func (h *Hub) startTimer(roomID string) {
	h.stopTimer(roomID)

	done := make(chan struct{})
	h.timers[roomID] = done

	go func() {
		timer := time.NewTimer(idleRoomTTL)
		defer timer.Stop()

		select {
		case <-timer.C:
		case <-done:
			return
		}

		h.mu.Lock()
		if h.timers[roomID] != done {
			h.mu.Unlock()
			return
		}
		delete(h.timers, roomID)

		clients := h.rooms[roomID]
		delete(h.rooms, roomID)

		cb := h.callbacks[roomID]
		delete(h.callbacks, roomID)
		h.mu.Unlock()

		for c := range clients {
			trySend(c, roomTimeout)
			_ = c.ws.Close()
		}
		if cb != nil {
			safeCallback(cb)
		}
	}()
}

func trySend(c *Client, msg []byte) {
	defer func() {
		_ = recover()
	}()
	select {
	case c.send <- msg:
	default:
		log.Printf("signaling: dropping message, slow client %p", c)
	}
}

func safeCallback(cb func()) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("signaling: callback panic: %v", r)
		}
	}()
	cb()
}
