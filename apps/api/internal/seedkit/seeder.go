// Package seedkit adalah kerangka seeder: tiap modul mendaftarkan seeder-nya
// (lapis, dependensi, fungsi Run) dan cmd/seed menjalankannya berurutan.
//
// Lapis menentukan keamanan: reference dan admin aman di produksi, demo
// ditolak bila APP_ENV=production. Semua seeder harus idempoten.
package seedkit

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

// Tier adalah lapis seeder.
type Tier string

const (
	// TierReference berisi data referensi yang aman di produksi.
	TierReference Tier = "reference"
	// TierAdmin berisi akun admin awal; aman di produksi.
	TierAdmin Tier = "admin"
	// TierDemo berisi data contoh; tidak boleh jalan di produksi.
	TierDemo Tier = "demo"
)

// AllTiers adalah urutan lapis bila semua dijalankan.
var AllTiers = []Tier{TierReference, TierAdmin, TierDemo}

func (t Tier) valid() bool { return slices.Contains(AllTiers, t) }

// rank mengurutkan lapis; lapis tak dikenal paling akhir.
func (t Tier) rank() int {
	if i := slices.Index(AllTiers, t); i >= 0 {
		return i
	}
	return len(AllTiers)
}

// Result melaporkan jumlah baris yang ditulis seeder, per label (mis. "fasilitas").
type Result map[string]int

// String menampilkan hasil terurut menurut label, mis. "fasilitas=5 stok=23".
func (r Result) String() string {
	labels := make([]string, 0, len(r))
	for label := range r {
		labels = append(labels, label)
	}
	sort.Strings(labels)

	parts := make([]string, len(labels))
	for i, label := range labels {
		parts[i] = fmt.Sprintf("%s=%d", label, r[label])
	}
	return strings.Join(parts, " ")
}

// Seeder mengisi satu bagian data. Name berbentuk "<modul>.<lapis>", mis.
// "coldstorage.demo".
type Seeder struct {
	Name string
	Tier Tier
	// DependsOn wajib terdaftar dan dijalankan lebih dulu bila ikut terpilih.
	// Nama yang tidak terdaftar adalah galat.
	DependsOn []string
	// After hanya mengatur urutan, dan hanya bila seeder itu terdaftar dan terpilih.
	After []string
	// Run harus idempoten; now dipakai untuk tanggal relatif agar deterministik.
	Run func(ctx context.Context, db *gorm.DB, now time.Time) (Result, error)
}

func (s Seeder) validate() error {
	switch {
	case s.Name == "":
		return errors.New("nama seeder kosong")
	case !s.Tier.valid():
		return fmt.Errorf("seeder %q: lapis %q tidak dikenal", s.Name, s.Tier)
	case !strings.HasSuffix(s.Name, "."+string(s.Tier)):
		return fmt.Errorf("seeder %q: nama harus berakhiran .%s sesuai lapisnya", s.Name, s.Tier)
	case s.Run == nil:
		return fmt.Errorf("seeder %q: Run kosong", s.Name)
	}
	return nil
}
