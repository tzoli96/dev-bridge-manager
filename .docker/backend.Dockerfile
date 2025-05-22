# Development stage
FROM golang:1.23-alpine AS development

RUN apk add --no-cache git ca-certificates curl bash

# Install Air for hot reload
RUN go install github.com/air-verse/air@latest
RUN go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
RUN go install github.com/go-delve/delve/cmd/dlv@latest

WORKDIR /app

EXPOSE 8080
EXPOSE 2345

CMD ["air"]

# Build stage
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Copy and download dependencies first (better caching)
COPY backend/go.* ./
RUN go mod download

# Copy source and build
COPY backend/ ./
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o app ./cmd/server

# Runtime stage
FROM alpine:latest

RUN apk add --no-cache ca-certificates curl && \
    adduser -D -s /bin/sh appuser

WORKDIR /app

COPY --from=builder /app/app .
RUN chown appuser:appuser app

USER appuser
EXPOSE 8080

CMD ["./app"]