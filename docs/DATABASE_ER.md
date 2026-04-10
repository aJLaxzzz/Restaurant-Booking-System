# Модель БД (ER-диаграмма)

Сводка по схеме из единственной миграции [`001_initial_schema.sql`](backend/internal/db/migrations/001_initial_schema.sql). В Mermaid-нотации связи по внешним ключам.

```mermaid
erDiagram
  users ||--o{ reservations : books_as_client
  users ||--o{ restaurants : owns
  users ||--o{ waiter_work_dates : schedule
  users ||--o{ waiter_notes : writes
  users ||--o{ notifications : receives
  users ||--o{ table_assignments : staff

  restaurants ||--o{ halls : contains
  restaurants ||--o{ menu_categories : has
  restaurants ||--o{ menu_items : has
  restaurants ||--o{ restaurant_settings : overrides
  restaurants ||--o{ waiter_work_dates : scope

  halls ||--o{ tables : layout

  tables ||--o{ reservations : booked
  tables ||--o{ table_assignments : links

  reservations ||--o{ payments : deposit
  reservations ||--o| reservation_orders : order
  reservations ||--o{ waiter_notes : notes
  reservations ||--o{ reservation_reminder_sent : reminders
  reservations ||--o{ table_assignments : assigns

  reservation_orders ||--o{ order_lines : lines
  menu_items ||--o{ order_lines : item

  users {
    uuid id PK
    string email UK
    string role
    uuid restaurant_id FK
  }

  restaurants {
    uuid id PK
    uuid owner_user_id FK
    string slug UK
  }

  restaurant_settings {
    uuid restaurant_id PK_FK
    string key PK
    jsonb value
  }

  settings {
    string key PK
    jsonb value
  }

  halls {
    uuid id PK
    uuid restaurant_id FK
    jsonb layout_json
  }

  tables {
    uuid id PK
    uuid hall_id FK
    int table_number
  }

  reservations {
    uuid id PK
    uuid table_id FK
    uuid user_id FK
    timestamptz start_time
    timestamptz end_time
    string status
    uuid assigned_waiter_id FK
    uuid idempotency_key
  }

  payments {
    uuid id PK
    uuid reservation_id FK
  }

  menu_categories {
    uuid id PK
    uuid restaurant_id FK
    uuid parent_id FK
  }

  menu_items {
    uuid id PK
    uuid restaurant_id FK
    uuid category_id FK
  }

  reservation_orders {
    uuid id PK
    uuid reservation_id FK
  }

  order_lines {
    uuid id PK
    uuid order_id FK
    uuid menu_item_id FK
  }

  waiter_work_dates {
    uuid user_id PK_FK
    date work_date PK
    uuid restaurant_id FK
  }

  waiter_notes {
    uuid id PK
    uuid reservation_id FK
    uuid user_id FK
  }

  table_assignments {
    uuid id PK
    uuid reservation_id FK
    uuid table_id FK
    uuid staff_user_id FK
  }

  notifications {
    uuid id PK
    uuid user_id FK
  }

  reservation_reminder_sent {
    uuid reservation_id PK_FK
    string kind PK
  }
```

## Примечания

- Глобальные дефолты бронирования и политик — таблица `settings`; переопределения на заведение — `restaurant_settings`.
- `users.restaurant_id` связывает админа/официанта с рестораном; владелец привязан через `restaurants.owner_user_id` (уникально один ресторан на владельца).
- `reservation_reminder_sent` и дополнительные поля заказа (`order_lines.add_by`, `served_at`) заданы в `004_plan_features.sql`.
