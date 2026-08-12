# syntax=docker/dockerfile:1

# Stage 1: Build UI (Node.js)
FROM node:22-bookworm AS ui
WORKDIR /app
COPY web/package.json web/package-lock.json ./web/
RUN cd web && npm ci
COPY web ./web
COPY internal/webui ./internal/webui
RUN cd web && npm run build

# Stage 2: Build Go (with CGO)
FROM golang:1.26-bookworm AS build
RUN apt-get update && apt-get install -y --no-install-recommends build-essential \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=ui /app/internal/webui/dist ./internal/webui/dist
RUN go install github.com/swaggo/swag/cmd/swag@v1.16.6 \
    && swag init -g cmd/server/main.go -o docs
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /out/server.run ./cmd/server

# Stage 3: final image (glibc compatible)
FROM gcr.io/distroless/cc-debian12:nonroot
WORKDIR /app
COPY --from=build /out/server.run ./server.run
RUN chmod +x ./server.run && mkdir -p /data/sources
ENTRYPOINT ["./server.run"]
