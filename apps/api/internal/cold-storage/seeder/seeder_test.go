package seeder

import (
	"math"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/cold-storage/domain"
	masterseeder "github.com/garda-inflasi/api/internal/master/seeder"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var seedNow = time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)

func round1(v float64) float64 { return math.Round(v*10) / 10 }

// Data contoh harus konsisten dengan angka yang tampil di kartu fasilitas dan
// subjudul tabel; uji ini mencegah seed berubah diam-diam.

func TestFasilitasSeeds_Shape(t *testing.T) {
	wilayah := map[string]string{}
	for _, f := range fasilitasSeeds {
		wilayah[f.Wilayah] = "11" + f.Kode[len(f.Kode)-2:] // kode tiruan, cukup unik per fasilitas
	}

	got := BuildFasilitas(wilayah)

	require.Len(t, got, 5)
	ids := map[string]bool{}
	for _, f := range got {
		require.NotNil(t, f.Kode)
		assert.False(t, ids[f.ID], "id ganda: %s", f.ID)
		ids[f.ID] = true
		assert.Equal(t, idx.UUIDStabil("cold_storage", *f.Kode), f.ID, "ID fasilitas = UUID stabil dari kode, sama dengan hasil migrasi 000113")
		assert.True(t, idx.ValidUUID(f.ID))
		assert.NotEmpty(t, f.KabupatenKode, "wilayah %s harus terpetakan", *f.Kode)
		assert.Positive(t, f.KapasitasTon)
		assert.Contains(t, []string{"OPERASIONAL", "PERAWATAN", "RENCANA"}, f.Status)
	}

	// Hanya fasilitas rencana yang belum punya telemetri.
	for _, f := range got {
		if f.Status == "RENCANA" {
			assert.Nil(t, f.SuhuAktualC, "%s belum dibangun", *f.Kode)
		} else {
			assert.NotNil(t, f.SuhuAktualC, "%s harus punya suhu aktual", *f.Kode)
		}
	}
}

func TestStokSeeds_FacilityVolumesMatchUtilisationCards(t *testing.T) {
	want := map[string]float64{"CS-01": 347.0, "CS-02": 240.0, "CS-03": 193.0, "CS-04": 33.0}

	got := map[string]float64{}
	for _, s := range BuildStok(seedNow) {
		got[s.ColdStorageID] += s.VolumeTon
	}

	require.Len(t, got, len(want), "fasilitas rencana (CS-05) tidak boleh punya stok")
	for kode, vol := range want {
		assert.Equal(t, vol, round1(got[FasilitasID(kode)]), "volume %s", kode)
	}
}

func TestStokSeeds_HighRiskVolume(t *testing.T) {
	var ton float64
	var lot int
	for _, s := range BuildStok(seedNow) {
		if s.RisikoFoodLoss > domain.RisikoTinggiAmbang {
			ton += s.VolumeTon
			lot++
		}
	}

	assert.Equal(t, 172.7, round1(ton))
	assert.Equal(t, 4, lot)
}

func TestStokSeeds_ValidRows(t *testing.T) {
	fasilitas := map[string]bool{}
	for _, f := range fasilitasSeeds {
		fasilitas[FasilitasID(f.Kode)] = true
	}

	stok := BuildStok(seedNow)

	assert.Len(t, stok, 23, "cukup untuk tiga halaman berisi 10 baris")
	kodeLot := map[string]bool{}
	for _, s := range stok {
		require.NotNil(t, s.Kode)
		assert.Zero(t, s.ID, "ID BIGINT diberi basis data, bukan seeder")
		assert.False(t, kodeLot[*s.Kode], "kode ganda: %s", *s.Kode)
		kodeLot[*s.Kode] = true

		assert.True(t, fasilitas[s.ColdStorageID], "%s merujuk fasilitas tak dikenal %s", *s.Kode, s.ColdStorageID)
		assert.NotEmpty(t, s.KomoditasKode)
		assert.NotEmpty(t, s.Pemilik)
		assert.Positive(t, s.VolumeTon)
		// Batas CHECK pada basis data (migrasi 000007).
		assert.GreaterOrEqual(t, s.RisikoFoodLoss, 0.0)
		assert.LessOrEqual(t, s.RisikoFoodLoss, 1.0)

		assert.True(t, s.MasukPada.Before(seedNow), "%s sudah masuk", *s.Kode)
		assert.True(t, s.KedaluwarsaPada.After(seedNow), "%s belum kedaluwarsa", *s.Kode)
	}
}

func TestStokSeeds_DatesAreRelativeToNow(t *testing.T) {
	first := BuildStok(seedNow)
	later := BuildStok(seedNow.Add(48 * time.Hour))

	require.Equal(t, len(first), len(later))
	for i := range first {
		assert.Equal(t, 48*time.Hour, later[i].MasukPada.Sub(first[i].MasukPada), "tanggal masuk bergeser mengikuti now")
		assert.Equal(t,
			first[i].KedaluwarsaPada.Sub(first[i].MasukPada),
			later[i].KedaluwarsaPada.Sub(later[i].MasukPada),
			"masa simpan lot tetap")
	}
}

// Lot dengan sisa masa simpan terpendek harus urutan teratas; lima baris pertama
// (menurut sisa lalu id) adalah baris yang tampil di desain halaman.
func TestStokSeeds_ShortestShelfLifeFirst(t *testing.T) {
	type lot struct {
		id   string
		sisa int
	}
	var lots []lot
	for _, s := range BuildStok(seedNow) {
		lots = append(lots, lot{*s.Kode, int(math.Round(s.KedaluwarsaPada.Sub(seedNow).Hours() / 24))})
	}

	// Urut menurut (sisa, id) seperti ORDER BY di repository.
	for i := 1; i < len(lots); i++ {
		for j := i; j > 0 && (lots[j].sisa < lots[j-1].sisa || (lots[j].sisa == lots[j-1].sisa && lots[j].id < lots[j-1].id)); j-- {
			lots[j], lots[j-1] = lots[j-1], lots[j]
		}
	}

	wantTop := []lot{
		{"STK-CS01-01", 5},
		{"STK-CS03-01", 6},
		{"STK-CS03-02", 6},
		{"STK-CS03-03", 7},
		{"STK-CS03-04", 10},
	}
	assert.Equal(t, wantTop, lots[:5])
}

// Setiap komoditas yang dipakai lot harus ada di seed master.reference.
func TestStokSeeds_UseMasterCommodities(t *testing.T) {
	master := map[string]bool{}
	for _, k := range masterseeder.Komoditas() {
		master[k.Kode] = true
	}
	for _, s := range stokSeeds {
		assert.True(t, master[s.Komoditas], "komoditas %s tidak ada di master.reference", s.Komoditas)
	}
}

// Setiap wilayah fasilitas harus ada di seed master.reference (dirujuk menurut nama).
func TestFasilitasSeeds_UseMasterRegions(t *testing.T) {
	master := map[string]bool{}
	for _, k := range masterseeder.Kabupaten() {
		master[k.Nama] = true
	}
	for _, f := range fasilitasSeeds {
		assert.True(t, master[f.Wilayah], "wilayah %q tidak ada di master.reference", f.Wilayah)
	}
}

// Transaksi demo: kode = "TRX-DEMO-" + kode lot, tanpa ID buatan seeder, dan menunjuk ID lot yang diberikan.
func TestBuildTransaksi_UsesLotIDsFromDatabaseAndKode(t *testing.T) {
	lotID := map[string]idx.ID{}
	for i, s := range stokSeeds {
		lotID[s.Kode] = idx.ID(100 + i)
	}

	trx, err := BuildTransaksi(seedNow, lotID)

	require.NoError(t, err)
	require.Len(t, trx, 23)
	for i, x := range trx {
		require.NotNil(t, x.Kode)
		assert.Equal(t, "TRX-DEMO-"+stokSeeds[i].Kode, *x.Kode)
		assert.Zero(t, x.ID)
		assert.Equal(t, idx.ID(100+i), x.LotID)
		assert.Equal(t, domain.TransaksiMasuk, x.Jenis)
		require.NotNil(t, x.FasilitasTujuanID)
		assert.Equal(t, FasilitasID(stokSeeds[i].Fasilitas), *x.FasilitasTujuanID)
	}
}

func TestBuildTransaksi_RejectsLotThatIsNotInTheDatabase(t *testing.T) {
	_, err := BuildTransaksi(seedNow, map[string]idx.ID{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "STK-CS01-01")
}

func TestBuildTelemetri_UsesFacilityUUIDAndKeepsCS04Spike(t *testing.T) {
	rows := BuildTelemetri(seedNow)

	require.Len(t, rows, 4*titikTelemetri)
	valid := map[string]bool{}
	for _, f := range fasilitasSeeds {
		if f.SuhuAktualC != nil {
			valid[FasilitasID(f.Kode)] = true
		}
	}
	var luar int
	for _, r := range rows {
		assert.True(t, valid[r.FasilitasID], "fasilitas_id harus UUID fasilitas beroperasi: %s", r.FasilitasID)
		if r.FasilitasID == FasilitasID("CS-04") && r.SuhuC > 6 {
			luar++
		}
	}
	assert.GreaterOrEqual(t, luar, 2, "lonjakan suhu CS-04 tetap ada walau kuncinya kini UUID")
}
