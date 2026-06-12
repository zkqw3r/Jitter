package config

import (
	"errors"
	"os"
	"strings"
)

type Config struct {
	DatabaseURL    string
	FrontendDir    string
	ServerAddr     string
	AllowedOrigins []string
	TrustedProxies []string

	TURNUsername   string
	TURNCredential string
	TURNSecret     string
	TURNTTL        int

	TURNURLUDP string
	TURNURLTCP string
	TURNURLTLS string
	STUNURL    string
}

func Load() (*Config, error) {
	frontendDir := getEnv("FRONTEND_DIR", "./frontend")
	addr := getEnv("SERVER_ADDR", ":8080")

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, errors.New("DATABASE_URL is not set")
	}

	allowed := splitCSV(os.Getenv("ALLOWED_ORIGINS"))
	if len(allowed) == 0 {
		return nil, errors.New("ALLOWED_ORIGINS is not set (comma-separated list of https://host[:port])")
	}

	return &Config{
		DatabaseURL:    dbURL,
		FrontendDir:    frontendDir,
		ServerAddr:     addr,
		AllowedOrigins: allowed,
		TrustedProxies: splitCSV(os.Getenv("TRUSTED_PROXIES")),

		TURNUsername:   os.Getenv("TURN_USERNAME"),
		TURNCredential: os.Getenv("TURN_CREDENTIAL"),
		TURNSecret:     os.Getenv("TURN_SECRET"),
		TURNTTL:        atoiDefault(os.Getenv("TURN_TTL_SECONDS"), 3600),

		TURNURLUDP: os.Getenv("TURN_URL_UDP"),
		TURNURLTCP: os.Getenv("TURN_URL_TCP"),
		TURNURLTLS: os.Getenv("TURN_URL_TLS"),
		STUNURL:    os.Getenv("STUN_URL"),
	}, nil
}

func getEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func splitCSV(v string) []string {
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := parts[:0]
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func atoiDefault(v string, def int) int {
	if v == "" {
		return def
	}
	n := 0
	for _, r := range v {
		if r < '0' || r > '9' {
			return def
		}
		n = n*10 + int(r-'0')
	}
	if n == 0 {
		return def
	}
	return n
}
