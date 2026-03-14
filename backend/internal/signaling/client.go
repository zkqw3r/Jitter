package signaling

import "github.com/gorilla/websocket"

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
	for {
		_, msg, err := c.ws.ReadMessage()
		if err != nil {
			break
		}
		c.h.Broadcast(c.roomID, c, msg)
	}
}

func (c *Client) WritePump() {
	for msg := range c.send {
		err := c.ws.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			return
		}
	}
	c.ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseAbnormalClosure, ""))
}
