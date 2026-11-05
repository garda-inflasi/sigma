package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	t0     = time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	durasi = 15 * time.Minute
)

// catat menerapkan n percobaan berturut-turut, satu menit terpisah, mulai dari awal; mengembalikan
// keadaan akhir dan waktu percobaan terakhir.
func catat(t *testing.T, k KunciLogin, awal time.Time, n int) (KunciLogin, time.Time) {
	t.Helper()
	now := awal
	for i := 0; i < n; i++ {
		var boleh bool
		k, boleh = k.CatatPercobaan(now, 5, durasi)
		require.True(t, boleh, "percobaan %d harus diizinkan", i+1)
		if i < n-1 {
			now = now.Add(time.Minute)
		}
	}
	return k, now
}

func TestCatatPercobaan_KunciDipasangPadaPercobaanKeN(t *testing.T) {
	k, akhir := catat(t, KunciLogin{}, t0, 4)
	assert.Equal(t, 4, k.Gagal)
	assert.Nil(t, k.TerkunciSampai, "empat percobaan belum mengunci")
	assert.False(t, k.Terkunci(akhir))

	k, akhir = catat(t, k, akhir.Add(time.Minute), 1)
	assert.Equal(t, 5, k.Gagal)
	require.NotNil(t, k.TerkunciSampai)
	assert.Equal(t, akhir.Add(durasi), *k.TerkunciSampai, "kunci berlaku satu durasi sejak percobaan ke-N")
	assert.True(t, k.Terkunci(akhir))
}

func TestCatatPercobaan_SelamaTerkunciDitolakDanTidakMemperpanjang(t *testing.T) {
	k, akhir := catat(t, KunciLogin{}, t0, 5)
	sampai := *k.TerkunciSampai

	for _, selisih := range []time.Duration{time.Second, 5 * time.Minute, durasi - time.Second} {
		baru, boleh := k.CatatPercobaan(akhir.Add(selisih), 5, durasi)
		assert.False(t, boleh, "percobaan %v setelah terkunci harus ditolak", selisih)
		assert.Equal(t, k, baru, "keadaan tidak berubah")
		assert.Equal(t, sampai, *baru.TerkunciSampai, "kunci tidak diperpanjang")
	}
}

func TestCatatPercobaan_SesudahKunciBerakhirHitunganMulaiDariNol(t *testing.T) {
	k, akhir := catat(t, KunciLogin{}, t0, 5)
	sampai := *k.TerkunciSampai
	assert.False(t, k.Terkunci(sampai), "tepat pada akhir kunci sudah tidak terkunci")
	assert.True(t, k.Terkunci(sampai.Add(-time.Nanosecond)))
	_ = akhir

	baru, boleh := k.CatatPercobaan(sampai, 5, durasi)
	require.True(t, boleh)
	assert.Equal(t, 1, baru.Gagal)
	assert.Nil(t, baru.TerkunciSampai)
	assert.Equal(t, sampai, baru.JendelaMulai)
}

func TestCatatPercobaan_JendelaKedaluwarsaMemulaiHitunganBaru(t *testing.T) {
	k, akhir := catat(t, KunciLogin{}, t0, 4)

	// Tepat satu durasi setelah percobaan gagal pertama, jendela itu habis.
	baru, boleh := k.CatatPercobaan(t0.Add(durasi), 5, durasi)
	require.True(t, boleh)
	assert.Equal(t, 1, baru.Gagal, "percobaan di luar jendela dihitung dari satu")
	assert.Nil(t, baru.TerkunciSampai)

	// Sebelum jendela habis, percobaan berikutnya masih menambah hitungan yang sama.
	dalam, boleh := k.CatatPercobaan(t0.Add(durasi-time.Second), 5, durasi)
	require.True(t, boleh)
	assert.Equal(t, 5, dalam.Gagal)
	require.NotNil(t, dalam.TerkunciSampai)
	_ = akhir
}

func TestCatatPercobaan_BarisKosongDimulaiDariWaktuSekarang(t *testing.T) {
	// Baris yang baru disisipkan repositori: gagal 0 dengan jendela mulai sembarang lampau.
	k := KunciLogin{JendelaMulai: t0.Add(-time.Hour)}
	baru, boleh := k.CatatPercobaan(t0, 5, durasi)
	require.True(t, boleh)
	assert.Equal(t, 1, baru.Gagal)
	assert.Equal(t, t0, baru.JendelaMulai, "jendela berawal dari percobaan pertama, bukan dari sisa baris lama")
}

func TestCatatPercobaan_BatasSatuLangsungMengunci(t *testing.T) {
	baru, boleh := KunciLogin{}.CatatPercobaan(t0, 1, durasi)
	require.True(t, boleh, "percobaan itu sendiri tetap diperiksa")
	require.NotNil(t, baru.TerkunciSampai)
	assert.Equal(t, t0.Add(durasi), *baru.TerkunciSampai)
}

func TestKembalikan(t *testing.T) {
	t.Run("mengurangi satu hitungan", func(t *testing.T) {
		k, akhir := catat(t, KunciLogin{}, t0, 3)
		baru := k.Kembalikan(5, akhir)
		assert.Equal(t, 2, baru.Gagal)
		assert.Nil(t, baru.TerkunciSampai)
	})
	t.Run("mencabut kunci yang dipasang percobaan itu sendiri", func(t *testing.T) {
		k, akhir := catat(t, KunciLogin{}, t0, 5)
		require.True(t, k.Terkunci(akhir))
		baru := k.Kembalikan(5, akhir)
		assert.Equal(t, 4, baru.Gagal)
		assert.Nil(t, baru.TerkunciSampai)
		assert.False(t, baru.Terkunci(akhir))
	})
	t.Run("tidak pernah di bawah nol", func(t *testing.T) {
		baru := KunciLogin{}.Kembalikan(5, t0)
		assert.Equal(t, 0, baru.Gagal)
	})
}

func TestTerkunci(t *testing.T) {
	assert.False(t, KunciLogin{}.Terkunci(t0), "tanpa kunci")
	sampai := t0.Add(time.Minute)
	k := KunciLogin{TerkunciSampai: &sampai}
	assert.True(t, k.Terkunci(t0))
	assert.False(t, k.Terkunci(sampai))
	assert.False(t, k.Terkunci(sampai.Add(time.Second)))
}
