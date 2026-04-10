package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// restaurantUUIDForUser возвращает UUID ресторана для владельца (по owner_user_id) или staff (users.restaurant_id).
func (a *Handlers) restaurantUUIDForUser(ctx context.Context, uid uuid.UUID, role string) (uuid.UUID, error) {
	if role == "owner" {
		var rid uuid.UUID
		err := a.Pool.QueryRow(ctx, `SELECT id FROM restaurants WHERE owner_user_id=$1`, uid).Scan(&rid)
		if err == pgx.ErrNoRows {
			return uuid.Nil, err
		}
		return rid, err
	}
	if role == "admin" || role == "waiter" {
		var rid uuid.UUID
		err := a.Pool.QueryRow(ctx, `SELECT restaurant_id FROM users WHERE id=$1`, uid).Scan(&rid)
		if err != nil || rid == uuid.Nil {
			return uuid.Nil, pgx.ErrNoRows
		}
		return rid, nil
	}
	return uuid.Nil, pgx.ErrNoRows
}

func (a *Handlers) mustRestaurant(w http.ResponseWriter, r *http.Request, u userCtx) (uuid.UUID, bool) {
	if u.Role == "superadmin" {
		q := strings.TrimSpace(r.URL.Query().Get("restaurant_id"))
		if q == "" {
			a.err(w, http.StatusBadRequest, "нужен restaurant_id")
			return uuid.Nil, false
		}
		rid, err := uuid.Parse(q)
		if err != nil {
			a.err(w, http.StatusBadRequest, "restaurant_id")
			return uuid.Nil, false
		}
		var one int
		if err := a.Pool.QueryRow(r.Context(), `SELECT 1 FROM restaurants WHERE id=$1`, rid).Scan(&one); err != nil {
			a.err(w, http.StatusNotFound, "ресторан")
			return uuid.Nil, false
		}
		return rid, true
	}
	rid, err := a.restaurantUUIDForUser(r.Context(), u.ID, u.Role)
	if err != nil || rid == uuid.Nil {
		a.err(w, http.StatusForbidden, "нет привязки к заведению")
		return uuid.Nil, false
	}
	return rid, true
}

// restaurantIDByHall из hall_id.
func (a *Handlers) restaurantIDByHall(ctx context.Context, hallID uuid.UUID) (uuid.UUID, error) {
	var rid uuid.UUID
	err := a.Pool.QueryRow(ctx, `SELECT restaurant_id FROM halls WHERE id=$1`, hallID).Scan(&rid)
	return rid, err
}

// restaurantIDByReservation из reservation_id.
func (a *Handlers) restaurantIDByReservation(ctx context.Context, resID uuid.UUID) (uuid.UUID, error) {
	var rid uuid.UUID
	err := a.Pool.QueryRow(ctx, `
		SELECT h.restaurant_id FROM reservations r
		JOIN tables t ON t.id = r.table_id
		JOIN halls h ON h.id = t.hall_id
		WHERE r.id=$1`, resID).Scan(&rid)
	return rid, err
}

func (a *Handlers) userMayAccessRestaurant(w http.ResponseWriter, r *http.Request, u userCtx, restaurantID uuid.UUID) bool {
	if u.Role == "superadmin" {
		return true
	}
	if u.Role == "client" {
		return true
	}
	if u.Role == "owner" {
		var oid uuid.UUID
		_ = a.Pool.QueryRow(r.Context(), `SELECT owner_user_id FROM restaurants WHERE id=$1`, restaurantID).Scan(&oid)
		return oid == u.ID
	}
	if u.Role == "admin" || u.Role == "waiter" {
		var rid uuid.UUID
		_ = a.Pool.QueryRow(r.Context(), `SELECT restaurant_id FROM users WHERE id=$1`, u.ID).Scan(&rid)
		return rid == restaurantID
	}
	return false
}
