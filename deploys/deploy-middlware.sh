#!/bin/bash

# 安装依赖
docker-compose -f ../docker-compose.override.yml --env-file ../.env.local up -d
echo "依赖安装完成"

