package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (a *Handlers) emitHallEvent(hallID uuid.UUID, msg any) {
	if a.Hub != nil {
		a.Hub.Broadcast(hallID, msg)
		return
	}
	if a.Cfg.HallServiceURL == "" {
		return
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	u := strings.TrimSuffix(a.Cfg.HallServiceURL, "/") + "/internal/emit"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(b))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Secret", a.Cfg.InternalSecret)
	req.Header.Set("X-Hall-ID", hallID.String())
	_, _ = http.DefaultClient.Do(req)
}

func (a *Handlers) handleInternalEmit(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Internal-Secret") != a.Cfg.InternalSecret {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	hid, err := uuid.Parse(r.Header.Get("X-Hall-ID"))
	if err != nil {
		http.Error(w, "hall id", http.StatusBadRequest)
		return
	}
	var msg map[string]any
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, "json", http.StatusBadRequest)
		return
	}
	if a.Hub != nil {
		a.Hub.Broadcast(hid, msg)
	}
	w.WriteHeader(http.StatusNoContent)
}
