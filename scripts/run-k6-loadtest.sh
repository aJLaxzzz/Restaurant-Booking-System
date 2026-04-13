#!/usr/bin/env bash
# Нагрузка k6 + запись метрик в InfluxDB из того же docker compose (сеть compose → нет connection refused на 8086).
# Перед первым запуском: docker compose -f deploy/loadtest/docker-compose.yml up -d
# API на машине (монолит :8080) — с контейнера k6: host.docker.internal.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
COMPOSE="$ROOT/deploy/loadtest/docker-compose.yml"
export BASE_URL="${BASE_URL:-http://host.docker.internal:8080}"
# Внутри compose-сети InfluxDB — сервис influxdb (не localhost / host.docker.internal).
INFLUX="${K6_INFLUX_URL:-http://influxdb:8086/k6}"

echo "BASE_URL=$BASE_URL"
echo "InfluxDB out: $INFLUX"
echo "Запуск k6 (docker compose run k6)…"

# Опционально: K6_REPORT_DIR=/out и том с хоста — см. run-k6-loadtest-with-report.sh
EXTRA_ENV=()
EXTRA_VOL=()
if [ -n "${K6_REPORT_DIR:-}" ]; then
  EXTRA_ENV+=(-e "K6_REPORT_DIR=${K6_REPORT_DIR}")
fi
if [ -n "${K6_REPORT_MOUNT:-}" ]; then
  EXTRA_VOL+=(-v "${K6_REPORT_MOUNT}")
fi

docker compose -f "$COMPOSE" --profile load run --rm \
  -e BASE_URL \
  -e TOKEN="${TOKEN:-}" \
  "${EXTRA_ENV[@]}" \
  "${EXTRA_VOL[@]}" \
  k6 \
  run --out "influxdb=$INFLUX" /scripts/booking-load.js

echo "Готово. Grafana: http://localhost:3001 (admin / пароль из GRAFANA_ADMIN_PASSWORD или admin)."
echo "Импорт дашборда k6: Create → Import → ID 2587 → datasource k6-influxdb."
