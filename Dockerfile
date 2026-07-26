# syntax=docker/dockerfile:1

FROM golang:1.25.12-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal

ARG VERSION=dev
ARG BUILD_DATE=unknown
RUN CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags="-s -w -buildid= -X main.version=${VERSION} -X main.date=${BUILD_DATE}" \
    -o /mcp-vmanomaly \
    ./cmd/mcp-vmanomaly

FROM alpine:3.22
RUN apk add --no-cache ca-certificates
RUN addgroup -g 1000 -S mcp && adduser -u 1000 -S mcp -G mcp

COPY --from=builder --chown=mcp:mcp /mcp-vmanomaly /usr/local/bin/mcp-vmanomaly

USER mcp
WORKDIR /home/mcp
ENTRYPOINT ["mcp-vmanomaly"]
