#!/usr/bin/env bash
# 编译 HTTP 服务二进制到 bin/wecom-http
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$ROOT/bin/wecom-http"

mkdir -p "$ROOT/bin" "$ROOT/logs"

echo "==> 编译 wecom-http ..."
cd "$ROOT/http"
go build -ldflags="-s -w" -o "$OUT" .

echo "==> 完成: $OUT"
ls -lh "$OUT"
