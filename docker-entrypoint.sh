#!/bin/sh
set -e

# Go backend di port internal tetap 8081; Next di depan. Railway menyediakan
# $PORT utk container web (default 8080). NEXT_PORT menyesuaikan.
WEB_PORT="${PORT:-3000}"

# Go (API + uploads) di background, binding localhost saja (tak terekspose).
DATABASE_URL="${DATABASE_URL}" REDIS_URL="${REDIS_URL:-}" \
  SECRET_KEY="${SECRET_KEY:-}" PORT=8081 \
  elearning &

# Verifikasi backend naik sebelum Next proxy aktif (pakai node fetch, curl tak ada di alpine)
i=0
until node -e "fetch('http://127.0.0.1:8081/api/health').then(r=>process.exit(r.ok?0:1)).catch(()=>process.exit(1))" 2>/dev/null; do
  i=$((i+1)); [ "$i" -gt 40 ] && echo "backend gagal start" && exit 1
  sleep 1
done

# Next.js di depan (proxy /api & /uploads ke Go via rewrites)
exec node node_modules/next/dist/bin/next start -p "$WEB_PORT"