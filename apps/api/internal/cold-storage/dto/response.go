package dto

import "time"

type FasilitasColdStorageResponse struct {
	ID               int64   `json:"id"`
	Nama             string  `json:"nama"`
	Lokasi           string  `json:"lokasi"`
	Kapasitas        float64 `json:"kapasitas"`
	KapasitasTerpakai float64 `json:"kapasitas_terpakai"`
	Status           string  `json:"status"`
}

type StokColdStorageResponse struct {
	ID            int64      `json:"id"`
	FasilitasID   int64      `json:"fasilitas_id"`
	KomoditasKode string     `json:"komoditas_kode"`
	Jumlah        float64    `json:"jumlah"`
	Satuan        string     `json:"satuan"`
	MasukPada     time.Time  `json:"masuk_pada"`
	KeluarPada    *time.Time `json:"keluar_pada,omitempty"`
}

type ColdStorageResponse struct {
	Fasilitas []FasilitasColdStorageResponse `json:"fasilitas"`
	Stok      []StokColdStorageResponse      `json:"stok"`
}
