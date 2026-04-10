package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	appauth "restaurant-booking/internal/auth"
	"restaurant-booking/internal/config"
	"restaurant-booking/internal/wshub"
)

type Handlers struct {
	Pool *pgxpool.Pool
	RDB  *redis.Client
	Hub  *wshub.Hub
	Cfg  config.Config
}

func NewHandlers(pool *pgxpool.Pool, rdb *redis.Client, hub *wshub.Hub, cfg config.Config) *Handlers {
	return &Handlers{Pool: pool, RDB: rdb, Hub: hub, Cfg: cfg}
}

// Router — монолит: все API + WebSocket + sweeper (локальная разработка без Nginx).
func (a *Handlers) Router() http.Handler {
	return a.RouterMonolith()
}

// RouterMonolith — один процесс со всеми маршрутами.
func (a *Handlers) RouterMonolith() http.Handler {
	r := chi.NewRouter()
	r.Use(UseStandardChi(a.Cfg))
	r.Get("/health", a.handleHealth)
	r.Get("/health/diag", a.handleHealthDiag)
	r.Route("/api", func(r chi.Router) {
		r.Get("/health/diag", a.handleHealthDiag)
		r.Get("/files/*", a.handleUploadedFile)
		a.MountAuth(r)
		a.MountHall(r)
		a.MountReservation(r)
		a.MountPayment(r)
	})
	r.Get("/ws/halls/{id}", a.handleWS)
	go a.runPendingPaymentSweeper()
	return r
}

// RouterAuth — только /api/auth/*.
func (a *Handlers) RouterAuth() http.Handler {
	r := chi.NewRouter()
	r.Use(UseStandardChi(a.Cfg))
	r.Get("/health", a.handleHealth)
	r.Route("/api", func(r chi.Router) {
		a.MountAuth(r)
	})
	return r
}

// RouterHall — залы, layout, блокировки, WebSocket, внутренняя рассылка.
func (a *Handlers) RouterHall() http.Handler {
	r := chi.NewRouter()
	r.Use(UseStandardChi(a.Cfg))
	r.Get("/health", a.handleHealth)
	r.Post("/internal/emit", a.handleInternalEmit)
	r.Route("/api", func(r chi.Router) {
		a.MountHall(r)
	})
	r.Get("/ws/halls/{id}", a.handleWS)
	return r
}

// RouterReservation — брони, owner, waiter, sweeper, события AMQP.
func (a *Handlers) RouterReservation() http.Handler {
	r := chi.NewRouter()
	r.Use(UseStandardChi(a.Cfg))
	r.Get("/health", a.handleHealth)
	r.Route("/api", func(r chi.Router) {
		r.Get("/files/*", a.handleUploadedFile)
		a.MountReservation(r)
	})
	go a.runPendingPaymentSweeper()
	return r
}

// RouterPayment — платежи и вебхуки.
func (a *Handlers) RouterPayment() http.Handler {
	r := chi.NewRouter()
	r.Use(UseStandardChi(a.Cfg))
	r.Get("/health", a.handleHealth)
	r.Route("/api", func(r chi.Router) {
		a.MountPayment(r)
	})
	return r
}

func (a *Handlers) json(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (a *Handlers) err(w http.ResponseWriter, status int, msg string) {
	a.json(w, status, map[string]string{"error": msg})
}

type ctxKey int

const userCtxKey ctxKey = 1

type userCtx struct {
	ID   uuid.UUID
	Role string
}

func (a *Handlers) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if len(h) < 8 || h[:7] != "Bearer " {
			a.err(w, http.StatusUnauthorized, "требуется авторизация")
			return
		}
		claims, err := appauth.ParseAccess([]byte(a.Cfg.JWTSecret), h[7:])
		if err != nil {
			a.err(w, http.StatusUnauthorized, "недействительный токен")
			return
		}
		ctx := context.WithValue(r.Context(), userCtxKey, userCtx{ID: claims.UserID, Role: claims.Role})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func userFrom(r *http.Request) userCtx {
	v, _ := r.Context().Value(userCtxKey).(userCtx)
	return v
}

func userFromOptional(r *http.Request) (userCtx, bool) {
	v, ok := r.Context().Value(userCtxKey).(userCtx)
	return v, ok
}

func (a *Handlers) optionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if strings.HasPrefix(h, "Bearer ") {
			if claims, err := appauth.ParseAccess([]byte(a.Cfg.JWTSecret), h[7:]); err == nil {
				ctx := context.WithValue(r.Context(), userCtxKey, userCtx{ID: claims.UserID, Role: claims.Role})
				r = r.WithContext(ctx)
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (a *Handlers) requireRoles(roles ...string) func(http.Handler) http.Handler {
	allowed := map[string]struct{}{}
	for _, x := range roles {
		allowed[x] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u := userFrom(r)
			if _, ok := allowed[u.Role]; !ok {
				a.err(w, http.StatusForbidden, "недостаточно прав")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
