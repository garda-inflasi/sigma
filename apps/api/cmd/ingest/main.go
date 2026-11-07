// Perintah ingest menjalankan dan mengonfigurasi ingestion sumber data eksternal.
//
//	ingest [jalankan] <sumber>                          jalankan satu sumber sekali (mis. SRC-BI)
//	ingest daftar                                       daftar sumber beserta konektornya
//	ingest pemetaan <sumber> <KOMODITAS|WILAYAH> <alias> <kode|->
//	                                                    petakan alias sumber ke kode master ("-" = abaikan alias)
//	ingest konfigurasi <sumber> <json>                  ganti konfigurasi permintaan sumber (objek JSON)
//
// <sumber> adalah kode sumber (sumber_data.kode, mis. SRC-BI) atau UUID-nya. Konfigurasi dan konektor merujuk
// sumber lewat kode; baris riwayat, pemetaan, dan audit memakai UUID.
//
// Satu eksekusi tercatat di eksekusi_sumber dan tampil di halaman /sumber-data.
// Perintah keluar dengan kode non-nol bila eksekusi berstatus GAGAL.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/dashboard/refresher"
	"github.com/garda-inflasi/api/internal/ingestion"
	"github.com/garda-inflasi/api/internal/ingestion/domain"
	"github.com/garda-inflasi/api/internal/ingestion/engine"
	"github.com/garda-inflasi/api/internal/ingestion/repository/postgres"
	sumberrepo "github.com/garda-inflasi/api/internal/sumber-data/repository/postgres"
	"github.com/garda-inflasi/api/pkg/logger"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Getenv, os.Stdout, nil); err != nil {
		log.Fatalf("ingest: %v", err)
	}
}

// run menjalankan satu perintah. fetcher nil memakai Fetcher HTTP nyata; tes menyuntikkan fixture.
func run(ctx context.Context, args []string, getenv func(string) string, out io.Writer, fetcher domain.Fetcher) error {
	if len(args) == 0 {
		return errors.New("perintah wajib: ingest <sumber> | daftar | pemetaan | konfigurasi")
	}
	dsn := getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL wajib diisi")
	}
	db, err := gorm.Open(gormpostgres.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Warn)})
	if err != nil {
		return fmt.Errorf("hubungkan basis data: %w", err)
	}
	if sqlDB, err := db.DB(); err == nil {
		defer sqlDB.Close()
	}

	switch args[0] {
	case "daftar":
		return daftar(ctx, db, out, fetcher)
	case "pemetaan":
		return pemetaan(ctx, db, args[1:], out)
	case "konfigurasi":
		return konfigurasi(ctx, db, args[1:], out)
	case "jalankan":
		args = args[1:]
	}
	if len(args) != 1 {
		return errors.New("jalankan: sebutkan tepat satu sumber (kode atau UUID), mis. ingest SRC-BI")
	}
	return jalankan(ctx, db, args[0], out, fetcher)
}

func newRunner(db *gorm.DB, fetcher domain.Fetcher) *engine.Runner {
	log := logger.NewDefault()
	if fetcher == nil {
		fetcher = ingestion.NewFetcher()
	}
	// Proses CLI berumur pendek: agregat disegarkan sinkron sesudah run, bukan lewat goroutine.
	return engine.New(postgres.New(db), fetcher, log, ingestion.Konektor()...)
}

func jalankan(ctx context.Context, db *gorm.DB, ref string, out io.Writer, fetcher domain.Fetcher) error {
	runner := newRunner(db, fetcher)
	s, err := postgres.New(db).Sumber(ctx, ref)
	if err != nil {
		return err
	}
	run, err := runner.Mulai(ctx, s.ID, domain.PemicuCLI, nil)
	if err != nil {
		return err
	}
	runCtx, cancel := context.WithTimeout(ctx, engine.BatasWaktu)
	defer cancel()
	e, err := run.Jalankan(runCtx)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "ingest %s: eksekusi=%d status=%s dokumen=%d diambil=%d tak_terpetakan=%d tidak_valid=%d\n",
		s.Kode, e.ID, e.Status, e.Dokumen, e.BarisDiambil, e.BarisTakTerpetakan, e.BarisTidakValid)
	if e.Galat != nil {
		fmt.Fprintf(out, "galat: %s\n", *e.Galat)
	}
	catatAudit(ctx, db, "JALANKAN_SUMBER", s.ID, map[string]any{"kode": s.Kode, "eksekusi_id": e.ID, "status": e.Status})

	if e.BarisDiambil > 0 {
		if err := refresher.Shared(db, logger.NewDefault()).Refresh(ctx); err != nil {
			return fmt.Errorf("segarkan agregat: %w", err)
		}
	}
	if e.Status == domain.StatusGagal {
		return fmt.Errorf("eksekusi %d gagal", e.ID)
	}
	return nil
}

func daftar(ctx context.Context, db *gorm.DB, out io.Writer, fetcher domain.Fetcher) error {
	runner := newRunner(db, fetcher)
	items, err := sumberrepo.New(db).List(ctx)
	if err != nil {
		return err
	}
	for _, s := range items {
		konektor := "tanpa konektor"
		if runner.Punya(s.Kode) {
			konektor = "konektor tersedia"
		}
		fmt.Fprintf(out, "%-12s %-10s %-16s %s\n", s.Kode, s.Status, konektor, s.Nama)
	}
	return nil
}

func pemetaan(ctx context.Context, db *gorm.DB, args []string, out io.Writer) error {
	if len(args) != 4 {
		return errors.New("pemetaan: ingest pemetaan <sumber> <KOMODITAS|WILAYAH> <alias> <kode|->")
	}
	ref, jenis, alias, kode := args[0], strings.ToUpper(args[1]), domain.NormalisasiAlias(args[2]), args[3]
	if jenis != domain.JenisKomoditas && jenis != domain.JenisWilayah {
		return fmt.Errorf("jenis %q tidak dikenal (KOMODITAS atau WILAYAH)", args[1])
	}
	if alias == "" {
		return errors.New("alias kosong")
	}

	s, err := postgres.New(db).Sumber(ctx, ref)
	if err != nil {
		return err
	}
	row := domain.PemetaanSumber{SumberID: s.ID, Jenis: jenis, Alias: alias}
	if kode == "-" {
		row.Diabaikan = true
	} else {
		ada, err := kodeMasterAda(ctx, db, jenis, kode)
		if err != nil {
			return err
		}
		if !ada {
			return fmt.Errorf("kode master %q tidak ada untuk jenis %s", kode, jenis)
		}
		row.KodeMaster = &kode
	}

	err = db.WithContext(ctx).Exec(`
		INSERT INTO pemetaan_sumber (sumber_id, jenis, alias, kode_master, diabaikan, catatan)
		VALUES (?, ?, ?, ?, ?, 'diatur lewat cmd/ingest')
		ON CONFLICT (sumber_id, jenis, alias) DO UPDATE
		   SET kode_master = EXCLUDED.kode_master, diabaikan = EXCLUDED.diabaikan, catatan = EXCLUDED.catatan`,
		row.SumberID, row.Jenis, row.Alias, row.KodeMaster, row.Diabaikan).Error
	if err != nil {
		return fmt.Errorf("simpan pemetaan: %w", err)
	}
	catatAudit(ctx, db, "UBAH_PEMETAAN_SUMBER", s.ID, map[string]any{"sumber": s.Kode, "jenis": jenis, "alias": alias, "kode": kode})
	fmt.Fprintf(out, "pemetaan %s %s %q -> %s\n", s.Kode, jenis, alias, kode)
	return nil
}

// kodeMasterAda memeriksa bahwa kode tujuan pemetaan ada di master.
func kodeMasterAda(ctx context.Context, db *gorm.DB, jenis, kode string) (bool, error) {
	query := `SELECT EXISTS (SELECT 1 FROM komoditas WHERE kode = ?)`
	args := []any{kode}
	if jenis == domain.JenisWilayah {
		query = `SELECT EXISTS (SELECT 1 FROM kabupaten WHERE kode = ?) OR EXISTS (SELECT 1 FROM provinsi WHERE kode = ?)`
		args = []any{kode, kode}
	}
	var ada bool
	if err := db.WithContext(ctx).Raw(query, args...).Scan(&ada).Error; err != nil {
		return false, fmt.Errorf("periksa kode master: %w", err)
	}
	return ada, nil
}

func konfigurasi(ctx context.Context, db *gorm.DB, args []string, out io.Writer) error {
	if len(args) != 2 {
		return errors.New("konfigurasi: ingest konfigurasi <sumber> <json>")
	}
	var objek map[string]json.RawMessage
	if err := json.Unmarshal([]byte(args[1]), &objek); err != nil {
		return fmt.Errorf("konfigurasi harus objek JSON: %w", err)
	}
	s, err := postgres.New(db).Sumber(ctx, args[0])
	if err != nil {
		return err
	}
	if err := db.WithContext(ctx).Exec(`UPDATE sumber_data SET konfigurasi = ?::jsonb WHERE id = ?`, args[1], s.ID).Error; err != nil {
		return fmt.Errorf("simpan konfigurasi: %w", err)
	}
	catatAudit(ctx, db, "UBAH_KONFIGURASI_SUMBER", s.ID, map[string]any{"sumber": s.Kode, "kunci": sortedKeys(objek)})
	fmt.Fprintf(out, "konfigurasi %s diperbarui (%d kunci)\n", s.Kode, len(objek))
	return nil
}

func sortedKeys(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// catatAudit mencatat aksi operator CLI (tanpa aktor terautentikasi) atas sumber ber-ID (UUID) itu. Kegagalan
// hanya dilaporkan ke stderr karena aksinya sudah terjadi.
func catatAudit(ctx context.Context, db *gorm.DB, aksi, sumberID string, metadata map[string]any) {
	metadata["lewat"] = "cmd/ingest"
	if operator := os.Getenv("USER"); operator != "" {
		metadata["operator"] = operator
	}
	err := audit.NewWriter(db).Write(ctx, audit.Entry{
		Aksi: aksi, Entitas: "sumber_data", EntitasID: sumberID, Hasil: audit.HasilBerhasil, Metadata: metadata,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "ingest: gagal mencatat audit: %v\n", err)
	}
}
