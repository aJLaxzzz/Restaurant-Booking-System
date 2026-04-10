#!/usr/bin/env bash
# Перезапуск локальной разработки: инфра в Docker, монолит API, Vite.
# Использование: из корня репозитория — bash scripts/dev-restart.sh
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

echo "==> Docker: postgres, redis, rabbitmq"
docker compose up -d postgres redis rabbitmq

# Шлюз nginx (8080:80) отдаёт свой /health и проксирует /api в микросервисы — без монолита и без /health/diag.
echo "==> Останавливаю nginx-шлюз, если был запущен (порт 8080 — монолит)"
docker compose stop nginx 2>/dev/null || true

export DATABASE_URL="${DATABASE_URL:-postgres://rbs:rbs@127.0.0.1:5433/restaurant_booking?sslmode=disable}"
export REDIS_ADDR="${REDIS_ADDR:-127.0.0.1:6379}"
export RABBITMQ_URL="${RABBITMQ_URL:-amqp://guest:guest@127.0.0.1:5672/}"
export ADDR="${ADDR:-:8080}"

# Все PID, слушающие порт (одной строкой kill с переводом строки не убить).
kill_port() {
  local port="$1"
  if ! command -v lsof >/dev/null 2>&1; then
    return 0
  fi
  local pids
  pids="$(lsof -ti:"$port" 2>/dev/null || true)"
  if [[ -z "${pids}" ]]; then
    return 0
  fi
  echo "==> Освобождаю порт ${port}: $(echo "$pids" | tr '\n' ' ')"
  echo "$pids" | xargs kill -9 2>/dev/null || true
  sleep 1
}

kill_port 8080
kill_port 5173

API_LOG="${TMPDIR:-/tmp}/rbs-api.log"
FE_LOG="${TMPDIR:-/tmp}/rbs-frontend.log"

echo "==> Restobook API (монолит) → http://localhost${ADDR} (лог: $API_LOG)"
cd "$ROOT/backend"
nohup env DATABASE_URL="$DATABASE_URL" REDIS_ADDR="$REDIS_ADDR" RABBITMQ_URL="$RABBITMQ_URL" ADDR="$ADDR" \
  go run ./cmd/api >>"$API_LOG" 2>&1 &
echo $! >"${TMPDIR:-/tmp}/rbs-api.pid"

for i in $(seq 1 40); do
  # Не доверяем только /health: nginx отдаёт «gateway ok» и маскирует отсутствие монолита.
  if curl -sf "http://localhost${ADDR}/health/diag" 2>/dev/null | grep -q current_database; then
    echo "==> Монолит API готов. Диагностика: curl -s http://localhost${ADDR}/health/diag"
    break
  fi
  sleep 0.25
  if [[ "$i" -eq 40 ]]; then
    echo "Таймаут: монолит не ответил JSON с current_database. См. $API_LOG"
    echo "Подсказка: не запускайте docker nginx на :8080 вместе с монолитом; см. scripts/dev-restart.sh"
    exit 1
  fi
done

echo "==> Restobook UI (Vite) → http://localhost:5173 (лог: $FE_LOG)"
cd "$ROOT/frontend"
nohup npm run dev >>"$FE_LOG" 2>&1 &
echo $! >"${TMPDIR:-/tmp}/rbs-frontend.pid"

echo ""
echo "Готово. Остановка вручную: kill \$(cat ${TMPDIR:-/tmp}/rbs-api.pid) и kill \$(cat ${TMPDIR:-/tmp}/rbs-frontend.pid)"
echo "Интеграционные тесты API: bash scripts/run-api-integration-tests.sh -v"
