package dto

// GenerateRequest meminta draf brief untuk satu periode bulanan berformat YYYY-MM.
type GenerateRequest struct {
	Periode string `json:"periode" binding:"required"`
}
