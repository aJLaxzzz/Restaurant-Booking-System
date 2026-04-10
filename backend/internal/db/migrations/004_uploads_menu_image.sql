-- Фото блюд в меню
ALTER TABLE menu_items ADD COLUMN IF NOT EXISTS image_url VARCHAR(1024) NOT NULL DEFAULT '';
