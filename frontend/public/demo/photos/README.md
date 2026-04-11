# Папка `demo/photos` (опционально)

## Где реально лежат картинки для приложения

Сайт отдаёт статику из **`frontend/public/demo/`**:

- **[`../dishes/`](../dishes/)** — файлы блюд; в URL это `/demo/dishes/имя-файла.ext`
- **[`../restaurants/`](../restaurants/)** — фото ресторанов; в URL это `/demo/restaurants/имя-файла.ext`

Бэкенд и сид **не ссылаются** на `photos/` — только на пути выше (см. [`backend/internal/handlers/demo_images.go`](../../../../backend/internal/handlers/demo_images.go) и [`docs/MENU_DEMO.md`](../../../../docs/MENU_DEMO.md)).

## Зачем тогда `photos/`

Подпапки `photos/restaurants/` и `photos/dishes/` можно использовать как **черновик**: сюда удобно складывать скачанные файлы, затем **скопировать или переименовать** их в `../restaurants/` или `../dishes/` и при необходимости обновить пути в коде сида / в БД.

Так не возникает путаницы «положил в photos, а в меню пусто» — рабочая копия всегда в `demo/dishes` и `demo/restaurants`.

## Git

Большие бинарники часто не коммитят. При необходимости добавьте в корневой `.gitignore`:

```
frontend/public/demo/photos/**/*.jpg
frontend/public/demo/photos/**/*.webp
frontend/public/demo/photos/**/*.png
```

(оставьте `.gitkeep`, если нужны пустые папки в репозитории.)
