package smoketest

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	// OPS_MAHASISWA membawa tugas.lihat_sendiri dan harga.input; STRATEGIS_BI tidak punya izin keduanya.
	const berhak, takBerhak = "mahasiswa.1@gardainflasi.id", "strategis.bi@gardainflasi.id"

	registerSmoke(
		smokeCase{Name: "lapangan: tugas saya", Method: http.MethodGet, Path: "/v1/lapangan/tugas/saya", Email: berhak, MinItems: map[string]int{"": 5}, ForbiddenEmail: takBerhak},
		smokeCase{Name: "lapangan: tugas saya (koordinator)", Method: http.MethodGet, Path: "/v1/lapangan/tugas/saya", Email: "koordinator.lapangan@gardainflasi.id", MinItems: map[string]int{"": 5}, ForbiddenEmail: takBerhak},
		smokeCase{Name: "lapangan: data acuan offline", Method: http.MethodGet, Path: "/v1/lapangan/acuan", Email: berhak, MinItems: map[string]int{"komoditas": 10, "pasar": 20, "harga_acuan": 20}, ForbiddenEmail: takBerhak},
	)
}

// Kiriman harga dari PWA: entri baru 201, kirim ulang dengan kunci sama 200 dan id sama.
func TestSmoke_KirimHargaLapanganIsIdempotent(t *testing.T) {
	s := shared(t)
	token := s.login(t, "mahasiswa.1@gardainflasi.id")
	body := map[string]any{
		"komoditas_kode": "BWM", "pasar_id": "PSR-1171-01", "tipe": "KONSUMEN", "harga": 41000,
		"dicatat_pada": time.Now().AddDate(-1, 0, 0).Format(time.RFC3339),
	}
	kirim := func() (int, string) {
		w := s.doWith(t, http.MethodPost, "/v1/lapangan/harga", token, map[string]string{"Idempotency-Key": "smoke-kirim-1"}, body)
		var data struct {
			ID string `json:"id"`
		}
		require.NoError(t, json.Unmarshal(readEnvelope(t, w).Data, &data))
		return w.Code, data.ID
	}

	code, id := kirim()
	assert.Equal(t, http.StatusCreated, code)
	code, again := kirim()
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, id, again)
}
