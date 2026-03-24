package handlers

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"restaurant-booking/internal/config"
)

// UseStandardChi — общий стек для всех HTTP-сервисов.
func UseStandardChi(cfg config.Config) func(http.Handler) http.Handler {
	c := cors.Handler(cors.Options{
		AllowedOrigins:   []string{cfg.FrontendOrigin},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Idempotency-Key", "Stripe-Signature"},
		AllowCredentials: true,
		MaxAge:           300,
	})
	return func(next http.Handler) http.Handler {
		h := middleware.RequestID(next)
		h = middleware.RealIP(h)
		h = middleware.Logger(h)
		h = middleware.Recoverer(h)
		h = middleware.Timeout(60 * time.Second)(h)
		h = c(h)
		return h
	}
}
