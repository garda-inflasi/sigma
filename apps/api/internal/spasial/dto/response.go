package dto

import (
	"encoding/json"
	"time"

	"github.com/garda-inflasi/api/internal/spasial/domain"
)

// MarkerResponse adalah satu wilayah di peta. Status: defisit | waspada | normal | surplus.
type MarkerResponse struct {
	Kode       string  `json:"kode"`
	Nama       string  `json:"nama"`
	Level      string  `json:"level"`
	Lat        float64 `json:"lat"`
	Lng        float64 `json:"lng"`
	Delta      float64 `json:"delta"`
	Komoditas  string  `json:"komoditas"`
	Status     string  `json:"status"`
	PunyaPasar bool    `json:"punya_pasar"`
}

// NeracaResponse adalah neraca satu komoditas di satu wilayah.
type NeracaResponse struct {
	KomoditasKode     string  `json:"komoditas_kode"`
	KomoditasNama     string  `json:"komoditas_nama"`
	KabupatenKode     string  `json:"kabupaten_kode"`
	Wilayah           string  `json:"wilayah"`
	Periode           string  `json:"periode"`
	SurplusDefisitTon float64 `json:"surplus_defisit_ton"`
	DeltaHargaPct     float64 `json:"delta_harga_pct"`
	Sumber            string  `json:"sumber"`
}

// RuteResponse adalah satu usulan rute distribusi.
type RuteResponse struct {
	ID            string    `json:"id"`
	AsalKode      string    `json:"asal_kode"`
	Asal          string    `json:"asal"`
	TujuanKode    string    `json:"tujuan_kode"`
	Tujuan        string    `json:"tujuan"`
	KomoditasKode string    `json:"komoditas_kode"`
	KomoditasNama string    `json:"komoditas_nama"`
	VolumeTon     float64   `json:"volume_ton"`
	JarakKm       int       `json:"jarak_km"`
	JamTempuh     float64   `json:"jam_tempuh"`
	Biaya         int64     `json:"biaya"`
	Status        string    `json:"status"`
	Sumber        string    `json:"sumber"`
	DibuatPada    time.Time `json:"dibuat_pada"`
}

// KandidatResponse adalah satu kandidat lokasi cold storage.
type KandidatResponse struct {
	ID            string             `json:"id"`
	KabupatenKode string             `json:"kabupaten_kode"`
	Wilayah       string             `json:"wilayah"`
	Lokasi        string             `json:"lokasi"`
	Skor          int                `json:"skor"`
	KomponenSkor  map[string]float64 `json:"komponen_skor"`
	Alasan        string             `json:"alasan"`
	Status        string             `json:"status"`
	Sumber        string             `json:"sumber"`
}

func ToMarkerResponses(items []domain.Marker) []MarkerResponse {
	out := make([]MarkerResponse, len(items))
	for i, m := range items {
		komoditas := m.Komoditas
		if komoditas == "" {
			komoditas = "—"
		}
		out[i] = MarkerResponse{
			Kode: m.Kode, Nama: m.Nama, Level: m.Level, Lat: m.Lat, Lng: m.Lng, Delta: m.Delta,
			Komoditas: komoditas, Status: domain.StatusDariDelta(m.Delta), PunyaPasar: m.PunyaPasar,
		}
	}
	return out
}

func ToNeracaResponses(items []domain.Neraca) []NeracaResponse {
	out := make([]NeracaResponse, len(items))
	for i, n := range items {
		out[i] = NeracaResponse{
			KomoditasKode: n.KomoditasKode, KomoditasNama: n.KomoditasNama, KabupatenKode: n.KabupatenKode,
			Wilayah: n.KabupatenNama, Periode: n.Periode.Format("2006-01-02"),
			SurplusDefisitTon: n.SurplusDefisitTon, DeltaHargaPct: n.DeltaHargaPct, Sumber: n.Sumber,
		}
	}
	return out
}

func ToRuteResponses(items []domain.Rute) []RuteResponse {
	out := make([]RuteResponse, len(items))
	for i, r := range items {
		out[i] = RuteResponse{
			ID: r.ID, AsalKode: r.AsalKode, Asal: r.AsalNama, TujuanKode: r.TujuanKode, Tujuan: r.TujuanNama,
			KomoditasKode: r.KomoditasKode, KomoditasNama: r.KomoditasNama, VolumeTon: r.VolumeTon,
			JarakKm: r.JarakKm, JamTempuh: r.JamTempuh, Biaya: r.Biaya, Status: r.Status, Sumber: r.Sumber,
			DibuatPada: r.DibuatPada,
		}
	}
	return out
}

// ToKandidatResponses gagal bila komponen_skor bukan JSON objek bilangan (tak mungkin
// terjadi bila data ditulis lewat seeder, tetapi basis data tidak menjaminnya).
func ToKandidatResponses(items []domain.Kandidat) ([]KandidatResponse, error) {
	out := make([]KandidatResponse, len(items))
	for i, c := range items {
		var komponen map[string]float64
		if err := json.Unmarshal([]byte(c.KomponenSkor), &komponen); err != nil {
			return nil, err
		}
		out[i] = KandidatResponse{
			ID: c.ID, KabupatenKode: c.KabupatenKode, Wilayah: c.KabupatenNama, Lokasi: c.Lokasi, Skor: c.Skor,
			KomponenSkor: komponen, Alasan: c.Alasan, Status: c.Status, Sumber: c.Sumber,
		}
	}
	return out, nil
}
