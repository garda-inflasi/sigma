// Perintah migrate menerapkan migrasi goose ke basis data. Dipakai di deploy
// (service migrate) dan lokal (make db/migrate); berhenti dengan kode keluar
// non-nol pada kegagalan sehingga deploy tidak berlanjut di atas skema rusak.
//
//	migrate up        terapkan semua migrasi baru
//	migrate down      batalkan satu migrasi terakhir
//	migrate version   cetak versi skema saat ini
//
// Konfigurasi: DATABASE_URL (wajib), MIGRATIONS_DIR (bawaan db/migrations).
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/garda-inflasi/api/internal/platform/migrate"
	_ "github.com/jackc/pgx/v5/stdlib" // driver database/sql "pgx"
)

const (
	usage            = "pemakaian: migrate up|down|version"
	defaultMigrasi   = "db/migrations"
	migrationTimeout = 10 * time.Minute

	// Basis data bisa baru dinyalakan bersamaan dengan perintah ini (docker compose up).
	connectTimeout  = 30 * time.Second
	connectInterval = time.Second
)

func main() {
	if err := run(os.Args[1:], os.Getenv, os.Stdout); err != nil {
		log.Fatalf("migrate: %v", err)
	}
}

func run(args []string, getenv func(string) string, out io.Writer) error {
	if len(args) != 1 {
		return errors.New(usage)
	}
	command := args[0]
	if command != "up" && command != "down" && command != "version" {
		return fmt.Errorf("perintah %q tidak dikenal\n%s", command, usage)
	}

	dsn := getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL wajib diisi")
	}
	dir := getenv("MIGRATIONS_DIR")
	if dir == "" {
		dir = defaultMigrasi
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("buka koneksi: %w", err)
	}
	defer db.Close()

	migrator, err := migrate.New(db, dir)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), migrationTimeout)
	defer cancel()

	if err := waitForDatabase(ctx, db, connectTimeout, connectInterval); err != nil {
		return err
	}

	switch command {
	case "up":
		err = migrator.Up(ctx)
	case "down":
		err = migrator.Down(ctx)
	}
	if err != nil {
		return err
	}

	version, err := migrator.Version(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "versi skema: %d\n", version)
	return nil
}

// waitForDatabase mencoba ping sampai berhasil atau timeout habis.
func waitForDatabase(ctx context.Context, db *sql.DB, timeout, interval time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		err := db.PingContext(ctx)
		if err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("basis data tidak dapat dihubungi dalam %s: %w", timeout, err)
		case <-time.After(interval):
		}
	}
}
