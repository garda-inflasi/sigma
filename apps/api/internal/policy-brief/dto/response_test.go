package dto

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/policy-brief/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToBriefDetail_VersiIDDitulisSebagaiStringDesimal(t *testing.T) {
	waktu := time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC)
	v := domain.DetailView{
		BriefView: domain.BriefView{ID: "3b241101-e2bb-4255-8caf-4136c566a962", Kode: "PB-2026-11", Periode: "November 2026", Status: "DRAF", SumberNarasi: "TEMPLAT"},
		Konten:    `{"bagian":[]}`,
		RiwayatVersi: []domain.VersiView{
			{ID: 12, Versi: 2, Oleh: "Penyusun", Waktu: waktu},
			{ID: 9, Versi: 1, Oleh: "Penyusun", Waktu: waktu.Add(-time.Hour)},
		},
	}

	raw, err := json.Marshal(ToBriefDetail(v))
	require.NoError(t, err)

	var got struct {
		ID           string `json:"id"`
		Kode         string `json:"kode"`
		RiwayatVersi []struct {
			ID    string `json:"id"`
			Versi int    `json:"versi"`
			Oleh  string `json:"oleh"`
			Waktu string `json:"waktu"`
		} `json:"riwayat_versi"`
	}
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, "3b241101-e2bb-4255-8caf-4136c566a962", got.ID, "ID brief UUID")
	assert.Equal(t, "PB-2026-11", got.Kode)
	require.Len(t, got.RiwayatVersi, 2)
	assert.Equal(t, "12", got.RiwayatVersi[0].ID, "versi_id BIGINT ditulis sebagai string desimal")
	assert.Equal(t, 2, got.RiwayatVersi[0].Versi)
	assert.Equal(t, "2026-11-05T09:00:00Z", got.RiwayatVersi[0].Waktu)
	assert.Contains(t, string(raw), `"id":"12"`)
}

func TestToBriefDetail_RiwayatVersiKosongBerupaLarikBukanNull(t *testing.T) {
	raw, err := json.Marshal(ToBriefDetail(domain.DetailView{Konten: `{}`}))
	require.NoError(t, err)

	assert.Contains(t, string(raw), `"riwayat_versi":[]`)
}
