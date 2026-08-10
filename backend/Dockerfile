FROM golang:1.23 AS builder

WORKDIR /app

RUN apt-get update && apt-get install -y gcc musl-dev git

COPY go.mod ./
RUN go mod download

COPY . .
RUN go mod tidy

RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -o /server ./cmd/server

FROM alpine:3.23

RUN apk add --no-cache ca-certificates tzdata postgresql-client

WORKDIR /app

COPY --from=builder /server .

EXPOSE 8080

CMD ["./server"]
