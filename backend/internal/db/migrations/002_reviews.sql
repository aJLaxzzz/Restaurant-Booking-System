-- Отзывы/рейтинги после оплаты счёта (tab).

CREATE TABLE IF NOT EXISTS reviews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reservation_id UUID NOT NULL REFERENCES reservations(id) ON DELETE CASCADE,
    restaurant_id UUID NOT NULL REFERENCES restaurants(id) ON DELETE CASCADE,
    client_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    waiter_id UUID REFERENCES users(id) ON DELETE SET NULL,
    rating_restaurant INT NOT NULL CHECK (rating_restaurant >= 1 AND rating_restaurant <= 5),
    rating_waiter INT CHECK (rating_waiter IS NULL OR (rating_waiter >= 1 AND rating_waiter <= 5)),
    comment TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (reservation_id, client_id)
);

CREATE INDEX IF NOT EXISTS idx_reviews_restaurant ON reviews(restaurant_id);
CREATE INDEX IF NOT EXISTS idx_reviews_waiter ON reviews(waiter_id);
CREATE INDEX IF NOT EXISTS idx_reviews_client ON reviews(client_id);

