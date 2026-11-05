package dto

import "time"

// NeracaRequest menyaring neraca. Tanpa periode dipakai periode terbaru.
type NeracaRequest struct {
	Komoditas string    `form:"komoditas" binding:"omitempty,max=255"`
	Periode   time.Time `form:"periode" time_format:"2006-01-02"`
}
