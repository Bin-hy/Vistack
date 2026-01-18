#!/bin/bash

# 检查 go 版本 是否安装
if ! command -v go &> /dev/null
then
    echo "go 未安装"
# 安装 go
sudo apt update
sudo apt install golang-go
fi

# 检查 go 版本是否为 1.25 以上
if [ "$(go version | cut -d ' ' -f 3 | cut -d '.' -f 1-2)" != "go1.25" ]; then
    echo "go 版本不是 1.25 以上, 请先升级至1.25及以上"
    exit 1
fi

# 检查 node 是否安装
if ! command -v node &> /dev/null
then
    echo "node 未安装，请安装 node"
    exit 1
fi

# 检查 pnpm 是否安装
if ! command -v pnpm &> /dev/null
then
    echo "pnpm 未安装，正在安装 pnpm"
    npm install -g pnpm
fi

# 安装 Ffmpeg
sudo apt update
sudo apt install ffmpeg

# 检查 Ffmpeg 是否安装成功
if ! command -v ffmpeg &> /dev/null
then
    echo "Ffmpeg 安装失败"
    exit 1
fi

echo "Ffmpeg 安装成功"

# 制作配置文件
if [ -f "../conf/app.local.toml" ]; then
    echo "已存在 ../app.local.toml 文件"
fi
cp ../conf/app.toml ../conf/app.local.toml

if [ -f "../.env.local" ]; then
    echo "已存在 ../.env.local 文件"
fi
cp ../.env.example ../.env.local

if [ -f "../docker-compose.override.yml" ]; then
    echo "已存在 ../docker-compose.override.yml 文件"
fi
cp ../compose.yml ../docker-compose.override.yml

if [ -f "../web/web-client/.env.prod" ]; then
    echo "已存在 ../web/web-client/.env.prod 文件"
fi
cp ../web/web-client/.env.development ../web/web-client/.env.prod

echo "配置文件制作完成,请检查 ../conf/app.local.toml 文件"
echo "请修改 ../.env.local 文件"
echo "请修改 ../docker-compose.override.yml 文件"
echo "请修改 ../web/web-client/.env.prod 文件"

# 设置 systemd
sudo cp ./vistack.service /etc/systemd/system/vistack.service
sudo systemctl enable vistack.service

echo "已设置 vistack.service, 允许开机自启"
echo "-------------------------------------"

echo "请执行依赖文件 deploy-dependency.sh， 启动mysql, reids, minio, kafka中间件"
