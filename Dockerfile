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
FROM golang:1.27-bookworm AS build
# .git is excluded from the build context (.dockerignore), so the version
# baked into the binary (internal/version) has to travel in as build args —
# the caller reads them off git itself, same as `mise run build` does inline.
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
RUN apt-get update && apt-get install -y --no-install-recommends build-essential \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=ui /app/internal/webui/dist ./internal/webui/dist
RUN go install github.com/swaggo/swag/cmd/swag@v1.16.6 \
    && swag init -g cmd/server/main.go -o docs
RUN CGO_ENABLED=1 go build -ldflags="-s -w -X darow/internal/version.Version=${VERSION} -X darow/internal/version.Commit=${COMMIT} -X darow/internal/version.BuildDate=${BUILD_DATE}" -o /out/server.run ./cmd/server
# data/ is excluded from the build context (.dockerignore): server.workDir defaults to ./data, and
# distroless carries no shell to mkdir it in the final stage, so an empty one is staged here instead,
# owned by the :nonroot image's fixed uid/gid (65532) so the server can write to it without root.
RUN mkdir -p /out/data && chown 65532:65532 /out/data

# Stage 3: final image (glibc compatible), running as the image's built-in nonroot user
FROM gcr.io/distroless/cc-debian12:nonroot
WORKDIR /app
COPY --from=build /out/server.run ./server.run
COPY --from=build /out/data /data
ENTRYPOINT ["./server.run"]
