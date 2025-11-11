# Backend build stage
FROM golang:1.24-alpine AS backend-builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary with embedded frontend (frontend already built locally)
RUN CGO_ENABLED=0 GOOS=linux go build -o ignite-api cmd/api/main.go

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from backend builder (includes embedded frontend)
COPY --from=backend-builder /build/ignite-api .

# Expose API port
EXPOSE 3000

# Run as non-root user for security
RUN addgroup -g 1000 ignite && \
    adduser -D -u 1000 -G ignite ignite && \
    chown -R ignite:ignite /app

USER ignite

ENTRYPOINT ["./ignite-api"]
