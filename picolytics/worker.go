package picolytics

import (
	"fmt"
	"time"

	"github.com/oschwald/maxminddb-golang"
)

type Worker struct {
	events chan PicolyticsEvent
	config *Config
	pool   PgxIface
	o11y   *PicolyticsO11y
	geo    *maxminddb.Reader
	quit   chan bool
}

func NewWorker(config *Config, pool PgxIface, o11y *PicolyticsO11y) (*Worker, error) {
	w := Worker{
		config: config,
		pool:   pool,
		o11y:   o11y,
		quit:   make(chan bool, 1),
	}
	var err error
	w.geo, err = maxminddb.Open(config.GeoIPFile)
	if err != nil {
		return nil, fmt.Errorf("error opening GeoIP database: %v", err)
	}
	w.events = make(chan PicolyticsEvent, config.QueueSize)
	return &w, nil
}

func (w *Worker) Shutdown() {
	w.quit <- true
}

func (w *Worker) processQueuedEvents() {
	toProcess := []PicolyticsEvent{}
	ticker := time.NewTicker(time.Duration(w.config.BatchMaxMsec) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case e := <-w.events:
			w.o11y.Metrics.queueUtilization.Set(float64(len(w.events)))
			if err := enrichEvent(&e, w.geo); err != nil {
				w.o11y.Metrics.eventErrors.WithLabelValues("enrich").Add(1)
				w.o11y.Logger.Warn("Error enriching event - saving anyway", "error", err)
			}
			e.ClientIpDONOTSTORE = "" // explicitly never store client IP
			e.UaDONOTSTORE = ""       // explicitly never store useragent

			toProcess = append(toProcess, e)
			if len(toProcess) >= w.config.BatchMaxSize {
				ticker.Reset(time.Duration(w.config.BatchMaxMsec) * time.Millisecond)
				if err := w.processBatch(&toProcess, "BatchMaxSize"); err != nil {
					w.o11y.Logger.Error("error saving queue events to db", "events", len(toProcess), "error", err)
					w.o11y.Metrics.eventErrors.WithLabelValues("save").Add(1)
				}
			}
		case <-ticker.C:
			w.o11y.Metrics.queueUtilization.Set(float64(len(w.events)))
			if len(toProcess) > 0 {
				if err := w.processBatch(&toProcess, "BatchMaxMsec"); err != nil {
					w.o11y.Logger.Error("error saving queue events to db", "events", len(toProcess), "error", err)
					w.o11y.Metrics.eventErrors.WithLabelValues("save").Add(1)
				}
			}
		case <-w.quit:
			w.geo.Close()
			return
		}
	}
}

func (w *Worker) processBatch(toProcess *[]PicolyticsEvent, reason string) error {
	if err := w.saveEvents(*toProcess); err != nil {
		return err
	}
	*toProcess = []PicolyticsEvent{}
	return nil
}
