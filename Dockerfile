# ===== 构建阶段 =====
FROM golang:1.26-alpine AS build
RUN apk add --no-cache git ca-certificates
WORKDIR /app

# 先缓存依赖
COPY go.mod go.sum ./
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}
RUN --mount=type=cache,target=/root/.cache/go-build go mod download

# 拷贝源码并构建
COPY . .
ENV CGO_ENABLED=0 GOOS=linux
RUN --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" -o /app/bin/vistack ./cmd/vistack

# ===== 核心镜像（api / worker，不含 ffmpeg） =====
FROM alpine:3.20 AS vistack
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 appuser
WORKDIR /app
COPY --from=build /app/bin/vistack /app/vistack
COPY conf /app/conf
RUN chmod -R a+rX /app/conf
USER appuser
EXPOSE 8080
ENTRYPOINT ["/app/vistack"]
CMD ["api"]

# ===== transcoder 镜像（内置 ffmpeg/ffprobe） =====
FROM alpine:3.20 AS vistack-transcoder
RUN echo "https://dl-cdn.alpinelinux.org/alpine/v3.20/community" >> /etc/apk/repositories \
    && apk add --no-cache ca-certificates tzdata ffmpeg \
    && adduser -D -u 10001 appuser
WORKDIR /app
COPY --from=build /app/bin/vistack /app/vistack
COPY conf /app/conf
RUN chmod -R a+rX /app/conf
USER appuser
EXPOSE 50051
ENTRYPOINT ["/app/vistack"]
CMD ["transcoder"]
