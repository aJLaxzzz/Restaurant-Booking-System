package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (a *Handlers) adminWaiterMustBelong(w http.ResponseWriter, r *http.Request, adminRid, waiterID uuid.UUID) bool {
	var n int
	err := a.Pool.QueryRow(r.Context(), `
		SELECT COUNT(*)::int FROM users
		WHERE id=$1 AND role='waiter' AND restaurant_id=$2`, waiterID, adminRid).Scan(&n)
	if err != nil || n == 0 {
		a.err(w, http.StatusForbidden, "официант не из вашего заведения")
		return false
	}
	return true
}

func parseDateParam(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation("2006-01-02", s, time.UTC)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// handleAdminWaitersWorkDatesBulkGet — по всем официантам заведения: даты в [from, to].
func (a *Handlers) handleAdminWaitersWorkDatesBulkGet(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	rid, ok := a.mustRestaurant(w, r, u)
	if !ok || u.Role != "admin" {
		return
	}
	fromS := r.URL.Query().Get("from")
	toS := r.URL.Query().Get("to")
	from, ok1 := parseDateParam(fromS)
	to, ok2 := parseDateParam(toS)
	if !ok1 || !ok2 || to.Before(from) {
		a.err(w, http.StatusBadRequest, "укажите from и to в формате YYYY-MM-DD")
		return
	}
	rows, err := a.Pool.Query(r.Context(), `
		SELECT user_id, work_date FROM waiter_work_dates
		WHERE restaurant_id=$1 AND work_date >= $2::date AND work_date <= $3::date
		ORDER BY user_id, work_date`,
		rid, from.Format("2006-01-02"), to.Format("2006-01-02"))
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	defer rows.Close()
	byWaiter := map[string][]string{}
	for rows.Next() {
		var wid uuid.UUID
		var d time.Time
		if err := rows.Scan(&wid, &d); err != nil {
			a.err(w, http.StatusInternalServerError, "БД")
			return
		}
		key := wid.String()
		ds := d.Format("2006-01-02")
		byWaiter[key] = append(byWaiter[key], ds)
	}
	a.json(w, http.StatusOK, map[string]any{"by_waiter": byWaiter})
}

// handleAdminWaiterWorkDatesGet — даты работы официанта в диапазоне [from, to] (календарные дни UTC).
func (a *Handlers) handleAdminWaiterWorkDatesGet(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	rid, ok := a.mustRestaurant(w, r, u)
	if !ok || u.Role != "admin" {
		return
	}
	wid, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		a.err(w, http.StatusBadRequest, "id")
		return
	}
	if !a.adminWaiterMustBelong(w, r, rid, wid) {
		return
	}
	fromS := r.URL.Query().Get("from")
	toS := r.URL.Query().Get("to")
	from, ok1 := parseDateParam(fromS)
	to, ok2 := parseDateParam(toS)
	if !ok1 || !ok2 || to.Before(from) {
		a.err(w, http.StatusBadRequest, "укажите from и to в формате YYYY-MM-DD")
		return
	}

	rows, err := a.Pool.Query(r.Context(), `
		SELECT work_date FROM waiter_work_dates
		WHERE user_id=$1 AND restaurant_id=$2 AND work_date >= $3::date AND work_date <= $4::date
		ORDER BY work_date`, wid, rid, from.Format("2006-01-02"), to.Format("2006-01-02"))
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	defer rows.Close()
	dates := make([]string, 0)
	for rows.Next() {
		var d time.Time
		if err := rows.Scan(&d); err != nil {
			a.err(w, http.StatusInternalServerError, "БД")
			return
		}
		dates = append(dates, d.Format("2006-01-02"))
	}
	a.json(w, http.StatusOK, map[string]any{"dates": dates})
}

// handleAdminWaiterWorkDatesPut — заменить множество дат внутри [from, to] (включительно).
func (a *Handlers) handleAdminWaiterWorkDatesPut(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	rid, ok := a.mustRestaurant(w, r, u)
	if !ok || u.Role != "admin" {
		return
	}
	wid, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		a.err(w, http.StatusBadRequest, "id")
		return
	}
	if !a.adminWaiterMustBelong(w, r, rid, wid) {
		return
	}
	var body struct {
		From  string   `json:"from"`
		To    string   `json:"to"`
		Dates []string `json:"dates"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		a.err(w, http.StatusBadRequest, "json")
		return
	}
	from, ok1 := parseDateParam(body.From)
	to, ok2 := parseDateParam(body.To)
	if !ok1 || !ok2 || to.Before(from) {
		a.err(w, http.StatusBadRequest, "from/to YYYY-MM-DD")
		return
	}

	tx, err := a.Pool.Begin(r.Context())
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()

	_, err = tx.Exec(r.Context(), `
		DELETE FROM waiter_work_dates
		WHERE user_id=$1 AND restaurant_id=$2 AND work_date >= $3::date AND work_date <= $4::date`,
		wid, rid, from.Format("2006-01-02"), to.Format("2006-01-02"))
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}

	fromDay := from
	toDay := to
	for _, ds := range body.Dates {
		d, ok := parseDateParam(ds)
		if !ok {
			continue
		}
		if d.Before(fromDay) || d.After(toDay) {
			continue
		}
		_, err = tx.Exec(r.Context(), `
			INSERT INTO waiter_work_dates (user_id, work_date, restaurant_id)
			VALUES ($1, $2::date, $3)
			ON CONFLICT (user_id, work_date) DO UPDATE SET restaurant_id = EXCLUDED.restaurant_id`,
			wid, d.Format("2006-01-02"), rid)
		if err != nil {
			a.err(w, http.StatusInternalServerError, "БД")
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		if err := a.rebalanceWaitersForRestaurantDay(r.Context(), rid, d.Format("2006-01-02")); err != nil {
			a.err(w, http.StatusInternalServerError, "пересчёт назначений")
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}
