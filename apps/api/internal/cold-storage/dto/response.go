package dto

import "time"

type FasilitasColdStorageResponse struct {
	ID            string  `json:"id"`
	Nama          string  `json:"nama"`
	KabupatenKode string  `json:"kabupaten_kode"`
	Lat           float64 `json:"lat"`
	Lng           float64 `json:"lng"`
	KapasitasTon  int     `json:"kapasitas_ton"`
	SuhuTargetC   float64 `json:"suhu_target_c"`
	Status        string  `json:"status"`
}

type StokColdStorageResponse struct {
	ID             string    `json:"id"`
	ColdStorageID  string    `json:"cold_storage_id"`
	KomoditasKode  string    `json:"komoditas_kode"`
	VolumeTon      float64   `json:"volume_ton"`
	MasukPada      time.Time `json:"masuk_pada"`
	Pemilik        string    `json:"pemilik"`
	RisikoFoodLoss float64   `json:"risiko_food_loss"`
}

type ColdStorageResponse struct {
	Fasilitas []FasilitasColdStorageResponse `json:"fasilitas"`
	Stok      []StokColdStorageResponse      `json:"stok"`
}

type BulkColdStorageResponse struct {
	Berhasil int `json:"berhasil"`
}
