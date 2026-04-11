# Развёртывание

## Локальная разработка (без Docker)

Как в репозитории по умолчанию:

1. **Postgres / Redis / RabbitMQ** — поднять локально или через отдельные контейнеры. В [backend/internal/config/config.go](../backend/internal/config/config.go) заданы значения по умолчанию (`DATABASE_URL` с портом `5433`, `REDIS_ADDR`, `RABBITMQ_URL`).
2. **Бэкенд (монолит):** из каталога `backend`:

   ```bash
   go run ./cmd/api
   ```

3. **Фронтенд:** из каталога `frontend`:

   ```bash
   npm install
   npm run dev
   ```

   [frontend/vite.config.ts](../frontend/vite.config.ts) проксирует `/api` и `/ws` на `http://localhost:8080`. Статика демо отдаётся самим Vite из [frontend/public/demo](../frontend/public/demo) по путям `/demo/...`.

### Картинки меню и обложки (`/demo/...`)

Ответы API содержат относительные URL вида `/demo/dishes/...`. Браузер запрашивает их с **того же origin**, что и SPA.

- В dev это `http://localhost:5173` — файлы из `public/demo` доступны автоматически.
- Если фронт собран и отдаётся **без** каталога `demo`, картинки пропадут. Нужно либо копировать `frontend/public/demo` в корень статики рядом с `index.html`, либо задать при сборке **`VITE_PUBLIC_ORIGIN`** — базовый URL, с которого отдаются `/demo` и при необходимости `/api/files` (см. [frontend/src/utils/publicAssetUrl.ts](../frontend/src/utils/publicAssetUrl.ts)).

Пример сборки с другим origin:

```bash
VITE_PUBLIC_ORIGIN=https://cdn.example.com npm run build
```

Пустая переменная — поведение как раньше (относительные пути).

---

## Docker: монолит API + nginx (рекомендуемый вариант)

В корне репозитория:

```bash
docker compose up --build
```

- **Сайт и API через один origin:** `http://localhost` (порт **80**). Контейнер `web` (nginx) отдаёт SPA, каталог **`/demo`**, проксирует `/api` и `/ws` на сервис `api`.
- **Postgres** наружу на **5433** (как в дефолтном `DATABASE_URL` для локального Go).
- Загрузки владельцев хранятся в volume `rbs_uploads`.

Образ API собирается из [backend/Dockerfile](../backend/Dockerfile) с `SERVICE=api`. Образ фронта — [frontend/Dockerfile](../frontend/Dockerfile) (multi-stage: Node build + nginx).

Переменные для продакшена задайте через `environment` / `.env` для сервиса `api` в [docker-compose.yml](../docker-compose.yml): в первую очередь **`JWT_SECRET`**, при необходимости **`DATABASE_URL`**, платежи (**`YOOKASSA_*`**, **`STRIPE_*`**), **`FRONTEND_ORIGIN`** / **`PUBLIC_APP_URL`** (URL, с которого открывают сайт).

---

## Docker: микросервисы (пример)

Отдельные бинарники: `cmd/auth`, `cmd/hall`, `cmd/reservation`, `cmd/payment`, `cmd/notify-worker` (см. [backend/internal/handlers/app.go](../backend/internal/handlers/app.go) — `RouterAuth`, `RouterHall`, и т.д.).

Файл [docker-compose.microservices.yml](../docker-compose.microservices.yml) поднимает Postgres, Redis, RabbitMQ, пять Go-сервисов и **gateway** (nginx) на порту **8090**.

```bash
docker compose -f docker-compose.microservices.yml up --build
curl -s http://localhost:8090/api/health
```

Маршрутизация на шлюзе описана в [deploy/nginx-gateway-microservices.conf](../deploy/nginx-gateway-microservices.conf):

| Префикс | Сервис |
|-----------|--------|
| `/api/auth/` | auth |
| `/api/payments/` | payment |
| `/api/halls`, `/api/booking-defaults`, `/api/restaurants` | hall |
| `/ws` | hall (WebSocket) |
| `/api/files/` | reservation |
| остальное `/api/` | reservation |

**Важно:** сервис `reservation` должен знать адрес hall для внутренних вызовов: **`HALL_SERVICE_URL=http://hall:8082`** (уже в compose).

Фронт в этом режиме удобно запускать локально с Vite, изменив proxy target на `http://localhost:8090`, либо собрать отдельный nginx, который отдаёт статику и проксирует `/api` на gateway.

`notify-worker` не входит в публичный API; слушает `:8085` только для `/health`.

---

## Образы бэкенда по одному Dockerfile

[backend/Dockerfile](../backend/Dockerfile) принимает аргумент **`SERVICE`**:

```bash
docker build --build-arg SERVICE=auth -t rbs-auth ./backend
docker build --build-arg SERVICE=hall -t rbs-hall ./backend
# … reservation, payment, notify-worker, api
```
