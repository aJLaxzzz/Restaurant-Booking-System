package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (a *Handlers) handleReservationsMy(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	rows, err := a.Pool.Query(r.Context(), `
		SELECT r.id, r.table_id, r.start_time, r.end_time, r.guest_count, r.status, r.comment, r.created_at,
		       t.table_number, h.id, h.restaurant_id, rest.name, COALESCE(rest.slug,'')
		FROM reservations r
		JOIN tables t ON t.id = r.table_id
		JOIN halls h ON h.id = t.hall_id
		JOIN restaurants rest ON rest.id = h.restaurant_id
		WHERE r.user_id=$1 AND r.status NOT IN ('cancelled_by_client','cancelled_by_admin')
		ORDER BY r.start_time DESC`, u.ID)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var id, tid, hid, restID uuid.UUID
		var st, end time.Time
		var guests int
		var status, comment string
		var created time.Time
		var tnum int
		var restName, restSlug string
		_ = rows.Scan(&id, &tid, &st, &end, &guests, &status, &comment, &created, &tnum, &hid, &restID, &restName, &restSlug)
		out = append(out, map[string]any{
			"id": id.String(), "table_id": tid.String(), "hall_id": hid.String(),
			"restaurant_id": restID.String(), "restaurant_name": restName, "restaurant_slug": restSlug,
			"start_time": st, "end_time": end, "guest_count": guests, "status": status,
			"comment": comment, "created_at": created, "table_number": tnum,
		})
	}
	a.json(w, http.StatusOK, out)
}

func (a *Handlers) handleReservationGet(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	rid, err := uuid.Parse(chi.URLParam(r, "rid"))
	if err != nil {
		a.err(w, http.StatusBadRequest, "id")
		return
	}
	var userID uuid.UUID
	var tid uuid.UUID
	var st, end time.Time
	var guests int
	var status, comment string
	err = a.Pool.QueryRow(r.Context(), `
		SELECT user_id, table_id, start_time, end_time, guest_count, status, comment FROM reservations WHERE id=$1`, rid).
		Scan(&userID, &tid, &st, &end, &guests, &status, &comment)
	if err == pgx.ErrNoRows {
		a.err(w, http.StatusNotFound, "не найдено")
		return
	}
	if u.Role != "superadmin" && u.Role == "client" && userID != u.ID {
		a.err(w, http.StatusForbidden, "нет доступа")
		return
	}
	a.json(w, http.StatusOK, map[string]any{
		"id": rid.String(), "table_id": tid.String(), "start_time": st, "end_time": end,
		"guest_count": guests, "status": status, "comment": comment,
	})
}

func (a *Handlers) handleReservationsList(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	if u.Role != "admin" && u.Role != "owner" && u.Role != "superadmin" {
		a.err(w, http.StatusForbidden, "только админ")
		return
	}
	var ridScope uuid.UUID
	var err error
	if u.Role == "superadmin" {
		qs := strings.TrimSpace(r.URL.Query().Get("restaurant_id"))
		if qs == "" {
			a.err(w, http.StatusBadRequest, "нужен restaurant_id")
			return
		}
		ridScope, err = uuid.Parse(qs)
		if err != nil {
			a.err(w, http.StatusBadRequest, "restaurant_id")
			return
		}
	} else {
		ridScope, err = a.restaurantUUIDForUser(r.Context(), u.ID, u.Role)
		if err != nil || ridScope == uuid.Nil {
			a.err(w, http.StatusForbidden, "нет привязки к заведению")
			return
		}
	}
	q := `
		SELECT r.id, r.user_id, u.full_name, u.phone, r.table_id, t.table_number, r.start_time, r.end_time,
		       r.guest_count, r.status, r.created_by, COALESCE(r.comment,'')
		FROM reservations r
		JOIN users u ON u.id = r.user_id
		JOIN tables t ON t.id = r.table_id
		JOIN halls h ON h.id = t.hall_id
		WHERE h.restaurant_id = $1`
	args := []any{ridScope}
	n := 2
	if st := r.URL.Query().Get("status"); st != "" {
		q += ` AND r.status = $` + strconv.Itoa(n)
		args = append(args, st)
		n++
	}
	if u.Role != "superadmin" {
		// Панель админа/владельца: только брони на сегодня (календарный день Europe/Moscow), от ранних к поздним
		loc, tzErr := time.LoadLocation("Europe/Moscow")
		if tzErr != nil {
			loc = time.UTC
		}
		now := time.Now().In(loc)
		dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
		dayEnd := dayStart.Add(24 * time.Hour)
		q += ` AND r.start_time >= $` + strconv.Itoa(n) + ` AND r.start_time < $` + strconv.Itoa(n+1)
		args = append(args, dayStart, dayEnd)
		n += 2
	}
	if u.Role == "superadmin" {
		q += ` ORDER BY r.start_time DESC LIMIT 200`
	} else {
		q += ` ORDER BY r.start_time ASC LIMIT 200`
	}
	rows, err := a.Pool.Query(r.Context(), q, args...)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var id, uid, tid uuid.UUID
		var fn, phone string
		var tnum int
		var st, end time.Time
		var guests int
		var status, createdBy, comment string
		_ = rows.Scan(&id, &uid, &fn, &phone, &tid, &tnum, &st, &end, &guests, &status, &createdBy, &comment)
		out = append(out, map[string]any{
			"id": id.String(), "user_id": uid.String(),
			"full_name": fn, "client_name": fn, "phone": phone,
			"table_id": tid.String(), "table_number": tnum, "start_time": st, "end_time": end,
			"guest_count": guests, "status": status, "created_by": createdBy, "comment": comment,
		})
	}
	a.json(w, http.StatusOK, out)
}

func (a *Handlers) handleReservationCreate(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	var body struct {
		TableID         uuid.UUID `json:"table_id"`
		StartTime       time.Time `json:"start_time"`
		GuestCount      int       `json:"guest_count"`
		Comment         string    `json:"comment"`
		IdempotencyKey  uuid.UUID `json:"idempotency_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		a.err(w, http.StatusBadRequest, "JSON")
		return
	}
	if ik := r.Header.Get("X-Idempotency-Key"); ik != "" {
		if parsed, err := uuid.Parse(ik); err == nil {
			body.IdempotencyKey = parsed
		}
	}
	if body.IdempotencyKey == uuid.Nil {
		a.err(w, http.StatusBadRequest, "нужен idempotency_key")
		return
	}

	var dupRid uuid.UUID
	err := a.Pool.QueryRow(r.Context(), `
		SELECT id FROM reservations WHERE idempotency_key=$1`, body.IdempotencyKey).Scan(&dupRid)
	if err == nil {
		var depPay uuid.UUID
		err2 := a.Pool.QueryRow(r.Context(), `
			SELECT id FROM payments WHERE reservation_id=$1
			AND COALESCE(purpose,'deposit')='deposit'
			AND status IN ('pending','succeeded')
			ORDER BY created_at ASC LIMIT 1`, dupRid).Scan(&depPay)
		if err2 == nil {
			a.json(w, http.StatusOK, map[string]any{
				"reservation_id": dupRid.String(),
				"payment_id":     depPay.String(),
				"checkout_url":   "/pay/" + depPay.String(),
				"duplicate":      true,
			})
			return
		}
		a.json(w, http.StatusOK, map[string]any{
			"reservation_id":      dupRid.String(),
			"no_payment_required": true,
			"duplicate":           true,
		})
		return
	}
	if err != pgx.ErrNoRows {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}

	if msg := bookingStartNotAllowed(body.StartTime); msg != "" {
		a.err(w, http.StatusBadRequest, msg)
		return
	}

	var cap int
	var status string
	var hallID uuid.UUID
	var restID uuid.UUID
	err = a.Pool.QueryRow(r.Context(), `
		SELECT t.capacity, t.status, t.hall_id, h.restaurant_id FROM tables t
		JOIN halls h ON h.id = t.hall_id
		WHERE t.id=$1`, body.TableID).
		Scan(&cap, &status, &hallID, &restID)
	if err != nil {
		if err == pgx.ErrNoRows {
			a.err(w, http.StatusNotFound, "стол не найден")
			return
		}
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}

	slotHours := a.getSettingIntForRestaurant(r.Context(), restID, "default_slot_duration_hours", 2)
	endTime := body.StartTime.Add(time.Duration(slotHours) * time.Hour)

	if status == "blocked" {
		a.err(w, http.StatusConflict, "стол заблокирован")
		return
	}
	if body.GuestCount < 1 || body.GuestCount > 12 || body.GuestCount > cap {
		a.err(w, http.StatusBadRequest, "некорректное число гостей для стола")
		return
	}

	var otherUserBooking uuid.UUID
	err = a.Pool.QueryRow(r.Context(), `
		SELECT id FROM reservations
		WHERE user_id = $1
		AND status NOT IN ('cancelled_by_client', 'cancelled_by_admin', 'no_show')
		AND start_time < $3 AND end_time > $2
		LIMIT 1`, u.ID, body.StartTime, endTime).Scan(&otherUserBooking)
	if err == nil {
		a.err(w, http.StatusConflict, "у вас уже есть бронь на это время (в том числе в другом ресторане)")
		return
	}
	if err != pgx.ErrNoRows {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}

	var conflict uuid.UUID
	err = a.Pool.QueryRow(r.Context(), `
		SELECT id FROM reservations
		WHERE table_id = $1
		AND status NOT IN ('cancelled_by_client', 'cancelled_by_admin', 'no_show')
		AND (
			(start_time <= $2 AND end_time > $2) OR
			(start_time < $3 AND end_time >= $3) OR
			(start_time >= $2 AND end_time <= $3)
		) LIMIT 1`,
		body.TableID, body.StartTime, endTime).Scan(&conflict)
	if err == nil {
		a.err(w, http.StatusConflict, "стол уже занят на это время")
		return
	}
	if err != pgx.ErrNoRows {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}

	key := "table:" + body.TableID.String() + ":lock"
	holder, _ := a.RDB.Get(r.Context(), key).Result()
	if holder != "" && holder != u.ID.String() {
		a.err(w, http.StatusConflict, "стол бронируется другим пользователем")
		return
	}
	if holder == "" {
		ok, err := a.RDB.SetNX(r.Context(), key, u.ID.String(), 5*time.Minute).Result()
		if err != nil || !ok {
			a.err(w, http.StatusConflict, "стол бронируется другим пользователем")
			return
		}
	}

	depositPct := a.getSettingIntForRestaurant(r.Context(), restID, "deposit_percent", 30)
	avgKop := a.getSettingIntForRestaurant(r.Context(), restID, "avg_check_kopecks", 150000)
	amountKop := avgKop * depositPct / 100
	zeroDeposit := amountKop < 1

	tx, err := a.Pool.Begin(r.Context())
	if err != nil {
		_ = a.RDB.Del(r.Context(), key).Err()
		a.err(w, http.StatusInternalServerError, "tx")
		return
	}
	defer tx.Rollback(r.Context())

	var resID uuid.UUID
	resStatus := "pending_payment"
	if zeroDeposit {
		resStatus = "confirmed"
	}
	err = tx.QueryRow(r.Context(), `
		INSERT INTO reservations (table_id, user_id, start_time, end_time, guest_count, status, comment, created_by, idempotency_key)
		VALUES ($1,$2,$3,$4,$5,$6,$7,'client',$8)
		RETURNING id`,
		body.TableID, u.ID, body.StartTime, endTime, body.GuestCount, resStatus, nullStr(body.Comment), body.IdempotencyKey).Scan(&resID)
	if err != nil {
		_ = a.RDB.Del(r.Context(), key).Err()
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "violates") {
			a.err(w, http.StatusConflict, "конфликт бронирования")
			return
		}
		a.err(w, http.StatusInternalServerError, "не создать бронь")
		return
	}

	if zeroDeposit {
		_, _ = tx.Exec(r.Context(), `UPDATE tables SET status='occupied', updated_at=NOW() WHERE id=$1`, body.TableID)
		if err := tx.Commit(r.Context()); err != nil {
			_ = a.RDB.Del(r.Context(), key).Err()
			a.err(w, http.StatusInternalServerError, "commit")
			return
		}
		_ = a.RDB.Del(r.Context(), key).Err()
		ts := time.Now().UTC().Format(time.RFC3339)
		a.emitHallEvent(hallID, map[string]any{
			"event": "table.booked", "table_id": body.TableID.String(), "status": "occupied",
			"reservation_id": resID.String(), "timestamp": ts,
		})
		a.emitHallEvent(hallID, map[string]any{
			"event": "reservation.status_changed", "reservation_id": resID.String(),
			"status": "confirmed", "timestamp": ts,
		})
		_ = a.publishEvent(r.Context(), "reservation.created", map[string]any{
			"reservation_id": resID.String(), "user_id": u.ID.String(),
		})
		_ = a.publishEvent(r.Context(), "reservation.confirmed", map[string]any{
			"reservation_id": resID.String(), "user_id": u.ID.String(),
		})
		a.json(w, http.StatusCreated, map[string]any{
			"reservation_id":      resID.String(),
			"amount_kopecks":      0,
			"no_payment_required": true,
		})
		return
	}

	var payID uuid.UUID
	err = tx.QueryRow(r.Context(), `
		INSERT INTO payments (reservation_id, amount_kopecks, status, idempotency_key, purpose)
		VALUES ($1,$2,'pending',$3,'deposit') RETURNING id`, resID, amountKop, body.IdempotencyKey).Scan(&payID)
	if err != nil {
		_ = a.RDB.Del(r.Context(), key).Err()
		a.err(w, http.StatusConflict, "повторите запрос с новым ключом")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		_ = a.RDB.Del(r.Context(), key).Err()
		a.err(w, http.StatusInternalServerError, "commit")
		return
	}

	_ = a.publishEvent(r.Context(), "reservation.created", map[string]any{
		"reservation_id": resID.String(), "user_id": u.ID.String(),
	})

	a.emitHallEvent(hallID, map[string]any{
		"event": "reservation.status_changed", "reservation_id": resID.String(),
		"status": "pending_payment", "timestamp": time.Now().UTC().Format(time.RFC3339),
	})

	a.json(w, http.StatusCreated, map[string]any{
		"reservation_id": resID.String(),
		"payment_id":     payID.String(),
		"amount_kopecks": amountKop,
		"checkout_url":   "/pay/" + payID.String(),
	})
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (a *Handlers) getSettingInt(ctx context.Context, key string, def int) int {
	var txt string
	err := a.Pool.QueryRow(ctx, `SELECT value #>> '{}' FROM settings WHERE key=$1`, key).Scan(&txt)
	if err != nil {
		return def
	}
	txt = strings.Trim(txt, `"`)
	if v, e := strconv.Atoi(txt); e == nil {
		return v
	}
	return def
}

func bookingSettingKeysList() []string {
	return []string{
		"default_slot_duration_hours",
		"booking_open_hour",
		"booking_close_hour",
		"slot_minutes",
		"deposit_percent",
		"avg_check_kopecks",
	}
}

// ownerRestaurantIntSettingKeys — числовые настройки, доступные владельцу (пер-ресторан + fallback на settings).
func ownerRestaurantIntSettingKeys() []string {
	return []string{
		"avg_check_kopecks",
		"deposit_percent",
		"slot_minutes",
		"booking_open_hour",
		"booking_close_hour",
		"default_slot_duration_hours",
		"refund_more_than_2h_percent",
		"refund_within_2h_percent",
		"refund_more_than_24h_percent",
		"refund_12_to_24h_percent",
		"refund_less_than_12h_percent",
	}
}

func defaultIntForOwnerSettingKey(key string) int {
	switch key {
	case "default_slot_duration_hours":
		return 2
	case "booking_open_hour":
		return 10
	case "booking_close_hour":
		return 23
	case "slot_minutes":
		return 30
	case "deposit_percent":
		return 30
	case "avg_check_kopecks":
		return 150000
	case "refund_more_than_2h_percent", "refund_more_than_24h_percent":
		return 100
	case "refund_within_2h_percent", "refund_less_than_12h_percent":
		return 0
	case "refund_12_to_24h_percent":
		return 50
	default:
		return 0
	}
}

func defaultIntForBookingKey(key string) int {
	return defaultIntForOwnerSettingKey(key)
}

const ownerNoShowGraceKey = "no_show_grace_minutes"

// ownerNoShowGraceJSON — объект { "minutes": N } из restaurant_settings, иначе глобальный settings, иначе дефолт.
func (a *Handlers) ownerNoShowGraceJSON(ctx context.Context, restaurantID uuid.UUID) json.RawMessage {
	var raw []byte
	err := a.Pool.QueryRow(ctx, `SELECT value FROM restaurant_settings WHERE restaurant_id=$1 AND key=$2`, restaurantID, ownerNoShowGraceKey).Scan(&raw)
	if err == nil && len(raw) > 0 {
		return json.RawMessage(raw)
	}
	err = a.Pool.QueryRow(ctx, `SELECT value FROM settings WHERE key=$1`, ownerNoShowGraceKey).Scan(&raw)
	if err == nil && len(raw) > 0 {
		return json.RawMessage(raw)
	}
	return json.RawMessage(`{"minutes":20}`)
}

// getSettingIntForRestaurant: значение из restaurant_settings, иначе глобальный settings, иначе def.
func (a *Handlers) getSettingIntForRestaurant(ctx context.Context, restaurantID uuid.UUID, key string, def int) int {
	if restaurantID == uuid.Nil {
		return a.getSettingInt(ctx, key, def)
	}
	var txt string
	err := a.Pool.QueryRow(ctx, `SELECT value #>> '{}' FROM restaurant_settings WHERE restaurant_id=$1 AND key=$2`, restaurantID, key).Scan(&txt)
	if err == nil {
		txt = strings.Trim(txt, `"`)
		if v, e := strconv.Atoi(txt); e == nil {
			return v
		}
	}
	return a.getSettingInt(ctx, key, def)
}
