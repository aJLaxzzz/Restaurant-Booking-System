#!/usr/bin/env bash
# Нагрузка k6 + InfluxDB + файлы отчёта в report/loadtest/<дата-время>/:
#   environment-snapshot.txt — хост, docker, параметры BASE_URL/TOKEN (без секрета)
#   report.html — визуальный отчёт (k6-reporter)
#   aggregates.json — сводка метрик по именам
#   summary.json — полный summary-export k6
#   k6-stdout.log — вывод прогона в терминал
#
# Требования: docker compose -f deploy/loadtest/docker-compose.yml up -d
# API на хосте :8080 (или задайте BASE_URL).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
COMPOSE="$ROOT/deploy/loadtest/docker-compose.yml"
export BASE_URL="${BASE_URL:-http://host.docker.internal:8080}"
INFLUX="${K6_INFLUX_URL:-http://influxdb:8086/k6}"

OUT="$ROOT/report/loadtest/$(date -u +"%Y%m%d_%H%M%S")"
mkdir -p "$OUT"

{
  echo "# Restobook — снимок окружения перед нагрузочным прогоном k6"
  echo "Время (UTC): $(date -u +"%Y-%m-%dT%H:%M:%SZ")"
  echo ""
  echo "## Хост"
  uname -a 2>/dev/null || true
  echo ""
  echo "## Docker (кратко)"
  docker version 2>/dev/null | head -20 || echo "(docker недоступен)"
  echo ""
  echo "## Параметры прогона"
  echo "BASE_URL=$BASE_URL"
  if [ -n "${TOKEN:-}" ]; then
    echo "TOKEN=<задан, длина ${#TOKEN} символов>"
  else
    echo "TOKEN=<не задан>"
  fi
  echo "K6_INFLUX_URL=$INFLUX"
  echo ""
  echo "## Выходные файлы (после завершения k6)"
  echo "- report.html — HTML с графиками/таблицами (k6-reporter)"
  echo "- aggregates.json — агрегированные метрики и пороги"
  echo "- summary.json — полный machine-readable summary k6"
  echo "- k6-stdout.log — консольный вывод прогона"
} > "$OUT/environment-snapshot.txt"

echo "Каталог отчёта: $OUT"
echo "BASE_URL=$BASE_URL"
echo "InfluxDB out: $INFLUX"
echo "Запуск k6…"

set +e
docker compose -f "$COMPOSE" --profile load run --rm \
  -e BASE_URL \
  -e TOKEN="${TOKEN:-}" \
  -e K6_REPORT_DIR=/out \
  -v "$OUT:/out" \
  k6 \
  run --out "influxdb=$INFLUX" --summary-export=/out/summary.json /scripts/booking-load.js \
  2>&1 | tee "$OUT/k6-stdout.log"
code=${PIPESTATUS[0]}
set -e

{
  echo ""
  echo "## Завершение прогона"
  echo "k6 exit code: $code"
  echo "Файлы в каталоге:"
  ls -la "$OUT" 2>/dev/null || true
} >> "$OUT/environment-snapshot.txt"

echo ""
echo "Готово. Откройте в браузере: file://$OUT/report.html"
echo "Grafana (живые метрики): http://localhost:3001"
exit "$code"
