package integration

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/ml/domain"
	mlrepo "github.com/garda-inflasi/api/internal/ml/repository/mongo"
	"github.com/garda-inflasi/api/internal/ml/usecase"
	"github.com/garda-inflasi/api/pkg/platform/mongodb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func repoUji(t *testing.T, m mongoUji) *mlrepo.Repository {
	t.Helper()
	k, err := mongodb.Buka(m.cfg())
	require.NoError(t, err)
	t.Cleanup(func() { _ = k.Tutup(context.Background()) })
	return mlrepo.New(k)
}

func TestRepo_SiklusTerbaruHanyaFinal(t *testing.T) {
	butuhDocker(t)
	m := mongoBaru(t)
	repo := repoUji(t, m)
	ctx := context.Background()

	_, err := repo.SiklusTerbaru(ctx)
	assert.ErrorIs(t, err, domain.ErrTidakDitemukan, "datasafe kosong: belum ada siklus")

	isiStandar(t, m)
	s, err := repo.SiklusTerbaru(ctx)
	require.NoError(t, err)
	assert.Equal(t, c09, s.CycleID, "C10 BERJALAN lebih baru tetapi setengah tertulis; C09 SEBAGIAN adalah terbaru")
	assert.Equal(t, "SEBAGIAN", s.Status)
	require.NotNil(t, s.TargetInflasi)
	assert.Equal(t, "INDEKS_PANGAN", *s.TargetInflasi)
	require.NotNil(t, s.ModelMetrics)
	require.NotNil(t, s.ModelMetrics.Regresi)
	assert.Equal(t, 3.2, *s.ModelMetrics.Regresi.MapeMedian)
	assert.Equal(t, 4.1, *s.ModelMetrics.Regresi.MapeBaselineMedian)
	assert.Len(t, s.SourceFreshness, 2)

	// Siklus apa pun statusnya tetap bisa dibaca metadatanya.
	g, err := repo.Siklus(ctx, c07)
	require.NoError(t, err)
	assert.Equal(t, "GAGAL", g.Status)
	_, err = repo.Siklus(ctx, "C20200101T00Z")
	assert.ErrorIs(t, err, domain.ErrTidakDitemukan)
}

func TestRepo_ListSiklusSaringDanHalaman(t *testing.T) {
	butuhDocker(t)
	m := mongoBaru(t)
	isiStandar(t, m)
	repo := repoUji(t, m)
	ctx := context.Background()

	items, total, err := repo.ListSiklus(ctx, domain.ListSiklusQuery{Page: 1, Limit: 2})
	require.NoError(t, err)
	assert.EqualValues(t, 4, total)
	require.Len(t, items, 2)
	assert.Equal(t, []string{c10, c09}, []string{items[0].CycleID, items[1].CycleID}, "terbaru dahulu")
	assert.Empty(t, items[0].SourceFreshness, "daftar tidak memuat source_freshness")

	items, total, err = repo.ListSiklus(ctx, domain.ListSiklusQuery{Page: 2, Limit: 2})
	require.NoError(t, err)
	assert.EqualValues(t, 4, total)
	assert.Equal(t, []string{c08, c07}, []string{items[0].CycleID, items[1].CycleID})

	items, total, err = repo.ListSiklus(ctx, domain.ListSiklusQuery{Status: domain.StatusFinal, Page: 1, Limit: 10})
	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	assert.Equal(t, []string{c09, c08}, []string{items[0].CycleID, items[1].CycleID})

	dari, sampai := tsSiklus[c08], tsSiklus[c10]
	items, total, err = repo.ListSiklus(ctx, domain.ListSiklusQuery{Dari: &dari, Sampai: &sampai, Page: 1, Limit: 10})
	require.NoError(t, err)
	assert.EqualValues(t, 2, total, "dari inklusif, sampai eksklusif")
	assert.Equal(t, []string{c09, c08}, []string{items[0].CycleID, items[1].CycleID})
}

func TestRepo_SnapshotDanForecastDetail(t *testing.T) {
	butuhDocker(t)
	m := mongoBaru(t)
	isiStandar(t, m)
	repo := repoUji(t, m)
	ctx := context.Background()

	s, err := repo.SiklusTerbaru(ctx)
	require.NoError(t, err)
	snap, err := repo.Snapshot(ctx, *s)
	require.NoError(t, err)
	assert.Len(t, snap.Forecast, 2)
	assert.Len(t, snap.Inflasi, 3)
	assert.Len(t, snap.Xai, 2)
	assert.Len(t, snap.Ews, 5, "sinyal KRITIS milik C10 tidak ikut")
	assert.Empty(t, snap.Makro)
	for _, f := range snap.Forecast {
		assert.Equal(t, c09, f.CycleID)
		assert.Empty(t, f.Riwayat, "snapshot tidak membawa riwayat")
		assert.Len(t, f.Kurva, 3)
	}
	assert.Equal(t, "1171", snap.Forecast[0].KabupatenKode, "urut kabupaten lalu komoditas")
	assert.Nil(t, snap.Inflasi[0].Proyeksi.Yoy, "yoy null tetap null, bukan 0")

	d, err := repo.Forecast(ctx, c09, "CBM-KRT", "1171")
	require.NoError(t, err)
	assert.Len(t, d.Riwayat, 2)
	require.NotNil(t, d.Metrik.Baseline)
	assert.Equal(t, "seasonal_naive", *d.Metrik.Baseline)
	_, err = repo.Forecast(ctx, c09, "CBM-KRT", "1101")
	assert.ErrorIs(t, err, domain.ErrTidakDitemukan)
}

func TestRepo_RunSumberMemisahkanBackfill(t *testing.T) {
	butuhDocker(t)
	m := mongoBaru(t)
	isiStandar(t, m)
	repo := repoUji(t, m)

	runs, err := repo.RunSumber(context.Background())
	require.NoError(t, err)
	per := map[string]domain.RunSumber{}
	for _, r := range runs {
		per[r.Sumber+":"+r.Dataset] = r
	}
	require.Len(t, per, 3)
	pihps := per["PIHPS:harga_harian"]
	require.NotNil(t, pihps.Terjadwal)
	assert.Equal(t, "run-pihps-2", pihps.Terjadwal.RunID, "run terjadwal terakhir, bukan backfill yang lebih baru")
	assert.Equal(t, usecase.KategoriGagal, usecase.Kategori(pihps.Terjadwal), "kegagalan crawler nyata tetap terlihat")
	require.NotNil(t, pihps.Backfill)
	assert.Equal(t, "run-pihps-bf", pihps.Backfill.RunID)
	assert.Equal(t, usecase.KategoriDilewati, usecase.Kategori(per["BMKG:prakiraan"].Terjadwal))
	assert.Nil(t, per["BPS:inflasi"].Terjadwal)
	assert.Equal(t, usecase.KategoriBelumAda, usecase.Kategori(per["BPS:inflasi"].Terjadwal))
}

// Mongo yang mati: Service menjawab cepat dengan alasan, pemutus terbuka, dan tidak ada panic.
func TestService_MongoMatiCepatDanTerputus(t *testing.T) {
	butuhDocker(t)
	k, err := mongodb.Buka(mongodb.Config{URI: "mongodb://127.0.0.1:1/?directConnection=true", DBUnseen: "u", DBSafe: "s", Timeout: 300 * time.Millisecond})
	require.NoError(t, err)
	t.Cleanup(func() { _ = k.Tutup(context.Background()) })
	svc := usecase.New(mlrepo.New(k), "", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	mulai := time.Now()
	_, ks := svc.Snapshot(ctx, "")
	assert.False(t, ks.Tersedia)
	assert.Contains(t, []string{usecase.AlasanMongoTidakTersedia, usecase.AlasanMongoLambat}, ks.Alasan)
	assert.Less(t, time.Since(mulai), 2*time.Second)

	mulai = time.Now()
	_, ks = svc.Snapshot(context.Background(), "")
	assert.Equal(t, usecase.AlasanMongoTidakTersedia, ks.Alasan, "pemutus terbuka: tidak menunggu timeout lagi")
	assert.Less(t, time.Since(mulai), 50*time.Millisecond)
}

// Paket repository Mongo hanya boleh membaca (docs/PIPELINE-DATA-ML.md §1).
func TestRepo_TidakAdaOperasiTulisMongo(t *testing.T) {
	tulis := regexp.MustCompile(`\.(InsertOne|InsertMany|UpdateOne|UpdateMany|UpdateByID|ReplaceOne|DeleteOne|DeleteMany|FindOneAndUpdate|FindOneAndReplace|FindOneAndDelete|BulkWrite|Drop|CreateIndex|CreateMany|CreateCollection)\(`)
	for _, dir := range []string{"../repository/mongo", "../usecase", "../delivery/http", "../jembatan", "../../../pkg/platform/mongodb"} {
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		require.NoError(t, err)
		for _, f := range files {
			if regexp.MustCompile(`_test\.go$`).MatchString(f) {
				continue
			}
			isi, err := os.ReadFile(f)
			require.NoError(t, err)
			assert.Falsef(t, tulis.Match(isi), "%s memakai operasi tulis Mongo", f)
		}
	}
}
