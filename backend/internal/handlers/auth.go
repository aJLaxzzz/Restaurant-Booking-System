package handlers

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	appauth "restaurant-booking/internal/auth"
)

var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
var phoneRe = regexp.MustCompile(`^\+7\d{10}$`)

func (a *Handlers) handleRegister(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email            string `json:"email"`
		Password         string `json:"password"`
		FullName         string `json:"full_name"`
		Phone            string `json:"phone"`
		RegisterAsOwner  bool   `json:"register_as_owner"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		a.err(w, http.StatusBadRequest, "неверный JSON")
		return
	}
	body.Email = strings.TrimSpace(strings.ToLower(body.Email))
	if !emailRe.MatchString(body.Email) {
		a.err(w, http.StatusBadRequest, "некорректный email")
		return
	}
	if !phoneRe.MatchString(strings.ReplaceAll(body.Phone, " ", "")) {
		a.err(w, http.StatusBadRequest, "телефон в формате +7XXXXXXXXXX")
		return
	}
	if len(body.FullName) < 2 {
		a.err(w, http.StatusBadRequest, "укажите имя")
		return
	}
	if err := validatePassword(body.Password); err != nil {
		a.err(w, http.StatusBadRequest, err.Error())
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), a.Cfg.BcryptCost)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "ошибка хеширования")
		return
	}
	phone := strings.ReplaceAll(body.Phone, " ", "")
	role := "client"
	if body.RegisterAsOwner {
		role = "owner"
	}
	var id uuid.UUID
	err = a.Pool.QueryRow(r.Context(), `
		INSERT INTO users (email, password_hash, full_name, phone, role, email_verified)
		VALUES ($1,$2,$3,$4,$5, true)
		RETURNING id`, body.Email, string(hash), body.FullName, phone, role).Scan(&id)
	if err != nil {
		if strings.Contains(err.Error(), "unique") {
			a.err(w, http.StatusConflict, "email уже зарегистрирован")
			return
		}
		a.err(w, http.StatusInternalServerError, "не удалось создать пользователя")
		return
	}
	a.json(w, http.StatusCreated, map[string]any{"id": id.String(), "message": "регистрация успешна (демо: письмо подтверждения не отправляется)"})
}

func validatePassword(p string) error {
	if len(p) < 8 {
		return errStr("пароль минимум 8 символов")
	}
	hasDigit := false
	hasLetter := false
	for _, c := range p {
		if c >= '0' && c <= '9' {
			hasDigit = true
		}
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= 'а' && c <= 'я') || (c >= 'А' && c <= 'Я') {
			hasLetter = true
		}
	}
	if !hasDigit || !hasLetter {
		return errStr("пароль должен содержать буквы и цифры")
	}
	return nil
}

type errStr string

func (e errStr) Error() string { return string(e) }

func (a *Handlers) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		a.err(w, http.StatusBadRequest, "неверный JSON")
		return
	}
	var id uuid.UUID
	var hash, role, status string
	err := a.Pool.QueryRow(r.Context(), `SELECT id, password_hash, role, status FROM users WHERE email=$1`,
		strings.ToLower(strings.TrimSpace(body.Email))).Scan(&id, &hash, &role, &status)
	if err == pgx.ErrNoRows {
		a.err(w, http.StatusUnauthorized, "неверный email или пароль")
		return
	}
	if err != nil {
		a.err(w, http.StatusInternalServerError, "ошибка БД")
		return
	}
	if status == "blocked" {
		a.err(w, http.StatusForbidden, "аккаунт заблокирован")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(body.Password)) != nil {
		a.err(w, http.StatusUnauthorized, "неверный email или пароль")
		return
	}
	access, err := appauth.SignAccess([]byte(a.Cfg.JWTSecret), id, role, a.Cfg.AccessTTL)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "токен")
		return
	}
	refresh := uuid.NewString()
	a.RDB.Set(r.Context(), "refresh:"+refresh, id.String(), a.Cfg.RefreshTTL)
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refresh,
		Path:     "/api/auth",
		MaxAge:   int(a.Cfg.RefreshTTL.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   false,
	})
	a.json(w, http.StatusOK, map[string]any{"access_token": access, "expires_in": int(a.Cfg.AccessTTL.Seconds())})
}

func (a *Handlers) handleRefresh(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("refresh_token")
	if err != nil || c.Value == "" {
		a.err(w, http.StatusUnauthorized, "нет refresh")
		return
	}
	uidStr, err := a.RDB.Get(r.Context(), "refresh:"+c.Value).Result()
	if err != nil || uidStr == "" {
		a.err(w, http.StatusUnauthorized, "сессия недействительна")
		return
	}
	uid, err := uuid.Parse(uidStr)
	if err != nil {
		a.err(w, http.StatusUnauthorized, "сессия недействительна")
		return
	}
	var role string
	if err := a.Pool.QueryRow(r.Context(), `SELECT role FROM users WHERE id=$1 AND status='active'`, uid).Scan(&role); err != nil {
		a.err(w, http.StatusUnauthorized, "пользователь не найден")
		return
	}
	access, err := appauth.SignAccess([]byte(a.Cfg.JWTSecret), uid, role, a.Cfg.AccessTTL)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "токен")
		return
	}
	a.json(w, http.StatusOK, map[string]any{"access_token": access, "expires_in": int(a.Cfg.AccessTTL.Seconds())})
}

func (a *Handlers) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("refresh_token"); err == nil && c.Value != "" {
		_ = a.RDB.Del(r.Context(), "refresh:"+c.Value).Err()
	}
	http.SetCookie(w, &http.Cookie{Name: "refresh_token", Value: "", Path: "/api/auth", MaxAge: -1})
	w.WriteHeader(http.StatusNoContent)
}

func (a *Handlers) handleMe(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	var email, fullName, phone, role, status string
	var createdAt time.Time
	err := a.Pool.QueryRow(r.Context(), `
		SELECT email, full_name, phone, role, status, created_at FROM users WHERE id=$1`, u.ID).
		Scan(&email, &fullName, &phone, &role, &status, &createdAt)
	if err != nil {
		a.err(w, http.StatusNotFound, "не найден")
		return
	}
	out := map[string]any{
		"id": u.ID.String(), "email": email, "full_name": fullName, "phone": phone,
		"role": role, "status": status, "created_at": createdAt,
	}
	if rid, err := a.restaurantUUIDForUser(r.Context(), u.ID, role); err == nil && rid != uuid.Nil {
		out["restaurant_id"] = rid.String()
	}
	a.json(w, http.StatusOK, out)
}

func (a *Handlers) handleMeUpdate(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	var body struct {
		FullName *string `json:"full_name"`
		Phone    *string `json:"phone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		a.err(w, http.StatusBadRequest, "неверный JSON")
		return
	}
	if body.Phone != nil && !phoneRe.MatchString(strings.ReplaceAll(*body.Phone, " ", "")) {
		a.err(w, http.StatusBadRequest, "телефон +7XXXXXXXXXX")
		return
	}
	_, err := a.Pool.Exec(r.Context(), `
		UPDATE users SET
			full_name = COALESCE($2, full_name),
			phone = COALESCE($3, phone),
			updated_at = NOW()
		WHERE id=$1`, u.ID, body.FullName, body.Phone)
	if err != nil {
		a.err(w, http.StatusInternalServerError, "не обновлено")
		return
	}
	a.handleMe(w, r)
}

func (a *Handlers) handlePassword(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	var body struct {
		Old string `json:"old_password"`
		New string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		a.err(w, http.StatusBadRequest, "неверный JSON")
		return
	}
	if err := validatePassword(body.New); err != nil {
		a.err(w, http.StatusBadRequest, err.Error())
		return
	}
	var hash string
	if err := a.Pool.QueryRow(r.Context(), `SELECT password_hash FROM users WHERE id=$1`, u.ID).Scan(&hash); err != nil {
		a.err(w, http.StatusInternalServerError, "ошибка")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(body.Old)) != nil {
		a.err(w, http.StatusUnauthorized, "неверный текущий пароль")
		return
	}
	nh, _ := bcrypt.GenerateFromPassword([]byte(body.New), a.Cfg.BcryptCost)
	_, _ = a.Pool.Exec(r.Context(), `UPDATE users SET password_hash=$2, updated_at=NOW() WHERE id=$1`, u.ID, string(nh))
	w.WriteHeader(http.StatusNoContent)
}
