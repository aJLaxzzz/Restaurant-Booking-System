package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/xuri/excelize/v2"
	"golang.org/x/crypto/bcrypt"
)

// ownerMoscowRangeFromRequest — период [from, to] по календарным дням Europe/Moscow; в SQL использовать start >= $from AND start < toExclusive.
func ownerMoscowRangeFromRequest(r *http.Request) (from time.Time, toExclusive time.Time) {
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	toExclusive = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).Add(24 * time.Hour)
	from = toExclusive.AddDate(0, 0, -90)
	if f := r.URL.Query().Get("from"); f != "" {
		if t, err := time.ParseInLocation("2006-01-02", f, loc); err == nil {
			from = t
		}
	}
	if t := r.URL.Query().Get("to"); t != "" {
		if parsed, err := time.ParseInLocation("2006-01-02", t, loc); err == nil {
			toExclusive = parsed.Add(24 * time.Hour)
		}
	}
	if !toExclusive.After(from) {
		toExclusive = from.Add(24 * time.Hour)
	}
	return from, toExclusive
}

func normalizePhoneRU(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, " ", ""))
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "+7") {
		return s
	}
	if len(s) == 11 && s[0] == '8' {
		return "+7" + s[1:]
	}
	if len(s) == 10 && s[0] == '9' {
		return "+7" + s
	}
	return s
}

func (a *Handlers) handleAdminClientLookup(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	if u.Role != "admin" && u.Role != "owner" {
		a.err(w, http.StatusForbidden, "нет доступа")
		return
	}
	raw := normalizePhoneRU(r.URL.Query().Get("phone"))
	if raw == "" || !phoneRe.MatchString(raw) {
		a.err(w, http.StatusBadRequest, "телефон в формате +7XXXXXXXXXX")
		return
	}
	var id uuid.UUID
	var fn string
	err := a.Pool.QueryRow(r.Context(), `
		SELECT id, full_name FROM users
		WHERE phone=$1 AND role='client' AND COALESCE(status,'active')='active'`, raw).Scan(&id, &fn)
	if err == pgx.ErrNoRows {
		a.err(w, http.StatusNotFound, "клиент не найден")
		return
	}
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	a.json(w, http.StatusOK, map[string]string{"id": id.String(), "full_name": fn})
}

func (a *Handlers) handleOwnerAnalytics(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	rid, ok := a.mustRestaurant(w, r, u)
	if !ok {
		return
	}
	fromT, toEx := ownerMoscowRangeFromRequest(r)
	rows, err := a.Pool.Query(r.Context(), `
		SELECT date_trunc('day', r.start_time)::date as d,
		       COUNT(*)::float / NULLIF((SELECT COUNT(*)::float FROM tables t JOIN halls h ON h.id=t.hall_id WHERE h.restaurant_id=$1), 0) * 100 as load
		FROM reservations r
		JOIN tables t ON t.id = r.table_id
		JOIN halls h ON h.id = t.hall_id
		WHERE h.restaurant_id = $1
		AND r.status IN ('confirmed','seated','in_service','completed')
		AND r.start_time >= $2 AND r.start_time < $3
		GROUP BY 1 ORDER BY 1`, rid, fromT, toEx)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	defer rows.Close()
	var labels []string
	var data []float64
	for rows.Next() {
		var d time.Time
		var load float64
		_ = rows.Scan(&d, &load)
		labels = append(labels, d.Format("2006-01-02"))
		data = append(data, load)
	}

	bRows, err := a.Pool.Query(r.Context(), `
		SELECT date_trunc('day', r.start_time)::date,
		       COUNT(*)::int,
		       COUNT(*) FILTER (WHERE r.status = 'no_show')::int
		FROM reservations r
		JOIN tables t ON t.id = r.table_id
		JOIN halls h ON h.id = t.hall_id
		WHERE h.restaurant_id = $1 AND r.start_time >= $2 AND r.start_time < $3
		GROUP BY 1 ORDER BY 1`, rid, fromT, toEx)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	defer bRows.Close()
	var blabels []string
	var bcounts []int
	var noshows []int
	for bRows.Next() {
		var d time.Time
		var c, ns int
		_ = bRows.Scan(&d, &c, &ns)
		blabels = append(blabels, d.Format("2006-01-02"))
		bcounts = append(bcounts, c)
		noshows = append(noshows, ns)
	}

	topRows, err := a.Pool.Query(r.Context(), `
		SELECT mi.name, SUM(l.quantity)::int, COALESCE(SUM(l.quantity * mi.price_kopecks),0)::bigint
		FROM order_lines l
		JOIN menu_items mi ON mi.id = l.menu_item_id
		JOIN reservation_orders ro ON ro.id = l.order_id
		JOIN reservations r ON r.id = ro.reservation_id
		JOIN tables t ON t.id = r.table_id
		JOIN halls h ON h.id = t.hall_id
		WHERE h.restaurant_id = $1 AND ro.created_at >= $2 AND ro.created_at < $3
		GROUP BY mi.id, mi.name
		ORDER BY 2 DESC
		LIMIT 12`, rid, fromT, toEx)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	defer topRows.Close()
	topDishes := make([]map[string]any, 0)
	for topRows.Next() {
		var name string
		var qty int
		var rev int64
		_ = topRows.Scan(&name, &qty, &rev)
		topDishes = append(topDishes, map[string]any{"name": name, "quantity": qty, "revenue_kopecks": rev})
	}

	flopRows, err := a.Pool.Query(r.Context(), `
		SELECT mi.name, COALESCE(SUM(l.quantity),0)::int
		FROM menu_items mi
		LEFT JOIN order_lines l ON l.menu_item_id = mi.id
		LEFT JOIN reservation_orders ro ON ro.id = l.order_id AND ro.created_at >= $2 AND ro.created_at < $3
		WHERE mi.restaurant_id = $1
		GROUP BY mi.id, mi.name
		HAVING COALESCE(SUM(l.quantity),0) < 5
		ORDER BY 2 ASC
		LIMIT 8`, rid, fromT, toEx)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	defer flopRows.Close()
	flopDishes := make([]map[string]any, 0)
	for flopRows.Next() {
		var name string
		var qty int
		_ = flopRows.Scan(&name, &qty)
		flopDishes = append(flopDishes, map[string]any{"name": name, "quantity": qty})
	}

	var totalRev int64
	_ = a.Pool.QueryRow(r.Context(), `
		SELECT COALESCE(SUM(p.amount_kopecks) FILTER (WHERE p.status='succeeded'),0)::bigint
		FROM payments p
		JOIN reservations r ON r.id = p.reservation_id
		JOIN tables t ON t.id = r.table_id
		JOIN halls h ON h.id = t.hall_id
		WHERE h.restaurant_id = $1 AND p.created_at >= $2 AND p.created_at < $3`, rid, fromT, toEx).Scan(&totalRev)

	var depRev, tabRev int64
	_ = a.Pool.QueryRow(r.Context(), `
		SELECT
			COALESCE(SUM(p.amount_kopecks) FILTER (WHERE p.status='succeeded' AND p.purpose='deposit'),0)::bigint,
			COALESCE(SUM(p.amount_kopecks) FILTER (WHERE p.status='succeeded' AND p.purpose='tab'),0)::bigint
		FROM payments p
		JOIN reservations r ON r.id = p.reservation_id
		JOIN tables t ON t.id = r.table_id
		JOIN halls h ON h.id = t.hall_id
		WHERE h.restaurant_id = $1 AND p.created_at >= $2 AND p.created_at < $3`, rid, fromT, toEx).Scan(&depRev, &tabRev)

	var completedN int
	_ = a.Pool.QueryRow(r.Context(), `
		SELECT COUNT(*)::int FROM reservations r
		JOIN tables t ON t.id = r.table_id
		JOIN halls h ON h.id = t.hall_id
		WHERE h.restaurant_id = $1 AND r.status = 'completed'
		AND r.completed_at >= $2 AND r.completed_at < $3`, rid, fromT, toEx).Scan(&completedN)

	var avgCheck int64
	_ = a.Pool.QueryRow(r.Context(), `
		SELECT COALESCE(AVG(s.tot), 0)::bigint FROM (
			SELECT SUM(l.quantity * mi.price_kopecks)::bigint AS tot
			FROM reservation_orders ro
			JOIN order_lines l ON l.order_id = ro.id
			JOIN menu_items mi ON mi.id = l.menu_item_id
			JOIN reservations r ON r.id = ro.reservation_id
			JOIN tables t ON t.id = r.table_id
			JOIN halls h ON h.id = t.hall_id
			WHERE h.restaurant_id = $1 AND ro.status = 'closed'
			AND COALESCE(ro.updated_at, ro.created_at) >= $2 AND COALESCE(ro.updated_at, ro.created_at) < $3
			GROUP BY ro.id
		) s`, rid, fromT, toEx).Scan(&avgCheck)

	revLabels := make([]string, 0)
	revKop := make([]int64, 0)
	revDayRows, err := a.Pool.Query(r.Context(), `
		SELECT date_trunc('day', p.created_at)::date,
		       COALESCE(SUM(p.amount_kopecks) FILTER (WHERE p.status='succeeded'),0)::bigint
		FROM payments p
		JOIN reservations r ON r.id = p.reservation_id
		JOIN tables t ON t.id = r.table_id
		JOIN halls h ON h.id = t.hall_id
		WHERE h.restaurant_id = $1 AND p.created_at >= $2 AND p.created_at < $3
		GROUP BY 1 ORDER BY 1`, rid, fromT, toEx)
	if err == nil {
		defer revDayRows.Close()
		for revDayRows.Next() {
			var d time.Time
			var k int64
			_ = revDayRows.Scan(&d, &k)
			revLabels = append(revLabels, d.Format("2006-01-02"))
			revKop = append(revKop, k)
		}
	}

	a.json(w, http.StatusOK, map[string]any{
		"period_from": fromT.Format("2006-01-02"),
		"period_to":   toEx.Add(-24 * time.Hour).Format("2006-01-02"),
		"labels": labels, "load_percent": data,
		"bookings_labels": blabels, "bookings_count": bcounts, "no_show_count": noshows,
		"top_dishes": topDishes, "flop_dishes": flopDishes,
		"total_revenue_kopecks_90d": totalRev,
		"deposit_revenue_kopecks_90d": depRev,
		"tab_revenue_kopecks_90d":     tabRev,
		"completed_visits_90d":        completedN,
		"avg_closed_check_kopecks_90d": avgCheck,
		"revenue_by_day_labels":       revLabels,
		"revenue_by_day_kopecks":      revKop,
	})
}

func (a *Handlers) handleOwnerFinance(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	rid, ok := a.mustRestaurant(w, r, u)
	if !ok {
		return
	}
	fromT, toEx := ownerMoscowRangeFromRequest(r)
	rows, err := a.Pool.Query(r.Context(), `
		SELECT date_trunc('day', p.created_at)::date,
		       COUNT(*) FILTER (WHERE p.status='succeeded'),
		       COALESCE(SUM(p.amount_kopecks) FILTER (WHERE p.status='succeeded'),0),
		       COALESCE(SUM(p.refund_amount_kopecks),0)
		FROM payments p
		JOIN reservations r ON r.id = p.reservation_id
		JOIN tables t ON t.id = r.table_id
		JOIN halls h ON h.id = t.hall_id
		WHERE h.restaurant_id = $1 AND p.created_at >= $2 AND p.created_at < $3
		GROUP BY 1 ORDER BY 1`, rid, fromT, toEx)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	defer rows.Close()
	rowsOut := make([]map[string]any, 0)
	for rows.Next() {
		var d time.Time
		var cnt int
		var sum, ref int
		_ = rows.Scan(&d, &cnt, &sum, &ref)
		rowsOut = append(rowsOut, map[string]any{
			"date": d.Format("2006-01-02"), "payments": cnt,
			"deposits_kopecks": sum, "refunds_kopecks": ref,
		})
	}
	a.json(w, http.StatusOK, rowsOut)
}

func (a *Handlers) handleOwnerFinanceExport(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	rid, ok := a.mustRestaurant(w, r, u)
	if !ok {
		return
	}
	from := time.Now().AddDate(0, 0, -90)
	to := time.Now()
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
	rows, err := a.Pool.Query(r.Context(), `
		SELECT date_trunc('day', p.created_at)::date,
		       COUNT(*) FILTER (WHERE p.status='succeeded'),
		       COALESCE(SUM(p.amount_kopecks) FILTER (WHERE p.status='succeeded'),0),
		       COALESCE(SUM(p.refund_amount_kopecks),0)
		FROM payments p
		JOIN reservations r ON r.id = p.reservation_id
		JOIN tables t ON t.id = r.table_id
		JOIN halls h ON h.id = t.hall_id
		WHERE h.restaurant_id = $3 AND p.created_at >= $1 AND p.created_at < $2
		GROUP BY 1 ORDER BY 1`, from, to, rid)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	defer rows.Close()

	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	sheet := f.GetSheetName(0)
	_ = f.SetSheetName(sheet, "Платежи")
	headers := []string{"Дата", "Успешных платежей", "Депозиты (коп.)", "Возвраты (коп.)"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue("Платежи", cell, h)
	}
	rowIdx := 2
	for rows.Next() {
		var d time.Time
		var cnt int
		var sum, ref int
		_ = rows.Scan(&d, &cnt, &sum, &ref)
		for col, v := range []any{d.Format("2006-01-02"), cnt, sum, ref} {
			cell, _ := excelize.CoordinatesToCellName(col+1, rowIdx)
			_ = f.SetCellValue("Платежи", cell, v)
		}
		rowIdx++
	}

	_, _ = f.NewSheet("Закрытые_счета")
	sh2 := "Закрытые_счета"
	h2 := []string{"Дата завершения визита", "ID брони", "Стол", "Сумма заказа (коп.)"}
	for i, h := range h2 {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sh2, cell, h)
	}
	ordRows, err := a.Pool.Query(r.Context(), `
		SELECT r.completed_at, r.id, t.table_number,
		       COALESCE(SUM(l.quantity * mi.price_kopecks), 0)::bigint
		FROM reservations r
		JOIN tables t ON t.id = r.table_id
		JOIN halls h ON h.id = t.hall_id
		JOIN reservation_orders ro ON ro.reservation_id = r.id AND ro.status = 'closed'
		JOIN order_lines l ON l.order_id = ro.id
		JOIN menu_items mi ON mi.id = l.menu_item_id
		WHERE h.restaurant_id = $3
		AND r.status = 'completed'
		AND r.completed_at >= $1 AND r.completed_at < $2
		GROUP BY r.id, r.completed_at, t.table_number
		ORDER BY r.completed_at DESC
		LIMIT 500`, from, to, rid)
	if err == nil {
		defer ordRows.Close()
		r2 := 2
		for ordRows.Next() {
			var ct time.Time
			var rid2 uuid.UUID
			var tnum int
			var kop int64
			_ = ordRows.Scan(&ct, &rid2, &tnum, &kop)
			for col, v := range []any{ct.Format("2006-01-02 15:04"), rid2.String(), tnum, kop} {
				cell, _ := excelize.CoordinatesToCellName(col+1, r2)
				_ = f.SetCellValue(sh2, cell, v)
			}
			r2++
		}
	}

	_, _ = f.NewSheet("Строки_меню")
	sh3 := "Строки_меню"
	h3 := []string{"Дата", "Бронь", "Блюдо", "Кол-во", "Сумма строки (коп.)"}
	for i, h := range h3 {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sh3, cell, h)
	}
	lineRows, err := a.Pool.Query(r.Context(), `
		SELECT COALESCE(r.completed_at, ro.updated_at), r.id, mi.name, l.quantity,
		       (l.quantity * mi.price_kopecks)::bigint
		FROM order_lines l
		JOIN menu_items mi ON mi.id = l.menu_item_id
		JOIN reservation_orders ro ON ro.id = l.order_id
		JOIN reservations r ON r.id = ro.reservation_id
		JOIN tables t ON t.id = r.table_id
		JOIN halls h ON h.id = t.hall_id
		WHERE h.restaurant_id = $3
		AND COALESCE(r.completed_at, ro.updated_at) >= $1
		AND COALESCE(r.completed_at, ro.updated_at) < $2
		ORDER BY COALESCE(r.completed_at, ro.updated_at) DESC
		LIMIT 800`, from, to, rid)
	if err == nil {
		defer lineRows.Close()
		r3 := 2
		for lineRows.Next() {
			var dt time.Time
			var rid3 uuid.UUID
			var nm string
			var qty int
			var line int64
			_ = lineRows.Scan(&dt, &rid3, &nm, &qty, &line)
			for col, v := range []any{dt.Format("2006-01-02"), rid3.String(), nm, qty, line} {
				cell, _ := excelize.CoordinatesToCellName(col+1, r3)
				_ = f.SetCellValue(sh3, cell, v)
			}
			r3++
		}
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="finance-report.xlsx"`)
	w.WriteHeader(http.StatusOK)
	if err := f.Write(w); err != nil {
		return
	}
}

func (a *Handlers) handleStaffStats(w http.ResponseWriter, r *http.Request) {
	a.json(w, http.StatusOK, map[string]any{
		"waiters": []any{},
		"admins":  map[string]int{"bookings_handled": 0, "no_shows": 0},
	})
}

func (a *Handlers) handleAdminClientsList(w http.ResponseWriter, r *http.Request) {
	rows, err := a.Pool.Query(r.Context(), `
		SELECT id, email, full_name, phone FROM users
		WHERE role = 'client' AND COALESCE(status,'active') = 'active'
		ORDER BY full_name NULLS LAST, email LIMIT 500`)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id uuid.UUID
		var em, fn, ph string
		_ = rows.Scan(&id, &em, &fn, &ph)
		out = append(out, map[string]any{
			"id": id.String(), "email": em, "full_name": fn, "phone": ph,
		})
	}
	a.json(w, http.StatusOK, out)
}

func (a *Handlers) handleUsersList(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	rid, ok := a.mustRestaurant(w, r, u)
	if !ok {
		return
	}
	rows, err := a.Pool.Query(r.Context(), `
		SELECT id, email, full_name, phone, role, status, created_at FROM users
		WHERE restaurant_id = $1 OR id = (SELECT owner_user_id FROM restaurants WHERE id = $1)
		ORDER BY created_at DESC LIMIT 200`, rid)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id uuid.UUID
		var em, fn, ph, role, st string
		var ct time.Time
		_ = rows.Scan(&id, &em, &fn, &ph, &role, &st, &ct)
		out = append(out, map[string]any{
			"id": id.String(), "email": em, "full_name": fn, "phone": ph,
			"role": role, "status": st, "created_at": ct,
		})
	}
	a.json(w, http.StatusOK, out)
}

func (a *Handlers) handleUserCreate(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	rid, ok := a.mustRestaurant(w, r, u)
	if !ok {
		return
	}
	var body struct {
		Email    string `json:"email"`
		Role     string `json:"role"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Email == "" || body.Password == "" {
		a.err(w, http.StatusBadRequest, "данные")
		return
	}
	if body.Role != "admin" && body.Role != "waiter" {
		a.err(w, http.StatusBadRequest, "роль admin или waiter")
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(body.Password), a.Cfg.BcryptCost)
	var id uuid.UUID
	err := a.Pool.QueryRow(r.Context(), `
		INSERT INTO users (email, password_hash, full_name, phone, role, email_verified, restaurant_id)
		VALUES ($1,$2,$3,'+7000000000',$4,true,$5) RETURNING id`,
		body.Email, string(hash), body.Email, body.Role, rid).Scan(&id)
	if err != nil {
		a.err(w, http.StatusConflict, "email занят")
		return
	}
	a.json(w, http.StatusCreated, map[string]string{"id": id.String()})
}

func (a *Handlers) handleUserUpdate(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	rid, ok := a.mustRestaurant(w, r, u)
	if !ok {
		return
	}
	uid, err := uuid.Parse(chi.URLParam(r, "uid"))
	if err != nil {
		a.err(w, http.StatusBadRequest, "id")
		return
	}
	if uid == u.ID {
		a.err(w, http.StatusBadRequest, "нельзя изменить себя")
		return
	}
	var body struct {
		Role   *string `json:"role"`
		Status *string `json:"status"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	var rv, sv interface{}
	if body.Role != nil {
		rv = *body.Role
	}
	if body.Status != nil {
		sv = *body.Status
	}
	ct, err := a.Pool.Exec(r.Context(), `
		UPDATE users SET
			role = COALESCE($4::varchar, role),
			status = COALESCE($5::varchar, status),
			updated_at = NOW()
		WHERE id = $1 AND restaurant_id = $2 AND id != $3 AND role IN ('admin','waiter')`,
		uid, rid, u.ID, rv, sv)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "ошибка")
		return
	}
	if ct.RowsAffected() == 0 {
		a.err(w, http.StatusNotFound, "сотрудник не найден")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *Handlers) handleSettingsGet(w http.ResponseWriter, r *http.Request) {
	rows, err := a.Pool.Query(r.Context(), `SELECT key, value FROM settings`)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	defer rows.Close()
	m := map[string]json.RawMessage{}
	for rows.Next() {
		var k string
		var v []byte
		_ = rows.Scan(&k, &v)
		m[k] = v
	}
	a.json(w, http.StatusOK, m)
}

func (a *Handlers) handleSettingsPut(w http.ResponseWriter, r *http.Request) {
	var body map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		a.err(w, http.StatusBadRequest, "json")
		return
	}
	for k, v := range body {
		_, _ = a.Pool.Exec(r.Context(), `INSERT INTO settings(key,value) VALUES($1,$2::jsonb) ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value`, k, string(v))
	}
	w.WriteHeader(http.StatusNoContent)
}
