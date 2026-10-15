package dto

// ColdStorageRequest is the query params for GET /v1/cold-storage
type ColdStorageRequest struct {
	FasilitasID *int64 `form:"fasilitas_id"`
}
