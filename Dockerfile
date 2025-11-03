# syntax=docker/dockerfile:1.6

FROM golang:1.25-alpine AS build
RUN apk add --no-cache git ca-certificates
WORKDIR /app

# Cache dependencies first
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/root/.cache/go-build go mod download

# Copy source
COPY . .

# Build
ENV CGO_ENABLED=0 GOOS=linux
RUN --mount=type=cache,target=/root/.cache/go-build go build -trimpath -ldflags="-s -w" -o /app/bin/vistack ./main.go

FROM alpine:3.20
RUN apk add --no-cache curl \
    && adduser -D -u 10001 appuser
WORKDIR /app

COPY --from=build /app/bin/vistack /app/vistack
COPY conf /app/conf

EXPOSE 8080
USER appuser
CMD ["/app/vistack"]