package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (a *Handlers) handleSuperadminOwnerApplications(w http.ResponseWriter, r *http.Request) {
	rows, err := a.Pool.Query(r.Context(), `
		SELECT id, email, full_name, phone, created_at
		FROM users
		WHERE owner_application_status = 'pending'
		ORDER BY created_at ASC`)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var id uuid.UUID
		var email, fn, phone string
		var created interface{}
		if err := rows.Scan(&id, &email, &fn, &phone, &created); err != nil {
			a.err(w, http.StatusInternalServerError, "БД")
			return
		}
		out = append(out, map[string]any{
			"id": id.String(), "email": email, "full_name": fn, "phone": phone, "created_at": created,
		})
	}
	a.json(w, http.StatusOK, out)
}

func (a *Handlers) handleSuperadminOwnerApprove(w http.ResponseWriter, r *http.Request) {
	uid, err := uuid.Parse(chi.URLParam(r, "uid"))
	if err != nil {
		a.err(w, http.StatusBadRequest, "id")
		return
	}
	ct, err := a.Pool.Exec(r.Context(), `
		UPDATE users SET role='owner', owner_application_status=NULL, updated_at=NOW()
		WHERE id=$1 AND owner_application_status='pending'`, uid)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	if ct.RowsAffected() == 0 {
		a.err(w, http.StatusNotFound, "нет заявки")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *Handlers) handleSuperadminOwnerReject(w http.ResponseWriter, r *http.Request) {
	uid, err := uuid.Parse(chi.URLParam(r, "uid"))
	if err != nil {
		a.err(w, http.StatusBadRequest, "id")
		return
	}
	ct, err := a.Pool.Exec(r.Context(), `
		UPDATE users SET owner_application_status='rejected', updated_at=NOW()
		WHERE id=$1 AND owner_application_status='pending'`, uid)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	if ct.RowsAffected() == 0 {
		a.err(w, http.StatusNotFound, "нет заявки")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *Handlers) handleSuperadminRestaurants(w http.ResponseWriter, r *http.Request) {
	rows, err := a.Pool.Query(r.Context(), `
		SELECT r.id, r.name, r.slug, r.city, u.email
		FROM restaurants r
		LEFT JOIN users u ON u.id = r.owner_user_id
		ORDER BY r.name`)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var id uuid.UUID
		var name, slug, city string
		var ownerEmail *string
		if err := rows.Scan(&id, &name, &slug, &city, &ownerEmail); err != nil {
			a.err(w, http.StatusInternalServerError, "БД")
			return
		}
		m := map[string]any{"id": id.String(), "name": name, "slug": slug, "city": city}
		if ownerEmail != nil {
			m["owner_email"] = *ownerEmail
		}
		out = append(out, m)
	}
	a.json(w, http.StatusOK, out)
}

func (a *Handlers) handleSuperadminUsers(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("q")))
	args := []any{}
	where := `WHERE 1=1`
	if q != "" {
		where += ` AND (lower(email) LIKE $1 OR lower(full_name) LIKE $1)`
		args = append(args, "%"+q+"%")
	}
	sqlStr := `SELECT id, email, full_name, phone, role, COALESCE(status,'active'), COALESCE(owner_application_status,''), created_at FROM users ` + where + ` ORDER BY created_at DESC LIMIT 200`
	var rows pgx.Rows
	var err error
	if len(args) > 0 {
		rows, err = a.Pool.Query(r.Context(), sqlStr, args...)
	} else {
		rows, err = a.Pool.Query(r.Context(), sqlStr)
	}
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var id uuid.UUID
		var email, fn, phone, role, st, oas string
		var created interface{}
		if err := rows.Scan(&id, &email, &fn, &phone, &role, &st, &oas, &created); err != nil {
			a.err(w, http.StatusInternalServerError, "БД")
			return
		}
		m := map[string]any{
			"id": id.String(), "email": email, "full_name": fn, "phone": phone, "role": role, "status": st, "created_at": created,
		}
		if oas != "" {
			m["owner_application_status"] = oas
		}
		out = append(out, m)
	}
	a.json(w, http.StatusOK, out)
}

func (a *Handlers) handleSuperadminUserPut(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r)
	uid, err := uuid.Parse(chi.URLParam(r, "uid"))
	if err != nil {
		a.err(w, http.StatusBadRequest, "id")
		return
	}
	var body struct {
		Role     *string `json:"role"`
		Status   *string `json:"status"`
		FullName *string `json:"full_name"`
		Phone    *string `json:"phone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		a.err(w, http.StatusBadRequest, "JSON")
		return
	}
	if body.Role == nil && body.Status == nil && body.FullName == nil && body.Phone == nil {
		a.err(w, http.StatusBadRequest, "укажите поля")
		return
	}
	if body.Role != nil {
		rval := strings.TrimSpace(*body.Role)
		switch rval {
		case "client", "owner", "admin", "waiter", "superadmin":
		default:
			a.err(w, http.StatusBadRequest, "недопустимая роль")
			return
		}
		if uid == me.ID && rval != "superadmin" {
			var n int
			_ = a.Pool.QueryRow(r.Context(), `SELECT COUNT(*)::int FROM users WHERE role='superadmin'`).Scan(&n)
			if n < 2 {
				a.err(w, http.StatusForbidden, "нельзя снять последнего суперадмина")
				return
			}
		}
	}
	if body.Status != nil {
		st := strings.TrimSpace(*body.Status)
		if st != "active" && st != "blocked" {
			a.err(w, http.StatusBadRequest, "status: active или blocked")
			return
		}
	}
	if body.FullName != nil && strings.TrimSpace(*body.FullName) == "" {
		a.err(w, http.StatusBadRequest, "full_name")
		return
	}
	var sets []string
	args := []any{}
	ph := 1
	if body.Role != nil {
		sets = append(sets, fmt.Sprintf("role=$%d", ph))
		args = append(args, strings.TrimSpace(*body.Role))
		ph++
	}
	if body.Status != nil {
		sets = append(sets, fmt.Sprintf("status=$%d", ph))
		args = append(args, strings.TrimSpace(*body.Status))
		ph++
	}
	if body.FullName != nil {
		sets = append(sets, fmt.Sprintf("full_name=$%d", ph))
		args = append(args, strings.TrimSpace(*body.FullName))
		ph++
	}
	if body.Phone != nil {
		sets = append(sets, fmt.Sprintf("phone=$%d", ph))
		args = append(args, strings.TrimSpace(*body.Phone))
		ph++
	}
	args = append(args, uid)
	q := "UPDATE users SET " + strings.Join(sets, ", ") + ", updated_at=NOW() WHERE id=$" + strconv.Itoa(ph)
	_, err = a.Pool.Exec(r.Context(), q, args...)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *Handlers) handleSuperadminRestaurantPut(w http.ResponseWriter, r *http.Request) {
	rid, err := uuid.Parse(chi.URLParam(r, "rid"))
	if err != nil {
		a.err(w, http.StatusBadRequest, "id")
		return
	}
	var body struct {
		Name         *string    `json:"name"`
		Slug         *string    `json:"slug"`
		City         *string    `json:"city"`
		Address      *string    `json:"address"`
		Description  *string    `json:"description"`
		OwnerUserID  *uuid.UUID `json:"owner_user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		a.err(w, http.StatusBadRequest, "JSON")
		return
	}
	if body.Name != nil {
		_, err = a.Pool.Exec(r.Context(), `UPDATE restaurants SET name=$2, updated_at=NOW() WHERE id=$1`, rid, strings.TrimSpace(*body.Name))
		if err != nil {
			a.err(w, http.StatusInternalServerError, "БД")
			return
		}
	}
	if body.Slug != nil {
		s := strings.ToLower(strings.TrimSpace(*body.Slug))
		if s != "" && !slugRe.MatchString(s) {
			a.err(w, http.StatusBadRequest, "slug")
			return
		}
		if s != "" {
			_, err = a.Pool.Exec(r.Context(), `UPDATE restaurants SET slug=$2, updated_at=NOW() WHERE id=$1`, rid, s)
			if err != nil {
				if strings.Contains(err.Error(), "unique") {
					a.err(w, http.StatusConflict, "slug занят")
					return
				}
				a.err(w, http.StatusInternalServerError, "БД")
				return
			}
		}
	}
	if body.City != nil {
		_, _ = a.Pool.Exec(r.Context(), `UPDATE restaurants SET city=$2, updated_at=NOW() WHERE id=$1`, rid, strings.TrimSpace(*body.City))
	}
	if body.Address != nil {
		_, _ = a.Pool.Exec(r.Context(), `UPDATE restaurants SET address=$2, updated_at=NOW() WHERE id=$1`, rid, strings.TrimSpace(*body.Address))
	}
	if body.Description != nil {
		_, _ = a.Pool.Exec(r.Context(), `UPDATE restaurants SET description=$2, updated_at=NOW() WHERE id=$1`, rid, strings.TrimSpace(*body.Description))
	}
	if body.OwnerUserID != nil {
		_, err = a.Pool.Exec(r.Context(), `UPDATE restaurants SET owner_user_id=$2, updated_at=NOW() WHERE id=$1`, rid, *body.OwnerUserID)
		if err != nil {
			if strings.Contains(err.Error(), "unique") {
				a.err(w, http.StatusConflict, "у пользователя уже есть ресторан")
				return
			}
			a.err(w, http.StatusInternalServerError, "БД")
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *Handlers) handleSuperadminRestaurantSettingsGet(w http.ResponseWriter, r *http.Request) {
	rid, err := uuid.Parse(chi.URLParam(r, "rid"))
	if err != nil {
		a.err(w, http.StatusBadRequest, "id")
		return
	}
	ctx := r.Context()
	m := map[string]json.RawMessage{}
	for _, k := range bookingSettingKeysList() {
		v := a.getSettingIntForRestaurant(ctx, rid, k, defaultIntForBookingKey(k))
		b, _ := json.Marshal(v)
		m[k] = b
	}
	a.json(w, http.StatusOK, m)
}

func (a *Handlers) handleSuperadminRestaurantSettingsPut(w http.ResponseWriter, r *http.Request) {
	rid, err := uuid.Parse(chi.URLParam(r, "rid"))
	if err != nil {
		a.err(w, http.StatusBadRequest, "id")
		return
	}
	var body map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		a.err(w, http.StatusBadRequest, "json")
		return
	}
	allowed := map[string]struct{}{}
	for _, k := range bookingSettingKeysList() {
		allowed[k] = struct{}{}
	}
	for k, v := range body {
		if _, ok := allowed[k]; !ok {
			continue
		}
		_, _ = a.Pool.Exec(r.Context(), `INSERT INTO restaurant_settings(restaurant_id,key,value) VALUES($1,$2,$3::jsonb) ON CONFLICT(restaurant_id,key) DO UPDATE SET value=EXCLUDED.value`, rid, k, string(v))
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *Handlers) MountSuperadmin(r chi.Router) {
	r.Get("/owner-applications", a.handleSuperadminOwnerApplications)
	r.Post("/owner-applications/{uid}/approve", a.handleSuperadminOwnerApprove)
	r.Post("/owner-applications/{uid}/reject", a.handleSuperadminOwnerReject)
	r.Get("/restaurants", a.handleSuperadminRestaurants)
	r.Get("/restaurants/{rid}/settings", a.handleSuperadminRestaurantSettingsGet)
	r.Put("/restaurants/{rid}/settings", a.handleSuperadminRestaurantSettingsPut)
	r.Put("/restaurants/{rid}", a.handleSuperadminRestaurantPut)
	r.Get("/users", a.handleSuperadminUsers)
	r.Put("/users/{uid}", a.handleSuperadminUserPut)
	r.Get("/settings", a.handleSettingsGet)
	r.Put("/settings", a.handleSettingsPut)
}
