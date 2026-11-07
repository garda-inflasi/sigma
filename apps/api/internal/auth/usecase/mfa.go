package usecase

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/pkg/totp"
)

// JumlahKodeCadangan adalah banyaknya kode cadangan yang diterbitkan tiap aktivasi atau regenerasi.
const JumlahKodeCadangan = 10

// kodeTOTP adalah bentuk kode aplikasi autentikator: enam digit.
var kodeTOTP = regexp.MustCompile(`^[0-9]{6}$`)

// Metode faktor kedua yang dicatat di audit.
const (
	metodeTOTP         = "totp"
	metodeKodeCadangan = "kode_cadangan"
)

// HashKodeCadangan menghitung hash kode cadangan seperti yang disimpan di mfa_kode_cadangan:
// SHA-256 dari "<id pengguna>:<kode ternormalisasi>". Migrasi 000063 memakai rumus yang sama. Kode yang terbit
// sebelum ID pengguna menjadi UUID (000110) di-hash dengan ID lamanya; lihat pakaiKodeCadanganLama.
func HashKodeCadangan(userID, kode string) string {
	return sha256Hex(userID + ":" + totp.NormalizeBackupCode(kode))
}

// MFASetup adalah bahan setup MFA yang ditampilkan sekali: rahasia untuk entri manual dan URI otpauth untuk QR.
type MFASetup struct {
	Secret          string
	ProvisioningURI string
}

// MFAActivation adalah hasil aktivasi MFA: sesi baru (sesi lama dicabut dan klaim wajib setup hilang)
// dan kode cadangan dalam bentuk polos, yang tidak dapat ditampilkan lagi.
type MFAActivation struct {
	AccessToken  string
	RefreshToken string
	KodeCadangan []string
}

// MFAStatus adalah keadaan MFA akun untuk profil.
type MFAStatus struct {
	// SetupDiwajibkan true bila akun wajib MFA tetapi belum mengaktifkannya.
	SetupDiwajibkan bool
	// Wajib true bila MFA tidak boleh dinonaktifkan oleh pemilik akun.
	Wajib bool
	// KodeCadanganTersisa terisi hanya untuk akun dengan MFA aktif.
	KodeCadanganTersisa *int
}

// mfaWajibPeran adalah peran yang wajib MFA: lapis strategis, manajerial, koordinator lapangan, dan admin.
func mfaWajibPeran(peran string) bool {
	switch roleLapis[peran] {
	case "STRATEGIS", "MANAJERIAL", "SISTEM":
		return true
	}
	return peran == "OPS_KOORDINATOR"
}

// mfaWajib melaporkan apakah MFA diwajibkan untuk akun ini (hanya bila MFAEnforce menyala).
func (uc *UseCase) mfaWajib(user *domain.Pengguna) bool {
	return uc.cfg.MFAEnforce && (mfaWajibPeran(user.Peran) || user.MfaWajib)
}

func (uc *UseCase) mfaSetupRequired(user *domain.Pengguna) bool {
	return uc.mfaWajib(user) && !user.MfaAktif
}

// MFAStatus melaporkan keadaan MFA akun.
func (uc *UseCase) MFAStatus(ctx context.Context, user *domain.Pengguna) (MFAStatus, error) {
	status := MFAStatus{SetupDiwajibkan: uc.mfaSetupRequired(user), Wajib: uc.mfaWajib(user)}
	if user.MfaAktif {
		n, err := uc.repo.CountBackupCodes(ctx, user.ID)
		if err != nil {
			return status, fmt.Errorf("hitung kode cadangan: %w", err)
		}
		sisa := int(n)
		status.KodeCadanganTersisa = &sisa
	}
	return status, nil
}

func (uc *UseCase) openSecret(user *domain.Pengguna) (string, error) {
	if uc.cfg.MFAKey == nil {
		return "", ErrMFAUnavailable
	}
	if user.MfaSecret == nil || *user.MfaSecret == "" {
		return "", ErrMFANotSetup
	}
	secret, err := uc.cfg.MFAKey.Open(*user.MfaSecret)
	if err != nil {
		return "", fmt.Errorf("buka rahasia MFA: %w", err)
	}
	return secret, nil
}

// SetupMFA menyiapkan rahasia TOTP baru (belum aktif sampai VerifyMFA). Memanggilnya lagi sebelum
// verifikasi mengganti rahasia yang belum terpakai.
func (uc *UseCase) SetupMFA(ctx context.Context, userID string) (*MFASetup, error) {
	if uc.cfg.MFAKey == nil {
		return nil, ErrMFAUnavailable
	}
	user, err := uc.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, ErrUserNotFound
	}
	if user.MfaAktif {
		return nil, ErrMFAAlreadyActive
	}

	secret := totp.GenerateSecret()
	sealed, err := uc.cfg.MFAKey.Seal(secret)
	if err != nil {
		return nil, fmt.Errorf("segel rahasia MFA: %w", err)
	}
	// Hanya rahasia setup yang ditulis, dan hanya bila MFA belum aktif: menyimpan seluruh baris dari salinan
	// yang dibaca di atas akan mematikan MFA yang diaktifkan serentak oleh VerifyMFA.
	ok, err := uc.repo.SetMFASecret(ctx, user.ID, sealed)
	if err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}
	if !ok {
		return nil, ErrMFAAlreadyActive
	}

	uc.recordAudit(ctx, userID, "MFA_SETUP", userID)
	return &MFASetup{Secret: secret, ProvisioningURI: totp.GenerateProvisioningURI(secret, user.Email)}, nil
}

// VerifyMFA memverifikasi kode pertama dari aplikasi autentikator lalu mengaktifkan MFA. Kode cadangan
// diterbitkan di sini (sekali tampil) dan sesi diganti: sesi lama dicabut, sesi baru tidak lagi
// membawa klaim wajib setup MFA.
func (uc *UseCase) VerifyMFA(ctx context.Context, userID, code, ip string) (*MFAActivation, error) {
	user, err := uc.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, ErrUserNotFound
	}
	if user.MfaAktif {
		return nil, ErrMFAAlreadyActive
	}
	secret, err := uc.openSecret(user)
	if err != nil {
		return nil, err
	}
	kunci, err := uc.mulaiPercobaan(ctx, user.Email)
	if err != nil {
		return nil, err
	}

	langkah, ok := totp.ValidateTOTPStep(secret, strings.TrimSpace(code), time.Now())
	if !ok {
		uc.recordLoginAttempt(ctx, &user.ID, ip, false, "mfa_setup_invalid")
		return nil, ErrMFACodeInvalid
	}
	uc.kembalikanPercobaan(ctx, kunci)

	kode := totp.GenerateBackupCodes(JumlahKodeCadangan)
	// Aktivasi bersyarat: MFA belum aktif dan rahasia tersimpan masih yang kodenya baru diverifikasi. Tanpa itu
	// dua verifikasi serentak sama-sama menerbitkan kode cadangan (yang pertama langsung hangus), dan setup yang
	// diulang di tab lain mengaktifkan rahasia baru yang belum pernah dipindai pengguna.
	aktif, err := uc.repo.ActivateMFA(ctx, user.ID, hashKode(user.ID, kode), langkah, *user.MfaSecret)
	if err != nil {
		return nil, fmt.Errorf("aktifkan MFA: %w", err)
	}
	if !aktif {
		return nil, uc.alasanAktivasiDitolak(ctx, user.ID)
	}
	user.MfaAktif, user.MfaLangkahTerakhir = true, langkah

	accessToken, refreshToken, err := uc.replaceCurrentSession(ctx, user, ip)
	if err != nil {
		return nil, err
	}

	uc.recordAudit(ctx, userID, "MFA_VERIFY", userID)
	return &MFAActivation{AccessToken: accessToken, RefreshToken: refreshToken, KodeCadangan: kode}, nil
}

// alasanAktivasiDitolak menjelaskan aktivasi MFA bersyarat yang tidak menulis apa pun: MFA sudah aktif, atau
// rahasia setup diganti setelah kode diverifikasi.
func (uc *UseCase) alasanAktivasiDitolak(ctx context.Context, userID string) error {
	fresh, err := uc.repo.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("muat ulang pengguna: %w", err)
	}
	if fresh.MfaAktif {
		return ErrMFAAlreadyActive
	}
	return ErrMFASetupBerubah
}

// ValidateMFALogin menukar token tantangan (dari Login) dan kode OTP atau kode cadangan dengan sesi.
// Percobaan yang salah dihitung terhadap batas kunci akun yang sama dengan kata sandi salah.
func (uc *UseCase) ValidateMFALogin(ctx context.Context, mfaToken, code, ip string) (*LoginResult, error) {
	challenge, err := uc.token.VerifyMFAChallenge(mfaToken)
	if err != nil {
		return nil, ErrMFAChallengeInvalid
	}
	// Metode login langkah pertama (mis. SSO) dibawa token tantangan; audit LOGIN akhir mencatatnya bersama faktor kedua.
	ctx = withLoginMethod(ctx, challenge.LoginMethod)
	user, err := uc.repo.GetUserByID(ctx, challenge.UserID)
	if err != nil || user.Status != "AKTIF" || !user.MfaAktif {
		return nil, ErrMFAChallengeInvalid
	}
	kunci, err := uc.mulaiPercobaan(ctx, user.Email)
	if err != nil {
		if errors.Is(err, ErrAccountLocked) {
			uc.recordFailedLogin(ctx, user.ID, user.Email, alasanAkunTerkunci)
		}
		return nil, err
	}

	metode, err := uc.verifySecondFactor(ctx, user, code)
	if err != nil {
		if errors.Is(err, ErrMFACodeInvalid) {
			uc.recordLoginAttempt(ctx, &user.ID, ip, false, "mfa_invalid")
			uc.recordFailedLogin(ctx, user.ID, user.Email, "mfa_invalid")
		} else {
			// Bukan tebakan salah (mis. MFA belum terkonfigurasi atau basis data galat): jangan dihitung.
			uc.kembalikanPercobaan(ctx, kunci)
		}
		return nil, err
	}
	return uc.completeLogin(ctx, user, ip, metode)
}

// verifySecondFactor memeriksa kode TOTP (enam digit, sekali pakai per langkah waktu) atau, selain itu,
// kode cadangan (sekali pakai). Mengembalikan metode yang cocok.
func (uc *UseCase) verifySecondFactor(ctx context.Context, user *domain.Pengguna, code string) (string, error) {
	code = strings.TrimSpace(code)
	if kodeTOTP.MatchString(code) {
		secret, err := uc.openSecret(user)
		if err != nil {
			return "", err
		}
		langkah, ok := totp.ValidateTOTPStep(secret, code, time.Now())
		if !ok {
			return "", ErrMFACodeInvalid
		}
		maju, err := uc.repo.AdvanceMFAStep(ctx, user.ID, langkah)
		if err != nil {
			return "", fmt.Errorf("catat langkah MFA: %w", err)
		}
		if !maju {
			return "", ErrMFACodeInvalid
		}
		user.MfaLangkahTerakhir = langkah
		return metodeTOTP, nil
	}

	pakai, err := uc.repo.UseBackupCode(ctx, user.ID, HashKodeCadangan(user.ID, code), time.Now())
	if err == nil && !pakai {
		pakai, err = uc.pakaiKodeCadanganLama(ctx, user.ID, code)
	}
	if err != nil {
		return "", fmt.Errorf("pakai kode cadangan: %w", err)
	}
	if !pakai {
		return "", ErrMFACodeInvalid
	}
	return metodeKodeCadangan, nil
}

// pakaiKodeCadanganLama mencocokkan kode cadangan yang terbit sebelum migrasi 000110: hash-nya dihitung dari ID
// lama (slug) akun, bukan UUID-nya, dan tidak bisa dihitung ulang karena kode polosnya tidak disimpan. Akun yang
// dibuat sesudah migrasi tidak punya ID lama, jadi jalur ini selesai tanpa mencocokkan apa pun. Kode yang
// diterbitkan ulang (regenerasi, aktivasi MFA) sudah memakai UUID.
func (uc *UseCase) pakaiKodeCadanganLama(ctx context.Context, userID, code string) (bool, error) {
	lama, err := uc.repo.IDLamaPengguna(ctx, userID)
	if err != nil || lama == "" {
		return false, err
	}
	return uc.repo.UseBackupCode(ctx, userID, HashKodeCadangan(lama, code), time.Now())
}

// RegenerateBackupCodes menerbitkan kode cadangan baru (yang lama hangus) setelah konfirmasi kata sandi,
// yang dihitung lockout seperti ganti kata sandi (lihat periksaKataSandiPemilik). Kode polos hanya
// dikembalikan di sini.
func (uc *UseCase) RegenerateBackupCodes(ctx context.Context, userID, password, ip string) ([]string, error) {
	user, err := uc.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, ErrUserNotFound
	}
	if !user.MfaAktif {
		return nil, ErrMFANotSetup
	}
	if err := uc.confirmPassword(ctx, user, password, ip); err != nil {
		return nil, err
	}

	kode := totp.GenerateBackupCodes(JumlahKodeCadangan)
	if err := uc.repo.ReplaceBackupCodes(ctx, user.ID, hashKode(user.ID, kode)); err != nil {
		return nil, fmt.Errorf("simpan kode cadangan: %w", err)
	}
	uc.recordAudit(ctx, userID, "MFA_BACKUP_REGENERATE", userID)
	return kode, nil
}

// DisableMFA menonaktifkan MFA setelah konfirmasi kata sandi, yang dihitung lockout seperti ganti kata
// sandi (lihat periksaKataSandiPemilik). Ditolak untuk akun yang MFA-nya diwajibkan.
func (uc *UseCase) DisableMFA(ctx context.Context, userID, password, ip string) error {
	user, err := uc.repo.GetUserByID(ctx, userID)
	if err != nil {
		return ErrUserNotFound
	}
	if err := uc.confirmPassword(ctx, user, password, ip); err != nil {
		return err
	}
	if uc.mfaWajib(user) {
		return ErrMFAMandatory
	}

	if err := uc.repo.ClearMFA(ctx, user.ID); err != nil {
		return fmt.Errorf("nonaktifkan MFA: %w", err)
	}
	uc.recordAudit(ctx, userID, "MFA_DISABLE", userID)
	return nil
}

// confirmPassword meminta kata sandi pemilik sebelum aksi sensitif MFA. Memakai lockout yang sama dengan
// ganti kata sandi: tanpa itu pemegang access token curian dapat menebak kata sandi di sini tanpa batas.
func (uc *UseCase) confirmPassword(ctx context.Context, user *domain.Pengguna, password, ip string) error {
	return uc.periksaKataSandiPemilik(ctx, user, password, ip, "wrong_password_confirm")
}

func hashKode(userID string, kode []string) []string {
	out := make([]string, len(kode))
	for i, k := range kode {
		out[i] = HashKodeCadangan(userID, k)
	}
	return out
}
