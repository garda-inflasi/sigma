package seedkit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ProductionEnv adalah nilai APP_ENV yang menolak seeder lapis demo.
const ProductionEnv = "production"

// ErrDemoInProduction dikembalikan bila seeder demo diminta jalan di produksi.
var ErrDemoInProduction = errors.New("seeder demo ditolak: APP_ENV=production")

// Entry mencatat satu seeder yang selesai dijalankan.
type Entry struct {
	Name     string
	Tier     Tier
	Result   Result
	Duration time.Duration
}

// Summary adalah ringkasan satu kali jalan.
type Summary []Entry

// Write mencetak satu baris per seeder.
func (s Summary) Write(w io.Writer) {
	for _, e := range s {
		fmt.Fprintf(w, "%-28s %-9s %-32s %s\n", e.Name, e.Tier, e.Result, e.Duration.Round(time.Millisecond))
	}
}

// ParseTiers menerjemahkan argumen CLI: satu atau lebih nama lapis, atau "all".
func ParseTiers(args []string) ([]Tier, error) {
	if len(args) == 0 {
		return nil, errors.New("sebutkan minimal satu lapis")
	}
	var tiers []Tier
	for _, arg := range args {
		if arg == "all" {
			return AllTiers, nil
		}
		tier := Tier(arg)
		if !tier.valid() {
			names := make([]string, len(AllTiers))
			for i, t := range AllTiers {
				names[i] = string(t)
			}
			return nil, fmt.Errorf("lapis %q tidak dikenal; pilih dari: %s, all", arg, strings.Join(names, ", "))
		}
		if !slices.Contains(tiers, tier) {
			tiers = append(tiers, tier)
		}
	}
	return tiers, nil
}

// CheckEnvironment menolak rencana yang memuat seeder demo bila appEnv adalah
// produksi. Dipanggil sebelum menyentuh basis data.
func CheckEnvironment(plan []Seeder, appEnv string) error {
	if !strings.EqualFold(strings.TrimSpace(appEnv), ProductionEnv) {
		return nil
	}
	for _, s := range plan {
		if s.Tier == TierDemo {
			return fmt.Errorf("%s: %w", s.Name, ErrDemoInProduction)
		}
	}
	return nil
}

// Run menjalankan plan berurutan dan berhenti pada galat pertama. Penjagaan
// APP_ENV diperiksa lebih dulu agar tidak ada seeder yang jalan separuh.
func Run(ctx context.Context, db *gorm.DB, plan []Seeder, now time.Time, appEnv string) (Summary, error) {
	if err := CheckEnvironment(plan, appEnv); err != nil {
		return nil, err
	}

	summary := make(Summary, 0, len(plan))
	for _, s := range plan {
		started := time.Now()
		result, err := s.Run(ctx, db, now)
		if err != nil {
			return summary, fmt.Errorf("seeder %s: %w", s.Name, err)
		}
		summary = append(summary, Entry{Name: s.Name, Tier: s.Tier, Result: result, Duration: time.Since(started)})
	}
	return summary, nil
}
