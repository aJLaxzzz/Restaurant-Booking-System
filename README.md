# Restaurant Booking System (Bella Vista)

Онлайн-бронирование столов с интерактивной схемой зала, ролями (гость, официант, администратор, владелец), депозитом и опциональной микросервисной архитектурой за **Nginx**.

---

## Возможности

| Роль | Что доступно |
|------|----------------|
| **Гость** (`client`) | Визард: дата и время → свободные столы на карте → комментарий и бронь → оплата. Раздел **«Мои брони»**. |
| **Владелец** (`owner`) | То же бронирование (тест), **«Мои брони»**, кабинет владельца (график загрузки, настройки, XLSX), панель броней как у админа, редактор схемы зала. |
| **Администратор** (`admin`) | **Нет** гостевого бронирования и **«Мои брони»**: панель броней, ручная бронь, чек-ин, редактор схемы и **блокировка столов**. |
| **Официант** (`waiter`) | Столы и статусы обслуживания, **заметки** к брони; бронирование как у гостя отключено. |

---

## Быстрый старт (монолит, без Docker)

Подходит для разработки: один процесс API и фронт на Vite.

### 1. База и Redis

Нужны **PostgreSQL** и **Redis**. Проще всего поднять только инфраструктуру:

```bash
docker compose up -d postgres redis rabbitmq
```

По умолчанию Postgres на хосте: `localhost:5433` (см. `docker-compose.yml`).

### 2. Backend

```bash
cd backend
export DATABASE_URL="postgres://rbs:rbs@localhost:5433/restaurant_booking?sslmode=disable"
export REDIS_ADDR="localhost:6379"
# опционально RabbitMQ для событий:
export RABBITMQ_URL="amqp://guest:guest@localhost:5672/"
go run ./cmd/api
```

API: **http://127.0.0.1:8080** (если не задан `ADDR`).

- `GET /health` — проверка живости  
- При первом запуске выполняются миграции и **демо-данные** (сид), если таблицы пустые.

### 3. Frontend

```bash
cd frontend
npm install
npm run dev
```

Откройте **http://localhost:5173**. Vite проксирует `/api` и `/ws` на `http://127.0.0.1:8080` (см. `vite.config.ts`).

**Не запускайте** второй сервер на порту 8080 одновременно с Nginx из Docker (см. ниже).

---

## Полный стек в Docker (микросервисы + Nginx)

Собирает отдельные сервисы `auth`, `hall`, `reservation`, `payment` и **nginx** как единую точку входа на порту **8080**.

```bash
docker compose up --build
```

- **Шлюз (фронт в dev):** `http://localhost:8080` — укажите этот адрес в proxy Vite или откройте API напрямую.  
- **RabbitMQ UI:** http://localhost:15672 (guest/guest)  
- **Postgres:** `localhost:5433`  

Переменные окружения для сервисов заданы в `docker-compose.yml` (`DATABASE_URL`, `HALL_SERVICE_URL`, `JWT_SECRET`, …).

Фронтенд в режиме разработки:

```bash
cd frontend
# прокси на шлюз:
# в vite.config.ts target: 'http://127.0.0.1:8080'
npm run dev
```

---

## Отдельные бинарники (без Docker)

Запускайте в разных терминалах, с **разными** `ADDR` и общей БД/Redis:

| Команда | Порт по умолчанию |
|---------|-------------------|
| `go run ./cmd/auth` | :8081 |
| `go run ./cmd/hall` | :8082 |
| `go run ./cmd/reservation` | :8083 |
| `go run ./cmd/payment` | :8084 |

Для `reservation` и `payment` задайте рассылку событий в зал:

```bash
export HALL_SERVICE_URL="http://127.0.0.1:8082"
```

Nginx из репозитория (`nginx/nginx.conf`) можно использовать локально, подставив свои upstream-порты.

---

## Демо-аккаунты

После сида (пустая БД) создаются пользователи. Пароль у всех демо:

**`Password1`**

| Email | Роль |
|-------|------|
| `client@demo.ru` | гость |
| `client1@demo.ru` … `client15@demo.ru` | гости |
| `admin@demo.ru` | администратор |
| `owner@demo.ru` | владелец |
| `waiter@demo.ru`, `waiter2@demo.ru` | официанты |

---

## Важные переменные окружения (backend)

| Переменная | Назначение |
|------------|------------|
| `ADDR` | Адрес прослушивания (например `:8080`) |
| `DATABASE_URL` | PostgreSQL |
| `REDIS_ADDR` | Redis |
| `RABBITMQ_URL` | Очередь событий (можно пусто) |
| `JWT_SECRET` | Подпись JWT (≥32 символов для прода) |
| `INTERNAL_SECRET` | Заголовок `X-Internal-Secret` для вызова hall из других сервисов |
| `HALL_SERVICE_URL` | URL сервиса зала для HTTP-рассылки WS (в монолите не нужен) |
| `FRONTEND_ORIGIN` | CORS (например `http://localhost:5173`) |
| `PUBLIC_APP_URL` | Редиректы после оплаты |
| `STRIPE_*` / `YOOKASSA_*` | Реальные платежи (опционально) |

---

## Структура репозитория

```
backend/          # Go: cmd/api (монолит), cmd/auth|hall|reservation|payment, internal/handlers, migrations
frontend/         # React + Vite + Konva
nginx/            # Конфиг шлюза для docker-compose
docker-compose.yml
```

---

## Полезные команды

```bash
# Backend
cd backend && go build ./...

# Frontend
cd frontend && npm run build

# Только Postgres + Redis
docker compose up -d postgres redis
```

---

## Лицензия и поддержка

Учебный / демо-проект. Перед продакшеном смените секреты, настройте HTTPS, лимиты и мониторинг.
