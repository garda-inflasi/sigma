package dto

type BulkSumberDataRequest struct {
	Action string   `json:"action" binding:"required,oneof=jeda lanjutkan"`
	IDs    []string `json:"ids" binding:"required,min=1,max=100,dive,required"`
}
