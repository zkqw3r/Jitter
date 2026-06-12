package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"log"
	"net/http"
	"os/signal"
	"strconv"
	"sync"
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
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("pgxpool: %v", err)
	}
	defer pool.Close()

	pingCtx, cancelPing := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelPing()
	if err := pool.Ping(pingCtx); err != nil {
		log.Fatalf("cannot connect to db: %v", err)
	}
	queries := db.New(pool)

	hub := signaling.NewHub()
	upgrader := handler.NewUpgrader(cfg.AllowedOrigins)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		SkipPaths: []string{"/healthz"},
	}))
	if err := r.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		log.Fatalf("trusted proxies: %v", err)
	}
	r.MaxMultipartMemory = 1 << 20
	r.Use(securityHeadersMiddleware())

	r.StaticFile("/", cfg.FrontendDir+"/index.html")
	r.StaticFile("/favicon.ico", cfg.FrontendDir+"/icons/favicon.ico")
	r.StaticFile("/call.html", cfg.FrontendDir+"/call.html")
	r.StaticFile("/style.css", cfg.FrontendDir+"/style.css")
	r.StaticFile("/app.js", cfg.FrontendDir+"/app.js")
	r.StaticFile("/manifest.json", cfg.FrontendDir+"/manifest.json")
	r.StaticFile("/bg.png", cfg.FrontendDir+"/bg.png")
	r.Static("/icons", cfg.FrontendDir+"/icons")

	r.GET("/healthz", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	r.GET("/room/:roomID", func(c *gin.Context) {
		c.File(cfg.FrontendDir + "/call.html")
	})

	createLimiter := newRateLimiter(10, time.Minute)
	r.POST("/rooms", createLimiter.middleware(), handler.CreateRoomHandler(queries))
	r.GET("/rooms/:roomID", handler.GetRoomHandler(queries))

	r.GET("/ws/:roomID", handler.WSHandler(upgrader, hub, queries))
	r.GET("/api/ice-config", iceConfigHandler(cfg))

	r.NoRoute(func(c *gin.Context) {
		c.File(cfg.FrontendDir + "/404.html")
	})

	srv := &http.Server{
		Addr:              cfg.ServerAddr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 14,
	}

	go func() {
		log.Printf("listening on %s", cfg.ServerAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
	log.Println("server stopped")
}

func securityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "camera=(self), microphone=(self), geolocation=()")
		h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Next()
	}
}

func iceConfigHandler(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		iceServers := []gin.H{}
		if cfg.STUNURL != "" {
			iceServers = append(iceServers, gin.H{"urls": []string{cfg.STUNURL}})
		}

		turnURLs := []string{}
		for _, u := range []string{cfg.TURNURLUDP, cfg.TURNURLTCP, cfg.TURNURLTLS} {
			if u != "" {
				turnURLs = append(turnURLs, u)
			}
		}
		if len(turnURLs) > 0 {
			user, cred := turnCredentials(cfg)
			if user != "" && cred != "" {
				iceServers = append(iceServers, gin.H{
					"urls":       turnURLs,
					"username":   user,
					"credential": cred,
				})
			}
		}

		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, gin.H{"iceServers": iceServers})
	}
}

func turnCredentials(cfg *config.Config) (user, cred string) {
	if cfg.TURNSecret != "" {
		expiry := time.Now().Add(time.Duration(cfg.TURNTTL) * time.Second).Unix()
		username := strconv.FormatInt(expiry, 10) + ":" + cfg.TURNUsername
		mac := hmac.New(sha1.New, []byte(cfg.TURNSecret))
		_, _ = mac.Write([]byte(username))
		return username, base64.StdEncoding.EncodeToString(mac.Sum(nil))
	}
	return cfg.TURNUsername, cfg.TURNCredential
}

type rateLimiter struct {
	mu      sync.Mutex
	hits    map[string][]time.Time
	limit   int
	window  time.Duration
	cleanAt time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		hits:   make(map[string][]time.Time),
		limit:  limit,
		window: window,
	}
}

func (l *rateLimiter) allow(ip string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	if now.After(l.cleanAt) {
		for k, ts := range l.hits {
			filtered := ts[:0]
			for _, t := range ts {
				if now.Sub(t) < l.window {
					filtered = append(filtered, t)
				}
			}
			if len(filtered) == 0 {
				delete(l.hits, k)
			} else {
				l.hits[k] = filtered
			}
		}
		l.cleanAt = now.Add(l.window)
	}

	ts := l.hits[ip]
	cutoff := now.Add(-l.window)
	filtered := ts[:0]
	for _, t := range ts {
		if t.After(cutoff) {
			filtered = append(filtered, t)
		}
	}
	if len(filtered) >= l.limit {
		l.hits[ip] = filtered
		return false
	}
	filtered = append(filtered, now)
	l.hits[ip] = filtered
	return true
}

func (l *rateLimiter) middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !l.allow(c.ClientIP()) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}
