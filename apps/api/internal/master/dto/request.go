package dto

// ListRequest adalah pencarian dan paginasi opsional untuk daftar master.
// Tanpa limit, seluruh baris dikembalikan; jumlah seluruh hasil ada di header X-Total-Count.
type ListRequest struct {
	Q     string `form:"q"`
	Page  int    `form:"page" binding:"omitempty,min=1"`
	Limit int    `form:"limit" binding:"omitempty,min=1,max=500"`
}

type BulkKomoditasRequest struct {
	Action string   `json:"action" binding:"required,oneof=ajukan_perubahan setujui arsipkan"`
	Kode   []string `json:"kode" binding:"required,min=1"`
}
