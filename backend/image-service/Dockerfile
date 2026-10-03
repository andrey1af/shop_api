# syntax=docker/dockerfile:1.7

FROM golang:1.27.1-alpine AS builder

WORKDIR /src

COPY gen ./gen

WORKDIR /src/image-service

COPY image-service/go.mod image-service/go.sum ./
RUN go mod download
COPY image-service/cmd ./cmd
COPY image-service/internal ./internal

RUN CGO_ENABLED=0 go build \
      -trimpath \
      -buildvcs=false \
      -ldflags='-s -w' \
      -o /out/image \
      ./cmd/image

RUN CGO_ENABLED=0 go build \
      -trimpath \
      -buildvcs=false \
      -ldflags='-s -w' \
      -o /out/migrator \
      ./cmd/migrator

FROM alpine:3.22 AS runtime

RUN addgroup -S app && adduser -S -G app app

WORKDIR /app
COPY --from=builder /out/image /app/image

USER app

ENTRYPOINT ["/app/image"]

FROM alpine:3.22 AS migrator

RUN addgroup -S app && adduser -S -G app app

WORKDIR /app
COPY --from=builder /out/migrator /app/migrator
COPY image-service/migrations /app/migrations

USER app

ENTRYPOINT ["/app/migrator"]
