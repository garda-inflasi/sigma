package domain_test

import (
	"errors"
	"strings"
	"testing"

	coldstorage "github.com/garda-inflasi/api/internal/cold-storage/domain"
	"github.com/garda-inflasi/api/internal/dashboard/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalisasiSaringan_Valid(t *testing.T) {
	tabel := []struct {
		nama string
		req  domain.PermintaanSaringan
	}{
		{"kosong: tanpa filter", domain.PermintaanSaringan{}},
		{"kabupaten saja", domain.PermintaanSaringan{KabupatenKode: "1171"}},
		{"komoditas saja", domain.PermintaanSaringan{KomoditasKode: "CBM-KRT"}},
		{"keduanya", domain.PermintaanSaringan{KabupatenKode: "1108", KomoditasKode: "TMT"}},
		{"komoditas 2 karakter, batas bawah", domain.PermintaanSaringan{KomoditasKode: "AB"}},
		{"komoditas 30 karakter, batas atas", domain.PermintaanSaringan{KomoditasKode: "A" + strings.Repeat("B", 29)}},
		{"kode yang bentuknya benar tetapi tidak ada di master tidak ditolak", domain.PermintaanSaringan{KabupatenKode: "9999", KomoditasKode: "TIDAK-ADA"}},
	}
	for _, k := range tabel {
		t.Run(k.nama, func(t *testing.T) {
			s, err := domain.NormalisasiSaringan(k.req)

			require.NoError(t, err)
			assert.Equal(t, domain.Saringan{KabupatenKode: k.req.KabupatenKode, KomoditasKode: k.req.KomoditasKode}, s)
		})
	}
}

func TestNormalisasiSaringan_TidakValid(t *testing.T) {
	tabel := []struct {
		nama  string
		req   domain.PermintaanSaringan
		pesan string
	}{
		{"kabupaten bukan angka", domain.PermintaanSaringan{KabupatenKode: "abcd"}, "kabupaten_kode harus 4 digit"},
		{"kabupaten 3 digit", domain.PermintaanSaringan{KabupatenKode: "117"}, "kabupaten_kode harus 4 digit"},
		{"kabupaten 5 digit", domain.PermintaanSaringan{KabupatenKode: "11711"}, "kabupaten_kode harus 4 digit"},
		{"kabupaten dengan spasi", domain.PermintaanSaringan{KabupatenKode: "1171 "}, "kabupaten_kode harus 4 digit"},
		{"kabupaten dengan baris baru di belakang", domain.PermintaanSaringan{KabupatenKode: "1171\n"}, "kabupaten_kode harus 4 digit"},
		{"komoditas huruf kecil", domain.PermintaanSaringan{KomoditasKode: "cabai"}, "komoditas_kode harus berupa huruf besar"},
		{"komoditas dengan spasi", domain.PermintaanSaringan{KomoditasKode: "CBM KRT"}, "komoditas_kode harus berupa huruf besar"},
		{"komoditas 1 karakter", domain.PermintaanSaringan{KomoditasKode: "A"}, "komoditas_kode harus berupa huruf besar"},
		{"komoditas 31 karakter", domain.PermintaanSaringan{KomoditasKode: "A" + strings.Repeat("B", 30)}, "komoditas_kode harus berupa huruf besar"},
		{"komoditas diawali tanda hubung", domain.PermintaanSaringan{KomoditasKode: "-ABC"}, "komoditas_kode harus berupa huruf besar"},
		{"komoditas dengan kutip (upaya injeksi)", domain.PermintaanSaringan{KomoditasKode: "A'; DROP TABLE x;--"}, "komoditas_kode harus berupa huruf besar"},
	}
	for _, k := range tabel {
		t.Run(k.nama, func(t *testing.T) {
			_, err := domain.NormalisasiSaringan(k.req)

			require.Error(t, err)
			assert.True(t, errors.Is(err, domain.ErrValidasi))
			assert.Contains(t, err.Error(), k.pesan)
			// Isian pemanggil tidak dipantulkan di pesan galat (isian pendek bisa kebetulan sama dengan contoh di pesan).
			for _, isian := range []string{k.req.KabupatenKode, k.req.KomoditasKode} {
				if len(strings.TrimSpace(isian)) >= 6 {
					assert.NotContains(t, err.Error(), isian)
				}
			}
		})
	}
}

// Aturan bentuk kode sama persis dengan KPI dan heatmap: satu pemeriksa, satu pesan.
func TestNormalisasiSaringan_AturanSamaDenganFilterKpi(t *testing.T) {
	for _, kab := range []string{"abc", "117", "1171"} {
		for _, kom := range []string{"cabai", "CBM-KRT", ""} {
			_, errFilter := domain.NormalisasiFilter(domain.PermintaanFilter{KabupatenKode: kab, KomoditasKode: kom}, hariIni)
			_, errSaringan := domain.NormalisasiSaringan(domain.PermintaanSaringan{KabupatenKode: kab, KomoditasKode: kom})

			assert.Equal(t, errFilter == nil, errSaringan == nil, "kab=%q kom=%q", kab, kom)
			if errFilter != nil {
				assert.Equal(t, errFilter.Error(), errSaringan.Error(), "pesan sama: kab=%q kom=%q", kab, kom)
			}
		}
	}
}

// Definisi yang dipinjam harus tetap sama dengan modul pemiliknya: dashboard tidak boleh punya "lot", "risiko tinggi",
// atau "rute aktif" versinya sendiri.
func TestKonstantaPinjamanSamaDenganModulPemilik(t *testing.T) {
	assert.Equal(t, coldstorage.RisikoTinggiAmbang, domain.AmbangRisikoTinggi)
	assert.Equal(t, coldstorage.StatusLotHabis, domain.StatusLotHabis)
	assert.Equal(t, coldstorage.StatusFasilitasOperasional, domain.StatusFasilitasOperasional)
	assert.Equal(t, "MASUK", domain.JenisMasuk)
	assert.Equal(t, "KELUAR", domain.JenisKeluar)
	assert.Equal(t, "PINDAH", domain.JenisPindah)
	assert.Equal(t, "DIGANTIKAN", domain.StatusRuteDigantikan)
	assert.Equal(t, "11", domain.WilayahProvinsi, "kode wilayah provinsi Aceh")
	assert.Equal(t, "OPS_MAHASISWA", domain.PeranPetugas)
}

// "Pekan terakhir" berarti hal yang sama di seluruh dashboard operasional: KPI memakai jendela validasi 7 hari.
func TestJendelaOperasionalSamaDenganJendelaValidasiKpi(t *testing.T) {
	assert.Equal(t, domain.JendelaValidasiHari, domain.JendelaOperasionalHari)
}

func TestPergerakanFasilitas_AsalDanTujuan(t *testing.T) {
	str := func(s string) *string { return &s }
	masuk := domain.Pergerakan{
		TujuanID: str("uuid-1"), TujuanKode: str("CS-01"), TujuanNama: str("Cold Storage Lambaro"),
		TujuanKabupatenKode: str("1108"), TujuanKabupatenNama: str("KABUPATEN ACEH BESAR"),
	}
	assert.Nil(t, masuk.Asal(), "MASUK tidak punya fasilitas asal")
	require.NotNil(t, masuk.Tujuan())
	assert.Equal(t, domain.FasilitasRingkas{ID: "uuid-1", Kode: str("CS-01"), Nama: "Cold Storage Lambaro", KabupatenKode: "1108", KabupatenNama: "KABUPATEN ACEH BESAR"}, *masuk.Tujuan())

	keluarTanpaKode := domain.Pergerakan{AsalID: str("uuid-2"), AsalNama: str("CS Tanpa Kode"), AsalKabupatenKode: str("1106")}
	assert.Nil(t, keluarTanpaKode.Tujuan(), "KELUAR tidak punya fasilitas tujuan")
	require.NotNil(t, keluarTanpaKode.Asal())
	assert.Nil(t, keluarTanpaKode.Asal().Kode, "fasilitas boleh tanpa kode tampilan")
	assert.Equal(t, "CS Tanpa Kode", keluarTanpaKode.Asal().Nama)
}

func TestProdusenFakta_Produksi(t *testing.T) {
	ton, sumber := 3100.0, "BPS"
	periode := hariIni
	ada := domain.ProdusenFakta{ProduksiTon: &ton, ProduksiPeriode: &periode, ProduksiSumber: &sumber}
	require.NotNil(t, ada.Produksi())
	assert.Equal(t, 3100.0, ada.Produksi().Ton)
	assert.False(t, ada.Produksi().Demo(), "sumber resmi")

	demo := "DEMO"
	assert.True(t, domain.ProdusenFakta{ProduksiTon: &ton, ProduksiPeriode: &periode, ProduksiSumber: &demo}.Produksi().Demo())

	assert.Nil(t, domain.ProdusenFakta{}.Produksi(), "tanpa neraca: tidak ada produksi (bukan nol)")
	assert.Nil(t, domain.ProdusenFakta{ProduksiTon: &ton}.Produksi(), "sebagian terisi dianggap tidak ada, bukan produksi tanpa periode")
}
