package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

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
	r.StaticFile("/favicon.ico", cfg.FrontendDir+"/icons/favicon.ico")
	r.StaticFile("/call.html", cfg.FrontendDir+"/call.html")
	r.StaticFile("/style.css", cfg.FrontendDir+"/style.css")
	r.StaticFile("/app.js", cfg.FrontendDir+"/app.js")
	r.StaticFile("/manifest.json", cfg.FrontendDir+"/manifest.json")
	r.Static("/icons", cfg.FrontendDir+"/icons")
	r.StaticFile("/lucide.min.js", cfg.FrontendDir+"/lucide.min.js")

	r.GET("/room/:roomID", func(c *gin.Context) {
		c.File(cfg.FrontendDir + "/call.html")
	})
	r.GET("/ws/:roomID", handler.WSHandler(hub, queries))
	r.POST("/rooms", handler.CreateRoomHandler(queries))
	r.GET("/rooms/:roomID", handler.GetRoomHandler(queries))
	r.GET("/api/ice-config", func(c *gin.Context) {
		iceServers := []gin.H{
			{
				"urls": []string{cfg.STUN_URL},
			},
		}

		if cfg.TURN_URL_UDP != "" || cfg.TURN_URL_TCP != "" || cfg.TURN_URL_TLS != "" {
			turnURLs := []string{}
			if cfg.TURN_URL_UDP != "" {
				turnURLs = append(turnURLs, cfg.TURN_URL_UDP)
			}
			if cfg.TURN_URL_TCP != "" {
				turnURLs = append(turnURLs, cfg.TURN_URL_TCP)
			}
			if cfg.TURN_URL_TLS != "" {
				turnURLs = append(turnURLs, cfg.TURN_URL_TLS)
			}

			iceServers = append(iceServers, gin.H{
				"urls":       turnURLs,
				"username":   cfg.TURNUsername,
				"credential": cfg.TURNCredential,
			})
		}

		c.JSON(200, gin.H{
			"iceServers": iceServers,
		})
	})

	r.NoRoute(func(c *gin.Context) {
		c.File(cfg.FrontendDir + "/404.html")
	})

	srv := &http.Server{
		Addr:    cfg.ServerAddr,
		Handler: r,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Println("shutdown error:", err)
	}
	log.Println("\nserver stopped")

}
