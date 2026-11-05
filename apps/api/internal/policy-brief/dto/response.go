package dto

import (
	"time"

	"github.com/garda-inflasi/api/internal/policy-brief/domain"
)

// PolicyBriefResponse adalah satu brief untuk daftar dan hasil Generate.
type PolicyBriefResponse struct {
	ID                string `json:"id"`
	Periode           string `json:"periode"`
	Judul             string `json:"judul"`
	Status            string `json:"status"`
	Ringkasan         string `json:"ringkasan"`
	DisusunOleh       string `json:"disusun_oleh"`
	JumlahRekomendasi int    `json:"jumlah_rekomendasi"`
	Halaman           int    `json:"halaman"`
	DibuatPada        string `json:"dibuat_pada"`
	Diperbarui        string `json:"diperbarui"`
}

func ToBriefResponse(v domain.BriefView) PolicyBriefResponse {
	return PolicyBriefResponse{
		ID: v.ID, Periode: v.Periode, Judul: v.Judul, Status: v.Status, Ringkasan: v.Ringkasan,
		DisusunOleh: v.DisusunOleh, JumlahRekomendasi: v.JumlahRekomendasi, Halaman: v.Halaman,
		DibuatPada: v.DibuatPada.Format(time.RFC3339), Diperbarui: v.Diperbarui.Format(time.RFC3339),
	}
}

func ToBriefResponses(items []domain.BriefView) []PolicyBriefResponse {
	res := make([]PolicyBriefResponse, len(items))
	for i, item := range items {
		res[i] = ToBriefResponse(item)
	}
	return res
}

// BulkResponse melaporkan jumlah brief yang berubah dan yang tidak.
type BulkResponse struct {
	Berhasil int `json:"berhasil"`
	Gagal    int `json:"gagal"`
}
