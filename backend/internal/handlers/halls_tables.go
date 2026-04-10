package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (a *Handlers) handleTableCreate(w http.ResponseWriter, r *http.Request) {
	hallID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		a.err(w, http.StatusBadRequest, "id зала")
		return
	}
	var body struct {
		Number      int     `json:"table_number"`
		Capacity    int     `json:"capacity"`
		X           float64 `json:"x"`
		Y           float64 `json:"y"`
		Shape       string  `json:"shape"`
		Width       float64 `json:"width"`
		Height      float64 `json:"height"`
		RotationDeg float64 `json:"rotation_deg"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Number < 1 || body.Capacity < 1 {
		a.err(w, http.StatusBadRequest, "данные стола")
		return
	}
	if body.Shape == "" {
		body.Shape = "circle"
	}
	tw, th := body.Width, body.Height
	if tw <= 0 {
		tw = 56
	}
	if th <= 0 {
		th = 56
	}
	var id uuid.UUID
	err = a.Pool.QueryRow(r.Context(), `
		INSERT INTO tables (hall_id, table_number, capacity, x_coordinate, y_coordinate, shape, width, height, rotation_deg)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`,
		hallID, body.Number, body.Capacity, body.X, body.Y, body.Shape, tw, th, body.RotationDeg).Scan(&id)
	if err != nil {
		a.err(w, http.StatusConflict, "стол с таким номером уже есть")
		return
	}
	a.emitHallEvent(hallID, map[string]any{"event": "hall.layout_updated", "timestamp": time.Now().UTC().Format(time.RFC3339)})
	a.json(w, http.StatusCreated, map[string]string{"id": id.String()})
}

func (a *Handlers) handleTableUpdate(w http.ResponseWriter, r *http.Request) {
	hallID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		a.err(w, http.StatusBadRequest, "зал")
		return
	}
	tid, err := uuid.Parse(chi.URLParam(r, "tid"))
	if err != nil {
		a.err(w, http.StatusBadRequest, "стол")
		return
	}
	var body struct {
		Capacity    *int     `json:"capacity"`
		X           *float64 `json:"x"`
		Y           *float64 `json:"y"`
		Shape       *string  `json:"shape"`
		Status      *string  `json:"status"`
		Number      *int     `json:"table_number"`
		Width       *float64 `json:"width"`
		Height      *float64 `json:"height"`
		RotationDeg *float64 `json:"rotation_deg"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	ct, err := a.Pool.Exec(r.Context(), `
		UPDATE tables SET
			capacity = COALESCE($3, capacity),
			x_coordinate = COALESCE($4, x_coordinate),
			y_coordinate = COALESCE($5, y_coordinate),
			shape = COALESCE($6, shape),
			status = COALESCE($7, status),
			table_number = COALESCE($8, table_number),
			width = COALESCE($9, width),
			height = COALESCE($10, height),
			rotation_deg = COALESCE($11, rotation_deg),
			updated_at = NOW()
		WHERE id=$1 AND hall_id=$2`,
		tid, hallID, body.Capacity, body.X, body.Y, body.Shape, body.Status, body.Number,
		body.Width, body.Height, body.RotationDeg)
	if err != nil || ct.RowsAffected() == 0 {
		a.err(w, http.StatusNotFound, "стол не найден")
		return
	}
	a.emitHallEvent(hallID, map[string]any{"event": "hall.layout_updated", "table_id": tid.String(), "timestamp": time.Now().UTC().Format(time.RFC3339)})
	w.WriteHeader(http.StatusNoContent)
}

func (a *Handlers) handleTableDelete(w http.ResponseWriter, r *http.Request) {
	hallID, _ := uuid.Parse(chi.URLParam(r, "id"))
	tid, err := uuid.Parse(chi.URLParam(r, "tid"))
	if err != nil {
		a.err(w, http.StatusBadRequest, "стол")
		return
	}
	ct, err := a.Pool.Exec(r.Context(), `DELETE FROM tables WHERE id=$1 AND hall_id=$2`, tid, hallID)
	if err != nil || ct.RowsAffected() == 0 {
		a.err(w, http.StatusNotFound, "не найден")
		return
	}
	a.emitHallEvent(hallID, map[string]any{"event": "hall.layout_updated", "timestamp": time.Now().UTC().Format(time.RFC3339)})
	w.WriteHeader(http.StatusNoContent)
}

func (a *Handlers) handleTableLock(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	tid, err := uuid.Parse(chi.URLParam(r, "tid"))
	if err != nil {
		a.err(w, http.StatusBadRequest, "стол")
		return
	}
	var status string
	err = a.Pool.QueryRow(r.Context(), `SELECT status FROM tables WHERE id=$1`, tid).Scan(&status)
	if err == pgx.ErrNoRows {
		a.err(w, http.StatusNotFound, "стол")
		return
	}
	if status != "available" {
		a.err(w, http.StatusConflict, "стол недоступен")
		return
	}
	key := "table:" + tid.String() + ":lock"
	ok, err := a.RDB.SetNX(r.Context(), key, u.ID.String(), 5*time.Minute).Result()
	if err != nil || !ok {
		a.err(w, http.StatusConflict, "стол занят другим пользователем")
		return
	}
	hallID, _ := uuid.Parse(chi.URLParam(r, "id"))
	a.emitHallEvent(hallID, map[string]any{
		"event": "table.locked", "table_id": tid.String(), "status": "locked",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
	a.json(w, http.StatusOK, map[string]string{"status": "locked"})
}

func (a *Handlers) handleTableUnlock(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	tid, err := uuid.Parse(chi.URLParam(r, "tid"))
	if err != nil {
		a.err(w, http.StatusBadRequest, "стол")
		return
	}
	key := "table:" + tid.String() + ":lock"
	val, _ := a.RDB.Get(r.Context(), key).Result()
	if val == u.ID.String() {
		_ = a.RDB.Del(r.Context(), key).Err()
	}
	hallID, _ := uuid.Parse(chi.URLParam(r, "id"))
	a.emitHallEvent(hallID, map[string]any{
		"event": "table.freed", "table_id": tid.String(),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
	w.WriteHeader(http.StatusNoContent)
}
