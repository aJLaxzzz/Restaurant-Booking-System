package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// handleAdminWaitersList — официанты ресторана админа и их брони на сегодня (календарный день Europe/Moscow), где они назначены ответственными.
func (a *Handlers) handleAdminWaitersList(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	rid, ok := a.mustRestaurant(w, r, u)
	if !ok || u.Role != "admin" {
		return
	}

	loc, tzErr := time.LoadLocation("Europe/Moscow")
	if tzErr != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	dayEnd := dayStart.Add(24 * time.Hour)

	rows, err := a.Pool.Query(r.Context(), `
		SELECT u.id, u.email, u.full_name, u.phone,
			COALESCE((
				SELECT json_agg(json_build_object(
					'id', r.id,
					'table_number', t.table_number,
					'status', r.status,
					'start_time', r.start_time
				) ORDER BY r.start_time)
				FROM reservations r
				JOIN tables t ON t.id = r.table_id
				JOIN halls h ON h.id = t.hall_id
				WHERE r.assigned_waiter_id = u.id
				AND h.restaurant_id = $1
				AND r.start_time >= $2 AND r.start_time < $3
				AND r.status IN ('confirmed','seated','in_service','pending_payment')
			), '[]'::json)
		FROM users u
		WHERE u.restaurant_id = $1 AND u.role = 'waiter'
		ORDER BY u.full_name`, rid, dayStart, dayEnd)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	defer rows.Close()

	out := make([]map[string]any, 0)
	for rows.Next() {
		var id uuid.UUID
		var email, fullName, phone string
		var raw []byte
		if err := rows.Scan(&id, &email, &fullName, &phone, &raw); err != nil {
			a.err(w, http.StatusInternalServerError, "БД")
			return
		}
		var today []any
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &today)
		}
		out = append(out, map[string]any{
			"id": id.String(), "email": email, "full_name": fullName, "phone": phone,
			"today_reservations": today,
		})
	}
	a.json(w, http.StatusOK, out)
}
