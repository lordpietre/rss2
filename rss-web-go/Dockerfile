FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install git 
RUN apk add --no-cache git

# Copy files
COPY . .

# Download dependencies
RUN go mod tidy && go mod download

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o rss-web .

# Final stage
FROM alpine:latest  
WORKDIR /root/
COPY --from=builder /app/rss-web .

# Copy static assets and templates manually for now (assuming context is root)
# In docker-compose we will mount volumes or copy them
# COPY templates ./templates
# COPY static ./static

CMD ["./rss-web"]
