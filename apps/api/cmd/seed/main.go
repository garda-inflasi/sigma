// Perintah seed mengisi basis data lewat seeder yang terdaftar di seedkit.
//
//	seed reference   data referensi (aman di produksi)
//	seed admin       akun admin awal (aman di produksi)
//	seed demo        data contoh (ditolak bila APP_ENV=production)
//	seed all         ketiganya, berurutan
//
// Beberapa lapis boleh disebut sekaligus, mis. `seed reference admin`.
//
// Seeder modul didaftarkan lewat berkas register_<modul>.go di paket ini.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/garda-inflasi/api/internal/seedkit"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	usage       = "pemakaian: seed reference|admin|demo|all [lapis...]"
	seedTimeout = 5 * time.Minute
)

func main() {
	if err := run(os.Args[1:], os.Getenv, os.Stdout); err != nil {
		log.Fatalf("seed: %v", err)
	}
}

func run(args []string, getenv func(string) string, out io.Writer) error {
	tiers, err := seedkit.ParseTiers(args)
	if err != nil {
		return fmt.Errorf("%w\n%s", err, usage)
	}
	plan, err := seedkit.Plan(tiers...)
	if err != nil {
		return err
	}
	appEnv := getenv("APP_ENV")
	if err := seedkit.CheckEnvironment(plan, appEnv); err != nil {
		return err
	}
	if len(plan) == 0 {
		fmt.Fprintf(out, "tidak ada seeder terdaftar untuk lapis %s\n", strings.Join(args, ", "))
		return nil
	}

	db, err := connect(getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer closeDB(db)

	ctx, cancel := context.WithTimeout(context.Background(), seedTimeout)
	defer cancel()

	summary, err := seedkit.Run(ctx, db, plan, time.Now(), appEnv)
	summary.Write(out)
	return err
}

func connect(dsn string) (*gorm.DB, error) {
	if dsn == "" {
		return nil, errors.New("DATABASE_URL wajib diisi")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
	if err != nil {
		return nil, fmt.Errorf("hubungkan basis data: %w", err)
	}
	return db, nil
}

func closeDB(db *gorm.DB) {
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
}
