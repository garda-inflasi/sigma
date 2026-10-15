package dto

type BulkKomoditasRequest struct {
	Action string   `json:"action" binding:"required,oneof=ajukan_perubahan setujui arsipkan"`
	Kode   []string `json:"kode" binding:"required,min=1"`
}
