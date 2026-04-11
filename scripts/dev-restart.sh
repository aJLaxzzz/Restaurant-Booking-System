#!/usr/bin/env bash
# Перезапуск локальной разработки: инфра в Docker, API (монолит или шлюз nginx + микросервисы), Vite.
# Использование из корня репозитория:
#   bash scripts/dev-restart.sh              # монолит на :8080 (как раньше)
#   bash scripts/dev-restart.sh --gateway      # nginx :8080 → auth, hall, reservation, payment
#   RBS_USE_GATEWAY=1 bash scripts/dev-restart.sh
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

USE_GATEWAY=0
for arg in "$@"; do
  case "$arg" in
    --gateway|-g) USE_GATEWAY=1 ;;
  esac
done
if [[ "${RBS_USE_GATEWAY:-}" == "1" ]]; then
  USE_GATEWAY=1
fi

echo "==> Docker: postgres, redis, rabbitmq"
docker compose up -d postgres redis rabbitmq

wait_for_postgres() {
  echo "==> Ожидание готовности Postgres (pg_isready в контейнере postgres)…"
  for i in $(seq 1 120); do
    if docker compose exec -T postgres pg_isready -U rbs -d restaurant_booking >/dev/null 2>&1; then
      echo "==> Postgres принимает подключения"
      return 0
    fi
    sleep 0.5
    if [[ "$i" -eq 120 ]]; then
      echo "Таймаут: Postgres не ответил. Часто это холодный старт Docker Desktop или нехватка ресурсов."
      echo "Полностью перезапустите приложение Docker, дождитесь «Running», затем снова: bash scripts/dev-restart.sh"
      echo "Диагностика: docker compose ps && docker compose logs postgres --tail 40"
      exit 1
    fi
  done
}
wait_for_postgres

export DATABASE_URL="${DATABASE_URL:-postgres://rbs:rbs@127.0.0.1:5433/restaurant_booking?sslmode=disable}"
export REDIS_ADDR="${REDIS_ADDR:-127.0.0.1:6379}"
export RABBITMQ_URL="${RABBITMQ_URL:-amqp://guest:guest@127.0.0.1:5672/}"
export ADDR="${ADDR:-:8080}"

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

kill_port 5173

API_LOG="${TMPDIR:-/tmp}/rbs-api.log"
FE_LOG="${TMPDIR:-/tmp}/rbs-frontend.log"

if [[ "$USE_GATEWAY" -eq 1 ]]; then
  echo "==> Режим шлюза: останавливаю локальный процесс на ${ADDR}, поднимаю nginx + микросервисы"
  kill_port "${ADDR#:}"

  echo "==> Docker: auth, hall, reservation, payment, nginx (шлюз → http://localhost${ADDR})"
  docker compose up -d --build auth hall reservation payment nginx

  # healthcheck сервисов + первый запрос nginx; на холодном Docker иногда >30s
  for i in $(seq 1 200); do
    if curl -sf "http://localhost${ADDR}/health" 2>/dev/null | grep -q 'gateway ok'; then
      # BSD grep: для «ИЛИ» надёжнее grep -E (не полагаться на \| в BRE)
      if curl -sf "http://localhost${ADDR}/api/booking-defaults" 2>/dev/null | grep -Eq 'slot_minutes|deposit_percent|avg_check'; then
        echo "==> Шлюз и hall/reservation доступны: GET /health, /api/booking-defaults"
        break
      fi
    fi
    sleep 0.25
    if [[ "$i" -eq 200 ]]; then
      echo "Таймаут: шлюз или API не ответили. Проверьте: docker compose ps, docker compose logs nginx auth hall reservation"
      exit 1
    fi
  done
  rm -f "${TMPDIR:-/tmp}/rbs-api.pid"
else
  echo "==> Останавливаю nginx-шлюз, если был запущен (порт 8080 — монолит)"
  docker compose stop nginx 2>/dev/null || true

  kill_port "${ADDR#:}"

  echo "==> Restobook API (монолит) → http://localhost${ADDR} (лог: $API_LOG)"
  cd "$ROOT/backend"
  nohup env DATABASE_URL="$DATABASE_URL" REDIS_ADDR="$REDIS_ADDR" RABBITMQ_URL="$RABBITMQ_URL" ADDR="$ADDR" \
    go run ./cmd/api >>"$API_LOG" 2>&1 &
  echo $! >"${TMPDIR:-/tmp}/rbs-api.pid"

  for i in $(seq 1 120); do
    if curl -sf "http://localhost${ADDR}/health/diag" 2>/dev/null | grep -q current_database; then
      echo "==> Монолит API готов. Диагностика: curl -s http://localhost${ADDR}/health/diag"
      break
    fi
    sleep 0.25
    if [[ "$i" -eq 120 ]]; then
      echo "Таймаут: монолит не ответил JSON с current_database. См. $API_LOG"
      echo "---- последние строки $API_LOG ----"
      tail -40 "$API_LOG" 2>/dev/null || true
      echo "Подсказка: для микросервисов: bash scripts/dev-restart.sh --gateway"
      exit 1
    fi
  done
fi

echo "==> Restobook UI (Vite) → http://localhost:5173 (лог: $FE_LOG)"
cd "$ROOT/frontend"
nohup npm run dev >>"$FE_LOG" 2>&1 &
echo $! >"${TMPDIR:-/tmp}/rbs-frontend.pid"

echo ""
if [[ "$USE_GATEWAY" -eq 1 ]]; then
  echo "Готово (шлюз). Остановка UI: kill \$(cat ${TMPDIR:-/tmp}/rbs-frontend.pid)"
  echo "Остановка стека: docker compose stop nginx auth hall reservation payment"
else
  echo "Готово (монолит). Остановка: kill \$(cat ${TMPDIR:-/tmp}/rbs-api.pid) и kill \$(cat ${TMPDIR:-/tmp}/rbs-frontend.pid)"
fi
echo "Интеграционные тесты API: bash scripts/run-api-integration-tests.sh -v"
