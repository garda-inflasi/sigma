package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/garda-inflasi/api/pkg/gardactl"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := gardactl.Run(ctx, os.Args[1:], gardactl.Options{
		Getenv: os.Getenv,
		Out:    os.Stdout,
		Now:    time.Now(),
	}); err != nil {
		log.Fatalf("gardactl: %v", err)
	}
}
