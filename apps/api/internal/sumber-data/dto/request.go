package dto

type BulkSumberDataRequest struct {
	Action string  `json:"action" binding:"required,oneof=jalankan jeda lanjutkan"`
	IDs    []int64 `json:"ids" binding:"required,min=1"`
}
