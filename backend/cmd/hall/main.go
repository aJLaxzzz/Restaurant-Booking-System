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

	"restaurant-booking/internal/cmdutil"
	"restaurant-booking/internal/config"
	"restaurant-booking/internal/db"
	"restaurant-booking/internal/handlers"
	"restaurant-booking/internal/wshub"
)

func main() {
	cfg := config.Load()
	cfg.Addr = cmdutil.ListenAddr(":8082")
	ctx := context.Background()

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

	hub := wshub.New()
	a := handlers.NewHandlers(pool, rdb, hub, cfg)
	srv := &http.Server{Addr: cfg.Addr, Handler: a.RouterHall()}

	go func() {
		log.Printf("hall service http://%s (ws /ws/halls/{id}, internal POST /internal/emit)", cfg.Addr)
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
