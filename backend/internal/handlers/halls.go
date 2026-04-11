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
	ridStr := r.URL.Query().Get("restaurant_id")
	var filter *uuid.UUID
	if ridStr != "" {
		rid, err := uuid.Parse(ridStr)
		if err != nil {
			a.err(w, http.StatusBadRequest, "restaurant_id")
			return
		}
		filter = &rid
	}
	if u, ok := userFromOptional(r); ok && (u.Role == "admin" || u.Role == "waiter" || u.Role == "owner") {
		sr, err := a.restaurantUUIDForUser(r.Context(), u.ID, u.Role)
		if err == nil && sr != uuid.Nil {
			if filter != nil && *filter != sr {
				a.err(w, http.StatusForbidden, "чужое заведение")
				return
			}
			filter = &sr
		}
	}
	q := `SELECT h.id, h.name, r.name, r.id FROM halls h JOIN restaurants r ON r.id = h.restaurant_id`
	args := []any{}
	if filter != nil {
		q += ` WHERE h.restaurant_id=$1`
		args = append(args, *filter)
	}
	q += ` ORDER BY r.name, h.name`
	rows, err := a.Pool.Query(r.Context(), q, args...)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, restID uuid.UUID
		var hn, rn string
		_ = rows.Scan(&id, &hn, &rn, &restID)
		out = append(out, map[string]any{
			"id": id.String(), "name": hn, "restaurant": rn,
			"restaurant_id": restID.String(),
		})
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
	ID          string  `json:"id"`
	Number      int     `json:"number"`
	Capacity    int     `json:"capacity"`
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	Shape       string  `json:"shape"`
	Radius      float64 `json:"radius,omitempty"`
	Width       float64 `json:"width,omitempty"`
	Height      float64 `json:"height,omitempty"`
	RotationDeg float64 `json:"rotation_deg,omitempty"`
	Status      string  `json:"status"`
}

// layoutJSON: walls — сегменты {x1,y1,x2,y2}; decorations — zone_label, window_band, door/window (сегмент), zone, zone_polygon, fixture.
type roomInsetJSON struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

type layoutJSON struct {
	Tables        []layoutTable          `json:"tables"`
	Walls         []map[string]float64   `json:"walls"`
	WallSegments  []map[string]any       `json:"wall_segments,omitempty"`
	Rooms         []map[string]any       `json:"rooms,omitempty"`
	ChairLayout   map[string]any         `json:"chair_layout,omitempty"`
	Decorations   []map[string]any       `json:"decorations"`
	CanvasWidth   float64                `json:"canvas_width"`
	CanvasHeight  float64                `json:"canvas_height"`
	RoomInset     *roomInsetJSON         `json:"room_inset,omitempty"`
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
	if len(layoutRaw) > 0 && layoutRaw[0] == '[' {
		_ = json.Unmarshal(layoutRaw, &lj.Walls)
	} else {
		_ = json.Unmarshal(layoutRaw, &lj)
	}
	if lj.Decorations == nil {
		lj.Decorations = []map[string]any{}
	}

	rows, err := a.Pool.Query(r.Context(), `
		SELECT id, table_number, capacity, x_coordinate, y_coordinate, shape, status, block_reason,
		       width, height, rotation_deg
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

	tables := make([]layoutTable, 0)
	for rows.Next() {
		var id uuid.UUID
		var num, cap int
		var x, y float64
		var shape, status, blockReason *string
		var width, height, rot float64
		_ = rows.Scan(&id, &num, &cap, &x, &y, &shape, &status, &blockReason, &width, &height, &rot)
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
		rad := width / 2
		if rad <= 0 {
			rad = 28
		}
		lt := layoutTable{
			ID: id.String(), Number: num, Capacity: cap, X: x, Y: y, Shape: sh, Status: st,
			Radius: rad, Width: width, Height: height, RotationDeg: rot,
		}
		tables = append(tables, lt)
	}

	walls := lj.Walls
	if walls == nil {
		walls = []map[string]float64{}
	}
	cw, ch := 920.0, 640.0
	if lj.CanvasWidth > 0 {
		cw = lj.CanvasWidth
	}
	if lj.CanvasHeight > 0 {
		ch = lj.CanvasHeight
	}
	out := map[string]any{
		"tables": tables, "walls": walls, "decorations": lj.Decorations,
		"canvas_width": cw, "canvas_height": ch,
	}
	if lj.RoomInset != nil {
		out["room_inset"] = lj.RoomInset
	}
	if len(lj.WallSegments) > 0 {
		out["wall_segments"] = lj.WallSegments
	}
	if len(lj.Rooms) > 0 {
		out["rooms"] = lj.Rooms
	}
	if len(lj.ChairLayout) > 0 {
		out["chair_layout"] = lj.ChairLayout
	}
	a.json(w, http.StatusOK, out)
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
	if msg := bookingStartNotAllowed(start); msg != "" {
		a.err(w, http.StatusBadRequest, msg)
		return
	}
	rows, err := a.Pool.Query(r.Context(), `
		SELECT id, table_number, capacity, x_coordinate, y_coordinate,
		       COALESCE(shape, 'circle'), COALESCE(status, 'available')
		FROM tables WHERE hall_id=$1 AND capacity >= $2 AND COALESCE(status, 'available') != 'blocked'`, hallID, guests)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
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
	if body.Decorations == nil {
		body.Decorations = []map[string]any{}
	}
	var exLayout layoutJSON
	var exRaw []byte
	_ = a.Pool.QueryRow(r.Context(), `SELECT layout_json FROM halls WHERE id=$1`, hallID).Scan(&exRaw)
	if len(exRaw) > 0 && exRaw[0] != '[' {
		_ = json.Unmarshal(exRaw, &exLayout)
	}
	if body.RoomInset == nil && exLayout.RoomInset != nil {
		body.RoomInset = exLayout.RoomInset
	}
	if body.WallSegments == nil && len(exLayout.WallSegments) > 0 {
		body.WallSegments = exLayout.WallSegments
	}
	if body.Rooms == nil && len(exLayout.Rooms) > 0 {
		body.Rooms = exLayout.Rooms
	}
	if body.ChairLayout == nil && len(exLayout.ChairLayout) > 0 {
		body.ChairLayout = exLayout.ChairLayout
	}
	layoutObj := map[string]any{"tables": body.Tables, "walls": body.Walls, "decorations": body.Decorations}
	if body.CanvasWidth > 0 {
		layoutObj["canvas_width"] = body.CanvasWidth
	}
	if body.CanvasHeight > 0 {
		layoutObj["canvas_height"] = body.CanvasHeight
	}
	if body.RoomInset != nil {
		layoutObj["room_inset"] = body.RoomInset
	}
	if body.WallSegments != nil {
		layoutObj["wall_segments"] = body.WallSegments
	}
	if body.Rooms != nil {
		layoutObj["rooms"] = body.Rooms
	}
	if body.ChairLayout != nil {
		layoutObj["chair_layout"] = body.ChairLayout
	}
	b, _ := json.Marshal(layoutObj)
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
		w := t.Width
		h := t.Height
		if w <= 0 {
			if t.Radius > 0 {
				w = t.Radius * 2
			} else {
				w = 56
			}
		}
		if h <= 0 {
			if t.Radius > 0 {
				h = t.Radius * 2
			} else {
				h = 56
			}
		}
		_, _ = tx.Exec(r.Context(), `
			UPDATE tables SET x_coordinate=$3, y_coordinate=$4, capacity=$5, shape=$6, table_number=$7,
				width=$8, height=$9, rotation_deg=$10, updated_at=NOW()
			WHERE id=$1 AND hall_id=$2`,
			tid, hallID, t.X, t.Y, t.Capacity, t.Shape, t.Number, w, h, t.RotationDeg)
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
