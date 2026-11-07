package postgres_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/kpi/domain"
	postgres "github.com/garda-inflasi/api/internal/kpi/repository/postgres"
	kpiseeder "github.com/garda-inflasi/api/internal/kpi/seeder"
	masterseeder "github.com/garda-inflasi/api/internal/master/seeder"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var pg *pgtest.Server

func TestMain(m *testing.M) {
	os.Exit(pgtest.Main(m, &pg))
}

var wib = time.FixedZone("WIB", 7*60*60)

// Oktober 2026 menurut WIB; asof di akhir bulan kecuali disebut lain.
func oktober() domain.Jendela {
	mulai := time.Date(2026, 10, 1, 0, 0, 0, 0, wib)
	akhir := time.Date(2026, 11, 1, 0, 0, 0, 0, wib)
	return domain.Jendela{Mulai: mulai, Akhir: akhir, AsOf: akhir}
}

func newDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db := pg.NewDatabase(t, name)
	pgtest.MigrateUp(t, db)
	_, err := masterseeder.SeedReference(t.Context(), db)
	require.NoError(t, err)
	return db
}

// denganTabelSumber memakai satu koneksi yang entri_harga dan intervensi-nya
// digantikan tabel sementara berkolom minimal, sehingga tes hanya bergantung pada
// kolom yang dibaca penghitung. Tabel master dan cold storage tetap yang asli.
func denganTabelSumber(t *testing.T, name string, fn func(repo *postgres.Repository, tx *gorm.DB)) {
	t.Helper()
	db := newDB(t, name)
	err := db.Connection(func(tx *gorm.DB) error {
		require.NoError(t, tx.Exec(`CREATE TEMP TABLE entri_harga (
			tipe text, kode_pedagang text, status text, pasar_id text, komoditas_kode text, harga int, dicatat_pada timestamptz)`).Error)
		require.NoError(t, tx.Exec(`CREATE TEMP TABLE intervensi (rekomendasi_ai boolean, tahap text, dibuat_pada timestamptz)`).Error)
		fn(postgres.New(tx), tx)
		return nil
	})
	require.NoError(t, err)
}

func hitung(t *testing.T, repo *postgres.Repository, query string, w domain.Jendela) *float64 {
	t.Helper()
	nilai, err := repo.Hitung(t.Context(), query, w)
	require.NoError(t, err, query)
	return nilai
}

func TestHitung_EveryCatalogQueryRunsAgainstTheRealSchema(t *testing.T) {
	db := newDB(t, "kpi_repo_schema")
	_, err := kpiseeder.SeedReference(t.Context(), db)
	require.NoError(t, err)
	repo := postgres.New(db)
	defs, err := repo.Definisi(t.Context())
	require.NoError(t, err)

	var dihitung int
	for _, def := range defs {
		if def.SumberQuery == nil {
			continue
		}
		dihitung++
		assert.Nil(t, hitung(t, repo, *def.SumberQuery, oktober()), "%s: tanpa data harus nil, bukan 0 atau galat", def.Kode)
	}
	assert.Equal(t, 11, dihitung)

	_, err = repo.Hitung(t.Context(), "tidak_ada", oktober())
	assert.ErrorIs(t, err, domain.ErrQueryTakDikenal)
}

func TestHitung_ProdusenTerverifikasi(t *testing.T) {
	denganTabelSumber(t, "kpi_repo_produsen", func(repo *postgres.Repository, tx *gorm.DB) {
		require.NoError(t, tx.Exec(`INSERT INTO entri_harga (tipe, kode_pedagang, status, dicatat_pada) VALUES
			('PRODUSEN', 'A', 'DIVALIDASI',  '2026-10-05 03:00+00'),
			('PRODUSEN', 'B', 'DITOLAK',     '2026-10-06 03:00+00'),
			('PRODUSEN', 'C', 'DIPUBLIKASI', '2026-10-07 03:00+00'),
			('PRODUSEN', 'C', 'DRAF',        '2026-10-08 03:00+00'),
			('KONSUMEN', 'D', 'DIVALIDASI',  '2026-10-09 03:00+00'),
			('PRODUSEN', NULL, 'DIVALIDASI', '2026-10-10 03:00+00'),
			('PRODUSEN', 'E', 'DIVALIDASI',  '2026-11-02 03:00+00')`).Error)

		nilai := hitung(t, repo, domain.QProdusenTerverifikasi, oktober())

		require.NotNil(t, nilai)
		assert.InDelta(t, 100.0*2/3, *nilai, 1e-3, "A dan C terverifikasi dari A, B, C; konsumen, tanpa kode, dan November tak dihitung")
	})
}

func TestHitung_KetepatanData(t *testing.T) {
	denganTabelSumber(t, "kpi_repo_ketepatan", func(repo *postgres.Repository, tx *gorm.DB) {
		require.NoError(t, tx.Exec(`INSERT INTO entri_harga (status, dicatat_pada) VALUES
			('DIVALIDASI', '2026-10-05 03:00+00'), ('DIVALIDASI', '2026-10-05 04:00+00'), ('DIVALIDASI', '2026-10-05 05:00+00'),
			('DIPUBLIKASI', '2026-10-06 03:00+00'), ('DITOLAK', '2026-10-07 03:00+00'),
			('DIAJUKAN', '2026-10-08 03:00+00'), ('DRAF', '2026-10-08 04:00+00')`).Error)

		nilai := hitung(t, repo, domain.QKetepatanData, oktober())

		require.NotNil(t, nilai)
		assert.InDelta(t, 80, *nilai, 1e-9, "4 lolos dari 5 yang sudah diputuskan; diajukan dan draf belum diputuskan")
	})
}

func TestHitung_KetepatanDataNilWhenNothingDecided(t *testing.T) {
	denganTabelSumber(t, "kpi_repo_ketepatan_kosong", func(repo *postgres.Repository, tx *gorm.DB) {
		require.NoError(t, tx.Exec(`INSERT INTO entri_harga (status, dicatat_pada) VALUES ('DIAJUKAN', '2026-10-05 03:00+00')`).Error)

		assert.Nil(t, hitung(t, repo, domain.QKetepatanData, oktober()))
	})
}

func TestHitung_KelengkapanHarian(t *testing.T) {
	denganTabelSumber(t, "kpi_repo_kelengkapan", func(repo *postgres.Repository, tx *gorm.DB) {
		// Hari 1 (1 Okt WIB): semua komoditas pilot di 2 pasar pertama. Entri ganda dan entri
		// pukul 00.30 WIB (= 30 September 17.30 UTC) ikut hari 1.
		require.NoError(t, tx.Exec(`
			INSERT INTO entri_harga (status, pasar_id, komoditas_kode, dicatat_pada)
			SELECT 'DIVALIDASI', p.id, k.kode, '2026-09-30 17:30+00'
			  FROM (SELECT id FROM pasar ORDER BY id LIMIT 2) p CROSS JOIN komoditas k WHERE k.pilot`).Error)
		require.NoError(t, tx.Exec(`
			INSERT INTO entri_harga (status, pasar_id, komoditas_kode, dicatat_pada)
			SELECT 'DIPUBLIKASI', p.id, k.kode, '2026-10-01 05:00+00'
			  FROM (SELECT id FROM pasar ORDER BY id LIMIT 1) p CROSS JOIN komoditas k WHERE k.pilot`).Error)
		// Hari 3 (3 Okt): 4 pasar pertama. Hari 2: kosong.
		require.NoError(t, tx.Exec(`
			INSERT INTO entri_harga (status, pasar_id, komoditas_kode, dicatat_pada)
			SELECT 'DIVALIDASI', p.id, k.kode, '2026-10-03 03:00+00'
			  FROM (SELECT id FROM pasar ORDER BY id LIMIT 4) p CROSS JOIN komoditas k WHERE k.pilot`).Error)
		// Tidak boleh dihitung: draf, komoditas non-pilot, dan hari setelah asof (5 Okt).
		require.NoError(t, tx.Exec(`
			INSERT INTO entri_harga (status, pasar_id, komoditas_kode, dicatat_pada)
			SELECT 'DRAF', p.id, k.kode, '2026-10-02 03:00+00' FROM pasar p CROSS JOIN komoditas k WHERE k.pilot`).Error)
		require.NoError(t, tx.Exec(`
			INSERT INTO entri_harga (status, pasar_id, komoditas_kode, dicatat_pada)
			SELECT 'DIVALIDASI', p.id, k.kode, '2026-10-02 03:00+00' FROM pasar p CROSS JOIN komoditas k WHERE NOT k.pilot`).Error)
		require.NoError(t, tx.Exec(`
			INSERT INTO entri_harga (status, pasar_id, komoditas_kode, dicatat_pada)
			SELECT 'DIVALIDASI', p.id, k.kode, '2026-10-05 03:00+00' FROM pasar p CROSS JOIN komoditas k WHERE k.pilot`).Error)

		var pilot, pasar int
		require.NoError(t, tx.Raw(`SELECT COUNT(*) FROM komoditas WHERE pilot`).Scan(&pilot).Error)
		require.NoError(t, tx.Raw(`SELECT COUNT(*) FROM pasar WHERE aktif`).Scan(&pasar).Error)
		require.Positive(t, pilot)
		require.GreaterOrEqual(t, pasar, 4)

		w := oktober()
		w.AsOf = time.Date(2026, 10, 3, 23, 0, 0, 0, wib) // hari 1–3 sudah berlalu
		nilai := hitung(t, repo, domain.QKelengkapanHarian, w)

		require.NotNil(t, nilai)
		want := 100.0 * float64((2*pilot)+(4*pilot)) / float64(3*pasar*pilot)
		assert.InDelta(t, want, *nilai, 1e-3)
	})
}

func TestHitung_KelengkapanHarianNilWithoutValidatedEntries(t *testing.T) {
	denganTabelSumber(t, "kpi_repo_kelengkapan_kosong", func(repo *postgres.Repository, tx *gorm.DB) {
		assert.Nil(t, hitung(t, repo, domain.QKelengkapanHarian, oktober()))
	})
}

func tambahFasilitas(t *testing.T, tx *gorm.DB) {
	t.Helper()
	require.NoError(t, tx.Exec(`
		INSERT INTO cold_storage (id, nama, kabupaten_kode, lat, lng, kapasitas_ton, suhu_target_c, status) VALUES
		('CS-A', 'A', '1171', 5.5, 95.3, 100, 4, 'OPERASIONAL'),
		('CS-B', 'B', '1171', 5.5, 95.3, 100, 4, 'OPERASIONAL'),
		('CS-C', 'C', '1171', 5.5, 95.3, 100, 4, 'PERAWATAN')`).Error)
}

func tambahLot(t *testing.T, tx *gorm.DB, id, fasilitas string, volume, risiko float64, masuk, kedaluwarsa string) {
	t.Helper()
	require.NoError(t, tx.Exec(`
		INSERT INTO stok_cold_storage (id, cold_storage_id, komoditas_kode, volume_ton, masuk_pada, pemilik, risiko_food_loss, kedaluwarsa_pada)
		VALUES (?, ?, 'CBM-KRT', ?, ?, 'Uji', ?, ?)`, id, fasilitas, volume, masuk, risiko, kedaluwarsa).Error)
}

func TestHitung_UtilisasiDanRisikoColdStorage(t *testing.T) {
	denganTabelSumber(t, "kpi_repo_coldstorage", func(repo *postgres.Repository, tx *gorm.DB) {
		tambahFasilitas(t, tx)
		tambahLot(t, tx, "L1", "CS-A", 30, 0.5, "2026-10-10 00:00+00", "2026-11-20 00:00+00")
		tambahLot(t, tx, "L2", "CS-B", 20, 0.2, "2026-09-01 00:00+00", "2026-12-01 00:00+00")
		tambahLot(t, tx, "L3", "CS-C", 50, 0.1, "2026-10-10 00:00+00", "2026-11-20 00:00+00") // perawatan
		tambahLot(t, tx, "L4", "CS-A", 40, 0.9, "2026-08-01 00:00+00", "2026-10-15 00:00+00") // sudah kedaluwarsa
		tambahLot(t, tx, "L5", "CS-A", 25, 0.9, "2026-11-10 00:00+00", "2026-12-20 00:00+00") // belum masuk

		util := hitung(t, repo, domain.QUtilisasiColdStorage, oktober())
		risiko := hitung(t, repo, domain.QRisikoFoodLoss, oktober())

		require.NotNil(t, util)
		assert.InDelta(t, 25, *util, 1e-9, "lot aktif di fasilitas operasional 30+20 dari kapasitas 200")
		require.NotNil(t, risiko)
		assert.InDelta(t, 24, *risiko, 1e-9, "tertimbang volume atas semua lot aktif: (15+4+5)/100")
	})
}

func TestHitung_ColdStorageNilBeforeFirstLot(t *testing.T) {
	denganTabelSumber(t, "kpi_repo_coldstorage_awal", func(repo *postgres.Repository, tx *gorm.DB) {
		tambahFasilitas(t, tx)
		tambahLot(t, tx, "L1", "CS-A", 30, 0.5, "2026-10-10 00:00+00", "2026-11-20 00:00+00")
		september := domain.Jendela{
			Mulai: time.Date(2026, 9, 1, 0, 0, 0, 0, wib), Akhir: time.Date(2026, 10, 1, 0, 0, 0, 0, wib),
		}
		september.AsOf = september.Akhir

		assert.Nil(t, hitung(t, repo, domain.QUtilisasiColdStorage, september), "belum ada stok: tidak ada nilai, bukan 0%")
		assert.Nil(t, hitung(t, repo, domain.QRisikoFoodLoss, september))
	})
}

func TestHitung_RekomendasiAIDieksekusi(t *testing.T) {
	denganTabelSumber(t, "kpi_repo_rekomendasi", func(repo *postgres.Repository, tx *gorm.DB) {
		require.NoError(t, tx.Exec(`INSERT INTO intervensi (rekomendasi_ai, tahap, dibuat_pada) VALUES
			(true,  't-selesai',  '2026-10-02 03:00+00'),
			(true,  'n-selesai',  '2026-10-03 03:00+00'),
			(true,  't-logistik', '2026-10-04 03:00+00'),
			(true,  's-deteksi',  '2026-10-05 03:00+00'),
			(false, 't-selesai',  '2026-10-06 03:00+00'),
			(true,  't-selesai',  '2026-11-03 03:00+00')`).Error)

		nilai := hitung(t, repo, domain.QRekomendasiAI, oktober())

		require.NotNil(t, nilai)
		assert.InDelta(t, 50, *nilai, 1e-9, "2 selesai dari 4 rekomendasi AI Oktober; non-AI dan November tak dihitung")
	})
}

func TestSimpanNilai_IsIdempotentAndVersioned(t *testing.T) {
	db := newDB(t, "kpi_repo_simpan")
	_, err := kpiseeder.SeedReference(t.Context(), db)
	require.NoError(t, err)
	repo := postgres.New(db)
	periode := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	simpan := func(v float64) domain.HasilSimpan {
		hasil, err := repo.SimpanNilai(t.Context(), "LOG-UTILISASI-CS", periode, v, time.Now())
		require.NoError(t, err)
		return hasil
	}

	assert.Equal(t, domain.Baru, simpan(80))
	assert.Equal(t, domain.Tetap, simpan(80), "nilai sama tidak menambah baris")
	assert.Equal(t, domain.Tetap, simpan(80.00001), "selisih di bawah ketelitian kolom dianggap sama")
	assert.Equal(t, domain.Direvisi, simpan(82.5))

	var semua []domain.Nilai
	require.NoError(t, db.Order("versi").Find(&semua).Error)
	require.Len(t, semua, 2, "revisi = versi baru, versi lama tetap ada")
	assert.InDelta(t, 80, semua[0].Nilai, 1e-9)
	assert.Equal(t, 2, semua[1].Versi)

	berlaku, err := repo.NilaiBerlaku(t.Context(), periode, periode, nil)
	require.NoError(t, err)
	require.Len(t, berlaku, 1)
	assert.InDelta(t, 82.5, berlaku[0].Nilai, 1e-9, "hanya versi terbesar yang berlaku")
}

func TestNilaiBerlakuDanRiwayat(t *testing.T) {
	db := newDB(t, "kpi_repo_riwayat")
	_, err := kpiseeder.SeedReference(t.Context(), db)
	require.NoError(t, err)
	repo := postgres.New(db)
	for bulan := 1; bulan <= 5; bulan++ {
		_, err := repo.SimpanNilai(t.Context(), "LOG-UTILISASI-CS", time.Date(2026, time.Month(bulan), 1, 0, 0, 0, 0, time.UTC), float64(bulan*10), time.Now())
		require.NoError(t, err)
	}
	// Nilai kabupaten tidak boleh bercampur dengan nilai provinsi.
	require.NoError(t, db.Exec(`INSERT INTO kpi_nilai (kpi_kode, wilayah_kode, periode, nilai) VALUES ('LOG-UTILISASI-CS', '1171', '2026-05-01', 99)`).Error)

	riwayat, err := repo.RiwayatKPI(t.Context(), "LOG-UTILISASI-CS", nil, 3)
	require.NoError(t, err)
	require.Len(t, riwayat, 3)
	assert.InDelta(t, 30, riwayat[0].Nilai, 1e-9, "3 periode terakhir, urut naik")
	assert.InDelta(t, 50, riwayat[2].Nilai, 1e-9)

	kota := "1171"
	riwayatKota, err := repo.RiwayatKPI(t.Context(), "LOG-UTILISASI-CS", &kota, 12)
	require.NoError(t, err)
	require.Len(t, riwayatKota, 1)
	assert.InDelta(t, 99, riwayatKota[0].Nilai, 1e-9)

	rentang, err := repo.NilaiBerlaku(t.Context(), time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), nil)
	require.NoError(t, err)
	assert.Len(t, rentang, 3)
}

func TestTargetBerlaku_PrefersRegionalThenLatestStart(t *testing.T) {
	db := newDB(t, "kpi_repo_target")
	_, err := kpiseeder.SeedReference(t.Context(), db)
	require.NoError(t, err)
	repo := postgres.New(db)
	require.NoError(t, db.Exec(`
		INSERT INTO kpi_target (kpi_kode, wilayah_kode, periode_mulai, target) VALUES
		('LOG-UTILISASI-CS', NULL,   '2026-06-01', 80),
		('LOG-UTILISASI-CS', NULL,   '2027-01-01', 90),
		('LOG-UTILISASI-CS', '1171', '2025-06-01', 70)`).Error)
	targetUtilisasi := func(periode time.Time, wilayah *string) float64 {
		targets, err := repo.TargetBerlaku(t.Context(), periode, wilayah)
		require.NoError(t, err)
		for _, tg := range targets {
			if tg.KpiKode == "LOG-UTILISASI-CS" {
				return tg.Target
			}
		}
		t.Fatal("target utilisasi tidak ditemukan")
		return 0
	}
	kota := "1171"

	assert.InDelta(t, 75, targetUtilisasi(time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), nil), 1e-9, "target bawaan seeder")
	assert.InDelta(t, 80, targetUtilisasi(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), nil), 1e-9, "periode_mulai terbaru yang sudah berlaku")
	assert.InDelta(t, 90, targetUtilisasi(time.Date(2027, 2, 1, 0, 0, 0, 0, time.UTC), nil), 1e-9)
	assert.InDelta(t, 70, targetUtilisasi(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), &kota), 1e-9, "target khusus wilayah didahulukan")
}

func TestDefinisiByKode(t *testing.T) {
	db := newDB(t, "kpi_repo_definisi")
	_, err := kpiseeder.SeedReference(t.Context(), db)
	require.NoError(t, err)
	repo := postgres.New(db)

	def, err := repo.DefinisiByKode(t.Context(), "TL-REKOMENDASI-AI")
	require.NoError(t, err)
	assert.Equal(t, domain.KelompokTindakLanjut, def.Kelompok)

	_, err = repo.DefinisiByKode(t.Context(), "TAK-ADA")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

// ── 16-B: model dan dampak ────────────────────────────────────────────

// harga menambah entri harga (kolom minimal tabel sementara).
func harga(t *testing.T, tx *gorm.DB, tipe, status, pasar, komoditas string, nilai int, waktu string) {
	t.Helper()
	require.NoError(t, tx.Exec(`INSERT INTO entri_harga (tipe, status, pasar_id, komoditas_kode, harga, dicatat_pada) VALUES (?, ?, ?, ?, ?, ?)`,
		tipe, status, pasar, komoditas, nilai, waktu).Error)
}

func TestHitung_MapeHarga(t *testing.T) {
	denganTabelSumber(t, "kpi_repo_mape", func(repo *postgres.Repository, tx *gorm.DB) {
		require.NoError(t, tx.Exec(`INSERT INTO model_registry (kode, komoditas_kode, nama, versi, mape, dilatih_pada) VALUES
			('M1', 'CBM-KRT', 'uji', 'v1', 5, now()), ('M2', 'BWM', 'uji', 'v1', 5, now())`).Error)
		// Pasar 1171 (PSR-1171-01 dan -02). Rata-rata aktual CBM-KRT 10 Okt = (50000 + 52000) / 2 = 51000.
		harga(t, tx, "KONSUMEN", "DIVALIDASI", "PSR-1171-01", "CBM-KRT", 50000, "2026-10-10 03:00+00")
		harga(t, tx, "KONSUMEN", "DIPUBLIKASI", "PSR-1171-02", "CBM-KRT", 52000, "2026-10-10 04:00+00")
		harga(t, tx, "KONSUMEN", "DIPUBLIKASI", "PSR-1171-01", "BWM", 40000, "2026-10-12 03:00+00")
		harga(t, tx, "KONSUMEN", "DIAJUKAN", "PSR-1171-01", "CBM-KRT", 99999, "2026-10-15 03:00+00")   // belum tervalidasi
		harga(t, tx, "PRODUSEN", "DIVALIDASI", "PSR-1171-01", "CBM-KRT", 99999, "2026-10-20 03:00+00") // bukan harga konsumen
		prediksi := func(id, komoditas, tanggal string, nilai int) {
			require.NoError(t, tx.Exec(`INSERT INTO prediksi_harga (kode, komoditas_kode, kabupaten_kode, tanggal_target, horizon_hari, nilai, bawah, atas, model_id, model_versi, faktor, harga_dasar)
				VALUES (?, ?, '1171', ?, 7, ?, 1, 999999, (SELECT id FROM model_registry WHERE komoditas_kode = ? AND versi = 'v1'), 'v1', '{}', 50000)`,
				id, komoditas, tanggal, nilai, komoditas).Error)
		}
		prediksi("P1", "CBM-KRT", "2026-10-10", 54060) // |54060 - 51000| / 51000 = 6,0%
		prediksi("P2", "BWM", "2026-10-12", 38000)     // |38000 - 40000| / 40000 = 5,0%
		prediksi("P3", "CBM-KRT", "2026-10-15", 50000) // aktual hanya DIAJUKAN: diabaikan
		prediksi("P4", "CBM-KRT", "2026-10-20", 50000) // aktual bukan konsumen: diabaikan
		prediksi("P5", "CBM-KRT", "2026-10-25", 50000) // tanpa aktual: diabaikan
		prediksi("P6", "CBM-KRT", "2026-11-02", 50000) // di luar bulan

		nilai := hitung(t, repo, domain.QMapeHarga, oktober())

		require.NotNil(t, nilai)
		assert.InDelta(t, 5.5, *nilai, 1e-9, "(6,0 + 5,0) / 2")
	})
}

func TestHitung_VolatilitasHarga(t *testing.T) {
	denganTabelSumber(t, "kpi_repo_volatilitas", func(repo *postgres.Repository, tx *gorm.DB) {
		// CBM-KRT di 1171: 100,110,90,100,100 -> rata-rata 100, simpangan baku sampel sqrt(200/4) = 7,0711 -> CV 7,0711%.
		for i, h := range []int{100, 110, 90, 100, 100} {
			harga(t, tx, "PRODUSEN", "DIVALIDASI", "PSR-1171-01", "CBM-KRT", h, fmt.Sprintf("2026-10-0%d 03:00+00", i+1))
		}
		harga(t, tx, "PRODUSEN", "DITOLAK", "PSR-1171-01", "CBM-KRT", 5000, "2026-10-09 03:00+00") // tak dihitung
		// BWM di 1171: hanya 4 entri, di bawah minimum 5: kelompok dilewati.
		for i, h := range []int{10, 20, 30, 40} {
			harga(t, tx, "PRODUSEN", "DIPUBLIKASI", "PSR-1171-01", "BWM", h, fmt.Sprintf("2026-10-1%d 03:00+00", i))
		}
		// CBM-KRT di 1108: harga tetap -> CV 0.
		for i := range 5 {
			harga(t, tx, "PRODUSEN", "DIVALIDASI", "PSR-1108-01", "CBM-KRT", 200, fmt.Sprintf("2026-10-2%d 03:00+00", i))
		}
		harga(t, tx, "KONSUMEN", "DIVALIDASI", "PSR-1108-01", "CBM-KRT", 1, "2026-10-21 03:00+00") // bukan harga produsen

		nilai := hitung(t, repo, domain.QVolatilitasHarga, oktober())

		require.NotNil(t, nilai)
		assert.InDelta(t, (7.0711+0)/2, *nilai, 1e-3, "rata-rata CV kelompok yang memenuhi syarat")
	})
}

func TestHitung_MarginProdusenKonsumen(t *testing.T) {
	denganTabelSumber(t, "kpi_repo_margin", func(repo *postgres.Repository, tx *gorm.DB) {
		// CBM-KRT di 1171: konsumen rata-rata 60000, produsen rata-rata 45000 -> margin 25%.
		harga(t, tx, "KONSUMEN", "DIVALIDASI", "PSR-1171-01", "CBM-KRT", 60000, "2026-10-02 03:00+00")
		harga(t, tx, "KONSUMEN", "DIVALIDASI", "PSR-1171-02", "CBM-KRT", 60000, "2026-10-03 03:00+00")
		harga(t, tx, "PRODUSEN", "DIVALIDASI", "PSR-1171-01", "CBM-KRT", 40000, "2026-10-04 03:00+00")
		harga(t, tx, "PRODUSEN", "DIPUBLIKASI", "PSR-1171-02", "CBM-KRT", 50000, "2026-10-05 03:00+00")
		// BWM di 1171: 50000 vs 40000 -> 20%.
		harga(t, tx, "KONSUMEN", "DIVALIDASI", "PSR-1171-01", "BWM", 50000, "2026-10-06 03:00+00")
		harga(t, tx, "PRODUSEN", "DIVALIDASI", "PSR-1171-01", "BWM", 40000, "2026-10-07 03:00+00")
		// CBM-KRT di 1108: hanya produsen tervalidasi -> tidak ada margin; konsumen belum tervalidasi tak dihitung.
		harga(t, tx, "PRODUSEN", "DIVALIDASI", "PSR-1108-01", "CBM-KRT", 10, "2026-10-08 03:00+00")
		harga(t, tx, "KONSUMEN", "DIAJUKAN", "PSR-1108-01", "CBM-KRT", 10000, "2026-10-08 04:00+00")

		nilai := hitung(t, repo, domain.QMarginHarga, oktober())

		require.NotNil(t, nilai)
		assert.InDelta(t, 22.5, *nilai, 1e-9, "(25 + 20) / 2")
	})
}

// transaksi menambah satu baris buku besar stok.
func transaksi(t *testing.T, tx *gorm.DB, id, lot, lotBaru, jenis string, volume float64, asal, tujuan, waktu string) {
	t.Helper()
	require.NoError(t, tx.Exec(`
		INSERT INTO transaksi_stok (id, lot_id, lot_baru_id, jenis, volume_ton, fasilitas_asal_id, fasilitas_tujuan_id, waktu)
		VALUES (?, ?, NULLIF(?, ''), ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?)`, id, lot, lotBaru, jenis, volume, asal, tujuan, waktu).Error)
}

// bukuBesar menyiapkan: L1 (risiko 0,5) masuk 80 ke CS-A 1 Okt, keluar 30 pada 20 Okt, pindah sebagian
// 10 ke CS-B (lot baru L3) pada 25 Okt; L2 (risiko 0,1) masuk 20 ke CS-B 2 Okt. Kolom stok_cold_storage
// sengaja tidak konsisten dengan buku (lot L1 "kedaluwarsa" 5 Okt): buku besar yang menentukan.
func bukuBesar(t *testing.T, tx *gorm.DB) {
	t.Helper()
	tambahFasilitas(t, tx)
	tambahLot(t, tx, "L1", "CS-A", 40, 0.5, "2026-10-01 00:00+00", "2026-10-05 00:00+00")
	tambahLot(t, tx, "L2", "CS-B", 20, 0.1, "2026-10-02 00:00+00", "2026-12-01 00:00+00")
	tambahLot(t, tx, "L3", "CS-B", 10, 0.5, "2026-10-01 00:00+00", "2026-12-01 00:00+00")
	transaksi(t, tx, "T1", "L1", "", "MASUK", 80, "", "CS-A", "2026-10-01 03:00+00")
	transaksi(t, tx, "T2", "L2", "", "MASUK", 20, "", "CS-B", "2026-10-02 03:00+00")
	transaksi(t, tx, "T3", "L1", "", "KELUAR", 30, "CS-A", "", "2026-10-20 03:00+00")
	transaksi(t, tx, "T4", "L1", "L3", "PINDAH", 10, "CS-A", "CS-B", "2026-10-25 03:00+00")
}

func TestHitung_ColdStorageUsesLedgerOnceTransactionsExist(t *testing.T) {
	denganTabelSumber(t, "kpi_repo_buku_besar", func(repo *postgres.Repository, tx *gorm.DB) {
		bukuBesar(t, tx)
		tengahBulan := oktober()
		tengahBulan.AsOf = time.Date(2026, 10, 15, 12, 0, 0, 0, wib)

		akhir := hitung(t, repo, domain.QUtilisasiColdStorage, oktober())
		tengah := hitung(t, repo, domain.QUtilisasiColdStorage, tengahBulan)
		risikoAkhir := hitung(t, repo, domain.QRisikoFoodLoss, oktober())
		risikoTengah := hitung(t, repo, domain.QRisikoFoodLoss, tengahBulan)

		require.NotNil(t, akhir)
		assert.InDelta(t, 35, *akhir, 1e-9, "CS-A 80-30-10 = 40, CS-B 20+10 = 30; kapasitas operasional 200")
		require.NotNil(t, tengah)
		assert.InDelta(t, 50, *tengah, 1e-9, "15 Okt: CS-A 80, CS-B 20")
		require.NotNil(t, risikoAkhir)
		assert.InDelta(t, 100.0*27/70, *risikoAkhir, 1e-4, "L1 40 x 0,5 + L2 20 x 0,1 + L3 10 x 0,5 dari 70 ton")
		require.NotNil(t, risikoTengah)
		assert.InDelta(t, 42, *risikoTengah, 1e-9, "15 Okt: (80 x 0,5 + 20 x 0,1) / 100")
	})
}

func TestHitung_ColdStorageNilBeforeFirstLedgerEntry(t *testing.T) {
	denganTabelSumber(t, "kpi_repo_buku_awal", func(repo *postgres.Repository, tx *gorm.DB) {
		bukuBesar(t, tx)
		september := domain.Jendela{Mulai: time.Date(2026, 9, 1, 0, 0, 0, 0, wib), Akhir: time.Date(2026, 10, 1, 0, 0, 0, 0, wib)}
		september.AsOf = september.Akhir

		assert.Nil(t, hitung(t, repo, domain.QUtilisasiColdStorage, september), "belum ada transaksi: tidak ada nilai, bukan 0%")
		assert.Nil(t, hitung(t, repo, domain.QRisikoFoodLoss, september))
	})
}

func TestHitung_VolumeAgregasi(t *testing.T) {
	denganTabelSumber(t, "kpi_repo_agregasi", func(repo *postgres.Repository, tx *gorm.DB) {
		assert.Nil(t, hitung(t, repo, domain.QVolumeAgregasi, oktober()), "tanpa transaksi: tidak ada nilai")

		bukuBesar(t, tx)
		transaksi(t, tx, "T5", "L2", "", "MASUK", 7, "", "CS-B", "2026-09-30 16:00+00") // 30 Sep 23.00 WIB: di luar Oktober
		transaksi(t, tx, "T6", "L2", "", "MASUK", 4.5, "", "CS-B", "2026-10-31 16:59+00")
		transaksi(t, tx, "T7", "L2", "", "MASUK", 9, "", "CS-B", "2026-10-31 17:00+00") // tepat 1 Nov 00.00 WIB: di luar

		nilai := hitung(t, repo, domain.QVolumeAgregasi, oktober())

		require.NotNil(t, nilai)
		assert.InDelta(t, 80+20+4.5, *nilai, 1e-9, "hanya MASUK pada Oktober menurut WIB")
	})
}

func TestHitung_EfisiensiBiayaLogistik(t *testing.T) {
	denganTabelSumber(t, "kpi_repo_efisiensi", func(repo *postgres.Repository, tx *gorm.DB) {
		assert.Nil(t, hitung(t, repo, domain.QEfisiensiBiaya, oktober()))

		rencana := func(id, komoditas, periode, dibuat string, total int64, pembanding any) {
			require.NoError(t, tx.Exec(`
				INSERT INTO optimasi_rute (id, komoditas_kode, periode, parameter, total_volume_ton, total_biaya, belum_terpenuhi_ton, biaya_pembanding, dibuat_pada)
				VALUES (?, ?, ?, '{}', 10, ?, 0, ?, ?)`, id, komoditas, periode, total, pembanding, dibuat).Error)
		}
		rencana("O1", "CBM-KRT", "2026-10-01", "2026-10-05 03:00+00", 800, 1000)  // digantikan O2 (sama komoditas dan periode)
		rencana("O2", "CBM-KRT", "2026-10-01", "2026-10-10 03:00+00", 1500, 2000) // berlaku
		rencana("O3", "BWM", "2026-10-01", "2026-10-12 03:00+00", 400, 500)       // berlaku
		rencana("O4", "CBM-KRT", "2026-09-01", "2026-10-15 03:00+00", 100, nil)   // tidak sebanding: diabaikan
		rencana("O5", "BWM", "2026-10-01", "2026-11-03 03:00+00", 1, 1000)        // November: bukan bagian Oktober dan tidak mengubah nilai Oktober

		nilai := hitung(t, repo, domain.QEfisiensiBiaya, oktober())

		require.NotNil(t, nilai)
		assert.InDelta(t, 100.0*(2000+500-1500-400)/(2000+500), *nilai, 1e-9, "O2 dan O3: (2500 - 1900) / 2500 = 24%")
	})
}
