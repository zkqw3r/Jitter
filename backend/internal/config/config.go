package config

import "os"

type Config struct {
	DatabaseURL string
	FrontendDir string
}

func Load() *Config {
	frontendDir := os.Getenv("FRONTEND_DIR")
	if frontendDir == "" {
		frontendDir = "./frontend"
	}
	return &Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		FrontendDir: frontendDir,
	}
}
