package picolytics

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	// injected values
	GitCommit, GitBranch, AppVersion string
	// db:
	PgConnString   string `mapstructure:"pgConnString"`
	PgHost         string `mapstructure:"pgHost"`     // required
	PgDatabase     string `mapstructure:"pgDatabase"` // required
	PgUser         string `mapstructure:"pgUser"`     // required
	PgPassword     string `mapstructure:"pgPassword"` // required
	PgPort         int    `mapstructure:"pgPort"`
	PgSslMode      string `mapstructure:"pgSslMode"`
	PgConnAttempts int    `mapstructure:"pgConnAttempts"`
	SkipMigrations bool   `mapstructure:"skipMigrations"`
	// server:
	ListenAddr     string `mapstructure:"listenAddr"`
	AutotlsEnabled bool   `mapstructure:"autotlsEnabled"`
	AutotlsHost    string `mapstructure:"autotlsHost"`
	AutotlsStaging bool   `mapstructure:"autotlsStaging"`
	AdminListen    string `mapstructure:"adminListen"`
	StaticDir      string `mapstructure:"staticDir"`
	RootRedirect   string `mapstructure:"rootRedirect"`
	// proxy:
	IPExtractor    string   `mapstructure:"ipExtractor"`
	TrustedProxies []string `mapstructure:"trustedProxies"`
	// privacy:
	GeoIPFile         string `mapstructure:"geoIpFile"`
	SessionTimeoutMin int    `mapstructure:"sessionTimeoutMin"`
	// tuning:
	QueueSize          int      `mapstructure:"queueSize"`
	BatchMaxSize       int      `mapstructure:"batchMaxSize"`
	BatchMaxMsec       int      `mapstructure:"batchMaxMsec"`
	RequestRateLimit   int      `mapstructure:"requestRateLimit"`
	BodyMaxSize        int64    `mapstructure:"bodyMaxSize"`
	StaticCacheMaxAge  int      `mapstructure:"staticCacheMaxAge"`
	DisableHostMetrics bool     `mapstructure:"disableHostMetrics"`
	LogFormat          string   `mapstructure:"logFormat"`
	PruneDays          int      `mapstructure:"pruneDays"`
	PruneCheckHours    int      `mapstructure:"pruneCheckHours"`
	ValidEventNames    []string `mapstructure:"validEventNames"`
	Debug              bool     `mapstructure:"debug"`

	// internal config
	StaticFiles fs.FS `mapstructure:"-"`
}

func SetConfigDefaults() {
	viper.SetDefault("pgPort", "5432")
	viper.SetDefault("pgSslMode", "prefer")
	viper.SetDefault("pgConnAttempts", 5)
	viper.SetDefault("skipMigrations", false)
	viper.SetDefault("listenAddr", ":8080")
	viper.SetDefault("adminListen", "") // disabled
	viper.SetDefault("staticDir", "static")
	viper.SetDefault("rootRedirect", "")
	viper.SetDefault("autotlsEnabled", false)
	viper.SetDefault("autotlsStaging", true)
	viper.SetDefault("ipExtractor", "direct")
	viper.SetDefault("geoIpFile", "geoip.mmdb")
	viper.SetDefault("sessionTimeoutMin", 30)
	viper.SetDefault("queueSize", 640000)
	viper.SetDefault("batchMaxSize", 6400)
	viper.SetDefault("batchMaxMsec", 500)
	viper.SetDefault("requestRateLimit", 10)
	viper.SetDefault("bodyMaxSize", int64(2*1024)) // 2KB
	viper.SetDefault("staticCacheMaxAge", 3600)    // 1 hour
	viper.SetDefault("disableHostMetrics", true)
	viper.SetDefault("logFormat", "json")
	viper.SetDefault("pruneDays", 0)
	viper.SetDefault("pruneCheckHours", 24)
	viper.SetDefault("validEventNames", []string{"load", "visible", "hidden", "hashchange", "ping"})
	viper.SetDefault("debug", false)
}

func BindEnvVars() {
	viper.BindEnv("configName", "CONFIG_NAME")
	viper.BindEnv("configPath", "CONFIG_PATH")
	viper.BindEnv("pgConnString", "PGCONNSTRING") // required unless (and overrides) PGHOST, PGDATABASE, PGUSER, and PGPASSWORD: no default
	viper.BindEnv("pgHost", "PGHOST")             // required unless PGCONNSTRING set: no default
	viper.BindEnv("pgUser", "PGUSER")             // required unless PGCONNSTRING set: no default
	viper.BindEnv("pgPassword", "PGPASSWORD")     // required unless PGCONNSTRING set: no default
	viper.BindEnv("pgDatabase", "PGDATABASE")     // required unless PGCONNSTRING set: no default
	viper.BindEnv("pgPort", "PGPORT")
	viper.BindEnv("pgSslMode", "PGSSLMODE")
	viper.BindEnv("pgConnAttempts", "PGCONNATTEMPTS")
	viper.BindEnv("skipMigrations", "SKIP_MIGRATIONS")
	viper.BindEnv("listenAddr", "LISTEN_ADDR")
	viper.BindEnv("autotlsEnabled", "AUTOTLS_ENABLED")
	viper.BindEnv("autotlsHost", "AUTOTLS_HOST") // required if enableAcme is true
	viper.BindEnv("autotlsStaging", "AUTOTLS_STAGING")
	viper.BindEnv("adminListen", "ADMIN_LISTEN")
	viper.BindEnv("staticDir", "STATIC_DIR")
	viper.BindEnv("rootRedirect", "ROOT_REDIRECT")
	viper.BindEnv("ipExtractor", "IP_EXTRACTOR")
	viper.BindEnv("trustedProxies", "TRUSTED_PROXIES") // comma separated list
	viper.BindEnv("geoIpFile", "GEO_IP_FILE")
	viper.BindEnv("sessionTimeoutMin", "SESSION_TIMEOUT_MIN")
	viper.BindEnv("queueSize", "QUEUE_SIZE")
	viper.BindEnv("batchMaxSize", "BATCH_MAX_SIZE")
	viper.BindEnv("batchMaxMsec", "BATCH_MAX_MSEC")
	viper.BindEnv("requestRateLimit", "REQUEST_RATE_LIMIT") // Limit is represented as number of events per second.
	viper.BindEnv("bodyMaxSize", "BODY_MAX_SIZE")
	viper.BindEnv("staticCacheMaxAge", "STATIC_CACHE_MAX_AGE") // seconds
	viper.BindEnv("disableHostMetrics", "DISABLE_HOST_METRICS")
	viper.BindEnv("logFormat", "LOG_FORMAT")
	viper.BindEnv("pruneDays", "PRUNE_DAYS")
	viper.BindEnv("pruneCheckHours", "PRUNE_CHECK_HOURS")
	viper.BindEnv("validEventNames", "VALID_EVENT_NAMES") // comma separated list
	viper.BindEnv("debug", "DEBUG")
}

func setupLogger(debug bool, logHandler slog.Handler) (*slog.Logger, error) {
	loggerOpts := &slog.HandlerOptions{}
	if debug {
		loggerOpts.Level = slog.LevelDebug
	} else {
		loggerOpts.Level = slog.LevelInfo
	}
	if logHandler != nil { // used for testing
		return slog.New(logHandler), nil
	}
	switch viper.GetString("logFormat") {
	case "text":
		return slog.New(slog.NewTextHandler(os.Stdout, loggerOpts)), nil
	case "json":
		return slog.New(slog.NewJSONHandler(os.Stdout, loggerOpts)), nil
	}
	return nil, fmt.Errorf("invalid logFormat: %s", viper.GetString("logFormat"))
}

func validateConfig(config *Config) error {
	ALLOWED_SSL_MODES := map[string]bool{"disable": true, "allow": true,
		"prefer": true, "require": true, "verify-ca": true, "verify-full": true}
	if _, ok := ALLOWED_SSL_MODES[config.PgSslMode]; !ok {
		return fmt.Errorf("invalid pgSslMode: %s", config.PgSslMode)
	}

	if len(config.PgConnString) < 1 {
		if len(config.PgHost) == 0 || len(config.PgDatabase) == 0 || len(config.PgUser) == 0 || len(config.PgPassword) == 0 {
			return fmt.Errorf("PGCONNSTRING must be set")
		}
		config.PgConnString = fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s", config.PgUser, config.PgPassword, config.PgHost, config.PgPort, config.PgDatabase, config.PgSslMode)
	} else {
		if !strings.HasPrefix(config.PgConnString, "postgres://") {
			return fmt.Errorf("PGCONNSTRING must begin with postgres://")
		}
	}

	ALLOWED_EXTRACTOR_MODES := map[string]bool{"direct": true, "xff": true, "realip": true}
	if _, ok := ALLOWED_EXTRACTOR_MODES[config.IPExtractor]; !ok {
		return fmt.Errorf("invalid ipExtractor mode: %s", config.IPExtractor)
	}

	return nil
}
