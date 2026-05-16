FROM golang:1.25-alpine

RUN apk add --no-cache curl bash

WORKDIR /app

# Copy dependency files first for caching
COPY go.mod go.sum* ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

# Copy source and build using cache mounts for faster rebuilds
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -o etl-worker main.go

COPY entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

ENTRYPOINT ["/app/entrypoint.sh"]
