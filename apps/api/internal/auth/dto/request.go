package dto

// ── Auth Requests ─────────────────────────────────────────────────────

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required,min=8"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type ResetPasswordRequest struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required,min=8"`
}

// ── User Requests ─────────────────────────────────────────────────────

type CreateUserRequest struct {
	ID       string `json:"id" binding:"required"`
	Nama     string `json:"nama" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	Peran    string `json:"peran" binding:"required"`
	Lapis    string `json:"lapis" binding:"required"`
	Lembaga  string `json:"lembaga" binding:"required"`
}

type UpdateUserRequest struct {
	Nama          *string `json:"nama"`
	Email         *string `json:"email" binding:"omitempty,email"`
	Peran         *string `json:"peran"`
	Lapis         *string `json:"lapis"`
	Lembaga       *string `json:"lembaga"`
	KabupatenKode *string `json:"kabupaten_kode"`
	Status        *string `json:"status" binding:"omitempty,oneof=AKTIF NONAKTIF MENUNGGU"`
}

// ListUserRequest adalah kueri GET /v1/users. Filternya (FilterPenggunaRequest) dipakai bersama ekspor CSV.
type ListUserRequest struct {
	FilterPenggunaRequest
	Page  int `form:"page" binding:"omitempty,min=1"`
	Limit int `form:"limit" binding:"omitempty,min=1,max=100"`
	// Sort dan Arah divalidasi domain.DaftarPenggunaQuery.Normal (daftar putih kolom).
	Sort string `form:"sort"`
	Arah string `form:"arah"`
}

type AssignRoleRequest struct {
	Peran string `json:"peran" binding:"required"`
}

type BulkUserRequest struct {
	Action string   `json:"action" binding:"required,oneof=aktifkan nonaktifkan wajibkan_mfa akhiri_sesi"`
	IDs    []string `json:"ids" binding:"required,min=1"`
}

type ResetPasswordAdminRequest struct {
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// ── MFA Requests ──────────────────────────────────────────────────────

type MFAVerifyRequest struct {
	Code string `json:"code" binding:"required,len=6"`
}

// MFAValidateRequest menukar token tantangan dari login dan kode OTP (enam digit) atau kode cadangan dengan sesi.
type MFAValidateRequest struct {
	MFAToken string `json:"mfa_token" binding:"required"`
	Code     string `json:"code" binding:"required,max=32"`
}

type MFADisableRequest struct {
	Password string `json:"password" binding:"required"`
}

type MFARegenerateRequest struct {
	Password string `json:"password" binding:"required"`
}
