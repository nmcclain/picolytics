package picolytics

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/nmcclain/picolytics/picolytics/db"
)

type Pruner struct {
	config *Config
	pool   PgxIface
	o11y   *PicolyticsO11y
	client *db.Queries
}

func NewPruner(config *Config, pool PgxIface, o11y *PicolyticsO11y) (*Pruner, error) {
	p := Pruner{
		config: config,
		pool:   pool,
		o11y:   o11y,
	}
	p.client = db.New(p.pool)
	return &p, nil
}

const microsecondsPerDay = 24 * 60 * 60 * 1000000

func (p *Pruner) prune() {
	ticker := time.NewTicker(time.Hour * time.Duration(p.config.PruneCheckHours))
	defer ticker.Stop()
	for range ticker.C {
		p.o11y.Logger.Debug("Pruning sesions and events", "days", p.config.PruneDays)
		ctx := context.Background()
		if err := p.client.PruneEvents(ctx, pgtype.Interval{Microseconds: int64(p.config.PruneDays * microsecondsPerDay)}); err != nil {
			p.o11y.Logger.Error("prune events error", "error", err)
		}
		if err := p.client.PruneSessions(ctx, pgtype.Interval{Microseconds: int64(p.config.PruneDays * microsecondsPerDay)}); err != nil {
			p.o11y.Logger.Error("prune sessions error", "error", err)
		}
	}
}
