# ---- Build backend Go ----
FROM golang:1.26-alpine AS gobuild
WORKDIR /app
COPY server/go.mod server/go.sum* ./
RUN go mod download
COPY server/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /elearning .

# ---- Build frontend Next.js ----
FROM node:22-alpine AS webuild
WORKDIR /web
COPY web/package.json web/package-lock.json* ./
RUN npm install --no-audit --no-fund
COPY web/ ./
ENV NEXT_TELEMETRY_DISABLED=1
RUN npm run build

# ---- Runtime: satu container, Next.js menyajikan SPA & proxy /api ke Go ----
FROM node:22-alpine
WORKDIR /srv

# Go binary
COPY --from=gobuild /elearning /usr/local/bin/elearning

# Frontend (standalone Next)
COPY --from=webuild /web/.next ./.next
COPY --from=webuild /web/public ./public
COPY --from=webuild /web/node_modules ./node_modules
COPY --from=webuild /web/package.json ./package.json
COPY --from=webuild /web/next.config.mjs ./next.config.mjs

# Upload dir (untuk file materi/tugas)
RUN mkdir -p /srv/uploads && chown -R node:node /srv

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
ENV BACKEND_URL=http://127.0.0.1:8080
ENV PORT=8080

# Jalankan keduanya: Go (8080) + Next (3000)
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

EXPOSE 3000
USER node
ENTRYPOINT ["/docker-entrypoint.sh"]