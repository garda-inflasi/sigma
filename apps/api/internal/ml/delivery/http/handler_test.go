package http

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseWaktu(t *testing.T) {
	d, err := parseWaktu("2026-11-24", false)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 11, 23, 17, 0, 0, 0, time.UTC), d.UTC(), "tanggal WIB")
	s, err := parseWaktu("2026-11-24", true)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 11, 24, 17, 0, 0, 0, time.UTC), s.UTC(), "sampai eksklusif hari berikutnya")
	r, err := parseWaktu("2026-11-24T09:00:00Z", true)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 11, 24, 9, 0, 0, 0, time.UTC), r.UTC(), "RFC3339 apa adanya")
	n, err := parseWaktu(" ", false)
	assert.NoError(t, err)
	assert.Nil(t, n)
	_, err = parseWaktu("24/11/2026", false)
	assert.Error(t, err)
}

func TestParseStatus(t *testing.T) {
	st, err := parseStatus("final, gagal")
	require.NoError(t, err)
	assert.Equal(t, []string{"BERHASIL", "SEBAGIAN", "GAGAL"}, st)
	st, err = parseStatus("")
	assert.NoError(t, err)
	assert.Nil(t, st)
	_, err = parseStatus("SELESAI")
	assert.Error(t, err)
}

func TestBlokUntuk(t *testing.T) {
	assert.Equal(t, []blok{blokForecast}, BlokUntuk("/v1/prediksi/:kode"))
	assert.Equal(t, []blok{blokForecastRingkas, blokEwsRingkas}, BlokUntuk("/v1/dashboard/operasional/heatmap"))
	assert.Nil(t, BlokUntuk("/v1/prediksi/:kode/export"), "ekspor CSV tidak diperkaya")
	assert.Nil(t, BlokUntuk("/v1/harga"))
	assert.Nil(t, BlokUntuk("/v1/ml/cycles"))
}
