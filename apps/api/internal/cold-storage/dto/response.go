package dto

import "time"

type FasilitasColdStorageResponse struct {
	ID            string `json:"id"`
	Nama          string `json:"nama"`
	KabupatenKode string `json:"kabupaten_kode"`
	// KabupatenNama adalah nama resmi pada master wilayah, mis. "KABUPATEN ACEH BESAR".
	KabupatenNama string  `json:"kabupaten_nama"`
	Lat           float64 `json:"lat"`
	Lng           float64 `json:"lng"`
	KapasitasTon  int     `json:"kapasitas_ton"`
	// TerisiTon adalah jumlah volume seluruh lot di fasilitas ini.
	TerisiTon       float64 `json:"terisi_ton"`
	UtilisasiPersen float64 `json:"utilisasi_persen"`
	SuhuTargetC     float64 `json:"suhu_target_c"`
	// SuhuAktualC bernilai null bila fasilitas belum memiliki telemetri.
	SuhuAktualC *float64 `json:"suhu_aktual_c"`
	Status      string   `json:"status"`
}

type StokColdStorageResponse struct {
	ID              string    `json:"id"`
	ColdStorageID   string    `json:"cold_storage_id"`
	ColdStorageNama string    `json:"cold_storage_nama"`
	KomoditasKode   string    `json:"komoditas_kode"`
	KomoditasNama   string    `json:"komoditas_nama"`
	VolumeTon       float64   `json:"volume_ton"`
	MasukPada       time.Time `json:"masuk_pada"`
	KedaluwarsaPada time.Time `json:"kedaluwarsa_pada"`
	// SisaMasaSimpanHari dihitung saat permintaan; 0 berarti sudah melewati batas.
	SisaMasaSimpanHari int `json:"sisa_masa_simpan_hari"`
	// RisikoFoodLoss berupa pecahan 0–1.
	RisikoFoodLoss float64 `json:"risiko_food_loss"`
	Pemilik        string  `json:"pemilik"`
}

// PageMetaResponse menjelaskan halaman stok yang dikembalikan.
type PageMetaResponse struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
	// VolumeHasilTon menjumlahkan seluruh hasil filter (semua halaman).
	VolumeHasilTon float64 `json:"volume_hasil_ton"`
}

// RingkasanColdStorageResponse mencakup seluruh stok, tidak terpengaruh filter.
type RingkasanColdStorageResponse struct {
	TotalLot           int64   `json:"total_lot"`
	TotalVolumeTon     float64 `json:"total_volume_ton"`
	RisikoTinggiLot    int64   `json:"risiko_tinggi_lot"`
	RisikoTinggiTon    float64 `json:"risiko_tinggi_ton"`
	AmbangRisikoTinggi float64 `json:"ambang_risiko_tinggi"`
}

type ColdStorageResponse struct {
	Fasilitas []FasilitasColdStorageResponse `json:"fasilitas"`
	Stok      []StokColdStorageResponse      `json:"stok"`
	Meta      PageMetaResponse               `json:"meta"`
	Ringkasan RingkasanColdStorageResponse   `json:"ringkasan"`
}

type BulkColdStorageResponse struct {
	Berhasil  int     `json:"berhasil"`
	Gagal     int     `json:"gagal"`
	VolumeTon float64 `json:"volume_ton"`
}
