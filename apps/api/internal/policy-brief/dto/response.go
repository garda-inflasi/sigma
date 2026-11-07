package dto

import (
	"encoding/json"
	"time"

	"github.com/garda-inflasi/api/internal/policy-brief/domain"
	"github.com/garda-inflasi/api/pkg/idx"
)

// PolicyBriefResponse adalah satu brief untuk daftar dan hasil Generate. ID adalah UUID; Kode adalah kode
// tampilan PB-YYYY-MM yang juga diterima sebagai alias ID di rute /v1/policy-brief/:id.
type PolicyBriefResponse struct {
	ID                string `json:"id"`
	Kode              string `json:"kode"`
	Periode           string `json:"periode"`
	Judul             string `json:"judul"`
	Status            string `json:"status"`
	Ringkasan         string `json:"ringkasan"`
	DisusunOleh       string `json:"disusun_oleh"`
	JumlahRekomendasi int    `json:"jumlah_rekomendasi"`
	Halaman           int    `json:"halaman"`
	// SumberNarasi adalah asal narasi: LLM, TEMPLAT, atau DEMO. LabelDraf adalah label yang
	// wajib ditampilkan bersama isi brief (mis. "Draf AI — wajib ditelaah manusia").
	SumberNarasi string `json:"sumber_narasi"`
	LabelDraf    string `json:"label_draf"`
	DibuatPada   string `json:"dibuat_pada"`
	Diperbarui   string `json:"diperbarui"`
}

func ToBriefResponse(v domain.BriefView) PolicyBriefResponse {
	return PolicyBriefResponse{
		ID: v.ID, Kode: v.Kode, Periode: v.Periode, Judul: v.Judul, Status: v.Status, Ringkasan: v.Ringkasan,
		DisusunOleh: v.DisusunOleh, JumlahRekomendasi: v.JumlahRekomendasi, Halaman: v.Halaman,
		SumberNarasi: v.SumberNarasi, LabelDraf: domain.LabelPenyusun(v.SumberNarasi),
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

// VersiResponse adalah satu versi isi brief. ID adalah versi_id (BIGINT, JSON string desimal) yang dirujuk
// telaah dan revisi; Versi adalah nomor urut versi pada brief itu.
type VersiResponse struct {
	ID    idx.ID `json:"id"`
	Versi int    `json:"versi"`
	Oleh  string `json:"oleh"`
	Waktu string `json:"waktu"`
}

// PolicyBriefDetailResponse adalah satu brief lengkap: isi per bagian beserta lampiran sitasi
// sumber (Konten), jejak model dan prompt bila LLM dipakai, catatan galat penyusunan, dan riwayat versi isi
// (terbaru dulu; kosong selama draf pertama belum selesai).
type PolicyBriefDetailResponse struct {
	PolicyBriefResponse
	Konten       json.RawMessage `json:"konten"`
	ModelLLM     *string         `json:"model_llm"`
	PromptVersi  *string         `json:"prompt_versi"`
	PromptHash   *string         `json:"prompt_hash"`
	Galat        *string         `json:"galat"`
	RiwayatVersi []VersiResponse `json:"riwayat_versi"`
}

func ToBriefDetail(v domain.DetailView) PolicyBriefDetailResponse {
	versi := make([]VersiResponse, len(v.RiwayatVersi))
	for i, x := range v.RiwayatVersi {
		versi[i] = VersiResponse{ID: x.ID, Versi: x.Versi, Oleh: x.Oleh, Waktu: x.Waktu.Format(time.RFC3339)}
	}
	return PolicyBriefDetailResponse{
		PolicyBriefResponse: ToBriefResponse(v.BriefView),
		Konten:              json.RawMessage(v.Konten),
		ModelLLM:            v.ModelLLM, PromptVersi: v.PromptVersi, PromptHash: v.PromptHash, Galat: v.Galat,
		RiwayatVersi: versi,
	}
}
