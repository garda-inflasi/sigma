package dto

import "time"

// Kode galat auth yang stabil (kolom `code` di amplop galat). Web (BFF dan form) memetakan kode ini ke
// pesan pengguna (apps/web/src/lib/auth/pesan-login.ts); jangan diubah tanpa mengubah keduanya dan OpenAPI.
const (
	// KodeKredensialSalah: email atau kata sandi salah (401). Sama untuk email terdaftar maupun tidak.
	KodeKredensialSalah = "INVALID_CREDENTIALS"
	// KodeAkunTerkunci: terlalu banyak percobaan gagal (423); data memuat AkunTerkunciData.
	KodeAkunTerkunci = "ACCOUNT_LOCKED"
	// KodeAkunNonaktif: akun dinonaktifkan (403). Hanya dijawab setelah kata sandi terbukti benar.
	KodeAkunNonaktif = "ACCOUNT_INACTIVE"
	// KodeMFAKodeSalah: kode OTP atau kode cadangan salah.
	KodeMFAKodeSalah = "MFA_CODE_INVALID"
	// KodeMFASesiBerakhir: token tantangan MFA tidak valid atau kedaluwarsa; pengguna harus masuk ulang.
	KodeMFASesiBerakhir = "MFA_CHALLENGE_EXPIRED"
)

// Pesan bawaan untuk kode di atas (klien web memakai tabel pesannya sendiri; ini untuk klien API lain).
const (
	PesanKredensialSalah = "Email atau password salah"
	PesanAkunTerkunci    = "Akun terkunci karena terlalu banyak percobaan gagal. Coba lagi nanti."
	PesanAkunNonaktif    = "Akun tidak aktif, hubungi administrator"
	PesanMFAKodeSalah    = "Kode verifikasi tidak valid"
	PesanMFASesiBerakhir = "Sesi verifikasi MFA tidak valid atau sudah kedaluwarsa, masuk ulang"
)

// AkunTerkunciData adalah `data` pada galat 423 ACCOUNT_LOCKED. Header Retry-After memuat SisaDetik.
type AkunTerkunciData struct {
	// TerkunciSampai adalah akhir kunci (UTC).
	TerkunciSampai time.Time `json:"terkunci_sampai"`
	// SisaDetik adalah sisa waktu kunci dibulatkan ke atas.
	SisaDetik int `json:"sisa_detik"`
}

// BukaKunciResponse adalah hasil POST /v1/users/:id/buka-kunci.
type BukaKunciResponse struct {
	ID string `json:"id"`
	// TerkunciSebelumnya true bila akun memang sedang terkunci ketika dibuka.
	TerkunciSebelumnya bool `json:"terkunci_sebelumnya"`
}
