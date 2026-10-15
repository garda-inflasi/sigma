package dto

type MasterKomoditasRowResponse struct {
	Kode     string `json:"kode"`
	Nama     string `json:"nama"`
	Kategori string `json:"kategori"`
	Satuan   string `json:"satuan"`
	Pilot    bool   `json:"pilot"`
	Status   string `json:"status"`
}
