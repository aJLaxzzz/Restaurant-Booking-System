package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (a *Handlers) handleHallsList(w http.ResponseWriter, r *http.Request) {
	rows, err := a.Pool.Query(r.Context(), `SELECT h.id, h.name, r.name FROM halls h JOIN restaurants r ON r.id = h.restaurant_id ORDER BY h.name`)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id uuid.UUID
		var hn, rn string
		_ = rows.Scan(&id, &hn, &rn)
		out = append(out, map[string]any{"id": id.String(), "name": hn, "restaurant": rn})
	}
	a.json(w, http.StatusOK, out)
}

func (a *Handlers) handleHallGet(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		a.err(w, http.StatusBadRequest, "id")
		return
	}
	var name string
	var rid uuid.UUID
	err = a.Pool.QueryRow(r.Context(), `SELECT name, restaurant_id FROM halls WHERE id=$1`, id).Scan(&name, &rid)
	if err == pgx.ErrNoRows {
		a.err(w, http.StatusNotFound, "не найден")
		return
	}
	a.json(w, http.StatusOK, map[string]any{"id": id.String(), "name": name, "restaurant_id": rid.String()})
}

type layoutTable struct {
	ID       string  `json:"id"`
	Number   int     `json:"number"`
	Capacity int     `json:"capacity"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Shape    string  `json:"shape"`
	Radius   float64 `json:"radius,omitempty"`
	Width    float64 `json:"width,omitempty"`
	Height   float64 `json:"height,omitempty"`
	Status   string  `json:"status"`
}

type layoutJSON struct {
	Tables []layoutTable  `json:"tables"`
	Walls  []map[string]float64 `json:"walls"`
}

func (a *Handlers) handleLayoutGet(w http.ResponseWriter, r *http.Request) {
	hallID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		a.err(w, http.StatusBadRequest, "id")
		return
	}
	var layoutRaw []byte
	err = a.Pool.QueryRow(r.Context(), `SELECT layout_json FROM halls WHERE id=$1`, hallID).Scan(&layoutRaw)
	if err == pgx.ErrNoRows {
		a.err(w, http.StatusNotFound, "зал не найден")
		return
	}
	var lj layoutJSON
	_ = json.Unmarshal(layoutRaw, &lj)

	rows, err := a.Pool.Query(r.Context(), `
		SELECT id, table_number, capacity, x_coordinate, y_coordinate, shape, status, block_reason
		FROM tables WHERE hall_id=$1 ORDER BY table_number`, hallID)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	defer rows.Close()

	u, hasUser := userFromOptional(r)
	var uidStr string
	if hasUser {
		uidStr = u.ID.String()
	}

	var tables []layoutTable
	for rows.Next() {
		var id uuid.UUID
		var num, cap int
		var x, y float64
		var shape, status, blockReason *string
		_ = rows.Scan(&id, &num, &cap, &x, &y, &shape, &status, &blockReason)
		sh := "circle"
		if shape != nil {
			sh = *shape
		}
		st := "available"
		if status != nil {
			st = *status
		}
		if st == "occupied" {
			// keep red
		} else if st == "blocked" {
			// gray
		} else {
			lockUID, _ := a.RDB.Get(r.Context(), "table:"+id.String()+":lock").Result()
			if lockUID != "" {
				if lockUID != uidStr {
					st = "locked_by_other"
				} else {
					st = "selected"
				}
			}
		}
		lt := layoutTable{
			ID: id.String(), Number: num, Capacity: cap, X: x, Y: y, Shape: sh, Status: st, Radius: 28,
		}
		tables = append(tables, lt)
	}

	// walls from layout_json if present
	walls := lj.Walls
	if walls == nil {
		walls = []map[string]float64{}
	}
	a.json(w, http.StatusOK, map[string]any{"tables": tables, "walls": walls})
}

// handleHallAvailability — столы, подходящие по вместимости и без пересечения по времени.
func (a *Handlers) handleHallAvailability(w http.ResponseWriter, r *http.Request) {
	hallID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		a.err(w, http.StatusBadRequest, "id")
		return
	}
	q := r.URL.Query()
	start, err := time.Parse(time.RFC3339, q.Get("start"))
	if err != nil {
		a.err(w, http.StatusBadRequest, "start (RFC3339)")
		return
	}
	end, err := time.Parse(time.RFC3339, q.Get("end"))
	if err != nil {
		a.err(w, http.StatusBadRequest, "end (RFC3339)")
		return
	}
	guests, _ := strconv.Atoi(q.Get("guests"))
	if guests < 1 {
		guests = 1
	}
	rows, err := a.Pool.Query(r.Context(), `
		SELECT id, table_number, capacity, x_coordinate, y_coordinate, shape, status
		FROM tables WHERE hall_id=$1 AND capacity >= $2 AND status != 'blocked'`, hallID, guests)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id uuid.UUID
		var num, cap int
		var x, y float64
		var shape, status string
		_ = rows.Scan(&id, &num, &cap, &x, &y, &shape, &status)
		var busy uuid.UUID
		_ = a.Pool.QueryRow(r.Context(), `
			SELECT id FROM reservations
			WHERE table_id=$1
			AND status NOT IN ('cancelled_by_client','cancelled_by_admin','no_show')
			AND (
				(start_time <= $2 AND end_time > $2) OR
				(start_time < $3 AND end_time >= $3) OR
				(start_time >= $2 AND end_time <= $3)
			) LIMIT 1`, id, start, end).Scan(&busy)
		ok := busy == uuid.Nil
		out = append(out, map[string]any{
			"id": id.String(), "number": num, "capacity": cap, "x": x, "y": y,
			"shape": shape, "status": status, "available_for_slot": ok,
		})
	}
	a.json(w, http.StatusOK, map[string]any{"tables": out})
}

func (a *Handlers) handleLayoutPut(w http.ResponseWriter, r *http.Request) {
	hallID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		a.err(w, http.StatusBadRequest, "id")
		return
	}
	var body layoutJSON
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		a.err(w, http.StatusBadRequest, "JSON")
		return
	}
	b, _ := json.Marshal(map[string]any{"tables": body.Tables, "walls": body.Walls})
	tx, err := a.Pool.Begin(r.Context())
	if err != nil {
		a.err(w, http.StatusInternalServerError, "tx")
		return
	}
	defer tx.Rollback(r.Context())
	_, err = tx.Exec(r.Context(), `UPDATE halls SET layout_json=$2::jsonb, updated_at=NOW() WHERE id=$1`, hallID, b)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "save layout")
		return
	}
	for _, t := range body.Tables {
		tid, err := uuid.Parse(t.ID)
		if err != nil {
			continue
		}
		radius := t.Radius
		if radius <= 0 {
			radius = 28
		}
		_, _ = tx.Exec(r.Context(), `
			UPDATE tables SET x_coordinate=$3, y_coordinate=$4, capacity=$5, shape=$6, table_number=$7, updated_at=NOW()
			WHERE id=$1 AND hall_id=$2`,
			tid, hallID, t.X, t.Y, t.Capacity, t.Shape, t.Number)
	}
	if err := tx.Commit(r.Context()); err != nil {
		a.err(w, http.StatusInternalServerError, "commit")
		return
	}
	a.emitHallEvent(hallID, map[string]any{
		"event": "hall.layout_updated", "hall_id": hallID.String(),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
	a.json(w, http.StatusOK, map[string]string{"status": "ok"})
}
