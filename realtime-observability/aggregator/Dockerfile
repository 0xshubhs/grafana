# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git

# Copy gen module first (from parent context)
COPY gen/ /build/gen/

# Copy aggregator module
COPY aggregator/go.mod aggregator/go.sum /build/aggregator/

WORKDIR /build/aggregator

# Update replace directive for Docker build context
RUN sed -i 's|\.\./gen|/build/gen|g' go.mod

RUN go mod download

# Copy aggregator source
COPY aggregator/ /build/aggregator/

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o aggregator ./cmd

# Runtime stage
FROM alpine:3.19

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Copy binary from builder
COPY --from=builder /build/aggregator/aggregator .

# Expose ports
# 9000 - gRPC
# 8080 - WebSocket
# 9100 - Prometheus metrics
EXPOSE 9000 8080 9100

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run
CMD ["./aggregator"]
