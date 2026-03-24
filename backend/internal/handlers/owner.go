package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
	"golang.org/x/crypto/bcrypt"
)

func (a *Handlers) handleOwnerAnalytics(w http.ResponseWriter, r *http.Request) {
	rows, err := a.Pool.Query(r.Context(), `
		SELECT date_trunc('day', start_time)::date as d,
		       COUNT(*)::float / NULLIF((SELECT COUNT(*) FROM tables), 0) * 100 as load
		FROM reservations
		WHERE status IN ('confirmed','seated','in_service','completed')
		AND start_time > NOW() - INTERVAL '30 days'
		GROUP BY 1 ORDER BY 1`)
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
	a.json(w, http.StatusOK, map[string]any{"labels": labels, "load_percent": data})
}

func (a *Handlers) handleOwnerFinance(w http.ResponseWriter, r *http.Request) {
	rows, err := a.Pool.Query(r.Context(), `
		SELECT date_trunc('day', p.created_at)::date,
		       COUNT(*) FILTER (WHERE p.status='succeeded'),
		       COALESCE(SUM(p.amount_kopecks) FILTER (WHERE p.status='succeeded'),0),
		       COALESCE(SUM(p.refund_amount_kopecks),0)
		FROM payments p
		WHERE p.created_at > NOW() - INTERVAL '90 days'
		GROUP BY 1 ORDER BY 1`)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	defer rows.Close()
	var rowsOut []map[string]any
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
		WHERE p.created_at >= $1 AND p.created_at < $2
		GROUP BY 1 ORDER BY 1`, from, to)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	defer rows.Close()

	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	sheet := f.GetSheetName(0)
	headers := []string{"Дата", "Успешных платежей", "Депозиты (коп.)", "Возвраты (коп.)"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
	}
	rowIdx := 2
	for rows.Next() {
		var d time.Time
		var cnt int
		var sum, ref int
		_ = rows.Scan(&d, &cnt, &sum, &ref)
		for col, v := range []any{d.Format("2006-01-02"), cnt, sum, ref} {
			cell, _ := excelize.CoordinatesToCellName(col+1, rowIdx)
			_ = f.SetCellValue(sheet, cell, v)
		}
		rowIdx++
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
	rows, err := a.Pool.Query(r.Context(), `
		SELECT id, email, full_name, phone, role, status, created_at FROM users ORDER BY created_at DESC LIMIT 200`)
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
	var body struct {
		Email    string `json:"email"`
		Role     string `json:"role"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Email == "" || body.Password == "" {
		a.err(w, http.StatusBadRequest, "данные")
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(body.Password), a.Cfg.BcryptCost)
	var id uuid.UUID
	err := a.Pool.QueryRow(r.Context(), `
		INSERT INTO users (email, password_hash, full_name, phone, role, email_verified)
		VALUES ($1,$2,$3,'+7000000000',$4,true) RETURNING id`,
		body.Email, string(hash), body.Email, body.Role).Scan(&id)
	if err != nil {
		a.err(w, http.StatusConflict, "email занят")
		return
	}
	a.json(w, http.StatusCreated, map[string]string{"id": id.String()})
}

func (a *Handlers) handleUserUpdate(w http.ResponseWriter, r *http.Request) {
	uid, err := uuid.Parse(chi.URLParam(r, "uid"))
	if err != nil {
		a.err(w, http.StatusBadRequest, "id")
		return
	}
	var body struct {
		Role   *string `json:"role"`
		Status *string `json:"status"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	_, err = a.Pool.Exec(r.Context(), `
		UPDATE users SET role=COALESCE($2,role), status=COALESCE($3,status), updated_at=NOW() WHERE id=$1`,
		uid, body.Role, body.Status)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "ошибка")
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
