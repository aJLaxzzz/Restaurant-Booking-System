#!/usr/bin/env bash
# Сброс броней без psql: вызывает backend/cmd/reset-reservations (тот же SQL, что в коде).
# Альтернатива: из каталога backend — go run ./cmd/reset-reservations
# Или при старте монолита: RESET_DEMO_RESERVATIONS=1 или флаг -reset-reservations
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
export DATABASE_URL="${DATABASE_URL:-postgres://rbs:rbs@127.0.0.1:5433/restaurant_booking?sslmode=disable}"
cd "$ROOT/backend"
go run ./cmd/reset-reservations
echo "Перезапустите backend, если он уже был запущен — Seed() дозаполнит демо-брони при пустой таблице reservations."
