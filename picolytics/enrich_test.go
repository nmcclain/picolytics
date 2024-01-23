package picolytics

import (
	"reflect"
	"testing"

	"github.com/oschwald/maxminddb-golang"
)

func TestIsBot(t *testing.T) {
	tests := []struct {
		name  string
		event PicolyticsEvent
		want  bool
	}{
		{
			name: "UA marked as Yahoo Bot",
			event: PicolyticsEvent{
				UaDONOTSTORE: "Mozilla/5.0 (compatible; Yahoo! Slurp; http://help.yahoo.com/help/us/ysearch/slurp)",
			},
			want: true,
		},
		{
			name: "UA marked as GoogleBot",
			event: PicolyticsEvent{
				UaDONOTSTORE: "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
			},
			want: true,
		},
		{
			name: "Non-bot Android",
			event: PicolyticsEvent{
				UaDONOTSTORE: "Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Mobile Safari/537.36",
			},
			want: false,
		},
		{
			name: "Non-bot macos",
			event: PicolyticsEvent{
				UaDONOTSTORE: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_2) AppleWebKit/601.3.9 (KHTML, like Gecko) Version/9.0.2 Safari/601.3.9",
			},
			want: false,
		},
		{
			name: "Non-bot Windows",
			event: PicolyticsEvent{
				UaDONOTSTORE: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/42.0.2311.135 Safari/537.36 Edge/12.246",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBot(&tt.event)
			if got != tt.want {
				t.Errorf("isBot() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLookupIP(t *testing.T) {
	// geoip setup
	geoFile := "../etc/geoip-city-test.mmdb"
	geo, err := maxminddb.Open(geoFile)
	if err != nil {
		t.Fatalf("error opening geoip file at %s: %v", geoFile, err)
	}
	tests := []struct {
		name       string
		ip         string
		wantParams func() geoQuery
		wantErr    bool
	}{
		{
			name: "valid IP",
			ip:   "1.0.1.1",
			wantParams: func() geoQuery {
				gq := geoQuery{}
				gq.Country.ISOCode = "CN"
				gq.Location.Latitude = 26.4837
				gq.Location.Longitude = 117.925
				gq.City.Names = map[string]string{"en": "Gaosha"}
				return gq
			},
			wantErr: false,
		},
		{
			name: "invalid IP",
			ip:   "invalid-ip",
			wantParams: func() geoQuery {
				return geoQuery{}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			geoResult, err := lookupIP(tt.ip, geo)
			// unset country names and subdivisions for testing as they vary across mmdb providers
			geoResult.Country.Names = nil
			geoResult.Subdivisions = nil
			if (err != nil) != tt.wantErr {
				t.Errorf("lookupIP() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(geoResult, tt.wantParams()) {
				t.Errorf("parseEvent() geoResult = %+v, want %+v", geoResult, tt.wantParams())
			}
		})
	}
	// should err on closed geo DB
	if err := geo.Close(); err != nil {
		t.Fatalf("error closing geoip file at %s: %v", geoFile, err)
	}
	if _, err := lookupIP("1.0.1.1", geo); err == nil {
		t.Errorf("expected error looking up ip on closed geo DB")
	}
}

func TestUpdateUserAgentDetails(t *testing.T) {
	tests := []struct {
		ua        string
		wantEvent PicolyticsEvent
	}{
		{
			ua: "Mozilla/5.0 (compatible; Yahoo! Slurp; http://help.yahoo.com/help/us/ysearch/slurp)",
			wantEvent: PicolyticsEvent{
				Browser: "YahooBot", BrowserVersion: "0.0", Os: "Bot", OsVersion: "0.0", Platform: "Bot", DeviceType: "Computer",
			},
		},
		{
			ua: "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
			wantEvent: PicolyticsEvent{
				Browser: "GoogleBot", BrowserVersion: "0.0", Os: "Bot", OsVersion: "0.0", Platform: "Bot", DeviceType: "Computer",
			},
		},
		{
			ua: "Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Mobile Safari/537.36",
			wantEvent: PicolyticsEvent{
				Browser: "Chrome", BrowserVersion: "114.0", Os: "Android", OsVersion: "10.0", Platform: "Linux", DeviceType: "Phone",
			},
		},
		{
			ua: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_2) AppleWebKit/601.3.9 (KHTML, like Gecko) Version/9.0.2 Safari/601.3.9",
			wantEvent: PicolyticsEvent{
				Browser: "Safari", BrowserVersion: "9.0", Os: "MacOSX", OsVersion: "10.11", Platform: "Mac", DeviceType: "Computer",
			},
		},
		{
			ua: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/42.0.2311.135 Safari/537.36 Edge/12.246",
			wantEvent: PicolyticsEvent{
				Browser: "IE", BrowserVersion: "12.246", Os: "Windows", OsVersion: "10.0", Platform: "Windows", DeviceType: "Computer",
			},
		},
	}

	for _, tt := range tests {
		e := PicolyticsEvent{UaDONOTSTORE: tt.ua}
		tt.wantEvent.UaDONOTSTORE = tt.ua
		updateUserAgentDetails(&e)
		if !reflect.DeepEqual(e, tt.wantEvent) {
			t.Errorf("updateUserAgentDetails() got=%+v, want=%+v", e, tt.wantEvent)
		}
	}
}

func TestEnrichEvent(t *testing.T) {
	// geoip setup
	geoFile := "../etc/geoip-city-test.mmdb"
	geo, err := maxminddb.Open(geoFile)
	if err != nil {
		t.Fatalf("error opening geoip file at %s: %v", geoFile, err)
	}
	tests := []struct {
		name      string
		event     PicolyticsEvent
		wantEvent PicolyticsEvent
		wantErr   bool
	}{
		{
			name: "typical event",
			event: PicolyticsEvent{
				ClientIpDONOTSTORE: "1.0.1.1",
				UaDONOTSTORE:       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/42.0.2311.135 Safari/537.36 Edge/12.246",
			},
			wantEvent: PicolyticsEvent{
				ClientIpDONOTSTORE: "1.0.1.1",
				UaDONOTSTORE:       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/42.0.2311.135 Safari/537.36 Edge/12.246",
				//Domain: Path: VisitorID:
				Browser:        "IE",
				BrowserVersion: "12.246",
				Os:             "Windows",
				OsVersion:      "10.0",
				Platform:       "Windows",
				DeviceType:     "Computer",
				Longitude:      117.925,
				Latitude:       26.4837,
				Country:        "CN",
				Subdivision:    "Fujian",
				City:           "Gaosha",
				Bot:            false,
			},
			wantErr: false,
		},
		{
			name:      "empty event",
			event:     PicolyticsEvent{},
			wantEvent: PicolyticsEvent{},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := enrichEvent(&tt.event, geo)
			if (err != nil) != tt.wantErr {
				t.Errorf("enrichEvent() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(tt.event, tt.wantEvent) {
				t.Errorf("enrichEvent() got = %+v, want %+v", tt.event, tt.wantEvent)
			}
		})
	}
}
