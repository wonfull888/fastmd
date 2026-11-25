#!/bin/sh
set -eu
cd /www/wwwroot/fastmd
exec python3 scripts/deploy.py "$@"
