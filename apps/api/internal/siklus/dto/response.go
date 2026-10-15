package dto

import "time"

// SiklusAktifResponse is the response for the active cycle endpoint.
type SiklusAktifResponse struct {
	Siklus string `json:"siklus"`
	Sejak  string `json:"sejak"`
}

// ToSiklusAktifResponse converts a domain entity to the response DTO.
func ToSiklusAktifResponse(siklus string, mulaiPada time.Time) SiklusAktifResponse {
	return SiklusAktifResponse{
		Siklus: siklus,
		Sejak:  mulaiPada.Format("2006-01-02"),
	}
}
