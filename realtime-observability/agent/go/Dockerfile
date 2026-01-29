# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod ./
RUN go mod download

# Copy source code
COPY . .

# Build the example agent
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o agent ./example

# Runtime stage
FROM alpine:3.19

WORKDIR /app

RUN apk --no-cache add ca-certificates

COPY --from=builder /app/agent .

CMD ["./agent"]
