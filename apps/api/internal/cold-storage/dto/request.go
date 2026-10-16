package dto

// ColdStorageRequest is the query params for GET /v1/cold-storage.
// Seluruhnya opsional; nilai kosong memakai bawaan di usecase.
type ColdStorageRequest struct {
	// ColdStorageID menyaring stok pada satu fasilitas.
	ColdStorageID string `form:"cold_storage_id" binding:"omitempty,max=255"`
	// Q mencari pada nama komoditas atau pemilik.
	Q     string `form:"q" binding:"omitempty,max=100"`
	Sort  string `form:"sort" binding:"omitempty,oneof=sisa_masa_simpan komoditas fasilitas volume masuk risiko pemilik"`
	Order string `form:"order" binding:"omitempty,oneof=asc desc"`
	Page  int    `form:"page" binding:"omitempty,min=1"`
	Limit int    `form:"limit" binding:"omitempty,min=1,max=100"`
}

type BulkColdStorageRequest struct {
	Action string   `json:"action" binding:"required,oneof=rencanakan_distribusi rekomendasi_olahan"`
	IDs    []string `json:"ids" binding:"required,min=1,max=500,dive,required,max=255"`
}
