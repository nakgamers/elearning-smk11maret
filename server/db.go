package main

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Aturan #1 master plan: pool dibatasi sesuai resource Railway.
// 700 siswa simultan → 100 koneksi sudah jauh di atas kebutuhan (tiap request
// hanya memegang koneksi sepersekian detik).
func ConnectDB(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	pc, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	pc.MaxConns = 100
	pc.MinConns = 5
	pc.MaxConnLifetime = time.Hour
	pc.MaxConnIdleTime = 10 * time.Minute
	pc.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, pc)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}
