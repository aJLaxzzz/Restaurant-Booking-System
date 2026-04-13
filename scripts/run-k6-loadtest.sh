#!/usr/bin/env bash
# Запуск k6 в Docker: нагрузка на API, запись в InfluxDB (docker compose из deploy/loadtest).
# Предварительно: docker compose -f deploy/loadtest/docker-compose.yml up -d
# API должен быть доступен с хоста (например монолит :8080).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# С хоста Mac/Win Docker: API на машине → host.docker.internal
export BASE_URL="${BASE_URL:-http://host.docker.internal:8080}"
INFLUX="${K6_INFLUX_URL:-http://host.docker.internal:8086/k6}"

echo "BASE_URL=$BASE_URL"
echo "InfluxDB out: $INFLUX"
echo "Запуск k6…"

docker run --rm -i \
  --add-host=host.docker.internal:host-gateway \
  -e BASE_URL \
  -e TOKEN="${TOKEN:-}" \
  -v "$ROOT/scripts/k6:/scripts:ro" \
  grafana/k6 run \
  --out "influxdb=$INFLUX" \
  /scripts/booking-load.js

echo "Готово. Откройте Grafana http://localhost:3001 (admin / пароль из GRAFANA_ADMIN_PASSWORD или admin)."
echo "Импортируйте дашборд k6: Create → Import → ID 2587 → datasource k6-influxdb."
