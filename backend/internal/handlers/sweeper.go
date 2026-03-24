package handlers

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
)

func (a *Handlers) runPendingPaymentSweeper() {
	t := time.NewTicker(1 * time.Minute)
	go func() {
		for range t.C {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			a.expirePendingPayments(ctx)
			cancel()
		}
	}()
}

func (a *Handlers) expirePendingPayments(ctx context.Context) {
	iv := fmt.Sprintf("%d minutes", int(a.Cfg.PaymentPendingTTL.Minutes()))
	rows, err := a.Pool.Query(ctx, `
		SELECT r.id, r.table_id FROM reservations r
		WHERE r.status = 'pending_payment'
		AND r.created_at < NOW() - $1::interval`, iv)
	if err != nil {
		log.Printf("sweeper query: %v", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var rid, tid uuid.UUID
		if err := rows.Scan(&rid, &tid); err != nil {
			continue
		}
		_, _ = a.Pool.Exec(ctx, `UPDATE reservations SET status='cancelled_by_client', updated_at=NOW() WHERE id=$1`, rid)
		_, _ = a.Pool.Exec(ctx, `UPDATE payments SET status='failed', updated_at=NOW() WHERE reservation_id=$1 AND status='pending'`, rid)
		_ = a.RDB.Del(ctx, "table:"+tid.String()+":lock").Err()
	}
}
