package picolytics

import (
	"fmt"
	"net"
	"strings"

	"github.com/avct/uasurfer"
	"github.com/oschwald/maxminddb-golang"
)

func enrichEvent(event *PicolyticsEvent, geo *maxminddb.Reader) error {
	if event == nil || geo == nil {
		return fmt.Errorf("nil event or geo")
	}
	g, err := lookupIP(event.ClientIpDONOTSTORE, geo)
	if err != nil {
		return fmt.Errorf("error looking up ip: %v", err)
	}
	event.Longitude = g.Location.Longitude
	event.Latitude = g.Location.Latitude
	event.Country = g.Country.ISOCode
	if len(g.Subdivisions) > 0 {
		event.Subdivision = g.Subdivisions[0].Names["en"]
	}
	if len(g.City.Names) > 0 {
		event.City = g.City.Names["en"]
	}
	if len(event.UaDONOTSTORE) > 1 {
		updateUserAgentDetails(event)
		event.Bot = isBot(event)
	}
	return nil
}

var botAgents = []string{"bot", "crawler", "spider", "headless",
	"yandex", "google-extended", "feedfetcher-google", "mediapartners-google", "apis-google", "google-inspectiontool",
	"googleother", "google-adwords-instant", "slurp", "wget", "Python-urllib", "python-requests", "aiohttp", "curl",
	"httpx", "libwww-perl", "httpunit", "nutch", "go-http-client", "vegeta",
}

func isBot(event *PicolyticsEvent) bool {
	ua := strings.ToLower(event.UaDONOTSTORE)
	for _, botUA := range botAgents {
		if strings.Contains(ua, botUA) {
			return true
		}
	}
	return false
}

type geoQuery struct {
	Country struct {
		ISOCode string            `maxminddb:"iso_code"`
		Names   map[string]string `maxminddb:"names"`
	} `maxminddb:"country"`
	Location struct {
		Latitude  float64 `maxminddb:"latitude"`
		Longitude float64 `maxminddb:"longitude"`
	} `maxminddb:"location"`
	Subdivisions []struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"subdivisions"`
	City struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`
}

func lookupIP(ip string, geo *maxminddb.Reader) (geoQuery, error) {
	var result geoQuery
	ipAddr := net.ParseIP(ip)
	if ipAddr == nil {
		return result, fmt.Errorf("invalid ip address: %s", ip)
	}
	if err := geo.Lookup(ipAddr, &result); err != nil {
		return result, err
	}
	return result, nil
}

func updateUserAgentDetails(event *PicolyticsEvent) {
	a := uasurfer.Parse(event.UaDONOTSTORE)
	event.Browser = a.Browser.Name.StringTrimPrefix()
	event.BrowserVersion = fmt.Sprintf("%v.%v", a.Browser.Version.Major, a.Browser.Version.Minor)
	event.Os = a.OS.Name.StringTrimPrefix()
	event.OsVersion = fmt.Sprintf("%v.%v", a.OS.Version.Major, a.OS.Version.Minor)
	event.Platform = a.OS.Platform.StringTrimPrefix()
	event.DeviceType = a.DeviceType.StringTrimPrefix()
}
