package http

import (
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHitungJarakDuaTitik(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewTitik(nil)
	r := gin.New()
	r.GET("/jarak", h.HitungJarak)

	// Takengon (4.627, 96.846) ke Banda Aceh (5.5577, 95.3222) ~ 197 km garis lurus
	w := httptest.NewRecorder()
	req := httptest.NewRequest(stdhttp.MethodGet, "/jarak?asal_lat=4.627&asal_lng=96.846&tujuan_lat=5.5577&tujuan_lng=95.3222&faktor_jalan=1.3", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, stdhttp.StatusOK, w.Code)

	var body struct {
		Data JarakResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.NotNil(t, body.Data.JarakLurusKm)
	require.NotNil(t, body.Data.JarakJalanKm)
	assert.Greater(t, *body.Data.JarakLurusKm, 180.0)
	assert.Less(t, *body.Data.JarakLurusKm, 215.0)
	assert.Greater(t, *body.Data.JarakJalanKm, *body.Data.JarakLurusKm)
	assert.Equal(t, "TINGGI", body.Data.RisikoFoodLoss)

	// Koordinat invalid -> 400
	wErr := httptest.NewRecorder()
	r.ServeHTTP(wErr, httptest.NewRequest(stdhttp.MethodGet, "/jarak?asal_lat=abc&asal_lng=95", nil))
	assert.Equal(t, stdhttp.StatusBadRequest, wErr.Code)
}
