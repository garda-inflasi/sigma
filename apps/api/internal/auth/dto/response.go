package dto

import (
	"time"

	"github.com/garda-inflasi/api/internal/auth/domain"
)

// ── Auth Responses ────────────────────────────────────────────────────

type LoginResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
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

type BulkUserResponse struct {
	Berhasil int      `json:"berhasil"`
	Gagal    int      `json:"gagal"`
	Errors   []string `json:"errors,omitempty"`
}

// ── RBAC Responses ────────────────────────────────────────────────────

type RoleResponse struct {
	ID          string   `json:"id"`
	Permissions []string `json:"permissions"`
}

type PermissionResponse struct {
	ID string `json:"id"`
}

// ── Helpers ───────────────────────────────────────────────────────────

// ToUserResponse maps a domain user onto its API representation.
func ToUserResponse(u *domain.Pengguna) UserResponse {
	if u == nil {
		return UserResponse{}
	}
	return UserResponse{
		ID:            u.ID,
		Nama:          u.Nama,
		Email:         u.Email,
		Peran:         u.Peran,
		Lapis:         u.Lapis,
		Lembaga:       u.Lembaga,
		KabupatenKode: u.KabupatenKode,
		MfaAktif:      u.MfaAktif,
		Status:        u.Status,
		TerakhirMasuk: u.TerakhirMasuk,
		DibuatPada:    u.DibuatPada,
	}
}

// ToUserResponses maps a slice of domain users onto API representations.
func ToUserResponses(users []domain.Pengguna) []UserResponse {
	items := make([]UserResponse, len(users))
	for i := range users {
		items[i] = ToUserResponse(&users[i])
	}
	return items
}

// ── MFA Responses ─────────────────────────────────────────────────────

type MFASetupResponse struct {
	Secret          string   `json:"secret"`
	ProvisioningURI string   `json:"provisioning_uri"`
	BackupCodes     []string `json:"backup_codes"`
}
