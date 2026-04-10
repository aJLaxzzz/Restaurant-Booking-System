-- Одноразовая чистка дубликатов Bella Vista (канонический slug: bella-vista).
-- order_lines.menu_item_id → menu_items с ON DELETE RESTRICT: снимаем строки заказов,
-- иначе каскад при удалении ресторана не сможет удалить позиции меню.
-- reservations.table_id → tables с ON DELETE RESTRICT: снимаем брони по столам зала дубликата,
-- иначе каскад не сможет удалить tables.

BEGIN;

WITH doomed AS (
  SELECT r.id
  FROM restaurants r
  WHERE EXISTS (SELECT 1 FROM restaurants x WHERE x.slug = 'bella-vista')
    AND r.slug IS DISTINCT FROM 'bella-vista'
    AND (
      LOWER(TRIM(r.slug)) LIKE 'bella-vista%'
      OR LOWER(TRIM(r.name)) LIKE '%bella%vista%'
      OR LOWER(TRIM(r.name)) = 'bella vista'
    )
)
DELETE FROM order_lines ol
USING menu_items mi, doomed d
WHERE ol.menu_item_id = mi.id
  AND mi.restaurant_id = d.id;

WITH doomed AS (
  SELECT r.id
  FROM restaurants r
  WHERE EXISTS (SELECT 1 FROM restaurants x WHERE x.slug = 'bella-vista')
    AND r.slug IS DISTINCT FROM 'bella-vista'
    AND (
      LOWER(TRIM(r.slug)) LIKE 'bella-vista%'
      OR LOWER(TRIM(r.name)) LIKE '%bella%vista%'
      OR LOWER(TRIM(r.name)) = 'bella vista'
    )
),
doomed_tables AS (
  SELECT t.id
  FROM tables t
  INNER JOIN halls h ON h.id = t.hall_id
  WHERE h.restaurant_id IN (SELECT id FROM doomed)
)
DELETE FROM reservations res
WHERE res.table_id IN (SELECT id FROM doomed_tables);

DELETE FROM restaurants r
WHERE EXISTS (SELECT 1 FROM restaurants x WHERE x.slug = 'bella-vista')
  AND r.slug IS DISTINCT FROM 'bella-vista'
  AND (
    LOWER(TRIM(r.slug)) LIKE 'bella-vista%'
    OR LOWER(TRIM(r.name)) LIKE '%bella%vista%'
    OR LOWER(TRIM(r.name)) = 'bella vista'
  );

COMMIT;
