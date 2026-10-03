# syntax=docker/dockerfile:1.7

FROM golang:1.27.1-alpine AS builder

WORKDIR /src/generator-service

COPY generator-service/go.mod generator-service/go.sum ./
RUN go mod download
COPY generator-service/cmd ./cmd
COPY generator-service/internal ./internal

RUN CGO_ENABLED=0 go build \
      -trimpath \
      -buildvcs=false \
      -ldflags='-s -w' \
      -o /out/generator \
      ./cmd/generator

FROM alpine:3.22 AS runtime

RUN addgroup -S app && adduser -S -G app app

WORKDIR /app
COPY --from=builder /out/generator /app/generator

USER app

ENTRYPOINT ["/app/generator"]
