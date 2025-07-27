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

# Build the application with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -a -installsuffix cgo \
    -ldflags="-w -s" \
    -o mongo-exporter main.go

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1001 -S mongo-exporter && \
    adduser -u 1001 -S mongo-exporter -G mongo-exporter

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/mongo-exporter .

# Copy configuration file
COPY --from=builder /app/config.yaml .

# Change ownership to non-root user
RUN chown -R mongo-exporter:mongo-exporter /app

# Switch to non-root user
USER mongo-exporter

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
ENTRYPOINT ["./mongo-exporter"]
CMD ["-config", "config.yaml"] 