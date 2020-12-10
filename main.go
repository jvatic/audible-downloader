package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/jvatic/audible-downloader/internal/runner"
)

func main() {
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
