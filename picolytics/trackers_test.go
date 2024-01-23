package picolytics

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

// A simple mock for echo.Context
type mockEchoContext struct {
	echo.Context
	request *http.Request
}

func (m *mockEchoContext) Request() *http.Request {
	return m.request
}

func (m *mockEchoContext) RealIP() string {
	return "127.0.0.1"
}

func (m *mockEchoContext) String(code int, s string) error {
	return nil
}

func (m *mockEchoContext) NoContent(code int) error {
	return nil
}

type TestEventSaver struct {
	events chan PicolyticsEvent
}

func (es TestEventSaver) SaveEvent(event PicolyticsEvent) {
	es.events <- event
}

func TestRecordPicolyticsEvent(t *testing.T) {
	eventSaver := TestEventSaver{
		events: make(chan PicolyticsEvent, 1),
	}
	trackers := NewTrackers(eventSaver, 1024)
	e := echo.New()

	// Test for a valid event
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(`{"n":"load","l":"http://example.com/","r":"https://google.com","lt":100,"fb":200,"sw":1920,"sh":1080,"pr":1.5,"pd":24,"tz":"Europe/Paris","utm_source":"testSource","utm_medium":"testMedium","utm_campaign":"testCampaign","utm_content":"testContent","utm_term":"testTerm"}`)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/42.0.2311.135 Safari/537.36 Edge/12.246")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,fr;q=0.8")

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := trackers.recordPicolyticsEvent(&mockEchoContext{Context: c, request: req})
	assert.NoError(t, err)

	wantEvent := PicolyticsEvent{
		ClientIpDONOTSTORE: "127.0.0.1",
		UaDONOTSTORE:       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/42.0.2311.135 Safari/537.36 Edge/12.246",
		Name:               "load",
		Location:           "http://example.com/",
		Referrer:           "https://google.com",
		Lang:               "en-US,en;q=0.9,fr;q=0.8",
		LoadTime:           100,
		TTFB:               200,
		ScreenW:            1920,
		ScreenH:            1080,
		PixelRatio:         1.5,
		PixelDepth:         24,
		Timezone:           "Europe/Paris",
		UtmSource:          "testSource",
		UtmMedium:          "testMedium",
		UtmCampaign:        "testCampaign",
		UtmContent:         "testContent",
		UtmTerm:            "testTerm",
	}
	select {
	case gotEvent := <-eventSaver.events:
		gotEvent.Created = time.Time{} // don't check Created field
		if !reflect.DeepEqual(gotEvent, wantEvent) {
			t.Errorf("recordPicolyticsEvent() = %v, want %v", gotEvent, wantEvent)
		}
	case <-time.After(time.Second * 1): // Set a timeout to avoid blocking indefinitely
		t.Error("asyncSaveEvent was not called within the expected time")
	}
}

func TestRecordPicolyticsEvent_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{
			name:  "Invalid JSON",
			input: []byte(`{invalid json}`),
		},
		{
			name:  "Invalid JSON field",
			input: []byte(`{"n":"load","l":"http://example.com/","r":"https://google.com","lt":"A00"}`),
		},
		{
			name:  "empty body",
			input: []byte{},
		},
		{
			name:  "too-large body",
			input: []byte(`{"n": "nnnnnnnnnnnnnnnnnnnnnnnnn nnnnnnnnnnnnnnnnnnnnnnnnnnnnnnn nnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnn nnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnn","l":"http://example.com/"}`),
		},
	}
	eventSaver := TestEventSaver{
		events: make(chan PicolyticsEvent, 1),
	}
	trackers := NewTrackers(eventSaver, 128)
	e := echo.New()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(tt.input))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			c := e.NewContext(req, httptest.NewRecorder())

			err := trackers.recordPicolyticsEvent(&mockEchoContext{Context: c, request: req})
			assert.Error(t, err, "Expected an error for invalid input")
		})
	}
}
