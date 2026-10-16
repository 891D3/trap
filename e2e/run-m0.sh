#!/usr/bin/env bash
# 启动独立 Vite 服务，再运行真实 macOS WKWebView M0 E2E。
# 不使用 mygo --web：SQLite 插件在 Go WASM 中不支持文件/内存操作；
# 本脚本由 Go E2E 在原生 mygo WebView 内驱动页面与 typed IPC。
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
PORT="${TRAP_E2E_PORT:-5199}"
URL="http://127.0.0.1:${PORT}/"
LOG_FILE="${TMPDIR:-/tmp}/trap-e2e-vite-${PORT}.log"

cleanup() {
  if [[ -n "${VITE_PID:-}" ]]; then
    kill "${VITE_PID}" 2>/dev/null || true
  fi
}
trap cleanup EXIT INT TERM

cd "$ROOT_DIR"
npm run dev:web -- --host 127.0.0.1 --port "$PORT" >"$LOG_FILE" 2>&1 &
VITE_PID=$!

for _ in $(seq 1 100); do
  if curl -fsS "$URL" >/dev/null 2>&1; then
    break
  fi
  sleep 0.1
done

if ! curl -fsS "$URL" >/dev/null 2>&1; then
  echo "Vite M0 E2E 服务未启动，日志：$LOG_FILE" >&2
  cat "$LOG_FILE" >&2
  exit 1
fi

TRAP_E2E_URL="$URL" go run -tags e2e ./e2e
TRAP_E2E_URL="$URL" go run -tags e2e ./e2e -onboarding
