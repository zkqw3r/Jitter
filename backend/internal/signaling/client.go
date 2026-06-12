package signaling

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 54 * time.Second
	maxMessageSize = 64 * 1024
	sendBuffer     = 64
)

type Client struct {
	send      chan []byte
	ws        *websocket.Conn
	h         *Hub
	roomID    string
	writeMu   sync.Mutex
	closeOnce sync.Once
}

func NewClient(conn *websocket.Conn, hub *Hub, roomID string) *Client {
	return &Client{
		send:   make(chan []byte, sendBuffer),
		ws:     conn,
		h:      hub,
		roomID: roomID,
	}
}

func (c *Client) writeMessage(messageType int, data []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = c.ws.SetWriteDeadline(time.Now().Add(writeWait))
	return c.ws.WriteMessage(messageType, data)
}

func (c *Client) close(code int, reason string) {
	c.closeOnce.Do(func() {
		_ = c.writeMessage(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason))
		_ = c.ws.Close()
	})
}

func (c *Client) ReadPump() {
	defer func() {
		c.h.Leave(c.roomID, c)
		c.close(websocket.CloseNormalClosure, "")
	}()

	c.ws.SetReadLimit(maxMessageSize)
	_ = c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error {
		_ = c.ws.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		msgType, msg, err := c.ws.ReadMessage()
		if err != nil {
			return
		}
		if msgType != websocket.TextMessage {
			continue
		}
		c.h.Broadcast(c.roomID, c, msg)
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.close(websocket.CloseNormalClosure, "")
	}()

	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				return
			}
			if err := c.writeMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.writeMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
