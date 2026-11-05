package dto

import (
	"time"

	"github.com/garda-inflasi/api/internal/kpi/domain"
)

const (
	formatPeriodeQuery = "2006-01"
	formatTanggal      = "2006-01-02"
)

// ListRequest: Periode "YYYY-MM" (bawaan: bulan berjalan); Wilayah kode kabupaten/kota
// (bawaan: seluruh provinsi).
type ListRequest struct {
	Periode string `form:"periode"`
	Wilayah string `form:"wilayah" binding:"omitempty,len=4,numeric"`
}

// ParsePeriode membaca periode query. Periode kosong menghasilkan waktu nol dengan ok true;
// ok false berarti formatnya salah.
func (r ListRequest) ParsePeriode() (t time.Time, ok bool) {
	if r.Periode == "" {
		return time.Time{}, true
	}
	t, err := time.Parse(formatPeriodeQuery, r.Periode)
	return t, err == nil
}

// WilayahPtr mengembalikan nil bila tanpa wilayah.
func (r ListRequest) WilayahPtr() *string {
	if r.Wilayah == "" {
		return nil
	}
	return &r.Wilayah
}

type TargetResponse struct {
	Nilai         float64  `json:"nilai"`
	AmbangWaspada *float64 `json:"ambang_waspada"`
	// Baseline kosong sampai ditetapkan pemangku kepentingan.
	Baseline     *float64 `json:"baseline"`
	PeriodeMulai string   `json:"periode_mulai"`
}

type NilaiResponse struct {
	Periode      string    `json:"periode"`
	Nilai        float64   `json:"nilai"`
	Versi        int       `json:"versi"`
	DihitungPada time.Time `json:"dihitung_pada"`
}

// IndikatorResponse: Nilai null berarti menunggu data (status MENUNGGU_DATA);
// Otomatis false berarti belum ada penghitung otomatis (butuh model atau data lain).
type IndikatorResponse struct {
	Kode      string          `json:"kode"`
	Kelompok  string          `json:"kelompok"`
	Nama      string          `json:"nama"`
	Deskripsi string          `json:"deskripsi"`
	Rumus     string          `json:"rumus"`
	Satuan    string          `json:"satuan"`
	ArahBaik  string          `json:"arah_baik"`
	Frekuensi string          `json:"frekuensi"`
	Otomatis  bool            `json:"otomatis"`
	Target    *TargetResponse `json:"target"`
	Nilai     *NilaiResponse  `json:"nilai"`
	Tren      []NilaiResponse `json:"tren"`
	Status    string          `json:"status"`
}

type RiwayatResponse struct {
	Kpi     IndikatorResponse `json:"kpi"`
	Riwayat []NilaiResponse   `json:"riwayat"`
}

func ToNilai(n domain.Nilai) NilaiResponse {
	return NilaiResponse{Periode: n.Periode.Format(formatTanggal), Nilai: n.Nilai, Versi: n.Versi, DihitungPada: n.DihitungPada}
}

func ToNilaiList(list []domain.Nilai) []NilaiResponse {
	res := make([]NilaiResponse, len(list))
	for i, n := range list {
		res[i] = ToNilai(n)
	}
	return res
}

func ToIndikator(i domain.Indikator) IndikatorResponse {
	res := IndikatorResponse{
		Kode: i.Definisi.Kode, Kelompok: i.Definisi.Kelompok, Nama: i.Definisi.Nama,
		Deskripsi: i.Definisi.Deskripsi, Rumus: i.Definisi.Rumus, Satuan: i.Definisi.Satuan,
		ArahBaik: i.Definisi.ArahBaik, Frekuensi: i.Definisi.Frekuensi, Otomatis: i.Otomatis,
		Tren: ToNilaiList(i.Tren), Status: i.Status,
	}
	if i.Target != nil {
		res.Target = &TargetResponse{
			Nilai: i.Target.Target, AmbangWaspada: i.Target.AmbangWaspada, Baseline: i.Target.Baseline,
			PeriodeMulai: i.Target.PeriodeMulai.Format(formatTanggal),
		}
	}
	if i.Nilai != nil {
		n := ToNilai(*i.Nilai)
		res.Nilai = &n
	}
	return res
}

func ToIndikatorList(list []domain.Indikator) []IndikatorResponse {
	res := make([]IndikatorResponse, len(list))
	for i, ind := range list {
		res[i] = ToIndikator(ind)
	}
	return res
}
