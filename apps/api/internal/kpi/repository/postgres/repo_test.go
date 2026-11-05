package postgres_test

import (
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
			tipe text, kode_pedagang text, status text, pasar_id text, komoditas_kode text, dicatat_pada timestamptz)`).Error)
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
	assert.Equal(t, 6, dihitung)

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
