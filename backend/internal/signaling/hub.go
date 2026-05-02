package signaling

import (
	"errors"
	"sync"
	"time"
)

type Hub struct {
	mu        sync.RWMutex
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
	defer h.mu.Unlock()

	if len(h.rooms[roomID]) >= 2 {
		return errors.New("room is full")
	}

	if h.rooms[roomID] == nil {
		h.rooms[roomID] = make(map[*Client]struct{})
	}
	h.rooms[roomID][c] = struct{}{}

	switch len(h.rooms[roomID]) {
	case 1:
		h.callbacks[roomID] = fn
		h.startTimer(roomID)
	case 2:
		h.stopTimer(roomID)
	}
	return nil
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
		h.stopTimer(roomID)
		delete(h.callbacks, roomID)
	case 1:
		h.startTimer(roomID)
	}
}

func (h *Hub) Broadcast(roomID string, sender *Client, msg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for c := range h.rooms[roomID] {
		if c == sender {
			continue
		}
		select {
		case c.send <- msg:
		default:
		}
	}
}

func (h *Hub) Count(roomID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.rooms[roomID])
}

func (h *Hub) startTimer(roomID string) {
	h.stopTimer(roomID)

	done := make(chan struct{})
	h.timers[roomID] = done

	go func() {
		select {
		case <-time.After(5 * time.Minute):

		case <-done:
			return
		}

		h.mu.Lock()
		defer h.mu.Unlock()

		if h.timers[roomID] != done {
			return
		}

		for c := range h.rooms[roomID] {
			select {
			case c.send <- []byte(`{"type":"room-timeout"}`):
			default:
			}
		}

		delete(h.rooms, roomID)
		delete(h.timers, roomID)

		if cb, ok := h.callbacks[roomID]; ok {
			cb()
			delete(h.callbacks, roomID)
		}
	}()
}
