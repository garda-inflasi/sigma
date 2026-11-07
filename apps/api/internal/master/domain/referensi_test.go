package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPilihVersiPada(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	// Terbaru dulu: v3 (1 Mei), v2 (1 Mar), v1 (1 Jan).
	riwayat := []KomoditasVersiRef{
		{Versi: 3, BerlakuDari: t0.AddDate(0, 4, 0)},
		{Versi: 2, BerlakuDari: t0.AddDate(0, 2, 0)},
		{Versi: 1, BerlakuDari: t0},
	}

	cases := []struct {
		nama string
		pada time.Time
		want int
	}{
		{"jauh sesudah versi terbaru", t0.AddDate(1, 0, 0), 3},
		{"tepat saat versi terbaru berlaku", t0.AddDate(0, 4, 0), 3},
		{"sesaat sebelum versi terbaru", t0.AddDate(0, 4, 0).Add(-time.Nanosecond), 2},
		{"di tengah versi 2", t0.AddDate(0, 3, 0), 2},
		{"tepat saat versi paling awal berlaku", t0, 1},
		{"lebih tua dari seluruh riwayat jatuh ke versi paling awal", t0.AddDate(-5, 0, 0), 1},
	}
	for _, tc := range cases {
		t.Run(tc.nama, func(t *testing.T) {
			got, ok := PilihVersiPada(riwayat, tc.pada)
			assert.True(t, ok)
			assert.Equal(t, tc.want, got.Versi)
		})
	}

	t.Run("riwayat kosong", func(t *testing.T) {
		_, ok := PilihVersiPada(nil, t0)
		assert.False(t, ok)
	})

	t.Run("dua versi pada waktu sama: yang lebih baru (urutan lebih depan) menang", func(t *testing.T) {
		kembar := []KomoditasVersiRef{{Versi: 2, BerlakuDari: t0}, {Versi: 1, BerlakuDari: t0}}
		got, _ := PilihVersiPada(kembar, t0.Add(time.Hour))
		assert.Equal(t, 2, got.Versi)
	})
}
