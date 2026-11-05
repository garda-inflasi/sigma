// Package migrate menerapkan migrasi SQL goose dari direktori db/migrations.
// Dipakai oleh cmd/migrate dan oleh tes agar skema yang diuji identik dengan
// skema yang dijalankan saat deploy.
package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/pressly/goose/v3"
)

// Migrator membungkus provider goose untuk satu koneksi dan satu direktori.
type Migrator struct {
	provider *goose.Provider
}

// New membuat Migrator untuk migrasi di direktori dir.
func New(db *sql.DB, dir string) (*Migrator, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("direktori migrasi %q: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("direktori migrasi %q bukan direktori", dir)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, os.DirFS(dir))
	if err != nil {
		return nil, fmt.Errorf("buat provider goose: %w", err)
	}
	return &Migrator{provider: provider}, nil
}

// Up menerapkan semua migrasi yang belum diterapkan.
func (m *Migrator) Up(ctx context.Context) error {
	if _, err := m.provider.Up(ctx); err != nil {
		return fmt.Errorf("migrasi up: %w", err)
	}
	return nil
}

// UpTo menerapkan migrasi sampai versi tertentu (termasuk).
func (m *Migrator) UpTo(ctx context.Context, version int64) error {
	if _, err := m.provider.UpTo(ctx, version); err != nil {
		return fmt.Errorf("migrasi up ke versi %d: %w", version, err)
	}
	return nil
}

// Down membatalkan satu migrasi terakhir.
func (m *Migrator) Down(ctx context.Context) error {
	if _, err := m.provider.Down(ctx); err != nil {
		return fmt.Errorf("migrasi down: %w", err)
	}
	return nil
}

// DownTo membatalkan migrasi sampai versi tertentu (versi itu tetap terpasang).
func (m *Migrator) DownTo(ctx context.Context, version int64) error {
	if _, err := m.provider.DownTo(ctx, version); err != nil {
		return fmt.Errorf("migrasi down ke versi %d: %w", version, err)
	}
	return nil
}

// Apply menjalankan tepat satu migrasi (Up bila up true, selain itu Down) tanpa
// memeriksa urutan; berguna untuk menguji satu migrasi secara terpisah.
func (m *Migrator) Apply(ctx context.Context, version int64, up bool) error {
	if _, err := m.provider.ApplyVersion(ctx, version, up); err != nil {
		return fmt.Errorf("terapkan migrasi %d (up=%t): %w", version, up, err)
	}
	return nil
}

// Version mengembalikan versi skema terakhir yang diterapkan (0 bila kosong).
func (m *Migrator) Version(ctx context.Context) (int64, error) {
	v, err := m.provider.GetDBVersion(ctx)
	if err != nil {
		return 0, fmt.Errorf("baca versi skema: %w", err)
	}
	return v, nil
}
