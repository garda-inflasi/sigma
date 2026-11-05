package dto

import (
	"time"

	"github.com/garda-inflasi/api/internal/auth/domain"
)

// ── Auth Responses ────────────────────────────────────────────────────

// LoginResponse adalah hasil login. Bila mfa_required, belum ada sesi: tidak ada token sesi maupun
// profil, dan klien menukar mfa_token dengan kode OTP di POST /v1/auth/mfa/validate.
type LoginResponse struct {
	AccessToken  string        `json:"access_token,omitempty"`
	RefreshToken string        `json:"refresh_token,omitempty"`
	User         *UserResponse `json:"user,omitempty"`
	MFARequired  bool          `json:"mfa_required,omitempty"`
	MFAToken     string        `json:"mfa_token,omitempty"`
	// MFASetupDiwajibkan true berarti sesi ini hanya membuka endpoint setup MFA (403 MFA_SETUP_REQUIRED selain itu).
	MFASetupDiwajibkan bool `json:"mfa_setup_diwajibkan"`
}

type RefreshTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// ChangePasswordResponse memuat sesi baru: ganti kata sandi mencabut semua sesi
// lama, dan token lama masih membawa klaim wajib ganti kata sandi.
type ChangePasswordResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// MeUserResponse adalah profil sendiri beserta keadaan MFA.
type MeUserResponse struct {
	UserResponse
	// MfaSetupDiwajibkan true bila akun wajib MFA tetapi belum mengaktifkannya.
	MfaSetupDiwajibkan bool `json:"mfa_setup_diwajibkan"`
	// MfaWajib true bila MFA tidak boleh dinonaktifkan oleh pemilik akun.
	MfaWajib bool `json:"mfa_wajib"`
	// KodeCadanganTersisa hanya ada untuk akun dengan MFA aktif.
	KodeCadanganTersisa *int `json:"kode_cadangan_tersisa,omitempty"`
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
	// WajibGantiPassword true berarti seluruh endpoint selain ganti kata sandi,
	// keluar, dan profil menjawab 403 PASSWORD_CHANGE_REQUIRED.
	WajibGantiPassword bool `json:"wajib_ganti_password"`
	// TerkunciSampai hanya ada pada daftar dan detail pengguna (pengguna.kelola) untuk akun yang login-nya
	// sedang terkunci; kosong bila tidak terkunci.
	TerkunciSampai *time.Time `json:"terkunci_sampai,omitempty"`
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

		WajibGantiPassword: u.WajibGantiPassword,
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

// MFASetupResponse adalah bahan setup MFA: rahasia untuk entri manual, URI otpauth, dan QR sebagai SVG.
type MFASetupResponse struct {
	Secret          string `json:"secret"`
	ProvisioningURI string `json:"provisioning_uri"`
	QRSVG           string `json:"qr_svg"`
}

// MFAVerifyResponse adalah hasil aktivasi MFA: sesi baru dan kode cadangan yang hanya tampil sekali.
type MFAVerifyResponse struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	KodeCadangan []string `json:"kode_cadangan"`
}

// MFABackupCodesResponse memuat kode cadangan baru yang hanya tampil sekali.
type MFABackupCodesResponse struct {
	KodeCadangan []string `json:"kode_cadangan"`
}
