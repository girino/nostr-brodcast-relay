# Multi-stage build for broadcast-relay
# Use latest Go version to support go.mod requirements
FROM golang:alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application (GOOS and GOARCH set by buildx for target platform)
RUN CGO_ENABLED=0 go build -a -installsuffix cgo -ldflags="-w -s" -o broadcast-relay .

# Final stage - minimal runtime image
FROM alpine:latest

# Install ca-certificates for HTTPS connections to relays
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 relay && \
    adduser -D -u 1000 -G relay relay

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/broadcast-relay .

# Copy static assets and templates
COPY --from=builder /build/static ./static
COPY --from=builder /build/templates ./templates

# Change ownership
RUN chown -R relay:relay /app

# Switch to non-root user
USER relay

# Expose relay port (default 3334)
EXPOSE 3334

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:3334/stats || exit 1

# Run the relay
ENTRYPOINT ["./broadcast-relay"]

