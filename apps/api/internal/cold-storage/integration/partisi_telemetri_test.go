package integration

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPartisiTelemetriSuhuDanBatchIngestion(t *testing.T) {
	if testing.Short() {
		t.Skip("tes integrasi dilewati (-short, tanpa Docker)")
	}
	s := newSuite(t, pg.NewDatabase(t, "cs_partisi_telemetri"))

	now := time.Now().UTC()
	expectedPartisi := fmt.Sprintf("telemetri_suhu_y%04dm%02d", now.Year(), int(now.Month()))

	// 1. Pastikan partisi bulan berjalan & bulan berikutnya tersedia dari migrasi 000127.
	var count int64
	err := s.db.Raw(
		`SELECT count(*) FROM pg_inherits WHERE inhparent = 'telemetri_suhu'::regclass`,
	).Scan(&count).Error
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, int64(3), "minimal partisi bulan berjalan, bulan depan, dan default")

	// 2. Fungsi pemeliharaan partisi harus idempoten.
	var partName string
	next2 := now.AddDate(0, 2, 0)
	require.NoError(t, s.db.Raw(`SELECT pastikan_partisi_telemetri_suhu(?::date)`, next2.Format("2006-01-02")).Scan(&partName).Error)
	assert.Equal(t, fmt.Sprintf("telemetri_suhu_y%04dm%02d", next2.Year(), int(next2.Month())), partName)

	// 3. Catat risiko awal salah satu lot aktif di CS-01 sebelum anomali suhu.
	var risikoSebelum float64
	require.NoError(t, s.db.Raw(`
		SELECT s.risiko_food_loss
		  FROM stok_cold_storage s
		  JOIN cold_storage c ON c.id = s.cold_storage_id
		 WHERE c.kode = 'CS-01' AND s.status <> 'HABIS'
		 ORDER BY s.id LIMIT 1`).Scan(&risikoSebelum).Error)

	// 4. Kirim batch telemetri via POST /v1/cold-storage/telemetri/batch dengan anomali suhu tinggi (15.0°C).
	waktuBaca := now.Truncate(time.Second)
	code, body := post(t, s, "/v1/cold-storage/telemetri/batch", userKelola, map[string]any{
		"gateway_id": "gw-aceh-01",
		"topic":      "mqtt/coldstorage/CS-01/suhu",
		"pembacaan": []map[string]any{
			{
				"fasilitas_id": "CS-01",
				"waktu":        waktuBaca.Format(time.RFC3339),
				"suhu_c":       15.0,
				"sumber":       "SENSOR",
			},
		},
	})
	require.Equal(t, http.StatusOK, code, "body=%v", body)
	d := dataMap(t, body)
	assert.Equal(t, float64(1), d["diterima"])
	assert.Equal(t, "gw-aceh-01", d["gateway_id"])
	assert.GreaterOrEqual(t, d["lot_risiko_diperbarui"].(float64), float64(1))

	// 5. Verifikasi baris masuk ke partisi bulan yang benar melalui tableoid::regclass.
	var targetPartisi string
	err = s.db.Raw(`
		SELECT t.tableoid::regclass::text
		  FROM telemetri_suhu t
		  JOIN cold_storage c ON c.id = t.fasilitas_id
		 WHERE c.kode = 'CS-01' AND t.waktu = ?`,
		waktuBaca,
	).Scan(&targetPartisi).Error
	require.NoError(t, err)
	assert.Equal(t, expectedPartisi, targetPartisi, "baris telemetri harus masuk ke partisi bulan berjalan")

	// 6. Verifikasi risiko_food_loss lot aktif di CS-01 meningkat.
	var risikoSesudah float64
	require.NoError(t, s.db.Raw(`
		SELECT s.risiko_food_loss
		  FROM stok_cold_storage s
		  JOIN cold_storage c ON c.id = s.cold_storage_id
		 WHERE c.kode = 'CS-01' AND s.status <> 'HABIS'
		 ORDER BY s.id LIMIT 1`).Scan(&risikoSesudah).Error)
	assert.Greater(t, risikoSesudah, risikoSebelum, "risiko_food_loss harus meningkat saat anomali suhu")
}
