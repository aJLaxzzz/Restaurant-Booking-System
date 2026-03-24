package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (a *Handlers) handleReservationUpdate(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	rid, err := uuid.Parse(chi.URLParam(r, "rid"))
	if err != nil {
		a.err(w, http.StatusBadRequest, "id")
		return
	}
	var body struct {
		AssignedWaiter *uuid.UUID `json:"assigned_waiter_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if u.Role != "admin" && u.Role != "owner" {
		a.err(w, http.StatusForbidden, "нет прав")
		return
	}
	_, err = a.Pool.Exec(r.Context(), `
		UPDATE reservations SET assigned_waiter_id = COALESCE($2, assigned_waiter_id), updated_at=NOW() WHERE id=$1`,
		rid, body.AssignedWaiter)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "ошибка")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *Handlers) handleReservationCancel(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	rid, err := uuid.Parse(chi.URLParam(r, "rid"))
	if err != nil {
		a.err(w, http.StatusBadRequest, "id")
		return
	}
	var userID uuid.UUID
	var status string
	var start time.Time
	var tableID uuid.UUID
	err = a.Pool.QueryRow(r.Context(), `
		SELECT user_id, status, start_time, table_id FROM reservations WHERE id=$1`, rid).
		Scan(&userID, &status, &start, &tableID)
	if err == pgx.ErrNoRows {
		a.err(w, http.StatusNotFound, "не найдено")
		return
	}
	if (u.Role == "client" || u.Role == "owner") && userID != u.ID {
		a.err(w, http.StatusForbidden, "нет доступа")
		return
	}
	if status != "pending_payment" && status != "confirmed" {
		a.err(w, http.StatusConflict, "нельзя отменить")
		return
	}

	newStatus := "cancelled_by_client"
	if u.Role != "client" && u.Role != "owner" {
		newStatus = "cancelled_by_admin"
	}

	refundPct := 0
	if status == "confirmed" {
		if u.Role == "client" || u.Role == "owner" {
			h := time.Until(start).Hours()
			if h >= 24 {
				refundPct = a.getSettingInt(r.Context(), "refund_more_than_24h_percent", 100)
			} else if h >= 12 {
				refundPct = a.getSettingInt(r.Context(), "refund_12_to_24h_percent", 50)
			} else {
				refundPct = a.getSettingInt(r.Context(), "refund_less_than_12h_percent", 0)
			}
		} else {
			refundPct = 100
		}
	}

	tx, err := a.Pool.Begin(r.Context())
	if err != nil {
		a.err(w, http.StatusInternalServerError, "tx")
		return
	}
	defer tx.Rollback(r.Context())

	_, err = tx.Exec(r.Context(), `UPDATE reservations SET status=$2, updated_at=NOW() WHERE id=$1`, rid, newStatus)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}

	if status == "pending_payment" {
		_, _ = tx.Exec(r.Context(), `UPDATE payments SET status='failed', updated_at=NOW() WHERE reservation_id=$1 AND status='pending'`, rid)
	}

	var payID uuid.UUID
	var amount int
	_ = tx.QueryRow(r.Context(), `SELECT id, amount_kopecks FROM payments WHERE reservation_id=$1 AND status='succeeded' ORDER BY created_at DESC LIMIT 1`, rid).
		Scan(&payID, &amount)
	if payID != uuid.Nil && refundPct > 0 {
		refund := amount * refundPct / 100
		_, _ = tx.Exec(r.Context(), `UPDATE payments SET status='refunded', refund_amount_kopecks=$2, updated_at=NOW() WHERE id=$1`, payID, refund)
	}

	_ = tx.Commit(r.Context())

	_ = a.RDB.Del(r.Context(), "table:"+tableID.String()+":lock").Err()
	_, _ = a.Pool.Exec(r.Context(), `UPDATE tables SET status='available', updated_at=NOW() WHERE id=$1`, tableID)

	var hallID uuid.UUID
	_ = a.Pool.QueryRow(r.Context(), `SELECT hall_id FROM tables WHERE id=$1`, tableID).Scan(&hallID)
	a.emitHallEvent(hallID, map[string]any{
		"event": "table.freed", "table_id": tableID.String(),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})

	a.json(w, http.StatusOK, map[string]any{"status": newStatus, "refund_percent": refundPct})
}

func (a *Handlers) handleCheckin(w http.ResponseWriter, r *http.Request) {
	rid, _ := uuid.Parse(chi.URLParam(r, "rid"))
	var body struct {
		WaiterID *uuid.UUID `json:"waiter_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	_, err := a.Pool.Exec(r.Context(), `
		UPDATE reservations SET status='seated', seated_at=NOW(), assigned_waiter_id=COALESCE($2, assigned_waiter_id), updated_at=NOW()
		WHERE id=$1 AND status='confirmed'`, rid, body.WaiterID)
	if err != nil {
		a.err(w, http.StatusBadRequest, "невозможно check-in")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *Handlers) handleNoshow(w http.ResponseWriter, r *http.Request) {
	rid, _ := uuid.Parse(chi.URLParam(r, "rid"))
	var tableID uuid.UUID
	err := a.Pool.QueryRow(r.Context(), `SELECT table_id FROM reservations WHERE id=$1 AND status='confirmed'`, rid).Scan(&tableID)
	if err != nil {
		a.err(w, http.StatusNotFound, "бронь не найдена")
		return
	}
	_, _ = a.Pool.Exec(r.Context(), `UPDATE reservations SET status='no_show', updated_at=NOW() WHERE id=$1`, rid)
	_, _ = a.Pool.Exec(r.Context(), `UPDATE tables SET status='available', updated_at=NOW() WHERE id=$1`, tableID)
	// Депозит удерживается (no-show), запись платежа не меняем
	var hallID uuid.UUID
	_ = a.Pool.QueryRow(r.Context(), `SELECT hall_id FROM tables WHERE id=$1`, tableID).Scan(&hallID)
	a.emitHallEvent(hallID, map[string]any{"event": "table.freed", "table_id": tableID.String()})
	w.WriteHeader(http.StatusNoContent)
}

func (a *Handlers) handleStartService(w http.ResponseWriter, r *http.Request) {
	rid, _ := uuid.Parse(chi.URLParam(r, "rid"))
	ct, err := a.Pool.Exec(r.Context(), `
		UPDATE reservations SET status='in_service', service_started_at=NOW(), updated_at=NOW() WHERE id=$1 AND status='seated'`, rid)
	if err != nil || ct.RowsAffected() == 0 {
		a.err(w, http.StatusBadRequest, "недопустимый переход")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *Handlers) handleComplete(w http.ResponseWriter, r *http.Request) {
	rid, _ := uuid.Parse(chi.URLParam(r, "rid"))
	var tableID uuid.UUID
	err := a.Pool.QueryRow(r.Context(), `SELECT table_id FROM reservations WHERE id=$1`, rid).Scan(&tableID)
	if err != nil {
		a.err(w, http.StatusNotFound, "не найдено")
		return
	}
	_, _ = a.Pool.Exec(r.Context(), `UPDATE reservations SET status='completed', completed_at=NOW(), updated_at=NOW() WHERE id=$1`, rid)
	_, _ = a.Pool.Exec(r.Context(), `UPDATE tables SET status='available', updated_at=NOW() WHERE id=$1`, tableID)
	var hallID uuid.UUID
	_ = a.Pool.QueryRow(r.Context(), `SELECT hall_id FROM tables WHERE id=$1`, tableID).Scan(&hallID)
	a.emitHallEvent(hallID, map[string]any{"event": "table.freed", "table_id": tableID.String()})
	w.WriteHeader(http.StatusNoContent)
}

func (a *Handlers) handleAdminReservationCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		UserID     uuid.UUID `json:"user_id"`
		TableID    uuid.UUID `json:"table_id"`
		StartTime  time.Time `json:"start_time"`
		GuestCount int       `json:"guest_count"`
		Comment    string    `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		a.err(w, http.StatusBadRequest, "JSON")
		return
	}
	slot := a.getSettingInt(r.Context(), "default_slot_duration_hours", 2)
	endTime := body.StartTime.Add(time.Duration(slot) * time.Hour)
	var cap int
	var hallID uuid.UUID
	var tstatus string
	err := a.Pool.QueryRow(r.Context(), `SELECT capacity, hall_id, status FROM tables WHERE id=$1`, body.TableID).
		Scan(&cap, &hallID, &tstatus)
	if err != nil {
		if err == pgx.ErrNoRows {
			a.err(w, http.StatusNotFound, "стол")
			return
		}
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	if tstatus == "blocked" {
		a.err(w, http.StatusConflict, "стол заблокирован")
		return
	}
	if body.GuestCount < 1 || body.GuestCount > cap {
		a.err(w, http.StatusBadRequest, "неверное число гостей")
		return
	}
	var conflict uuid.UUID
	err = a.Pool.QueryRow(r.Context(), `
		SELECT id FROM reservations WHERE table_id=$1
		AND status NOT IN ('cancelled_by_client','cancelled_by_admin','no_show')
		AND (
			(start_time <= $2 AND end_time > $2) OR
			(start_time < $3 AND end_time >= $3) OR
			(start_time >= $2 AND end_time <= $3)
		) LIMIT 1`, body.TableID, body.StartTime, endTime).Scan(&conflict)
	if err == nil {
		a.err(w, http.StatusConflict, "время занято")
		return
	}
	if err != pgx.ErrNoRows {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	var rid uuid.UUID
	err = a.Pool.QueryRow(r.Context(), `
		INSERT INTO reservations (table_id, user_id, start_time, end_time, guest_count, status, comment, created_by)
		VALUES ($1,$2,$3,$4,$5,'confirmed',$6,'admin')
		RETURNING id`,
		body.TableID, body.UserID, body.StartTime, endTime, body.GuestCount, nullStr(body.Comment)).Scan(&rid)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "не создано")
		return
	}
	_, _ = a.Pool.Exec(r.Context(), `UPDATE tables SET status='occupied', updated_at=NOW() WHERE id=$1`, body.TableID)
	a.emitHallEvent(hallID, map[string]any{
		"event": "table.booked", "table_id": body.TableID.String(), "reservation_id": rid.String(),
	})
	_ = a.publishEvent(r.Context(), "reservation.admin_created", map[string]any{"reservation_id": rid.String()})
	a.json(w, http.StatusCreated, map[string]string{"id": rid.String()})
}
