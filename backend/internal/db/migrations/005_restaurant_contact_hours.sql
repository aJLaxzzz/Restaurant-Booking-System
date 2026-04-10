-- Контакты и часы для карточки ресторана / кабинета владельца
ALTER TABLE restaurants ADD COLUMN IF NOT EXISTS phone TEXT;
ALTER TABLE restaurants ADD COLUMN IF NOT EXISTS opens_at TEXT;
ALTER TABLE restaurants ADD COLUMN IF NOT EXISTS closes_at TEXT;
ALTER TABLE restaurants ADD COLUMN IF NOT EXISTS extra_json JSONB DEFAULT '{}'::jsonb;
