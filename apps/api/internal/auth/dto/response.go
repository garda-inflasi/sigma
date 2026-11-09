package dto

import (
	"time"

	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/pkg/pii"
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
	AccessToken string `json:"access_token"`
	// RefreshToken tidak ada pada pemutaran ulang dalam jendela toleransi (dua tab menyegarkan sesi
	// bersamaan): klien mempertahankan refresh token dari permintaan yang menang.
	RefreshToken string `json:"refresh_token,omitempty"`
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
	Telepon       *string    `json:"telepon,omitempty"` // E.164 (+6281234567890) utuh hanya untuk pemilik akun; untuk pihak lain tersamar (+62********890)
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

// RoleResponse adalah satu peran dengan izinnya yang berlaku (basis data). Bawaan adalah matriks bawaan di
// kode (pembanding "pulihkan bawaan"); Versi adalah kunci optimistis penyuntingan matriks.
type RoleResponse struct {
	ID             string     `json:"id"`
	Label          string     `json:"label"`
	Lapis          string     `json:"lapis"`
	Permissions    []string   `json:"permissions"`
	Bawaan         []string   `json:"bawaan"`
	Versi          int64      `json:"versi"`
	DiperbaruiPada *time.Time `json:"diperbarui_pada,omitempty"`
	DiperbaruiOleh *string    `json:"diperbarui_oleh,omitempty"`
}

// PermissionResponse adalah satu izin di katalog (didefinisikan di kode; UI tidak dapat menciptakan izin baru).
type PermissionResponse struct {
	ID        string `json:"id"`
	Deskripsi string `json:"deskripsi"`
	Kelompok  string `json:"kelompok"`
}

// ── Helpers ───────────────────────────────────────────────────────────

// ToUserResponse memetakan pengguna untuk PEMILIKNYA sendiri (login, validasi MFA, SSO, GET dan PATCH /v1/auth/me):
// telepon tampil utuh. Respons untuk pihak lain (daftar, detail, ubah, ganti peran, dan undangan oleh admin) wajib
// memakai ToUserResponseTersamar.
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
		Telepon:       u.Telepon,
		MfaAktif:      u.MfaAktif,
		Status:        u.Status,
		TerakhirMasuk: u.TerakhirMasuk,
		DibuatPada:    u.DibuatPada,

		WajibGantiPassword: u.WajibGantiPassword,
	}
}

// ToUserResponseTersamar memetakan pengguna untuk pihak selain pemiliknya (UU PDP): nomor telepon disamarkan
// (pii.TeleponTersamar), jadi admin yang mengelola akun tidak pernah membaca nomor utuh. Ia hanya dapat mengganti
// atau menghapusnya (PATCH /v1/users/:id). Nomor yang tidak ada tetap tidak ada.
func ToUserResponseTersamar(u *domain.Pengguna) UserResponse {
	r := ToUserResponse(u)
	if r.Telepon != nil {
		if tersamar := pii.TeleponTersamar(*r.Telepon); tersamar != "" {
			r.Telepon = &tersamar
		} else {
			r.Telepon = nil
		}
	}
	return r
}

// ToUserResponsesTersamar memetakan daftar pengguna untuk pihak lain (ToUserResponseTersamar). Tidak ada padanan
// untuk pemilik: seorang pemilik hanya melihat dirinya sendiri.
func ToUserResponsesTersamar(users []domain.Pengguna) []UserResponse {
	items := make([]UserResponse, len(users))
	for i := range users {
		items[i] = ToUserResponseTersamar(&users[i])
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
