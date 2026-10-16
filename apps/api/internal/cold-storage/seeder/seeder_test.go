package seeder

import (
	"math"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/cold-storage/domain"
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
		wilayah[f.Wilayah] = "11" + f.ID[len(f.ID)-2:] // kode tiruan, cukup unik per fasilitas
	}

	got := BuildFasilitas(wilayah)

	require.Len(t, got, 5)
	ids := map[string]bool{}
	for _, f := range got {
		assert.False(t, ids[f.ID], "id ganda: %s", f.ID)
		ids[f.ID] = true
		assert.NotEmpty(t, f.KabupatenKode, "wilayah %s harus terpetakan", f.ID)
		assert.Positive(t, f.KapasitasTon)
		assert.Contains(t, []string{"OPERASIONAL", "PERAWATAN", "RENCANA"}, f.Status)
	}

	// Hanya fasilitas rencana yang belum punya telemetri.
	for _, f := range got {
		if f.Status == "RENCANA" {
			assert.Nil(t, f.SuhuAktualC, "%s belum dibangun", f.ID)
		} else {
			assert.NotNil(t, f.SuhuAktualC, "%s harus punya suhu aktual", f.ID)
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
	for id, vol := range want {
		assert.Equal(t, vol, round1(got[id]), "volume %s", id)
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
		fasilitas[f.ID] = true
	}

	stok := BuildStok(seedNow)

	assert.Len(t, stok, 23, "cukup untuk tiga halaman berisi 10 baris")
	ids := map[string]bool{}
	for _, s := range stok {
		assert.False(t, ids[s.ID], "id ganda: %s", s.ID)
		ids[s.ID] = true

		assert.True(t, fasilitas[s.ColdStorageID], "%s merujuk fasilitas tak dikenal %s", s.ID, s.ColdStorageID)
		assert.NotEmpty(t, s.KomoditasKode)
		assert.NotEmpty(t, s.Pemilik)
		assert.Positive(t, s.VolumeTon)
		// Batas CHECK pada basis data (migrasi 000007).
		assert.GreaterOrEqual(t, s.RisikoFoodLoss, 0.0)
		assert.LessOrEqual(t, s.RisikoFoodLoss, 1.0)

		assert.True(t, s.MasukPada.Before(seedNow), "%s sudah masuk", s.ID)
		assert.True(t, s.KedaluwarsaPada.After(seedNow), "%s belum kedaluwarsa", s.ID)
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
		lots = append(lots, lot{s.ID, int(math.Round(s.KedaluwarsaPada.Sub(seedNow).Hours() / 24))})
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

func TestKomoditasPelengkap_AreHorticulture(t *testing.T) {
	kode := map[string]bool{}
	for _, k := range komoditasPelengkap {
		kode[k.Kode] = true
		assert.Positive(t, k.ShelfLifeHari)
		assert.Positive(t, k.HargaAcuan)
	}
	assert.True(t, kode["tomat"])
	assert.True(t, kode["kentang"])

	// Setiap komoditas yang dipakai lot harus ada di master seed (000006) atau pelengkap.
	master := map[string]bool{
		"cabai_merah_keriting": true, "cabai_rawit_merah": true, "bawang_merah": true,
		"ikan_tongkol": true, "telur_ayam": true,
	}
	for _, s := range stokSeeds {
		assert.True(t, master[s.Komoditas] || kode[s.Komoditas], "komoditas %s tidak tersedia", s.Komoditas)
	}
}
