package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type reviewCreateReq struct {
	ReservationID    string `json:"reservation_id"`
	RatingRestaurant int    `json:"rating_restaurant"`
	RatingWaiter     *int   `json:"rating_waiter,omitempty"`
	Comment          string `json:"comment,omitempty"`
}

func (a *Handlers) handleReviewCreate(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	if u.ID == uuid.Nil {
		a.err(w, http.StatusUnauthorized, "требуется авторизация")
		return
	}

	var body reviewCreateReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		a.err(w, http.StatusBadRequest, "JSON")
		return
	}
	rid, err := uuid.Parse(body.ReservationID)
	if err != nil {
		a.err(w, http.StatusBadRequest, "reservation_id")
		return
	}
	if body.RatingRestaurant < 1 || body.RatingRestaurant > 5 {
		a.err(w, http.StatusBadRequest, "rating_restaurant")
		return
	}
	if body.RatingWaiter != nil && (*body.RatingWaiter < 1 || *body.RatingWaiter > 5) {
		a.err(w, http.StatusBadRequest, "rating_waiter")
		return
	}

	// Валидация: отзыв можно оставить только после успешной оплаты счёта (purpose=tab),
	// и только по своей брони.
	ctx := r.Context()
	var restaurantID uuid.UUID
	var waiterID *uuid.UUID
	var paymentOK bool
	err = a.Pool.QueryRow(ctx, `
		SELECT
			rest.id AS restaurant_id,
			res.assigned_waiter_id,
			EXISTS (
				SELECT 1
				FROM payments p
				WHERE p.reservation_id = res.id
				  AND p.purpose = 'tab'
				  AND p.status = 'succeeded'
			) AS payment_ok
		FROM reservations res
		JOIN tables t ON t.id = res.table_id
		JOIN halls h ON h.id = t.hall_id
		JOIN restaurants rest ON rest.id = h.restaurant_id
		WHERE res.id = $1 AND res.user_id = $2
	`, rid, u.ID).Scan(&restaurantID, &waiterID, &paymentOK)
	if err == pgx.ErrNoRows {
		a.err(w, http.StatusNotFound, "бронь не найдена или нет доступа")
		return
	}
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	if !paymentOK {
		a.err(w, http.StatusBadRequest, "сначала оплатите счёт")
		return
	}

	// Если официант не назначен — оценку официанта игнорируем.
	rw := body.RatingWaiter
	if waiterID == nil {
		rw = nil
	}

	_, err = a.Pool.Exec(ctx, `
		INSERT INTO reviews (reservation_id, restaurant_id, client_id, waiter_id, rating_restaurant, rating_waiter, comment)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (reservation_id, client_id) DO NOTHING
	`, rid, restaurantID, u.ID, waiterID, body.RatingRestaurant, rw, body.Comment)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	a.json(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (a *Handlers) handleOwnerRatings(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	ctx := r.Context()

	var restID uuid.UUID
	if err := a.Pool.QueryRow(ctx, `SELECT id FROM restaurants WHERE owner_user_id=$1 LIMIT 1`, u.ID).Scan(&restID); err != nil {
		a.err(w, http.StatusBadRequest, "ресторан не найден")
		return
	}

	var avg *float64
	var cnt int
	_ = a.Pool.QueryRow(ctx, `
		SELECT AVG(rating_restaurant)::float8, COUNT(*)::int
		FROM reviews
		WHERE restaurant_id = $1
	`, restID).Scan(&avg, &cnt)

	rows, err := a.Pool.Query(ctx, `
		SELECT u.id, COALESCE(u.full_name,''), AVG(rv.rating_waiter)::float8, COUNT(rv.rating_waiter)::int
		FROM users u
		LEFT JOIN reviews rv ON rv.waiter_id = u.id AND rv.restaurant_id = $1 AND rv.rating_waiter IS NOT NULL
		WHERE u.role='waiter' AND u.restaurant_id = $1
		GROUP BY u.id, u.full_name
		ORDER BY COUNT(rv.rating_waiter) DESC, COALESCE(AVG(rv.rating_waiter), 0) DESC, u.full_name
	`, restID)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	defer rows.Close()

	type waiterRow struct {
		ID    string   `json:"id"`
		Name  string   `json:"name"`
		Avg   *float64 `json:"avg"`
		Count int      `json:"count"`
	}
	var waiters []waiterRow
	for rows.Next() {
		var wid uuid.UUID
		var name string
		var wavg *float64
		var wcnt int
		if err := rows.Scan(&wid, &name, &wavg, &wcnt); err == nil {
			waiters = append(waiters, waiterRow{ID: wid.String(), Name: name, Avg: wavg, Count: wcnt})
		}
	}

	a.json(w, http.StatusOK, map[string]any{
		"restaurant_id":     restID.String(),
		"restaurant_avg":    avg,
		"restaurant_count":  cnt,
		"waiters":           waiters,
	})
}

func (a *Handlers) handleAdminWaitersRatings(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	ctx := r.Context()

	var restID uuid.UUID
	if err := a.Pool.QueryRow(ctx, `SELECT restaurant_id FROM users WHERE id=$1`, u.ID).Scan(&restID); err != nil || restID == uuid.Nil {
		a.err(w, http.StatusBadRequest, "restaurant_id")
		return
	}

	rows, err := a.Pool.Query(ctx, `
		SELECT u.id, COALESCE(u.full_name,''), AVG(rv.rating_waiter)::float8, COUNT(rv.rating_waiter)::int
		FROM users u
		LEFT JOIN reviews rv ON rv.waiter_id = u.id AND rv.restaurant_id = $1 AND rv.rating_waiter IS NOT NULL
		WHERE u.role='waiter' AND u.restaurant_id = $1
		GROUP BY u.id, u.full_name
		ORDER BY COUNT(rv.rating_waiter) DESC, COALESCE(AVG(rv.rating_waiter), 0) DESC, u.full_name
	`, restID)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	defer rows.Close()

	type waiterRow struct {
		ID    string   `json:"id"`
		Name  string   `json:"name"`
		Avg   *float64 `json:"avg"`
		Count int      `json:"count"`
	}
	var waiters []waiterRow
	for rows.Next() {
		var wid uuid.UUID
		var name string
		var avg *float64
		var cnt int
		if err := rows.Scan(&wid, &name, &avg, &cnt); err == nil {
			waiters = append(waiters, waiterRow{ID: wid.String(), Name: name, Avg: avg, Count: cnt})
		}
	}

	a.json(w, http.StatusOK, map[string]any{
		"restaurant_id": restID.String(),
		"waiters":       waiters,
	})
}

