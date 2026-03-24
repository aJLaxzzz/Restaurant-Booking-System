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
	"restaurant-booking/internal/cmdutil"
	"restaurant-booking/internal/config"
	"restaurant-booking/internal/db"
	"restaurant-booking/internal/handlers"
)

func main() {
	cfg := config.Load()
	cfg.Addr = cmdutil.ListenAddr(":8084")
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

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	defer rdb.Close()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatal("redis: ", err)
	}

	a := handlers.NewHandlers(pool, rdb, nil, cfg)
	srv := &http.Server{Addr: cfg.Addr, Handler: a.RouterPayment()}

	go func() {
		log.Printf("payment service http://%s", cfg.Addr)
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
