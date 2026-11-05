package seeder

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	masterseeder "github.com/garda-inflasi/api/internal/master/seeder"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/internal/prediksi/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var pg *pgtest.Server

func TestMain(m *testing.M) {
	os.Exit(pgtest.Main(m, &pg))
}

var now = time.Date(2026, 11, 5, 9, 30, 0, 0, time.UTC)

func TestBuildProyeksi_EveryPilotCommodityHasConsistentProjections(t *testing.T) {
	rows, err := BuildProyeksi(now)
	require.NoError(t, err)

	pilot := map[string]bool{}
	for _, k := range masterseeder.Komoditas() {
		if k.Pilot {
			pilot[k.Kode] = true
		}
	}
	require.NotEmpty(t, pilot)

	// komoditas -> kabupaten -> horizon
	got := map[string]map[string]map[int]bool{}
	for _, r := range rows {
		if got[r.KomoditasKode] == nil {
			got[r.KomoditasKode] = map[string]map[int]bool{}
		}
		if got[r.KomoditasKode][r.KabupatenKode] == nil {
			got[r.KomoditasKode][r.KabupatenKode] = map[int]bool{}
		}
		got[r.KomoditasKode][r.KabupatenKode][r.HorizonHari] = true

		assert.LessOrEqual(t, r.Bawah, r.Nilai, r.ID)
		assert.LessOrEqual(t, r.Nilai, r.Atas, r.ID)
		assert.Positive(t, r.Bawah, r.ID)
		assert.Positive(t, r.HargaDasar, r.ID)
		assert.Equal(t, domain.SumberDemo, r.Sumber, r.ID)
		assert.Equal(t, now.Truncate(24*time.Hour).AddDate(0, 0, r.HorizonHari), r.TanggalTarget, r.ID)

		var faktor []domain.Faktor
		require.NoError(t, json.Unmarshal([]byte(r.Faktor), &faktor), r.ID)
		assert.NotEmpty(t, faktor, r.ID)
		for _, f := range faktor {
			assert.NotEmpty(t, f.Nama)
			assert.NotEmpty(t, f.Detail)
		}
	}

	kabupaten := masterseeder.Kabupaten()
	for kode := range pilot {
		require.Len(t, got[kode], len(kabupaten), "komoditas pilot %s harus ada di semua kabupaten", kode)
		for kab, horizons := range got[kode] {
			for _, h := range Horizons {
				assert.True(t, horizons[h], "%s/%s horizon %d", kode, kab, h)
			}
		}
	}
	for _, k := range masterseeder.Komoditas() {
		assert.Contains(t, got[k.Kode], KabupatenUtama, "semua komoditas tampil di kabupaten utama: %s", k.Kode)
	}
	assert.Len(t, rows, len(masterseeder.Komoditas())*len(Horizons)+len(pilot)*(len(kabupaten)-1)*len(Horizons))
}

func TestBuildProyeksi_FactorsAddUpToProjectedChange(t *testing.T) {
	rows, err := BuildProyeksi(now)
	require.NoError(t, err)

	for _, r := range rows {
		var faktor []domain.Faktor
		require.NoError(t, json.Unmarshal([]byte(r.Faktor), &faktor))
		var jumlah float64
		for _, f := range faktor {
			jumlah += f.Kontribusi
		}
		pct := float64(r.Nilai-r.HargaDasar) / float64(r.HargaDasar) * 100
		assert.InDelta(t, pct, jumlah, 0.5, "%s: jumlah kontribusi harus sebesar perubahan proyeksi", r.ID)
	}
}

func TestBuildProyeksi_IsDeterministic(t *testing.T) {
	a, errA := BuildProyeksi(now)
	b, errB := BuildProyeksi(now)
	require.NoError(t, errA)
	require.NoError(t, errB)
	assert.Equal(t, a, b)

	later, err := BuildProyeksi(now.AddDate(0, 0, 3))
	require.NoError(t, err)
	require.Len(t, later, len(a))
	for i := range a {
		assert.Equal(t, a[i].ID, later[i].ID, "id tidak bergantung pada tanggal")
		assert.Equal(t, a[i].Nilai, later[i].Nilai)
		assert.Equal(t, a[i].TanggalTarget.AddDate(0, 0, 3), later[i].TanggalTarget)
	}
}

func TestBuildModels_OneProductionAndOneCandidatePerCommodity(t *testing.T) {
	models, err := BuildModels(now)
	require.NoError(t, err)

	produksi, kandidat := map[string]int{}, map[string]int{}
	for _, m := range models {
		assert.Equal(t, domain.SumberDemo, m.Sumber)
		assert.Positive(t, m.Mape)
		assert.Contains(t, m.Catatan, "bukan keluaran model sungguhan", "model demo tidak boleh tampak seperti hasil pelatihan")
		assert.Nil(t, m.ArtefakURI)
		switch m.Status {
		case domain.StatusProduksi:
			produksi[m.KomoditasKode]++
			assert.Equal(t, VersiProduksi, m.Versi)
		case domain.StatusKandidat:
			kandidat[m.KomoditasKode]++
			assert.Equal(t, VersiKandidat, m.Versi)
		}
	}
	assert.Len(t, produksi, len(masterseeder.Komoditas()))
	for kode, n := range produksi {
		assert.Equal(t, 1, n, kode)
		assert.Equal(t, 1, kandidat[kode], kode)
	}
}

func TestSeedDemo_IsIdempotentAcrossDays(t *testing.T) {
	db := pg.NewDatabase(t, "prediksi_seeder")
	pgtest.MigrateUp(t, db)
	_, err := masterseeder.SeedReference(t.Context(), db)
	require.NoError(t, err)

	sum, err := SeedDemo(t.Context(), db, now)
	require.NoError(t, err)
	assert.Equal(t, 28, sum.Model)
	assert.Equal(t, 240, sum.Proyeksi)

	count := func(table string) int64 {
		var n int64
		require.NoError(t, db.Table(table).Count(&n).Error)
		return n
	}
	models, proyeksi := count("model_registry"), count("prediksi_harga")
	assert.EqualValues(t, sum.Model, models)
	assert.EqualValues(t, sum.Proyeksi, proyeksi)

	for _, waktu := range []time.Time{now, now.AddDate(0, 0, 4)} {
		_, err = SeedDemo(t.Context(), db, waktu)
		require.NoError(t, err)
		assert.Equal(t, models, count("model_registry"))
		assert.Equal(t, proyeksi, count("prediksi_harga"), "seed ulang menggeser tanggal, bukan menambah baris")
	}

	var tanggal time.Time
	require.NoError(t, db.Raw("SELECT tanggal_target FROM prediksi_harga WHERE id = 'PRD-CBM-KRT-1171-14'").Scan(&tanggal).Error)
	assert.Equal(t, time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC), tanggal.UTC())
}
