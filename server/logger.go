package main

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Aturan #4 master plan: logging terstruktur performa tinggi (uber-go/zap),
// bukan log bawaan. Production = JSON, dev = console.
func NewLogger(env string) *zap.Logger {
	var l *zap.Logger
	if env == "production" {
		l, _ = zap.NewProduction()
	} else {
		l, _ = zap.NewDevelopment()
	}
	return l
}

func zapErr(err error) zapcore.Field { return zap.Error(err) }
