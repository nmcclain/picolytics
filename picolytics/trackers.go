package picolytics

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

type PicolyticsEvent struct {
	// populated by tracker javascript
	Name        string  `json:"n"`
	Location    string  `json:"l"`
	Referrer    string  `json:"r"`
	LoadTime    int32   `json:"lt"`
	TTFB        int32   `json:"fb"`
	ScreenW     int32   `json:"sw"`
	ScreenH     int32   `json:"sh"`
	PixelRatio  float64 `json:"pr"`
	PixelDepth  int32   `json:"pd"`
	Timezone    string  `json:"tz"`
	UtmSource   string  `json:"utm_source"`
	UtmMedium   string  `json:"utm_medium"`
	UtmCampaign string  `json:"utm_campaign"`
	UtmContent  string  `json:"utm_content"`
	UtmTerm     string  `json:"utm_term"`

	// populated by tracker handler
	Lang    string
	Created time.Time

	// populated by tracker handler - DO NOT store in DB
	ClientIpDONOTSTORE string
	UaDONOTSTORE       string

	// populated by parseEvent
	Domain, Path, VisitorID string

	// populated by updateUserAgentDetails
	Browser, BrowserVersion, Os, OsVersion, Platform, DeviceType string

	// populated by enrichEvent
	Longitude, Latitude        float64
	Country, Subdivision, City string
	Bot                        bool
}

type EventSaver interface {
	SaveEvent(event PicolyticsEvent)
}

type Trackers struct {
	eventSaver  EventSaver
	bodyMaxSize int64
}

func NewTrackers(eventSaver EventSaver, bodyMaxSize int64) *Trackers {
	return &Trackers{
		eventSaver:  eventSaver,
		bodyMaxSize: bodyMaxSize,
	}
}

func (t *Trackers) recordPicolyticsEvent(c echo.Context) error {
	event := PicolyticsEvent{Created: time.Now()}
	if err := unmarshallBody(c, &event, t.bodyMaxSize); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid event data")
	}

	event.ClientIpDONOTSTORE = c.RealIP()
	event.UaDONOTSTORE = c.Request().UserAgent()
	event.Lang = c.Request().Header.Get("Accept-Language")
	go t.eventSaver.SaveEvent(event)

	return c.String(http.StatusAccepted, "ok")
}

func unmarshallBody(c echo.Context, e interface{}, maxBodySize int64) error {
	r := c.Request()
	r.Body = http.MaxBytesReader(c.Response().Writer, r.Body, maxBodySize)
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
		if err == io.EOF {
			return echo.NewHTTPError(http.StatusRequestEntityTooLarge, "Request body empty")
		}
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid event data")
	}
	return nil
}
