package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garda-inflasi/api/internal/ml/dto"
	"github.com/garda-inflasi/api/internal/ml/usecase"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaringDemoData(t *testing.T) {
	t.Run("menghapus baris slice bertanda DEMO dan mempertahankan baris resmi", func(t *testing.T) {
		rows := []map[string]any{
			{"kode": "EWS-1", "sumber": "DEMO", "judul": "Demo"},
			{"kode": "EWS-2", "sumber": "RULE_ENGINE", "judul": "Nyata"},
		}
		got := SaringDemoData(rows).([]any)
		require.Len(t, got, 1)
		assert.Equal(t, "EWS-2", got[0].(map[string]any)["kode"])
	})

	t.Run("membuang sub-blok inflasi/neraca DEMO pada ringkasan eksekutif dan mempertahankan non-demo", func(t *testing.T) {
		ringkasan := map[string]any{
			"demo":           true,
			"inflasi":        map[string]any{"sumber": "DEMO", "yoy": 3.5},
			"neraca":         map[string]any{"sumber": "BAPANAS", "periode": "2026-11-01"},
			"fiskal":         map[string]any{"sumber": "DEMO", "pagu": 100},
			"siklus_dominan": "normal",
		}
		got := SaringDemoData(ringkasan).(map[string]any)
		assert.Equal(t, false, got["demo"])
		assert.Nil(t, got["inflasi"])
		assert.Nil(t, got["fiskal"])
		assert.NotNil(t, got["neraca"])
	})

	t.Run("menolkan hitungan PipelineResponse demo", func(t *testing.T) {
		pipe := map[string]any{
			"bronze": 120,
			"silver": 80,
			"gold":   40,
			"demo":   true,
			"mutu":   map[string]any{"kelengkapan": 98.5, "akurasi": 95.0},
		}
		got := SaringDemoData(pipe).(map[string]any)
		assert.Equal(t, false, got["demo"])
		assert.Equal(t, 0, got["bronze"])
		assert.Nil(t, got["mutu"].(map[string]any)["kelengkapan"])
	})
}

func TestPengayaDataEnv(t *testing.T) {
	gin.SetMode(gin.TestMode)
	p := NewPengaya(usecase.New(nil, usecase.AlasanTidakDikonfigurasi, nil), nil)

	t.Run("mode staging menandai MODE_STAGING dan POSTGRES tanpa error", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/v1/dashboard/eksekutif", nil)
		c.Request.Header.Set("X-Garda-Data-Env", "staging")

		l := p.Susun(c, []blok{blokInflasi})
		assert.False(t, l.Tersedia)
		assert.Equal(t, "MODE_STAGING", l.Alasan)
		assert.Equal(t, dto.DataUtamaPostgres, l.DataUtama)
	})
}
