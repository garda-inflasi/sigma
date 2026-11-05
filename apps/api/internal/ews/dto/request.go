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

// UpdateAturanRequest mengubah ambang suatu aturan. Versi adalah versi yang dibaca
// klien; permintaan ditolak (409) bila aturan sudah berubah sejak itu.
type UpdateAturanRequest struct {
	Versi         int      `json:"versi" binding:"required,min=1"`
	AmbangWaspada float64  `json:"ambang_waspada" binding:"required,gt=0"`
	AmbangSiaga   float64  `json:"ambang_siaga" binding:"required,gt=0"`
	AmbangKritis  float64  `json:"ambang_kritis" binding:"required,gt=0"`
	JendelaHari   int      `json:"jendela_hari" binding:"required,min=1,max=90"`
	CooldownJam   int      `json:"cooldown_jam" binding:"min=0,max=720"`
	Kanal         []string `json:"kanal" binding:"required,min=1,max=4,dive,oneof=DASHBOARD PUSH EMAIL WHATSAPP"`
	Aktif         *bool    `json:"aktif" binding:"required"`
	Keterangan    string   `json:"keterangan" binding:"max=500"`
}
