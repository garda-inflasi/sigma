package usecase

import (
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/dashboard/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// acuan adalah tanggal acuan (hari ini) semua tes RakitKpi: 16 Oktober 2026.
var acuan = time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)

// periode membuat filter periode `hari` hari yang berakhir di acuan.
func periode(hari int) domain.Filter {
	return domain.Filter{Dari: acuan.AddDate(0, 0, -(hari - 1)), Sampai: acuan}
}

// fakta membuat satu titik harian per nilai, berakhir di acuan; nilai[i] dan penyebut[i] untuk hari ke-i dari yang
// terlama. penyebut boleh nil (tidak dipakai indikator itu).
func fakta(indikator string, pembilang []float64, penyebut []float64) []domain.TitikKpi {
	out := make([]domain.TitikKpi, len(pembilang))
	for i, p := range pembilang {
		out[i] = domain.TitikKpi{Indikator: indikator, Tanggal: acuan.AddDate(0, 0, i-len(pembilang)+1), Pembilang: p}
		if penyebut != nil {
			out[i].Penyebut = penyebut[i]
		}
	}
	return out
}

// seragam mengulang satu nilai sebanyak n kali.
func seragam(nilai float64, n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = nilai
	}
	return out
}

func naik(dari, sampai float64, n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = dari + (sampai-dari)*float64(i)/float64(n-1)
	}
	return out
}

func kartu(kpis []domain.Kpi, id string) domain.Kpi {
	for _, k := range kpis {
		if k.ID == id {
			return k
		}
	}
	panic("kartu tidak ada: " + id)
}

func TestRakitKpi_UrutanLabelDanSparkline(t *testing.T) {
	// 20 hari fakta: 14 hari tren ditambah 6 hari di belakangnya untuk jendela validasi.
	var titik []domain.TitikKpi
	for _, indikator := range []string{domain.IndikatorCS, domain.IndikatorEntri, domain.IndikatorWaspada, domain.IndikatorValidasi} {
		titik = append(titik, fakta(indikator, naik(1, 20, 20), seragam(100, 20))...)
	}

	kpis := RakitKpi(titik, periode(14), acuan, nil)

	require.Len(t, kpis, 4)
	assert.Equal(t, []string{"entri", "validasi", "waspada", "cs"}, []string{kpis[0].ID, kpis[1].ID, kpis[2].ID, kpis[3].ID})
	for _, k := range kpis {
		assert.Len(t, k.Tren, 14, k.ID)
		assert.Equal(t, "data", k.Tone, "tanpa target warna netral: %s", k.ID)
		assert.NotEmpty(t, k.Label)
		assert.NotEmpty(t, k.Href)
	}
	assert.Equal(t, 20.0, kartu(kpis, "entri").Nilai, "nilai adalah titik tanggal acuan")
	assert.Equal(t, 7.0, kartu(kpis, "entri").Tren[0], "tren dimulai 13 hari sebelum tanggal acuan")
	assert.Equal(t, 20.0, kartu(kpis, "waspada").Nilai)
	assert.Equal(t, 20.0, kartu(kpis, "cs").Nilai, "cs: 100 x 20 / 100")
	assert.Equal(t, 17.0, kartu(kpis, "validasi").Nilai, "validasi: jendela 7 hari terakhir, (14+...+20) / 700 x 100")
	assert.Equal(t, "Entri harga hari ini", kartu(kpis, "entri").Label)
	assert.Equal(t, "/data/harga?status=DIAJUKAN", kartu(kpis, "validasi").Href)
}

func TestRakitKpi_UrutanTitikTidakMemengaruhiHasil(t *testing.T) {
	var titik []domain.TitikKpi
	for _, indikator := range []string{domain.IndikatorEntri, domain.IndikatorValidasi, domain.IndikatorCS} {
		titik = append(titik, fakta(indikator, naik(1, 20, 20), seragam(40, 20))...)
	}
	want := RakitKpi(titik, periode(14), acuan, nil)

	acak := append([]domain.TitikKpi(nil), titik...)
	rand.New(rand.NewSource(1)).Shuffle(len(acak), func(i, j int) { acak[i], acak[j] = acak[j], acak[i] })

	assert.Equal(t, want, RakitKpi(acak, periode(14), acuan, nil))
}

func TestRakitKpi_SelaluEmpatKartuBernilaiNolTanpaData(t *testing.T) {
	kpis := RakitKpi(nil, periode(14), acuan, nil)

	require.Len(t, kpis, 4, "basis data kosong atau irisan kosong tetap menghasilkan empat kartu")
	for _, k := range kpis {
		assert.Zero(t, k.Nilai, k.ID)
		assert.Zero(t, k.Delta, k.ID)
		assert.Equal(t, seragam(0, 14), k.Tren, k.ID)
		assert.Equal(t, "data", k.Tone, k.ID)
	}
}

func TestRakitKpi_ValidasiDariPembilangDanPenyebutBukanRataRataPersen(t *testing.T) {
	// Kemarin 1 dari 1 entri tervalidasi (100%), hari ini 0 dari 9 (0%). Rata-rata persen harian = 50%; persentase
	// jendela yang benar = (1+0) / (1+9) = 10%.
	pembilang := append(seragam(0, 18), 1, 0)
	penyebut := append(seragam(0, 18), 1, 9)

	k := kartu(RakitKpi(fakta(domain.IndikatorValidasi, pembilang, penyebut), periode(14), acuan, nil), "validasi")

	assert.Equal(t, 10.0, k.Nilai)
	assert.Equal(t, 10.0, k.Tren[len(k.Tren)-1])
	assert.Equal(t, 100.0, k.Tren[len(k.Tren)-2], "kemarin: jendelanya hanya memuat 1 dari 1 entri")
}

func TestRakitKpi_ValidasiJendelaTujuhHariTepat(t *testing.T) {
	// Entri tervalidasi hanya pada hari ke-7 sebelum tanggal acuan: sudah di luar jendela hari ini, masih di dalam kemarin.
	pembilang := seragam(0, 20)
	penyebut := seragam(0, 20)
	pembilang[12], penyebut[12] = 5, 10 // acuan - 7
	k := kartu(RakitKpi(fakta(domain.IndikatorValidasi, pembilang, penyebut), periode(14), acuan, nil), "validasi")

	n := len(k.Tren)
	assert.Zero(t, k.Nilai, "jendela hari ini acuan-6..acuan: entri di acuan-7 sudah di luar")
	assert.Equal(t, 50.0, k.Tren[n-2], "kemarin: jendela acuan-7..acuan-1 memuat entri itu pada hari pertamanya")
	assert.Equal(t, 50.0, k.Tren[n-8], "acuan-7: entri itu pada hari terakhir jendela acuan-13..acuan-7")
	assert.Zero(t, k.Tren[n-9], "acuan-8: entri itu belum ada")
}

func TestRakitKpi_ValidasiTanpaEntriNol(t *testing.T) {
	k := kartu(RakitKpi(fakta(domain.IndikatorValidasi, seragam(0, 20), seragam(0, 20)), periode(14), acuan, nil), "validasi")
	assert.Zero(t, k.Nilai, "tanpa entri tak ada pembagian dengan nol")
}

func TestRakitKpi_CSAdalahVolumeTerhadapKapasitas(t *testing.T) {
	titik := fakta(domain.IndikatorCS, append(seragam(10, 19), 82.5), append(seragam(100, 19), 110))

	k := kartu(RakitKpi(titik, periode(14), acuan, nil), "cs")

	assert.Equal(t, 75.0, k.Nilai, "100 x 82,5 / 110")
	assert.Equal(t, 10.0, k.Tren[0], "100 x 10 / 100")
	assert.Equal(t, 65.0, k.Delta, "poin terhadap sepekan lalu: 75 - 10")

	t.Run("tanpa kapasitas nol, bukan tak terhingga", func(t *testing.T) {
		k := kartu(RakitKpi(fakta(domain.IndikatorCS, seragam(5, 20), seragam(0, 20)), periode(14), acuan, nil), "cs")
		assert.Zero(t, k.Nilai)
	})
	t.Run("hari tanpa baris (belum ada penyegaran) nol", func(t *testing.T) {
		titik := fakta(domain.IndikatorCS, seragam(5, 20), seragam(10, 20))[:19] // sampai kemarin; hari ini belum ada
		k := kartu(RakitKpi(titik, periode(14), acuan, nil), "cs")
		assert.Zero(t, k.Nilai, "tanggal acuan tanpa titik")
		assert.Equal(t, 50.0, k.Tren[len(k.Tren)-2])
	})
}

func TestRakitKpi_WaspadaDanEntriMemakaiPembilangApaAdanya(t *testing.T) {
	titik := append(fakta(domain.IndikatorWaspada, append(seragam(0, 19), 3), nil), fakta(domain.IndikatorEntri, append(seragam(0, 19), 128), nil)...)

	kpis := RakitKpi(titik, periode(14), acuan, nil)

	assert.Equal(t, 3.0, kartu(kpis, "waspada").Nilai)
	assert.Equal(t, 128.0, kartu(kpis, "entri").Nilai)
}

func TestRakitKpi_HariTanpaTitikDiisiNol(t *testing.T) {
	// Entri hanya pada 2 dari 14 hari.
	titik := []domain.TitikKpi{
		{Indikator: domain.IndikatorEntri, Tanggal: acuan, Pembilang: 7},
		{Indikator: domain.IndikatorEntri, Tanggal: acuan.AddDate(0, 0, -5), Pembilang: 3},
	}

	k := kartu(RakitKpi(titik, periode(14), acuan, nil), "entri")

	want := seragam(0, 14)
	want[13], want[8] = 7, 3
	assert.Equal(t, want, k.Tren)
}

func TestRakitKpi_TrenMengikutiPeriode(t *testing.T) {
	// Nilai entri tiap hari sama dengan tanggal kalendernya: urutan dan batas periode mudah dibuktikan.
	var titik []domain.TitikKpi
	for d := acuan.AddDate(0, 0, -40); !d.After(acuan); d = d.AddDate(0, 0, 1) {
		titik = append(titik, domain.TitikKpi{Indikator: domain.IndikatorEntri, Tanggal: d, Pembilang: float64(d.Day())})
	}

	t.Run("periode 3 hari", func(t *testing.T) {
		k := kartu(RakitKpi(titik, periode(3), acuan, nil), "entri")
		assert.Equal(t, []float64{14, 15, 16}, k.Tren)
		assert.Equal(t, 16.0, k.Nilai)
	})
	t.Run("periode 30 hari melintasi pergantian bulan", func(t *testing.T) {
		k := kartu(RakitKpi(titik, periode(30), acuan, nil), "entri")
		require.Len(t, k.Tren, 30)
		assert.Equal(t, 17.0, k.Tren[0], "17 September")
		assert.Equal(t, 30.0, k.Tren[13], "30 September")
		assert.Equal(t, 1.0, k.Tren[14], "1 Oktober")
		assert.Equal(t, 16.0, k.Tren[29])
	})
	t.Run("tanggal acuan di masa lalu", func(t *testing.T) {
		sampai := acuan.AddDate(0, 0, -10) // 6 Oktober
		f := domain.Filter{Dari: sampai.AddDate(0, 0, -6), Sampai: sampai}
		k := kartu(RakitKpi(titik, f, acuan, nil), "entri")
		assert.Equal(t, []float64{30, 1, 2, 3, 4, 5, 6}, k.Tren)
		assert.Equal(t, 6.0, k.Nilai, "nilai adalah nilai pada tanggal acuan, bukan hari ini")
	})
}

func TestRakitKpi_DeltaTidakBergantungPadaPanjangPeriode(t *testing.T) {
	var titik []domain.TitikKpi
	titik = append(titik, fakta(domain.IndikatorEntri, append(seragam(100, 13), seragam(110, 7)...), nil)...)
	titik = append(titik, fakta(domain.IndikatorValidasi, append(seragam(82.5, 13), seragam(87.5, 7)...), seragam(100, 20))...)
	titik = append(titik, fakta(domain.IndikatorWaspada, append(seragam(0, 18), 2, 5), nil)...)

	panjang := RakitKpi(titik, periode(14), acuan, nil)
	pendek := RakitKpi(titik, periode(2), acuan, nil)

	require.Len(t, kartu(pendek, "entri").Tren, 2)
	for _, id := range []string{"entri", "validasi", "waspada"} {
		assert.Equal(t, kartu(panjang, id).Delta, kartu(pendek, id).Delta, "delta %s dibandingkan terhadap tanggal acuan, bukan awal periode", id)
		assert.Equal(t, kartu(panjang, id).Nilai, kartu(pendek, id).Nilai, id)
	}
	assert.Equal(t, 10.0, kartu(pendek, "entri").Delta)
	assert.Equal(t, 3.0, kartu(pendek, "waspada").Delta)
}

func TestRakitKpi_DeltaPerIndikator(t *testing.T) {
	t.Run("entri: persen 7 hari terakhir terhadap 7 hari sebelumnya", func(t *testing.T) {
		nilai := append(seragam(100, 13), seragam(110, 7)...)
		assert.Equal(t, 10.0, kartu(RakitKpi(fakta(domain.IndikatorEntri, nilai, nil), periode(14), acuan, nil), "entri").Delta)
	})
	t.Run("entri: pembanding kosong memberi nol", func(t *testing.T) {
		nilai := append(seragam(0, 13), seragam(5, 7)...)
		assert.Zero(t, kartu(RakitKpi(fakta(domain.IndikatorEntri, nilai, nil), periode(14), acuan, nil), "entri").Delta)
	})
	t.Run("validasi: poin terhadap sepekan lalu", func(t *testing.T) {
		pembilang := append(seragam(82.5, 13), seragam(87.5, 7)...)
		k := kartu(RakitKpi(fakta(domain.IndikatorValidasi, pembilang, seragam(100, 20)), periode(14), acuan, nil), "validasi")
		assert.Equal(t, 5.0, k.Delta)
	})
	t.Run("waspada: selisih dengan kemarin", func(t *testing.T) {
		assert.Equal(t, 3.0, kartu(RakitKpi(fakta(domain.IndikatorWaspada, append(seragam(0, 18), 2, 5), nil), periode(14), acuan, nil), "waspada").Delta)
	})
}

// delta bekerja pada deret nilai harian; deret pendek (kurang dari yang dibutuhkan pembanding) memberi nol.
func TestDelta_DeretPendekMemberiNol(t *testing.T) {
	assert.Zero(t, delta(domain.IndikatorCS, []float64{10, 20, 30}))
	assert.Zero(t, delta(domain.IndikatorWaspada, []float64{4}))
	assert.Zero(t, delta(domain.IndikatorEntri, []float64{1, 2, 3}))
}

func TestRakitKpi_NadaDariTarget(t *testing.T) {
	cases := []struct {
		nama      string
		indikator string
		nilai     float64
		target    domain.TargetKpi
		tone      string
	}{
		{"naik tercapai", domain.IndikatorValidasi, 90, domain.TargetKpi{Indikator: "validasi", Target: 85, Arah: domain.ArahNaik}, "normal"},
		{"naik hampir", domain.IndikatorValidasi, 70, domain.TargetKpi{Indikator: "validasi", Target: 85, Arah: domain.ArahNaik}, "waspada"},
		{"naik jauh", domain.IndikatorValidasi, 50, domain.TargetKpi{Indikator: "validasi", Target: 85, Arah: domain.ArahNaik}, "defisit"},
		{"turun tercapai", domain.IndikatorWaspada, 2, domain.TargetKpi{Indikator: "waspada", Target: 3, Arah: domain.ArahTurun}, "normal"},
		{"turun nol selalu baik", domain.IndikatorWaspada, 0, domain.TargetKpi{Indikator: "waspada", Target: 3, Arah: domain.ArahTurun}, "normal"},
		{"turun melewati batas", domain.IndikatorWaspada, 6, domain.TargetKpi{Indikator: "waspada", Target: 3, Arah: domain.ArahTurun}, "defisit"},
	}
	for _, c := range cases {
		t.Run(c.nama, func(t *testing.T) {
			// Validasi: penyebut 100 di semua hari, jadi persen jendela = nilai.
			kpis := RakitKpi(fakta(c.indikator, seragam(c.nilai, 20), seragam(100, 20)), periode(14), acuan, []domain.TargetKpi{c.target})
			assert.Equal(t, c.tone, kartu(kpis, c.indikator).Tone)
			assert.Equal(t, "data", kartu(kpis, domain.IndikatorEntri).Tone, "kartu tanpa target tetap netral")
		})
	}
}

func TestRakitKpi_Pembulatan(t *testing.T) {
	k := kartu(RakitKpi(fakta(domain.IndikatorValidasi, seragam(8745.67, 20), seragam(10000, 20)), periode(14), acuan, nil), "validasi")

	assert.Equal(t, 87.5, k.Nilai)
	assert.Equal(t, seragam(87.5, 14), k.Tren)
}

func TestRakitKpi_LabelEntriMengikutiTanggalAcuan(t *testing.T) {
	assert.Equal(t, "Entri harga hari ini", kartu(RakitKpi(nil, periode(14), acuan, nil), "entri").Label)

	sampai := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	f := domain.Filter{Dari: sampai.AddDate(0, 0, -13), Sampai: sampai}
	kpis := RakitKpi(nil, f, acuan, nil)

	assert.Equal(t, "Entri harga 5 Nov 2026", kartu(kpis, "entri").Label, "label tidak boleh bilang 'hari ini' untuk tanggal lampau")
	assert.Equal(t, "Tingkat validasi", kartu(kpis, "validasi").Label, "label lain netral terhadap tanggal")
}

func TestTanggalID(t *testing.T) {
	assert.Equal(t, "1 Jan 2026", tanggalID(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)))
	assert.Equal(t, "17 Mei 2026", tanggalID(time.Date(2026, 5, 17, 0, 0, 0, 0, time.UTC)))
	assert.Equal(t, "31 Des 2026", tanggalID(time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)))
}

func TestTingkatAceh(t *testing.T) {
	semua, kab := domain.CakupanSemua(), domain.CakupanKabupaten("1171")
	tabel := []struct {
		nama    string
		cakupan domain.Cakupan
		f       domain.Filter
		want    bool
	}{
		{"seluruh Aceh tanpa filter", semua, domain.Filter{}, true},
		{"seluruh Aceh, hanya periode", semua, periode(7), true},
		{"seluruh Aceh dengan filter kabupaten", semua, domain.Filter{KabupatenKode: "1171"}, false},
		{"seluruh Aceh dengan filter komoditas", semua, domain.Filter{KomoditasKode: "CBM-KRT"}, false},
		{"cakupan kabupaten walau tanpa filter", kab, domain.Filter{}, false},
		{"cakupan tertutup", domain.Cakupan{}, domain.Filter{}, false},
	}
	for _, k := range tabel {
		assert.Equal(t, k.want, TingkatAceh(k.cakupan, k.f), k.nama)
	}
}

// math.Round(-0.4) menghasilkan -0, yang ditulis JSON sebagai "-0": hasil bulat nol harus selalu +0.
func TestBulatkan_NolTidakPernahNegatif(t *testing.T) {
	for _, v := range []float64{-0.04, -0.0001, -0.049999, 0, 0.04} {
		got := bulatkan(v)
		assert.Zero(t, got, "%v", v)
		assert.False(t, math.Signbit(got), "bulatkan(%v) tidak boleh -0", v)
	}
	assert.Equal(t, -0.1, bulatkan(-0.05), "nilai yang tidak nol tetap bertanda")
	assert.Equal(t, 12.3, bulatkan(12.34))
	assert.Equal(t, 0.1, bulatkan(0.05))
}
