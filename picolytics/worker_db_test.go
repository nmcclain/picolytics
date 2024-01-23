package picolytics

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nmcclain/picolytics/picolytics/db"
	"github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"
)

func TestUpsertDomains(t *testing.T) {
	tests := []struct {
		name    string
		events  []PicolyticsEvent
		want    EventDomains
		wantErr bool
		getMock func() pgxmock.PgxPoolIface
	}{
		{
			name: "single domain",
			events: []PicolyticsEvent{
				{Domain: "example.com"},
			},
			getMock: func() pgxmock.PgxPoolIface {
				mock, err := pgxmock.NewPool()
				if err != nil {
					t.Fatal(err)
				}

				mock.ExpectQuery("INSERT INTO domains").
					WithArgs("example.com").
					WillReturnRows(mock.NewRows([]string{"domain_id"}).AddRow(int32(1)))

				return mock
			},
			want: EventDomains{
				"example.com": 1,
			},
		},
		{
			name: "multiple domains",
			events: []PicolyticsEvent{
				{Domain: "google.com"},
				{Domain: "example.com"},
			},
			getMock: func() pgxmock.PgxPoolIface {
				mock, err := pgxmock.NewPool()
				if err != nil {
					t.Fatal(err)
				}
				mock.ExpectQuery("INSERT INTO domains").
					WithArgs(pgxmock.AnyArg()). // order is not guaranteed here
					WillReturnRows(mock.NewRows([]string{"domain_id"}).AddRow(int32(1)))
				mock.ExpectQuery("INSERT INTO domains").
					WithArgs(pgxmock.AnyArg()).
					WillReturnRows(mock.NewRows([]string{"domain_id"}).AddRow(int32(1)))

				return mock
			},
			want: EventDomains{
				"example.com": 1,
				"google.com":  1,
			},
		},
		{
			name: "db failure",
			events: []PicolyticsEvent{
				{Domain: "example.com"},
			},
			getMock: func() pgxmock.PgxPoolIface {
				mock, err := pgxmock.NewPool()
				if err != nil {
					t.Fatal(err)
				}
				mock.ExpectQuery("INSERT INTO domains").
					WithArgs("example.com").
					WillReturnError(fmt.Errorf("Test error"))
				return mock
			},
			want:    EventDomains{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := tt.getMock()
			defer mock.Close()
			client := db.New(mock)
			got, err := upsertDomains(context.Background(), client, tt.events)
			if (err != nil) != tt.wantErr {
				t.Errorf("upsertDomains() error = %v, wantErr %v", err, tt.wantErr)
			}
			if len(got) != len(tt.want) {
				log.Printf("%T %T", got, tt.want)
				t.Errorf("upsertDomains() got = %+v, want %+v", got, tt.want)
			}
		})
	}
}
func TestUpsertSessions(t *testing.T) {
	visitorID := "1234567890"
	sessionID := 123456
	domainID := 1234
	baseEvents := []PicolyticsEvent{
		{
			Name:               "load",
			Location:           "https://example.com/hello",
			Domain:             "example.com",
			Path:               "/hello",
			VisitorID:          visitorID,
			Referrer:           "https://google.com/",
			LoadTime:           100,
			TTFB:               50,
			ScreenW:            1920,
			ScreenH:            1080,
			PixelRatio:         1.0,
			PixelDepth:         24,
			Timezone:           "America/New_York",
			UtmSource:          "testSource",
			UtmMedium:          "testMedium",
			UtmCampaign:        "testCampaign",
			UtmContent:         "testContent",
			UtmTerm:            "testTerm",
			Lang:               "en-US",
			Created:            time.Now(),
			ClientIpDONOTSTORE: "8.8.8.8",
			UaDONOTSTORE:       "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko)",
			Browser:            "Safari",
			BrowserVersion:     "1.0",
			Os:                 "macos",
			OsVersion:          "10_15_7",
			Platform:           "computer",
			DeviceType:         "computer",
			Longitude:          10.0,
			Latitude:           11.1,
			Country:            "World",
			Subdivision:        "NY",
			City:               "Metropolis",
			Bot:                false,
		}}
	tests := []struct {
		name    string
		events  []PicolyticsEvent
		want    EventSessions
		wantErr bool
		domains EventDomains
		getMock func() pgxmock.PgxPoolIface
	}{
		{
			name:   "existing session",
			events: baseEvents,
			getMock: func() pgxmock.PgxPoolIface {
				mock, err := pgxmock.NewPool()
				if err != nil {
					t.Fatal(err)
				}

				mock.ExpectBeginTx(pgx.TxOptions{})
				mock.ExpectQuery("SELECT id FROM sessions WHERE visitor_id").
					WithArgs(pgtype.Text{
						String: visitorID, Valid: true},
						pgtype.Interval{Microseconds: 60000000, Days: 0, Months: 0, Valid: true},
					).
					WillReturnRows(mock.NewRows([]string{"session_id"}).AddRow(int64(sessionID)))

				mock.ExpectExec("UPDATE sessions SET").
					WithArgs("/hello", int64(sessionID), "load").
					WillReturnResult(pgxmock.NewResult("INSERT", 1))
				mock.ExpectCommit()

				return mock
			},
			domains: EventDomains{
				"example.com": int32(domainID),
			},
			want: EventSessions{
				visitorID: int64(sessionID),
			},
		},
		{
			name:   "new session",
			events: baseEvents,
			getMock: func() pgxmock.PgxPoolIface {
				mock, err := pgxmock.NewPool()
				if err != nil {
					t.Fatal(err)
				}

				mock.ExpectBeginTx(pgx.TxOptions{})
				mock.ExpectQuery("SELECT id FROM sessions WHERE visitor_id").
					WithArgs(pgtype.Text{
						String: visitorID, Valid: true},
						pgtype.Interval{Microseconds: 60000000, Days: 0, Months: 0, Valid: true},
					).
					WillReturnError(pgx.ErrNoRows)

				mock.ExpectQuery("INSERT INTO sessions").
					WithArgs(
						int32(domainID),
						"/hello",
						newPGText(visitorID),
						"/hello",
						newPGText("World"),
						newPGFloat8(11.1),
						newPGFloat8(10.0),
						newPGText("NY"),
						newPGText("Metropolis"),
						newPGText("Safari"),
						newPGText("1.0"),
						newPGText("macos"),
						newPGText("10_15_7"),
						newPGText("computer"),
						newPGText("computer"),
						false,
						newPGInt4(1920),
						newPGInt4(1080),
						newPGText("America/New_York"),
						newPGFloat8(1.0),
						newPGInt4(24),
						newPGText("testSource"),
						newPGText("testMedium"),
						newPGText("testCampaign"),
						newPGText("testContent"),
						newPGText("testTerm"),
					).
					WillReturnRows(mock.NewRows([]string{"session_id"}).AddRow(int64(sessionID)))
				mock.ExpectCommit()

				return mock
			},
			domains: EventDomains{
				"example.com": int32(domainID),
			},
			want: EventSessions{
				visitorID: int64(sessionID),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := tt.getMock()
			defer mock.Close()
			client := db.New(mock)

			sessionTimeoutMin := 1
			got, err := upsertSessions(context.Background(), mock, client, tt.events, tt.domains, sessionTimeoutMin)
			assert.NoError(t, err)
			if !reflect.DeepEqual(got, &tt.want) {
				t.Errorf("upsertSessions() got = %+v, want %+v", got, &tt.want)
			}
		})
	}
}

func TestCreateEvents(t *testing.T) {
	visitorID := "1234567890"
	sessionID := 123456
	domainID := 1234
	tests := []struct {
		name     string
		events   []PicolyticsEvent
		sessions EventSessions
		domains  EventDomains
		getMock  func() pgxmock.PgxPoolIface
	}{
		{
			name: "successful event",
			events: []PicolyticsEvent{
				{
					Name:      "load",
					Domain:    "example.com",
					VisitorID: visitorID,
					Path:      "/hello",
					Referrer:  "https://google.com/",
					LoadTime:  100,
					TTFB:      50,
				},
			},
			sessions: EventSessions{
				visitorID: int64(sessionID),
			},
			domains: EventDomains{
				"example.com": int32(domainID),
			},
			getMock: func() pgxmock.PgxPoolIface {
				mock, err := pgxmock.NewPool()
				if err != nil {
					t.Fatal(err)
				}
				mock.ExpectCopyFrom(pgx.Identifier{"events"}, []string{"domain_id", "session_id", "visitor_id",
					"name", "path", "referrer", "load_time", "ttfb"}).WillReturnResult(1)
				return mock
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := tt.getMock()
			defer mock.Close()
			client := db.New(mock)
			metrics := setupMetrics(1, "", "", "")
			err := createEvents(context.Background(), client, tt.events, tt.domains, tt.sessions, metrics, time.Now())
			assert.NoError(t, err)
		})
	}
}
