package gardactl

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/dashboard/refresher"
	"github.com/garda-inflasi/api/internal/ingestion"
	"github.com/garda-inflasi/api/internal/ingestion/domain"
	"github.com/garda-inflasi/api/internal/ingestion/engine"
	"github.com/garda-inflasi/api/internal/ingestion/repository/postgres"
	sumberrepo "github.com/garda-inflasi/api/internal/sumber-data/repository/postgres"
	"github.com/garda-inflasi/api/pkg/logger"
	"gorm.io/gorm"
)

func runIngest(ctx context.Context, args []string, opts Options) error {
	if len(args) == 0 {
		return errors.New("perintah wajib: ingest <sumber> | daftar | pemetaan | konfigurasi")
	}
	if args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(opts.out(), `gardactl ingest - Kelola dan jalankan ingestion sumber data eksternal

Pemakaian:
  gardactl ingest [jalankan] <sumber>                          jalankan satu sumber sekali (mis. SRC-BI)
  gardactl ingest daftar                                       daftar sumber beserta status konektornya
  gardactl ingest pemetaan <sumber> <KOMODITAS|WILAYAH> <alias> <kode|->
                                                               petakan alias sumber ke kode master ("-" = abaikan alias)
  gardactl ingest konfigurasi [-ganti] <sumber> <json>         isi konfigurasi permintaan sumber (objek JSON)
                                                               digabung ke konfigurasi yang ada; nilai null menghapus kunci;
                                                               -ganti mengganti seluruh konfigurasi
                                                               mis. SRC-BI '{"province_id":"7"}'; SRC-BPS '{"variabel":{"ihk":"..."}}'`)
		return nil
	}

	db, err := connectDB(opts.getenv)
	if err != nil {
		return err
	}
	if sqlDB, err := db.DB(); err == nil {
		defer sqlDB.Close()
	}

	switch args[0] {
	case "daftar":
		return ingestDaftar(ctx, db, opts.out(), opts.Fetcher)
	case "pemetaan":
		return ingestPemetaan(ctx, db, args[1:], opts.out(), opts.getenv)
	case "konfigurasi":
		return ingestKonfigurasi(ctx, db, args[1:], opts.out(), opts.getenv)
	case "jalankan":
		args = args[1:]
	}
	if len(args) != 1 {
		return errors.New("jalankan: sebutkan tepat satu sumber (kode atau UUID), mis. ingest SRC-BI")
	}
	return ingestJalankan(ctx, db, args[0], opts.out(), opts.Fetcher, opts.getenv)
}

func newRunner(db *gorm.DB, fetcher domain.Fetcher) *engine.Runner {
	log := logger.NewDefault()
	if fetcher == nil {
		fetcher = ingestion.NewFetcher()
	}
	return engine.New(postgres.New(db), fetcher, log, ingestion.Konektor()...)
}

func ingestJalankan(ctx context.Context, db *gorm.DB, ref string, out io.Writer, fetcher domain.Fetcher, getenv func(string) string) error {
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
	catatAudit(ctx, db, "JALANKAN_SUMBER", s.ID, map[string]any{"kode": s.Kode, "eksekusi_id": e.ID, "status": e.Status}, getenv)

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

func ingestDaftar(ctx context.Context, db *gorm.DB, out io.Writer, fetcher domain.Fetcher) error {
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

func ingestPemetaan(ctx context.Context, db *gorm.DB, args []string, out io.Writer, getenv func(string) string) error {
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
		VALUES (?, ?, ?, ?, ?, 'diatur lewat gardactl ingest')
		ON CONFLICT (sumber_id, jenis, alias) DO UPDATE
		   SET kode_master = EXCLUDED.kode_master, diabaikan = EXCLUDED.diabaikan, catatan = EXCLUDED.catatan`,
		row.SumberID, row.Jenis, row.Alias, row.KodeMaster, row.Diabaikan).Error
	if err != nil {
		return fmt.Errorf("simpan pemetaan: %w", err)
	}
	catatAudit(ctx, db, "UBAH_PEMETAAN_SUMBER", s.ID, map[string]any{"sumber": s.Kode, "jenis": jenis, "alias": alias, "kode": kode}, getenv)
	fmt.Fprintf(out, "pemetaan %s %s %q -> %s\n", s.Kode, jenis, alias, kode)
	return nil
}

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

// ingestKonfigurasi mengisi sumber_data.konfigurasi. Bawaannya menggabungkan kunci tingkat atas ke konfigurasi yang
// ada (seed mengisi sebagian, mis. {"domain":"1100"} pada SRC-BPS, dan operator hanya menambah sisanya); nilai null
// menghapus kunci; -ganti mengganti seluruhnya. Nilai tidak pernah dicatat ke audit atau keluaran, hanya nama kunci.
func ingestKonfigurasi(ctx context.Context, db *gorm.DB, args []string, out io.Writer, getenv func(string) string) error {
	const pakai = "konfigurasi: ingest konfigurasi [-ganti] <sumber> <json>"
	fs := flag.NewFlagSet("konfigurasi", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	ganti := fs.Bool("ganti", false, "ganti seluruh konfigurasi, bukan menggabungkan")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%s: %w", pakai, err)
	}
	args = fs.Args()
	if len(args) != 2 {
		return errors.New(pakai)
	}
	var patch map[string]json.RawMessage
	if err := json.Unmarshal([]byte(args[1]), &patch); err != nil {
		return fmt.Errorf("konfigurasi harus objek JSON: %w", err)
	}
	if patch == nil {
		return errors.New("konfigurasi harus objek JSON, bukan null")
	}
	s, err := postgres.New(db).Sumber(ctx, args[0])
	if err != nil {
		return err
	}

	pemeriksa, _ := cariKonektor(s.Kode).(domain.PemeriksaKonfigurasi)
	if pemeriksa != nil {
		// Kunci asing pasti salah ketik (konektor mengabaikannya diam-diam). null tetap boleh: membersihkan sisa.
		dikenal := pemeriksa.KunciKonfigurasi()
		var asing []string
		for k, v := range patch {
			if string(v) != "null" && !slices.Contains(dikenal, k) {
				asing = append(asing, k)
			}
		}
		if len(asing) > 0 {
			slices.Sort(asing)
			return fmt.Errorf("kunci konfigurasi %s tidak dikenal: %s (dikenal: %s)", s.Kode, strings.Join(asing, ", "), strings.Join(dikenal, ", "))
		}
	}

	var baru map[string]json.RawMessage
	var tersimpan []byte
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		baru = map[string]json.RawMessage{}
		if !*ganti {
			var lama string
			if err := tx.Raw(`SELECT COALESCE(konfigurasi::text, '{}') FROM sumber_data WHERE id = ? FOR UPDATE`, s.ID).Scan(&lama).Error; err != nil {
				return fmt.Errorf("baca konfigurasi: %w", err)
			}
			if err := json.Unmarshal([]byte(lama), &baru); err != nil || baru == nil {
				baru = map[string]json.RawMessage{} // bukan objek (mis. null): mulai dari kosong
			}
		}
		for k, v := range patch {
			if string(v) == "null" {
				delete(baru, k)
			} else {
				baru[k] = v
			}
		}
		var err error
		if tersimpan, err = json.Marshal(baru); err != nil {
			return fmt.Errorf("susun konfigurasi: %w", err)
		}
		if err := tx.Exec(`UPDATE sumber_data SET konfigurasi = ?::jsonb WHERE id = ?`, string(tersimpan), s.ID).Error; err != nil {
			return fmt.Errorf("simpan konfigurasi: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	mode := "gabung"
	if *ganti {
		mode = "ganti"
	}
	catatAudit(ctx, db, "UBAH_KONFIGURASI_SUMBER", s.ID, map[string]any{"sumber": s.Kode, "kunci": sortedKeys(patch), "mode": mode}, getenv)
	fmt.Fprintf(out, "konfigurasi %s diperbarui (%d kunci: %s)\n", s.Kode, len(baru), strings.Join(sortedKeys(baru), ", "))
	if pemeriksa != nil {
		if err := pemeriksa.PeriksaKonfigurasi(tersimpan); err != nil {
			fmt.Fprintf(out, "peringatan: %v\n  konfigurasi tersimpan, tetapi run %s belum akan berhasil sampai dilengkapi\n", err, s.Kode)
		}
	}
	return nil
}

// cariKonektor mengembalikan konektor terdaftar untuk kode sumber, atau nil bila tidak ada.
func cariKonektor(kode string) domain.Konektor {
	for _, k := range ingestion.Konektor() {
		if k.Kode() == kode {
			return k
		}
	}
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

func catatAudit(ctx context.Context, db *gorm.DB, aksi, sumberID string, metadata map[string]any, getenv func(string) string) {
	metadata["lewat"] = "gardactl ingest"
	if getenv != nil {
		if operator := getenv("USER"); operator != "" {
			metadata["operator"] = operator
		}
	} else if operator := os.Getenv("USER"); operator != "" {
		metadata["operator"] = operator
	}
	err := audit.NewWriter(db).Write(ctx, audit.Entry{
		Aksi: aksi, Entitas: "sumber_data", EntitasID: sumberID, Hasil: audit.HasilBerhasil, Metadata: metadata,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "gardactl ingest: gagal mencatat audit: %v\n", err)
	}
}
