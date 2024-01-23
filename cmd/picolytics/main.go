package main

import (
	"log"

	"github.com/nmcclain/picolytics/picolytics"

	_ "go.uber.org/automaxprocs" // automaxprocs automatically sets GOMAXPROCS to match the Linux container CPU quota, if any.
)

func main() {
	config, warnings, err := getConfig()
	if err != nil {
		log.Fatalf("Config error: %v", err)
	}
	p, err := picolytics.New(config, nil, nil)
	if err != nil {
		log.Fatalf("Initialization error: %v", err)
	}
	defer p.Shutdown()
	for _, warning := range warnings {
		p.O11y.Logger.Warn(warning)
	}

	p.Run()
	p.HandleShutdown()
}
