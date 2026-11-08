package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/garda-inflasi/api/internal/worker"
	"github.com/garda-inflasi/api/pkg/logger"
)

func main() {
	log := logger.NewDefault()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := worker.Run(ctx, os.Args[1:], os.Getenv, os.Stdout, log); err != nil {
		log.Error("worker berhenti dengan galat", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}
}
