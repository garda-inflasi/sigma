package dto

import (
	"time"

	"github.com/garda-inflasi/api/internal/eksekutif/domain"
)

const (
	formatTanggal = "2006-01-02"
)

// RingkasanResponse: Demo benar bila ada angka contoh (sumber DEMO) di dalamnya;
// UI wajib menampilkan label demo. Blok tanpa data bernilai null (neraca: null,
// fiskal: null juga bila tanpa izin fiskal.lihat).
type RingkasanResponse struct {
	Demo          bool                        `json:"demo"`
	Inflasi       *InflasiResponse            `json:"inflasi"`
	Neraca        *NeracaResponse             `json:"neraca"`
	Fiskal        *FiskalResponse             `json:"fiskal"`
	EWS           map[string]int              `json:"ews"`
	Intervensi    map[string]IntervensiHitung `json:"intervensi"`
	SiklusDominan string                      `json:"siklus_dominan"`
}

type InflasiResponse struct {
	Periode     string               `json:"periode"`
	Sumber      string               `json:"sumber"`
	DiimporPada time.Time            `json:"diimpor_pada"`
	IHK         float64              `json:"ihk"`
	Yoy         float64              `json:"yoy"`
	Mtm         float64              `json:"mtm"`
	Ytd         float64              `json:"ytd"`
	Sasaran     SasaranResponse      `json:"sasaran"`
	Seri        []TitikSeriResponse  `json:"seri"`
	Penyumbang  []PenyumbangResponse `json:"penyumbang"`
}

type SasaranResponse struct {
	Tahun int     `json:"tahun"`
	Bawah float64 `json:"bawah"`
	Atas  float64 `json:"atas"`
}

type TitikSeriResponse struct {
	Periode string  `json:"periode"`
	Yoy     float64 `json:"yoy"`
}

type PenyumbangResponse struct {
	Kode  string  `json:"kode"`
	Nama  string  `json:"nama"`
	Andil float64 `json:"andil"`
}

type NeracaResponse struct {
	Periode string               `json:"periode"`
	Sumber  string               `json:"sumber"`
	Items   []NeracaItemResponse `json:"items"`
}

type NeracaItemResponse struct {
	KomoditasKode string   `json:"komoditas_kode"`
	Nama          string   `json:"nama"`
	ProduksiTon   float64  `json:"produksi_ton"`
	KonsumsiTon   float64  `json:"konsumsi_ton"`
	StokTon       *float64 `json:"stok_ton"`
}

type FiskalResponse struct {
	Sumber     string              `json:"sumber"`
	PerTanggal string              `json:"per_tanggal"`
	Pagu       int64               `json:"pagu"`
	Realisasi  int64               `json:"realisasi"`
	Skor       int                 `json:"skor"`
	Status     string              `json:"status"`
	Instrumen  []InstrumenResponse `json:"instrumen"`
}

type InstrumenResponse struct {
	Nama      string `json:"nama"`
	Pagu      int64  `json:"pagu"`
	Realisasi int64  `json:"realisasi"`
}

type IntervensiHitung struct {
	Aktif   int `json:"aktif"`
	Selesai int `json:"selesai"`
}

func ToRingkasanResponse(r *domain.Ringkasan) RingkasanResponse {
	res := RingkasanResponse{
		Demo: r.Demo, EWS: r.EWS, SiklusDominan: r.SiklusDominan,
		Intervensi: make(map[string]IntervensiHitung, len(r.Intervensi)),
	}
	for siklus, h := range r.Intervensi {
		res.Intervensi[siklus] = IntervensiHitung{Aktif: h.Aktif, Selesai: h.Selesai}
	}
	if r.Inflasi != nil {
		res.Inflasi = toInflasi(r.Inflasi)
	}
	if r.Neraca != nil {
		res.Neraca = toNeraca(r.Neraca)
	}
	if r.Fiskal != nil {
		res.Fiskal = toFiskal(r.Fiskal)
	}
	return res
}

func toInflasi(i *domain.InflasiRingkas) *InflasiResponse {
	res := &InflasiResponse{
		Periode: i.Periode.Format(formatTanggal), Sumber: i.Sumber, DiimporPada: i.DiimporPada,
		IHK: i.IHK, Yoy: i.Yoy, Mtm: i.Mtm, Ytd: i.Ytd,
		Sasaran:    SasaranResponse{Tahun: i.Sasaran.Tahun, Bawah: i.Sasaran.Bawah, Atas: i.Sasaran.Atas},
		Seri:       make([]TitikSeriResponse, len(i.Seri)),
		Penyumbang: make([]PenyumbangResponse, len(i.Penyumbang)),
	}
	for n, t := range i.Seri {
		res.Seri[n] = TitikSeriResponse{Periode: t.Periode.Format(formatTanggal), Yoy: t.Yoy}
	}
	for n, p := range i.Penyumbang {
		res.Penyumbang[n] = PenyumbangResponse{Kode: p.Kode, Nama: p.Nama, Andil: p.Andil}
	}
	return res
}

func toNeraca(n *domain.NeracaRingkas) *NeracaResponse {
	res := &NeracaResponse{Periode: n.Periode.Format(formatTanggal), Sumber: n.Sumber, Items: make([]NeracaItemResponse, len(n.Items))}
	for i, item := range n.Items {
		res.Items[i] = NeracaItemResponse{
			KomoditasKode: item.KomoditasKode, Nama: item.Nama,
			ProduksiTon: item.ProduksiTon, KonsumsiTon: item.KonsumsiTon, StokTon: item.StokTon,
		}
	}
	return res
}

func toFiskal(f *domain.FiskalRingkas) *FiskalResponse {
	res := &FiskalResponse{
		Sumber: f.Sumber, PerTanggal: f.PerTanggal.Format(formatTanggal),
		Pagu: f.Pagu, Realisasi: f.Realisasi, Skor: f.Skor, Status: f.Status,
		Instrumen: make([]InstrumenResponse, len(f.Instrumen)),
	}
	for i, ins := range f.Instrumen {
		res.Instrumen[i] = InstrumenResponse{Nama: ins.Instrumen, Pagu: ins.Pagu, Realisasi: ins.Realisasi}
	}
	return res
}
