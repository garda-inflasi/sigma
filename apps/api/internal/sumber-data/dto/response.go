package dto

import (
	"encoding/json"
	"time"

	ingest "github.com/garda-inflasi/api/internal/ingestion/domain"
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
	// BisaDijalankan benar bila sumber punya konektor ingestion (tombol "jalankan sekarang").
	BisaDijalankan bool `json:"bisa_dijalankan"`
}

// BulkSumberDataResponse: Gagal adalah id yang diminta tetapi tidak berubah
// (tidak ada, atau tidak memenuhi syarat aksi).
type BulkSumberDataResponse struct {
	Berhasil int `json:"berhasil"`
	Gagal    int `json:"gagal"`
}

func ToSumberDataResponse(s domain.SumberData, bisaDijalankan bool) SumberDataResponse {
	return SumberDataResponse{
		ID: s.ID, Nama: s.Nama, Penyedia: s.Penyedia, Jenis: s.Jenis, Cakupan: s.Cakupan,
		JadwalCron: s.JadwalCron, Status: s.Status, TerakhirJalan: s.TerakhirJalan,
		Keberhasilan7h: s.Keberhasilan7h, LatensiMs: s.LatensiMs, RobotsTxtPatuh: s.RobotsTxtPatuh,
		Atribusi: s.Atribusi, URLDasar: s.URLDasar, BisaDijalankan: bisaDijalankan,
	}
}

// ToSumberDataResponses memetakan katalog; bisaDijalankan menentukan tombol per sumber.
func ToSumberDataResponses(items []domain.SumberData, bisaDijalankan func(id string) bool) []SumberDataResponse {
	res := make([]SumberDataResponse, len(items))
	for i, item := range items {
		res[i] = ToSumberDataResponse(item, bisaDijalankan(item.ID))
	}
	return res
}

// EksekusiResponse adalah satu baris riwayat eksekusi ingestion. Ringkasan berisi
// rincian baris ditolak (alias tak terpetakan dan alasan tidak valid).
type EksekusiResponse struct {
	ID                 int64           `json:"id"`
	SumberID           string          `json:"sumber_id"`
	Pemicu             string          `json:"pemicu"`
	Status             string          `json:"status"`
	Mulai              time.Time       `json:"mulai"`
	Selesai            *time.Time      `json:"selesai"`
	DurasiMs           *int            `json:"durasi_ms"`
	LatensiMs          *int            `json:"latensi_ms"`
	Dokumen            int             `json:"dokumen"`
	BarisDibaca        int             `json:"baris_dibaca"`
	BarisDiambil       int             `json:"baris_diambil"`
	BarisTakTerpetakan int             `json:"baris_tak_terpetakan"`
	BarisTidakValid    int             `json:"baris_tidak_valid"`
	BarisDitolak       int             `json:"baris_ditolak"`
	Galat              *string         `json:"galat"`
	Ringkasan          json.RawMessage `json:"ringkasan"`
}

func ToEksekusiResponse(e ingest.EksekusiSumber) EksekusiResponse {
	ringkasan := json.RawMessage(e.Ringkasan)
	if len(ringkasan) == 0 {
		ringkasan = json.RawMessage("{}")
	}
	return EksekusiResponse{
		ID: e.ID, SumberID: e.SumberID, Pemicu: e.Pemicu, Status: e.Status, Mulai: e.Mulai, Selesai: e.Selesai,
		DurasiMs: e.DurasiMs, LatensiMs: e.LatensiMs, Dokumen: e.Dokumen, BarisDibaca: e.BarisDibaca,
		BarisDiambil: e.BarisDiambil, BarisTakTerpetakan: e.BarisTakTerpetakan, BarisTidakValid: e.BarisTidakValid,
		BarisDitolak: e.BarisTakTerpetakan + e.BarisTidakValid, Galat: e.Galat, Ringkasan: ringkasan,
	}
}

func ToEksekusiResponses(items []ingest.EksekusiSumber) []EksekusiResponse {
	res := make([]EksekusiResponse, len(items))
	for i, item := range items {
		res[i] = ToEksekusiResponse(item)
	}
	return res
}

// SampelResponse adalah cuplikan satu payload mentah (bronze). Ukuran adalah panjang
// payload utuh dalam karakter; Cuplikan memuat awalnya saja.
type SampelResponse struct {
	ID              int64     `json:"id"`
	EksekusiID      int64     `json:"eksekusi_id"`
	KunciPermintaan string    `json:"kunci_permintaan"`
	DiambilPada     time.Time `json:"diambil_pada"`
	Ukuran          int       `json:"ukuran"`
	Cuplikan        string    `json:"cuplikan"`
}

func ToSampelResponses(items []ingest.SampelMentah) []SampelResponse {
	res := make([]SampelResponse, len(items))
	for i, item := range items {
		res[i] = SampelResponse(item)
	}
	return res
}
