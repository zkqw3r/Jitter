package signaling

import (
	"sync"
	"time"
)

type Hub struct {
	mu     sync.RWMutex
	rooms  map[string]map[*Client]struct{}
	timers map[string]*time.Timer
}

func NewHub() *Hub {
	return &Hub{
		rooms:  make(map[string]map[*Client]struct{}),
		timers: make(map[string]*time.Timer),
	}
}

func (h *Hub) Join(roomID string, c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.rooms[roomID] == nil {
		h.rooms[roomID] = make(map[*Client]struct{})
	}
	h.rooms[roomID][c] = struct{}{}

	switch len(h.rooms[roomID]) {
	case 1:
		h.startTimer(roomID)
	case 2:
		if t := h.timers[roomID]; t != nil {
			t.Stop()
		}
	}
}

func (h *Hub) Leave(roomID string, c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	clients, ok := h.rooms[roomID]
	if !ok {
		return
	}
	delete(clients, c)

	switch len(clients) {
	case 0:
		delete(h.rooms, roomID)
		if t := h.timers[roomID]; t != nil {
			t.Stop()
		}
		delete(h.timers, roomID)
	case 1:
		h.startTimer(roomID)
	}
}

func (h *Hub) Broadcast(roomID string, sender *Client, msg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for c := range h.rooms[roomID] {
		if c != sender {
			c.send <- msg
		}
	}
}

func (h *Hub) Count(roomID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.rooms[roomID])
}

func (h *Hub) startTimer(roomID string) {
	if t := h.timers[roomID]; t != nil {
		t.Stop()
	}
	h.timers[roomID] = time.AfterFunc(5*time.Minute, func() {
		h.mu.RLock()
		defer h.mu.RUnlock()
		for c := range h.rooms[roomID] {
			c.send <- []byte(`{"type":"room-timeout"}`)
		}
	})
}
