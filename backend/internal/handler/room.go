package handler

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/zkqw3r/Jitter/internal/db/sqlc"
)

type CreateRoomResponse struct {
	RoomID string `json:"room_id"`
}

type GetRoomResponse struct {
	RoomID    string `json:"room_id"`
	CreatedAt string `json:"created_at"`
}

func CreateRoomHandler(queries *db.Queries) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		reqCtx, cancel := context.WithTimeout(ctx.Request.Context(), 3*time.Second)
		defer cancel()

		room, err := queries.CreateRoom(reqCtx)
		if err != nil {
			log.Printf("room: CreateRoom failed: %v", err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "can't create room"})
			return
		}
		ctx.JSON(http.StatusCreated, CreateRoomResponse{RoomID: room.ID.String()})
	}
}

func GetRoomHandler(queries *db.Queries) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var uuid pgtype.UUID
		roomID := ctx.Param("roomID")
		if err := uuid.Scan(roomID); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid room ID format"})
			return
		}

		reqCtx, cancel := context.WithTimeout(ctx.Request.Context(), 3*time.Second)
		defer cancel()

		room, err := queries.GetRoom(reqCtx, uuid)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				ctx.JSON(http.StatusNotFound, gin.H{"error": "room not found"})
				return
			}
			log.Printf("room: GetRoom(%s) failed: %v", roomID, err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		ctx.JSON(http.StatusOK, GetRoomResponse{
			RoomID:    room.ID.String(),
			CreatedAt: room.CreatedAt.Time.Format(time.RFC3339),
		})
	}
}
