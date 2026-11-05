package seeder

import (
	"os"
	"testing"
	"time"

	coldseeder "github.com/garda-inflasi/api/internal/cold-storage/seeder"
	"github.com/garda-inflasi/api/internal/kpi/domain"
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

var waktuTetap = time.Date(2026, 11, 5, 8, 0, 0, 0, time.UTC)

func newDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db := pg.NewDatabase(t, name)
	pgtest.MigrateUp(t, db)
	_, err := masterseeder.SeedReference(t.Context(), db)
	require.NoError(t, err)
	return db
}

func count(t *testing.T, db *gorm.DB, table string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Table(table).Count(&n).Error)
	return n
}

func TestDefinisi_Shape(t *testing.T) {
	rows := Definisi()

	require.Len(t, rows, 17)
	kode, kelompok, query := map[string]bool{}, map[string]int{}, map[string]bool{}
	for _, d := range rows {
		assert.False(t, kode[d.Kode], "kode ganda %s", d.Kode)
		kode[d.Kode] = true
		kelompok[d.Kelompok]++
		assert.Contains(t, []string{domain.ArahNaik, domain.ArahTurun}, d.ArahBaik, d.Kode)
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
	}, query, "hanya KPI yang datanya sudah ada yang punya penghitung")
}

func TestTarget_FollowsPRD12AndLeavesBaselineEmpty(t *testing.T) {
	byKode := map[string]domain.Target{}
	for _, tg := range Target() {
		byKode[tg.KpiKode] = tg
		assert.Nil(t, tg.Baseline, "%s: baseline adalah keputusan bisnis, tidak boleh diisi seeder", tg.KpiKode)
		assert.Nil(t, tg.WilayahKode)
	}

	// PRD §12.5 dan §12.1–12.4.
	assert.InDelta(t, 5.4, byKode["DATA-MAPE-HARGA"].Target, 1e-9)
	assert.InDelta(t, 0.88, byKode["DATA-F1-EWS"].Target, 1e-9)
	assert.InDelta(t, 0.94, byKode["DATA-AUC-EWS"].Target, 1e-9)
	assert.InDelta(t, 1.0, byKode["DATA-RMSE-SHELFLIFE"].Target, 1e-9)
	assert.InDelta(t, 75, byKode["LOG-UTILISASI-CS"].Target, 1e-9)
	assert.InDelta(t, 78, byKode["TL-REKOMENDASI-AI"].Target, 1e-9)
	assert.InDelta(t, 4.2, byKode["TL-KOORDINASI"].Target, 1e-9)

	assert.InDelta(t, 67.5, *byKode["LOG-UTILISASI-CS"].AmbangWaspada, 1e-9, "NAIK: 90% target")
	assert.InDelta(t, 5.94, *byKode["DATA-MAPE-HARGA"].AmbangWaspada, 1e-9, "TURUN: 110% target")
	assert.NotContains(t, byKode, "DATA-KETEPATAN", "target belum ditetapkan: tidak dikarang")
	assert.NotContains(t, byKode, "LOG-RISIKO-FOODLOSS")
}

func TestSeedReference_IsIdempotentAndWritesNoValues(t *testing.T) {
	db := newDB(t, "kpi_seed_ref_idem")

	for range 2 {
		sum, err := SeedReference(t.Context(), db)
		require.NoError(t, err)
		assert.Equal(t, ReferenceSummary{Definisi: 17, Target: 15}, sum)
	}

	assert.EqualValues(t, 17, count(t, db, "kpi_definisi"))
	assert.EqualValues(t, 15, count(t, db, "kpi_target"))
	assert.Zero(t, count(t, db, "kpi_nilai"), "referensi tidak membawa angka kinerja")
}

func TestSeedReference_KeepsTargetAndBaselineSetByStakeholders(t *testing.T) {
	db := newDB(t, "kpi_seed_ref_keep")
	_, err := SeedReference(t.Context(), db)
	require.NoError(t, err)
	require.NoError(t, db.Exec(`UPDATE kpi_target SET target = 85, baseline = 40 WHERE kpi_kode = 'LOG-UTILISASI-CS'`).Error)
	require.NoError(t, db.Exec(`UPDATE kpi_definisi SET nama = 'diubah' WHERE kode = 'LOG-UTILISASI-CS'`).Error)

	_, err = SeedReference(t.Context(), db)
	require.NoError(t, err)

	var tg domain.Target
	require.NoError(t, db.First(&tg, "kpi_kode = ?", "LOG-UTILISASI-CS").Error)
	assert.InDelta(t, 85, tg.Target, 1e-9)
	require.NotNil(t, tg.Baseline)
	assert.InDelta(t, 40, *tg.Baseline, 1e-9)
	var def domain.Definisi
	require.NoError(t, db.First(&def, "kode = ?", "LOG-UTILISASI-CS").Error)
	assert.Equal(t, "Utilisasi cold storage", def.Nama, "definisi adalah kode referensi: dipulihkan")
}

func TestSeedDemo_WithoutSourceDataWritesNothing(t *testing.T) {
	db := newDB(t, "kpi_seed_demo_kosong")
	_, err := SeedReference(t.Context(), db)
	require.NoError(t, err)

	sum, err := SeedDemo(t.Context(), db, waktuTetap)

	require.NoError(t, err)
	assert.Zero(t, sum.Baru)
	assert.Positive(t, sum.TanpaDasar)
	assert.Zero(t, count(t, db, "kpi_nilai"), "tanpa data sumber tidak ada nilai, bukan nilai 0")
}

func TestSeedDemo_ComputesFromColdStorageDemoAndIsIdempotent(t *testing.T) {
	db := newDB(t, "kpi_seed_demo_cs")
	_, err := SeedReference(t.Context(), db)
	require.NoError(t, err)
	_, err = coldseeder.SeedReference(t.Context(), db)
	require.NoError(t, err)
	_, err = coldseeder.SeedDemo(t.Context(), db, waktuTetap)
	require.NoError(t, err)

	first, err := SeedDemo(t.Context(), db, waktuTetap)
	require.NoError(t, err)
	var snapshot []domain.Nilai
	require.NoError(t, db.Order("kpi_kode, periode, versi").Find(&snapshot).Error)
	second, err := SeedDemo(t.Context(), db, waktuTetap)
	require.NoError(t, err)
	var again []domain.Nilai
	require.NoError(t, db.Order("kpi_kode, periode, versi").Find(&again).Error)

	assert.Positive(t, first.Baru)
	assert.Equal(t, DemoSummary{Tetap: first.Baru, TanpaDasar: first.TanpaDasar}, second, "putaran kedua tidak menulis apa pun")
	assert.Equal(t, snapshot, again)

	// Hanya KPI yang punya data sumber dan penghitung yang terisi.
	var kodeBernilai []string
	require.NoError(t, db.Model(&domain.Nilai{}).Distinct().Pluck("kpi_kode", &kodeBernilai).Error)
	assert.ElementsMatch(t, []string{"LOG-UTILISASI-CS", "LOG-RISIKO-FOODLOSS"}, kodeBernilai)

	// Bulan berjalan: lot demo 347+240+193 ton (fasilitas operasional) dari kapasitas 400+300+250.
	var utilisasi domain.Nilai
	require.NoError(t, db.Where("kpi_kode = ? AND periode = ?", "LOG-UTILISASI-CS", time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)).First(&utilisasi).Error)
	assert.InDelta(t, 100.0*780/950, utilisasi.Nilai, 1e-3)
}
