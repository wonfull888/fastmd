#!/bin/bash
set -e

PROJECT="/www/wwwroot/fastmd"
cd $PROJECT

echo "[$(date '+%Y-%m-%d %H:%M:%S')] Deploy started"

# 拉取最新代码
git pull origin main

# 找到 Go 可执行文件路径（宝塔安装的位置）
GO_BIN=$(which go 2>/dev/null || echo "/usr/local/go/bin/go")
export PATH=$(dirname $GO_BIN):$PATH

# 创建输出目录
mkdir -p dist data

# 编译服务端
$GO_BIN build \
  -ldflags "-X main.Version=$(git describe --tags --always 2>/dev/null || echo 'dev')" \
  -o dist/fastmd-server \
  ./cmd/server

# 停止旧进程（宝塔守护进程会自动重启）
pkill -f "fastmd-server" || true

echo "[$(date '+%Y-%m-%d %H:%M:%S')] Deploy complete. Binary: dist/fastmd-server"
