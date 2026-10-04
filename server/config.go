package main

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Port        string
	Env         string
	DatabaseURL string // postgres://user:pass@host:port/db
	RedisURL    string // redis://host:port (kosong = cache in-memory)
	Secret      string // kunci HMAC token
	UploadDir   string
	MaxUploadMB int64
	BootstrapPw string // password admin awal
	// AI untuk generator kuis (OpenAI-compatible). Kosong = mode Tempel JSON saja.
	AIBaseURL   string
	AIAPIKey    string
	AIModel     string
	AITimeoutSec int
}

func LoadConfig() Config {
	c := Config{
		Port:        getenv("PORT", "8081"),
		Env:         getenv("APP_ENV", "dev"),
		DatabaseURL: getenv("DATABASE_URL", "postgres://postgres@127.0.0.1:5433/elearning?sslmode=disable"),
		RedisURL:    os.Getenv("REDIS_URL"),
		Secret:      getenv("SECRET_KEY", "dev-secret-change-me"),
		UploadDir:   getenv("UPLOAD_DIR", "./uploads"),
		MaxUploadMB: 20,
		BootstrapPw: getenv("ADMIN_PASSWORD", "admin123"),
		AIBaseURL:   strings.TrimSpace(os.Getenv("AI_BASE_URL")),
		AIAPIKey:    strings.TrimSpace(os.Getenv("AI_API_KEY")),
		AIModel:     strings.TrimSpace(os.Getenv("AI_MODEL")),
		AITimeoutSec: atoiEnv("AI_TIMEOUT_SEC", 120),
	}
	// Railway memberi DATABASE_URL dgn sslmode=require; pgx paham apa adanya.
	c.DatabaseURL = strings.TrimSpace(c.DatabaseURL)
	return c
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func atoiEnv(k string, def int) int {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil || n <= 0 {
		return def
	}
	return n
}
