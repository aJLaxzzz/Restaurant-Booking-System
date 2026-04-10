package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// handleOwnerStaffAssign — владелец назначает существующие аккаунты админами или официантами (или снимает: role=client).
func (a *Handlers) handleOwnerStaffAssign(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	rid, ok := a.mustRestaurant(w, r, u)
	if !ok || u.Role != "owner" {
		return
	}
	a.staffAssign(w, r, rid, u, true)
}

// handleAdminStaffAssign — админ заведения назначает только официантов (или снимает).
func (a *Handlers) handleAdminStaffAssign(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	rid, ok := a.mustRestaurant(w, r, u)
	if !ok || u.Role != "admin" {
		return
	}
	a.staffAssign(w, r, rid, u, false)
}

func (a *Handlers) staffAssign(w http.ResponseWriter, r *http.Request, restaurantID uuid.UUID, actor userCtx, ownerCanAdmin bool) {
	var body struct {
		Email string `json:"email"`
		Role  string `json:"role"` // waiter, admin, client
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		a.err(w, http.StatusBadRequest, "json")
		return
	}
	body.Email = strings.ToLower(strings.TrimSpace(body.Email))
	if body.Email == "" {
		a.err(w, http.StatusBadRequest, "email")
		return
	}
	body.Role = strings.TrimSpace(body.Role)
	if body.Role != "waiter" && body.Role != "admin" && body.Role != "client" {
		a.err(w, http.StatusBadRequest, "роль: waiter, admin или client")
		return
	}
	if !ownerCanAdmin && body.Role == "admin" {
		a.err(w, http.StatusForbidden, "только владелец может назначать администраторов")
		return
	}

	var targetID uuid.UUID
	var targetRole string
	var targetRest uuid.NullUUID
	err := a.Pool.QueryRow(r.Context(), `
		SELECT id, role, restaurant_id FROM users WHERE lower(email)=$1`, body.Email).
		Scan(&targetID, &targetRole, &targetRest)
	if err == pgx.ErrNoRows {
		a.err(w, http.StatusNotFound, "пользователь не найден — сначала регистрация")
		return
	}
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	if targetID == actor.ID {
		a.err(w, http.StatusBadRequest, "нельзя изменить самого себя")
		return
	}
	if targetRole == "owner" {
		a.err(w, http.StatusBadRequest, "нельзя переназначить владельца")
		return
	}

	// Уже владелец другого ресторана?
	var ownCount int
	_ = a.Pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM restaurants WHERE owner_user_id=$1`, targetID).Scan(&ownCount)
	if ownCount > 0 {
		a.err(w, http.StatusConflict, "пользователь уже владелец другого ресторана")
		return
	}

	if targetRest.Valid && targetRest.UUID != restaurantID && (targetRole == "admin" || targetRole == "waiter") {
		a.err(w, http.StatusConflict, "пользователь уже в другом заведении")
		return
	}

	if body.Role == "client" {
		ct, err := a.Pool.Exec(r.Context(), `
			UPDATE users SET role='client', restaurant_id=NULL, updated_at=NOW()
			WHERE id=$1 AND restaurant_id=$2 AND role IN ('admin','waiter')`,
			targetID, restaurantID)
		if err != nil {
			a.err(w, http.StatusInternalServerError, "БД")
			return
		}
		if ct.RowsAffected() == 0 {
			a.err(w, http.StatusNotFound, "не найден сотрудник этого заведения")
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Назначение admin / waiter
	ct, err := a.Pool.Exec(r.Context(), `
		UPDATE users SET role=$2, restaurant_id=$3, updated_at=NOW()
		WHERE id=$1
		AND (
			(role = 'client' AND restaurant_id IS NULL)
			OR (role IN ('waiter','admin') AND restaurant_id = $3)
		)`,
		targetID, body.Role, restaurantID)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "БД")
		return
	}
	if ct.RowsAffected() == 0 {
		a.err(w, http.StatusConflict, "не удалось назначить (проверьте роль пользователя)")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
