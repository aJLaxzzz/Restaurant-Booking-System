package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (a *Handlers) reservationOrderAccess(w http.ResponseWriter, r *http.Request, resID uuid.UUID, needEdit bool) (uuid.UUID, uuid.UUID, bool) {
	u := userFrom(r)
	var userID uuid.UUID
	var status string
	var assignedPtr *uuid.UUID
	err := a.Pool.QueryRow(r.Context(), `
		SELECT user_id, status, assigned_waiter_id FROM reservations WHERE id=$1`, resID).
		Scan(&userID, &status, &assignedPtr)
	if err == pgx.ErrNoRows {
		a.err(w, http.StatusNotFound, "бронь")
		return uuid.Nil, uuid.Nil, false
	}
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return uuid.Nil, uuid.Nil, false
	}
	hasAssign := assignedPtr != nil && *assignedPtr != uuid.Nil
	assignedWaiter := uuid.Nil
	if assignedPtr != nil {
		assignedWaiter = *assignedPtr
	}
	restID, err := a.restaurantIDByReservation(r.Context(), resID)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return uuid.Nil, uuid.Nil, false
	}
	if u.Role == "client" {
		if userID != u.ID {
			a.err(w, http.StatusForbidden, "нет доступа")
			return uuid.Nil, uuid.Nil, false
		}
		if needEdit {
			if status != "seated" && status != "in_service" {
				a.err(w, http.StatusForbidden, "заказ по меню доступен после посадки")
				return uuid.Nil, uuid.Nil, false
			}
			return restID, uuid.Nil, true
		}
		return restID, uuid.Nil, true
	}
	if !a.userMayAccessRestaurant(w, r, u, restID) {
		a.err(w, http.StatusForbidden, "чужое заведение")
		return uuid.Nil, uuid.Nil, false
	}
	if needEdit {
		if u.Role != "waiter" {
			a.err(w, http.StatusForbidden, "строки заказа добавляет только официант")
			return uuid.Nil, uuid.Nil, false
		}
		if !hasAssign || assignedWaiter != u.ID {
			a.err(w, http.StatusForbidden, "не назначенный официант за этот стол")
			return uuid.Nil, uuid.Nil, false
		}
	}
	return restID, uuid.Nil, true
}

func (a *Handlers) ensureOrderOpen(w http.ResponseWriter, r *http.Request, resID uuid.UUID) (uuid.UUID, bool) {
	var oid uuid.UUID
	err := a.Pool.QueryRow(r.Context(), `
		SELECT o.id FROM reservation_orders o
		WHERE o.reservation_id=$1 AND o.status='open'`, resID).Scan(&oid)
	if err == pgx.ErrNoRows {
		a.err(w, http.StatusConflict, "счёт закрыт или не найден")
		return uuid.Nil, false
	}
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return uuid.Nil, false
	}
	return oid, true
}

func (a *Handlers) getOrCreateOrder(w http.ResponseWriter, r *http.Request, resID uuid.UUID) (uuid.UUID, bool) {
	var oid uuid.UUID
	var ost string
	err := a.Pool.QueryRow(r.Context(), `SELECT id, status FROM reservation_orders WHERE reservation_id=$1`, resID).Scan(&oid, &ost)
	if err == nil {
		if ost != "open" {
			a.err(w, http.StatusConflict, "счёт закрыт")
			return uuid.Nil, false
		}
		return oid, true
	}
	if err != pgx.ErrNoRows {
		a.err(w, http.StatusInternalServerError, "БД")
		return uuid.Nil, false
	}
	err = a.Pool.QueryRow(r.Context(), `
		INSERT INTO reservation_orders (reservation_id, status) VALUES ($1,'open') RETURNING id`, resID).Scan(&oid)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "не создан счёт")
		return uuid.Nil, false
	}
	return oid, true
}

func (a *Handlers) handleReservationOrderGet(w http.ResponseWriter, r *http.Request) {
	rid, err := uuid.Parse(chi.URLParam(r, "rid"))
	if err != nil {
		a.err(w, http.StatusBadRequest, "id")
		return
	}
	if _, _, ok := a.reservationOrderAccess(w, r, rid, false); !ok {
		return
	}
	var oid uuid.UUID
	var st string
	var created time.Time
	err = a.Pool.QueryRow(r.Context(), `
		SELECT id, status, created_at FROM reservation_orders WHERE reservation_id=$1`, rid).Scan(&oid, &st, &created)
	restaurantID, _ := a.restaurantIDByReservation(r.Context(), rid)
	if err == pgx.ErrNoRows {
		a.json(w, http.StatusOK, map[string]any{
			"restaurant_id": restaurantID.String(),
			"order":         nil,
			"lines":         []any{},
			"total_kopecks": 0,
		})
		return
	}
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	rows, err := a.Pool.Query(r.Context(), `
		SELECT l.id, l.menu_item_id, l.quantity, l.guest_label, COALESCE(l.note,''), m.name, m.price_kopecks
		FROM order_lines l
		JOIN menu_items m ON m.id = l.menu_item_id
		WHERE l.order_id=$1 ORDER BY l.created_at`, oid)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	defer rows.Close()
	var lines []map[string]any
	total := 0
	for rows.Next() {
		var lid, mid uuid.UUID
		var qty int
		var gl, note, mname string
		var price int
		_ = rows.Scan(&lid, &mid, &qty, &gl, &note, &mname, &price)
		lineTotal := price * qty
		total += lineTotal
		lines = append(lines, map[string]any{
			"id": lid.String(), "menu_item_id": mid.String(), "item_name": mname,
			"quantity": qty, "guest_label": gl, "note": note,
			"line_total_kopecks": lineTotal,
		})
	}
	a.json(w, http.StatusOK, map[string]any{
		"restaurant_id": restaurantID.String(),
		"order": map[string]any{
			"id": oid.String(), "status": st, "created_at": created,
		},
		"lines": lines, "total_kopecks": total,
	})
}

func (a *Handlers) handleReservationOrderLinePost(w http.ResponseWriter, r *http.Request) {
	rid, err := uuid.Parse(chi.URLParam(r, "rid"))
	if err != nil {
		a.err(w, http.StatusBadRequest, "id")
		return
	}
	if _, _, ok := a.reservationOrderAccess(w, r, rid, true); !ok {
		return
	}
	var st string
	_ = a.Pool.QueryRow(r.Context(), `SELECT status FROM reservations WHERE id=$1`, rid).Scan(&st)
	if st != "seated" && st != "in_service" && st != "confirmed" {
		a.err(w, http.StatusConflict, "заказ недоступен для этого статуса брони")
		return
	}
	if u := userFrom(r); u.Role == "client" && st != "seated" && st != "in_service" {
		a.err(w, http.StatusConflict, "добавление блюд после посадки за столом")
		return
	}
	oid, ok := a.getOrCreateOrder(w, r, rid)
	if !ok {
		return
	}
	var body struct {
		MenuItemID  uuid.UUID `json:"menu_item_id"`
		Quantity    int       `json:"quantity"`
		GuestLabel  string    `json:"guest_label"`
		Note        string    `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.MenuItemID == uuid.Nil || body.Quantity < 1 {
		a.err(w, http.StatusBadRequest, "данные")
		return
	}
	if body.GuestLabel == "" {
		body.GuestLabel = "Гость"
	}
	restID, _ := a.restaurantIDByReservation(r.Context(), rid)
	var mrest uuid.UUID
	err = a.Pool.QueryRow(r.Context(), `SELECT restaurant_id FROM menu_items WHERE id=$1`, body.MenuItemID).Scan(&mrest)
	if err != nil || mrest != restID {
		a.err(w, http.StatusBadRequest, "блюдо")
		return
	}
	var lid uuid.UUID
	err = a.Pool.QueryRow(r.Context(), `
		INSERT INTO order_lines (order_id, menu_item_id, quantity, guest_label, note)
		VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		oid, body.MenuItemID, body.Quantity, body.GuestLabel, nullStr(body.Note)).Scan(&lid)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "не добавлено")
		return
	}
	a.json(w, http.StatusCreated, map[string]string{"id": lid.String()})
}

func (a *Handlers) handleReservationOrderLineDelete(w http.ResponseWriter, r *http.Request) {
	rid, err := uuid.Parse(chi.URLParam(r, "rid"))
	if err != nil {
		a.err(w, http.StatusBadRequest, "id")
		return
	}
	if _, _, ok := a.reservationOrderAccess(w, r, rid, true); !ok {
		return
	}
	var stDel string
	_ = a.Pool.QueryRow(r.Context(), `SELECT status FROM reservations WHERE id=$1`, rid).Scan(&stDel)
	if userFrom(r).Role == "client" && stDel != "seated" && stDel != "in_service" {
		a.err(w, http.StatusConflict, "изменение заказа после посадки")
		return
	}
	lid, err := uuid.Parse(chi.URLParam(r, "lid"))
	if err != nil {
		a.err(w, http.StatusBadRequest, "line")
		return
	}
	oid, ok := a.ensureOrderOpen(w, r, rid)
	if !ok {
		return
	}
	ct, err := a.Pool.Exec(r.Context(), `
		DELETE FROM order_lines WHERE id=$1 AND order_id=$2`, lid, oid)
	if err != nil || ct.RowsAffected() == 0 {
		a.err(w, http.StatusNotFound, "строка")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *Handlers) handleReservationOrderCheckout(w http.ResponseWriter, r *http.Request) {
	rid, err := uuid.Parse(chi.URLParam(r, "rid"))
	if err != nil {
		a.err(w, http.StatusBadRequest, "id")
		return
	}
	u := userFrom(r)
	if u.Role != "client" {
		a.err(w, http.StatusForbidden, "оплату инициирует гость")
		return
	}
	if _, _, ok := a.reservationOrderAccess(w, r, rid, false); !ok {
		return
	}
	var oid uuid.UUID
	var ost string
	err = a.Pool.QueryRow(r.Context(), `SELECT id, status FROM reservation_orders WHERE reservation_id=$1`, rid).Scan(&oid, &ost)
	if err == pgx.ErrNoRows {
		a.err(w, http.StatusBadRequest, "нет заказа")
		return
	}
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	if ost != "open" {
		a.err(w, http.StatusConflict, "счёт уже закрыт")
		return
	}
	var total int
	err = a.Pool.QueryRow(r.Context(), `
		SELECT COALESCE(SUM(l.quantity * m.price_kopecks),0)
		FROM order_lines l
		JOIN menu_items m ON m.id = l.menu_item_id
		WHERE l.order_id=$1`, oid).Scan(&total)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	if total < 1 {
		a.err(w, http.StatusBadRequest, "пустой счёт")
		return
	}
	var existing uuid.UUID
	err = a.Pool.QueryRow(r.Context(), `
		SELECT id FROM payments WHERE reservation_order_id=$1 AND status='pending'`, oid).Scan(&existing)
	if err == nil {
		a.json(w, http.StatusOK, map[string]any{
			"payment_id": existing.String(), "amount_kopecks": total,
			"checkout_url": "/pay/" + existing.String(), "duplicate": true,
		})
		return
	}
	if err != pgx.ErrNoRows {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	idem := uuid.New()
	var payID uuid.UUID
	err = a.Pool.QueryRow(r.Context(), `
		INSERT INTO payments (reservation_id, amount_kopecks, status, idempotency_key, purpose, reservation_order_id)
		VALUES ($1,$2,'pending',$3,'tab',$4) RETURNING id`,
		rid, total, idem, oid).Scan(&payID)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "платёж")
		return
	}
	a.json(w, http.StatusCreated, map[string]any{
		"payment_id": payID.String(), "amount_kopecks": total,
		"checkout_url": "/pay/" + payID.String(),
	})
}

func (a *Handlers) MountOrders(r chi.Router) {
	r.Get("/reservations/{rid}/order", a.handleReservationOrderGet)
	r.Post("/reservations/{rid}/order/lines", a.handleReservationOrderLinePost)
	r.Delete("/reservations/{rid}/order/lines/{lid}", a.handleReservationOrderLineDelete)
	r.Post("/reservations/{rid}/order/checkout", a.handleReservationOrderCheckout)
}
