package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Addr              string
	DatabaseURL       string
	RedisAddr         string
	RabbitMQURL       string
	JWTSecret         string
	AccessTTL         time.Duration
	RefreshTTL        time.Duration
	WebhookSecret     string
	StripeWebhookSecret string
	FrontendOrigin    string
	BcryptCost        int
	PaymentPendingTTL time.Duration
	// HallServiceURL — для reservation/payment: HTTP-вызов рассылки в hall-сервис (микросервисы).
	HallServiceURL string
	InternalSecret string
	// Платежи
	YooKassaShopID    string
	YooKassaSecretKey string
	StripeSecretKey   string
	PublicAppURL      string // редирект после оплаты
}

func Load() Config {
	return Config{
		Addr:                getEnv("ADDR", ":8080"),
		DatabaseURL:         getEnv("DATABASE_URL", "postgres://rbs:rbs@localhost:5433/restaurant_booking?sslmode=disable"),
		RedisAddr:           getEnv("REDIS_ADDR", "localhost:6379"),
		RabbitMQURL:         getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		JWTSecret:           getEnv("JWT_SECRET", "dev-secret-change-me-min-32-chars-long!!"),
		AccessTTL:           15 * time.Minute,
		RefreshTTL:          7 * 24 * time.Hour,
		WebhookSecret:       getEnv("WEBHOOK_SECRET", "webhook-hmac-secret-change"),
		StripeWebhookSecret: getEnv("STRIPE_WEBHOOK_SECRET", ""),
		FrontendOrigin:      getEnv("FRONTEND_ORIGIN", "http://localhost:5173"),
		BcryptCost:          12,
		PaymentPendingTTL:   10 * time.Minute,
		HallServiceURL:      getEnv("HALL_SERVICE_URL", ""),
		InternalSecret:      getEnv("INTERNAL_SECRET", "internal-shared-secret-change"),
		YooKassaShopID:      getEnv("YOOKASSA_SHOP_ID", ""),
		YooKassaSecretKey:   getEnv("YOOKASSA_SECRET_KEY", ""),
		StripeSecretKey:     getEnv("STRIPE_SECRET_KEY", ""),
		PublicAppURL:        getEnv("PUBLIC_APP_URL", "http://localhost:5173"),
	}
}

func getEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func GetIntSetting(s string, def int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
