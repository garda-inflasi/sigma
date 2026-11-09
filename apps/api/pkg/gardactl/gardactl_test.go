package gardactl

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/ingestion/ingestiontest"
	"github.com/garda-inflasi/api/internal/master"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/pkg/seed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var pg *pgtest.Server

func TestMain(m *testing.M) {
	os.Exit(pgtest.Main(m, &pg))
}

func env(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

// ── Routing & Lazy Database Connection Tests ─────────────────────────────

func TestRun_TopLevelRouting(t *testing.T) {
	ctx := context.Background()

	t.Run("tanpa argumen ditolak", func(t *testing.T) {
		err := Run(ctx, nil, Options{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "perintah wajib: gardactl")
	})

	t.Run("perintah tidak dikenal ditolak", func(t *testing.T) {
		err := Run(ctx, []string{"tak-dikenal"}, Options{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "tidak dikenal")
	})

	t.Run("bantuan berhasil", func(t *testing.T) {
		var out bytes.Buffer
		err := Run(ctx, []string{"--help"}, Options{Out: &out})
		require.NoError(t, err)
		assert.Contains(t, out.String(), "gardactl - CLI Utilitas Terpadu")
	})
}

func TestRun_LazyDatabaseConnection(t *testing.T) {
	ctx := context.Background()

	t.Run("master impor-kecamatan tidak membutuhkan DATABASE_URL", func(t *testing.T) {
		err := Run(ctx, []string{"master", "impor-kecamatan", "-bendera-asing"}, Options{
			Getenv: env(nil),
		})
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "DATABASE_URL")
	})

	t.Run("master tanpa sub-perintah ditolak tanpa menyentuh DB", func(t *testing.T) {
		err := Run(ctx, []string{"master"}, Options{Getenv: env(nil)})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "sub-perintah master wajib")
	})

	t.Run("master sub-perintah asing ditolak", func(t *testing.T) {
		err := Run(ctx, []string{"master", "asing"}, Options{Getenv: env(nil)})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "tidak dikenal")
	})

	t.Run("ingest tanpa argumen ditolak sebelum menyentuh DB", func(t *testing.T) {
		err := Run(ctx, []string{"ingest"}, Options{Getenv: env(nil)})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "perintah wajib")
		assert.NotContains(t, err.Error(), "DATABASE_URL")
	})

	t.Run("ingest dengan sumber membutuhkan DATABASE_URL", func(t *testing.T) {
		err := Run(ctx, []string{"ingest", "SRC-BI"}, Options{Getenv: env(nil)})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "DATABASE_URL")
	})

	t.Run("kpi dengan argumen tidak valid ditolak sebelum menyentuh DB", func(t *testing.T) {
		for _, args := range [][]string{
			{"kpi", "-bulan", "0"},
			{"kpi", "-bulan", "37"},
			{"kpi", "-bulan", "abc"},
			{"kpi", "-tak-dikenal"},
			{"kpi", "hitung", "-bulan", "0"},
		} {
			err := Run(ctx, args, Options{Getenv: env(nil), Now: time.Now()})
			assert.Error(t, err, "%v", args)
			assert.NotContains(t, err.Error(), "DATABASE_URL")
		}
	})

	t.Run("kpi membutuhkan DATABASE_URL bila argumen valid", func(t *testing.T) {
		err := Run(ctx, []string{"kpi", "hitung"}, Options{Getenv: env(nil), Now: time.Now()})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "DATABASE_URL")
	})
}

// ── Ingest Tests (Database & Logic) ──────────────────────────────────────

func siapkanDBIngest(t *testing.T, name string) (*gorm.DB, func(args ...string) (string, error), *ingestiontest.Fetcher) {
	t.Helper()
	db := pg.NewDatabase(t, name)
	pgtest.MigrateUp(t, db)
	require.NoError(t, seed.SeedReference(t.Context(), db))
	grid, err := os.ReadFile("../../internal/ingestion/connector/pihps/testdata/grid_kota.json")
	require.NoError(t, err)
	fetcher := &ingestiontest.Fetcher{Respons: map[string][]byte{"wilayah=101": grid}}

	jalan := func(args ...string) (string, error) {
		var out bytes.Buffer
		fullArgs := append([]string{"ingest"}, args...)
		err := Run(t.Context(), fullArgs, Options{
			Getenv:  env(map[string]string{"DATABASE_URL": pg.DSN(name)}),
			Out:     &out,
			Fetcher: fetcher,
		})
		return out.String(), err
	}
	return db, jalan, fetcher
}

func TestIngest_KonfigurasiDanPemetaanLewatCLI(t *testing.T) {
	db, jalan, fetcher := siapkanDBIngest(t, "cmd_gardactl_ingest_alur")

	out, err := jalan("SRC-BI")
	require.Error(t, err)
	assert.Contains(t, out, "status=GAGAL")
	assert.Contains(t, out, "province_id")

	out, err = jalan("konfigurasi", "SRC-BI", `{"province_id":"7"}`)
	require.NoError(t, err, out)
	out, err = jalan("pemetaan", "SRC-BI", "wilayah", "101", "1171")
	require.NoError(t, err, out)
	assert.Contains(t, out, `"101" -> 1171`)

	out, err = jalan("jalankan", "SRC-BI")
	require.NoError(t, err, out)
	assert.Contains(t, out, "status=SUKSES")
	assert.Contains(t, out, "diambil=14")
	assert.Contains(t, out, "tak_terpetakan=3")
	assert.Equal(t, []string{"wilayah=101"}, fetcher.Dipanggil())

	var n int64
	require.NoError(t, db.Table("harga_eksternal").Count(&n).Error)
	assert.EqualValues(t, 14, n)
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM eksekusi_sumber WHERE pemicu = 'CLI'`).Scan(&n).Error)
	assert.EqualValues(t, 2, n)

	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM audit_log WHERE aksi IN ('UBAH_KONFIGURASI_SUMBER', 'UBAH_PEMETAAN_SUMBER', 'JALANKAN_SUMBER')`).Scan(&n).Error)
	assert.EqualValues(t, 4, n)
}

func TestIngest_PemetaanMenolakMasukanSalah(t *testing.T) {
	_, jalan, _ := siapkanDBIngest(t, "cmd_gardactl_ingest_pemetaan")

	for nama, args := range map[string][]string{
		"jumlah argumen":           {"pemetaan", "SRC-BI", "WILAYAH", "101"},
		"jenis asing":              {"pemetaan", "SRC-BI", "DESA", "x", "1171"},
		"alias kosong":             {"pemetaan", "SRC-BI", "WILAYAH", "   ", "1171"},
		"kode tak ada":             {"pemetaan", "SRC-BI", "WILAYAH", "101", "9999"},
		"komoditas asing":          {"pemetaan", "SRC-BI", "KOMODITAS", "ikan", "XXX"},
		"sumber tak ada":           {"pemetaan", "SRC-ASING", "WILAYAH", "101", "1171"},
		"konfigurasi bukan objek":  {"konfigurasi", "SRC-BI", `[1]`},
		"konfigurasi sumber asing": {"konfigurasi", "SRC-ASING", `{}`},
		"sumber tak diberikan":     {"jalankan"},
	} {
		_, err := jalan(args...)
		assert.Error(t, err, nama)
	}

	out, err := jalan("pemetaan", "SRC-BI", "KOMODITAS", "  Ikan   SEGAR Baru ", "-")
	require.NoError(t, err, out)
	assert.Contains(t, out, `"ikan segar baru" -> -`)
}

func TestIngest_DaftarMenampilkanKetersediaanKonektor(t *testing.T) {
	_, jalan, _ := siapkanDBIngest(t, "cmd_gardactl_ingest_daftar")

	out, err := jalan("daftar")

	require.NoError(t, err)
	assert.Regexp(t, `SRC-BI\s+\S+\s+konektor tersedia`, out)
	assert.Regexp(t, `SRC-DJPB\s+\S+\s+tanpa konektor`, out)
}

func TestIngest_SumberTanpaKonektorDitolak(t *testing.T) {
	_, jalan, _ := siapkanDBIngest(t, "cmd_gardactl_ingest_tanpa")

	_, err := jalan("SRC-DJPB")
	assert.ErrorContains(t, err, "konektor")
	_, err = jalan("SRC-ASING")
	assert.ErrorContains(t, err, "tidak ditemukan")
}

// ── Master Impor-Kecamatan Tests ─────────────────────────────────────────

func TestHurufJudul(t *testing.T) {
	assert.Equal(t, "Kuta Alam", hurufJudul("KUTA ALAM"))
	assert.Equal(t, "Grong-Grong", hurufJudul("GRONG-GRONG"))
	assert.Equal(t, "T. Jambo Aye", hurufJudul("T. JAMBO AYE"))
	assert.Equal(t, "Meuraxa", hurufJudul("meuraxa"))
}

func TestUbahKecamatan(t *testing.T) {
	t.Run("nama_dagri dipakai bila ada, selain itu nama_bps berhuruf judul", func(t *testing.T) {
		rows, err := ubahKecamatan("1171", []wilayahBPS{
			{KodeBPS: "1171010", NamaBPS: "MEURAXA", KodeDagri: "117103", NamaDagri: "Meuraxa"},
			{KodeBPS: "1171020", NamaBPS: "BAITURRAHMAN", KodeDagri: "", NamaDagri: "  "},
			{KodeBPS: "1171030", NamaBPS: "KUTA   ALAM", NamaDagri: "Kuta  Alam"},
		})
		require.NoError(t, err)
		require.Len(t, rows, 3)
		assert.Equal(t, "Meuraxa", rows[0].Nama)
		require.NotNil(t, rows[0].KodeDagri)
		assert.Equal(t, "117103", *rows[0].KodeDagri)
		assert.Equal(t, "Baiturrahman", rows[1].Nama)
		assert.Nil(t, rows[1].KodeDagri)
		assert.Equal(t, "Kuta Alam", rows[2].Nama, "spasi ganda dirapikan")
		assert.Equal(t, "1171", rows[2].KabupatenKode)
	})

	t.Run("jawaban kosong ditolak", func(t *testing.T) {
		_, err := ubahKecamatan("1171", nil)
		assert.Error(t, err)
	})

	t.Run("kode di luar kabupaten, ganda, atau salah bentuk ditolak", func(t *testing.T) {
		for name, items := range map[string][]wilayahBPS{
			"awalan lain": {{KodeBPS: "1101010", NamaBPS: "X"}},
			"ganda":       {{KodeBPS: "1171010", NamaBPS: "A"}, {KodeBPS: "1171010", NamaBPS: "B"}},
			"enam digit":  {{KodeBPS: "117101", NamaBPS: "X"}},
			"tanpa nama":  {{KodeBPS: "1171010"}},
		} {
			_, err := ubahKecamatan("1171", items)
			assert.Error(t, err, name)
		}
	})
}

func serverBPS(t *testing.T, gagalDulu int32) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var panggilan atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := panggilan.Add(1)
		assert.Equal(t, userAgent, r.UserAgent())
		assert.Equal(t, "kecamatan", r.URL.Query().Get("level"))
		assert.Equal(t, "2024_1.2022", r.URL.Query().Get("periode_merge"))
		if n <= gagalDulu {
			http.Error(w, "gangguan", http.StatusBadGateway)
			return
		}
		parent := r.URL.Query().Get("parent")
		_ = json.NewEncoder(w).Encode([]wilayahBPS{{KodeBPS: parent + "010", NamaBPS: "KECAMATAN " + parent, KodeDagri: "900001"}})
	}))
	t.Cleanup(srv.Close)
	return srv, &panggilan
}

func konfigUji(t *testing.T, base string) konfig {
	t.Helper()
	return konfig{
		out: filepath.Join(t.TempDir(), "kecamatan.csv"), base: base, periode: "2024_1.2022",
		timeout: 5 * time.Second, jeda: 0,
		sekarang: func() time.Time { return time.Date(2026, 11, 7, 23, 0, 0, 0, time.UTC) },
	}
}

func TestAmbilKecamatan_UlangSekali(t *testing.T) {
	t.Run("gagal sekali lalu berhasil", func(t *testing.T) {
		srv, panggilan := serverBPS(t, 1)
		items, err := ambilKecamatan(t.Context(), srv.Client(), konfigUji(t, srv.URL), "1171")
		require.NoError(t, err)
		assert.Len(t, items, 1)
		assert.EqualValues(t, 2, panggilan.Load())
	})

	t.Run("gagal dua kali menyerah", func(t *testing.T) {
		srv, panggilan := serverBPS(t, 5)
		_, err := ambilKecamatan(t.Context(), srv.Client(), konfigUji(t, srv.URL), "1171")
		require.Error(t, err)
		assert.EqualValues(t, 2, panggilan.Load(), "tepat dua percobaan")
	})

	t.Run("bukan JSON daftar dianggap gagal", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("<html>")) }))
		defer srv.Close()
		_, err := ambilSekali(t.Context(), srv.Client(), konfigUji(t, srv.URL), "1171")
		assert.Error(t, err)
	})
}

func TestImpor_MenulisCSVYangDiterimaSeeder(t *testing.T) {
	srv, panggilan := serverBPS(t, 0)
	cfg := konfigUji(t, srv.URL)

	var out bytes.Buffer
	require.NoError(t, impor(t.Context(), cfg, srv.Client(), &out))
	assert.EqualValues(t, 23, panggilan.Load(), "satu permintaan per kabupaten/kota, berurutan")

	raw, err := os.ReadFile(cfg.out)
	require.NoError(t, err)
	meta, rows, err := master.BacaKecamatanCSV(bytes.NewReader(raw), master.KodeKabupaten())
	require.NoError(t, err)
	assert.Len(t, rows, 23)
	assert.Equal(t, 23, meta.Jumlah)
	assert.Equal(t, "2024_1.2022", meta.PeriodeMerge)
	assert.Equal(t, "2026-11-07", meta.Diambil)
	assert.Equal(t, "Kecamatan 1101", rows[0].Nama)

	cfg2 := cfg
	cfg2.out = filepath.Join(t.TempDir(), "lagi.csv")
	require.NoError(t, impor(t.Context(), cfg2, srv.Client(), &bytes.Buffer{}))
	lagi, err := os.ReadFile(cfg2.out)
	require.NoError(t, err)
	assert.Equal(t, string(raw), string(lagi))
}

func TestImpor_GagalTidakMerusakBerkasLama(t *testing.T) {
	srv, _ := serverBPS(t, 1000)
	cfg := konfigUji(t, srv.URL)
	require.NoError(t, os.WriteFile(cfg.out, []byte("lama"), 0o644))

	err := impor(t.Context(), cfg, srv.Client(), &bytes.Buffer{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "1101")

	isi, readErr := os.ReadFile(cfg.out)
	require.NoError(t, readErr)
	assert.Equal(t, "lama", string(isi))
}

func TestImpor_BerhentiBilaKonteksDibatalkan(t *testing.T) {
	srv, _ := serverBPS(t, 0)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := impor(ctx, konfigUji(t, srv.URL), srv.Client(), &bytes.Buffer{})
	assert.Error(t, err)
}

// konfigurasiTersimpan membaca sumber_data.konfigurasi sebagai peta kunci -> JSON mentah.
func konfigurasiTersimpan(t *testing.T, db *gorm.DB, kode string) map[string]json.RawMessage {
	t.Helper()
	var teks string
	require.NoError(t, db.Raw(`SELECT konfigurasi::text FROM sumber_data WHERE kode = ?`, kode).Scan(&teks).Error)
	var m map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(teks), &m))
	return m
}

func TestIngest_KonfigurasiMenggabungBukanMengganti(t *testing.T) {
	db, jalan, _ := siapkanDBIngest(t, "cmd_gardactl_ingest_gabung")

	// Seed mengisi SRC-BPS dengan {"domain":"1100"}. Operator hanya menambah id variabel dan domain harus bertahan.
	awal := konfigurasiTersimpan(t, db, "SRC-BPS")
	require.JSONEq(t, `"1100"`, string(awal["domain"]))

	out, err := jalan("konfigurasi", "SRC-BPS", `{"variabel":{"ihk":"1","inflasi_mtm":"2"}}`)
	require.NoError(t, err, out)
	assert.Contains(t, out, "2 kunci: domain, variabel")
	assert.Contains(t, out, "peringatan:", "variabel belum lengkap: operator diberi tahu apa yang kurang")
	assert.Contains(t, out, "inflasi_yoy")
	got := konfigurasiTersimpan(t, db, "SRC-BPS")
	assert.JSONEq(t, `"1100"`, string(got["domain"]), "domain dari seed tidak hilang")

	out, err = jalan("konfigurasi", "SRC-BPS", `{"variabel":{"ihk":"1","inflasi_mtm":"2","inflasi_yoy":"3","inflasi_ytd":"4"},"tahun_surut":2}`)
	require.NoError(t, err, out)
	assert.NotContains(t, out, "peringatan", "konfigurasi sudah lengkap")
	got = konfigurasiTersimpan(t, db, "SRC-BPS")
	assert.JSONEq(t, `"1100"`, string(got["domain"]))
	assert.JSONEq(t, `{"ihk":"1","inflasi_mtm":"2","inflasi_yoy":"3","inflasi_ytd":"4"}`, string(got["variabel"]))
	assert.JSONEq(t, `2`, string(got["tahun_surut"]))

	t.Run("null menghapus kunci", func(t *testing.T) {
		out, err := jalan("konfigurasi", "SRC-BPS", `{"tahun_surut":null}`)
		require.NoError(t, err, out)
		got := konfigurasiTersimpan(t, db, "SRC-BPS")
		assert.NotContains(t, got, "tahun_surut")
		assert.Contains(t, got, "variabel")
	})

	t.Run("-ganti mengganti seluruhnya", func(t *testing.T) {
		out, err := jalan("konfigurasi", "-ganti", "SRC-BPS", `{"domain":"1100"}`)
		require.NoError(t, err, out)
		got := konfigurasiTersimpan(t, db, "SRC-BPS")
		assert.Len(t, got, 1)
		assert.Contains(t, out, "peringatan:", "variabel hilang lagi")
	})

	t.Run("audit mencatat nama kunci dan mode, tanpa nilai", func(t *testing.T) {
		var meta string
		require.NoError(t, db.Raw(`SELECT metadata::text FROM audit_log WHERE aksi = 'UBAH_KONFIGURASI_SUMBER' ORDER BY waktu, id LIMIT 1`).Scan(&meta).Error)
		assert.Contains(t, meta, `"mode": "gabung"`)
		assert.Contains(t, meta, "variabel")
		assert.NotContains(t, meta, "1100", "nilai konfigurasi tidak boleh masuk audit")
	})
}

func TestIngest_KonfigurasiMenolakKunciSalahKetik(t *testing.T) {
	db, jalan, _ := siapkanDBIngest(t, "cmd_gardactl_ingest_kunci_asing")
	_, err := jalan("konfigurasi", "SRC-BI", `{"province_id":"7"}`)
	require.NoError(t, err)

	_, err = jalan("konfigurasi", "SRC-BI", `{"provinsi_id":"9"}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "provinsi_id")
	assert.Contains(t, err.Error(), "province_id", "menyebut kunci yang dikenal")
	assert.JSONEq(t, `"7"`, string(konfigurasiTersimpan(t, db, "SRC-BI")["province_id"]), "konfigurasi tidak berubah")

	out, err := jalan("konfigurasi", "SRC-BI", `{"province_id":"Aceh"}`)
	require.NoError(t, err, out)
	assert.Contains(t, out, "bukan id angka", "id PIHPS harus angka; disimpan tetapi dengan peringatan")

	for nama, args := range map[string][]string{
		"null di puncak":     {"konfigurasi", "SRC-BI", `null`},
		"tanpa argumen":      {"konfigurasi"},
		"flag tak dikenal":   {"konfigurasi", "-x", "SRC-BI", `{}`},
		"metrik BPS asing":   {"konfigurasi", "SRC-BPS", `{"varibel":{}}`},
		"sumber tanpa objek": {"konfigurasi", "SRC-BI", `"7"`},
	} {
		_, err := jalan(args...)
		assert.Error(t, err, nama)
	}

	t.Run("nama metrik asing di variabel BPS diberi peringatan", func(t *testing.T) {
		out, err := jalan("konfigurasi", "SRC-BPS", `{"variabel":{"ihk":"1","inflasi_mom":"2","inflasi_yoy":"3","inflasi_ytd":"4","inflasi_mtm":"5"}}`)
		require.NoError(t, err, out)
		assert.Contains(t, out, "inflasi_mom")
	})
}
