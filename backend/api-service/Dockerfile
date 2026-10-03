# syntax=docker/dockerfile:1.7

FROM golang:1.27.1-alpine AS builder

WORKDIR /src

COPY gen ./gen

WORKDIR /src/api-service

COPY api-service/go.mod api-service/go.sum ./
RUN go mod download
COPY api-service/cmd ./cmd
COPY api-service/internal ./internal
COPY api-service/pkg ./pkg

RUN CGO_ENABLED=0 go build \
      -trimpath \
      -buildvcs=false \
      -ldflags='-s -w' \
      -o /out/api \
      ./cmd/api

RUN CGO_ENABLED=0 go build \
      -trimpath \
      -buildvcs=false \
      -ldflags='-s -w' \
      -o /out/migrator \
      ./cmd/migrator

FROM alpine:3.22 AS runtime

RUN addgroup -S app && adduser -S -G app app

WORKDIR /app
COPY --from=builder /out/api /app/api

USER app

ENTRYPOINT ["/app/api"]

FROM alpine:3.22 AS migrator

RUN addgroup -S app && adduser -S -G app app

WORKDIR /app
COPY --from=builder /out/migrator /app/migrator
COPY api-service/migrations /app/migrations

USER app

ENTRYPOINT ["/app/migrator"]
