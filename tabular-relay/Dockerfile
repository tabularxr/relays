# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o relay ./cmd/relay

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

# Create app directory
WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/relay .

# Create config directory
RUN mkdir -p config

# Create a non-root user
RUN addgroup -g 1001 -S relay && \
    adduser -S -D -H -u 1001 -h /root -s /sbin/nologin -G relay -g relay relay

# Change ownership of the app directory
RUN chown -R relay:relay /root

# Switch to non-root user
USER relay

# Expose the default port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Set environment variables with defaults
ENV RELAY_PORT=8080
ENV RELAY_LOG_LEVEL=info
ENV RELAY_MAX_CLIENTS=100
ENV RELAY_WORKER_THREADS=4
ENV RELAY_BATCH_INTERVAL_MS=100
ENV RELAY_SNAPSHOT_THRESHOLD=0.1
ENV RELAY_STAGS_ENDPOINT=http://localhost:8000/ingest

# Run the application
CMD ["./relay"]