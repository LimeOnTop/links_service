# syntax=docker/dockerfile:1.6

FROM golang:1.22 AS builder
WORKDIR /src
ENV CGO_ENABLED=0 GOOS=linux GOTOOLCHAIN=auto
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -ldflags="-s -w" -o /out/links-service ./cmd/server

FROM alpine:3.20
RUN addgroup -S app && adduser -S app -G app
WORKDIR /app
COPY --from=builder /out/links-service /app/links-service
RUN mkdir -p /app/data && chown -R app:app /app
USER app
EXPOSE 8080
VOLUME ["/app/data"]
ENTRYPOINT ["/app/links-service"]
