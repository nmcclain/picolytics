package picolytics

import (
	"errors"
	"log/slog"
	"os"
	"testing"
)

type TestSalter struct {
}

func (s TestSalter) getSalt() (string, error) {
	return "salt", nil
}
func TestAsyncSaveEvent(t *testing.T) {

	o11yMock := &PicolyticsO11y{
		Logger:  slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
		Metrics: setupMetrics(1, "", "", ""),
	}
	validEventNames := []string{"load", "ping"}
	salter := TestSalter{}
	events := make(chan PicolyticsEvent, 1)
	eventSaver := NewAsyncEventSaver(events, salter, validEventNames, o11yMock)

	// Test valid event processing
	validEvent := PicolyticsEvent{Name: "load", Location: "http://www.example.com/goodpath"}
	eventSaver.SaveEvent(validEvent)

	// Validate that the event is queued correctly
	select {
	case event := <-events:
		if event.Name != validEvent.Name {
			t.Errorf("Expected event name %s, got %s", validEvent.Name, event.Name)
		}
	default:
		t.Error("Event was not queued")
	}

	// TODO: Test event parsing error handling
}

func TestParseEvent(t *testing.T) { // onlky need to test name validation, and domain+path extraction
	validEventNames := []string{"load", "ping"}
	tests := []struct {
		name       string
		event      PicolyticsEvent
		wantEvent  string
		wantDomain string
		wantPath   string
		wantErr    error
	}{
		{
			name: "valid event",
			event: PicolyticsEvent{
				Name:     "load",
				Location: "http://www.example.com/goodpath",
			},
			wantEvent:  "load",
			wantDomain: "example.com",
			wantPath:   "/goodpath",
			wantErr:    nil,
		},
		{
			name: "invalid event name",
			event: PicolyticsEvent{
				Name:     "invalidEventName",
				Location: "http://www.example.com/goodpath",
			},
			wantEvent: "invalidEventName",
			wantErr:   errors.New("invalid event name: invalidEventName"),
		},
		{
			name: "empty event name",
			event: PicolyticsEvent{
				Location: "http://www.example.com/goodpath",
			},
			wantEvent: "",
			wantErr:   errors.New("invalid event name: "),
		},
		{
			name: "empty url",
			event: PicolyticsEvent{
				Name:     "load",
				Location: "",
			},
			wantEvent: "load",
			wantErr:   errors.New("missing event url"),
		},
		{
			name: "invalid url",
			event: PicolyticsEvent{
				Name:     "load",
				Location: ":in-valid-url",
			},
			wantEvent: "load",
			wantErr:   errors.New(`parsing url :in-valid-url: parse ":in-valid-url": missing protocol scheme`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parseEvent(&tt.event, validEventNames)
			if err == nil {
				if err != tt.wantErr {
					t.Errorf("parseEvent() error = %+v, wantErr %+v", err, tt.wantErr)
				}
			} else {
				if err.Error() != tt.wantErr.Error() {
					t.Errorf("parseEvent() error = %+v, wantErr %+v", err, tt.wantErr)
				}
			}
			if tt.event.Name != tt.wantEvent {
				t.Errorf("parseEvent() event name = %v, want %v", tt.event.Name, tt.wantEvent)
			}
			if tt.event.Domain != tt.wantDomain {
				t.Errorf("parseEvent() event domain = %v, want %v", tt.event.Domain, tt.wantDomain)
			}
			if tt.event.Path != tt.wantPath {
				t.Errorf("parseEvent() event path = %v, want %v", tt.event.Path, tt.wantPath)
			}
		})
	}
}

func TestQueueEvent(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(chan PicolyticsEvent)
		event     PicolyticsEvent
		wantErr   bool
		errMsg    string
	}{
		{
			name: "successful enqueue",
			setupFunc: func(events chan PicolyticsEvent) {
			},
			event: PicolyticsEvent{
				Name:     "load",
				Location: "http://www.example.com/goodpath",
			},
			wantErr: false,
			errMsg:  "",
		},
		{
			name: "queue full",
			setupFunc: func(events chan PicolyticsEvent) {
				for i := 0; i < cap(events); i++ {
					events <- PicolyticsEvent{}
				}
			},
			event: PicolyticsEvent{
				Name:     "load",
				Location: "http://www.example.com/goodpath",
			},
			wantErr: true,
			errMsg:  "event queue full",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := make(chan PicolyticsEvent, 1)
			tt.setupFunc(events)

			err := queueEvent(events, tt.event)
			if (err != nil) != tt.wantErr {
				t.Errorf("queueEvent() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && err.Error() != tt.errMsg {
				t.Errorf("queueEvent() error = %v, want %v", err, tt.errMsg)
			}
		})
	}
}

func TestCreateVisitID(t *testing.T) { // only need to test name validation, and domain+path extraction
	salter := TestSalter{}
	o11yMock := &PicolyticsO11y{
		Logger:  slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
		Metrics: setupMetrics(1, "", "", ""),
	}
	tests := []struct {
		name   string
		event  PicolyticsEvent
		wantId string
	}{
		{
			name:   "empty event",
			event:  PicolyticsEvent{},
			wantId: "96ceb83f021da890",
		},
		{
			name: "typical event",
			event: PicolyticsEvent{
				Domain:             "example.com",
				ClientIpDONOTSTORE: "8.8.8.8",
				UaDONOTSTORE:       "Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Mobile Safari/537.36",
				Lang:               "en-US",
				Timezone:           "America/Los_Angeles",
				ScreenW:            1020,
				ScreenH:            840,
				PixelDepth:         8,
				PixelRatio:         1,
			},
			wantId: "22dc4269dc06fe00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := createVisitID(&tt.event, salter, o11yMock)
			if id != tt.wantId {
				t.Errorf("createVisitID() got=%v, want=%v", id, tt.wantId)
			}
		})
	}
}
