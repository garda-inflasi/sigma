package dto

type AuditRowResponse struct {
	ID        string  `json:"id"`
	Waktu     string  `json:"waktu"`
	AktorID   *string `json:"aktor_id,omitempty"`
	Aksi      string  `json:"aksi"`
	Entitas   string  `json:"entitas"`
	EntitasID string  `json:"entitas_id"`
	Hasil     string  `json:"hasil"`
	IP        *string `json:"ip,omitempty"`
	Metadata  *string `json:"metadata,omitempty"`
}
