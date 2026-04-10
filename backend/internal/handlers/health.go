package handlers

import (
	"context"
	"net/http"
	"time"

	"restaurant-booking/internal/config"
)

func (a *Handlers) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := a.Pool.Ping(ctx); err != nil {
		a.json(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "db": err.Error()})
		return
	}
	if err := a.RDB.Ping(ctx).Err(); err != nil {
		a.json(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "redis": err.Error()})
		return
	}
	a.json(w, http.StatusOK, map[string]any{"ok": true})
}

// handleHealthDiag — диагностика: какая БД подключена, миграции, счётчики (без секретов).
func (a *Handlers) handleHealthDiag(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	out := map[string]any{
		"database_url": config.RedactDatabaseURL(a.Cfg.DatabaseURL),
	}
	var dbName, dbUser string
	if err := a.Pool.QueryRow(ctx, `SELECT current_database()`).Scan(&dbName); err != nil {
		out["error"] = err.Error()
		a.json(w, http.StatusInternalServerError, out)
		return
	}
	_ = a.Pool.QueryRow(ctx, `SELECT current_user`).Scan(&dbUser)
	out["current_database"] = dbName
	out["current_user"] = dbUser

	var nRest, nMenu, nCat, nHall, nMigr int
	_ = a.Pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM restaurants`).Scan(&nRest)
	_ = a.Pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM menu_items`).Scan(&nMenu)
	_ = a.Pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM menu_categories`).Scan(&nCat)
	_ = a.Pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM halls`).Scan(&nHall)
	_ = a.Pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM schema_migrations`).Scan(&nMigr)
	out["counts"] = map[string]int{
		"restaurants": nRest, "menu_items": nMenu, "menu_categories": nCat,
		"halls": nHall, "schema_migrations_rows": nMigr,
	}

	rows, err := a.Pool.Query(ctx, `SELECT filename FROM schema_migrations ORDER BY filename`)
	if err != nil {
		out["migrations_error"] = err.Error()
	} else {
		defer rows.Close()
		var files []string
		for rows.Next() {
			var f string
			if rows.Scan(&f) == nil {
				files = append(files, f)
			}
		}
		out["schema_migrations"] = files
	}

	rows2, err := a.Pool.Query(ctx, `SELECT id::text, name, COALESCE(slug,'') FROM restaurants ORDER BY name`)
	if err != nil {
		out["restaurants_preview_error"] = err.Error()
	} else {
		defer rows2.Close()
		var list []map[string]string
		for rows2.Next() {
			var id, name, slug string
			if rows2.Scan(&id, &name, &slug) != nil {
				continue
			}
			list = append(list, map[string]string{"id": id, "name": name, "slug": slug})
		}
		out["restaurants"] = list
	}

	a.json(w, http.StatusOK, out)
}
