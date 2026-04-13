package handlers

import (
	"context"
	"fmt"
	"log"
)

// ResetDemoReservations — то же, что scripts/reset_reservations.sql: очистка броней (CASCADE по FK),
// сброс статусов столов occupied/selected. После перезапуска API при пустой таблице reservations
// Seed() снова вызовет seedLifeData и ensureClientDemoHasBookings.
func (a *Handlers) ResetDemoReservations(ctx context.Context) error {
	tx, err := a.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	if _, err := tx.Exec(ctx, `TRUNCATE TABLE reservations CASCADE`); err != nil {
		return fmt.Errorf("truncate reservations: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE tables
		SET status = 'available', block_reason = NULL
		WHERE status IN ('occupied', 'selected')`); err != nil {
		return fmt.Errorf("update tables: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	committed = true
	log.Println("демо: брони очищены, статусы столов сброшены (эквивалент reset_reservations.sql)")
	return nil
}
