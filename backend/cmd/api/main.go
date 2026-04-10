package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"restaurant-booking/internal/amqp"
	"restaurant-booking/internal/config"
	"restaurant-booking/internal/db"
	"restaurant-booking/internal/handlers"
	"restaurant-booking/internal/wshub"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	if err := amqp.Init(cfg.RabbitMQURL); err != nil {
		log.Printf("rabbitmq: %v (события отключены)", err)
	}
	defer amqp.Close()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal("postgres: ", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		log.Fatal("migrate: ", err)
	}

	var dbName string
	var nRest int
	if err := pool.QueryRow(ctx, `SELECT current_database()`).Scan(&dbName); err == nil {
		_ = pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM restaurants`).Scan(&nRest)
		log.Printf("postgres: БД=%s ресторанов=%d url=%s", dbName, nRest, config.RedactDatabaseURL(cfg.DatabaseURL))
	}

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	defer rdb.Close()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatal("redis: ", err)
	}

	hub := wshub.New()
	a := handlers.NewHandlers(pool, rdb, hub, cfg)
	a.Seed(ctx)

	srv := &http.Server{Addr: cfg.Addr, Handler: a.RouterMonolith()}

	go func() {
		log.Printf("Restobook API (монолит) http://localhost%s", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(c)
}
