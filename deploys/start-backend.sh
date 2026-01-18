#!/bin/bash
# 部署脚本 - 打包并发布前端到 nginx 目录

set -e  # 遇到错误立即退出

# === 基础配置 ===
PROJECT_DIR="/home/ubuntu/service/Vistack"
ROOT_DIR="/home/ubuntu/service/Vistack"
DIST_DIR="$ROOT_DIR/build"
TARGET_DIR="/home/ubuntu/dockers/caddy/website"
BEFORE_NAME="web-client"
TARGET_NAME="web-client"

echo "🚀 开始前端打包..."

cd "$PROJECT_DIR"

# 清理旧的 dist
rm -rf "$DIST_DIR"

# 打包
pnpm run build

# 检查打包是否成功
if [ ! -d "$DIST_DIR/$BEFORE_NAME" ]; then
  echo "打包失败：未找到 $DIST_DIR/$BEFORE_NAME 目录"
  exit 1
fi

echo "打包完成，准备部署..."

# 删除 nginx 目标目录旧版本
sudo rm -rf "$TARGET_DIR/$TARGET_NAME"

# 复制到 nginx 目录
sudo cp -r "$DIST_DIR/$TARGET_NAME" "$TARGET_DIR/"

echo "部署成功！文件已复制到：$TARGET_DIR/$TARGET_NAME"
