package dto

type GenerateRequest struct {
	Periode string `json:"periode" binding:"required"`
}
