package dto

// ── Harga Requests ────────────────────────────────────────────────────

type ListHargaRequest struct {
	Limit int `form:"limit" binding:"min=1,max=1000"`
}

type TrenHargaRequest struct {
	Komoditas string `form:"komoditas" binding:"required"`
	Wilayah   string `form:"wilayah" binding:"required"`
}
