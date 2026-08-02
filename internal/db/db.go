// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// Package db owns the PostgreSQL connection pool and all typed query helpers.
//
// Architecture:
//   - DB wraps pgxpool.Pool — one pool shared across the entire process.
//   - Open() connects, verifies reachability with Ping, and returns a ready *DB.
//   - Every public method on *DB is a named query (no raw SQL outside this package).
//   - All queries accept a context so HTTP request cancellations propagate to the DB.
//   - Errors are always wrapped with fmt.Errorf("operation: %w", err) so callers
//     can use errors.Is() to match pgx sentinel errors like pgx.ErrNoRows.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ALabiyb/platform_devportal/internal/config"
)

// DB wraps a pgxpool.Pool and exposes all query methods for devportal.
// Always pass *DB by pointer — it holds a reference to the shared pool.
type DB struct {
	pool *pgxpool.Pool
}

// Open creates and validates a PostgreSQL connection pool from cfg.
// It dials the database, runs a ping to confirm reachability, and returns
// a ready *DB. Caller must call Close() when the application shuts down.
func Open(ctx context.Context, cfg *config.Config) (*DB, error) {
	// Build the DSN from individual config fields so each field can come
	// from its own environment variable without exposing the full URL.
	dsn := fmt.Sprintf(
		"host=%s port=%s dbname=%s user=%s password=%s sslmode=%s",
		cfg.DBHost, cfg.DBPort, cfg.DBName, cfg.DBUser, cfg.DBPassword, cfg.DBSSLMode,
	)

	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("db: parse config: %w", err)
	}

	// Pool sizing — devportal is an HTTP server with short-lived request-scoped
	// queries. pgbouncer sits in front so we don't need a large pool here;
	// pgbouncer multiplexes many devportal connections onto few PG connections.
	poolCfg.MaxConns = 20                      // max simultaneous connections to postgres/pgbouncer
	poolCfg.MinConns = 2                        // keep 2 connections warm to avoid cold-start latency
	poolCfg.MaxConnLifetime = 30 * time.Minute  // recycle connections to avoid stale state
	poolCfg.MaxConnIdleTime = 5 * time.Minute   // release idle connections back to the OS
	poolCfg.HealthCheckPeriod = 1 * time.Minute // background ping to detect dead connections early

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("db: create pool: %w", err)
	}

	// Ping verifies the database is reachable and the credentials are correct.
	// Fail fast here so the service doesn't start and then crash on the first request.
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}

	return &DB{pool: pool}, nil
}

// Close drains in-flight queries and releases all pool connections.
// Call this during graceful shutdown after the HTTP server has stopped
// accepting new requests.
func (db *DB) Close() {
	db.pool.Close()
}

// Ping checks that the database connection is still alive.
// Used by the /readyz health endpoint (added in Day 04).
func (db *DB) Ping(ctx context.Context) error {
	return db.pool.Ping(ctx)
}
