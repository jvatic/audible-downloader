package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/jvatic/audible-downloader/internal/runner"
	log "github.com/sirupsen/logrus"
)

func main() {
	switch os.Getenv("LOG_LEVEL") {
	case "trace":
		log.SetLevel(log.TraceLevel)
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	default:
		log.SetLevel(log.InfoLevel)
	}

	ctx, cancel := context.WithCancel(context.Background())
	// SIGINT or SIGTERM cancels ctx, triggering a graceful shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Reset()
	go func() {
		select {
		case <-sigs:
			cancel()
			signal.Reset() // repeated signals will have default behaviour
		case <-ctx.Done():
		}
	}()

	if err := runner.Run(ctx); err != nil {
		log.Fatalf("Error: %v", err)
	}
}
