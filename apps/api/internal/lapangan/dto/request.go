package dto

// BulkTugasRequest adalah isian POST /v1/lapangan/tugas/bulk.
type BulkTugasRequest struct {
	Aksi string   `json:"aksi" binding:"required,oneof=unduh_data_acuan tandai_selesai"`
	IDs  []string `json:"ids" binding:"required,min=1,max=500"`
}
