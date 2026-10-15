package dto

// ── Harga Requests ────────────────────────────────────────────────────

type ListHargaRequest struct {
	Limit  int    `form:"limit" binding:"min=1,max=1000"`
	Status string `form:"status"`
}

type TrenHargaRequest struct {
	Komoditas string `form:"komoditas" binding:"required"`
	Wilayah   string `form:"wilayah" binding:"required"`
}

type CreateHargaRequest struct {
	KomoditasKode string  `json:"komoditas_kode" binding:"required"`
	PasarID       string  `json:"pasar_id" binding:"required"`
	Harga         float64 `json:"harga" binding:"required,min=0"`
	Satuan        string  `json:"satuan" binding:"required"`
	Periode       string  `json:"periode" binding:"required"`
	Sumber        string  `json:"sumber"`
	PetugasID     string  `json:"petugas_id"`
}

type BulkHargaRequest struct {
	Action string   `json:"action" binding:"required,oneof=validasi publikasi tolak"`
	IDs    []string `json:"ids" binding:"required,min=1"`
	Alasan string   `json:"alasan"`
}

type HargaDetailRequest struct {
	IncludeAnomali bool `form:"include_anomali"`
}
