-- Полная схема БД (единственная миграция): бывшие 001–006.

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    full_name VARCHAR(255) NOT NULL,
    phone VARCHAR(20) NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'client',
    status VARCHAR(50) DEFAULT 'active',
    email_verified BOOLEAN DEFAULT FALSE,
    owner_application_status VARCHAR(32),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);

CREATE TABLE IF NOT EXISTS restaurants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    address TEXT,
    slug VARCHAR(160),
    city VARCHAR(255),
    description TEXT,
    photo_url TEXT,
    owner_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    phone TEXT,
    opens_at TEXT,
    closes_at TEXT,
    extra_json JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_restaurants_owner_one
  ON restaurants(owner_user_id) WHERE owner_user_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_restaurants_slug_unique ON restaurants(slug);

ALTER TABLE users ADD COLUMN IF NOT EXISTS restaurant_id UUID REFERENCES restaurants(id) ON DELETE SET NULL;

CREATE TABLE IF NOT EXISTS halls (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    restaurant_id UUID REFERENCES restaurants(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    layout_json JSONB DEFAULT '{"tables":[],"walls":[]}'::jsonb,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tables (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    hall_id UUID REFERENCES halls(id) ON DELETE CASCADE,
    table_number INT NOT NULL,
    capacity INT NOT NULL,
    x_coordinate DOUBLE PRECISION NOT NULL,
    y_coordinate DOUBLE PRECISION NOT NULL,
    shape VARCHAR(50) DEFAULT 'circle',
    status VARCHAR(50) DEFAULT 'available',
    block_reason TEXT,
    width DOUBLE PRECISION NOT NULL DEFAULT 56,
    height DOUBLE PRECISION NOT NULL DEFAULT 56,
    rotation_deg DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(hall_id, table_number)
);
CREATE INDEX IF NOT EXISTS idx_tables_hall_id ON tables(hall_id);
CREATE INDEX IF NOT EXISTS idx_tables_status ON tables(status);

CREATE TABLE IF NOT EXISTS reservations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    table_id UUID REFERENCES tables(id) ON DELETE RESTRICT,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ NOT NULL,
    guest_count INT NOT NULL,
    status VARCHAR(50) DEFAULT 'pending_payment',
    comment TEXT,
    created_by VARCHAR(50) DEFAULT 'client',
    assigned_waiter_id UUID REFERENCES users(id),
    seated_at TIMESTAMPTZ,
    service_started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    idempotency_key UUID,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_reservations_table_time ON reservations(table_id, start_time, end_time);
CREATE INDEX IF NOT EXISTS idx_reservations_user_id ON reservations(user_id);
CREATE INDEX IF NOT EXISTS idx_reservations_status ON reservations(status);
CREATE UNIQUE INDEX IF NOT EXISTS reservations_idempotency_key_uq ON reservations(idempotency_key) WHERE idempotency_key IS NOT NULL;

CREATE TABLE IF NOT EXISTS menu_categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    restaurant_id UUID NOT NULL REFERENCES restaurants(id) ON DELETE CASCADE,
    parent_id UUID REFERENCES menu_categories(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_menu_categories_restaurant ON menu_categories(restaurant_id);

CREATE TABLE IF NOT EXISTS menu_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    restaurant_id UUID NOT NULL REFERENCES restaurants(id) ON DELETE CASCADE,
    category_id UUID NOT NULL REFERENCES menu_categories(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    price_kopecks INT NOT NULL CHECK (price_kopecks >= 0),
    is_available BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order INT NOT NULL DEFAULT 0,
    image_url VARCHAR(1024) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_menu_items_restaurant ON menu_items(restaurant_id);
CREATE INDEX IF NOT EXISTS idx_menu_items_category ON menu_items(category_id);

CREATE TABLE IF NOT EXISTS reservation_orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reservation_id UUID NOT NULL REFERENCES reservations(id) ON DELETE CASCADE,
    status VARCHAR(32) NOT NULL DEFAULT 'open',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(reservation_id)
);

CREATE TABLE IF NOT EXISTS payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reservation_id UUID REFERENCES reservations(id) ON DELETE CASCADE,
    amount_kopecks INT NOT NULL,
    status VARCHAR(50) DEFAULT 'pending',
    idempotency_key UUID UNIQUE NOT NULL,
    gateway_payment_id VARCHAR(255),
    gateway_response JSONB,
    refund_amount_kopecks INT DEFAULT 0,
    purpose VARCHAR(32) NOT NULL DEFAULT 'deposit',
    reservation_order_id UUID REFERENCES reservation_orders(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_payments_reservation_id ON payments(reservation_id);

CREATE TABLE IF NOT EXISTS order_lines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES reservation_orders(id) ON DELETE CASCADE,
    menu_item_id UUID NOT NULL REFERENCES menu_items(id) ON DELETE RESTRICT,
    quantity INT NOT NULL CHECK (quantity > 0),
    guest_label VARCHAR(64) NOT NULL DEFAULT '',
    note TEXT,
    added_by VARCHAR(16) NOT NULL DEFAULT 'waiter',
    served_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_order_lines_order ON order_lines(order_id);

CREATE TABLE IF NOT EXISTS table_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reservation_id UUID REFERENCES reservations(id) ON DELETE CASCADE,
    staff_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    table_id UUID REFERENCES tables(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS settings (
    key VARCHAR(255) PRIMARY KEY,
    value JSONB NOT NULL
);

CREATE TABLE IF NOT EXISTS notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id),
    type VARCHAR(50) NOT NULL,
    template VARCHAR(100),
    status VARCHAR(50) DEFAULT 'sent',
    sent_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS waiter_notes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reservation_id UUID REFERENCES reservations(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    note TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS waiter_work_dates (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    work_date DATE NOT NULL,
    restaurant_id UUID NOT NULL REFERENCES restaurants(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, work_date)
);
CREATE INDEX IF NOT EXISTS idx_waiter_work_dates_restaurant ON waiter_work_dates (restaurant_id);
CREATE INDEX IF NOT EXISTS idx_waiter_work_dates_user_range ON waiter_work_dates (user_id, work_date);

CREATE TABLE IF NOT EXISTS reservation_reminder_sent (
    reservation_id UUID NOT NULL REFERENCES reservations(id) ON DELETE CASCADE,
    kind VARCHAR(64) NOT NULL,
    sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (reservation_id, kind)
);

CREATE TABLE IF NOT EXISTS restaurant_settings (
    restaurant_id UUID NOT NULL REFERENCES restaurants(id) ON DELETE CASCADE,
    key VARCHAR(255) NOT NULL,
    value JSONB NOT NULL,
    PRIMARY KEY (restaurant_id, key)
);
CREATE INDEX IF NOT EXISTS idx_restaurant_settings_restaurant ON restaurant_settings (restaurant_id);

UPDATE restaurants SET slug = 'r-' || REPLACE(id::text, '-', '')
WHERE slug IS NULL OR trim(COALESCE(slug, '')) = '';

UPDATE restaurants r
SET owner_user_id = u.id
FROM users u
WHERE u.email = 'owner@demo.ru'
  AND r.owner_user_id IS NULL
  AND NOT EXISTS (SELECT 1 FROM restaurants x WHERE x.owner_user_id = u.id)
  AND r.id = (SELECT id FROM restaurants WHERE owner_user_id IS NULL ORDER BY created_at LIMIT 1);

UPDATE users SET restaurant_id = (SELECT id FROM restaurants ORDER BY created_at LIMIT 1)
WHERE role IN ('admin', 'waiter') AND restaurant_id IS NULL;

INSERT INTO settings (key, value) VALUES
    ('avg_check_kopecks', '150000'),
    ('deposit_percent', '30'),
    ('slot_minutes', '30'),
    ('booking_open_hour', '10'),
    ('booking_close_hour', '23'),
    ('default_slot_duration_hours', '2'),
    ('refund_more_than_24h_percent', '100'),
    ('refund_12_to_24h_percent', '50'),
    ('refund_less_than_12h_percent', '0'),
    ('refund_more_than_2h_percent', '100'),
    ('refund_within_2h_percent', '0')
ON CONFLICT (key) DO NOTHING;
