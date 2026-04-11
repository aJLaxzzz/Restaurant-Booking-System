# Демо-меню ресторанов (seed)

Соответствует данным в [`backend/internal/handlers/seed.go`](../backend/internal/handlers/seed.go): функции `seedMenuTrattoria`, `seedMenuLaLuna`, `seedMenuSakura`, `seedMenuBellaVista`.  
Цены в рублях (в БД хранятся в копейках).

Пути к картинкам блюд заданы **явно** в этих функциях; списки для дозаполнения пустых `image_url` совпадают с ними — см. [`demoMenuImagesForSlug` в `demo_images.go`](../backend/internal/handlers/demo_images.go).

## Где лежат файлы (рабочие папки)

Статика раздаётся с корня фронта: URL `/demo/...` → каталог [`frontend/public/demo/`](../frontend/public/demo/).

- Блюда: [`frontend/public/demo/dishes/`](../frontend/public/demo/dishes/)
- Рестораны: [`frontend/public/demo/restaurants/`](../frontend/public/demo/restaurants/)

Папка [`frontend/public/demo/photos/`](../frontend/public/demo/photos/) **не участвует в URL** — только черновик; см. [README там же](../frontend/public/demo/photos/README.md).

Чтобы заменить картинку: положите файл в `demo/dishes` или `demo/restaurants` и обновите путь в `seed.go` / `demo_images.go` (или `image_url` / `photo_url` / `extra_json.photo_gallery` в БД / через кабинет владельца).

---

## Траттория Тверская

**Slug:** `trattoria-tverskaya`

| Файл (public URL) | Блюдо | Цена, ₽ |
|-------------------|--------|---------|
| `/demo/dishes/margarita.webp` | Маргарита | 690 |
| `/demo/dishes/4cheese.webp` | Четыре сыра | 890 |
| `/demo/dishes/karbonara.jpg` | Паста карбонара | 650 |
| `/demo/dishes/tiramisu.jpg` | Тирамису | 420 |
| `/demo/dishes/lemonade.webp` | Домашний лимонад | 290 |
| `/demo/dishes/espresso.jpg` | Эспрессо | 180 |

**Категории:** Кухня → Пицца (Маргарита, Четыре сыра); Кухня (карбонара, тирамису); Бар.

---

## La Luna

**Slug:** `la-luna`

| Файл (public URL) | Блюдо | Цена, ₽ |
|-------------------|--------|---------|
| `/demo/dishes/duck.jpg` | Утиная грудка с вишнёвым соусом | 780 |
| `/demo/dishes/risotto.webp` | Ризотто с белыми грибами | 720 |
| `/demo/dishes/tartar.webp` | Тартар из лосося | 690 |
| `/demo/dishes/cupuchino.jpg` | Капучино | 220 |
| `/demo/dishes/lemonade.webp` | Домашний лимонад | 310 |

**Категории:** Европейская кухня; Напитки.

---

## Сакура Лайт

**Slug:** `sakura-lite`

| Файл (public URL) | Блюдо | Цена, ₽ |
|-------------------|--------|---------|
| `/demo/dishes/filadelfia.jpeg` | Филадельфия | 590 |
| `/demo/dishes/california.webp` | Калифорния | 520 |
| `/demo/dishes/ramen.webp` | Рамен с курицей | 480 |
| `/demo/dishes/tyahan.jpg` | Тяхан с морепродуктами | 550 |
| `/demo/dishes/miso.jpg` | Мисо-суп | 290 |

**Категории:** Роллы и суши; Горячее.

---

## Bella Vista

**Slug:** `bella-vista`

| Файл (public URL) | Блюдо | Описание (кратко) | Цена, ₽ |
|-------------------|--------|-------------------|---------|
| `/demo/dishes/bruskette.webp` | Брускетта с томатами и базиликом | хлеб, чеснок, масло | 420 |
| `/demo/dishes/vitello.webp` | Вителло тоннато | телятина, тоннато, каперсы | 690 |
| `/demo/dishes/kaprese.jpg` | Капрезе | моцарелла, томаты | 550 |
| `/demo/dishes/karbonara.jpg` | Спагетти карбонара | гуанчиале, яйцо, пекорино | 720 |
| `/demo/dishes/tailitely.jpg` | Тальятелле с белыми грибами | сливки, пармезан | 780 |
| `/demo/dishes/risotto.webp` | Ризотто с шафраном и морепродуктами | — | 890 |
| `/demo/dishes/ossobuko.webp` | Оссобуко по-милански | гремолата | 1250 |
| `/demo/dishes/fish.jpg` | Рыба дня на гриле | овощи | 980 |
| `/demo/dishes/pannakota.webp` | Панна котта с ягодами | — | 390 |
| `/demo/dishes/tiramisu.jpg` | Тирамису | маскарпоне, кофе | 450 |
| `/demo/dishes/espresso.jpg` | Эспрессо | — | 220 |
| `/demo/dishes/aperol.jpg` | Апероль-спритц | — | 490 |
| `/demo/dishes/mineralwater.webp` | Минеральная вода 0,75 л | — | 350 |

**Категории:** Закуски; Паста и ризотто; Основные блюда; Десерты; Напитки.

---

## Фото ресторанов

- **`photo_url`** — обложка в каталоге на главной (первое фото набора).
- **`extra_json.photo_gallery`** — массив URL; показывается на публичной странице ресторана. Задаётся в сиде и синхронизируется для демо-slug в [`seed_topup.go`](../backend/internal/handlers/seed_topup.go) (`syncDemoRestaurantContacts`).

Источник списков URL: [`demo_images.go`](../backend/internal/handlers/demo_images.go) (`demoRestaurantGalleryURLs`).

| Ресторан | Обложка (`photo_url`) | Галерея (`photo_gallery`) |
|----------|------------------------|---------------------------|
| Траттория Тверская | `/demo/restaurants/tratoria1.webp` | `tratoria1.webp`, `tratoria2.webp`, `tratoria3.webp` |
| La Luna | `/demo/restaurants/laluna1.webp` | `laluna1.webp`, `laluna2.jpg`, `laluna3.jpg` |
| Сакура Лайт | `/demo/restaurants/japanrest1.jpeg` | `japanrest1.jpeg`, `japanrest2.webp`, `japanrest3.jpg` |
| Bella Vista | `/demo/restaurants/bella1.webp` | `bella1.webp`, `bella2.webp`, `bella3.jpeg` |

(Имена файлов в таблице галереи — в каталоге `frontend/public/demo/restaurants/`, в БД хранятся полные пути `/demo/restaurants/...`.)

Владелец может менять обложку и галерею в кабинете: загрузка обложки (`POST /upload/restaurant-photo`), добавление в галерею (`POST /upload/restaurant-photo?target=gallery`), сохранение списка (`PUT /owner/restaurant` с `photo_gallery_urls`).

---

## Итого позиций

- **29** позиций меню + **4** ресторана (обложка + галерея из трёх снимков на демо).
