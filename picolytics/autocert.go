package picolytics

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/nmcclain/picolytics/picolytics/db"
	"golang.org/x/crypto/acme/autocert"
)

type PostgresAutocertCache struct {
	pool   PgxIface
	client *db.Queries
}

func NewPostgresCache(pool PgxIface) *PostgresAutocertCache {
	return &PostgresAutocertCache{
		pool:   pool,
		client: db.New(pool),
	}
}

func (c *PostgresAutocertCache) Get(ctx context.Context, key string) ([]byte, error) {
	d, err := c.client.AutocertCacheGet(ctx, key)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, autocert.ErrCacheMiss
		}
		return nil, err
	}
	return d, nil
}

func (c *PostgresAutocertCache) Put(ctx context.Context, key string, data []byte) error {
	return c.client.AutocertCachePut(context.Background(), db.AutocertCachePutParams{
		Key:  key,
		Data: data,
	})
}

func (c *PostgresAutocertCache) Delete(ctx context.Context, key string) error {
	return c.client.AutocertCacheDelete(context.Background(), key)
}
