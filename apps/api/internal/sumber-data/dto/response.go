package dto

import (
	"time"

	"github.com/garda-inflasi/api/internal/sumber-data/domain"
)

type SumberDataResponse struct {
	ID             string     `json:"id"`
	Nama           string     `json:"nama"`
	Penyedia       string     `json:"penyedia"`
	Jenis          string     `json:"jenis"`
	Cakupan        string     `json:"cakupan"`
	JadwalCron     string     `json:"jadwal_cron"`
	Status         string     `json:"status"`
	TerakhirJalan  *time.Time `json:"terakhir_jalan"`
	Keberhasilan7h *float64   `json:"keberhasilan_7h"`
	LatensiMs      int        `json:"latensi_ms"`
	RobotsTxtPatuh bool       `json:"robots_txt_patuh"`
	Atribusi       string     `json:"atribusi"`
	URLDasar       *string    `json:"url_dasar"`
}

// BulkSumberDataResponse: Gagal adalah id yang diminta tetapi tidak berubah
// (tidak ada, atau tidak memenuhi syarat aksi).
type BulkSumberDataResponse struct {
	Berhasil int `json:"berhasil"`
	Gagal    int `json:"gagal"`
}

func ToSumberDataResponse(s domain.SumberData) SumberDataResponse {
	return SumberDataResponse{
		ID: s.ID, Nama: s.Nama, Penyedia: s.Penyedia, Jenis: s.Jenis, Cakupan: s.Cakupan,
		JadwalCron: s.JadwalCron, Status: s.Status, TerakhirJalan: s.TerakhirJalan,
		Keberhasilan7h: s.Keberhasilan7h, LatensiMs: s.LatensiMs, RobotsTxtPatuh: s.RobotsTxtPatuh,
		Atribusi: s.Atribusi, URLDasar: s.URLDasar,
	}
}

func ToSumberDataResponses(items []domain.SumberData) []SumberDataResponse {
	res := make([]SumberDataResponse, len(items))
	for i, item := range items {
		res[i] = ToSumberDataResponse(item)
	}
	return res
}
