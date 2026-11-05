package seeder

import (
	"math"
	"os"
	"testing"
	"time"

	authseeder "github.com/garda-inflasi/api/internal/auth/seeder"
	"github.com/garda-inflasi/api/internal/harga/domain"
	masterseeder "github.com/garda-inflasi/api/internal/master/seeder"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var pg *pgtest.Server

func TestMain(m *testing.M) {
	os.Exit(pgtest.Main(m, &pg))
}

// seedNow jatuh pukul 14.30 WIB sehingga hari ini sudah punya jam kerja lapangan.
var seedNow = time.Date(2026, 10, 16, 7, 30, 0, 0, time.UTC)

func sampleInput() Input {
	return Input{
		Komoditas: []Komoditas{
			{Kode: "CBM-KRT", Satuan: "kg", HargaAcuan: 52000},
			{Kode: "BWM", Satuan: "kg", HargaAcuan: 41000},
			{Kode: "TLR", Satuan: "kg", HargaAcuan: 29500},
		},
		Petugas: []Petugas{
			{ID: "mahasiswa_1", KabupatenKode: "1171", PasarID: "PSR-1171-01", PasarLat: 5.5603, PasarLng: 95.3237, RadiusMeter: 400},
			{ID: "mahasiswa_2", KabupatenKode: "1108", PasarID: "PSR-1108-01", PasarLat: 5.5085, PasarLng: 95.364, RadiusMeter: 600},
		},
		Validator:  "koordinator_lapangan",
		Publikator: "manajer_data",
	}
}

func TestBuild_IsDeterministic(t *testing.T) {
	assert.Equal(t, Build(sampleInput(), seedNow), Build(sampleInput(), seedNow))
}

func TestBuild_UniqueIDsAndRowCount(t *testing.T) {
	entri := Build(sampleInput(), seedNow)

	ids := map[string]bool{}
	for _, e := range entri {
		assert.False(t, ids[e.ID], "id ganda: %s", e.ID)
		ids[e.ID] = true
	}
	// 2 petugas x 3 komoditas x 90 hari = 540 harga konsumen, ditambah harga produsen
	// cabai dan bawang merah dari kedua petugas, dan duplikat.
	assert.GreaterOrEqual(t, len(entri), 540+2*2*HariHistori)
}

func TestBuild_RowsAreValid(t *testing.T) {
	in := sampleInput()
	dalamHistori := seedNow.AddDate(0, 0, -HariHistori-1)

	for _, e := range Build(in, seedNow) {
		require.Positive(t, e.Harga, e.ID)
		require.True(t, domain.StatusValid(e.Status), "%s: status %q", e.ID, e.Status)
		assert.False(t, e.DicatatPada.After(seedNow), "%s dicatat di masa depan", e.ID)
		assert.True(t, e.DicatatPada.After(dalamHistori), "%s di luar histori", e.ID)
		require.NotNil(t, e.PetugasID)
		assert.NotEqual(t, in.Validator, *e.PetugasID)

		if e.Status == domain.StatusDivalidasi || e.Status == domain.StatusDipublikasi {
			require.NotNil(t, e.DivalidasiOleh, e.ID)
			assert.NotEqual(t, *e.PetugasID, *e.DivalidasiOleh, "pemisahan tugas: %s", e.ID)
			assert.False(t, e.DivalidasiPada.After(seedNow), e.ID)
		} else {
			assert.Nil(t, e.DivalidasiOleh, e.ID)
		}
		if e.Status == domain.StatusDipublikasi {
			require.NotNil(t, e.DipublikasiOleh, e.ID)
			assert.False(t, e.DipublikasiPada.After(seedNow), e.ID)
		} else {
			assert.Nil(t, e.DipublikasiOleh, e.ID)
		}
	}
}

func TestBuild_ShapesHistoryAndStatuses(t *testing.T) {
	entri := Build(sampleInput(), seedNow)

	perStatus := map[string]int{}
	var outlier, luarGeofence, duplikat, foto int
	for _, e := range entri {
		perStatus[e.Status]++
		if math.Abs(e.DeviasiPct) > domain.AmbangAnomaliPct {
			outlier++
		}
		if !e.DalamGeofence {
			luarGeofence++
		}
		if len(e.ID) > 4 && e.ID[len(e.ID)-4:] == "-dup" {
			duplikat++
		}
		if e.FotoURL != nil {
			foto++
		}
	}

	for _, status := range []string{domain.StatusDraf, domain.StatusDiajukan, domain.StatusDivalidasi, domain.StatusDipublikasi, domain.StatusDitolak} {
		assert.Positive(t, perStatus[status], "sebaran status harus memuat %s", status)
	}
	assert.Greater(t, perStatus[domain.StatusDipublikasi], perStatus[domain.StatusDiajukan], "histori didominasi entri terbit")
	assert.Positive(t, outlier, "harus ada outlier")
	assert.Positive(t, luarGeofence, "harus ada entri di luar geofence")
	assert.Positive(t, duplikat, "harus ada duplikat")
	assert.Positive(t, foto)
	assert.Less(t, foto, len(entri), "sebagian entri tanpa foto")
}

func TestBuild_GeofenceFlagMatchesDistance(t *testing.T) {
	in := sampleInput()
	pasar := map[string]Petugas{}
	for _, p := range in.Petugas {
		pasar[p.PasarID] = p
	}

	for _, e := range Build(in, seedNow) {
		p := pasar[e.PasarID]
		const meterPerDerajat = 111_320.0
		dLat := (*e.Lat - p.PasarLat) * meterPerDerajat
		dLng := (*e.Lng - p.PasarLng) * meterPerDerajat * math.Cos(p.PasarLat*math.Pi/180)
		jarak := math.Hypot(dLat, dLng)
		if e.DalamGeofence {
			assert.LessOrEqual(t, jarak, float64(p.RadiusMeter), "%s ditandai dalam geofence", e.ID)
		} else {
			assert.Greater(t, jarak, float64(p.RadiusMeter), "%s ditandai luar geofence", e.ID)
		}
	}
}

func TestBuild_CabaiTertekanDanTomatSurplus(t *testing.T) {
	// Skenario demo: harga cabai hari ini jauh di atas awal histori, tomat di bawahnya.
	in := sampleInput()
	in.Komoditas = []Komoditas{{Kode: "CBM-KRT", Satuan: "kg", HargaAcuan: 52000}, {Kode: "TMT", Satuan: "kg", HargaAcuan: 12000}}
	acuan := map[string]float64{"CBM-KRT": 52000, "TMT": 12000}
	entri := Build(in, seedNow)

	// rasio adalah rata-rata harga konsumen wajar terhadap harga acuan pada rentang umur (hari).
	rasio := func(kode string, dari, sampai int) float64 {
		var total, n float64
		for _, e := range entri {
			if e.KomoditasKode != kode || e.Tipe != domain.TipeKonsumen || math.Abs(e.DeviasiPct) > domain.AmbangAnomaliPct {
				continue
			}
			if umur := int(seedNow.Sub(e.DicatatPada).Hours() / 24); umur >= dari && umur <= sampai {
				total += float64(e.Harga) / acuan[kode]
				n++
			}
		}
		return total / n
	}

	assert.Greater(t, rasio("CBM-KRT", 0, 3), rasio("CBM-KRT", 60, 80)*1.15)
	assert.Less(t, rasio("TMT", 0, 3), rasio("TMT", 60, 80)*0.9)
}

func TestSeedDemo_RequiresReferenceData(t *testing.T) {
	db := pg.NewDatabase(t, "harga_seed_kosong")
	pgtest.MigrateUp(t, db)

	_, err := SeedDemo(t.Context(), db, seedNow)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "master.reference")
}

func TestSeedDemo_IsIdempotent(t *testing.T) {
	db := pg.NewDatabase(t, "harga_seed")
	pgtest.MigrateUp(t, db)
	_, err := masterseeder.SeedReference(t.Context(), db)
	require.NoError(t, err)
	_, err = authseeder.SeedDemo(t.Context(), db)
	require.NoError(t, err)

	first, err := SeedDemo(t.Context(), db, seedNow)
	require.NoError(t, err)
	assert.Greater(t, first.Entri, 10_000)

	count := func() (n int64) {
		require.NoError(t, db.Raw("SELECT count(*) FROM entri_harga").Scan(&n).Error)
		return n
	}
	assert.EqualValues(t, first.Entri, count())

	// Dijalankan ulang pada waktu berbeda: baris sama, tanggal bergeser.
	second, err := SeedDemo(t.Context(), db, seedNow.Add(26*time.Hour))
	require.NoError(t, err)
	assert.Equal(t, first.Entri, second.Entri)
	assert.EqualValues(t, first.Entri, count())

	var tanpaPetugas int64
	require.NoError(t, db.Raw("SELECT count(*) FROM entri_harga WHERE petugas_id IS NULL").Scan(&tanpaPetugas).Error)
	assert.Zero(t, tanpaPetugas)
}
