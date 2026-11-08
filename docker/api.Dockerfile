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

# Build the binaries: API server, migration runner, seeder, background worker, and CLI utility (gardactl).
WORKDIR /app/apps/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/api ./cmd/api \
 && CGO_ENABLED=0 GOOS=linux go build -o /app/bin/migrate ./cmd/migrate \
 && CGO_ENABLED=0 GOOS=linux go build -o /app/bin/seed ./cmd/seed \
 && CGO_ENABLED=0 GOOS=linux go build -o /app/bin/worker ./cmd/worker \
 && CGO_ENABLED=0 GOOS=linux go build -o /app/bin/gardactl ./cmd/gardactl

# ── Runtime stage ──────────────────────────────────────────────────────
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

COPY --from=builder /app/bin/api ./api
COPY --from=builder /app/bin/migrate ./migrate
COPY --from=builder /app/bin/seed ./seed
COPY --from=builder /app/bin/worker ./worker
COPY --from=builder /app/bin/gardactl ./gardactl
# Migrations ship inside the image so deploy never depends on files on the VM.
COPY db/migrations /migrations
ENV MIGRATIONS_DIR=/migrations

# Direktori arsip audit (volume audit_arsip) harus bisa ditulis worker yang berjalan sebagai appuser.
RUN mkdir -p /app/arsip/audit && chown -R appuser:appgroup /app/arsip

# Use non-root user
USER appuser

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/healthz || exit 1

CMD ["./api"]

