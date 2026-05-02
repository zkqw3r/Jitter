package config

import (
	"log"
	"os"
)

type Config struct {
	DatabaseURL    string
	FrontendDir    string
	ServerAddr     string
	TURNUsername   string
	TURNCredential string
	TURN_URL_UDP   string
	TURN_URL_TCP   string
	TURN_URL_TLS   string
	STUN_URL       string
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
	turnURLUDP := os.Getenv("TURN_URL_UDP")
	turnURLTCP := os.Getenv("TURN_URL_TCP")
	turnURLTLS := os.Getenv("TURN_URL_TLS")
	stunURL := os.Getenv("STUN_URL")

	return &Config{
		DatabaseURL:    dbURL,
		FrontendDir:    frontendDir,
		ServerAddr:     addr,
		TURNUsername:   os.Getenv("TURN_USERNAME"),
		TURNCredential: os.Getenv("TURN_CREDENTIAL"),
		TURN_URL_UDP:   turnURLUDP,
		TURN_URL_TCP:   turnURLTCP,
		TURN_URL_TLS:   turnURLTLS,
		STUN_URL:       stunURL,
	}
}
