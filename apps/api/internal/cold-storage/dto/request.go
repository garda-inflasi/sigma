package dto

// ColdStorageRequest is the query params for GET /v1/cold-storage
type ColdStorageRequest struct {
	ColdStorageID *string `form:"cold_storage_id"`
}

type BulkColdStorageRequest struct {
	Action string   `json:"action" binding:"required,oneof=rencanakan_distribusi rekomendasi_olahan"`
	IDs    []string `json:"ids" binding:"required,min=1"`
}
