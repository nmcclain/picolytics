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

// getConfig retrieves the configuration settings for the application.
// Order of priority: defaults < config file < environment variables
//
// The configuration file can be specified in two ways:
// 1. Using the `-c` or `--config` command-line flag.
// 2. Using the `CONFIG_PATH` and `CONFIG_NAME` environment variables.
//
// If the `--write-default-config` flag is set to true, it writes the default
// configuration to a file and exits.
//
// Returns:
// - A pointer to the config struct populated with the configuration settings.
// - A slice of warnings encountered during the configuration process.
// - An error if there was a problem reading or parsing the configuration.
func getConfig() (*picolytics.Config, []string, error) {
	var config picolytics.Config
	picolytics.SetConfigDefaults()

	// TK: figure out logger situation.
	viper.AutomaticEnv() // Automatically read from environment variables
	picolytics.BindEnvVars()

	// Define flags for configuration file path and writing default config
	configFile := pflag.StringP("config", "c", "", "Path to config file")
	writeConfig := pflag.Bool("write-default-config", false, "Set to true to write default config file")
	pflag.Parse()

	// Check if write default config flag is set
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

	// Check if the config file is specified either via flag or environment variable
	usingConfig := false
	if *configFile != "" {
		usingConfig = true
		viper.SetConfigFile(*configFile)
	} else if viper.GetString("configPath") != "" && viper.GetString("configName") != "" {
		usingConfig = true
		viper.SetConfigFile(viper.GetString("configPath") + "/" + viper.GetString("configName") + ".yaml")
	} else {
		log.Println("No configuration file specified, using defaults and any environment variables.")
	}

	// Read the configuration file
	if usingConfig {
		if err := viper.ReadInConfig(); err != nil {
			return nil, nil, fmt.Errorf("failed to read config file: %v", err)
		}
	}

	if err := viper.Unmarshal(&config); err != nil {
		return nil, nil, fmt.Errorf("unable to decode config file: %v", err)
	}

	// Internal config
	config.GitCommit = InjectedGitCommit
	config.GitBranch = InjectedGitBranch
	config.AppVersion = InjectedAppVersion
	config.StaticFiles = staticFilesSource

	return &config, nil, nil
}
