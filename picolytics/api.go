package picolytics

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/http/pprof"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	slogecho "github.com/nmcclain/slog-echo"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"
)

type EchoAPI struct {
	E                 *echo.Echo
	staticFS          http.FileSystem
	staticCacheMaxAge int
	o11y              *PicolyticsO11y
}

func NewEchoAPI(config *Config, o11y *PicolyticsO11y) (EchoAPI, error) {
	api := EchoAPI{
		staticCacheMaxAge: config.StaticCacheMaxAge,
		o11y:              o11y,
	}
	api.E = echo.New()
	api.E.HidePort = true
	api.E.HideBanner = true
	api.E.Use(middleware.Recover())
	slogCfg := slogecho.Config{
		ClientErrorLevel: slog.LevelWarn,
		ServerErrorLevel: slog.LevelError,
		WithRequestID:    true,
		WithIP:           false,
		WithEndTime:      false,
	}
	if config.Debug {
		slogCfg.DefaultLevel = slog.LevelDebug
	} else {
		slogCfg.DefaultLevel = slog.LevelInfo
	}
	api.E.Use(slogecho.NewWithConfig(o11y.Logger, slogCfg))

	// see: https://echo.labstack.com/docs/middleware/rate-limiter
	// The default in-memory implementation is focused on correctness and may not be the best option for a high number of concurrent requests or a large number of different identifiers (>16k).
	if config.RequestRateLimit > 0 { // allow disabling rate limiter
		api.E.Use(middleware.RateLimiterWithConfig(middleware.RateLimiterConfig{
			Store: middleware.NewRateLimiterMemoryStore(rate.Limit(config.RequestRateLimit)),
			DenyHandler: func(context echo.Context, identifier string, err error) error {
				o11y.Metrics.rateLimiterDrops.Inc()
				o11y.Logger.Debug("rate limit exceeded", "identifier", identifier, "error", err)
				return &echo.HTTPError{
					Code:     middleware.ErrRateLimitExceeded.Code,
					Message:  middleware.ErrRateLimitExceeded.Message,
					Internal: err,
				}
			},
		}))
	}
	api.E.Use(middleware.CORS()) // https://echo.labstack.com/docs/middleware/cors#default-configuration

	if err := proxySetup(api.E, config, o11y); err != nil {
		return api, fmt.Errorf("error setting up proxy: %v", err)
	}

	var err error
	var usingEmbeddedStaticFiles UsingEmbeddedStaticFiles
	api.staticFS, usingEmbeddedStaticFiles, err = setupStaticFS(config.StaticFiles, config.StaticDir)
	if err != nil {
		return api, fmt.Errorf("error setting up static file system: %v", err)
	}
	if usingEmbeddedStaticFiles {
		o11y.Logger.Debug("Using embedded static files")
	} else {
		o11y.Logger.Debug(fmt.Sprintf("Using static files from local filesystem %q", config.StaticDir))
	}

	api.E.GET("/*", api.HandleStatic)

	return api, nil
}

func (api EchoAPI) Start(address string, autotls bool) error {
	if autotls {
		api.o11y.Logger.Debug("running with autotls")
		return api.E.StartAutoTLS(address)
	}
	return api.E.Start(address)
}

func (api EchoAPI) Shutdown(ctx context.Context) error {
	return api.E.Shutdown(ctx)
}

type UsingEmbeddedStaticFiles bool

func setupStaticFS(staticFiles fs.FS, staticDir string) (http.FileSystem, UsingEmbeddedStaticFiles, error) {
	if _, err := os.Stat(staticDir); err == nil {
		return http.Dir(staticDir), false, nil
	}
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return nil, true, fmt.Errorf("error accessing embedded static files: %v", err)
	}
	return http.FS(staticFS), true, nil
}

func proxySetup(e *echo.Echo, config *Config, o11y *PicolyticsO11y) error {
	// proxy trust and ip extraction setup
	trustOptions := []echo.TrustOption{}
	if len(config.TrustedProxies) > 0 {
		trustOptions = append(trustOptions, echo.TrustLoopback(false))   // e.g. ipv4 start with 127.
		trustOptions = append(trustOptions, echo.TrustLinkLocal(false))  // e.g. ipv4 start with 169.254
		trustOptions = append(trustOptions, echo.TrustPrivateNet(false)) // e.g. ipv4 start with 10. or 192.168
		for _, ipRange := range config.TrustedProxies {
			_, ipNet, err := net.ParseCIDR(ipRange)
			if err != nil {
				return fmt.Errorf("error parsing trustedProxies CIDR %q: %v", ipRange, err)
			}
			trustOptions = append(trustOptions, echo.TrustIPRange(ipNet))
		}
	}

	switch config.IPExtractor { // See: https://echo.labstack.com/docs/ip-address
	case "xff":
		// Never forget to configure the outermost proxy (i.e.; at the edge of your infrastructure) not to pass through incoming headers. Otherwise there is a chance of fraud, as it is what clients can control.
		e.IPExtractor = echo.ExtractIPFromXFFHeader(trustOptions...)
	case "realip":
		// Never forget to configure the outermost proxy (i.e.; at the edge of your infrastructure) not to pass through incoming headers. Otherwise there is a chance of fraud, as it is what clients can control.
		e.IPExtractor = echo.ExtractIPFromRealIPHeader(trustOptions...)
	case "direct": // config default is direct IP extraction, which is safe but will not work behind a proxy
		// Any HTTP header is untrustable because the clients have full control what headers to be set.
		e.IPExtractor = echo.ExtractIPDirect()
	default:
		return fmt.Errorf("unknown IP extractor: %s", config.IPExtractor)
	}
	m := fmt.Sprintf("Proxy IP extractor: %s", config.IPExtractor)
	if config.IPExtractor != "direct" {
		if len(config.TrustedProxies) > 0 {
			m += fmt.Sprintf(". Trusted proxy IP ranges: %v.", config.TrustedProxies)
		} else {
			m += ". No trusted proxy IP ranges specified, using defaults: ipv4 start with 127, 169.254, 10, and 192.168."
		}
	}
	o11y.Logger.Info(m)
	return nil
}

func (e *EchoAPI) HandleStatic(c echo.Context) error {
	file, err := getFile(e.staticFS, c.Param("*"))
	if err != nil {
		return c.NoContent(http.StatusNotFound)
	}
	defer file.Close()

	e.setCacheControlHeader(c, e.staticCacheMaxAge)
	return c.Stream(http.StatusOK, "application/javascript", file)
}

func getFile(staticFiles http.FileSystem, filePath string) (fs.File, error) {
	file, err := staticFiles.Open(filePath)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil || info.IsDir() {
		file.Close()
		return nil, errors.New("file not found or is a directory")
	}
	return file, nil
}

func (e *EchoAPI) setCacheControlHeader(c echo.Context, maxAge int) {
	c.Response().Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", maxAge))
}

func NewAdminAPI(debug bool) *echo.Echo {
	admin := echo.New()
	admin.HidePort = true
	admin.HideBanner = true
	admin.Use(middleware.Recover())
	admin.GET("/healthz", func(c echo.Context) error { return c.String(http.StatusOK, "OK") })
	admin.GET("/ready", func(c echo.Context) error { return c.String(http.StatusOK, "OK") }) // FUTURE: check db connection?
	admin.GET("/metrics", echo.WrapHandler(promhttp.Handler()))

	if debug {
		admin.GET("/debug/pprof/", echo.WrapHandler(http.HandlerFunc(pprof.Index)))
		admin.GET("/debug/pprof/heap", echo.WrapHandler(http.HandlerFunc(pprof.Handler("heap").ServeHTTP)))
		admin.GET("/debug/pprof/goroutine", echo.WrapHandler(http.HandlerFunc(pprof.Handler("goroutine").ServeHTTP)))
		admin.GET("/debug/pprof/block", echo.WrapHandler(http.HandlerFunc(pprof.Handler("block").ServeHTTP)))
		admin.GET("/debug/pprof/threadcreate", echo.WrapHandler(http.HandlerFunc(pprof.Handler("threadcreate").ServeHTTP)))
		admin.GET("/debug/pprof/cmdline", echo.WrapHandler(http.HandlerFunc(pprof.Cmdline)))
		admin.GET("/debug/pprof/profile", echo.WrapHandler(http.HandlerFunc(pprof.Profile)))
		admin.GET("/debug/pprof/symbol", echo.WrapHandler(http.HandlerFunc(pprof.Symbol)))
		admin.GET("/debug/pprof/trace", echo.WrapHandler(http.HandlerFunc(pprof.Trace)))
	}
	return admin
}

func (p *Picolytics) runAdmin() {
	if len(p.config.AdminListen) > 0 {
		p.O11y.Logger.Info(fmt.Sprintf("Admin listening on %s", p.config.AdminListen))
		defer p.admin.Shutdown(context.Background())
		if err := p.admin.Start(p.config.AdminListen); err != nil && err != http.ErrServerClosed {
			p.O11y.Logger.Error(fmt.Sprintf("Error listening on %s: %v", p.config.AdminListen, err))
			os.Exit(1)
		}
	}
}
