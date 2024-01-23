package picolytics

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"math"
	"math/rand"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/tern/v2/migrate"
)

type PgxIface interface {
	Begin(context.Context) (pgx.Tx, error)
	Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
	Query(context.Context, string, ...interface{}) (pgx.Rows, error)
	QueryRow(context.Context, string, ...interface{}) pgx.Row
	CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error)
	Close()
}

//go:embed migrations
var migrationsFiles embed.FS

// connect to postgres and run migrations with tern
func setupDB(config *Config, o11y *PicolyticsO11y) (*pgxpool.Pool, error) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, config.PgConnString)
	if err != nil {
		return nil, fmt.Errorf("error initializing pgxpool: %v", err)
	}

	// tern migrations requires a connection from the pool
	var conn *pgxpool.Conn
	for attempt := 0; ; attempt++ {
		conn, err = pool.Acquire(ctx)
		defer conn.Release() // Make sure to release the connection when done
		if err == nil {
			break
		}
		if attempt >= config.PgConnAttempts {
			return nil, fmt.Errorf("failed to acquire a connection from the pool after %d tries: %v", attempt, err)
		}
		backoff := backoffWithJitter(attempt)
		o11y.Logger.Warn(fmt.Sprintf("Error connecting to db, trying again in %v", backoff), "error", err)
		time.Sleep(backoff)
	}
	if config.SkipMigrations {
		return pool, nil
	}

	migrator, err := migrate.NewMigrator(ctx, conn.Conn(), "schema_version")
	if err != nil {
		return nil, fmt.Errorf("error initializing migrator: %v", err)
	}
	migrationsFS, err := fs.Sub(migrationsFiles, "migrations")
	if err != nil {
		return nil, fmt.Errorf("error accessing embedded migrations files: %v", err)
	}
	if err := migrator.LoadMigrations(migrationsFS); err != nil {
		return nil, fmt.Errorf("error loading migrations: %v", err)
	}
	if len(migrator.Migrations) < 1 { // no migrations
		return pool, nil
	}

	o11y.Logger.Debug("Running migrations")
	if err := migrator.Migrate(ctx); err != nil {
		return nil, fmt.Errorf("error executing migrations: %v", err)
	}
	o11y.Logger.Debug("Migrations complete")

	return pool, nil
}

func backoffWithJitter(attempt int) time.Duration {
	maxDelay := time.Duration(10) * time.Second
	delay := time.Duration(.5*math.Pow(2, float64(attempt))) * time.Second
	if delay > maxDelay {
		delay = maxDelay
	}
	jitter := float64(delay) * .1
	delayWithJitter := delay + time.Duration(rand.Float64()*jitter-jitter/2)
	return delayWithJitter
}
