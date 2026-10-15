package dto

type UpdateTahapRequest struct {
	Tahap  string `json:"tahap" binding:"required,oneof=usulan kaji setuju laksana selesai batal"`
	Urutan int    `json:"urutan" binding:"min=0"`
}
