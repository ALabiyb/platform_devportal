// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// dbprovider.go will implement DBProvider by connecting to the App PostgreSQL
// instance (separate from devportal's own DB) and running DDL statements.
// Full implementation: Day 08.
//
// DDL executed per environment:
//   CREATE DATABASE <project>_<env> WITH OWNER <project>_<env>_user;
//   CREATE USER <project>_<env>_user WITH PASSWORD '<generated>';
//   GRANT ALL PRIVILEGES ON DATABASE <project>_<env> TO <project>_<env>_user;
//
// Connection uses APP_DB_HOST + APP_DB_ADMIN_PASSWORD (postgres superuser).
// Passwords are generated with crypto/rand and stored via the Crypto module.

package plugin

import (
	"context"
	"fmt"

	"github.com/ALabiyb/platform_devportal/internal/config"
)

// PostgresDBAdapter implements DBProvider. Stub until Day 08.
type PostgresDBAdapter struct {
	cfg *config.Config
}

// NewPostgresDBAdapter constructs a PostgresDBAdapter stub.
func NewPostgresDBAdapter(cfg *config.Config) *PostgresDBAdapter {
	return &PostgresDBAdapter{cfg: cfg}
}

func (p *PostgresDBAdapter) EnsureDatabase(ctx context.Context, dbName string) error {
	return fmt.Errorf("PostgresDBAdapter.EnsureDatabase: %w (Day 08)", ErrNotImplemented)
}

func (p *PostgresDBAdapter) EnsureUser(ctx context.Context, username, password string) error {
	return fmt.Errorf("PostgresDBAdapter.EnsureUser: %w (Day 08)", ErrNotImplemented)
}

func (p *PostgresDBAdapter) GrantPrivileges(ctx context.Context, dbName, username string) error {
	return fmt.Errorf("PostgresDBAdapter.GrantPrivileges: %w (Day 08)", ErrNotImplemented)
}
