package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/zkqw3r/Jitter/internal/db/sqlc"
	"github.com/zkqw3r/Jitter/internal/signaling"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func WSHandler(hub *signaling.Hub, queries *db.Queries) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var uuid pgtype.UUID
		roomID := ctx.Param("roomID")
		err := uuid.Scan(roomID)
		if err != nil {
			ctx.AbortWithStatus(http.StatusBadRequest)
			return
		}
		_, err = queries.GetRoom(ctx.Request.Context(), uuid)
		if err != nil {
			ctx.AbortWithStatus(http.StatusNotFound)
			return
		}

		conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
		if err != nil {
			return
		}
		client := signaling.NewClient(conn, hub, roomID)

		if hub.Count(roomID) >= 2 {
			conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(4000, "room is full"))
			conn.Close()
			return
		}

		hub.Join(roomID, client)
		hub.Broadcast(roomID, client, []byte(`{"type":"peer-joined"}`))
		go client.WritePump()
		client.ReadPump()
		time.AfterFunc(5*time.Minute, func() {
			if hub.Count(roomID) == 0 {
				queries.DeleteRoom(context.Background(), uuid)
			}
		})
	}
}
