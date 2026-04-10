package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var slugRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func (a *Handlers) handleOwnerRestaurantCreate(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	if u.Role != "owner" {
		a.err(w, http.StatusForbidden, "только владелец")
		return
	}
	var n int
	_ = a.Pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM restaurants WHERE owner_user_id=$1`, u.ID).Scan(&n)
	if n > 0 {
		a.err(w, http.StatusConflict, "ресторан уже создан")
		return
	}
	var body struct {
		Name        string `json:"name"`
		Address     string `json:"address"`
		City        string `json:"city"`
		Slug        string `json:"slug"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Name) == "" {
		a.err(w, http.StatusBadRequest, "укажите название")
		return
	}
	slug := strings.ToLower(strings.TrimSpace(body.Slug))
	if slug == "" {
		slug = slugify(body.Name)
		if slug == "" {
			slug = "restaurant-" + u.ID.String()[:8]
		}
	}
	if !slugRe.MatchString(slug) {
		a.err(w, http.StatusBadRequest, "slug: латиница, цифры и дефис")
		return
	}
	rid := uuid.New()
	hid := uuid.New()
	tx, err := a.Pool.Begin(r.Context())
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()

	_, err = tx.Exec(r.Context(), `
		INSERT INTO restaurants (id, name, address, slug, city, description, owner_user_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		rid, strings.TrimSpace(body.Name), strings.TrimSpace(body.Address), slug,
		strings.TrimSpace(body.City), strings.TrimSpace(body.Description), u.ID)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			a.err(w, http.StatusConflict, "slug или название занято")
			return
		}
		a.err(w, http.StatusInternalServerError, "не создано")
		return
	}
	_, err = tx.Exec(r.Context(), `
		INSERT INTO halls (id, restaurant_id, name, layout_json)
		VALUES ($1,$2,'Основной зал', '{"tables":[],"walls":[]}'::jsonb)`, hid, rid)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "зал")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	a.json(w, http.StatusCreated, map[string]string{"id": rid.String(), "hall_id": hid.String()})
}

func slugify(s string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == ' ' || r == '-' || r == '_':
			if b.Len() > 0 && !lastDash {
				b.WriteRune('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	return out
}

func (a *Handlers) handleRestaurantUpdate(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	rid, ok := a.mustRestaurant(w, r, u)
	if !ok {
		return
	}
	if u.Role != "owner" && u.Role != "admin" {
		a.err(w, http.StatusForbidden, "нет доступа")
		return
	}
	if !a.userMayAccessRestaurant(w, r, u, rid) {
		a.err(w, http.StatusForbidden, "нет доступа")
		return
	}
	var body struct {
		Name          *string          `json:"name"`
		Address       *string          `json:"address"`
		City          *string          `json:"city"`
		Slug          *string          `json:"slug"`
		Description   *string          `json:"description"`
		PhotoURL      *string          `json:"photo_url"`
		Phone         *string          `json:"phone"`
		OpensAt       *string          `json:"opens_at"`
		ClosesAt      *string          `json:"closes_at"`
		ExtraJSON     *json.RawMessage `json:"extra_json"`
		ContactEmail  *string          `json:"contact_email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		a.err(w, http.StatusBadRequest, "json")
		return
	}
	if body.Slug != nil && *body.Slug != "" && !slugRe.MatchString(strings.ToLower(strings.TrimSpace(*body.Slug))) {
		a.err(w, http.StatusBadRequest, "slug")
		return
	}
	var sets []string
	var args []any
	args = append(args, rid)
	n := 2
	if body.Name != nil {
		sets = append(sets, fmt.Sprintf("name=$%d", n))
		args = append(args, strings.TrimSpace(*body.Name))
		n++
	}
	if body.Address != nil {
		sets = append(sets, fmt.Sprintf("address=$%d", n))
		args = append(args, strings.TrimSpace(*body.Address))
		n++
	}
	if body.City != nil {
		sets = append(sets, fmt.Sprintf("city=$%d", n))
		args = append(args, strings.TrimSpace(*body.City))
		n++
	}
	if body.Slug != nil {
		sets = append(sets, fmt.Sprintf("slug=$%d", n))
		args = append(args, strings.TrimSpace(strings.ToLower(*body.Slug)))
		n++
	}
	if body.Description != nil {
		sets = append(sets, fmt.Sprintf("description=$%d", n))
		args = append(args, strings.TrimSpace(*body.Description))
		n++
	}
	if body.PhotoURL != nil {
		sets = append(sets, fmt.Sprintf("photo_url=$%d", n))
		args = append(args, *body.PhotoURL)
		n++
	}
	if body.Phone != nil {
		sets = append(sets, fmt.Sprintf("phone=$%d", n))
		args = append(args, strings.TrimSpace(*body.Phone))
		n++
	}
	if body.OpensAt != nil {
		sets = append(sets, fmt.Sprintf("opens_at=$%d", n))
		args = append(args, strings.TrimSpace(*body.OpensAt))
		n++
	}
	if body.ClosesAt != nil {
		sets = append(sets, fmt.Sprintf("closes_at=$%d", n))
		args = append(args, strings.TrimSpace(*body.ClosesAt))
		n++
	}
	if body.ContactEmail != nil || (body.ExtraJSON != nil && len(*body.ExtraJSON) > 0) {
		merged, err := a.mergeRestaurantExtraJSON(r.Context(), rid, body.ContactEmail, body.ExtraJSON)
		if err != nil {
			a.err(w, http.StatusBadRequest, "extra_json")
			return
		}
		sets = append(sets, fmt.Sprintf("extra_json=$%d::jsonb", n))
		args = append(args, merged)
		n++
	}
	if len(sets) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	q := "UPDATE restaurants SET " + strings.Join(sets, ", ") + fmt.Sprintf(" WHERE id=$1")
	_, err := a.Pool.Exec(r.Context(), q, args...)
	if err != nil {
		if strings.Contains(err.Error(), "unique") {
			a.err(w, http.StatusConflict, "slug занят")
			return
		}
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *Handlers) handleUploadRestaurantPhoto(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	rid, ok := a.mustRestaurant(w, r, u)
	if !ok {
		return
	}
	if u.Role != "owner" && u.Role != "admin" {
		a.err(w, http.StatusForbidden, "нет доступа")
		return
	}
	if !a.userMayAccessRestaurant(w, r, u, rid) {
		a.err(w, http.StatusForbidden, "нет доступа")
		return
	}
	url, err := a.saveUploadFile(r, "photo")
	if err != nil {
		a.err(w, http.StatusBadRequest, err.Error())
		return
	}
	_, err = a.Pool.Exec(r.Context(), `UPDATE restaurants SET photo_url=$2 WHERE id=$1`, rid, url)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	a.json(w, http.StatusOK, map[string]string{"url": url})
}

func (a *Handlers) handleUploadMenuItemPhoto(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	rid, ok := a.mustRestaurant(w, r, u)
	if !ok {
		return
	}
	if u.Role != "owner" && u.Role != "admin" {
		a.err(w, http.StatusForbidden, "нет доступа")
		return
	}
	iid, err := uuid.Parse(strings.TrimSpace(r.URL.Query().Get("item_id")))
	if err != nil {
		a.err(w, http.StatusBadRequest, "item_id")
		return
	}
	var cr uuid.UUID
	err = a.Pool.QueryRow(r.Context(), `SELECT restaurant_id FROM menu_items WHERE id=$1`, iid).Scan(&cr)
	if err == pgx.ErrNoRows {
		a.err(w, http.StatusNotFound, "позиция")
		return
	}
	if err != nil || cr != rid {
		a.err(w, http.StatusForbidden, "нет доступа")
		return
	}
	url, err := a.saveUploadFile(r, "photo")
	if err != nil {
		a.err(w, http.StatusBadRequest, err.Error())
		return
	}
	_, err = a.Pool.Exec(r.Context(), `UPDATE menu_items SET image_url=$2, updated_at=NOW() WHERE id=$1`, iid, url)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	a.json(w, http.StatusOK, map[string]string{"url": url})
}

func (a *Handlers) mergeRestaurantExtraJSON(ctx context.Context, rid uuid.UUID, contactEmail *string, patch *json.RawMessage) ([]byte, error) {
	var raw []byte
	err := a.Pool.QueryRow(ctx, `SELECT COALESCE(extra_json::text, '{}') FROM restaurants WHERE id=$1`, rid).Scan(&raw)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		m = map[string]interface{}{}
	}
	if contactEmail != nil {
		m["contact_email"] = strings.TrimSpace(*contactEmail)
	}
	if patch != nil && len(*patch) > 0 {
		var p map[string]interface{}
		if err := json.Unmarshal(*patch, &p); err != nil {
			return nil, err
		}
		for k, v := range p {
			m[k] = v
		}
	}
	return json.Marshal(m)
}
