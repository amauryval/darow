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
# pkg/auth is a nested module the main one reaches through a replace directive,
# so its go.mod has to exist before go mod download resolves anything.
COPY go.mod go.sum ./
COPY pkg/auth/go.mod pkg/auth/go.sum ./pkg/auth/
RUN go mod download
COPY . .
COPY --from=ui /app/internal/webui/dist ./internal/webui/dist
RUN go install github.com/swaggo/swag/cmd/swag@v1.16.6 \
    && swag init -g cmd/server/main.go -o docs
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /out/server ./cmd/server
RUN mkdir -p /out/data/sources

# distroless/cc ships glibc, libstdc++ and ca-certificates (duckdb-go links
# against libstdc++ via cgo; the spatial extension is fetched over HTTPS at
# first run) with no shell or package manager — a smaller, narrower runtime
# than debian-slim. The nonroot tag runs as uid/gid 65532 already.
FROM gcr.io/distroless/cc-debian12:nonroot
WORKDIR /app
COPY --from=build /out/server ./server
COPY config.yaml ./config.yaml
COPY --from=build --chown=nonroot:nonroot /out/data ./data

ENTRYPOINT ["./server"]
