#!/bin/sh
set -e

# Go backend di port internal tetap 8081; Next di depan. Railway menyediakan
# $PORT utk container web (default 8080). NEXT_PORT menyesuaikan.
WEB_PORT="${PORT:-3000}"

# Go (API + uploads) di background, binding localhost saja (tak terekspose).
DATABASE_URL="${DATABASE_URL}" REDIS_URL="${REDIS_URL:-}" \
  SECRET_KEY="${SECRET_KEY:-}" PORT=8081 \
  elearning &

# Verifikasi backend naik sebelum Next proxy aktif
i=0
until curl -sf http://127.0.0.1:8081/api/health >/dev/null 2>&1; do
  i=$((i+1)); [ "$i" -gt 30 ] && echo "backend gagal start" && exit 1
  sleep 1
done

# Next.js di depan (proxy /api & /uploads ke Go via rewrites)
exec node node_modules/next/dist/bin/next start -p "$WEB_PORT"