package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/zkqw3r/Jitter/internal/signaling"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func WSHandler(hub *signaling.Hub) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		roomID := ctx.Param("roomID")
		conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
		if err != nil {
			return
		}
		client := signaling.NewClient(conn, hub, roomID)
		hub.Join(roomID, client)
		go client.WritePump()
		client.ReadPump()
	}
}
