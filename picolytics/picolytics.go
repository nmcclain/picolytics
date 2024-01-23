package picolytics

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

type Picolytics struct {
	api        EchoAPI
	pool       PgxIface
	config     *Config
	trackers   *Trackers
	pruner     *Pruner
	worker     *Worker
	eventSaver EventSaver
	quit       chan os.Signal
	salter     Salter
	admin      *echo.Echo

	// exported
	O11y *PicolyticsO11y

	// injected values
	gitCommit, gitBranch, appVersion string
}

type PicolyticsO11y struct {
	Metrics *Metrics
	Logger  *slog.Logger
}

func New(config *Config, logHandler slog.Handler, pool PgxIface) (Picolytics, error) {
	p := Picolytics{
		gitCommit:  config.GitCommit,
		gitBranch:  config.GitBranch,
		appVersion: config.AppVersion,
		O11y:       &PicolyticsO11y{},
		config:     config,
		pool:       pool,
	}
	var err error
	if err := validateConfig(p.config); err != nil {
		return p, fmt.Errorf("config error: %v", err)
	}

	p.O11y.Logger, err = setupLogger(p.config.Debug, logHandler)
	if err != nil {
		return p, fmt.Errorf("logger setup error: %v", err)
	}

	p.O11y.Logger.Info("Picolytics starting up...", "commit", p.config.GitCommit, "branch", p.config.GitBranch, "version", p.config.AppVersion)
	if p.config.Debug {
		safeconfig := *p.config
		safeconfig.PgPassword = "********"
		safeconfig.PgConnString = "********"
		p.O11y.Logger.Debug("debugging enabled.", "config", safeconfig)
	}

	p.O11y.Metrics = setupMetrics(float64(p.config.QueueSize), p.config.GitCommit, p.config.GitBranch, p.config.AppVersion)

	// database setup
	if p.pool == nil { // allow for testing without postgres
		p.pool, err = setupDB(p.config, p.O11y)
		if err != nil {
			return p, fmt.Errorf("db setup error: %v", err)
		}
	}

	// salter setup
	p.salter = NewDailySalt(p.pool)

	// worker setup
	p.worker, err = NewWorker(p.config, p.pool, p.O11y)
	if err != nil {
		return p, fmt.Errorf("worker setup error: %v", err)
	}

	// event saver setup
	p.eventSaver = NewAsyncEventSaver(p.worker.events, p.salter, p.config.ValidEventNames, p.O11y)

	// API setup
	p.trackers = NewTrackers(p.eventSaver, p.config.BodyMaxSize)
	p.api, err = NewEchoAPI(p.config, p.O11y)
	if err != nil {
		return p, fmt.Errorf("error setting up API: %v", err)
	}
	p.api.E.POST("/p", p.trackers.recordPicolyticsEvent)
	p.api.E.GET("/robots.txt", func(c echo.Context) error { return c.String(http.StatusOK, "User-agent: *\nDisallow: /\n") })
	p.api.E.GET("/", func(c echo.Context) error {
		if len(p.config.RootRedirect) > 0 {
			return c.Redirect(http.StatusFound, p.config.RootRedirect)
		}
		return c.String(http.StatusOK, "OK")
	})

	// Setup Autotls manager
	var acmeCache *PostgresAutocertCache
	if p.config.AutotlsEnabled {
		p.O11y.Logger.Debug(fmt.Sprintf("Autotls enabled for: %s", p.config.AutotlsHost))
		acmeClient := &acme.Client{}
		if p.config.AutotlsStaging {
			p.O11y.Logger.Info("Autotls using STAGING letsencrypt service for TLS certificate")
			acmeClient.DirectoryURL = "https://acme-staging-v02.api.letsencrypt.org/directory"
		}
		acmeCache = NewPostgresCache(p.pool)
		p.api.E.AutoTLSManager = autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			Cache:      acmeCache,
			Client:     acmeClient,
			HostPolicy: autocert.HostWhitelist(p.config.AutotlsHost),
		}
	}

	p.pruner, err = NewPruner(p.config, p.pool, p.O11y)
	if err != nil {
		return p, fmt.Errorf("pruner setup error: %v", err)
	}

	if len(p.config.AdminListen) > 0 {
		p.admin = NewAdminAPI(config.Debug)
	}

	// exit signal handling
	p.quit = make(chan os.Signal, 1)
	signal.Notify(p.quit, syscall.SIGINT, syscall.SIGTERM)

	return p, nil
}

func (p *Picolytics) Run() {
	go func() {
		if err := p.api.Start(p.config.ListenAddr, p.config.AutotlsEnabled); err != nil && err != http.ErrServerClosed {
			p.O11y.Logger.Error(fmt.Sprintf("Error listening on %s: %v", p.config.ListenAddr, err))
		}
	}()
	p.O11y.Logger.Info(fmt.Sprintf("Listening on %s", p.config.ListenAddr))
	startMetrics(p.O11y.Metrics, p.config.DisableHostMetrics)
	go p.worker.processQueuedEvents()
	go p.pruner.prune()
	go p.runAdmin()
}

func (p *Picolytics) Shutdown() {
	p.quit <- os.Interrupt
}

func (p *Picolytics) HandleShutdown() {
	<-p.quit
	p.O11y.Logger.Info("Picolytics shutdown via signal")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	done := make(chan struct{})
	step := ""
	go func() {
		step = "stopMetrics"
		stopMetrics(p.O11y.Metrics, p.config.DisableHostMetrics)
		step = "api.Shutdown"
		p.api.Shutdown(ctx)
		step = "worker.Shutdown"
		p.worker.Shutdown()
		step = "pool.Close"
		p.pool.Close()
		step = "done"
		close(done)
	}()

	select {
	case <-done:
		p.O11y.Logger.Debug("Picolytics shutdown completed")
	case <-ctx.Done():
		p.O11y.Logger.Warn(fmt.Sprintf("Picolytics shutdown timed out at %s", step))
		os.Exit(1)
	}
}
