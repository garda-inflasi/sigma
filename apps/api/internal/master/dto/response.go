package dto

import "github.com/garda-inflasi/api/internal/master/domain"

type MasterKomoditasRowResponse struct {
	Kode          string `json:"kode"`
	Nama          string `json:"nama"`
	Kategori      string `json:"kategori"`
	Satuan        string `json:"satuan"`
	HargaAcuan    int    `json:"harga_acuan"`
	Pilot         bool   `json:"pilot"`
	ShelfLifeHari int    `json:"shelf_life_hari"`
	Versi         int    `json:"versi"`
	Status        string `json:"status"`
}

type MasterWilayahRowResponse struct {
	Kode         string  `json:"kode"`
	Nama         string  `json:"nama"`
	Jenis        string  `json:"jenis"`
	Ibukota      string  `json:"ibukota"`
	Lat          float64 `json:"lat"`
	Lng          float64 `json:"lng"`
	ProvinsiKode string  `json:"provinsi_kode"`
}

type MasterPasarRowResponse struct {
	ID            string  `json:"id"`
	Nama          string  `json:"nama"`
	KabupatenKode string  `json:"kabupaten_kode"`
	Lat           float64 `json:"lat"`
	Lng           float64 `json:"lng"`
	RadiusMeter   int     `json:"radius_meter"`
	Aktif         bool    `json:"aktif"`
}

type BulkKomoditasResponse struct {
	Berhasil int `json:"berhasil"`
}

// ToKomoditasRows memetakan entity komoditas ke bentuk API.
func ToKomoditasRows(items []domain.Komoditas) []MasterKomoditasRowResponse {
	rows := make([]MasterKomoditasRowResponse, len(items))
	for i, k := range items {
		rows[i] = MasterKomoditasRowResponse{
			Kode: k.Kode, Nama: k.Nama, Kategori: k.Kategori, Satuan: k.Satuan, HargaAcuan: k.HargaAcuan,
			Pilot: k.Pilot, ShelfLifeHari: k.ShelfLifeHari, Versi: k.Versi, Status: k.Status,
		}
	}
	return rows
}

// ToWilayahRows memetakan entity kabupaten ke bentuk API.
func ToWilayahRows(items []domain.Kabupaten) []MasterWilayahRowResponse {
	rows := make([]MasterWilayahRowResponse, len(items))
	for i, k := range items {
		rows[i] = MasterWilayahRowResponse{
			Kode: k.Kode, Nama: k.Nama, Jenis: k.Jenis, Ibukota: k.Ibukota, Lat: k.Lat, Lng: k.Lng, ProvinsiKode: k.ProvinsiKode,
		}
	}
	return rows
}

// ToPasarRows memetakan entity pasar ke bentuk API.
func ToPasarRows(items []domain.Pasar) []MasterPasarRowResponse {
	rows := make([]MasterPasarRowResponse, len(items))
	for i, p := range items {
		rows[i] = MasterPasarRowResponse{
			ID: p.ID, Nama: p.Nama, KabupatenKode: p.KabupatenKode, Lat: p.Lat, Lng: p.Lng, RadiusMeter: p.RadiusMeter, Aktif: p.Aktif,
		}
	}
	return rows
}
