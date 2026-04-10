package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"restaurant-booking/internal/amqp"
	"restaurant-booking/internal/config"
	"restaurant-booking/internal/db"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	if err := amqp.Init(cfg.RabbitMQURL); err != nil {
		log.Printf("rabbitmq: %v", err)
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

	consumeCtx, cancelConsume := context.WithCancel(context.Background())
	defer cancelConsume()

	if err := amqp.RunConsumer(consumeCtx, "notify_events", func(c context.Context, key string, body []byte) error {
		return persistNotification(c, pool, key, body)
	}); err != nil {
		log.Printf("consumer start: %v", err)
	}

	go runReminderLoop(pool)

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8085"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	srv := &http.Server{Addr: addr, Handler: mux}
	go func() {
		log.Printf("notify-worker health http://localhost%s/health", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Println("health server:", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	cancelConsume()
	shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shCtx)
}

func persistNotification(ctx context.Context, pool *pgxpool.Pool, key string, body []byte) error {
	var payload map[string]any
	_ = json.Unmarshal(body, &payload)
	tpl := string(body)
	if len(tpl) > 8000 {
		tpl = tpl[:8000]
	}
	var uidArg any
	if v, ok := payload["user_id"].(string); ok {
		if id, err := uuid.Parse(v); err == nil {
			uidArg = id
		}
	}
	log.Printf("[notify] %s (user_id=%v) SMS noop: payload logged", key, uidArg)
	_, err := pool.Exec(ctx, `
		INSERT INTO notifications (user_id, type, template, status, sent_at)
		VALUES ($1, $2, $3, 'queued', NOW())`, uidArg, key, tpl)
	return err
}

func runReminderLoop(pool *pgxpool.Pool) {
	run := func() {
		c, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		sendReservationReminders(c, pool)
		cancel()
	}
	run()
	t := time.NewTicker(5 * time.Minute)
	defer t.Stop()
	for range t.C {
		run()
	}
}

func sendReservationReminders(ctx context.Context, pool *pgxpool.Pool) {
	rows, err := pool.Query(ctx, `
		SELECT r.id, r.user_id FROM reservations r
		WHERE r.status = 'confirmed'
		AND r.start_time > NOW() + INTERVAL '115 minutes'
		AND r.start_time <= NOW() + INTERVAL '125 minutes'
		AND NOT EXISTS (
			SELECT 1 FROM reservation_reminder_sent s
			WHERE s.reservation_id = r.id AND s.kind = 'reminder_2h'
		)`)
	if err != nil {
		log.Printf("reminder query: %v", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var rid, uid uuid.UUID
		if err := rows.Scan(&rid, &uid); err != nil {
			continue
		}
		payload := map[string]any{"reservation_id": rid.String(), "user_id": uid.String()}
		if err := amqp.PublishJSON(ctx, "reservation.reminder_2h", payload); err != nil {
			log.Printf("reminder publish: %v", err)
			continue
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO reservation_reminder_sent (reservation_id, kind) VALUES ($1, 'reminder_2h')
			ON CONFLICT (reservation_id, kind) DO NOTHING`, rid); err != nil {
			log.Printf("reminder mark: %v", err)
		}
	}
}
