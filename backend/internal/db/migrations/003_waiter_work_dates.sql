-- Рабочие дни официантов (график по календарю; назначает админ заведения).
CREATE TABLE IF NOT EXISTS waiter_work_dates (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    work_date DATE NOT NULL,
    restaurant_id UUID NOT NULL REFERENCES restaurants(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, work_date)
);

CREATE INDEX IF NOT EXISTS idx_waiter_work_dates_restaurant ON waiter_work_dates (restaurant_id);
CREATE INDEX IF NOT EXISTS idx_waiter_work_dates_user_range ON waiter_work_dates (user_id, work_date);
