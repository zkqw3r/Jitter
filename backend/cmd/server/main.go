package main

import (
	"context"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/zkqw3r/Jitter/internal/config"
	db "github.com/zkqw3r/Jitter/internal/db/sqlc"
	"github.com/zkqw3r/Jitter/internal/handler"
	"github.com/zkqw3r/Jitter/internal/signaling"
)

func main() {
	_ = godotenv.Load()
	cfg := config.Load()
	log.Println("Frontend dir:", cfg.FrontendDir)

	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Ping(context.Background()); err != nil {
		log.Fatal("cannot connect to db:", err)
	}
	queries := db.New(pool)

	hub := signaling.NewHub()

	r := gin.Default()

	r.StaticFile("/", cfg.FrontendDir+"/index.html")
	r.StaticFile("/favicon.ico", cfg.FrontendDir+"/favicon.ico")
	r.StaticFile("/call.html", cfg.FrontendDir+"/call.html")
	r.StaticFile("/app.js", cfg.FrontendDir+"/app.js")

	r.GET("/ws/:roomID", handler.WSHandler(hub))
	r.POST("/rooms", handler.CreateRoomHandler(queries))
	r.GET("/rooms/:roomID", handler.GetRoomHandler(queries))

	log.Fatal(r.Run(":8080"))
}
