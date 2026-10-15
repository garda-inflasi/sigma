FROM golang:1.26-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy Go module files first for layer caching
COPY apps/api/go.mod apps/api/go.sum ./apps/api/
COPY go.work go.work.sum ./

# Download dependencies
WORKDIR /app/apps/api
RUN go mod download

# Copy all source code
WORKDIR /app
COPY apps/api/ ./apps/api/

# Build the binary
WORKDIR /app/apps/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/api ./cmd/api

# ── Runtime stage ──────────────────────────────────────────────────────
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

COPY --from=builder /app/bin/api ./api

# Use non-root user
USER appuser

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/healthz || exit 1

CMD ["./api"]

