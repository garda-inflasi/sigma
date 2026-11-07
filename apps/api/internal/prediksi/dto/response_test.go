package dto

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/prediksi/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToModel_IDStringDesimalDanKodeTampilan(t *testing.T) {
	kode := "MDL-CBM-KRT-v2.3.1"
	m := domain.ModelView{ID: 123, Kode: &kode, KomoditasKode: "CBM-KRT", Versi: "v2.3.1", DilatihPada: time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)}

	raw, err := json.Marshal(ToModel(m))
	require.NoError(t, err)

	assert.Contains(t, string(raw), `"id":"123"`, "ID BIGINT ditulis sebagai string desimal, bukan angka")
	assert.Contains(t, string(raw), `"kode":"MDL-CBM-KRT-v2.3.1"`)
}

func TestToModel_ModelTanpaKodeMenulisNull(t *testing.T) {
	raw, err := json.Marshal(ToModel(domain.ModelView{ID: 7}))
	require.NoError(t, err)

	assert.Contains(t, string(raw), `"id":"7"`)
	assert.Contains(t, string(raw), `"kode":null`)
}
