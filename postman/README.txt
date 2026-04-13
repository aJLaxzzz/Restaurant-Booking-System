Импорт в Postman
-----------------
File → Import → выберите:
  Restaurant-Booking-API.postman_collection.json
  Local.postman_environment.json

Вверху справа активируйте окружение «Local».

Обязательный порядок
----------------------
1) Папка Auth → запрос Login (email/password демо из docs/DEMO_USERS.md).
   В Tests токен записывается в переменные; также есть pm.test(...) — без них Runner
   показывал бы «No tests found». На уровне коллекции добавлены общие проверки для
   каждого запроса (не 5xx, время ответа).

2) Остальные запросы: hall_id, restaurant_id и т.д. подставьте из ответов API
   (списки залов/ресторанов) или вручную в Variables окружения Local.

Запуск всей коллекции в Postman
--------------------------------
Collections → «⋯» у коллекции → Run collection → выберите запросы
(рекомендуется: сначала Login, затем остальные) → Run.

CLI (Newman) — для отчёта и CI
-------------------------------
Из корня репозитория:

  npx newman run postman/Restaurant-Booking-API.postman_collection.json \
    -e postman/Local.postman_environment.json

С HTML-отчётом:

  npx newman run postman/Restaurant-Booking-API.postman_collection.json \
    -e postman/Local.postman_environment.json \
    --reporters cli,htmlextra \
    --reporter-htmlextra-export postman-report.html

Подробнее: docs/LOADTEST_AND_GRAFANA.md (раздел Postman).
