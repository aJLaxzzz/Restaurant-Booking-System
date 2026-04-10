package db

import (
	"context"
	"embed"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrations embed.FS

func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = 50
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		filename TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`); err != nil {
		return fmt.Errorf("schema_migrations: %w", err)
	}

	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	// БД без записей о миграциях, но схема уже содержит 005 (extra_json) — помечаем 001–005 применёнными,
	// чтобы не гонять повторно UPDATE из 003. Новые файлы (006+) по-прежнему применятся.
	var smCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM schema_migrations`).Scan(&smCount); err != nil {
		return fmt.Errorf("count schema_migrations: %w", err)
	}
	if smCount == 0 {
		var hasExtraJSON bool
		if err := pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_schema = 'public' AND table_name = 'restaurants' AND column_name = 'extra_json'
			)`).Scan(&hasExtraJSON); err != nil {
			return fmt.Errorf("legacy check: %w", err)
		}
		if hasExtraJSON {
			legacy := []string{
				"001_initial_schema.sql",
			}
			for _, name := range legacy {
				if _, err := pool.Exec(ctx, `INSERT INTO schema_migrations (filename) VALUES ($1) ON CONFLICT DO NOTHING`, name); err != nil {
					return fmt.Errorf("bootstrap %s: %w", name, err)
				}
			}
		}
	}

	for _, name := range names {
		var already int
		if err := pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM schema_migrations WHERE filename = $1`, name).Scan(&already); err != nil {
			return fmt.Errorf("check %s: %w", name, err)
		}
		if already > 0 {
			continue
		}
		sqlBytes, err := migrations.ReadFile(path.Join("migrations", name))
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		if _, err := pool.Exec(ctx, string(sqlBytes)); err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO schema_migrations (filename) VALUES ($1)`, name); err != nil {
			return fmt.Errorf("record %s: %w", name, err)
		}
	}
	return nil
}
