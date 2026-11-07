package seeder

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	masterseeder "github.com/garda-inflasi/api/internal/master/seeder"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/internal/prediksi/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
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

		assert.LessOrEqual(t, r.Bawah, r.Nilai, *r.Kode)
		assert.LessOrEqual(t, r.Nilai, r.Atas, *r.Kode)
		assert.Positive(t, r.Bawah, *r.Kode)
		assert.Positive(t, r.HargaDasar, *r.Kode)
		assert.Equal(t, domain.SumberDemo, r.Sumber, *r.Kode)
		assert.Equal(t, now.Truncate(24*time.Hour).AddDate(0, 0, r.HorizonHari), r.TanggalTarget, *r.Kode)

		var faktor []domain.Faktor
		require.NoError(t, json.Unmarshal([]byte(r.Faktor), &faktor), *r.Kode)
		assert.NotEmpty(t, faktor, *r.Kode)
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
		assert.InDelta(t, pct, jumlah, 0.5, "%s: jumlah kontribusi harus sebesar perubahan proyeksi", *r.Kode)
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
		assert.Zero(t, a[i].ID, "ID BIGINT diberikan basis data, bukan seeder")
		assert.Equal(t, *a[i].Kode, *later[i].Kode, "kode (kunci idempotensi) tidak bergantung pada tanggal")
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

func TestBuildModels_KodeUnikDanBerbentukIDLama(t *testing.T) {
	models, err := BuildModels(now)
	require.NoError(t, err)

	terlihat := map[string]bool{}
	for _, m := range models {
		require.NotNil(t, m.Kode)
		assert.Zero(t, m.ID, "ID BIGINT diberikan basis data")
		assert.Equal(t, "MDL-"+m.KomoditasKode+"-"+m.Versi, *m.Kode, "kode = ID lama model demo")
		assert.LessOrEqual(t, len(*m.Kode), 64)
		assert.False(t, terlihat[*m.Kode], "kode ganda: %s", *m.Kode)
		terlihat[*m.Kode] = true
	}
}

func TestBuildProyeksi_KodeUnikDanBerbentukIDLama(t *testing.T) {
	rows, err := BuildProyeksi(now)
	require.NoError(t, err)

	terlihat := map[string]bool{}
	for _, r := range rows {
		require.NotNil(t, r.Kode)
		assert.Equal(t, fmt.Sprintf("PRD-%s-%s-%d", r.KomoditasKode, r.KabupatenKode, r.HorizonHari), *r.Kode, "kode = ID lama proyeksi demo")
		assert.Equal(t, VersiProduksi, r.ModelVersi)
		assert.Zero(t, r.ModelID, "ID model baru diketahui setelah model disimpan; SeedDemo yang menautkannya")
		assert.False(t, terlihat[*r.Kode], "kode ganda: %s", *r.Kode)
		terlihat[*r.Kode] = true
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
	require.NoError(t, db.Raw("SELECT tanggal_target FROM prediksi_harga WHERE kode = 'PRD-CBM-KRT-1171-14'").Scan(&tanggal).Error)
	assert.Equal(t, time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC), tanggal.UTC())
}

// idTerpasang membaca ID BIGINT baris berdasarkan kode.
func idTerpasang(t *testing.T, db *gorm.DB, tabel string) map[string]int64 {
	t.Helper()
	var rows []struct {
		Kode string
		ID   int64
	}
	require.NoError(t, db.Raw("SELECT kode, id FROM "+tabel).Scan(&rows).Error)
	out := make(map[string]int64, len(rows))
	for _, r := range rows {
		out[r.Kode] = r.ID
	}
	return out
}

func TestSeedDemo_IDBigintDiberikanDBDanProyeksiTertautKeModel(t *testing.T) {
	db := pg.NewDatabase(t, "prediksi_seeder_id")
	pgtest.MigrateUp(t, db)
	_, err := masterseeder.SeedReference(t.Context(), db)
	require.NoError(t, err)
	_, err = SeedDemo(t.Context(), db, now)
	require.NoError(t, err)

	modelAwal, proyeksiAwal := idTerpasang(t, db, "model_registry"), idTerpasang(t, db, "prediksi_harga")
	assert.Len(t, modelAwal, 28)
	assert.Len(t, proyeksiAwal, 240)
	assert.Contains(t, modelAwal, "MDL-CBM-KRT-v2.3.1")
	assert.Contains(t, proyeksiAwal, "PRD-CBM-KRT-1171-14")

	var konsisten int64
	require.NoError(t, db.Raw(`SELECT count(*) FROM prediksi_harga p JOIN model_registry m ON m.id = p.model_id
		WHERE m.komoditas_kode = p.komoditas_kode AND m.versi = p.model_versi AND m.status = 'PRODUKSI'`).Scan(&konsisten).Error)
	assert.EqualValues(t, 240, konsisten, "model_id dan model_versi ditulis konsisten (model PRODUKSI demo)")

	_, err = SeedDemo(t.Context(), db, now.AddDate(0, 0, 2))
	require.NoError(t, err)
	assert.Equal(t, modelAwal, idTerpasang(t, db, "model_registry"), "seed ulang tidak mengganti ID baris yang ada")
	assert.Equal(t, proyeksiAwal, idTerpasang(t, db, "prediksi_harga"))
}

// TestSeedDemo_DBHasilMigrasiTidakDigandakan: basis data lama (ID varchar) yang sudah dimigrasi 000114 memiliki baris
// demo ber-kode ID lama; SeedDemo mengenalinya lewat kode, tidak menambah baris, dan mempertahankan ID BIGINT-nya.
func TestSeedDemo_DBHasilMigrasiTidakDigandakan(t *testing.T) {
	db := pg.NewDatabase(t, "prediksi_seeder_migrasi")
	migrator := pgtest.Migrator(t, db)
	require.NoError(t, migrator.UpTo(t.Context(), 113))
	_, err := masterseeder.SeedReference(t.Context(), db)
	require.NoError(t, err)

	// Baris demo berbentuk lama: id varchar = ID lama, FK komposit (komoditas, versi) ke registri.
	models, err := BuildModels(now)
	require.NoError(t, err)
	for _, m := range models {
		require.NoError(t, db.Exec(`INSERT INTO model_registry (id, komoditas_kode, nama, versi, mape, drift, status, dilatih_pada, algorithm, sumber, catatan)
			VALUES (?, ?, ?, ?, ?, ?, ?::status_model, ?, ?, ?, ?)`,
			*m.Kode, m.KomoditasKode, m.Nama, m.Versi, m.Mape, m.Drift, m.Status, m.DilatihPada, m.Algorithm, m.Sumber, m.Catatan).Error)
	}
	rows, err := BuildProyeksi(now)
	require.NoError(t, err)
	for _, r := range rows {
		require.NoError(t, db.Exec(`INSERT INTO prediksi_harga (id, komoditas_kode, kabupaten_kode, tanggal_target, horizon_hari, nilai, bawah, atas, harga_dasar, model_versi, faktor, sumber, dibuat_pada)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?::jsonb, ?, ?)`,
			*r.Kode, r.KomoditasKode, r.KabupatenKode, r.TanggalTarget, r.HorizonHari, r.Nilai, r.Bawah, r.Atas, r.HargaDasar, r.ModelVersi, r.Faktor, r.Sumber, r.DibuatPada).Error)
	}
	require.NoError(t, migrator.UpTo(t.Context(), 114))
	modelAwal, proyeksiAwal := idTerpasang(t, db, "model_registry"), idTerpasang(t, db, "prediksi_harga")
	require.Len(t, modelAwal, 28)
	require.Len(t, proyeksiAwal, 240)

	sum, err := SeedDemo(t.Context(), db, now.AddDate(0, 0, 3))
	require.NoError(t, err)

	assert.Equal(t, 28, sum.Model)
	assert.Equal(t, modelAwal, idTerpasang(t, db, "model_registry"), "tidak ada baris baru dan ID tidak berubah")
	assert.Equal(t, proyeksiAwal, idTerpasang(t, db, "prediksi_harga"))
	var tanggal time.Time
	require.NoError(t, db.Raw("SELECT tanggal_target FROM prediksi_harga WHERE kode = 'PRD-CBM-KRT-1171-14'").Scan(&tanggal).Error)
	assert.Equal(t, time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC), tanggal.UTC(), "tanggal target bergeser mengikuti hari seed")
}
