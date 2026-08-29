# ===== 前端构建阶段（web-client，产物内置到 api 镜像供静态托管） =====
# NODE_IMAGE 可覆盖（如网络受限时 --build-arg NODE_IMAGE=docker.m.daocloud.io/library/node:22-alpine）
ARG NODE_IMAGE=node:22-alpine
FROM ${NODE_IMAGE} AS web-build
RUN corepack enable
WORKDIR /app
# 先缓存依赖：仅拷贝各 workspace 包的 package.json
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY web/ui/package.json ./web/ui/package.json
COPY web/web-client/package.json ./web/web-client/package.json
COPY web/web-admin/package.json ./web/web-admin/package.json
RUN --mount=type=cache,target=/root/.local/share/pnpm/store pnpm install --frozen-lockfile
# 拷贝源码并构建（vite outDir 为仓库根 build/*）
COPY web/ui ./web/ui
COPY web/web-client ./web/web-client
COPY web/web-admin ./web/web-admin
RUN pnpm --filter web-client build
# 管理端托管于 /admin/ 子路径，构建时注入 base
RUN VITE_BASE=/admin/ pnpm --filter web-admin build

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

# ===== 核心镜像（api / worker / auth，不含 ffmpeg） =====
FROM alpine:3.20 AS vistack
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 appuser
WORKDIR /app
COPY --from=build /app/bin/vistack /app/vistack
COPY conf /app/conf
COPY --from=web-build /app/build/web-client /app/web
COPY --from=web-build /app/build/web-admin /app/web-admin
RUN chmod -R a+rX /app/conf /app/web /app/web-admin
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
