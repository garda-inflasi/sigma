FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go.mod and go.sum first for caching
COPY apps/api/go.mod apps/api/go.sum ./apps/api/
COPY go.work ./

# Download dependencies
WORKDIR /app/apps/api
RUN go mod download

# Copy source code
WORKDIR /app
COPY apps/api/ ./apps/api/

# Build
WORKDIR /app/apps/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/api ./cmd/api

# Runtime stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/bin/api ./api

EXPOSE 8080

CMD ["./api"]
