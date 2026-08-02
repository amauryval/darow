# syntax=docker/dockerfile:1

FROM node:22-bookworm AS ui
WORKDIR /app
COPY web/package.json web/package-lock.json ./web/
RUN cd web && npm ci
COPY web ./web
# Vite's outDir (../internal/webui/dist, see web/vite.config.ts) lands one
# level up from web/, so the build needs internal/webui to exist first.
COPY internal/webui ./internal/webui
RUN cd web && npm run build

FROM golang:1.26-bookworm AS build
# duckdb-go links against libstdc++ via cgo — build-essential provides the
# C++ toolchain needed at link time (règle 21 concerns the app's own write
# lock, not this: this is purely a build-time linker dependency).
RUN apt-get update && apt-get install -y --no-install-recommends build-essential \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=ui /app/internal/webui/dist ./internal/webui/dist
RUN go install github.com/swaggo/swag/cmd/swag@v1.16.6 \
    && swag init -g cmd/server/main.go -o docs
RUN CGO_ENABLED=1 go build -o /out/server ./cmd/server

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates libstdc++6 \
    && rm -rf /var/lib/apt/lists/* \
    && useradd -r -u 1000 darow
WORKDIR /app
COPY --from=build /out/server ./server
COPY config.yaml ./config.yaml
RUN mkdir -p /app/data/sources && chown -R darow:darow /app
USER darow
EXPOSE 8080
ENTRYPOINT ["./server"]
