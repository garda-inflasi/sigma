package dto

import "github.com/garda-inflasi/api/internal/intervensi/domain"

type KartuIntervensiResponse struct {
	ID            int64  `json:"id"`
	Judul         string `json:"judul"`
	Deskripsi     string `json:"deskripsi"`
	Tahap         string `json:"tahap"`
	Urutan        int    `json:"urutan"`
	Prioritas     string `json:"prioritas"`
	PIC           string `json:"pic"`
	KomoditasKode string `json:"komoditas_kode"`
	WilayahKode   string `json:"wilayah_kode"`
	DibuatPada    string `json:"dibuat_pada"`
	DiupdatePada  string `json:"diupdate_pada"`
}

func ToKartuIntervensiResponse(k domain.KartuIntervensi) KartuIntervensiResponse {
	return KartuIntervensiResponse{
		ID:            k.ID,
		Judul:         k.Judul,
		Deskripsi:     k.Deskripsi,
		Tahap:         string(k.Tahap),
		Urutan:        k.Urutan,
		Prioritas:     k.Prioritas,
		PIC:           k.PIC,
		KomoditasKode: k.KomoditasKode,
		WilayahKode:   k.WilayahKode,
		DibuatPada:    k.DibuatPada.Format("2006-01-02T15:04:05Z07:00"),
		DiupdatePada:  k.DiupdatePada.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func ToKartuIntervensiListResponse(kartu []domain.KartuIntervensi) []KartuIntervensiResponse {
	items := make([]KartuIntervensiResponse, len(kartu))
	for i, k := range kartu {
		items[i] = ToKartuIntervensiResponse(k)
	}
	return items
}
