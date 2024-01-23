package picolytics

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/cespare/xxhash"
)

type AsyncEventSaver struct {
	events          chan PicolyticsEvent
	salter          Salter
	validEventNames []string
	o11y            *PicolyticsO11y
}

func NewAsyncEventSaver(events chan PicolyticsEvent, salter Salter, validEventNames []string, o11y *PicolyticsO11y) *AsyncEventSaver {
	return &AsyncEventSaver{
		events:          events,
		salter:          salter,
		validEventNames: validEventNames,
		o11y:            o11y,
	}
}

func (es *AsyncEventSaver) SaveEvent(event PicolyticsEvent) {
	if err := parseEvent(&event, es.validEventNames); err != nil {
		es.o11y.Metrics.eventErrors.WithLabelValues("parse").Add(1)
		es.o11y.Logger.Info("error parsing event", "error", err)
		return
	}
	event.VisitorID = createVisitID(&event, es.salter, es.o11y)
	es.o11y.Metrics.ingestedEvents.WithLabelValues(event.Domain).Add(1)
	if err := queueEvent(es.events, event); err != nil {
		es.o11y.Metrics.eventErrors.WithLabelValues("enqueue").Add(1)
		es.o11y.Logger.Info("error queueing event", "error", err)
		return
	}
}

func parseEvent(event *PicolyticsEvent, validEventNames []string) error {
	if !validEventName(validEventNames, event.Name) {
		return fmt.Errorf("invalid event name: %s", event.Name)
	}
	var err error
	event.Domain, event.Path, err = extractDomainPath(event.Location)
	if err != nil {
		return err
	}
	return nil
}

func createVisitID(event *PicolyticsEvent, salter Salter, o11y *PicolyticsO11y) string {
	salt, err := salter.getSalt()
	if err != nil { // salt is usable even if there is an error
		o11y.Metrics.eventErrors.WithLabelValues("salt").Add(1)
		o11y.Logger.Warn("error getting salt from DB, using old salt", "error", err)
	}

	var builder strings.Builder
	builder.WriteString(salt)
	builder.WriteString(event.Domain)
	builder.WriteString(event.ClientIpDONOTSTORE)
	builder.WriteString(event.UaDONOTSTORE)
	builder.WriteString(event.Lang)
	builder.WriteString(event.Timezone)
	builder.WriteString(fmt.Sprintf("%d%d%d%.2f", event.ScreenW, event.ScreenH, event.PixelDepth, event.PixelRatio))
	hash := xxhash.Sum64String(builder.String())
	return fmt.Sprintf("%x", hash)
}

func extractDomainPath(eventURL string) (string, string, error) {
	if len(eventURL) < 1 {
		return "", "", fmt.Errorf("missing event url")
	}
	parsedURL, err := url.Parse(eventURL)
	if err != nil {
		return "", "", fmt.Errorf("parsing url %s: %v", eventURL, err)
	}
	return strings.TrimPrefix(parsedURL.Hostname(), "www."), parsedURL.Path, nil
}

func queueEvent(events chan PicolyticsEvent, event PicolyticsEvent) error {
	select {
	case events <- event:
	default:
		return fmt.Errorf("event queue full")
	}
	return nil
}

func validEventName(validEventNames []string, name string) bool {
	if len(name) < 1 {
		return false
	}
	for _, n := range validEventNames {
		if name == n {
			return true
		}
	}
	return false
}
