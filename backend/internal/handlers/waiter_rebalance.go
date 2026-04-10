package handlers

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// rebalanceWaitersForRestaurantDay перераспределяет assigned_waiter_id для броней со статусом confirmed
// на указанный календарный день Europe/Moscow (YYYY-MM-DD). Столы уже в обслуживании (seated, in_service, pending_payment) не трогаем.
// Если в этот день нет официантов в waiter_work_dates — снимает назначение (NULL) у confirmed на этот день.
func (a *Handlers) rebalanceWaitersForRestaurantDay(ctx context.Context, restaurantID uuid.UUID, dateYMD string) error {
	loc := bookingLocationMoscow()
	day, err := timeParseYMDInLoc(dateYMD, loc)
	if err != nil {
		return err
	}
	dayStart := timeDateAtLoc(day, loc)
	dayEnd := dayStart.Add(24 * time.Hour)

	rows, err := a.Pool.Query(ctx, `
		SELECT user_id FROM waiter_work_dates
		WHERE restaurant_id=$1 AND work_date=$2::date
		ORDER BY user_id`,
		restaurantID, dateYMD)
	if err != nil {
		return err
	}
	var waiters []uuid.UUID
	for rows.Next() {
		var wid uuid.UUID
		if err := rows.Scan(&wid); err != nil {
			rows.Close()
			return err
		}
		waiters = append(waiters, wid)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	resRows, err := a.Pool.Query(ctx, `
		SELECT r.id FROM reservations r
		JOIN tables t ON t.id = r.table_id
		JOIN halls h ON h.id = t.hall_id
		WHERE h.restaurant_id = $1
		AND r.status = 'confirmed'
		AND r.start_time >= $2 AND r.start_time < $3
		ORDER BY r.start_time ASC, r.id ASC`,
		restaurantID, dayStart, dayEnd)
	if err != nil {
		return err
	}
	var resIDs []uuid.UUID
	for resRows.Next() {
		var id uuid.UUID
		if err := resRows.Scan(&id); err != nil {
			resRows.Close()
			return err
		}
		resIDs = append(resIDs, id)
	}
	resRows.Close()
	if err := resRows.Err(); err != nil {
		return err
	}

	if len(waiters) == 0 {
		for _, rid := range resIDs {
			if _, err := a.Pool.Exec(ctx, `UPDATE reservations SET assigned_waiter_id=NULL, updated_at=NOW() WHERE id=$1`, rid); err != nil {
				return err
			}
		}
		return nil
	}

	for i, rid := range resIDs {
		w := waiters[i%len(waiters)]
		if _, err := a.Pool.Exec(ctx, `UPDATE reservations SET assigned_waiter_id=$2, updated_at=NOW() WHERE id=$1`, rid, w); err != nil {
			return err
		}
	}
	return nil
}

func timeParseYMDInLoc(s string, loc *time.Location) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", s, loc)
}

func timeDateAtLoc(t time.Time, loc *time.Location) time.Time {
	t = t.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
}
