package response

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSuccess_TanpaLampiranTidakMengirimKunciML(t *testing.T) {
	w := performRequest(func(c *gin.Context) { Success(c, []int{1}) })
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &raw))
	assert.NotContains(t, raw, "ml", "kontrak lama tidak berubah bila tidak ada pengaya")
}

func TestSuccess_MembawaLampiranML(t *testing.T) {
	dipanggil := 0
	w := performRequest(func(c *gin.Context) {
		LampirkanML(c, func() interface{} { dipanggil++; return map[string]bool{"tersedia": true} })
		Success(c, []int{1})
	})
	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"code":"200","message":"OK","data":[1],"ml":{"tersedia":true}}`, w.Body.String())
	assert.Equal(t, 1, dipanggil)
}

func TestLampiranML_HanyaSaatSukses(t *testing.T) {
	dipanggil := 0
	w := performRequest(func(c *gin.Context) {
		LampirkanML(c, func() interface{} { dipanggil++; return "x" })
		NotFound(c, "tidak ada")
	})
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.NotContains(t, w.Body.String(), `"ml"`)
	assert.Zero(t, dipanggil, "respons galat tidak memicu bacaan ML")
}

func TestLampiranML_PanicTidakMenggagalkanRespons(t *testing.T) {
	w := performRequest(func(c *gin.Context) {
		LampirkanML(c, func() interface{} { panic("mongo meledak") })
		Success(c, map[string]string{"k": "v"})
	})
	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"code":"200","message":"OK","data":{"k":"v"}}`, w.Body.String())
}
