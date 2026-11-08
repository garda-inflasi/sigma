package seed

import (
	"context"
	"os"
	"time"

	auditseeder "github.com/garda-inflasi/api/internal/audit/seeder"
	authseeder "github.com/garda-inflasi/api/internal/auth/seeder"
	coldstorage "github.com/garda-inflasi/api/internal/cold-storage/seeder"
	dashboardseeder "github.com/garda-inflasi/api/internal/dashboard/seeder"
	eksekutifseeder "github.com/garda-inflasi/api/internal/eksekutif/seeder"
	ewsseeder "github.com/garda-inflasi/api/internal/ews/seeder"
	hargaseeder "github.com/garda-inflasi/api/internal/harga/seeder"
	ingestionseeder "github.com/garda-inflasi/api/internal/ingestion/seeder"
	intervensiseeder "github.com/garda-inflasi/api/internal/intervensi/seeder"
	kpiseeder "github.com/garda-inflasi/api/internal/kpi/seeder"
	lapanganseeder "github.com/garda-inflasi/api/internal/lapangan/seeder"
	masterseeder "github.com/garda-inflasi/api/internal/master/seeder"
	briefseeder "github.com/garda-inflasi/api/internal/policy-brief/seeder"
	prediksiseeder "github.com/garda-inflasi/api/internal/prediksi/seeder"
	"github.com/garda-inflasi/api/internal/seedkit"
	spasialseeder "github.com/garda-inflasi/api/internal/spasial/seeder"
	sumberseeder "github.com/garda-inflasi/api/internal/sumber-data/seeder"
	"gorm.io/gorm"
)

func init() {
	// ── Master ─────────────────────────────────────────────────────────────
	seedkit.Register(seedkit.Seeder{
		Name: "master.reference",
		Tier: seedkit.TierReference,
		Run: func(ctx context.Context, db *gorm.DB, _ time.Time) (seedkit.Result, error) {
			sum, err := masterseeder.SeedReference(ctx, db)
			return seedkit.Result{
				"satuan": sum.Satuan, "provinsi": sum.Provinsi, "kabupaten": sum.Kabupaten, "kecamatan": sum.Kecamatan,
				"pasar": sum.Pasar, "komoditas": sum.Komoditas,
			}, err
		},
	})
	seedkit.Register(seedkit.Seeder{
		Name:      "master.demo",
		Tier:      seedkit.TierDemo,
		DependsOn: []string{"master.reference", "harga.demo"},
		Run: func(ctx context.Context, db *gorm.DB, _ time.Time) (seedkit.Result, error) {
			sum, err := masterseeder.SeedDemo(ctx, db)
			return seedkit.Result{"pedagang": sum.Pedagang}, err
		},
	})

	// ── Auth & RBAC ────────────────────────────────────────────────────────
	seedkit.Register(seedkit.Seeder{
		Name: "auth.reference",
		Tier: seedkit.TierReference,
		Run: func(ctx context.Context, db *gorm.DB, _ time.Time) (seedkit.Result, error) {
			sum, err := authseeder.SeedRBAC(ctx, db)
			return seedkit.Result{"peran": sum.PeranBaru, "izin": sum.IzinBaru, "penugasan": sum.PenugasanBaru}, err
		},
	})
	seedkit.Register(seedkit.Seeder{
		Name: "auth.admin",
		Tier: seedkit.TierAdmin,
		Run: func(ctx context.Context, db *gorm.DB, _ time.Time) (seedkit.Result, error) {
			sum, err := authseeder.SeedAdmin(ctx, db, authseeder.AdminConfig{
				Email:    os.Getenv("SEED_ADMIN_EMAIL"),
				Password: os.Getenv("SEED_ADMIN_PASSWORD"),
				Output:   os.Stdout,
			})
			return seedkit.Result{"admin": sum.Dibuat}, err
		},
	})
	seedkit.Register(seedkit.Seeder{
		Name:      "auth.demo",
		Tier:      seedkit.TierDemo,
		DependsOn: []string{"master.reference", "auth.reference"},
		After:     []string{"auth.admin"},
		Run: func(ctx context.Context, db *gorm.DB, _ time.Time) (seedkit.Result, error) {
			sum, err := authseeder.SeedDemo(ctx, db)
			return seedkit.Result{"akun": sum.Akun}, err
		},
	})

	// ── Sumber Data ────────────────────────────────────────────────────────
	seedkit.Register(seedkit.Seeder{
		Name: "sumber.reference",
		Tier: seedkit.TierReference,
		Run: func(ctx context.Context, db *gorm.DB, _ time.Time) (seedkit.Result, error) {
			sum, err := sumberseeder.SeedReference(ctx, db)
			return seedkit.Result{"sumber": sum.Sumber}, err
		},
	})
	seedkit.Register(seedkit.Seeder{
		Name:  "sumber.demo",
		Tier:  seedkit.TierDemo,
		After: []string{"sumber.reference"},
		Run: func(ctx context.Context, db *gorm.DB, now time.Time) (seedkit.Result, error) {
			sum, err := sumberseeder.SeedDemo(ctx, db, now)
			return seedkit.Result{"sumber": sum.Sumber}, err
		},
	})

	// ── Ingestion ──────────────────────────────────────────────────────────
	seedkit.Register(seedkit.Seeder{
		Name:      "ingestion.reference",
		Tier:      seedkit.TierReference,
		DependsOn: []string{"sumber.reference"},
		Run: func(ctx context.Context, db *gorm.DB, _ time.Time) (seedkit.Result, error) {
			sum, err := ingestionseeder.SeedReference(ctx, db)
			return seedkit.Result{"pemetaan": sum.Pemetaan}, err
		},
	})
	seedkit.Register(seedkit.Seeder{
		Name:      "ingestion.demo",
		Tier:      seedkit.TierDemo,
		DependsOn: []string{"master.reference", "sumber.reference"},
		After:     []string{"sumber.demo", "harga.demo"},
		Run: func(ctx context.Context, db *gorm.DB, now time.Time) (seedkit.Result, error) {
			sum, err := ingestionseeder.SeedDemo(ctx, db, now)
			return seedkit.Result{"eksekusi": sum.Eksekusi, "mentah": sum.Mentah, "harga": sum.Harga, "cuaca": sum.Cuaca}, err
		},
	})

	// ── Harga Pangan ───────────────────────────────────────────────────────
	seedkit.Register(seedkit.Seeder{
		Name:      "harga.reference",
		Tier:      seedkit.TierReference,
		DependsOn: []string{"master.reference"},
		Run: func(ctx context.Context, db *gorm.DB, _ time.Time) (seedkit.Result, error) {
			sum, err := hargaseeder.SeedReference(ctx, db)
			return seedkit.Result{"batas": sum.Batas}, err
		},
	})
	seedkit.Register(seedkit.Seeder{
		Name:      "harga.demo",
		Tier:      seedkit.TierDemo,
		DependsOn: []string{"master.reference"},
		After:     []string{"auth.demo"},
		Run: func(ctx context.Context, db *gorm.DB, now time.Time) (seedkit.Result, error) {
			sum, err := hargaseeder.SeedDemo(ctx, db, now)
			return seedkit.Result{"entri": sum.Entri, "riwayat": sum.Riwayat}, err
		},
	})

	// ── Petugas Lapangan ───────────────────────────────────────────────────
	seedkit.Register(seedkit.Seeder{
		Name:      "lapangan.demo",
		Tier:      seedkit.TierDemo,
		DependsOn: []string{"master.reference"},
		After:     []string{"auth.demo", "harga.demo"},
		Run: func(ctx context.Context, db *gorm.DB, now time.Time) (seedkit.Result, error) {
			sum, err := lapanganseeder.SeedDemo(ctx, db, now)
			return seedkit.Result{"tugas": sum.Tugas}, err
		},
	})

	// ── Cold Storage ───────────────────────────────────────────────────────
	seedkit.Register(seedkit.Seeder{
		Name:      "coldstorage.reference",
		Tier:      seedkit.TierReference,
		DependsOn: []string{"master.reference"},
		Run: func(ctx context.Context, db *gorm.DB, _ time.Time) (seedkit.Result, error) {
			sum, err := coldstorage.SeedReference(ctx, db)
			return seedkit.Result{"fasilitas": sum.Fasilitas}, err
		},
	})
	seedkit.Register(seedkit.Seeder{
		Name:      "coldstorage.demo",
		Tier:      seedkit.TierDemo,
		DependsOn: []string{"coldstorage.reference"},
		Run: func(ctx context.Context, db *gorm.DB, now time.Time) (seedkit.Result, error) {
			sum, err := coldstorage.SeedDemo(ctx, db, now)
			return seedkit.Result{"lot": sum.Stok, "telemetri": sum.Telemetri, "transaksi": sum.Transaksi}, err
		},
	})

	// ── EWS (Early Warning System) ─────────────────────────────────────────
	seedkit.Register(seedkit.Seeder{
		Name:      "ews.reference",
		Tier:      seedkit.TierReference,
		DependsOn: []string{"master.reference"},
		Run: func(ctx context.Context, db *gorm.DB, _ time.Time) (seedkit.Result, error) {
			sum, err := ewsseeder.SeedReference(ctx, db)
			return seedkit.Result{"aturan": sum.Aturan}, err
		},
	})
	seedkit.Register(seedkit.Seeder{
		Name:      "ews.demo",
		Tier:      seedkit.TierDemo,
		DependsOn: []string{"ews.reference", "auth.demo"},
		Run: func(ctx context.Context, db *gorm.DB, now time.Time) (seedkit.Result, error) {
			sum, err := ewsseeder.SeedDemo(ctx, db, now)
			return seedkit.Result{"alert": sum.Alert, "riwayat": sum.Riwayat}, err
		},
	})

	// ── Intervensi ─────────────────────────────────────────────────────────
	seedkit.Register(seedkit.Seeder{
		Name: "intervensi.reference",
		Tier: seedkit.TierReference,
		Run: func(ctx context.Context, db *gorm.DB, _ time.Time) (seedkit.Result, error) {
			sum, err := intervensiseeder.SeedReference(ctx, db)
			return seedkit.Result{"tahap": sum.Tahap, "alur": sum.Alur, "langkah_alur": sum.LangkahAlur}, err
		},
	})
	seedkit.Register(seedkit.Seeder{
		Name:      "intervensi.demo",
		Tier:      seedkit.TierDemo,
		DependsOn: []string{"intervensi.reference", "master.reference", "auth.demo", "ews.demo"},
		Run: func(ctx context.Context, db *gorm.DB, now time.Time) (seedkit.Result, error) {
			sum, err := intervensiseeder.SeedDemo(ctx, db, now)
			return seedkit.Result{"kartu": sum.Kartu, "penanggung_jawab": sum.PenanggungJawab, "riwayat": sum.Riwayat, "persetujuan": sum.Persetujuan}, err
		},
	})

	// ── KPI ────────────────────────────────────────────────────────────────
	seedkit.Register(seedkit.Seeder{
		Name: "kpi.reference",
		Tier: seedkit.TierReference,
		Run: func(ctx context.Context, db *gorm.DB, _ time.Time) (seedkit.Result, error) {
			sum, err := kpiseeder.SeedReference(ctx, db)
			return seedkit.Result{"definisi": sum.Definisi, "target": sum.Target}, err
		},
	})
	seedkit.Register(seedkit.Seeder{
		Name:      "kpi.demo",
		Tier:      seedkit.TierDemo,
		DependsOn: []string{"master.reference"},
		After:     []string{"kpi.reference", "harga.demo", "coldstorage.demo", "ews.demo", "intervensi.demo", "prediksi.demo"},
		Run: func(ctx context.Context, db *gorm.DB, now time.Time) (seedkit.Result, error) {
			sum, err := kpiseeder.SeedDemo(ctx, db, now)
			return seedkit.Result{"baru": sum.Baru, "direvisi": sum.Direvisi, "tetap": sum.Tetap, "tanpa_dasar": sum.TanpaDasar}, err
		},
	})

	// ── Prediksi ML ────────────────────────────────────────────────────────
	seedkit.Register(seedkit.Seeder{
		Name:      "prediksi.demo",
		Tier:      seedkit.TierDemo,
		DependsOn: []string{"master.reference"},
		After:     []string{"harga.demo"},
		Run: func(ctx context.Context, db *gorm.DB, now time.Time) (seedkit.Result, error) {
			sum, err := prediksiseeder.SeedDemo(ctx, db, now)
			return seedkit.Result{"model": sum.Model, "proyeksi": sum.Proyeksi}, err
		},
	})

	// ── Policy Brief ───────────────────────────────────────────────────────
	seedkit.Register(seedkit.Seeder{
		Name:      "brief.demo",
		Tier:      seedkit.TierDemo,
		DependsOn: []string{"master.reference"},
		After:     []string{"auth.demo"},
		Run: func(ctx context.Context, db *gorm.DB, now time.Time) (seedkit.Result, error) {
			sum, err := briefseeder.SeedDemo(ctx, db, now)
			return seedkit.Result{"brief": sum.Brief}, err
		},
	})

	// ── Spasial ────────────────────────────────────────────────────────────
	seedkit.Register(seedkit.Seeder{
		Name:      "spasial.demo",
		Tier:      seedkit.TierDemo,
		DependsOn: []string{"master.reference"},
		After:     []string{"coldstorage.reference"},
		Run: func(ctx context.Context, db *gorm.DB, now time.Time) (seedkit.Result, error) {
			sum, err := spasialseeder.SeedDemo(ctx, db, now)
			return seedkit.Result{"neraca": sum.Neraca, "rute": sum.Rute, "kandidat": sum.Kandidat, "geometri": sum.Geometri}, err
		},
	})

	// ── Eksekutif ──────────────────────────────────────────────────────────
	seedkit.Register(seedkit.Seeder{
		Name: "eksekutif.reference",
		Tier: seedkit.TierReference,
		Run: func(ctx context.Context, db *gorm.DB, _ time.Time) (seedkit.Result, error) {
			sum, err := eksekutifseeder.SeedReference(ctx, db)
			return seedkit.Result{"target": sum.Target}, err
		},
	})
	seedkit.Register(seedkit.Seeder{
		Name:      "eksekutif.demo",
		Tier:      seedkit.TierDemo,
		DependsOn: []string{"master.reference"},
		After:     []string{"eksekutif.reference", "ews.demo", "intervensi.demo"},
		Run: func(ctx context.Context, db *gorm.DB, now time.Time) (seedkit.Result, error) {
			sum, err := eksekutifseeder.SeedDemo(ctx, db, now)
			return seedkit.Result{"inflasi": sum.Inflasi, "andil": sum.Andil, "neraca": sum.Neraca, "fiskal": sum.Fiskal}, err
		},
	})

	// ── Dashboard ──────────────────────────────────────────────────────────
	seedkit.Register(seedkit.Seeder{
		Name:      "dashboard.reference",
		Tier:      seedkit.TierReference,
		DependsOn: []string{"master.reference"},
		Run: func(ctx context.Context, db *gorm.DB, _ time.Time) (seedkit.Result, error) {
			sum, err := dashboardseeder.SeedReference(ctx, db)
			return seedkit.Result{"target": sum.Target}, err
		},
	})
	seedkit.Register(seedkit.Seeder{
		Name:      "dashboard.demo",
		Tier:      seedkit.TierDemo,
		DependsOn: []string{"dashboard.reference"},
		After:     []string{"auth.demo", "harga.demo", "lapangan.demo", "coldstorage.demo"},
		Run: func(ctx context.Context, db *gorm.DB, now time.Time) (seedkit.Result, error) {
			sum, err := dashboardseeder.SeedDemo(ctx, db, now)
			return seedkit.Result{"siklus": sum.Siklus}, err
		},
	})

	// ── Audit Log ──────────────────────────────────────────────────────────
	seedkit.Register(seedkit.Seeder{
		Name:      "audit.demo",
		Tier:      seedkit.TierDemo,
		DependsOn: []string{"auth.demo"},
		Run: func(ctx context.Context, db *gorm.DB, now time.Time) (seedkit.Result, error) {
			sum, err := auditseeder.SeedDemo(ctx, db, now)
			return seedkit.Result{"entri": sum.Entri}, err
		},
	})
}
