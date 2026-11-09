package integration

// Tes seed SQL KPI: db/seeds/reference/13_kpi_definisi.sql dan db/seeds/demo/13_kpi_rekam.sql. Menggantikan tes seeder
// Go lama (kpi/seeder) setelah seeder itu menjadi berkas SQL: bentuk definisi dan target menurut PRD §12, idempotensi,
// target yang ditetapkan pemangku kepentingan tidak ditimpa, dan nilai demo yang DIHITUNG dari data sumber (bukan angka
// tulis tangan), termasuk paritas dengan job penghitung Go.

import (
	"fmt"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/kpi/domain"
	kpipg "github.com/garda-inflasi/api/internal/kpi/repository/postgres"
	"github.com/garda-inflasi/api/internal/kpi/usecase"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/seed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	berkasDefinisiKPI = "reference/13_kpi_definisi.sql"
	berkasRekamKPI    = "demo/13_kpi_rekam.sql"
)

// seedDB membuat basis data baru bermigrasi lalu menjalankan berkas seed (relatif terhadap db/seeds) berurutan.
func seedDB(t *testing.T, name string, berkas ...string) *gorm.DB {
	t.Helper()
	db := pg.NewDatabase(t, name)
	pgtest.MigrateUp(t, db)
	require.NoError(t, seed.SeedFiles(t.Context(), db, berkas...))
	return db
}

func jumlah(t *testing.T, db *gorm.DB, tabel string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Table(tabel).Count(&n).Error)
	return n
}

func semuaNilai(t *testing.T, db *gorm.DB) []domain.Nilai {
	t.Helper()
	var rows []domain.Nilai
	require.NoError(t, db.Order("kpi_kode, periode, versi").Find(&rows).Error)
	return rows
}

func kodeBernilai(t *testing.T, db *gorm.DB) []string {
	t.Helper()
	var kode []string
	require.NoError(t, db.Model(&domain.Nilai{}).Distinct().Order("kpi_kode").Pluck("kpi_kode", &kode).Error)
	return kode
}

// hitungUlangDenganGo menjalankan job penghitung Go (gardactl kpi hitung) atas 12 bulan yang berakhir di bulan berjalan,
// jendela yang sama dengan berkas seed: bulan berjalan menurut UTC (seperti time.Now() pada container), batas bulan menurut
// WIB (di dalam job). "Sekarang" diambil dari jam basis data, yang juga dipakai berkas seed (now()), supaya selisih jam antara
// proses uji dan container tidak menggeser batas ukuran berbasis keadaan seperti stok.
func hitungUlangDenganGo(t *testing.T, db *gorm.DB) usecase.Laporan {
	t.Helper()
	uc := usecase.New(kpipg.New(db), audit.NewWriter(db), logger.NewDefault())
	var jamDB time.Time
	require.NoError(t, db.Raw(`SELECT now()`).Scan(&jamDB).Error)
	sekarang := jamDB.UTC()
	lap, err := uc.Hitung(t.Context(), usecase.AwalBulan(sekarang).AddDate(0, -11, 0), sekarang, sekarang)
	require.NoError(t, err)
	return lap
}

func TestSeedKPI_DefinitionsShape(t *testing.T) {
	db := seedDB(t, "kpi_seed_bentuk", berkasDefinisiKPI)

	var defs []domain.Definisi
	require.NoError(t, db.Order("urutan").Find(&defs).Error)

	require.Len(t, defs, 17)
	kode, kelompok, query := map[string]bool{}, map[string]int{}, map[string]bool{}
	for i, d := range defs {
		assert.False(t, kode[d.Kode], "kode ganda %s", d.Kode)
		kode[d.Kode] = true
		kelompok[d.Kelompok]++
		assert.Equal(t, (i+1)*10, d.Urutan, "urutan mengikuti daftar: %s", d.Kode)
		assert.Contains(t, []string{domain.ArahNaik, domain.ArahTurun}, d.ArahBaik, d.Kode)
		assert.Equal(t, "BULANAN", d.Frekuensi, d.Kode)
		assert.NotEmpty(t, d.Rumus, d.Kode)
		assert.NotEmpty(t, d.Deskripsi, d.Kode)
		if d.SumberQuery != nil {
			query[*d.SumberQuery] = true
		}
	}
	assert.Equal(t, map[string]int{
		domain.KelompokDataSistem: 8, domain.KelompokLogistik: 4, domain.KelompokDampakEkonomi: 3, domain.KelompokTindakLanjut: 2,
	}, kelompok, "empat kelompok sesuai blueprint")
	assert.Equal(t, map[string]bool{
		domain.QProdusenTerverifikasi: true, domain.QKetepatanData: true, domain.QKelengkapanHarian: true,
		domain.QUtilisasiColdStorage: true, domain.QRisikoFoodLoss: true, domain.QRekomendasiAI: true,
		domain.QMapeHarga: true, domain.QVolatilitasHarga: true, domain.QMarginHarga: true,
		domain.QVolumeAgregasi: true, domain.QEfisiensiBiaya: true,
	}, query, "hanya KPI yang datanya sudah ada yang punya penghitung")
}

func TestSeedKPI_TargetsFollowPRD12AndLeaveBaselineEmpty(t *testing.T) {
	db := seedDB(t, "kpi_seed_target", berkasDefinisiKPI)
	var defs []domain.Definisi
	require.NoError(t, db.Find(&defs).Error)
	arah := map[string]string{}
	for _, d := range defs {
		arah[d.Kode] = d.ArahBaik
	}

	var targets []domain.Target
	require.NoError(t, db.Find(&targets).Error)

	require.Len(t, targets, 15)
	byKode := map[string]domain.Target{}
	for _, tg := range targets {
		byKode[tg.KpiKode] = tg
		assert.Nil(t, tg.Baseline, "%s: baseline adalah keputusan bisnis, tidak boleh diisi seeder", tg.KpiKode)
		assert.Nil(t, tg.WilayahKode, tg.KpiKode)
		require.NotNil(t, tg.AmbangWaspada, tg.KpiKode)
		faktor := 0.9 // NAIK: 90% target
		if arah[tg.KpiKode] == domain.ArahTurun {
			faktor = 1.1 // TURUN: 110% target
		}
		assert.InDelta(t, tg.Target*faktor, *tg.AmbangWaspada, 5e-5, "ambang waspada bawaan 10%% dari target: %s", tg.KpiKode)
	}

	// PRD §12.5 dan §12.1–12.4.
	assert.InDelta(t, 5.4, byKode["DATA-MAPE-HARGA"].Target, 1e-9)
	assert.InDelta(t, 0.88, byKode["DATA-F1-EWS"].Target, 1e-9)
	assert.InDelta(t, 0.94, byKode["DATA-AUC-EWS"].Target, 1e-9)
	assert.InDelta(t, 1.0, byKode["DATA-RMSE-SHELFLIFE"].Target, 1e-9)
	assert.InDelta(t, 75, byKode["LOG-UTILISASI-CS"].Target, 1e-9)
	assert.InDelta(t, 78, byKode["TL-REKOMENDASI-AI"].Target, 1e-9)
	assert.InDelta(t, 4.2, byKode["TL-KOORDINASI"].Target, 1e-9)
	assert.NotContains(t, byKode, "DATA-KETEPATAN", "target belum ditetapkan: tidak dikarang")
	assert.NotContains(t, byKode, "LOG-RISIKO-FOODLOSS")
}

func TestSeedKPI_ReferenceIsIdempotentAndWritesNoValues(t *testing.T) {
	db := seedDB(t, "kpi_seed_ref_idem", berkasDefinisiKPI)

	require.NoError(t, seed.SeedFiles(t.Context(), db, berkasDefinisiKPI))

	assert.EqualValues(t, 17, jumlah(t, db, "kpi_definisi"))
	assert.EqualValues(t, 15, jumlah(t, db, "kpi_target"))
	assert.EqualValues(t, 3, jumlah(t, db, "target_kpi"))
	assert.EqualValues(t, 3, jumlah(t, db, "target_inflasi"))
	assert.Zero(t, jumlah(t, db, "kpi_nilai"), "referensi tidak membawa angka kinerja")
}

func TestSeedKPI_ReferenceKeepsTargetAndBaselineSetByStakeholders(t *testing.T) {
	db := seedDB(t, "kpi_seed_ref_keep", berkasDefinisiKPI)
	require.NoError(t, db.Exec(`UPDATE kpi_target SET target = 85, baseline = 40 WHERE kpi_kode = 'LOG-UTILISASI-CS'`).Error)
	require.NoError(t, db.Exec(`UPDATE kpi_definisi SET nama = 'diubah', sumber_query = NULL WHERE kode = 'LOG-UTILISASI-CS'`).Error)

	require.NoError(t, seed.SeedFiles(t.Context(), db, berkasDefinisiKPI))

	var tg domain.Target
	require.NoError(t, db.First(&tg, "kpi_kode = ?", "LOG-UTILISASI-CS").Error)
	assert.InDelta(t, 85, tg.Target, 1e-9)
	require.NotNil(t, tg.Baseline)
	assert.InDelta(t, 40, *tg.Baseline, 1e-9)
	var def domain.Definisi
	require.NoError(t, db.First(&def, "kode = ?", "LOG-UTILISASI-CS").Error)
	assert.Equal(t, "Utilisasi cold storage", def.Nama, "definisi adalah kode referensi: dipulihkan")
	require.NotNil(t, def.SumberQuery, "penghitung baru harus sampai ke basis data lama lewat seed referensi")
	assert.Equal(t, domain.QUtilisasiColdStorage, *def.SumberQuery)
}

func TestSeedKPI_DemoWithoutSourceDataWritesNothing(t *testing.T) {
	db := seedDB(t, "kpi_seed_demo_kosong", berkasDefinisiKPI, berkasRekamKPI)

	assert.Zero(t, jumlah(t, db, "kpi_nilai"), "tanpa data sumber tidak ada nilai, bukan nilai 0")
}

func TestSeedKPI_DemoComputesFromColdStorageOnlyAndIsIdempotent(t *testing.T) {
	db := seedDB(t, "kpi_seed_demo_cs", "reference/01_master.sql", berkasDefinisiKPI, "reference/09_cold_storage.sql",
		"demo/01_auth_users.sql", "demo/11_cold_storage_ops.sql", berkasRekamKPI)
	snapshot := semuaNilai(t, db)

	require.NoError(t, seed.SeedFiles(t.Context(), db, berkasRekamKPI))

	assert.NotEmpty(t, snapshot)
	assert.Equal(t, snapshot, semuaNilai(t, db), "putaran kedua tidak menulis apa pun")
	// Hanya KPI yang punya data sumber dan penghitung yang terisi; KPI harga dan intervensi menunggu data.
	assert.Equal(t, []string{"LOG-RISIKO-FOODLOSS", "LOG-UTILISASI-CS", "LOG-VOLUME-AGREGASI"}, kodeBernilai(t, db))
	for _, n := range snapshot {
		assert.Equal(t, 1, n.Versi, "putaran pertama menulis versi 1: %s %s", n.KpiKode, n.Periode.Format("2006-01"))
		assert.Equal(t, 1, n.Periode.Day(), "periode selalu tanggal 1")
	}
}

// Nilai di seed SQL harus sama dengan hasil job Go pada data yang sama: job tidak menemukan nilai baru maupun revisi.
// Ini menjaga rumus di berkas SQL tidak menyimpang dari repo.go (map penghitung). Dua cakupan: hanya cold storage (KPI
// logistik) dan seluruh tier demo (harga, intervensi, prediksi, cold storage: kesebelas penghitung).
func TestSeedKPI_DemoMatchesTheGoJob(t *testing.T) {
	cases := []struct {
		name string
		// siapkan mengisi basis data baru; minKode adalah jumlah minimum KPI berbeda yang harus bernilai, supaya paritas
		// tidak lulus karena kosong.
		siapkan func(t *testing.T, db *gorm.DB)
		minKode int
	}{
		{"cold storage saja", func(t *testing.T, db *gorm.DB) {
			require.NoError(t, seed.SeedFiles(t.Context(), db, "reference/01_master.sql", berkasDefinisiKPI, "reference/09_cold_storage.sql",
				"demo/01_auth_users.sql", "demo/11_cold_storage_ops.sql", berkasRekamKPI))
		}, 3},
		{"seluruh tier demo", func(t *testing.T, db *gorm.DB) {
			require.NoError(t, seed.SeedReference(t.Context(), db))
			require.NoError(t, seed.SeedDemo(t.Context(), db))
		}, 4},
	}
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			db := pg.NewDatabase(t, fmt.Sprintf("kpi_seed_paritas_%d", i))
			pgtest.MigrateUp(t, db)
			c.siapkan(t, db)
			sebelum := jumlah(t, db, "kpi_nilai")
			require.GreaterOrEqual(t, len(kodeBernilai(t, db)), c.minKode, "KPI bernilai hasil seed")

			lap := hitungUlangDenganGo(t, db)

			assert.Zero(t, lap.Baru, "job Go menemukan KPI/periode yang tidak dihitung seed SQL")
			assert.Zero(t, lap.Direvisi, "job Go menghasilkan nilai berbeda dari seed SQL")
			assert.EqualValues(t, sebelum, lap.Tetap, "setiap nilai seed dikonfirmasi ulang oleh job Go")
			assert.Equal(t, sebelum, jumlah(t, db, "kpi_nilai"), "job Go tidak menambah versi")
		})
	}
}
