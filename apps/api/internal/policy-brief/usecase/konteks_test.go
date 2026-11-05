package usecase

import (
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/policy-brief/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func bulanUji(tahun int, m time.Month) time.Time { return time.Date(tahun, m, 1, 0, 0, 0, 0, time.UTC) }

// dataLengkap adalah data tetap untuk menguji pengumpul konteks: semua bagian terisi, sebagian bersumber DEMO.
func dataLengkap() domain.DataPeriode {
	return domain.DataPeriode{
		Periode: bulanUji(2026, time.November),
		Inflasi: &domain.InflasiData{Periode: bulanUji(2026, time.October), Mtm: 0.41, Yoy: 2.84, Ytd: 2.13, Sumber: "BPS"},
		Target:  &domain.TargetData{Tahun: 2026, Bawah: 1.5, Atas: 3.5},
		Andil: []domain.AndilData{
			{Komoditas: "Cabai merah keriting", Andil: 0.18, Sumber: "BPS"},
			{Komoditas: "Bawang merah", Andil: 0.06, Sumber: "BPS"},
		},
		Harga: []domain.HargaData{
			{Komoditas: "Beras medium", RataIni: 13000, RataLalu: 12950, PerubahanPct: 0.386},
			{Komoditas: "Cabai merah keriting", RataIni: 56000, RataLalu: 50000, PerubahanPct: 12},
			{Komoditas: "Tomat", RataIni: 9000, RataLalu: 10000, PerubahanPct: -10},
			{Komoditas: "Garam", RataIni: 8000, RataLalu: 8000, PerubahanPct: 0},
		},
		Ews: domain.EwsData{Total: 17, Kritis: 2, Siaga: 5, Waspada: 10, Teratas: []domain.AlertData{
			{Komoditas: "Cabai merah keriting", Wilayah: "KABUPATEN ACEH TENGAH", Level: "KRITIS"},
			{Komoditas: "", Wilayah: "KOTA BANDA ACEH", Level: "SIAGA"},
		}},
		Proyeksi: []domain.ProyeksiData{
			{Komoditas: "Beras medium", HorizonHari: 14, PerubahanPct: -2.4, Sumber: "MODEL"},
			{Komoditas: "Cabai merah keriting", HorizonHari: 14, PerubahanPct: 14.2, Sumber: "MODEL"},
		},
	}
}

func nilaiFakta(t *testing.T, k domain.Konteks, id string) string {
	t.Helper()
	f, ok := k.Cari(id)
	require.True(t, ok, "fakta %s tidak ada", id)
	return f.Nilai
}

func TestBangunKonteks_FormatDanUrutanFakta(t *testing.T) {
	k := BangunKonteks(dataLengkap())

	assert.Equal(t, "November 2026", nilaiFakta(t, k, domain.FaktaPeriode))
	assert.Equal(t, "Provinsi Aceh", nilaiFakta(t, k, domain.FaktaWilayah))
	assert.Equal(t, "Oktober 2026", nilaiFakta(t, k, domain.FaktaInflasiPeriode), "bulan data, bukan bulan brief")
	assert.Equal(t, "2,84%", nilaiFakta(t, k, domain.FaktaInflasiYoy))
	assert.Equal(t, "0,41%", nilaiFakta(t, k, domain.FaktaInflasiMtm))
	assert.Equal(t, "2,13%", nilaiFakta(t, k, domain.FaktaInflasiYtd))
	assert.Equal(t, "1,5%", nilaiFakta(t, k, domain.FaktaTargetBawah))
	assert.Equal(t, "3,5%", nilaiFakta(t, k, domain.FaktaTargetAtas))
	assert.Equal(t, "berada dalam koridor sasaran inflasi", nilaiFakta(t, k, domain.FaktaStatusSasaran))

	assert.Equal(t, "Cabai merah keriting", nilaiFakta(t, k, domain.IDFakta(domain.KelompokAndil, 1, "KOMODITAS")))
	assert.Equal(t, "0,18 poin persen", nilaiFakta(t, k, domain.IDFakta(domain.KelompokAndil, 1, "NILAI")))

	// Harga diurut menurut |perubahan| terbesar, bukan urutan masuk: cabai (+12), tomat (-10), beras (+0,4); garam (0) tidak masuk tiga teratas.
	assert.Equal(t, "Cabai merah keriting", nilaiFakta(t, k, domain.IDFakta(domain.KelompokHarga, 1, "KOMODITAS")))
	assert.Equal(t, "+12%", nilaiFakta(t, k, domain.IDFakta(domain.KelompokHarga, 1, "PERUBAHAN")))
	assert.Equal(t, "Rp56.000", nilaiFakta(t, k, domain.IDFakta(domain.KelompokHarga, 1, "RATA")))
	assert.Equal(t, "naik", nilaiFakta(t, k, domain.IDFakta(domain.KelompokHarga, 1, "ARAH")))
	assert.Equal(t, "Tomat", nilaiFakta(t, k, domain.IDFakta(domain.KelompokHarga, 2, "KOMODITAS")))
	assert.Equal(t, "-10%", nilaiFakta(t, k, domain.IDFakta(domain.KelompokHarga, 2, "PERUBAHAN")))
	assert.Equal(t, "turun", nilaiFakta(t, k, domain.IDFakta(domain.KelompokHarga, 2, "ARAH")))
	assert.Equal(t, "+0,4%", nilaiFakta(t, k, domain.IDFakta(domain.KelompokHarga, 3, "PERUBAHAN")))
	_, adaKeempat := k.Cari(domain.IDFakta(domain.KelompokHarga, 4, "KOMODITAS"))
	assert.False(t, adaKeempat)

	assert.Equal(t, "17", nilaiFakta(t, k, domain.FaktaEwsTotal))
	assert.Equal(t, "2", nilaiFakta(t, k, domain.FaktaEwsKritis))
	assert.Equal(t, "Kabupaten Aceh Tengah", nilaiFakta(t, k, domain.IDFakta(domain.KelompokEws, 1, "WILAYAH")))
	assert.Equal(t, "kritis", nilaiFakta(t, k, domain.IDFakta(domain.KelompokEws, 1, "LEVEL")))
	assert.Equal(t, "komoditas umum", nilaiFakta(t, k, domain.IDFakta(domain.KelompokEws, 2, "KOMODITAS")), "peringatan tanpa komoditas")

	assert.Equal(t, "14 hari", nilaiFakta(t, k, domain.FaktaProyeksiHoriz))
	assert.Equal(t, "model hasil pelatihan", nilaiFakta(t, k, domain.FaktaProyeksiAsal))
	assert.Equal(t, "Cabai merah keriting", nilaiFakta(t, k, domain.IDFakta(domain.KelompokProyeksi, 1, "KOMODITAS")), "proyeksi terbesar lebih dulu")
	assert.Equal(t, "+14,2%", nilaiFakta(t, k, domain.IDFakta(domain.KelompokProyeksi, 1, "PERUBAHAN")))

	_, adaCatatan := k.Cari(domain.FaktaCatatanData)
	assert.False(t, adaCatatan, "tanpa data demo tidak ada catatan keandalan")
}

func TestBangunKonteks_Deterministik(t *testing.T) {
	assert.Equal(t, BangunKonteks(dataLengkap()), BangunKonteks(dataLengkap()))
}

func TestBangunKonteks_IdFaktaUnik(t *testing.T) {
	ada := map[string]bool{}
	for _, f := range BangunKonteks(dataLengkap()).Fakta {
		assert.False(t, ada[f.ID], "id ganda: %s", f.ID)
		ada[f.ID] = true
	}
}

func TestBangunKonteks_SemuaFaktaBersumberKecualiPenanda(t *testing.T) {
	for _, f := range BangunKonteks(dataLengkap()).Fakta {
		if f.ID == domain.FaktaPeriode || f.ID == domain.FaktaWilayah {
			assert.Empty(t, f.Sumber, f.ID)
			continue
		}
		assert.NotEmpty(t, f.Sumber, "fakta %s harus punya sumber untuk lampiran sitasi", f.ID)
	}
}

func TestBangunKonteks_DataDemoMenambahCatatanKeandalan(t *testing.T) {
	d := dataLengkap()
	d.Inflasi.Sumber = "DEMO"

	k := BangunKonteks(d)

	assert.Contains(t, nilaiFakta(t, k, domain.FaktaCatatanData), "data demo")
	assert.Equal(t, "inflasi_bulanan (DEMO)", mustFakta(t, k, domain.FaktaInflasiYoy).Sumber)

	d = dataLengkap()
	d.Proyeksi[0].Sumber = "DEMO"
	k = BangunKonteks(d)
	assert.Contains(t, nilaiFakta(t, k, domain.FaktaCatatanData), "data demo")
	assert.Equal(t, "data demo, bukan keluaran model sungguhan", nilaiFakta(t, k, domain.FaktaProyeksiAsal))
}

func mustFakta(t *testing.T, k domain.Konteks, id string) domain.Fakta {
	t.Helper()
	f, ok := k.Cari(id)
	require.True(t, ok, id)
	return f
}

func TestBangunKonteks_TanpaDataHanyaPenanda(t *testing.T) {
	k := BangunKonteks(domain.DataPeriode{Periode: bulanUji(2026, time.December)})

	require.Len(t, k.Fakta, 2)
	assert.Equal(t, "Desember 2026", nilaiFakta(t, k, domain.FaktaPeriode))
}

func TestPosisiSasaran(t *testing.T) {
	target := domain.TargetData{Bawah: 1.5, Atas: 3.5}
	assert.Equal(t, "berada di atas batas atas sasaran inflasi", posisiSasaran(3.51, target))
	assert.Equal(t, "berada di bawah batas bawah sasaran inflasi", posisiSasaran(1.49, target))
	assert.Equal(t, "berada dalam koridor sasaran inflasi", posisiSasaran(3.5, target), "batas atas termasuk dalam koridor")
	assert.Equal(t, "berada dalam koridor sasaran inflasi", posisiSasaran(1.5, target))
}

func TestFormatAngka(t *testing.T) {
	assert.Equal(t, "2,84%", persen(2.84))
	assert.Equal(t, "2,5%", persen(2.5))
	assert.Equal(t, "0%", persen(0.001), "dibulatkan dua desimal")
	assert.Equal(t, "0%", persen(-0.001), "tanpa nol negatif")
	assert.Equal(t, "-0,31%", persen(-0.31))

	assert.Equal(t, "+12,3%", persenBertanda(12.34))
	assert.Equal(t, "-3,1%", persenBertanda(-3.14))
	assert.Equal(t, "0%", persenBertanda(0.01))

	assert.Equal(t, "Rp500", rupiah(500))
	assert.Equal(t, "Rp1.000", rupiah(1000))
	assert.Equal(t, "Rp123.456", rupiah(123456))
	assert.Equal(t, "Rp1.234.567", rupiah(1234567))

	assert.Equal(t, "naik", arah(0.05))
	assert.Equal(t, "stabil", arah(0.049))
	assert.Equal(t, "stabil", arah(-0.049))
	assert.Equal(t, "turun", arah(-0.05))

	assert.Equal(t, "Kota Banda Aceh", kapital("KOTA BANDA ACEH"))
}
