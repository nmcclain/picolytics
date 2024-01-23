package picolytics

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/nmcclain/picolytics/picolytics/db"
)

func (w *Worker) saveEvents(events []PicolyticsEvent) error {
	start := time.Now()
	ctx := context.Background()
	client := db.New(w.pool)
	eventDomains, err := upsertDomains(ctx, client, events)
	if err != nil {
		return err
	}

	eventSessions, err := upsertSessions(ctx, w.pool, client, events, eventDomains, w.config.SessionTimeoutMin)
	if err != nil {
		return err
	}

	if err := createEvents(ctx, client, events, eventDomains, *eventSessions, w.o11y.Metrics, start); err != nil {
		return err
	}
	return nil
}

type EventDomains map[string]int32

func upsertDomains(ctx context.Context, client *db.Queries, events []PicolyticsEvent) (EventDomains, error) {
	eventDomains := EventDomains{} // domain-> domainID
	for _, e := range events {
		eventDomains[e.Domain] = 0
	}

	for domain := range eventDomains {
		domainID, err := client.UpsertDomain(ctx, domain)
		if err != nil {
			return nil, fmt.Errorf("error upserting domain %s: %v", domain, err)
		}
		eventDomains[domain] = domainID
	}
	return eventDomains, nil
}

type EventSessions map[string]int64

func upsertSessions(ctx context.Context, pool PgxIface, client *db.Queries, events []PicolyticsEvent, domains EventDomains, sessionTimeoutMin int) (*EventSessions, error) {
	eventSessions := EventSessions{} // visitorID-> sessionID
	for _, e := range events {
		tx, err := pool.Begin(ctx)
		if err != nil {
			return nil, fmt.Errorf("error beginning transaction: %v", err)
		}

		txClient := client.WithTx(tx)
		sessionID, err := txClient.GetSession(ctx, db.GetSessionParams{
			VisitorID: newPGText(e.VisitorID),
			Column2:   pgtype.Interval{Microseconds: int64(sessionTimeoutMin) * 1000 * 1000 * 60, Valid: true},
		})
		if err != nil {
			if err != pgx.ErrNoRows {
				_ = tx.Rollback(ctx)
				return nil, fmt.Errorf("error getting session: %v", err)
			}
			sessionID, err = txClient.CreateSession(ctx, db.CreateSessionParams{
				DomainID:       domains[e.Domain],
				VisitorID:      newPGText(e.VisitorID),
				EntryPath:      e.Path,
				ExitPath:       e.Path,
				Country:        newPGText(e.Country),
				Latitude:       newPGFloat8(e.Latitude),
				Longitude:      newPGFloat8(e.Longitude),
				Subdivision:    newPGText(e.Subdivision),
				City:           newPGText(e.City),
				Browser:        newPGText(e.Browser),
				BrowserVersion: newPGText(e.BrowserVersion),
				Os:             newPGText(e.Os),
				OsVersion:      newPGText(e.OsVersion),
				Platform:       newPGText(e.Platform),
				DeviceType:     newPGText(e.DeviceType),
				Bot:            e.Bot,
				ScreenW:        newPGInt4(e.ScreenW),
				ScreenH:        newPGInt4(e.ScreenH),
				Timezone:       newPGText(e.Timezone),
				PixelRatio:     newPGFloat8(e.PixelRatio),
				PixelDepth:     newPGInt4(e.PixelDepth),
				UtmSource:      newPGText(e.UtmSource),
				UtmMedium:      newPGText(e.UtmMedium),
				UtmTerm:        newPGText(e.UtmTerm),
				UtmContent:     newPGText(e.UtmContent),
				UtmCampaign:    newPGText(e.UtmCampaign),
			})
			if err != nil {
				_ = tx.Rollback(ctx)
				return nil, fmt.Errorf("error creating session: %v", err)
			}
		} else {
			if err := txClient.UpdateSession(ctx, db.UpdateSessionParams{ // updated_at and duration are updated by the db
				ID:        sessionID,
				ExitPath:  e.Path,
				EventName: e.Name, // the event name is used to determine if "bounce" should be set
			}); err != nil {
				_ = tx.Rollback(ctx)
				return nil, fmt.Errorf("error updating session: %v", err)
			}
		}
		eventSessions[e.VisitorID] = sessionID
		if err = tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("error committing transaction: %v", err)
		}
	}
	return &eventSessions, nil
}

func createEvents(ctx context.Context, client *db.Queries, events []PicolyticsEvent,
	eventDomains EventDomains,
	eventSessions EventSessions,
	metrics *Metrics,
	start time.Time) error {
	params := []db.CreateEventsParams{}
	for _, e := range events {
		params = append(params, db.CreateEventsParams{
			DomainID:  eventDomains[e.Domain],
			VisitorID: e.VisitorID,
			SessionID: eventSessions[e.VisitorID],
			Name:      e.Name,
			Path:      e.Path,
			Referrer:  e.Referrer,
			LoadTime:  e.LoadTime,
			Ttfb:      e.TTFB,
		})
		metrics.ingestLatency.WithLabelValues(e.Domain).Observe(float64(time.Since(e.Created).Seconds()))
		metrics.workerLatency.WithLabelValues(e.Domain).Observe(float64(time.Since(start).Seconds()))
	}

	if _, err := client.CreateEvents(ctx, params); err != nil {
		return fmt.Errorf("error writing event to db: %v", err)
	}
	return nil
}

func newPGText(val string) pgtype.Text {
	return pgtype.Text{String: val, Valid: true}
}

func newPGInt4(val int32) pgtype.Int4 {
	return pgtype.Int4{Int32: val, Valid: true}
}

func newPGFloat8(val float64) pgtype.Float8 {
	return pgtype.Float8{Float64: val, Valid: true}
}
