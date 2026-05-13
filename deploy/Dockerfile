# Multi-stage build
FROM golang:alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o gos3 ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o gos3c ./cmd/client

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/gos3 /usr/local/bin/
COPY --from=builder /app/gos3c /usr/local/bin/
COPY deploy/config.example.yaml /app/config.yaml
VOLUME ["/data"]
EXPOSE 9000 9001
HEALTHCHECK --interval=30s --timeout=5s \
  CMD wget -qO- http://localhost:9000/_health || exit 1
ENTRYPOINT ["gos3"]
