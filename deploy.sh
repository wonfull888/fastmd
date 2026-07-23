#!/bin/bash
set -e

APP_DIR="/www/wwwroot/fastmd"
BINARY="${APP_DIR}/dist/fastmd-server"
ARGS="--port 9000 --db ${APP_DIR}/data/fastmd.db"

echo "=== fastmd deploy ==="

cd "$APP_DIR"
echo "[1/4] git pull..."
git pull

echo "[2/4] go build..."
make build-server

echo "[3/4] stop old process..."
OLDPID=$(ps aux | grep 'fastmd-server' | grep -v grep | awk '{print $2}')
if [ -n "$OLDPID" ]; then
    kill "$OLDPID"
    sleep 1
    echo "  killed PID $OLDPID"
else
    echo "  no running process"
fi

echo "[4/4] start new process..."
nohup "$BINARY" $ARGS > /dev/null 2>&1 &
NEWPID=$!
sleep 1
if kill -0 "$NEWPID" 2>/dev/null; then
    echo "  started PID $NEWPID"
    echo "=== deploy OK ==="
else
    echo "  ERROR: process failed to start"
    exit 1
fi
