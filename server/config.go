package main

import (
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
