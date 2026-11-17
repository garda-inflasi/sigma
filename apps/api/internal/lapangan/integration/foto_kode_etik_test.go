package integration

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFotoEntriKodeEtikDanKinerja(t *testing.T) {
	if testing.Short() {
		t.Skip("tes integrasi dilewati (-short, tanpa Docker)")
	}
	t.Setenv("LAPANGAN_UPLOAD_DIR", t.TempDir())
	s := sharedSuite(t)
	tok1 := s.login(t, emailMahasiswa1)
	tok2 := s.login(t, emailMahasiswa2)

	// 1. Kode etik aktif dari seed reference/15_kode_etik.sql
	wEtik := s.call(t, http.MethodGet, "/v1/lapangan/kode-etik", emailMahasiswa1, nil)
	require.Equal(t, http.StatusOK, wEtik.Code)
	var etikResp struct {
		Versi       string `json:"versi"`
		SudahSetuju bool   `json:"sudah_setuju"`
	}
	etikResp = data[struct {
		Versi       string `json:"versi"`
		SudahSetuju bool   `json:"sudah_setuju"`
	}](t, wEtik)
	assert.Equal(t, "v1.0", etikResp.Versi)
	assert.False(t, etikResp.SudahSetuju)

	// Setujui kode etik (idempoten bila dipanggil ulang)
	for i := 0; i < 2; i++ {
		wSetuju := s.call(t, http.MethodPost, "/v1/lapangan/kode-etik/setuju", emailMahasiswa1, map[string]string{"versi": "v1.0"})
		assert.Equal(t, http.StatusOK, wSetuju.Code)
	}

	wEtikAfter := s.call(t, http.MethodGet, "/v1/lapangan/kode-etik", emailMahasiswa1, nil)
	require.Equal(t, http.StatusOK, wEtikAfter.Code)
	etikResp = data[struct {
		Versi       string `json:"versi"`
		SudahSetuju bool   `json:"sudah_setuju"`
	}](t, wEtikAfter)
	assert.True(t, etikResp.SudahSetuju)

	// 2. Buat entri harga lapangan sebagai mahasiswa_1
	wHarga := s.callWith(t, http.MethodPost, "/v1/lapangan/harga", emailMahasiswa1,
		map[string]string{"Idempotency-Key": "foto-int-key-001"},
		map[string]any{
			"komoditas_kode": "BRS-MED",
			"pasar_id":       "PSR-BNA-01",
			"tipe":           "KONSUMEN",
			"harga":          14500,
			"satuan":         "kg",
			"kualitas":       "MEDIUM",
		})
	require.Equal(t, http.StatusCreated, wHarga.Code, wHarga.Body.String())

	var hargaResp struct {
		ID string `json:"id"`
	}
	hargaResp = data[struct {
		ID string `json:"id"`
	}](t, wHarga)
	entriID := hargaResp.ID
	require.NotEmpty(t, entriID)

	// 3. Unggah foto JPEG oleh mahasiswa_1 -> 201
	jpegBytes := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01, 0x01, 0x01, 0x00, 0x48, 0x00, 0x48, 0x00, 0x00, 0xFF, 0xD9}
	upload := func(tok, id string, content []byte) *httptest.ResponseRecorder {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		fw, err := mw.CreateFormFile("foto", "bukti.jpg")
		require.NoError(t, err)
		_, err = fw.Write(content)
		require.NoError(t, err)
		require.NoError(t, mw.Close())

		req := httptest.NewRequest(http.MethodPost, "/v1/lapangan/harga/"+id+"/foto", &buf)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+tok)
		w := httptest.NewRecorder()
		s.router.ServeHTTP(w, req)
		return w
	}

	wUp := upload(tok1, entriID, jpegBytes)
	assert.Equal(t, http.StatusCreated, wUp.Code, wUp.Body.String())

	var upBody map[string]any
	require.NoError(t, json.Unmarshal(wUp.Body.Bytes(), &upBody))
	assert.NotNil(t, upBody["data"])

	// Mahasiswa lain tidak boleh mengunggah foto ke entri orang lain -> 404
	wOther := upload(tok2, entriID, jpegBytes)
	assert.Equal(t, http.StatusNotFound, wOther.Code)

	// ID salah bentuk -> 404
	wBadID := upload(tok1, "not-a-number", jpegBytes)
	assert.Equal(t, http.StatusNotFound, wBadID.Code)

	// Tipe bukan gambar -> 400
	wBadMIME := upload(tok1, entriID, []byte("text/plain fake image"))
	assert.Equal(t, http.StatusBadRequest, wBadMIME.Code)

	// Pastikan foto_url di DB terisi
	var fotoURL string
	require.NoError(t, s.db.Raw("SELECT foto_url FROM entri_harga WHERE id = ?", entriID).Scan(&fotoURL).Error)
	assert.Equal(t, "/v1/lapangan/harga/"+entriID+"/foto", fotoURL)

	// 4. Kinerja petugas dari view kinerja_petugas
	wKinerja := s.call(t, http.MethodGet, "/v1/lapangan/kinerja", emailMahasiswa1, nil)
	assert.Equal(t, http.StatusOK, wKinerja.Code)
}
