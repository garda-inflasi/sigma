package dto

type BulkTugasRequest struct {
	Action string  `json:"action" binding:"required,oneof=unduh_data_acuan tandai_selesai"`
	IDs    []int64 `json:"ids" binding:"required,min=1"`
}
