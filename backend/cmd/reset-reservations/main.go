// Одноразовая утилита: очистка броней без psql (эквивалент scripts/reset_reservations.sql).
// Запуск из каталога backend: go run ./cmd/reset-reservations
// Затем поднимите API — при COUNT(reservations)=0 Seed() заново создаст демо-брони.
package main

import (
	"context"
	"log"
	"os"

	"restaurant-booking/internal/config"
	"restaurant-booking/internal/db"
	"restaurant-booking/internal/handlers"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal("postgres: ", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		log.Fatal("migrate: ", err)
	}

	a := handlers.NewHandlers(pool, nil, nil, cfg)
	if err := a.ResetDemoReservations(ctx); err != nil {
		log.Fatal(err)
	}
	log.Println("Готово. Запустите backend — Seed() дозаполнит демо-данные, если таблица reservations пуста.")
	os.Exit(0)
}
