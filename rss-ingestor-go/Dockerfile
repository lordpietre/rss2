# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install git and SSL certs
RUN apk add --no-cache git ca-certificates

# Copy source code immediately
COPY . .

# Download dependencies
RUN go mod tidy && go mod download

# Build the Go app
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o rss-ingestor .

# Final stage
FROM alpine:latest  

WORKDIR /root/

# Copy the Pre-built binary file from the previous stage
COPY --from=builder /app/rss-ingestor .
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Command to run the executable
CMD ["./rss-ingestor"]
