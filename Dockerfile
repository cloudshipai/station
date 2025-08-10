# Station Docker Image
# Multi-stage build for minimal production image

# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata gcc musl-dev sqlite-dev

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o stn ./cmd/main

# Production stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates sqlite tzdata

# Create non-root user
RUN addgroup -g 1001 -S station && \
    adduser -u 1001 -S station -G station

# Set working directory
WORKDIR /home/station

# Copy binary from builder stage
COPY --from=builder /app/stn /usr/local/bin/stn

# Create necessary directories
RUN mkdir -p /home/station/.config/station /home/station/data && \
    chown -R station:station /home/station

# Switch to non-root user
USER station

# Set environment variables
ENV STATION_CONFIG_DIR=/home/station/.config/station
ENV STATION_DATA_DIR=/home/station/data
ENV STATION_DATABASE_URL=/home/station/data/station.db

# Expose ports
EXPOSE 8080 2222 3000

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD stn --version || exit 1

# Default command
CMD ["stn", "serve", "--host", "0.0.0.0"]