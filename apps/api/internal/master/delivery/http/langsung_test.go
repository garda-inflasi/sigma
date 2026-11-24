package http

import (
	"bytes"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestValidKoordinatIndonesia(t *testing.T) {
	assert.True(t, validKoordinat(5.5577, 95.3222))  // Banda Aceh
	assert.True(t, validKoordinat(4.6270, 96.8460))  // Takengon
	assert.False(t, validKoordinat(45.0, 95.3222))   // di luar Indonesia
	assert.False(t, validKoordinat(5.5577, 10.0))    // di luar Indonesia
}

func TestCreateLangsungValidasiKoordinatDanEntitas(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewLangsung(nil, nil)
	r := gin.New()
	r.POST("/langsung/:entitas", h.CreateLangsung)

	// Koordinat pasar di luar Indonesia ditolak 400 tanpa menyentuh DB.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(stdhttp.MethodPost, "/langsung/pasar", bytes.NewBufferString(`{
		"kode":"PSR-X","nama":"Pasar Uji","kabupaten_kode":"1171","lat":30.0,"lng":95.3
	}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, stdhttp.StatusBadRequest, w.Code)

	// Entitas tak dikenal ditolak 400.
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(stdhttp.MethodPost, "/langsung/tidak_ada", bytes.NewBufferString(`{}`))
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, req2)
	assert.Equal(t, stdhttp.StatusBadRequest, w2.Code)
}
