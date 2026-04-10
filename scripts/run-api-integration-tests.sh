#!/usr/bin/env bash
# Запуск интеграционных тестов API (нужен работающий монолит из backend этой ветки).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
export API_TEST_BASE_URL="${API_TEST_BASE_URL:-http://127.0.0.1:8080}"
echo "API_TEST_BASE_URL=$API_TEST_BASE_URL"
cd "$ROOT/backend"
exec go test ./test/integration/... -count=1 -timeout=10m "$@"
