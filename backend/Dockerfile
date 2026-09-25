FROM golang:1.23 AS builder

WORKDIR /app

RUN apt-get update && apt-get install -y gcc musl-dev git

COPY . .
RUN go mod tidy

# Build all commands
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -o /server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -o /scraper ./cmd/scraper
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -o /discovery ./cmd/discovery
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -o /wiki-worker ./cmd/wiki_worker
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -o /topics ./cmd/topics
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -o /related ./cmd/related

FROM alpine:3.23

RUN apk add --no-cache ca-certificates tzdata postgresql-client

WORKDIR /app

COPY --from=builder /server /server
COPY --from=builder /scraper /scraper
COPY --from=builder /discovery /discovery
COPY --from=builder /wiki-worker /wiki-worker
COPY --from=builder /topics /topics
COPY --from=builder /related /related

# Symlinks for convenience
RUN ln -s /server /usr/local/bin/server && \
    ln -s /scraper /usr/local/bin/scraper && \
    ln -s /discovery /usr/local/bin/discovery && \
    ln -s /wiki-worker /usr/local/bin/wiki-worker && \
    ln -s /topics /usr/local/bin/topics && \
    ln -s /related /usr/local/bin/related

ENV PATH="/usr/local/bin:${PATH}"

EXPOSE 8080

ENTRYPOINT ["/server"]