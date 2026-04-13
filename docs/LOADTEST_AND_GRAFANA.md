# Нагрузочное тестирование (k6 + InfluxDB + Grafana)

Стек для **отчёта**: генератор нагрузки **k6** пишет метрики в **InfluxDB 1.x**, **Grafana** строит графики (RPS, латентность, ошибки).

## 1. Поднять мониторинг

Из корня репозитория:

```bash
docker compose -f deploy/loadtest/docker-compose.yml up -d
```

- **Grafana**: http://localhost:3001 — логин `admin`, пароль из переменной `GRAFANA_ADMIN_PASSWORD` или по умолчанию `admin` (смените при первом входе).
- **InfluxDB**: порт `8086` (используется k6; БД `k6` создаётся автоматически).

## 2. Запустить API под нагрузкой

В отдельном терминале поднимите бэкенд (монолит), например:

```bash
cd backend && DATABASE_URL=postgres://rbs:rbs@127.0.0.1:5433/restaurant_booking?sslmode=disable \
  REDIS_ADDR=127.0.0.1:6379 go run ./cmd/api
```

Порт по умолчанию `:8080`.

## 3. Прогнать k6

```bash
chmod +x scripts/run-k6-loadtest.sh
bash scripts/run-k6-loadtest.sh
```

Скрипт `scripts/k6/booking-load.js` бьёт в `GET /health` и `GET /api/booking-defaults`. Параметры:

| Переменная | Назначение |
|------------|------------|
| `BASE_URL` | URL API (в Docker-скрипте по умолчанию `http://host.docker.internal:8080`) |
| `TOKEN` | Опционально: JWT для ручек с авторизацией |
| `K6_INFLUX_URL` | URL Influx для `--out` (по умолчанию `http://host.docker.internal:8086/k6`) |

Если k6 установлен локально (`brew install k6`):

```bash
export BASE_URL=http://127.0.0.1:8080
k6 run --out influxdb=http://127.0.0.1:8086/k6 scripts/k6/booking-load.js
```

## 4. Дашборд в Grafana

1. Войти в Grafana → **Connections → Data sources** — должен быть **k6-influxdb** (InfluxDB, БД `k6`).
2. **Dashboards → Import** → в поле ID ввести **2587** (официальный дашборд k6 для InfluxDB) или **14763** (альтернатива; при несовпадении схемы используйте 2587).
3. Выбрать datasource **k6-influxdb** → Import.
4. В отчёт: скриншоты панелей (VUs, RPS, latency p95, ошибки), описание сценария из `booking-load.js` (стадии ramp-up/ramp-down).

При пустом Influx сначала выполните прогон k6, затем обновите дашборд.

## 5. Остановка

```bash
docker compose -f deploy/loadtest/docker-compose.yml down
```

---

# Запуск коллекции Postman

## Вариант A — интерфейс Postman

1. **Import**: File → Import → выберите `postman/Restaurant-Booking-API.postman_collection.json` и `postman/Local.postman_environment.json`.
2. Вверху справа выберите окружение **Local**.
3. Выполните запрос **Auth → Login** (во вкладке **Tests** токен обычно пишется в переменные коллекции/окружения).
4. **Runner**: Collections → «…» у коллекции → **Run collection** (или кнопка **Run**). Выберите нужные запросы, при необходимости включите **Persist responses**, нажмите **Run Restaurant-Booking-API**.

Имеет смысл задать **Iteration** (например 1) и порядок: сначала Login, затем остальные.

## Вариант B — Newman (CLI, удобно для CI и отчёта)

Установка: `npm install -g newman` или `npx newman`.

Из корня репозитория:

```bash
npx newman run postman/Restaurant-Booking-API.postman_collection.json \
  -e postman/Local.postman_environment.json \
  --reporters cli,htmlextra \
  --reporter-htmlextra-export postman-report.html
```

Отчёт откроется в `postman-report.html`. Без HTML:

```bash
npx newman run postman/Restaurant-Booking-API.postman_collection.json \
  -e postman/Local.postman_environment.json
```

Если в коллекции для Login нужны тестовые данные — подставьте `email`/`password` в окружении **Local** (Variables).

## Типичные проблемы

- **401 на всех запросах** — не выполнен Login или протух `access_token`; прогоните Login снова.
- **`hall_id` / `restaurant_id` пустые** — возьмите из ответов API (список залов / ресторанов) и пропишите в Environment.

Подробнее кратко: `postman/README.txt`.
