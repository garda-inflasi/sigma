package main

import (
	"log"
	"os"

	"github.com/garda-inflasi/api/pkg/seed"
)

func main() {
	if err := seed.Run(os.Args[1:], os.Getenv, os.Stdout); err != nil {
		log.Fatalf("seed: %v", err)
	}
}
