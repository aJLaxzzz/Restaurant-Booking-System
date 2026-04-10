package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (a *Handlers) menuAdminRestaurant(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	u := userFrom(r)
	rid, ok := a.mustRestaurant(w, r, u)
	if !ok {
		return uuid.Nil, false
	}
	return rid, true
}

func (a *Handlers) handleMenuCategoriesList(w http.ResponseWriter, r *http.Request) {
	rid, ok := a.menuAdminRestaurant(w, r)
	if !ok {
		return
	}
	rows, err := a.Pool.Query(r.Context(), `
		SELECT id, parent_id, name, sort_order, is_active
		FROM menu_categories WHERE restaurant_id=$1 ORDER BY sort_order, name`, rid)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id uuid.UUID
		var pid uuid.NullUUID
		var name string
		var so int
		var active bool
		_ = rows.Scan(&id, &pid, &name, &so, &active)
		m := map[string]any{"id": id.String(), "name": name, "sort_order": so, "is_active": active}
		if pid.Valid {
			m["parent_id"] = pid.UUID.String()
		}
		out = append(out, m)
	}
	a.json(w, http.StatusOK, out)
}

func (a *Handlers) handleMenuCategoryCreate(w http.ResponseWriter, r *http.Request) {
	rid, ok := a.menuAdminRestaurant(w, r)
	if !ok {
		return
	}
	var body struct {
		Name      string     `json:"name"`
		ParentID  *uuid.UUID `json:"parent_id"`
		SortOrder int        `json:"sort_order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		a.err(w, http.StatusBadRequest, "данные")
		return
	}
	if body.ParentID != nil {
		var pr uuid.UUID
		err := a.Pool.QueryRow(r.Context(), `SELECT restaurant_id FROM menu_categories WHERE id=$1`, *body.ParentID).Scan(&pr)
		if err != nil || pr != rid {
			a.err(w, http.StatusBadRequest, "категория")
			return
		}
	}
	var id uuid.UUID
	err := a.Pool.QueryRow(r.Context(), `
		INSERT INTO menu_categories (restaurant_id, parent_id, name, sort_order)
		VALUES ($1,$2,$3,$4) RETURNING id`,
		rid, body.ParentID, body.Name, body.SortOrder).Scan(&id)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "не создано")
		return
	}
	a.json(w, http.StatusCreated, map[string]string{"id": id.String()})
}

func (a *Handlers) handleMenuCategoryUpdate(w http.ResponseWriter, r *http.Request) {
	rid, ok := a.menuAdminRestaurant(w, r)
	if !ok {
		return
	}
	cid, err := uuid.Parse(chi.URLParam(r, "cid"))
	if err != nil {
		a.err(w, http.StatusBadRequest, "id")
		return
	}
	var body struct {
		Name      *string `json:"name"`
		SortOrder *int    `json:"sort_order"`
		IsActive  *bool   `json:"is_active"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	ct, err := a.Pool.Exec(r.Context(), `
		UPDATE menu_categories SET
			name = COALESCE($3, name),
			sort_order = COALESCE($4, sort_order),
			is_active = COALESCE($5, is_active)
		WHERE id=$1 AND restaurant_id=$2`,
		cid, rid, body.Name, body.SortOrder, body.IsActive)
	if err != nil || ct.RowsAffected() == 0 {
		a.err(w, http.StatusNotFound, "не найдено")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *Handlers) handleMenuCategoryDelete(w http.ResponseWriter, r *http.Request) {
	rid, ok := a.menuAdminRestaurant(w, r)
	if !ok {
		return
	}
	cid, err := uuid.Parse(chi.URLParam(r, "cid"))
	if err != nil {
		a.err(w, http.StatusBadRequest, "id")
		return
	}
	_, err = a.Pool.Exec(r.Context(), `DELETE FROM menu_categories WHERE id=$1 AND restaurant_id=$2`, cid, rid)
	if err != nil {
		a.err(w, http.StatusConflict, "есть позиции или подкатегории")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *Handlers) handleMenuItemsList(w http.ResponseWriter, r *http.Request) {
	rid, ok := a.menuAdminRestaurant(w, r)
	if !ok {
		return
	}
	rows, err := a.Pool.Query(r.Context(), `
		SELECT id, category_id, name, COALESCE(description,''), price_kopecks, is_available, sort_order, COALESCE(image_url,'')
		FROM menu_items WHERE restaurant_id=$1 ORDER BY sort_order, name`, rid)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, cat uuid.UUID
		var name, desc, img string
		var price, so int
		var av bool
		_ = rows.Scan(&id, &cat, &name, &desc, &price, &av, &so, &img)
		out = append(out, map[string]any{
			"id": id.String(), "category_id": cat.String(), "name": name, "description": desc,
			"price_kopecks": price, "is_available": av, "sort_order": so, "image_url": img,
		})
	}
	a.json(w, http.StatusOK, out)
}

func (a *Handlers) handleMenuItemCreate(w http.ResponseWriter, r *http.Request) {
	rid, ok := a.menuAdminRestaurant(w, r)
	if !ok {
		return
	}

	var (
		categoryID   uuid.UUID
		name         string
		description  string
		imageURL     string
		priceKopecks int
		sortOrder    int
	)

	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(uploadMaxBytes + 2048); err != nil {
			a.err(w, http.StatusBadRequest, "форма")
			return
		}
		var err error
		categoryID, err = uuid.Parse(strings.TrimSpace(r.FormValue("category_id")))
		if err != nil || categoryID == uuid.Nil {
			a.err(w, http.StatusBadRequest, "категория")
			return
		}
		name = strings.TrimSpace(r.FormValue("name"))
		description = strings.TrimSpace(r.FormValue("description"))
		priceKopecks, _ = strconv.Atoi(strings.TrimSpace(r.FormValue("price_kopecks")))
		if priceKopecks == 0 {
			if pk := strings.TrimSpace(r.FormValue("price_rub")); pk != "" {
				if v, e := strconv.ParseFloat(strings.ReplaceAll(pk, ",", "."), 64); e == nil {
					priceKopecks = int(v * 100)
				}
			}
		}
		sortOrder, _ = strconv.Atoi(strings.TrimSpace(r.FormValue("sort_order")))
		if r.MultipartForm != nil {
			if fs := r.MultipartForm.File["photo"]; len(fs) > 0 && fs[0].Size > 0 {
				url, err := a.saveMultipartField(r, "photo")
				if err != nil {
					a.err(w, http.StatusBadRequest, err.Error())
					return
				}
				imageURL = url
			}
		}
	} else {
		var body struct {
			CategoryID   uuid.UUID `json:"category_id"`
			Name         string    `json:"name"`
			Description  string    `json:"description"`
			ImageURL     string    `json:"image_url"`
			PriceKopecks int       `json:"price_kopecks"`
			SortOrder    int       `json:"sort_order"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" || body.CategoryID == uuid.Nil {
			a.err(w, http.StatusBadRequest, "данные")
			return
		}
		categoryID = body.CategoryID
		name = body.Name
		description = body.Description
		imageURL = body.ImageURL
		priceKopecks = body.PriceKopecks
		sortOrder = body.SortOrder
	}

	if name == "" || categoryID == uuid.Nil {
		a.err(w, http.StatusBadRequest, "данные")
		return
	}
	var cr uuid.UUID
	err := a.Pool.QueryRow(r.Context(), `SELECT restaurant_id FROM menu_categories WHERE id=$1`, categoryID).Scan(&cr)
	if err != nil || cr != rid {
		a.err(w, http.StatusBadRequest, "категория")
		return
	}
	var id uuid.UUID
	err = a.Pool.QueryRow(r.Context(), `
		INSERT INTO menu_items (restaurant_id, category_id, name, description, image_url, price_kopecks, sort_order)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		rid, categoryID, name, description, imageURL, priceKopecks, sortOrder).Scan(&id)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "не создано")
		return
	}
	a.json(w, http.StatusCreated, map[string]string{"id": id.String(), "image_url": imageURL})
}

func (a *Handlers) handleMenuItemUpdate(w http.ResponseWriter, r *http.Request) {
	rid, ok := a.menuAdminRestaurant(w, r)
	if !ok {
		return
	}
	iid, err := uuid.Parse(chi.URLParam(r, "iid"))
	if err != nil {
		a.err(w, http.StatusBadRequest, "id")
		return
	}
	var body struct {
		Name         *string `json:"name"`
		Description  *string `json:"description"`
		ImageURL     *string `json:"image_url"`
		PriceKopecks *int    `json:"price_kopecks"`
		IsAvailable  *bool   `json:"is_available"`
		SortOrder    *int    `json:"sort_order"`
		CategoryID   *uuid.UUID `json:"category_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.CategoryID != nil {
		var cr uuid.UUID
		err := a.Pool.QueryRow(r.Context(), `SELECT restaurant_id FROM menu_categories WHERE id=$1`, *body.CategoryID).Scan(&cr)
		if err != nil || cr != rid {
			a.err(w, http.StatusBadRequest, "категория")
			return
		}
	}
	ct, err := a.Pool.Exec(r.Context(), `
		UPDATE menu_items SET
			name = COALESCE($3, name),
			description = COALESCE($4, description),
			image_url = COALESCE($5, image_url),
			price_kopecks = COALESCE($6, price_kopecks),
			is_available = COALESCE($7, is_available),
			sort_order = COALESCE($8, sort_order),
			category_id = COALESCE($9, category_id),
			updated_at = NOW()
		WHERE id=$1 AND restaurant_id=$2`,
		iid, rid, body.Name, body.Description, body.ImageURL, body.PriceKopecks, body.IsAvailable, body.SortOrder, body.CategoryID)
	if err != nil || ct.RowsAffected() == 0 {
		a.err(w, http.StatusNotFound, "не найдено")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *Handlers) handleMenuItemDelete(w http.ResponseWriter, r *http.Request) {
	rid, ok := a.menuAdminRestaurant(w, r)
	if !ok {
		return
	}
	iid, err := uuid.Parse(chi.URLParam(r, "iid"))
	if err != nil {
		a.err(w, http.StatusBadRequest, "id")
		return
	}
	ct, err := a.Pool.Exec(r.Context(), `DELETE FROM menu_items WHERE id=$1 AND restaurant_id=$2`, iid, rid)
	if err != nil || ct.RowsAffected() == 0 {
		a.err(w, http.StatusNotFound, "не найдено")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *Handlers) MountMenuAdmin(r chi.Router) {
	r.Get("/admin/menu/categories", a.handleMenuCategoriesList)
	r.Post("/admin/menu/categories", a.handleMenuCategoryCreate)
	r.Put("/admin/menu/categories/{cid}", a.handleMenuCategoryUpdate)
	r.Delete("/admin/menu/categories/{cid}", a.handleMenuCategoryDelete)
	r.Get("/admin/menu/items", a.handleMenuItemsList)
	r.Post("/admin/menu/items", a.handleMenuItemCreate)
	r.Put("/admin/menu/items/{iid}", a.handleMenuItemUpdate)
	r.Delete("/admin/menu/items/{iid}", a.handleMenuItemDelete)
}
