// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// dbprovider.go implements DBProvider by connecting directly to the App PostgreSQL
// instance (separate from devportal's own DB) and running DDL statements.
//
// Connection: APP_DB_HOST + APP_DB_PORT + APP_DB_ADMIN_USER + APP_DB_ADMIN_PASSWORD
// A fresh *pgx.Conn is opened per method call — these are rare, one-off operations
// (one provisioning run per project) so pool overhead is not warranted.
//
// SQL injection prevention:
//
//	Identifiers (database name, username): pgx.Identifier{}.Sanitize() — double-quotes and escapes.
//	Password literals: pgQuoteLiteral() — single-quotes with internal quote escaping.

package plugin

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/ALabiyb/platform_devportal/internal/config"
)

// PostgresDBAdapter implements DBProvider. Connects to the App PostgreSQL instance
// as a superuser and provisions databases and users for developer applications.
type PostgresDBAdapter struct {
	cfg *config.Config
}

// NewPostgresDBAdapter constructs a PostgresDBAdapter from config.
func NewPostgresDBAdapter(cfg *config.Config) *PostgresDBAdapter {
	return &PostgresDBAdapter{cfg: cfg}
}

// EnsureDatabase creates the named database if it does not already exist.
// Idempotent — queries pg_database before issuing CREATE DATABASE.
func (p *PostgresDBAdapter) EnsureDatabase(ctx context.Context, dbName string) error {
	conn, err := p.connect(ctx)
	if err != nil {
		return fmt.Errorf("dbprovider EnsureDatabase connect: %w", err)
	}
	defer conn.Close(ctx)

	var exists bool
	if err := conn.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", dbName,
	).Scan(&exists); err != nil {
		return fmt.Errorf("dbprovider EnsureDatabase check: %w", err)
	}
	if exists {
		return nil
	}

	if _, err := conn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{dbName}.Sanitize()); err != nil {
		return fmt.Errorf("dbprovider EnsureDatabase create: %w", err)
	}
	return nil
}

// EnsureUser creates the named database user if it does not already exist.
// Idempotent — queries pg_roles before issuing CREATE USER.
func (p *PostgresDBAdapter) EnsureUser(ctx context.Context, username, password string) error {
	conn, err := p.connect(ctx)
	if err != nil {
		return fmt.Errorf("dbprovider EnsureUser connect: %w", err)
	}
	defer conn.Close(ctx)

	var exists bool
	if err := conn.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)", username,
	).Scan(&exists); err != nil {
		return fmt.Errorf("dbprovider EnsureUser check: %w", err)
	}
	if exists {
		return nil
	}

	sql := fmt.Sprintf("CREATE USER %s WITH PASSWORD %s",
		pgx.Identifier{username}.Sanitize(),
		pgQuoteLiteral(password),
	)
	if _, err := conn.Exec(ctx, sql); err != nil {
		return fmt.Errorf("dbprovider EnsureUser create: %w", err)
	}
	return nil
}

// GrantPrivileges grants ALL PRIVILEGES on the database to the user.
// Safe to call multiple times — PostgreSQL GRANT is idempotent.
func (p *PostgresDBAdapter) GrantPrivileges(ctx context.Context, dbName, username string) error {
	conn, err := p.connect(ctx)
	if err != nil {
		return fmt.Errorf("dbprovider GrantPrivileges connect: %w", err)
	}
	defer conn.Close(ctx)

	sql := fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE %s TO %s",
		pgx.Identifier{dbName}.Sanitize(),
		pgx.Identifier{username}.Sanitize(),
	)
	if _, err := conn.Exec(ctx, sql); err != nil {
		return fmt.Errorf("dbprovider GrantPrivileges: %w", err)
	}
	return nil
}

// connect opens a single pgx.Conn to the App PostgreSQL instance.
// Always connects to the "postgres" admin database — DDL for CREATE DATABASE
// and CREATE USER must be issued outside the target database.
func (p *PostgresDBAdapter) connect(ctx context.Context) (*pgx.Conn, error) {
	dsn := fmt.Sprintf("host=%s port=%s dbname=postgres user=%s password=%s sslmode=disable",
		p.cfg.AppDBHost,
		p.cfg.AppDBPort,
		p.cfg.AppDBUser,
		p.cfg.AppDBPassword,
	)
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("dbprovider connect: %w", err)
	}
	return conn, nil
}

// pgQuoteLiteral wraps s in single quotes and escapes internal single quotes.
// Used for password literals in CREATE USER DDL where parameterized queries
// are not available (DDL statements cannot use $1 placeholders).
func pgQuoteLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
