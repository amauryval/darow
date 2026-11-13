# syntax=docker/dockerfile:1
# The platform's libraries (github.com/MyAppLibs/go-pkg, web-platform) are private repositories: the
# build reads them with a token given as the BuildKit secret `github_token`
# (docker build --secret id=github_token,env=PLATFORM_TOKEN .), passed to git for one command through
# GIT_CONFIG_* and never written to a layer.

# ─────────────────────────────────────────────
# Stage 1 — Build frontend (Vite / TypeScript)
# ─────────────────────────────────────────────
FROM node:26-alpine AS frontend-builder
# The owner after the © of the sidebar's foot, read by Vite at build time; empty for none.
ARG VITE_OWNER=
WORKDIR /build
COPY ./web ./web
WORKDIR /build/web
RUN apk add --no-cache git
RUN --mount=type=secret,id=github_token,env=GITHUB_TOKEN \
    GIT_CONFIG_COUNT=2 \
    GIT_CONFIG_KEY_0="url.https://x-access-token:${GITHUB_TOKEN}@github.com/.insteadOf" \
    GIT_CONFIG_VALUE_0="https://github.com/" \
    GIT_CONFIG_KEY_1="url.https://x-access-token:${GITHUB_TOKEN}@github.com/.insteadOf" \
    GIT_CONFIG_VALUE_1="ssh://git@github.com/" \
    npm ci
RUN npm run build-only

# ─────────────────────────────────────────────
# Stage 2 — Build Go server binary, with cgo (the DuckDB driver links its C library)
# ─────────────────────────────────────────────
FROM golang:1.27.4-bookworm AS go-builder
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
ENV GOPRIVATE=github.com/MyAppLibs/*,github.com/amauryval/*
WORKDIR /build
COPY . .
RUN --mount=type=secret,id=github_token,env=GITHUB_TOKEN \
    GIT_CONFIG_COUNT=1 \
    GIT_CONFIG_KEY_0="url.https://x-access-token:${GITHUB_TOKEN}@github.com/.insteadOf" \
    GIT_CONFIG_VALUE_0="https://github.com/" \
    go mod download
COPY --from=frontend-builder /build/web/dist ./web/dist
RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags="-s -w \
        -X github.com/MyAppLibs/go-pkg/version.Version=${VERSION} \
        -X github.com/MyAppLibs/go-pkg/version.Commit=${COMMIT} \
        -X github.com/MyAppLibs/go-pkg/version.BuildDate=${BUILD_DATE}" \
    -trimpath -o darow ./cmd/darow

# ─────────────────────────────────────────────
# Stage 3 — Final minimal image
# ─────────────────────────────────────────────
FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --uid 10001 --no-create-home darow \
    && mkdir -p /data \
    && chown darow:darow /data
COPY --from=go-builder /build/darow /darow

VOLUME /data
EXPOSE 3000

ENV HOST=0.0.0.0
ENV PORT=3000
# DuckDB installs its spatial extension under $HOME/.duckdb: the volume, writable by uid 10001.
ENV HOME=/data

ENTRYPOINT ["sh", "-c", "chown -R darow:darow /data && exec setpriv --reuid=10001 --regid=10001 --init-groups /darow"]
