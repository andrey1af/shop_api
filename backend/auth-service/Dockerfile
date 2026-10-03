# syntax=docker/dockerfile:1.7

FROM golang:1.27.1-alpine AS builder

WORKDIR /src

COPY gen ./gen

WORKDIR /src/auth-service

COPY auth-service/go.mod auth-service/go.sum ./
RUN go mod download
COPY auth-service/cmd ./cmd
COPY auth-service/internal ./internal
COPY auth-service/pkg ./pkg

RUN CGO_ENABLED=0 go build \
      -trimpath \
      -buildvcs=false \
      -ldflags='-s -w' \
      -o /out/auth \
      ./cmd/auth

RUN CGO_ENABLED=0 go build \
      -trimpath \
      -buildvcs=false \
      -ldflags='-s -w' \
      -o /out/migrator \
      ./cmd/migrator

FROM alpine:3.22 AS runtime

RUN addgroup -S app && adduser -S -G app app

WORKDIR /app
COPY --from=builder /out/auth /app/auth

USER app

ENTRYPOINT ["/app/auth"]

FROM alpine:3.22 AS migrator

RUN addgroup -S app && adduser -S -G app app

WORKDIR /app
COPY --from=builder /out/migrator /app/migrator
COPY auth-service/migrations /app/migrations

USER app

ENTRYPOINT ["/app/migrator"]
