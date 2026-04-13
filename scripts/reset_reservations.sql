-- Полная очистка броней и всех связанных строк (счета, платежи по брони, заметки официанта и т.д.).
-- Столы в зале не удаляются; «залипшие» occupied/selected сбрасываются в available.
--
-- Предпочтительно без psql: backend/internal/handlers/ResetDemoReservations + `go run ./cmd/reset-reservations`
-- или старт API с RESET_DEMO_RESERVATIONS=1 / флаг -reset-reservations.
-- После сброса перезапустите API: при COUNT(reservations)=0 Seed() вызовет seedLifeData и демо-брони.
BEGIN;

TRUNCATE TABLE reservations CASCADE;

UPDATE tables
SET status = 'available', block_reason = NULL
WHERE status IN ('occupied', 'selected');

COMMIT;
