package picolytics

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nmcclain/picolytics/picolytics/db"
)

type Salter interface {
	getSalt() (string, error)
}

type DailySalt struct {
	salt       string
	created_at time.Time
	pool       PgxIface
	client     *db.Queries
	lock       sync.Mutex
}

func NewDailySalt(pool PgxIface) *DailySalt {
	ds := DailySalt{pool: pool}
	ds.client = db.New(ds.pool)
	ds.salt = uuid.NewString() // this would only be used incase the salt DB query never works
	return &ds
}

// getSalt returns a usable salt, with or without an error
func (ds *DailySalt) getSalt() (string, error) {
	ds.lock.Lock()
	defer ds.lock.Unlock()
	var err error
	if ds.created_at.Before(time.Now().Add(-24 * time.Hour)) {
		ctx := context.Background()
		tx, err := ds.pool.Begin(ctx)
		if err != nil {
			return "", err
		}
		defer func() {
			if err != nil {
				_ = tx.Rollback(ctx)
			}
		}()
		txClient := ds.client.WithTx(tx)
		if err := txClient.UpdateSalt(ctx); err != nil {
			return "", err
		}
		result, err := txClient.GetSalt(ctx)
		if err != nil {
			return "", err
		}
		if err = tx.Commit(ctx); err != nil {
			return "", fmt.Errorf("salter error committing transaction: %v", err)
		}
		ds.created_at = result.CreatedAt.Time
		ds.salt = string(result.Salt.Bytes[:])
		return ds.salt, nil
	}
	return ds.salt, err
}
