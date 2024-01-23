package main

import (
	"embed"
	"fmt"
	"log"
	"os"

	"github.com/nmcclain/picolytics/picolytics"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

//go:embed static
var staticFilesSource embed.FS

// build-time injected variables
var InjectedGitCommit, InjectedGitBranch, InjectedAppVersion string

func getConfig() (*picolytics.Config, []string, error) {
	var config picolytics.Config
	picolytics.SetConfigDefaults()

	viper.AutomaticEnv() // Automatically read from environment variables

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

	viper.SetConfigType("yaml")
	viper.SetConfigName(viper.GetString("configName")) // configName is set as a default or through BIND_ENV
	viper.AddConfigPath(viper.GetString("configPath")) // configPath is set as a default or through BIND_ENV

	writeConfig := pflag.Bool("write-default-config", false, "Set to true to write default config file")
	pflag.Parse()
	if *writeConfig {
		if err := viper.SafeWriteConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileAlreadyExistsError); ok {
				log.Printf("Config file already exists - not written.")
			} else {
				log.Printf("Error writing config file: %s", err)
			}
			os.Exit(1)
		}
		log.Printf("Config file written.")
		os.Exit(0)
	}

	// save warnings until logger is setup
	warnings := []string{}
	if err := viper.ReadInConfig(); err != nil {
		warnings = append(warnings, fmt.Sprintf("config file error, using env vars and defaults: %s", err))
	}

	// populate config from env vars here
	if err := viper.Unmarshal(&config); err != nil {
		return nil, warnings, fmt.Errorf("unable to decode config file: %v", err)
	}

	// internal config
	config.GitCommit = InjectedGitCommit
	config.GitBranch = InjectedGitBranch
	config.AppVersion = InjectedAppVersion
	config.StaticFiles = staticFilesSource

	return &config, warnings, nil
}
