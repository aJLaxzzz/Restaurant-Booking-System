-- Расширение под агрегатор, меню, заказы, геометрия столов

ALTER TABLE restaurants ADD COLUMN IF NOT EXISTS slug VARCHAR(160);
ALTER TABLE restaurants ADD COLUMN IF NOT EXISTS city VARCHAR(255);
ALTER TABLE restaurants ADD COLUMN IF NOT EXISTS description TEXT;
ALTER TABLE restaurants ADD COLUMN IF NOT EXISTS photo_url TEXT;
ALTER TABLE restaurants ADD COLUMN IF NOT EXISTS owner_user_id UUID REFERENCES users(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_restaurants_owner_one
  ON restaurants(owner_user_id) WHERE owner_user_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_restaurants_slug_unique ON restaurants(slug);

ALTER TABLE users ADD COLUMN IF NOT EXISTS restaurant_id UUID REFERENCES restaurants(id) ON DELETE SET NULL;

ALTER TABLE tables ADD COLUMN IF NOT EXISTS width DOUBLE PRECISION NOT NULL DEFAULT 56;
ALTER TABLE tables ADD COLUMN IF NOT EXISTS height DOUBLE PRECISION NOT NULL DEFAULT 56;
ALTER TABLE tables ADD COLUMN IF NOT EXISTS rotation_deg DOUBLE PRECISION NOT NULL DEFAULT 0;

ALTER TABLE payments ADD COLUMN IF NOT EXISTS purpose VARCHAR(32) NOT NULL DEFAULT 'deposit';
ALTER TABLE payments ADD COLUMN IF NOT EXISTS reservation_order_id UUID;

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

CREATE TABLE IF NOT EXISTS order_lines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES reservation_orders(id) ON DELETE CASCADE,
    menu_item_id UUID NOT NULL REFERENCES menu_items(id) ON DELETE RESTRICT,
    quantity INT NOT NULL CHECK (quantity > 0),
    guest_label VARCHAR(64) NOT NULL DEFAULT '',
    note TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_order_lines_order ON order_lines(order_id);

DO $$ BEGIN
  ALTER TABLE payments
    ADD CONSTRAINT fk_payments_reservation_order
    FOREIGN KEY (reservation_order_id) REFERENCES reservation_orders(id) ON DELETE SET NULL;
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Существующие рестораны: уникальный slug
UPDATE restaurants SET slug = 'r-' || REPLACE(id::text, '-', '')
WHERE slug IS NULL OR trim(COALESCE(slug, '')) = '';

UPDATE restaurants r
SET owner_user_id = u.id
FROM users u
WHERE u.email = 'owner@demo.ru'
  AND r.owner_user_id IS NULL
  AND r.id = (SELECT id FROM restaurants WHERE owner_user_id IS NULL ORDER BY created_at LIMIT 1);

UPDATE users SET restaurant_id = (SELECT id FROM restaurants ORDER BY created_at LIMIT 1)
WHERE role IN ('admin', 'waiter') AND restaurant_id IS NULL;
