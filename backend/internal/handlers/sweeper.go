package handlers

import (
	"context"
	"encoding/json"
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
			a.expireNoShows(ctx)
			cancel()
		}
	}()
}

func (a *Handlers) noShowGraceMinutes(ctx context.Context) int {
	var raw []byte
	err := a.Pool.QueryRow(ctx, `SELECT value FROM settings WHERE key=$1`, "no_show_grace_minutes").Scan(&raw)
	if err != nil {
		return 20
	}
	var m struct {
		Minutes int `json:"minutes"`
	}
	if json.Unmarshal(raw, &m) != nil || m.Minutes < 1 {
		return 20
	}
	return m.Minutes
}

func (a *Handlers) expireNoShows(ctx context.Context) {
	grace := a.noShowGraceMinutes(ctx)
	rows, err := a.Pool.Query(ctx, `
		UPDATE reservations SET status='no_show', updated_at=NOW()
		WHERE status='confirmed'
		AND seated_at IS NULL
		AND service_started_at IS NULL
		AND start_time + ($1::int * INTERVAL '1 minute') < NOW()
		RETURNING id, table_id`, grace)
	if err != nil {
		log.Printf("no-show sweeper: %v", err)
		return
	}
	defer rows.Close()
	var n int
	for rows.Next() {
		var rid, tid uuid.UUID
		if err := rows.Scan(&rid, &tid); err != nil {
			continue
		}
		n++
		_, _ = a.Pool.Exec(ctx, `UPDATE tables SET status='available', updated_at=NOW() WHERE id=$1`, tid)
		var hallID uuid.UUID
		_ = a.Pool.QueryRow(ctx, `SELECT hall_id FROM tables WHERE id=$1`, tid).Scan(&hallID)
		ts := time.Now().UTC().Format(time.RFC3339)
		a.emitHallEvent(hallID, map[string]any{
			"event": "table.freed", "table_id": tid.String(),
			"reservation_id": rid.String(), "reason": "no_show", "timestamp": ts,
		})
		a.emitHallEvent(hallID, map[string]any{
			"event": "reservation.status_changed", "reservation_id": rid.String(),
			"status": "no_show", "timestamp": ts,
		})
	}
	if n > 0 {
		log.Printf("no-show sweeper: помечено неявок: %d", n)
	}
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
