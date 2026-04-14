-- Координаты ресторанов для карты (Leaflet/OSM).

ALTER TABLE restaurants ADD COLUMN IF NOT EXISTS lat DOUBLE PRECISION;
ALTER TABLE restaurants ADD COLUMN IF NOT EXISTS lng DOUBLE PRECISION;

CREATE INDEX IF NOT EXISTS idx_restaurants_city ON restaurants(city);

