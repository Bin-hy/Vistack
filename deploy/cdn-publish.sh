#!/usr/bin/env bash
# =============================================================
# cdn-publish.sh —— 一键发布前端到 CDN
#
# 用法:
#   ./deploy/cdn-publish.sh s3          # 对象存储（默认）
#   ./deploy/cdn-publish.sh pages       # Cloudflare Pages
#   ./deploy/cdn-publish.sh dry         # 仅构建 + 合并布局，不上传（预检用）
#
# 环境变量（也可直接前置在命令前）:
#   VITE_API_BASE   前端 API 地址，默认 /api（同域反代场景）
#   VITE_BASE       管理端 base，默认 /admin/
#   CDN_BUCKET      s3 模式: 桶名（必填）
#   CDN_ENDPOINT    s3 模式: S3 兼容端点（MinIO / OSS / COS），配合 mc alias
#   CF_PROJECT_NAME pages 模式: Cloudflare Pages 项目名（必填）
#
# 示例:
#   VITE_API_BASE=/api ./deploy/cdn-publish.sh s3
#   PROVIDER=pages CF_PROJECT_NAME=vistack ./deploy/cdn-publish.sh pages
# =============================================================
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROVIDER="${1:-${PROVIDER:-s3}}"
VITE_API_BASE="${VITE_API_BASE:-/api}"
VITE_BASE="${VITE_BASE:-/admin/}"

echo "==> 构建前端（VITE_API_BASE=${VITE_API_BASE}，VITE_BASE=${VITE_BASE}）"
cd "$ROOT/web"
# CI=true：无 TTY 环境下的非交互模式（脚本 / CI 场景）
export CI=true
pnpm install
VITE_API_BASE="$VITE_API_BASE" pnpm --filter web-client build
VITE_API_BASE="$VITE_API_BASE" VITE_BASE="$VITE_BASE" pnpm --filter web-admin build

# 合并布局：管理端产物并入用户端产物的 admin/ 子目录
# （单桶 / 单 Pages 项目同时承载用户端与管理端）
STAGE="$ROOT/build/cdn"
rm -rf "$STAGE"
mkdir -p "$STAGE"
cp -r "$ROOT/build/web-client/." "$STAGE/"
mkdir -p "$STAGE/admin"
cp -r "$ROOT/build/web-admin/." "$STAGE/admin/"

# SPA 回退规则（Cloudflare Pages 专用；S3/OSS 静态网站需另配 Error Document）
cat > "$STAGE/_redirects" <<'EOF'
/admin       /admin/index.html  200
/admin/*     /admin/index.html  200
/*           /index.html        200
EOF

case "$PROVIDER" in
  dry)
    echo "==> dry 模式：仅构建 + 合并布局，不执行上传"
    echo "    产物目录: $STAGE （用户端 /，管理端 /admin/）"
    ;;
  s3)
    : "${CDN_BUCKET:?请设置 CDN_BUCKET（桶名）}"
    if command -v mc >/dev/null; then
      echo "==> 使用 mc 同步到 ${CDN_BUCKET}"
      # 需先配置 alias：mc alias set local <ENDPOINT> <AK> <SK>
      mc mirror --overwrite "$STAGE/" "local/${CDN_BUCKET}/"
    elif command -v aws >/dev/null; then
      echo "==> 使用 aws cli 同步到 s3://${CDN_BUCKET}"
      aws s3 sync "$STAGE/" "s3://${CDN_BUCKET}/" --delete
    else
      echo "错误: s3 模式需要安装 mc（minio/mc）或 aws cli" >&2
      exit 1
    fi
    echo "提示: 若使用 S3/OSS 静态网站托管，请将 Error Document / 404 规则指向 /index.html（SPA 回退）"
    ;;
  pages)
    : "${CF_PROJECT_NAME:?请设置 CF_PROJECT_NAME（Cloudflare Pages 项目名）}"
    if ! command -v wrangler >/dev/null; then
      echo "错误: pages 模式需要安装 wrangler（npm i -g wrangler）" >&2
      exit 1
    fi
    echo "==> 部署到 Cloudflare Pages 项目: ${CF_PROJECT_NAME}"
    wrangler pages deploy "$STAGE" --project-name "$CF_PROJECT_NAME"
    ;;
  *)
    echo "错误: 未知 PROVIDER '${PROVIDER}'（支持 s3 | pages）" >&2
    exit 1
    ;;
esac

echo "==> 发布完成 ✅（入口：用户端 /，管理端 /admin/）"
