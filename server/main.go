package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg := LoadConfig()
	logger := NewLogger(cfg.Env)
	ctx := context.Background()

	pool, err := ConnectDB(ctx, cfg)
	if err != nil {
		logger.Fatal("db", zapErr(err))
	}
	defer pool.Close()

	if err := Migrate(ctx, pool, logger); err != nil {
		logger.Fatal("migrate", zapErr(err))
	}
	if err := Seed(ctx, pool, cfg, logger); err != nil {
		logger.Fatal("seed", zapErr(err))
	}

	cache := NewCache(ctx, cfg, logger)

	app := NewApp(cfg, pool, cache, logger)

	// Graceful shutdown: Railway mengirim SIGTERM saat restart/deploy.
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		logger.Info("shutdown...")
		sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = app.ShutdownWithContext(sctx)
	}()

	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
