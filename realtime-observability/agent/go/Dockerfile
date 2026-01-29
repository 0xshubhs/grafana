# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /build

# Copy gen module first (from parent context)
COPY gen/ /build/gen/

# Copy agent module
COPY agent/go/go.mod /build/agent/go/

WORKDIR /build/agent/go

# Update replace directive for Docker build context
RUN sed -i 's|\.\./\.\./gen|/build/gen|g' go.mod

RUN go mod download

# Copy agent source
COPY agent/go/ /build/agent/go/

# Build the example agent
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o agent ./example

# Runtime stage
FROM alpine:3.19

WORKDIR /app

RUN apk --no-cache add ca-certificates

COPY --from=builder /build/agent/go/agent .

CMD ["./agent"]
