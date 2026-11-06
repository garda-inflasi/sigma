// Package usecase provides the business logic for authentication operations.
package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/pkg/crypto"
	"github.com/garda-inflasi/api/pkg/email"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/token"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ── Errors ────────────────────────────────────────────────────────────

var (
	ErrInvalidCredentials = errors.New("email atau password salah")
	ErrAccountLocked      = errors.New("akun terkunci, coba lagi nanti")
	ErrAccountInactive    = errors.New("akun tidak aktif")
	ErrInvalidToken       = errors.New("token tidak valid")
	ErrTokenExpired       = errors.New("token sudah kedaluwarsa")
	ErrUserNotFound       = errors.New("pengguna tidak ditemukan")
	ErrEmailExists        = errors.New("email sudah terdaftar")
	ErrWeakPassword       = errors.New("kata sandi terlalu lemah")
	ErrPasswordReuse      = errors.New("kata sandi sudah pernah digunakan sebelumnya")
	ErrWrongPassword      = errors.New("kata sandi lama salah")
	ErrResetTokenInvalid  = errors.New("token atur ulang tidak valid")
	ErrResetTokenExpired  = errors.New("token atur ulang sudah kedaluwarsa")
	ErrInvalidAction      = errors.New("aksi tidak valid")
	ErrRoleNotFound       = errors.New("peran tidak dikenal")
	ErrCannotDeleteSelf   = errors.New("tidak dapat menghapus akun sendiri")
	ErrUserHasReferences  = errors.New("pengguna masih memiliki data terkait")

	ErrMFAChallengeInvalid = errors.New("sesi verifikasi MFA tidak valid atau sudah kedaluwarsa, masuk ulang")
	ErrMFACodeInvalid      = errors.New("kode MFA tidak valid")
	ErrMFAAlreadyActive    = errors.New("MFA sudah aktif")
	ErrMFANotSetup         = errors.New("MFA belum disiapkan, mulai setup lebih dulu")
	ErrMFAMandatory        = errors.New("MFA wajib untuk akun Anda dan tidak dapat dinonaktifkan")
	ErrMFAUnavailable      = errors.New("kunci enkripsi MFA belum dikonfigurasi")
)

// ── Password Policy ───────────────────────────────────────────────────

var (
	minPasswordLength = 8
	// Common password deny-list (top 100 most common passwords)
	denyList = map[string]bool{
		"password": true, "12345678": true, "123456789": true, "qwerty123": true,
		"admin123": true, "letmein": true, "welcome": true, "monkey": true,
		"dragon": true, "master": true, "acehkukaya": true,
	}
)

// ValidatePassword checks password strength against policy.
func ValidatePassword(password string) error {
	if len(password) < minPasswordLength {
		return fmt.Errorf("%w: minimal %d karakter", ErrWeakPassword, minPasswordLength)
	}

	if denyList[strings.ToLower(password)] {
		return fmt.Errorf("%w: kata sandi terlalu umum", ErrWeakPassword)
	}

	// Check for at least one uppercase letter
	hasUpper := regexp.MustCompile(`[A-Z]`).MatchString
	if !hasUpper(password) {
		return fmt.Errorf("%w: harus mengandung huruf kapital", ErrWeakPassword)
	}

	// Check for at least one digit
	hasDigit := regexp.MustCompile(`[0-9]`).MatchString
	if !hasDigit(password) {
		return fmt.Errorf("%w: harus mengandung angka", ErrWeakPassword)
	}

	// Check for at least one special character
	hasSpecial := regexp.MustCompile(`[^a-zA-Z0-9]`).MatchString
	if !hasSpecial(password) {
		return fmt.Errorf("%w: harus mengandung karakter khusus", ErrWeakPassword)
	}

	return nil
}

// ── Config ────────────────────────────────────────────────────────────

type Config struct {
	AccessTokenTTL    time.Duration
	RefreshTokenTTL   time.Duration
	MaxFailedAttempts int
	LockoutDuration   time.Duration
	ResetTokenTTL     time.Duration
	// MFAEnforce mewajibkan MFA bagi peran wajib (lihat mfaWajibPeran) dan akun bertanda mfa_wajib:
	// sebelum setup, sesi mereka hanya membuka endpoint setup MFA. Akun yang sudah mengaktifkan MFA
	// selalu ditantang kode OTP, apa pun nilai ini.
	MFAEnforce bool
	// MFAKey menyegel rahasia TOTP di basis data; nil membuat setup dan verifikasi MFA gagal dengan ErrMFAUnavailable.
	MFAKey *crypto.Keyring
	// RefreshGraceWindow adalah jendela setelah rotasi saat refresh token lama yang disajikan lagi dianggap
	// permintaan serentak yang sah (mis. dua tab), bukan pemakaian ulang. 0 mematikan toleransi. Lihat sesi.go.
	RefreshGraceWindow time.Duration
}

func DefaultConfig() Config {
	return Config{
		AccessTokenTTL:    15 * time.Minute,
		RefreshTokenTTL:   7 * 24 * time.Hour,
		MaxFailedAttempts: 5,
		LockoutDuration:   15 * time.Minute,
		ResetTokenTTL:     30 * time.Minute,

		RefreshGraceWindow: DefaultRefreshGraceWindow,
	}
}

// ── UseCase ───────────────────────────────────────────────────────────

type UseCase struct {
	repo        domain.Repository
	token       *token.Manager
	cfg         Config
	log         *logger.Logger
	emailSender *email.Sender
	audit       audit.Writer
}

func New(repo domain.Repository, tokenMgr *token.Manager, cfg Config, log *logger.Logger, emailSender *email.Sender, auditWriter audit.Writer) *UseCase {
	return &UseCase{
		repo:        repo,
		token:       tokenMgr,
		cfg:         cfg,
		log:         log,
		emailSender: emailSender,
		audit:       auditWriter,
	}
}

// ── Auth Use Cases ────────────────────────────────────────────────────

// LoginResult adalah hasil langkah kata sandi. Bila MFARequired, belum ada sesi: klien menukar MFAToken
// dan kode OTP lewat ValidateMFALogin. Selain itu AccessToken dan RefreshToken terisi.
type LoginResult struct {
	User         *domain.Pengguna
	AccessToken  string
	RefreshToken string
	MFARequired  bool
	MFAToken     string
	// MFASetupRequired true bila akun wajib MFA tetapi belum mengaktifkannya; sesinya hanya membuka endpoint setup.
	MFASetupRequired bool
}

// Login authenticates a user. Akun dengan MFA aktif menerima token tantangan, bukan sesi.
func (uc *UseCase) Login(ctx context.Context, email, password, ip string) (*LoginResult, error) {
	// Lockout dihitung per email (lihat lockout.go): email tak terdaftar mendapat jawaban yang sama
	// persis dengan email terdaftar, jadi kunci diperiksa sebelum akun dicari.
	kunci, err := uc.mulaiPercobaan(ctx, email)
	if err != nil {
		if errors.Is(err, ErrAccountLocked) {
			uc.auditTerkunci(ctx, email)
		}
		return nil, err
	}

	user, err := uc.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			uc.kembalikanPercobaan(ctx, kunci)
			return nil, fmt.Errorf("get user by email: %w", err)
		}
		uc.log.Warn("login: user not found", map[string]interface{}{"email": email, "ip": ip})
		uc.recordLoginAttempt(ctx, nil, ip, false, "user_not_found")
		uc.recordFailedLogin(ctx, "", email, "user_not_found")
		perlambatPalsu(password)
		return nil, ErrInvalidCredentials
	}

	if user.PasswordHash == nil {
		uc.log.Warn("login: no password set (SSO only)", map[string]interface{}{"user_id": user.ID})
		uc.recordLoginAttempt(ctx, &user.ID, ip, false, "no_password")
		uc.recordFailedLogin(ctx, user.ID, email, "no_password")
		perlambatPalsu(password)
		return nil, ErrInvalidCredentials
	}

	if !crypto.VerifyPassword(password, *user.PasswordHash) {
		uc.log.Warn("login: wrong password", map[string]interface{}{"user_id": user.ID, "ip": ip})
		uc.recordLoginAttempt(ctx, &user.ID, ip, false, "wrong_password")
		uc.recordFailedLogin(ctx, user.ID, email, "wrong_password")
		return nil, ErrInvalidCredentials
	}

	// Status akun baru diperiksa setelah kata sandi terbukti benar: email dengan kata sandi salah mendapat
	// 401 yang sama baik akunnya aktif maupun tidak. Kata sandi benar bukan tebakan, jadi tidak dihitung.
	if user.Status != "AKTIF" {
		uc.kembalikanPercobaan(ctx, kunci)
		uc.log.Warn("login: inactive account", map[string]interface{}{"user_id": user.ID, "status": user.Status})
		uc.recordLoginAttempt(ctx, &user.ID, ip, false, "account_inactive")
		uc.recordFailedLogin(ctx, user.ID, email, "account_inactive")
		return nil, ErrAccountInactive
	}

	if user.MfaAktif {
		// Kata sandi benar bukan tebakan gagal, dan juga tidak menghapus hitungan: itu baru terjadi saat
		// OTP lolos (completeLogin), supaya kata sandi yang bocor tidak memberi tebakan OTP tanpa batas.
		uc.kembalikanPercobaan(ctx, kunci)
		challenge, err := uc.token.CreateMFAChallengeToken(user.ID)
		if err != nil {
			return nil, fmt.Errorf("create mfa challenge: %w", err)
		}
		uc.log.Info("login: password ok, menunggu kode MFA", map[string]interface{}{"user_id": user.ID, "ip": ip})
		return &LoginResult{User: user, MFARequired: true, MFAToken: challenge}, nil
	}
	return uc.completeLogin(ctx, user, ip, "")
}

// completeLogin menerbitkan sesi untuk pengguna yang seluruh faktornya sudah terbukti. metodeMFA
// ("totp" atau "kode_cadangan") dicatat di audit; kosong bila akun tanpa MFA.
func (uc *UseCase) completeLogin(ctx context.Context, user *domain.Pengguna, ip, metodeMFA string) (*LoginResult, error) {
	accessToken, rawRefresh, err := uc.issueSession(ctx, user, uuid.New().String(), ip)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	uc.recordLoginAttempt(ctx, &user.ID, ip, true, "")
	uc.resetKunci(ctx, user.Email)

	// Hanya kolom terakhir_masuk. Menyimpan seluruh baris (Save) dari salinan yang dibaca sebelum bcrypt
	// membatalkan perubahan admin yang terjadi di sela-selanya: penonaktifan, reset kata sandi, atau
	// pergantian peran akan tertimpa kembali ke nilai lama oleh login yang sedang berjalan.
	user.TerakhirMasuk = &now
	if err := uc.repo.TouchLastLogin(ctx, user.ID, now); err != nil {
		uc.log.Warn("login: gagal mencatat waktu masuk terakhir", map[string]interface{}{"user_id": user.ID, "error": err.Error()})
	}

	entry := audit.Entry{AktorID: user.ID, Aksi: "LOGIN", Entitas: "pengguna", EntitasID: user.ID, Hasil: audit.HasilBerhasil}
	if metodeMFA != "" {
		entry.Metadata = map[string]any{"mfa": metodeMFA}
	}
	entry.Metadata = withLoginMethodMetadata(ctx, entry.Metadata)
	uc.writeAudit(ctx, entry)
	uc.log.Info("login success", map[string]interface{}{"user_id": user.ID, "email": user.Email, "ip": ip})
	return &LoginResult{User: user, AccessToken: accessToken, RefreshToken: rawRefresh, MFASetupRequired: uc.mfaSetupRequired(user)}, nil
}

// Refresh rotates a refresh token. Wrapper tipis RefreshSession untuk pemanggil yang hanya butuh token;
// pada pemutaran ulang yang ditoleransi refresh token yang dikembalikan kosong.
func (uc *UseCase) Refresh(ctx context.Context, refreshTokenStr, ip string) (string, string, error) {
	res, err := uc.RefreshSession(ctx, refreshTokenStr, ip)
	if err != nil {
		return "", "", err
	}
	return res.AccessToken, res.RefreshToken, nil
}

// Logout mengakhiri sesi yang sedang dipakai dan memasukkan JWT-nya ke daftar cabut (lihat LogoutSession).
func (uc *UseCase) Logout(ctx context.Context, userID string, jti string) error {
	return uc.LogoutSession(ctx, userID, jti, "")
}

// GetMe returns the current user profile.
func (uc *UseCase) GetMe(ctx context.Context, userID string) (*domain.Pengguna, error) {
	user, err := uc.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, ErrUserNotFound
	}
	return user, nil
}

// ── Password Management ───────────────────────────────────────────────

// ChangePassword mengganti kata sandi setelah memverifikasi yang lama, mematikan
// penanda wajib ganti, lalu menerbitkan sesi baru: semua sesi lama dicabut, dan
// token yang sedang dipakai masih membawa klaim wajib ganti kata sandi.
func (uc *UseCase) ChangePassword(ctx context.Context, userID, oldPassword, newPassword string, ip string) (accessToken, refreshToken string, err error) {
	user, err := uc.repo.GetUserByID(ctx, userID)
	if err != nil {
		return "", "", ErrUserNotFound
	}

	if err := uc.periksaKataSandiPemilik(ctx, user, oldPassword, ip, "wrong_password_change"); err != nil {
		return "", "", err
	}

	if err := ValidatePassword(newPassword); err != nil {
		return "", "", err
	}

	if crypto.VerifyPassword(newPassword, *user.PasswordHash) {
		return "", "", ErrPasswordReuse
	}

	if err := uc.setPassword(ctx, user, newPassword, false); err != nil {
		return "", "", err
	}

	// Sesi yang sedang dipakai tetap hidup (token barunya tanpa penanda wajib ganti); sesi lain dicabut.
	accessToken, refreshToken, err = uc.replaceCurrentSession(ctx, user, ip)
	if err != nil {
		return "", "", err
	}

	uc.recordAudit(ctx, userID, "CHANGE_PASSWORD", userID)
	uc.log.Info("password changed", map[string]interface{}{"user_id": userID, "ip": ip})
	return accessToken, refreshToken, nil
}

// ForgotPassword creates a password reset token and stores it.
func (uc *UseCase) ForgotPassword(ctx context.Context, email string) error {
	user, err := uc.repo.GetUserByEmail(ctx, email)
	if err != nil {
		// Don't reveal whether email exists
		uc.log.Info("forgot password: email not found", map[string]interface{}{"email": email})
		return nil
	}

	if user.Status != "AKTIF" {
		uc.log.Warn("forgot password: inactive account", map[string]interface{}{"user_id": user.ID})
		return nil
	}

	resetToken := uuid.New().String()
	tokenHash := sha256Hex(resetToken)
	expiresAt := time.Now().Add(uc.cfg.ResetTokenTTL)

	if err := uc.repo.SaveResetToken(ctx, user.ID, tokenHash, expiresAt); err != nil {
		return fmt.Errorf("save reset token: %w", err)
	}

	uc.recordAudit(ctx, user.ID, "FORGOT_PASSWORD", user.ID)
	uc.log.Info("forgot password: token created", map[string]interface{}{"user_id": user.ID, "email": email})

	// Email dikirim di latar belakang. SMTP tanpa batas waktu menahan permintaan selama berdetik-detik atau
	// sampai timeout TCP, dan hanya email terdaftar yang sampai ke langkah ini: menunggunya menjadikan lama
	// jawaban penanda bahwa email itu terdaftar (enumerasi) dan menggantung permintaan saat SMTP bermasalah.
	// Jawabannya sama untuk semua email; kegagalan kirim hanya tercatat di log.
	go uc.kirimEmailReset(user.ID, email, resetToken)

	return nil
}

// kirimEmailReset mengirim tautan atur ulang; dipanggil di goroutine sendiri oleh ForgotPassword.
func (uc *UseCase) kirimEmailReset(userID, to, resetToken string) {
	if err := uc.emailSender.SendResetPassword(to, resetToken); err != nil {
		uc.log.Error("forgot password: failed to send email", map[string]interface{}{"user_id": userID, "error": err.Error()})
	}
}

// ResetPassword resets the password using a reset token.
func (uc *UseCase) ResetPassword(ctx context.Context, resetTokenStr, newPassword string) error {
	tokenHash := sha256Hex(resetTokenStr)

	stored, err := uc.repo.GetResetToken(ctx, tokenHash)
	if err != nil {
		return ErrResetTokenInvalid
	}

	if stored.UsedAt != nil {
		return ErrResetTokenInvalid
	}

	if time.Now().After(stored.ExpiresAt) {
		return ErrResetTokenExpired
	}

	if err := ValidatePassword(newPassword); err != nil {
		return err
	}

	user, err := uc.repo.GetUserByID(ctx, stored.UserID)
	if err != nil {
		return ErrUserNotFound
	}

	// Prevent password reuse
	if user.PasswordHash != nil && crypto.VerifyPassword(newPassword, *user.PasswordHash) {
		return ErrPasswordReuse
	}

	// Token dipakai lebih dulu dan atomik: dua permintaan serentak dengan token yang sama
	// tidak bisa sama-sama mengganti kata sandi.
	claimed, err := uc.repo.ConsumeResetToken(ctx, stored.ID, time.Now())
	if err != nil {
		return fmt.Errorf("consume reset token: %w", err)
	}
	if !claimed {
		return ErrResetTokenInvalid
	}

	if err := uc.setPassword(ctx, user, newPassword, false); err != nil {
		uc.log.Error("reset password: gagal menyimpan kata sandi setelah token dipakai", map[string]interface{}{"user_id": user.ID, "error": err.Error()})
		return err
	}

	// Revoke all sessions
	if err := uc.repo.RevokeAllUserRefreshTokens(ctx, stored.UserID); err != nil {
		return fmt.Errorf("revoke sessions: %w", err)
	}

	uc.recordAudit(ctx, stored.UserID, "RESET_PASSWORD", stored.UserID)
	uc.log.Info("password reset success", map[string]interface{}{"user_id": stored.UserID})
	return nil
}

// ── User Use Cases ────────────────────────────────────────────────────

// UserPatch carries the optional fields of a partial user update.
// A nil pointer means "leave unchanged".
type UserPatch struct {
	Nama          *string
	Email         *string
	Peran         *string
	Lapis         *string
	Lembaga       *string
	KabupatenKode *string
	Status        *string
}

// RoleInfo describes a role and the permissions granted to it.
type RoleInfo struct {
	ID          string   `json:"id"`
	Permissions []string `json:"permissions"`
}

// CreateUser creates a new user.
func (uc *UseCase) CreateUser(ctx context.Context, user *domain.Pengguna, password, actorID string) error {
	if err := ValidatePassword(password); err != nil {
		return err
	}

	existing, _ := uc.repo.GetUserByEmail(ctx, user.Email)
	if existing != nil {
		return ErrEmailExists
	}

	hash, err := crypto.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	user.PasswordHash = &hash
	user.WajibGantiPassword = true

	if err := uc.repo.CreateUser(ctx, user); err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	uc.writeAudit(ctx, audit.Entry{
		AktorID: actorID, Aksi: "CREATE_USER", Entitas: "pengguna", EntitasID: user.ID,
		Hasil: audit.HasilBerhasil, Sesudah: userSnapshot(user),
	})
	return nil
}

// ListUsers returns a paginated list of users, optionally filtered by a search term.
func (uc *UseCase) ListUsers(ctx context.Context, page, limit int, search string) ([]domain.Pengguna, int64, error) {
	return uc.repo.ListUsers(ctx, page, limit, search)
}

// GetUser returns a user by ID.
func (uc *UseCase) GetUser(ctx context.Context, id string) (*domain.Pengguna, error) {
	user, err := uc.repo.GetUserByID(ctx, id)
	if err != nil {
		// Hanya "tidak ada" yang menjadi 404; galat basis data tidak boleh menyamar sebagai pengguna hilang.
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

// UpdateUser applies a partial update to an existing user.
func (uc *UseCase) UpdateUser(ctx context.Context, id string, patch UserPatch, actorID string) (*domain.Pengguna, error) {
	user, err := uc.repo.GetUserByID(ctx, id)
	if err != nil {
		return nil, ErrUserNotFound
	}

	sebelum := userSnapshot(user)
	peranSebelum := user.Peran

	if patch.Email != nil && *patch.Email != user.Email {
		if existing, _ := uc.repo.GetUserByEmail(ctx, *patch.Email); existing != nil && existing.ID != id {
			return nil, ErrEmailExists
		}
		user.Email = *patch.Email
	}
	if patch.Nama != nil {
		user.Nama = *patch.Nama
	}
	if patch.Peran != nil {
		if _, ok := rolePermissions[*patch.Peran]; !ok {
			return nil, ErrRoleNotFound
		}
		user.Peran = *patch.Peran
		// Keep lapis consistent with peran unless the caller overrides it.
		if patch.Lapis == nil {
			if lapis, ok := roleLapis[*patch.Peran]; ok {
				user.Lapis = lapis
			}
		}
	}
	if patch.Lapis != nil {
		user.Lapis = *patch.Lapis
	}
	if patch.Lembaga != nil {
		user.Lembaga = *patch.Lembaga
	}
	if patch.KabupatenKode != nil {
		user.KabupatenKode = patch.KabupatenKode
	}
	if patch.Status != nil {
		user.Status = *patch.Status
	}

	if err := uc.repo.UpdateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}

	// Menonaktifkan akun dan mengganti peran sama-sama harus mengakhiri sesi aktifnya: access token lama
	// masih membawa izin peran sebelumnya (AssignRole sudah begitu; PATCH tidak boleh menjadi jalan pintas).
	if (patch.Status != nil && *patch.Status != "AKTIF") || user.Peran != peranSebelum {
		if err := uc.revokeAllSessions(ctx, user.ID); err != nil {
			return nil, err
		}
	}

	uc.writeAudit(ctx, audit.Entry{
		AktorID: actorID, Aksi: "UPDATE_USER", Entitas: "pengguna", EntitasID: user.ID,
		Hasil: audit.HasilBerhasil, Sebelum: sebelum, Sesudah: userSnapshot(user),
	})
	return user, nil
}

// DeleteUser removes a user and terminates all of its sessions.
func (uc *UseCase) DeleteUser(ctx context.Context, id, actorID string) error {
	if id == actorID {
		return ErrCannotDeleteSelf
	}

	if _, err := uc.repo.GetUserByID(ctx, id); err != nil {
		return ErrUserNotFound
	}

	if err := uc.repo.DeleteUser(ctx, id); err != nil {
		// Baris terkait (tugas lapangan, entri harga, audit log, dsb.) memakai
		// ON DELETE NO ACTION, sehingga penghapusan gagal di level basis data.
		// Sesi tidak dicabut lebih dulu: penghapusan yang ditolak tidak boleh mengeluarkan pengguna yang tetap ada.
		if errors.Is(err, gorm.ErrForeignKeyViolated) {
			return ErrUserHasReferences
		}
		return fmt.Errorf("delete user: %w", err)
	}

	// refresh_tokens ikut terhapus (ON DELETE CASCADE) sehingga sesinya mati lewat sid; pencabutan ini hanya
	// jaring pengaman bila kaskade itu tidak ada.
	if err := uc.repo.RevokeAllUserRefreshTokens(ctx, id); err != nil {
		uc.log.Warn("delete user: gagal mencabut sesi sesudah penghapusan", map[string]interface{}{"user_id": id, "error": err.Error()})
	}

	uc.recordAudit(ctx, actorID, "DELETE_USER", id)
	return nil
}

// ResetPasswordAdmin sets a new password for a user without knowing the old one.
func (uc *UseCase) ResetPasswordAdmin(ctx context.Context, id, newPassword, actorID string) error {
	if err := ValidatePassword(newPassword); err != nil {
		return err
	}

	user, err := uc.repo.GetUserByID(ctx, id)
	if err != nil {
		return ErrUserNotFound
	}

	if err := uc.setPassword(ctx, user, newPassword, true); err != nil {
		return err
	}

	// Force re-authentication everywhere after an administrative reset.
	if err := uc.revokeAllSessions(ctx, user.ID); err != nil {
		return err
	}

	uc.recordAudit(ctx, actorID, "RESET_PASSWORD_ADMIN", user.ID)
	return nil
}

// AssignRole changes a user's role and revokes existing sessions so the new
// permission set takes effect on the next login.
func (uc *UseCase) AssignRole(ctx context.Context, id, role, actorID string) (*domain.Pengguna, error) {
	if _, ok := rolePermissions[role]; !ok {
		return nil, ErrRoleNotFound
	}

	user, err := uc.repo.GetUserByID(ctx, id)
	if err != nil {
		return nil, ErrUserNotFound
	}

	sebelum := userSnapshot(user)
	user.Peran = role
	if lapis, ok := roleLapis[role]; ok {
		user.Lapis = lapis
	}
	if err := uc.repo.UpdateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("assign role: %w", err)
	}

	if err := uc.revokeAllSessions(ctx, user.ID); err != nil {
		return nil, err
	}

	uc.writeAudit(ctx, audit.Entry{
		AktorID: actorID, Aksi: "ASSIGN_ROLE", Entitas: "pengguna", EntitasID: user.ID,
		Hasil: audit.HasilBerhasil, Sebelum: sebelum, Sesudah: userSnapshot(user),
	})
	return user, nil
}

// ListRoles returns the hardcoded RBAC role catalogue.
func (uc *UseCase) ListRoles() []RoleInfo {
	roles := make([]RoleInfo, 0, len(rolePermissions))
	for _, r := range roleOrder {
		perms, ok := rolePermissions[r]
		if !ok {
			continue
		}
		roles = append(roles, RoleInfo{ID: r, Permissions: perms})
	}
	return roles
}

// ListPermissions returns every permission known to the RBAC matrix.
func (uc *UseCase) ListPermissions() []string {
	seen := make(map[string]bool)
	perms := make([]string, 0, 64)
	for _, r := range roleOrder {
		for _, p := range rolePermissions[r] {
			if !seen[p] {
				seen[p] = true
				perms = append(perms, p)
			}
		}
	}
	sort.Strings(perms)
	return perms
}

// BulkUserAction processes a bulk action on users.
func (uc *UseCase) BulkUserAction(ctx context.Context, ids []string, action, actorID string) (int64, error) {
	domainAction := domain.BulkUserAction(action)
	switch domainAction {
	case domain.BulkAktifkan, domain.BulkNonaktifkan, domain.BulkWajibkanMFA, domain.BulkAkhiriSesi:
		// valid
	default:
		return 0, ErrInvalidAction
	}

	count, err := uc.repo.BulkUpdateUsers(ctx, ids, domainAction)
	if err != nil {
		uc.log.Error("bulk user action failed", map[string]interface{}{
			"action": action,
			"count":  len(ids),
			"error":  err.Error(),
		})
		return 0, fmt.Errorf("bulk user action: %w", err)
	}

	uc.writeAudit(ctx, audit.Entry{
		AktorID: actorID, Aksi: "BULK_" + string(domainAction), Entitas: "pengguna", EntitasID: fmt.Sprintf("%d pengguna", count),
		Hasil: audit.HasilBerhasil, Metadata: map[string]any{"ids": ids},
	})
	uc.log.Info("bulk user action completed", map[string]interface{}{
		"action": action,
		"count":  count,
	})
	return count, nil
}

// getPermissionsForRole returns the permission list for a given role.
func (uc *UseCase) getPermissionsForRole(role string) []string {
	if p, ok := rolePermissions[role]; ok {
		return p
	}
	return []string{}
}

// ── Helpers ───────────────────────────────────────────────────────────

func (uc *UseCase) recordLoginAttempt(ctx context.Context, userID *string, ip string, success bool, reason string) {
	attempt := &domain.LoginAttempt{
		UserID:  userID,
		IP:      &ip,
		Success: success,
	}
	if reason != "" {
		attempt.FailureReason = &reason
	}
	_ = uc.repo.CreateLoginAttempt(ctx, attempt)
}

// recordAudit mencatat aksi berhasil atas satu pengguna tanpa perubahan data.
func (uc *UseCase) recordAudit(ctx context.Context, aktorID, aksi, penggunaID string) {
	uc.writeAudit(ctx, audit.Entry{AktorID: aktorID, Aksi: aksi, Entitas: "pengguna", EntitasID: penggunaID, Hasil: audit.HasilBerhasil})
}

// recordFailedLogin mencatat percobaan masuk yang gagal. aktorID kosong bila email tak dikenal.
func (uc *UseCase) recordFailedLogin(ctx context.Context, aktorID, email, alasan string) {
	entitasID := aktorID
	if entitasID == "" {
		entitasID = email
	}
	uc.writeAudit(ctx, audit.Entry{
		AktorID: aktorID, Aksi: "LOGIN", Entitas: "pengguna", EntitasID: entitasID,
		Hasil: audit.HasilGagal, Metadata: map[string]any{"alasan": alasan, "email": email},
	})
}

// writeAudit mencatat entri audit. Kegagalan mencatat tidak menggagalkan aksinya,
// tetapi dilaporkan di log supaya tidak senyap.
func (uc *UseCase) writeAudit(ctx context.Context, entry audit.Entry) {
	if err := uc.audit.Write(ctx, entry); err != nil {
		uc.log.Warn("auth: gagal mencatat audit", map[string]interface{}{"aksi": entry.Aksi, "error": err.Error()})
	}
}

// userSnapshot adalah keadaan pengguna yang aman disimpan di audit (tanpa rahasia).
func userSnapshot(u *domain.Pengguna) map[string]any {
	return map[string]any{
		"nama": u.Nama, "email": u.Email, "peran": u.Peran, "lapis": u.Lapis,
		"lembaga": u.Lembaga, "kabupaten_kode": u.KabupatenKode, "status": u.Status,
	}
}

// issueSession menerbitkan access token (membawa penanda wajib ganti kata sandi
// bila aktif, dan sid = familyID) dan refresh token baru dalam keluarga familyID.
func (uc *UseCase) issueSession(ctx context.Context, user *domain.Pengguna, familyID, ip string) (accessToken, refreshToken string, err error) {
	minted, err := uc.mintSession(ctx, user, familyID, ip, time.Time{}, time.Now())
	if err != nil {
		return "", "", err
	}
	if err := uc.repo.CreateRefreshToken(ctx, minted.row); err != nil {
		return "", "", fmt.Errorf("save refresh token: %w", err)
	}
	return minted.accessToken, minted.refreshToken, nil
}

// setPassword menyimpan kata sandi baru. wajibGanti true untuk kata sandi yang
// ditetapkan pihak lain (reset admin): pemilik akun harus menggantinya sendiri.
func (uc *UseCase) setPassword(ctx context.Context, user *domain.Pengguna, password string, wajibGanti bool) error {
	hash, err := crypto.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	now := time.Now()
	user.PasswordHash = &hash
	user.WajibGantiPassword = wajibGanti
	user.PasswordDiubahPada = &now
	if err := uc.repo.UpdateUser(ctx, user); err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	return nil
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
