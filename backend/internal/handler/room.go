package handler

import (
	"time"

	"github.com/gin-gonic/gin"
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
		room, err := queries.CreateRoom(ctx.Request.Context())
		if err != nil {
			ctx.JSON(500, gin.H{"error": "cant create room"})
			return
		}
		ctx.JSON(201, CreateRoomResponse{RoomID: room.ID.String()})
	}
}

func GetRoomHandler(queries *db.Queries) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var uuid pgtype.UUID
		roomID := ctx.Param("roomID")
		err := uuid.Scan(roomID)
		if err != nil {
			ctx.JSON(400, gin.H{"error": "invalid room ID format"})
			return
		}
		room, err := queries.GetRoom(ctx, uuid)
		if err != nil {
			ctx.JSON(404, gin.H{"error": "room not found"})
			return
		}
		ctx.JSON(200, GetRoomResponse{RoomID: room.ID.String(), CreatedAt: room.CreatedAt.Time.Format(time.RFC3339)})
	}
}
