package dto

// ListEWSRequest adalah filter dan paginasi daftar alert (semua opsional).
type ListEWSRequest struct {
	Status        string `form:"status" binding:"omitempty,oneof=BARU DIAKUI DIESKALASI SELESAI"`
	Severity      string `form:"severity" binding:"omitempty,oneof=INFO WASPADA SIAGA KRITIS"`
	Jenis         string `form:"jenis" binding:"omitempty,oneof=LONJAKAN_HARGA KELANGKAAN_STOK ANOMALI_CUACA OVER_SUPPLY ANOMALI_DATA"`
	KabupatenKode string `form:"kabupaten_kode" binding:"omitempty,max=4"`
	Page          int    `form:"page" binding:"omitempty,min=1"`
	Limit         int    `form:"limit" binding:"omitempty,min=1,max=1000"`
}

// BulkEWSRequest adalah aksi massal pada alert menurut id.
type BulkEWSRequest struct {
	Action string   `json:"action" binding:"required,oneof=akui eskalasi selesai"`
	IDs    []string `json:"ids" binding:"required,min=1,max=500,dive,required"`
}
