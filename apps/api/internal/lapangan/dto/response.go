package dto

import (
	"time"

	"github.com/garda-inflasi/api/internal/lapangan/usecase"
)

// TugasLapanganResponse adalah satu tugas survei. Terisi adalah jumlah komoditas
// yang sudah tercatat terhadap Target.
type TugasLapanganResponse struct {
	ID        string    `json:"id"`
	Judul     string    `json:"judul"`
	PasarID   string    `json:"pasar_id"`
	PasarNama string    `json:"pasar_nama"`
	Komoditas []string  `json:"komoditas"`
	Tenggat   time.Time `json:"tenggat"`
	Status    string    `json:"status"`
	Target    int       `json:"target"`
	Terisi    int       `json:"terisi"`
	Tipe      string    `json:"tipe"`
}

// ToTugasLapanganResponse memetakan tugas ke respons; Komoditas selalu larik (bukan null).
func ToTugasLapanganResponse(t usecase.Tugas) TugasLapanganResponse {
	komoditas := []string(t.Komoditas)
	if komoditas == nil {
		komoditas = []string{}
	}
	return TugasLapanganResponse{
		ID: t.ID, Judul: t.Judul, PasarID: t.PasarID, PasarNama: t.PasarNama, Komoditas: komoditas,
		Tenggat: t.Tenggat, Status: t.StatusTampil, Target: t.Target, Terisi: t.JumlahSelesai, Tipe: t.Tipe,
	}
}

// BulkTugasResponse melaporkan tugas yang diproses dan yang dilewati.
type BulkTugasResponse struct {
	Berhasil int `json:"berhasil"`
	Gagal    int `json:"gagal"`
}
