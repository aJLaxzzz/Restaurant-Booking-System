package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func (a *Handlers) handleRestaurantsList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := a.Pool.Query(ctx, `
		SELECT
		  r.id,
		  COALESCE(r.name,''),
		  COALESCE(r.slug,''),
		  COALESCE(r.city,''),
		  COALESCE(r.description,''),
		  COALESCE(r.photo_url,''),
		  COALESCE(r.address,''),
		  COALESCE(r.phone,''),
		  COALESCE(r.opens_at,''),
		  COALESCE(r.closes_at,''),
		  r.lat,
		  r.lng,
		  AVG(rv.rating_restaurant)::float8 AS rating_avg,
		  COUNT(rv.id)::int AS rating_count
		FROM restaurants r
		LEFT JOIN reviews rv ON rv.restaurant_id = r.id
		GROUP BY r.id
		ORDER BY COALESCE(r.name,'')`)
	if err != nil {
		log.Printf("GET /restaurants list primary (будет использован упрощённый fallback — проверьте схему БД и миграции): %v", err)
		rows, err = a.Pool.Query(ctx, `
			SELECT id, COALESCE(name::text,''), COALESCE(address,'')
			FROM restaurants ORDER BY name`)
		if err != nil {
			log.Printf("GET /restaurants list fallback: %v", err)
			a.err(w, http.StatusInternalServerError, "БД")
			return
		}
		defer rows.Close()
		var out []map[string]any
		for rows.Next() {
			var id uuid.UUID
			var name, addr string
			if err := rows.Scan(&id, &name, &addr); err != nil {
				log.Printf("GET /restaurants scan: %v", err)
				continue
			}
			out = append(out, map[string]any{
				"id": id.String(), "name": name, "slug": "", "city": "",
				"description": addr, "photo_url": "",
				"address": "", "phone": "", "opens_at": "", "closes_at": "",
			})
		}
		a.json(w, http.StatusOK, out)
		return
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id uuid.UUID
		var name, slug, city, desc, photo, addr, phone, opensAt, closesAt string
		var lat *float64
		var lng *float64
		var ratingAvg *float64
		var ratingCount int
		if err := rows.Scan(&id, &name, &slug, &city, &desc, &photo, &addr, &phone, &opensAt, &closesAt, &lat, &lng, &ratingAvg, &ratingCount); err != nil {
			log.Printf("GET /restaurants scan: %v", err)
			continue
		}
		out = append(out, map[string]any{
			"id": id.String(), "name": name, "slug": slug, "city": city,
			"description": desc, "photo_url": photo,
			"address": addr, "phone": phone, "opens_at": opensAt, "closes_at": closesAt,
			"lat": lat, "lng": lng,
			"rating_avg": ratingAvg, "rating_count": ratingCount,
		})
	}
	a.json(w, http.StatusOK, out)
}

func (a *Handlers) handleRestaurantGet(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		a.err(w, http.StatusBadRequest, "id")
		return
	}
	ctx := r.Context()
	var name, slug, city, desc, photo, address string
	err = a.Pool.QueryRow(ctx, `
		SELECT COALESCE(name,''), COALESCE(slug,''), COALESCE(city,''), COALESCE(description,''), COALESCE(photo_url,''),
		       COALESCE(address,'')
		FROM restaurants WHERE id=$1`, id).Scan(&name, &slug, &city, &desc, &photo, &address)
	if errors.Is(err, pgx.ErrNoRows) {
		a.err(w, http.StatusNotFound, "не найден")
		return
	}
	if err != nil {
		log.Printf("GET /restaurants/%s primary: %v", id, err)
		var addr string
		err2 := a.Pool.QueryRow(ctx, `
			SELECT COALESCE(name::text,''), COALESCE(address,'')
			FROM restaurants WHERE id=$1`, id).Scan(&name, &addr)
		if errors.Is(err2, pgx.ErrNoRows) {
			a.err(w, http.StatusNotFound, "не найден")
			return
		}
		if err2 != nil {
			log.Printf("GET /restaurants/%s fallback: %v", id, err2)
			a.err(w, http.StatusInternalServerError, "БД")
			return
		}
		slug, city, desc, photo, address = "", "", "", "", addr
	}

	phone, opensAt, closesAt := "", "", ""
	extraObj := map[string]any{}
	var extraJSON []byte
	errExt := a.Pool.QueryRow(ctx, `
		SELECT COALESCE(phone,''), COALESCE(opens_at,''), COALESCE(closes_at,''),
		       COALESCE(extra_json, '{}'::jsonb)
		FROM restaurants WHERE id=$1`, id).Scan(&phone, &opensAt, &closesAt, &extraJSON)
	if errExt != nil {
		log.Printf("GET /restaurants/%s contacts/extra (игнор): %v", id, errExt)
		phone, opensAt, closesAt = "", "", ""
		extraJSON = []byte("{}")
	}
	if len(extraJSON) > 0 {
		_ = json.Unmarshal(extraJSON, &extraObj)
	}
	if extraObj == nil {
		extraObj = map[string]any{}
	}

	a.json(w, http.StatusOK, map[string]any{
		"id": id.String(), "name": name, "slug": slug, "city": city,
		"description": desc, "photo_url": photo, "address": address,
		"phone": phone, "opens_at": opensAt, "closes_at": closesAt,
		"extra_json":         extraObj,
		"photo_gallery_urls": photoGalleryURLsFromExtraMap(extraObj),
	})
}

func (a *Handlers) handleRestaurantMenuPublic(w http.ResponseWriter, r *http.Request) {
	rid, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		a.err(w, http.StatusBadRequest, "id")
		return
	}
	ctx := r.Context()
	cats, items, err := a.queryPublicMenu(ctx, a.Pool, rid)
	if err != nil {
		log.Printf("GET /restaurants/%s/menu: %v", rid, err)
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	a.json(w, http.StatusOK, map[string]any{"categories": cats, "items": items})
}

type menuCatRow struct {
	ID       string  `json:"id"`
	ParentID *string `json:"parent_id"`
	Name     string  `json:"name"`
	Sort     int     `json:"sort_order"`
}

type menuItemRow struct {
	ID           string `json:"id"`
	CategoryID   string `json:"category_id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	PriceKopecks int    `json:"price_kopecks"`
	Sort         int    `json:"sort_order"`
	ImageURL     string `json:"image_url"`
}

func scanMenuCategories(rows pgx.Rows) ([]menuCatRow, error) {
	defer rows.Close()
	var cats []menuCatRow
	for rows.Next() {
		var id uuid.UUID
		var pid uuid.NullUUID
		var name string
		var so int
		if err := rows.Scan(&id, &pid, &name, &so); err != nil {
			return nil, err
		}
		c := menuCatRow{ID: id.String(), Name: name, Sort: so}
		if pid.Valid {
			s := pid.UUID.String()
			c.ParentID = &s
		}
		cats = append(cats, c)
	}
	return cats, rows.Err()
}

func scanMenuItems(rows pgx.Rows, withImage bool) ([]menuItemRow, error) {
	defer rows.Close()
	var items []menuItemRow
	for rows.Next() {
		var id, cid uuid.UUID
		var name, desc string
		var price, so int
		var img string
		var err error
		if withImage {
			err = rows.Scan(&id, &cid, &name, &desc, &price, &so, &img)
		} else {
			err = rows.Scan(&id, &cid, &name, &desc, &price, &so)
			img = ""
		}
		if err != nil {
			return nil, err
		}
		items = append(items, menuItemRow{
			ID: id.String(), CategoryID: cid.String(), Name: name, Description: desc,
			PriceKopecks: price, Sort: so, ImageURL: img,
		})
	}
	return items, rows.Err()
}

// queryPublicMenu — полный SELECT; при отсутствии колонок (старая схема) — упрощённые запросы.
func (a *Handlers) queryPublicMenu(ctx context.Context, pool *pgxpool.Pool, rid uuid.UUID) ([]menuCatRow, []menuItemRow, error) {
	catRows, err := pool.Query(ctx, `
		SELECT id, parent_id, name, sort_order
		FROM menu_categories
		WHERE restaurant_id=$1 AND is_active=TRUE
		ORDER BY sort_order, name`, rid)
	if err != nil {
		catRows, err = pool.Query(ctx, `
			SELECT id, parent_id, name, sort_order
			FROM menu_categories
			WHERE restaurant_id=$1
			ORDER BY sort_order, name`, rid)
		if err != nil {
			return nil, nil, err
		}
	}
	cats, err := scanMenuCategories(catRows)
	if err != nil {
		return nil, nil, err
	}

	itemRows, err := pool.Query(ctx, `
		SELECT id, category_id, name, COALESCE(description,''), price_kopecks, sort_order, COALESCE(image_url,'')
		FROM menu_items
		WHERE restaurant_id=$1 AND is_available=TRUE
		ORDER BY sort_order, name`, rid)
	if err != nil {
		itemRows, err = pool.Query(ctx, `
			SELECT id, category_id, name, COALESCE(description,''), price_kopecks, sort_order
			FROM menu_items
			WHERE restaurant_id=$1 AND is_available=TRUE
			ORDER BY sort_order, name`, rid)
		if err != nil {
			return nil, nil, err
		}
		items, err2 := scanMenuItems(itemRows, false)
		return cats, items, err2
	}
	items, err := scanMenuItems(itemRows, true)
	return cats, items, err
}

// MountRestaurantsPublic — каталог и меню (без секретов).
func (a *Handlers) MountRestaurantsPublic(r chi.Router) {
	r.Get("/restaurants", a.handleRestaurantsList)
	r.Get("/restaurants/{id}", a.handleRestaurantGet)
	r.Get("/restaurants/{id}/menu", a.handleRestaurantMenuPublic)
}
