package config

import (
	"log"
	"os"
)

type Config struct {
	DatabaseURL string
	FrontendDir string
	ServerAddr  string
	TURNUsername   string
    TURNCredential string
}

func Load() *Config {
    frontendDir := os.Getenv("FRONTEND_DIR")
    if frontendDir == "" {
        frontendDir = "./frontend"
    }

    dbURL := os.Getenv("DATABASE_URL")
    if dbURL == "" {
        log.Fatal("DATABASE_URL is not set")
    }

    addr := os.Getenv("SERVER_ADDR")
    if addr == "" {
        addr = ":8080"
    }

    return &Config{
        DatabaseURL:    dbURL,
        FrontendDir:    frontendDir,
        ServerAddr:     addr,
        TURNUsername:   os.Getenv("TURN_USERNAME"),
        TURNCredential: os.Getenv("TURN_CREDENTIAL"),
    }
}