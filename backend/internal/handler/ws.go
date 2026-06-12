package handler

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/zkqw3r/Jitter/internal/db/sqlc"
	"github.com/zkqw3r/Jitter/internal/signaling"
)

func NewUpgrader(allowedOrigins []string) websocket.Upgrader {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = struct{}{}
	}
	return websocket.Upgrader{
		ReadBufferSize:  4 * 1024,
		WriteBufferSize: 4 * 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true
			}
			_, ok := allowed[origin]
			return ok
		},
	}
}

func WSHandler(upgrader websocket.Upgrader, hub *signaling.Hub, queries *db.Queries) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var roomUUID pgtype.UUID
		roomID := ctx.Param("roomID")
		if err := roomUUID.Scan(roomID); err != nil {
			ctx.AbortWithStatus(http.StatusBadRequest)
			return
		}

		lookupCtx, cancel := context.WithTimeout(ctx.Request.Context(), 3*time.Second)
		defer cancel()
		if _, err := queries.GetRoom(lookupCtx, roomUUID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				ctx.AbortWithStatus(http.StatusNotFound)
				return
			}
			log.Printf("ws: GetRoom failed: %v", err)
			ctx.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
		if err != nil {
			return
		}
		client := signaling.NewClient(conn, hub, roomID)

		err = hub.Join(roomID, client, func() {
			delCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := queries.DeleteRoom(delCtx, roomUUID); err != nil {
				log.Printf("ws: DeleteRoom(%s) failed: %v", roomID, err)
			}
		})
		if err != nil {
			_ = conn.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(4000, "room is full"),
				time.Now().Add(2*time.Second),
			)
			_ = conn.Close()
			return
		}

		go client.WritePump()
		hub.BroadcastPeerJoined(roomID, client)
		client.ReadPump()
	}
}
