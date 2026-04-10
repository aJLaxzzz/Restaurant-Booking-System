package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (a *Handlers) handleWaiterShifts(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	from := time.Now().Truncate(24 * time.Hour)
	to := from.AddDate(0, 0, 14)
	if f := r.URL.Query().Get("from"); f != "" {
		if t, err := time.ParseInLocation("2006-01-02", f, time.Local); err == nil {
			from = t
		}
	}
	if t := r.URL.Query().Get("to"); t != "" {
		if parsed, err := time.ParseInLocation("2006-01-02", t, time.Local); err == nil {
			to = parsed.Add(24 * time.Hour)
		}
	}

	var out []map[string]any
	appendSingle := func(rows interface {
		Close()
		Next() bool
		Scan(dest ...any) error
		Err() error
	}) error {
		defer rows.Close()
		for rows.Next() {
			var d time.Time
			var cnt, guests int
			var st, en time.Time
			if err := rows.Scan(&d, &cnt, &guests, &st, &en); err != nil {
				return err
			}
			out = append(out, map[string]any{
				"date": d.Format("2006-01-02"), "slots": cnt, "guests_total": guests,
				"first_start": st, "last_end": en,
			})
		}
		return rows.Err()
	}
	appendMulti := func(rows interface {
		Close()
		Next() bool
		Scan(dest ...any) error
		Err() error
	}) error {
		defer rows.Close()
		for rows.Next() {
			var d time.Time
			var wname string
			var cnt, guests int
			var st, en time.Time
			if err := rows.Scan(&d, &wname, &cnt, &guests, &st, &en); err != nil {
				return err
			}
			out = append(out, map[string]any{
				"date": d.Format("2006-01-02"), "waiter_name": wname, "slots": cnt,
				"guests_total": guests, "first_start": st, "last_end": en,
			})
		}
		return rows.Err()
	}

	var err error
	switch u.Role {
	case "waiter":
		rows, qerr := a.Pool.Query(r.Context(), `
			SELECT date_trunc('day', r.start_time)::date,
			       COUNT(*)::int,
			       COALESCE(SUM(r.guest_count),0)::int,
			       MIN(r.start_time), MAX(r.end_time)
			FROM reservations r
			WHERE r.assigned_waiter_id = $1
			AND r.start_time >= $2 AND r.start_time < $3
			AND r.status IN ('confirmed','seated','in_service','pending_payment')
			GROUP BY 1 ORDER BY 1`, u.ID, from, to)
		if qerr != nil {
			a.err(w, http.StatusInternalServerError, "БД")
			return
		}
		err = appendSingle(rows)
	default:
		if wid := r.URL.Query().Get("waiter_id"); wid != "" {
			widUUID, perr := uuid.Parse(wid)
			if perr != nil {
				a.err(w, http.StatusBadRequest, "waiter_id")
				return
			}
			rows, qerr := a.Pool.Query(r.Context(), `
				SELECT date_trunc('day', r.start_time)::date,
				       COUNT(*)::int,
				       COALESCE(SUM(r.guest_count),0)::int,
				       MIN(r.start_time), MAX(r.end_time)
				FROM reservations r
				WHERE r.assigned_waiter_id = $1
				AND r.start_time >= $2 AND r.start_time < $3
				AND r.status IN ('confirmed','seated','in_service','pending_payment')
				GROUP BY 1 ORDER BY 1`, widUUID, from, to)
			if qerr != nil {
				a.err(w, http.StatusInternalServerError, "БД")
				return
			}
			err = appendSingle(rows)
		} else {
			rows, qerr := a.Pool.Query(r.Context(), `
				SELECT date_trunc('day', r.start_time)::date,
				       u.full_name,
				       COUNT(*)::int,
				       COALESCE(SUM(r.guest_count),0)::int,
				       MIN(r.start_time), MAX(r.end_time)
				FROM reservations r
				JOIN users u ON u.id = r.assigned_waiter_id
				WHERE r.assigned_waiter_id IS NOT NULL
				AND r.start_time >= $1 AND r.start_time < $2
				AND r.status IN ('confirmed','seated','in_service','pending_payment')
				GROUP BY 1, 2 ORDER BY 1, 2`, from, to)
			if qerr != nil {
				a.err(w, http.StatusInternalServerError, "БД")
				return
			}
			err = appendMulti(rows)
		}
	}
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	a.json(w, http.StatusOK, out)
}

func (a *Handlers) handleWaiterNote(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	var body struct {
		ReservationID uuid.UUID `json:"reservation_id"`
		Note          string    `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Note == "" {
		a.err(w, http.StatusBadRequest, "данные")
		return
	}
	var assigned sql.NullString
	err := a.Pool.QueryRow(r.Context(), `
		SELECT assigned_waiter_id::text FROM reservations WHERE id=$1`, body.ReservationID).Scan(&assigned)
	if err == pgx.ErrNoRows {
		a.err(w, http.StatusNotFound, "бронь")
		return
	}
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	var assignedID uuid.UUID
	if assigned.Valid && assigned.String != "" {
		assignedID, err = uuid.Parse(assigned.String)
		if err != nil {
			a.err(w, http.StatusInternalServerError, "БД")
			return
		}
	}
	allowed := u.Role == "admin" || u.Role == "owner"
	if !allowed && (!assigned.Valid || assigned.String == "" || assignedID != u.ID) {
		a.err(w, http.StatusForbidden, "не ваш стол")
		return
	}
	var nid uuid.UUID
	err = a.Pool.QueryRow(r.Context(), `
		INSERT INTO waiter_notes (reservation_id, user_id, note)
		VALUES ($1,$2,$3) RETURNING id`,
		body.ReservationID, u.ID, body.Note).Scan(&nid)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "не сохранено")
		return
	}
	a.json(w, http.StatusCreated, map[string]string{"id": nid.String()})
}

func (a *Handlers) handleWaiterTables(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	ridScope, err := a.restaurantUUIDForUser(r.Context(), u.ID, u.Role)
	if err != nil || ridScope == uuid.Nil {
		a.err(w, http.StatusForbidden, "нет привязки к заведению")
		return
	}
	loc := bookingLocationMoscow()
	now := time.Now().In(loc)
	todayYMD := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).Format("2006-01-02")
	scheduledToday := true
	if u.Role == "waiter" {
		var n int
		err := a.Pool.QueryRow(r.Context(), `
			SELECT COUNT(*)::int FROM waiter_work_dates
			WHERE user_id=$1 AND restaurant_id=$2 AND work_date=$3::date`,
			u.ID, ridScope, todayYMD).Scan(&n)
		if err != nil {
			a.err(w, http.StatusInternalServerError, "БД")
			return
		}
		scheduledToday = n > 0
	}
	rows, err := a.Pool.Query(r.Context(), `
		SELECT r.id, t.table_number, r.start_time, r.end_time, r.guest_count, r.status,
		       us.full_name, us.phone
		FROM reservations r
		JOIN tables t ON t.id = r.table_id
		JOIN halls h ON h.id = t.hall_id
		JOIN users us ON us.id = r.user_id
		WHERE h.restaurant_id = $3
		AND (
			($2 = 'waiter' AND r.assigned_waiter_id = $1)
			OR $2 IN ('admin','owner')
		)
		AND r.status IN ('confirmed','seated','in_service','pending_payment')
		ORDER BY
		  CASE
		    WHEN r.status IN ('seated','in_service','pending_payment') THEN 0
		    ELSE 1
		  END,
		  r.start_time ASC`, u.ID, u.Role, ridScope)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id uuid.UUID
		var tnum, guests int
		var st, end time.Time
		var status, fn, phone string
		_ = rows.Scan(&id, &tnum, &st, &end, &guests, &status, &fn, &phone)
		out = append(out, map[string]any{
			"reservation_id": id.String(), "table_number": tnum,
			"start_time": st, "end_time": end, "guest_count": guests,
			"status": status, "client_name": fn, "phone": phone,
		})
	}
	a.json(w, http.StatusOK, map[string]any{
		"scheduled_today": scheduledToday,
		"tables":          out,
	})
}
