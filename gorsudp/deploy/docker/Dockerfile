# Multi-stage build for GoRSUDP
FROM golang:1.19-alpine AS builder

# Install necessary packages
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
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o gorsudp \
    ./cmd/gorsudp

# Check if test generator exists and build it if available
RUN if [ -d "./cmd/gorsudp-test" ] && [ -n "$(find ./cmd/gorsudp-test -name '*.go' -print -quit)" ]; then \
        echo "Building test generator..."; \
        CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
            -ldflags='-w -s -extldflags "-static"' \
            -a -installsuffix cgo \
            -o gorsudp-testgen \
            ./cmd/gorsudp-test; \
    else \
        echo "Test generator not found or no Go files, skipping"; \
    fi

# Production stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add \
    ca-certificates \
    tzdata \
    curl \
    file \
    && adduser -D -s /bin/sh gorsudp

# Set timezone (can be overridden via environment)
ENV TZ=UTC

# Create necessary directories
RUN mkdir -p /app/data /app/logs /app/screenshots /app/config \
    && chown -R gorsudp:gorsudp /app

# Copy main binary from builder stage
COPY --from=builder /app/gorsudp /usr/local/bin/gorsudp

# Create directory structure for user
RUN mkdir -p /app/web /usr/local/bin

# Switch to non-root user
USER gorsudp
WORKDIR /app

# Default configuration (can be overridden by volume mount)
ENV GORSUDP_CONFIG=/app/config/config.json
ENV GORSUDP_DATA_DIR=/app/data
ENV GORSUDP_LOG_DIR=/app/logs
ENV GORSUDP_SCREENSHOT_DIR=/app/screenshots

# Expose ports
EXPOSE 8888/udp 8080/tcp

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD gorsudp --health-check || exit 1

# Set the signal used to stop the container
STOPSIGNAL SIGTERM

# Default command
CMD ["gorsudp"]