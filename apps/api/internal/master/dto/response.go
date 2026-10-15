package dto

type MasterKomoditasRowResponse struct {
	Kode     string `json:"kode"`
	Nama     string `json:"nama"`
	Kategori string `json:"kategori"`
	Satuan   string `json:"satuan"`
	Pilot    bool   `json:"pilot"`
	Status   string `json:"status"`
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
