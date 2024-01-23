package picolytics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pashagolub/pgxmock/v3"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

type TestLogHandler struct {
	messages *[]string
	lock     *sync.Mutex
	whoami   string
}

func NewTestLogHandler() TestLogHandler {
	return TestLogHandler{
		messages: &[]string{},
		lock:     &sync.Mutex{},
	}
}

func (h TestLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return true
}
func (h TestLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}
func (h TestLogHandler) WithGroup(name string) slog.Handler {
	return h
}
func (h TestLogHandler) Handle(ctx context.Context, r slog.Record) error {
	h.lock.Lock()
	defer h.lock.Unlock()
	attrs := []string{}
	r.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, fmt.Sprintf("%s=%v", a.Key, a.Value))
		return true
	})
	*h.messages = append(*h.messages, fmt.Sprintf("%s %s %s", r.Level, r.Message, strings.Join(attrs, " ")))
	return nil
}
func (h TestLogHandler) getMessages() []string {
	h.lock.Lock()
	defer h.lock.Unlock()
	return *h.messages
}

type TestLogHandlerMessage struct {
	Level         string
	Msg           string
	KeysAndValues []interface{}
}

func TestPicolyticsIntegration(t *testing.T) {
	SetConfigDefaults()
	baseConfig := Config{}

	if err := viper.Unmarshal(&baseConfig); err != nil {
		t.Fatalf("Unable to decode config, %v", err)
	}

	baseConfig.PgHost = "notused_mustbeset"
	baseConfig.PgPort = 5432
	baseConfig.PgDatabase = "notused_mustbeset"
	baseConfig.PgUser = "notused_mustbeset"
	baseConfig.PgPassword = "notused_mustbeset"
	baseConfig.GeoIPFile = "../etc/geoip-city-test.mmdb"
	baseConfig.StaticDir = "../cmd/picolytics/static"
	baseConfig.BatchMaxMsec = 1
	baseConfig.BatchMaxSize = 1
	baseConfig.QueueSize = 1
	baseConfig.ListenAddr = "localhost:8080"

	sessionID := int64(123456)
	domainID := 1234
	dbSalt := "2ed75c40-5212-4a5f-8748-0a0f4513febe" // this is a random UUID for consistent testing
	tests := []struct {
		name         string
		getReq       func() *http.Request
		getMock      func() pgxmock.PgxPoolIface
		getConfig    func() *Config
		expectedCode int
		check        func(res *http.Response, messages []string, metrics *Metrics, config *Config) error
		config       Config
	}{
		{
			name: "root request",
			getReq: func() *http.Request {
				req, err := http.NewRequest("GET", "http://localhost:8080/", nil)
				if err != nil {
					t.Fatal(err)
				}
				return req
			},
			getConfig: func() *Config {
				return &baseConfig
			},
			getMock: func() pgxmock.PgxPoolIface {
				mock, err := pgxmock.NewPool()
				if err != nil {
					t.Fatal(err)
				}
				return mock
			},
			expectedCode: http.StatusOK,
			check: func(res *http.Response, messages []string, metrics *Metrics, config *Config) error {
				if res.ContentLength != 2 {
					return fmt.Errorf("expected content length 2, got %d", res.ContentLength)
				}
				if messages[0] != "INFO Picolytics starting up... commit= branch= version=" {
					return fmt.Errorf("expected startup message, got %s", messages[0])
				}
				if messages[1] != "INFO Proxy IP extractor: direct " {
					return fmt.Errorf("expected 'INFO Proxy IP extractor: direct' , got %s", messages[1])
				}
				if !strings.HasPrefix(messages[2], "DEBUG Using static files from local filesystem ") {
					return fmt.Errorf("expected 'DEBUG Using static files from local filesystem'..., got %s", messages[2])
				}
				if messages[3] != fmt.Sprintf("INFO Listening on %s ", config.ListenAddr) {
					return fmt.Errorf("expected 'INFO Listening on %s' , got %s", config.ListenAddr, messages[3])
				}
				re := regexp.MustCompile(`INFO Incoming request .* status=200`)
				if !re.MatchString(messages[4]) {
					return fmt.Errorf("expected 'INFO Incoming request .* status=200' , got %s", messages[4])
				}
				return nil
			},
		},
		{
			name: "root request with redirect",
			getReq: func() *http.Request {
				req, err := http.NewRequest("GET", "http://localhost:8080/", nil)
				if err != nil {
					t.Fatal(err)
				}
				return req
			},
			getConfig: func() *Config {
				c := baseConfig
				c.RootRedirect = "https://example.com"
				return &c
			},
			getMock: func() pgxmock.PgxPoolIface {
				mock, err := pgxmock.NewPool()
				if err != nil {
					t.Fatal(err)
				}
				return mock
			},
			expectedCode: http.StatusFound,
			check: func(res *http.Response, messages []string, metrics *Metrics, config *Config) error {
				if res.ContentLength != 0 {
					return fmt.Errorf("expected content length 0, got %d", res.ContentLength)
				}

				return nil
			},
		},
		{
			name: "tracker js request",
			getReq: func() *http.Request {
				req, err := http.NewRequest("GET", "http://localhost:8080/pico.js", nil)
				if err != nil {
					t.Fatal(err)
				}
				return req
			},
			getConfig: func() *Config {
				return &baseConfig
			},
			getMock: func() pgxmock.PgxPoolIface {
				mock, err := pgxmock.NewPool()
				if err != nil {
					t.Fatal(err)
				}
				return mock
			},
			expectedCode: http.StatusOK,
			check: func(res *http.Response, messages []string, metrics *Metrics, config *Config) error {
				body, err := io.ReadAll(res.Body)
				if err != nil {
					return fmt.Errorf("error reading response body: %v", err)
				}
				if !strings.Contains(string(body), "window.document.currentScript.src.split") {
					return fmt.Errorf("body missing script")
				}
				if res.ContentLength < 800 {
					return fmt.Errorf("expected content length > 800, got %d", res.ContentLength)
				}
				if res.ContentLength > 1000 {
					return fmt.Errorf("expected content length < 1000, got %d", res.ContentLength)
				}
				return nil
			},
		},
		{
			name: "admin /healthz request",
			getReq: func() *http.Request {
				req, err := http.NewRequest("GET", "http://localhost:8081/healthz", nil)
				if err != nil {
					t.Fatal(err)
				}
				return req
			},
			getConfig: func() *Config {
				c := baseConfig
				c.AdminListen = "localhost:8081"
				return &c
			},
			getMock: func() pgxmock.PgxPoolIface {
				mock, err := pgxmock.NewPool()
				if err != nil {
					t.Fatal(err)
				}
				return mock
			},
			expectedCode: http.StatusOK,
			check: func(res *http.Response, messages []string, metrics *Metrics, config *Config) error {
				if res.ContentLength != 2 {
					return fmt.Errorf("expected content length 2, got %d", res.ContentLength)
				}
				if messages[0] != "INFO Picolytics starting up... commit= branch= version=" {
					return fmt.Errorf("expected startup message, got %s", messages[0])
				}
				if messages[1] != "INFO Proxy IP extractor: direct " {
					return fmt.Errorf("expected 'INFO Proxy IP extractor: direct' , got %s", messages[1])
				}
				if !strings.HasPrefix(messages[2], "DEBUG Using static files from local filesystem ") {
					return fmt.Errorf("expected 'DEBUG Using static files from local filesystem'..., got %s", messages[2])
				}
				if messages[3] != fmt.Sprintf("INFO Listening on %s ", config.ListenAddr) {
					return fmt.Errorf("expected 'INFO Listening on %s' , got %s", config.ListenAddr, messages[3])
				}
				if messages[4] != fmt.Sprintf("INFO Admin listening on %s ", config.AdminListen) {
					return fmt.Errorf("expected 'INFO Admin listening on localhost:8081' , got %s", messages[4])
				}
				if messages[5] != "INFO Picolytics shutdown via signal " {
					return fmt.Errorf("expected 'INFO Picolytics shutdown via signal' , got %s", messages[5])
				}
				return nil
			},
		},
		{
			name: "empty js tracking event",
			getReq: func() *http.Request {
				payload, err := json.Marshal(map[string]interface{}{})
				if err != nil {
					t.Fatal(err)
				}
				req, err := http.NewRequest("POST", "http://localhost:8080/p", bytes.NewReader(payload))
				if err != nil {
					t.Fatal(err)
				}
				req.Header.Set("Host", "localhost:8080")
				req.Header.Set("Referer", "https://www.google.com/")
				req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "+
					"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.114 Safari/537.36")
				req.Header.Set("Accept-Language", "en-US,en;q=0.9")
				return req
			},
			getConfig: func() *Config {
				return &baseConfig
			},
			getMock: func() pgxmock.PgxPoolIface {
				mock, err := pgxmock.NewPool()
				if err != nil {
					t.Fatal(err)
				}
				return mock
			},
			expectedCode: http.StatusAccepted,
			check: func(res *http.Response, messages []string, metrics *Metrics, config *Config) error {
				if res.ContentLength != 2 {
					return fmt.Errorf("expected content length 2, got %d", res.ContentLength)
				}
				re := regexp.MustCompile(`INFO Incoming request .* status=202`)
				if !re.MatchString(messages[4]) {
					return fmt.Errorf("expected 'INFO Incoming request .* status=202' , got %s", messages[4])
				}
				if messages[5] != "INFO error parsing event error=invalid event name: " {
					return fmt.Errorf("expected 'INFO error parsing event' , got %s", messages[5])
				}
				return nil
			},
		},
		{
			name: "valid js tracking event",
			getReq: func() *http.Request {
				payload := []byte(`{"n":"load","l":"http://example.com/","r":"https://google.com","lt":100,"fb":200,"sw":1920,"sh":1080,"pr":1.5,"pd":24,"tz":"Europe/Paris","utm_source":"testSource","utm_medium":"testMedium","utm_campaign":"testCampaign","utm_content":"testContent","utm_term":"testTerm"}`)
				req, err := http.NewRequest("POST", "http://localhost:8080/p", bytes.NewReader(payload))
				if err != nil {
					t.Fatal(err)
				}
				req.Header.Set("Host", "localhost:8080")
				req.Header.Set("Referer", "https://www.google.com/")
				req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "+
					"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.114 Safari/537.36")
				req.Header.Set("Accept-Language", "en-US,en;q=0.9")
				return req
			},
			getConfig: func() *Config {
				c := baseConfig
				c.BatchMaxMsec = 1
				c.BatchMaxSize = 1
				c.QueueSize = 1
				return &c
			},
			getMock: func() pgxmock.PgxPoolIface {
				mock, err := pgxmock.NewPool()
				if err != nil {
					t.Fatal(err)
				}
				mock.ExpectBeginTx(pgx.TxOptions{})
				mock.ExpectExec("DO .. BEGIN IF NOT EXISTS").
					WillReturnResult(pgxmock.NewResult("INSERT", 1))
				mock.ExpectQuery("SELECT salt, created_at FROM salt LIMIT 1").
					WillReturnRows(mock.NewRows([]string{"salt", "created_at"}).AddRow(dbSalt, time.Now()))
				mock.ExpectCommit()
				mock.ExpectQuery(`INSERT INTO domains .domain_name. VALUES`).
					WithArgs("example.com").
					WillReturnRows(mock.NewRows([]string{"domain_id"}).AddRow(int32(domainID)))
				mock.ExpectBeginTx(pgx.TxOptions{})
				mock.ExpectQuery(`SELECT id FROM sessions WHERE visitor_id`).
					WithArgs(
						pgxmock.AnyArg(), // this arg is visitor_id, which varies based on time, salt, client IP, and UA
						pgtype.Interval{Microseconds: 1800000000, Days: 0, Months: 0, Valid: true}).
					WillReturnRows(mock.NewRows([]string{"session_id"}).AddRow(sessionID))
				mock.ExpectExec("UPDATE sessions SET bounce").
					WithArgs("/", sessionID, "load").
					WillReturnResult(pgxmock.NewResult("UPDATE", 1))
				mock.ExpectCommit()
				mock.ExpectCopyFrom(pgx.Identifier{"events"}, []string{"domain_id", "session_id", "visitor_id",
					"name", "path", "referrer", "load_time", "ttfb"}).WillReturnResult(1)
				return mock
			},
			expectedCode: http.StatusAccepted,
			check: func(res *http.Response, messages []string, metrics *Metrics, config *Config) error {
				if res.ContentLength != 2 {
					return fmt.Errorf("expected content length 2, got %d", res.ContentLength)
				}
				re := regexp.MustCompile(`INFO Incoming request .* status=202`)
				if !re.MatchString(messages[4]) {
					return fmt.Errorf("expected 'INFO Incoming request .* status=202' , got %s", messages[4])
				}
				m, err := metrics.eventErrors.GetMetricWith(prometheus.Labels{"kind": "salt"})
				if err != nil {
					return fmt.Errorf("expected 'INFO Incoming request .* status=202' , got %s", messages[4])
				}
				if getCounterValue(m) > 0 {
					return fmt.Errorf("expected no salter errors, got %d", int(getCounterValue(m)))
				}
				for _, m := range messages {
					if strings.Contains(m, "WARN Error enriching event") {
						return fmt.Errorf("expected no enrichment errors errors, got %s", m)
					}
					if strings.Contains(m, "error saving queue events to db") {
						return fmt.Errorf("error saving queue events to db, got %s", m)
					}
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var _ slog.Handler = TestLogHandler{}
			logHandler := NewTestLogHandler()
			logHandler.whoami = tt.name
			mock := tt.getMock()
			p, err := New(tt.getConfig(), logHandler, mock)
			if err != nil {
				t.Fatalf("Initialization error: %v", err)
			}

			p.Run()

			// run tests here
			var res *http.Response
			go func() {
				defer func() {
					time.Sleep(10 * time.Millisecond) // wait for worker to finish
					p.Shutdown()
				}()
				req := tt.getReq()
				client := http.Client{
					Timeout: 10 * time.Millisecond,
					// we don't want to actually follow the root redirect
					CheckRedirect: func(req *http.Request, via []*http.Request) error {
						return http.ErrUseLastResponse
					},
				}

				res, err = client.Do(req)
				if err != nil {
					t.Errorf("expected no response error, got %s", err)
					return
				}
				if res == nil {
					t.Error("expected non-nil response")
					return
				}
				assert.Equal(t, tt.expectedCode, res.StatusCode, fmt.Sprintf("expected status code %v", tt.expectedCode))
			}()
			p.HandleShutdown()
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("there were unfulfilled expectations: %s", err)
			}
			assert.Equal(t, nil, tt.check(res, logHandler.getMessages(), p.O11y.Metrics, p.config), "response check failed")
		})
	}
}

func getCounterValue(counter prometheus.Counter) float64 {
	var metric dto.Metric
	if err := counter.Write(&metric); err != nil {
		log.Printf("Error writing metric: %v", err)
		return 0
	}
	return metric.Counter.GetValue()
}
