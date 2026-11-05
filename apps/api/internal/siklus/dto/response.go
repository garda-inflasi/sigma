package dto

import (
	"strings"
	"time"
)

var wib = time.FixedZone("WIB", 7*60*60)

// SiklusAktifResponse is the response for the active cycle endpoint.
// Siklus berhuruf kecil (normal, tekanan, surplus) dan Sejak berformat YYYY-MM-DD waktu Aceh.
type SiklusAktifResponse struct {
	Siklus string `json:"siklus"`
	Sejak  string `json:"sejak"`
}

// ToSiklusAktifResponse converts a domain entity to the response DTO.
func ToSiklusAktifResponse(siklus string, mulaiPada time.Time) SiklusAktifResponse {
	return SiklusAktifResponse{
		Siklus: strings.ToLower(siklus),
		Sejak:  mulaiPada.In(wib).Format("2006-01-02"),
	}
}
