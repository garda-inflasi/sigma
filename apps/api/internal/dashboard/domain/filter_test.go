package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/dashboard/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// hariIni adalah 10 November 2026 (tanggal Aceh), sebagai nilai DATE.
var hariIni = time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)

func tgl(s string) time.Time {
	t, err := time.Parse(domain.TanggalLayout, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestNormalisasiFilter_Bawaan(t *testing.T) {
	f, err := domain.NormalisasiFilter(domain.PermintaanFilter{}, hariIni)

	require.NoError(t, err)
	assert.Equal(t, hariIni, f.Sampai, "tanggal acuan bawaan: hari ini")
	assert.Equal(t, tgl("2026-10-28"), f.Dari, "periode bawaan 14 hari termasuk hari ini, seperti tren sebelum filter ada")
	assert.Equal(t, domain.TrenHariBawaan, domain.HariAntara(f.Dari, f.Sampai))
	assert.False(t, f.Periode)
	assert.Empty(t, f.KabupatenKode)
	assert.Empty(t, f.KomoditasKode)
}

func TestNormalisasiFilter_HariIniDariJamBerapaPun(t *testing.T) {
	// hariIni dari pemanggil yang lupa memotong jam tidak menggeser tanggal acuan.
	f, err := domain.NormalisasiFilter(domain.PermintaanFilter{}, time.Date(2026, 11, 10, 17, 30, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.Equal(t, hariIni, f.Sampai)
}

func TestNormalisasiFilter_Valid(t *testing.T) {
	tabel := []struct {
		nama                 string
		req                  domain.PermintaanFilter
		wantDari, wantSampai string
		wantPeriode          bool
	}{
		{"hanya sampai: dari 13 hari sebelumnya", domain.PermintaanFilter{Sampai: "2026-11-05"}, "2026-10-23", "2026-11-05", true},
		{"hanya dari: sampai hari ini", domain.PermintaanFilter{Dari: "2026-11-01"}, "2026-11-01", "2026-11-10", true},
		{"dari dan sampai", domain.PermintaanFilter{Dari: "2026-11-01", Sampai: "2026-11-05"}, "2026-11-01", "2026-11-05", true},
		{"satu hari", domain.PermintaanFilter{Dari: "2026-11-10", Sampai: "2026-11-10"}, "2026-11-10", "2026-11-10", true},
		{"rentang 90 hari persis, tepat di awal riwayat", domain.PermintaanFilter{Dari: "2026-08-13", Sampai: "2026-11-10"}, "2026-08-13", "2026-11-10", true},
		{"sampai di awal riwayat: dari bawaan dipangkas ke awal riwayat", domain.PermintaanFilter{Sampai: "2026-08-13"}, "2026-08-13", "2026-08-13", true},
		{"sampai dua hari setelah awal riwayat: dari bawaan dipangkas", domain.PermintaanFilter{Sampai: "2026-08-15"}, "2026-08-13", "2026-08-15", true},
	}
	for _, k := range tabel {
		t.Run(k.nama, func(t *testing.T) {
			f, err := domain.NormalisasiFilter(k.req, hariIni)
			require.NoError(t, err)
			assert.Equal(t, tgl(k.wantDari), f.Dari)
			assert.Equal(t, tgl(k.wantSampai), f.Sampai)
			assert.Equal(t, k.wantPeriode, f.Periode)
		})
	}
}

func TestNormalisasiFilter_KodeValid(t *testing.T) {
	f, err := domain.NormalisasiFilter(domain.PermintaanFilter{KabupatenKode: "1171", KomoditasKode: "CBM-KRT"}, hariIni)
	require.NoError(t, err)
	assert.Equal(t, "1171", f.KabupatenKode)
	assert.Equal(t, "CBM-KRT", f.KomoditasKode)
	assert.False(t, f.Periode, "filter kabupaten dan komoditas bukan periode")

	for _, kode := range []string{"TMT", "BWM", "DAG-SPI", "X1"} {
		_, err := domain.NormalisasiFilter(domain.PermintaanFilter{KomoditasKode: kode}, hariIni)
		assert.NoError(t, err, kode)
	}
}

func TestNormalisasiFilter_Tolak(t *testing.T) {
	tabel := []struct {
		nama  string
		req   domain.PermintaanFilter
		pesan string // potongan pesan yang jelas bagi pemanggil
	}{
		{"kabupaten bukan angka", domain.PermintaanFilter{KabupatenKode: "abcd"}, "kabupaten_kode"},
		{"kabupaten terlalu pendek", domain.PermintaanFilter{KabupatenKode: "117"}, "kabupaten_kode"},
		{"kabupaten terlalu panjang", domain.PermintaanFilter{KabupatenKode: "11711"}, "kabupaten_kode"},
		{"kabupaten dengan spasi", domain.PermintaanFilter{KabupatenKode: " 1171"}, "kabupaten_kode"},
		{"kabupaten berupa suntikan SQL", domain.PermintaanFilter{KabupatenKode: "1171' OR '1'='1"}, "kabupaten_kode"},
		{"komoditas huruf kecil", domain.PermintaanFilter{KomoditasKode: "cbm-krt"}, "komoditas_kode"},
		{"komoditas dengan simbol", domain.PermintaanFilter{KomoditasKode: "CBM_KRT"}, "komoditas_kode"},
		{"komoditas terlalu panjang", domain.PermintaanFilter{KomoditasKode: "ABCDEFGHIJKLMNOPQRSTUVWXYZABCDE"}, "komoditas_kode"},
		{"komoditas diawali tanda hubung", domain.PermintaanFilter{KomoditasKode: "-ABC"}, "komoditas_kode"},
		{"dari bukan tanggal", domain.PermintaanFilter{Dari: "kemarin"}, "dari harus berformat YYYY-MM-DD"},
		{"dari tanpa nol di depan", domain.PermintaanFilter{Dari: "2026-1-5"}, "dari harus berformat YYYY-MM-DD"},
		{"dari tanggal tidak ada", domain.PermintaanFilter{Dari: "2026-02-30"}, "dari harus berformat YYYY-MM-DD"},
		{"dari bulan tidak ada", domain.PermintaanFilter{Dari: "2026-13-01"}, "dari harus berformat YYYY-MM-DD"},
		{"dari dengan jam", domain.PermintaanFilter{Dari: "2026-11-01T00:00:00Z"}, "dari harus berformat YYYY-MM-DD"},
		{"sampai bukan tanggal", domain.PermintaanFilter{Sampai: "10/11/2026"}, "sampai harus berformat YYYY-MM-DD"},
		{"sampai di masa depan", domain.PermintaanFilter{Sampai: "2026-11-11"}, "sampai tidak boleh setelah hari ini"},
		{"dari di masa depan (sampai bawaan hari ini)", domain.PermintaanFilter{Dari: "2026-11-11"}, "dari tidak boleh setelah hari ini"},
		{"dari setelah sampai", domain.PermintaanFilter{Dari: "2026-11-06", Sampai: "2026-11-05"}, "dari tidak boleh setelah sampai"},
		{"rentang 91 hari", domain.PermintaanFilter{Dari: "2026-08-12", Sampai: "2026-11-10"}, "paling lama 90 hari"},
		{"rentang jauh melebihi batas", domain.PermintaanFilter{Dari: "2025-01-01", Sampai: "2026-11-10"}, "paling lama 90 hari"},
		{"periode lampau di luar riwayat", domain.PermintaanFilter{Dari: "2026-01-01", Sampai: "2026-01-31"}, "di luar riwayat dashboard"},
		{"dari sehari sebelum awal riwayat", domain.PermintaanFilter{Dari: "2026-08-12", Sampai: "2026-08-20"}, "di luar riwayat dashboard"},
		{"sampai di luar riwayat tanpa dari", domain.PermintaanFilter{Sampai: "2026-08-12"}, "di luar riwayat dashboard"},
	}
	for _, k := range tabel {
		t.Run(k.nama, func(t *testing.T) {
			_, err := domain.NormalisasiFilter(k.req, hariIni)
			require.Error(t, err)
			assert.True(t, errors.Is(err, domain.ErrValidasi), "galat harus dibungkus ErrValidasi (jadi 400): %v", err)
			assert.Contains(t, err.Error(), k.pesan)
			assert.NotContains(t, err.Error(), "OR '1'", "pesan tidak memantulkan isian pemanggil")
		})
	}
}

// Setiap periode yang lolos validasi harus punya data pembanding dan jendela validasi di dalam riwayat fakta
// mv_kpi_operasional (migrasi 000124): kalau tidak, tren di tepi periode terpotong tanpa tanda.
func TestRiwayatMVMencukupi(t *testing.T) {
	// Titik tren paling awal yang dihitung: dari paling awal (hari ini - 89), atau 13 hari sebelum sampai paling
	// awal (sampai >= dari), mana yang lebih dulu; ditambah 6 hari jendela validasi di belakangnya.
	dariPalingAwal := domain.RiwayatHari - 1
	bandingPalingAwal := domain.RiwayatHari - 1 + domain.BandingHari
	terjauh := max(dariPalingAwal, bandingPalingAwal) + domain.JendelaValidasiHari - 1
	assert.LessOrEqual(t, terjauh, domain.RiwayatMVHari-1, "fakta harus mencakup hari ke-%d sebelum hari ini", terjauh)

	// Bukti langsung: periode paling tua yang diterima (sampai = dari = awal riwayat).
	awalRiwayat := hariIni.AddDate(0, 0, -(domain.RiwayatHari - 1))
	f, err := domain.NormalisasiFilter(domain.PermintaanFilter{Dari: awalRiwayat.Format(domain.TanggalLayout), Sampai: awalRiwayat.Format(domain.TanggalLayout)}, hariIni)
	require.NoError(t, err)
	awalFakta := f.Sampai.AddDate(0, 0, -domain.BandingHari-(domain.JendelaValidasiHari-1))
	assert.False(t, awalFakta.Before(hariIni.AddDate(0, 0, -(domain.RiwayatMVHari-1))),
		"fakta yang dibutuhkan mulai %s, riwayat view mulai %s", awalFakta.Format(domain.TanggalLayout),
		hariIni.AddDate(0, 0, -(domain.RiwayatMVHari-1)).Format(domain.TanggalLayout))
}

func TestHariAntara(t *testing.T) {
	assert.Equal(t, 1, domain.HariAntara(tgl("2026-11-10"), tgl("2026-11-10")))
	assert.Equal(t, 14, domain.HariAntara(tgl("2026-10-28"), tgl("2026-11-10")))
	assert.Equal(t, 90, domain.HariAntara(tgl("2026-08-13"), tgl("2026-11-10")))
	// Melintasi pergantian tahun dan tahun kabisat.
	assert.Equal(t, 3, domain.HariAntara(tgl("2023-12-31"), tgl("2024-01-02")))
	assert.Equal(t, 2, domain.HariAntara(tgl("2024-02-28"), tgl("2024-02-29")))
}
