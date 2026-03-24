package handlers

import (
	"context"
	"net/http"
	"time"
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
