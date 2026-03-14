package signaling

import (
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 54 * time.Second
)

type Client struct {
	send   chan []byte
	ws     *websocket.Conn
	h      *Hub
	roomID string
}

func NewClient(conn *websocket.Conn, hub *Hub, roomID string) *Client {
	return &Client{
		send:   make(chan []byte, 256),
		ws:     conn,
		h:      hub,
		roomID: roomID,
	}
}

func (c *Client) ReadPump() {
	defer c.h.Leave(c.roomID, c)
	defer c.ws.Close()
	c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error {
		c.ws.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		_, msg, err := c.ws.ReadMessage()
		if err != nil {
			break
		}
		c.h.Broadcast(c.roomID, c, msg)
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-c.send:
			c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.ws.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			err := c.ws.WriteMessage(websocket.TextMessage, msg)
			if err != nil {
				return
			}
		case <-ticker.C:
			c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			err := c.ws.WriteMessage(websocket.PingMessage, nil)
			if err != nil {
				return
			}
		}
	}
}
