package dto

import "time"

// ── Auth Responses ────────────────────────────────────────────────────

type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	User         UserResponse `json:"user"`
}

type RefreshTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type MeResponse struct {
	User UserResponse `json:"user"`
}

// ── User Responses ────────────────────────────────────────────────────

type UserResponse struct {
	ID            string     `json:"id"`
	Nama          string     `json:"nama"`
	Email         string     `json:"email"`
	Peran         string     `json:"peran"`
	Lapis         string     `json:"lapis"`
	Lembaga       string     `json:"lembaga"`
	KabupatenKode *string    `json:"kabupaten_kode,omitempty"`
	MfaAktif      bool       `json:"mfa_aktif"`
	Status        string     `json:"status"`
	TerakhirMasuk *time.Time `json:"terakhir_masuk,omitempty"`
	DibuatPada    time.Time  `json:"dibuat_pada"`
}

type ListResponse struct {
	Items interface{} `json:"items"`
	Total int64       `json:"total"`
	Page  int         `json:"page"`
	Limit int         `json:"limit"`
}

// ── Helpers ───────────────────────────────────────────────────────────

func ToUserResponse(u interface{}) UserResponse {
	// Type assertion handled by caller
	return UserResponse{}
}

// ── MFA Responses ─────────────────────────────────────────────────────

type MFASetupResponse struct {
	Secret          string   `json:"secret"`
	ProvisioningURI string   `json:"provisioning_uri"`
	BackupCodes     []string `json:"backup_codes"`
}
