package seeder

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	masterseeder "github.com/garda-inflasi/api/internal/master/seeder"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/internal/spasial/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var (
	pg      *pgtest.Server
	seedNow = time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
)

func TestMain(m *testing.M) {
	os.Exit(pgtest.Main(m, &pg))
}

// koordinatMaster memakai koordinat seed master.reference, seperti di basis data.
func koordinatMaster() map[string]Koordinat {
	out := map[string]Koordinat{}
	for _, k := range masterseeder.Kabupaten() {
		out[k.Kode] = Koordinat{k.Lat, k.Lng}
	}
	return out
}

func TestBuildNeraca_ShapeAndDeterminism(t *testing.T) {
	wilayah := KodeWilayah(koordinatMaster())
	neraca := BuildNeraca(seedNow, wilayah)

	require.Len(t, neraca, len(komoditasNeraca)*23)
	assert.Equal(t, neraca, BuildNeraca(seedNow, wilayah), "PRNG tetap: dua kali bangun harus sama")

	keys := map[string]bool{}
	for _, n := range neraca {
		key := n.KomoditasKode + "/" + n.KabupatenKode
		assert.False(t, keys[key], "kunci ganda %s", key)
		keys[key] = true
		assert.Equal(t, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), n.Periode, "periode = awal bulan now")
		assert.Equal(t, SumberDemo, n.Sumber)
		if _, hot := titikPanas[titik{n.KomoditasKode, n.KabupatenKode}]; !hot {
			assert.LessOrEqual(t, n.DeltaHargaPct, 4.5, key)
			assert.GreaterOrEqual(t, n.DeltaHargaPct, -4.5, key)
		}
		// Harga naik berarti pasokan kurang: tanda berlawanan.
		if n.DeltaHargaPct > 0 {
			assert.LessOrEqual(t, n.SurplusDefisitTon, 0.0, key)
		} else if n.DeltaHargaPct < 0 {
			assert.GreaterOrEqual(t, n.SurplusDefisitTon, 0.0, key)
		}
	}
}

func TestBuildNeraca_HotspotsMatchEwsDemoAlerts(t *testing.T) {
	byKey := map[titik]float64{}
	for _, n := range BuildNeraca(seedNow, KodeWilayah(koordinatMaster())) {
		byKey[titik{n.KomoditasKode, n.KabupatenKode}] = n.DeltaHargaPct
	}
	assert.Equal(t, 21.4, byKey[titik{"CBM-KRT", "1171"}], "sama dengan EWS-1000")
	assert.Equal(t, 19.2, byKey[titik{"CBM-KRT", "1108"}], "sama dengan EWS-1001")
	assert.Equal(t, -18.0, byKey[titik{"KTG", "1106"}], "sama dengan EWS-1012")
	assert.Equal(t, -14.6, byKey[titik{"TMT", "1117"}], "sama dengan EWS-1013")
}

func TestBuildRute_FiveMostPressuredRegionsFromSurplusCenters(t *testing.T) {
	koordinat := koordinatMaster()
	rute := BuildRute(BuildNeraca(seedNow, KodeWilayah(koordinat)), koordinat, seedNow)

	require.Len(t, rute, 5)
	tujuan := map[string]string{}
	for _, r := range rute {
		tujuan[r.TujuanKode] = r.AsalKode
		assert.NotEqual(t, r.AsalKode, r.TujuanKode)
		assert.Positive(t, r.VolumeTon)
		assert.Positive(t, r.JarakKm)
		assert.Positive(t, r.Biaya)
		assert.Equal(t, "USULAN", r.Status)
		assert.Equal(t, "RT-"+r.AsalKode+"-"+r.TujuanKode, r.ID)
	}
	assert.Equal(t, map[string]string{"1171": "1106", "1108": "1117", "1174": "1109", "1107": "1106", "1109": "1117"}, tujuan)

	// 1171 dari 1106: volume = 8 + 21,4 x 1,6 = 42 ton; biaya = jarak x volume x tarif.
	first := rute[0]
	assert.Equal(t, "RT-1106-1171", first.ID)
	assert.EqualValues(t, 42, first.VolumeTon)
	km := jarakMeter(koordinat["1106"], koordinat["1171"]) / 1000 * faktorJalan
	assert.InDelta(t, km*42*tarifTonKm, float64(first.Biaya), 1)
	assert.InDelta(t, km/kecepatanKm, first.JamTempuh, 0.05)
}

func TestBuildKandidat_ScoreIsWeightedAverageOfComponents(t *testing.T) {
	kandidat, err := BuildKandidat(seedNow)
	require.NoError(t, err)

	require.Len(t, kandidat, 3)
	assert.Equal(t, 82, kandidat[0].Skor)
	assert.Equal(t, 77, kandidat[1].Skor)
	assert.Equal(t, 71, kandidat[2].Skor)
	var weight float64
	for _, w := range bobotSkor {
		weight += w
	}
	assert.InDelta(t, 1.0, weight, 1e-9)
	for _, k := range kandidat {
		var komponen map[string]float64
		require.NoError(t, json.Unmarshal([]byte(k.KomponenSkor), &komponen))
		assert.Len(t, komponen, len(bobotSkor), k.ID)
		assert.Equal(t, k.Skor, Skor(komponen))
	}
}

func seededDB(t *testing.T, name string) *gorm.DB {
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

func TestSeedDemo_IsIdempotentWithinAMonthAndKeepsPreviousPeriods(t *testing.T) {
	db := seededDB(t, "spasial_seed_idempotent")
	ctx := t.Context()

	for range 2 {
		sum, err := SeedDemo(ctx, db, seedNow)
		require.NoError(t, err)
		assert.Equal(t, DemoSummary{Neraca: len(komoditasNeraca) * 23, Rute: 5, Kandidat: 3}, sum)
	}
	assert.EqualValues(t, len(komoditasNeraca)*23, count(t, db, "neraca_wilayah"))
	assert.EqualValues(t, 5, count(t, db, "rute_distribusi"))
	assert.EqualValues(t, 3, count(t, db, "kandidat_cold_storage"))

	_, err := SeedDemo(ctx, db, seedNow.AddDate(0, 1, 0))
	require.NoError(t, err)
	assert.EqualValues(t, 2*len(komoditasNeraca)*23, count(t, db, "neraca_wilayah"), "bulan baru menambah periode, tidak menimpa")
	assert.EqualValues(t, 5, count(t, db, "rute_distribusi"))
}

func TestSeedDemo_WithoutMasterExplainsWhatToRunFirst(t *testing.T) {
	db := pg.NewDatabase(t, "spasial_seed_no_master")
	pgtest.MigrateUp(t, db)

	_, err := SeedDemo(t.Context(), db, seedNow)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "master.reference")
}

func TestWilayahView_ExposesEveryKabupatenAsAPoint(t *testing.T) {
	db := seededDB(t, "spasial_view")

	var rows []domain.Wilayah
	require.NoError(t, db.Order("kode").Find(&rows).Error)

	require.Len(t, rows, 23)
	levels := map[string]int{}
	for _, w := range rows {
		levels[w.Level]++
	}
	assert.Equal(t, map[string]int{"kabupaten": 18, "kota": 5}, levels)
	assert.Equal(t, "KOTA BANDA ACEH", rows[len(rows)-5].Nama)
}

func TestMigration_RejectsInvalidRouteAndScore(t *testing.T) {
	db := seededDB(t, "spasial_checks")

	err := db.Exec(`INSERT INTO rute_distribusi (id, asal_kode, tujuan_kode, komoditas_kode, volume_ton, jarak_km, jam_tempuh, biaya)
		VALUES ('X', '1171', '1171', 'BWM', 5, 1, 1, 1)`).Error
	assert.ErrorContains(t, err, "chk_rute_distribusi_beda")

	err = db.Exec(`INSERT INTO kandidat_cold_storage (id, kabupaten_kode, lokasi, skor, komponen_skor) VALUES ('X', '1171', 'X', 101, '{}')`).Error
	assert.ErrorContains(t, err, "chk_kandidat_cold_storage_skor")
}
