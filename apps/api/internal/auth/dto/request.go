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

type ListUserRequest struct {
	Page   int    `form:"page" binding:"omitempty,min=1"`
	Limit  int    `form:"limit" binding:"omitempty,min=1,max=100"`
	Search string `form:"search"`
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

type MFAValidateRequest struct {
	Code string `json:"code" binding:"required"`
}

type MFADisableRequest struct {
	Password string `json:"password" binding:"required"`
}
