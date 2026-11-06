package usecase

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/pkg/sso"
	"github.com/garda-inflasi/api/pkg/token"
	"gorm.io/gorm"
)

// ── Galat ─────────────────────────────────────────────────────────────

var (
	ErrSSODisabled         = errors.New("masuk dengan SSO belum dikonfigurasi")
	ErrSSOUnavailable      = errors.New("penyedia SSO tidak dapat dihubungi")
	ErrSSOStateInvalid     = errors.New("sesi masuk SSO tidak valid atau sudah kedaluwarsa")
	ErrSSOExchangeFailed   = errors.New("verifikasi identitas SSO gagal")
	ErrSSOEmailUnverified  = errors.New("email akun SSO belum terverifikasi")
	ErrSSODomainNotAllowed = errors.New("domain akun SSO tidak diizinkan")
	ErrSSOAccountNotFound  = errors.New("tidak ada akun untuk identitas SSO ini")
	ErrSSOAccountConflict  = errors.New("akun sudah tertaut ke identitas SSO yang berbeda")
)

// MetodeSSOGoogle dicatat di audit LOGIN (metadata "metode") untuk masuk lewat Google.
const MetodeSSOGoogle = "sso_google"

const (
	// DefaultSSOStateTTL adalah umur satu alur SSO sejak URL otorisasi dibuat.
	DefaultSSOStateTTL = 10 * time.Minute

	// Batas panjang masukan callback; nilai wajar jauh di bawahnya.
	maxSSOStateLen = 128
	maxSSOCodeLen  = 2048
)

// ── Metode masuk di audit LOGIN ───────────────────────────────────────

type loginMethodKey struct{}

// withLoginMethod menandai context supaya audit LOGIN yang ditulis completeLogin memuat metode masuk.
func withLoginMethod(ctx context.Context, metode string) context.Context {
	return context.WithValue(ctx, loginMethodKey{}, metode)
}

// withLoginMethodMetadata menambahkan "metode" ke metadata audit LOGIN bila context menandainya.
func withLoginMethodMetadata(ctx context.Context, metadata map[string]any) map[string]any {
	metode, _ := ctx.Value(loginMethodKey{}).(string)
	if metode == "" {
		return metadata
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["metode"] = metode
	return metadata
}

// ── SSO ───────────────────────────────────────────────────────────────

// SSOProvider adalah penyedia identitas OIDC yang dipakai usecase (diimplementasikan pkg/sso.Provider).
type SSOProvider interface {
	AuthCodeURL(ctx context.Context, state, nonce, verifier string) (string, error)
	Exchange(ctx context.Context, code, verifier, nonce string) (*sso.Identity, error)
}

// SSOConfig menyusun SSOUseCase. Provider nil berarti SSO nonaktif.
type SSOConfig struct {
	Provider SSOProvider
	// Name adalah nilai pengguna.sso_provider; bawaan domain.ProviderGoogle.
	Name string
	// StateTTL adalah umur alur; bawaan DefaultSSOStateTTL.
	StateTTL time.Duration
	// Now menggantikan time.Now (tes).
	Now func() time.Time
}

// SSOUseCase adalah masuk lewat OIDC yang dipetakan ke akun yang sudah ada. Ia memakai UseCase untuk
// pemeriksaan kunci akun, tantangan MFA, dan penerbitan sesi (completeLogin) sehingga aturannya sama
// dengan masuk kata sandi.
type SSOUseCase struct {
	uc       *UseCase
	repo     domain.SSORepository
	provider SSOProvider
	name     string
	ttl      time.Duration
	now      func() time.Time
}

// NewSSO membuat SSOUseCase di atas UseCase auth.
func NewSSO(uc *UseCase, repo domain.SSORepository, cfg SSOConfig) *SSOUseCase {
	s := &SSOUseCase{uc: uc, repo: repo, provider: cfg.Provider, name: cfg.Name, ttl: cfg.StateTTL, now: cfg.Now}
	if s.name == "" {
		s.name = domain.ProviderGoogle
	}
	if s.ttl <= 0 {
		s.ttl = DefaultSSOStateTTL
	}
	if s.now == nil {
		s.now = time.Now
	}
	return s
}

// Enabled melaporkan apakah SSO dikonfigurasi.
func (s *SSOUseCase) Enabled() bool { return s.provider != nil }

// SSOAuthorization adalah hasil Authorize: URL tujuan pengalihan browser dan binding yang harus
// disimpan klien (cookie httpOnly) lalu dikirim kembali di callback.
type SSOAuthorization struct {
	URL     string
	Binding string
}

// Authorize memulai alur Authorization Code + PKCE. State, nonce, dan code_verifier dibangkitkan di sini,
// disimpan di server (state dan binding sebagai hash), dan hanya state yang ikut ke penyedia. Binding
// mengikat alur ke browser pemulai: tanpanya penyerang dapat memaksa korban memakai alurnya (login CSRF).
func (s *SSOUseCase) Authorize(ctx context.Context) (*SSOAuthorization, error) {
	if !s.Enabled() {
		return nil, ErrSSODisabled
	}
	state, err := sso.RandomToken(32)
	if err != nil {
		return nil, err
	}
	nonce, err := sso.RandomToken(32)
	if err != nil {
		return nil, err
	}
	binding, err := sso.RandomToken(32)
	if err != nil {
		return nil, err
	}
	verifier := sso.NewCodeVerifier()

	// Penemuan lebih dulu: bila penyedia tidak terjangkau, tidak ada baris state yang tertulis.
	authURL, err := s.provider.AuthCodeURL(ctx, state, nonce, verifier)
	if err != nil {
		if errors.Is(err, sso.ErrUnavailable) {
			s.uc.log.Warn("sso: penyedia tidak terjangkau saat authorize", map[string]interface{}{"error": err.Error()})
			return nil, ErrSSOUnavailable
		}
		return nil, fmt.Errorf("sso authorize: %w", err)
	}

	err = s.repo.SaveLoginState(ctx, &domain.SSOLoginState{
		StateHash:    sha256Hex(state),
		BindingHash:  sha256Hex(binding),
		Provider:     s.name,
		Nonce:        nonce,
		CodeVerifier: verifier,
		ExpiresAt:    s.now().Add(s.ttl),
	})
	if err != nil {
		return nil, fmt.Errorf("sso authorize: %w", err)
	}
	return &SSOAuthorization{URL: authURL, Binding: binding}, nil
}

// SSOCallbackInput adalah masukan callback: kode dan state dari penyedia, binding dari cookie browser.
type SSOCallbackInput struct {
	Code, State, Binding, IP string
}

// Callback menyelesaikan alur: memakai state (sekali pakai), menukar kode, memverifikasi ID token, memetakan
// ke akun yang sudah ada, menautkannya pada masuk pertama, lalu mengikuti jalur yang sama dengan masuk
// kata sandi (tantangan MFA bila MFA aktif, selain itu sesi lewat completeLogin).
func (s *SSOUseCase) Callback(ctx context.Context, in SSOCallbackInput) (*LoginResult, error) {
	if !s.Enabled() {
		return nil, ErrSSODisabled
	}
	flow, err := s.consumeState(ctx, in)
	if err != nil {
		return nil, err
	}

	identity, err := s.provider.Exchange(ctx, in.Code, flow.CodeVerifier, flow.Nonce)
	if err != nil {
		return nil, s.exchangeError(ctx, err)
	}

	if !identity.EmailVerified || identity.Email == "" {
		s.auditLoginGagal(ctx, "", identity, "sso_email_belum_terverifikasi", audit.HasilGagal)
		return nil, ErrSSOEmailUnverified
	}

	user, err := s.findAccount(ctx, identity)
	if err != nil {
		return nil, err
	}

	if user.Status != "AKTIF" {
		s.uc.log.Warn("sso: akun tidak aktif", map[string]interface{}{"user_id": user.ID, "status": user.Status})
		s.uc.recordLoginAttempt(ctx, &user.ID, in.IP, false, "account_inactive")
		s.auditLoginGagal(ctx, user.ID, identity, "account_inactive", audit.HasilGagal)
		return nil, ErrAccountInactive
	}
	if err := s.uc.periksaKunci(ctx, user.Email); err != nil {
		if errors.Is(err, ErrAccountLocked) {
			s.auditLoginGagal(ctx, user.ID, identity, "account_locked", audit.HasilGagal)
		}
		return nil, err
	}

	if user.SSOSubject == nil {
		if user, err = s.link(ctx, user, identity); err != nil {
			return nil, err
		}
	}

	if user.MfaAktif {
		// Metode login ikut dalam token tantangan supaya audit LOGIN sesudah OTP memuat metode dan faktor keduanya.
		challenge, err := s.uc.token.CreateMFAChallengeToken(user.ID, token.WithLoginMethod(MetodeSSOGoogle))
		if err != nil {
			return nil, fmt.Errorf("create mfa challenge: %w", err)
		}
		s.uc.log.Info("sso: identitas terbukti, menunggu kode MFA", map[string]interface{}{"user_id": user.ID, "ip": in.IP})
		s.uc.writeAudit(ctx, audit.Entry{
			AktorID: user.ID, Aksi: "SSO_LOGIN", Entitas: "pengguna", EntitasID: user.ID, Hasil: audit.HasilBerhasil,
			Metadata: map[string]any{"metode": MetodeSSOGoogle, "tahap": "menunggu_mfa"},
		})
		return &LoginResult{User: user, MFARequired: true, MFAToken: challenge}, nil
	}
	return s.uc.completeLogin(withLoginMethod(ctx, MetodeSSOGoogle), user, in.IP, "")
}

// consumeState memakai state sekali pakai dan memastikannya milik browser ini (binding) dan belum kedaluwarsa.
// State dihapus pada percobaan pertama apa pun hasilnya, jadi tidak ada percobaan kedua.
func (s *SSOUseCase) consumeState(ctx context.Context, in SSOCallbackInput) (*domain.SSOLoginState, error) {
	if in.Code == "" || in.State == "" || in.Binding == "" ||
		len(in.Code) > maxSSOCodeLen || len(in.State) > maxSSOStateLen || len(in.Binding) > maxSSOStateLen {
		s.auditStateGagal(ctx, "parameter_tidak_lengkap", audit.HasilGagal)
		return nil, ErrSSOStateInvalid
	}
	flow, err := s.repo.ConsumeLoginState(ctx, sha256Hex(in.State))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.auditStateGagal(ctx, "state_tidak_dikenal", audit.HasilDitolak)
			return nil, ErrSSOStateInvalid
		}
		return nil, fmt.Errorf("sso callback: %w", err)
	}
	if subtle.ConstantTimeCompare([]byte(sha256Hex(in.Binding)), []byte(flow.BindingHash)) != 1 {
		// State milik alur lain: kemungkinan login CSRF. Barisnya sudah terhapus.
		s.auditStateGagal(ctx, "binding_tidak_cocok", audit.HasilDitolak)
		return nil, ErrSSOStateInvalid
	}
	if flow.Provider != s.name || !flow.ExpiresAt.After(s.now()) {
		s.auditStateGagal(ctx, "state_kedaluwarsa", audit.HasilGagal)
		return nil, ErrSSOStateInvalid
	}
	return flow, nil
}

// exchangeError memetakan galat pkg/sso ke galat usecase dan mencatatnya.
func (s *SSOUseCase) exchangeError(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, sso.ErrUnavailable):
		s.uc.log.Error("sso: penyedia tidak dapat dipakai saat penukaran kode", map[string]interface{}{"error": err.Error()})
		s.auditStateGagal(ctx, "sso_penyedia_tidak_tersedia", audit.HasilGagal)
		return ErrSSOUnavailable
	case errors.Is(err, sso.ErrHostedDomain):
		s.auditStateGagal(ctx, "sso_domain_ditolak", audit.HasilDitolak)
		return ErrSSODomainNotAllowed
	case errors.Is(err, sso.ErrNonceMismatch):
		s.auditStateGagal(ctx, "sso_nonce_tidak_cocok", audit.HasilDitolak)
	default:
		s.uc.log.Warn("sso: penukaran kode atau verifikasi ID token ditolak", map[string]interface{}{"error": err.Error()})
		s.auditStateGagal(ctx, "sso_token_ditolak", audit.HasilDitolak)
	}
	return ErrSSOExchangeFailed
}

// findAccount memetakan identitas ke akun yang sudah ada: lebih dulu menurut subjek (stabil walau email
// berubah), baru menurut email. Tidak pernah membuat akun. Email milik akun yang sudah bertaut ke subjek
// lain ditolak dan diaudit.
func (s *SSOUseCase) findAccount(ctx context.Context, identity *sso.Identity) (*domain.Pengguna, error) {
	user, err := s.repo.GetUserBySSO(ctx, s.name, identity.Subject)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("sso: cari akun menurut subjek: %w", err)
	}

	user, err = s.uc.repo.GetUserByEmail(ctx, identity.Email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.uc.log.Warn("sso: tidak ada akun untuk email", map[string]interface{}{"email": identity.Email})
			s.auditLoginGagal(ctx, "", identity, "sso_akun_tidak_ditemukan", audit.HasilGagal)
			return nil, ErrSSOAccountNotFound
		}
		return nil, fmt.Errorf("sso: cari akun menurut email: %w", err)
	}
	if user.SSOSubject != nil {
		s.uc.log.Warn("sso: email sudah tertaut ke subjek lain", map[string]interface{}{"user_id": user.ID})
		s.auditLoginGagal(ctx, user.ID, identity, "sso_subjek_berbeda", audit.HasilDitolak)
		return nil, ErrSSOAccountConflict
	}
	return user, nil
}

// link menautkan identitas ke akun yang belum bertaut pada masuk SSO pertama. Penautan atomik
// (WHERE sso_subject IS NULL): bila permintaan serentak lebih dulu menaut, hasilnya dinilai ulang.
func (s *SSOUseCase) link(ctx context.Context, user *domain.Pengguna, identity *sso.Identity) (*domain.Pengguna, error) {
	now := s.now()
	linked, err := s.repo.LinkSSO(ctx, user.ID, s.name, identity.Subject, now)
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		// Identitas ini sudah dipakai akun lain (penautan serentak).
		s.auditLoginGagal(ctx, user.ID, identity, "sso_subjek_dipakai_akun_lain", audit.HasilDitolak)
		return nil, ErrSSOAccountConflict
	}
	if err != nil {
		return nil, fmt.Errorf("sso: tautkan akun: %w", err)
	}
	if !linked {
		fresh, err := s.uc.repo.GetUserByID(ctx, user.ID)
		if err != nil {
			return nil, fmt.Errorf("sso: muat ulang akun: %w", err)
		}
		if fresh.SSOProvider == nil || fresh.SSOSubject == nil || *fresh.SSOProvider != s.name || *fresh.SSOSubject != identity.Subject {
			s.auditLoginGagal(ctx, user.ID, identity, "sso_subjek_berbeda", audit.HasilDitolak)
			return nil, ErrSSOAccountConflict
		}
		return fresh, nil
	}

	// LinkSSO sudah menulis tautannya di basis data; struct disamakan agar pengguna yang dikembalikan lengkap.
	provider, subject := s.name, identity.Subject
	user.SSOProvider, user.SSOSubject, user.SSOTerhubungPada = &provider, &subject, &now
	s.uc.writeAudit(ctx, audit.Entry{
		AktorID: user.ID, Aksi: "SSO_LINK", Entitas: "pengguna", EntitasID: user.ID, Hasil: audit.HasilBerhasil,
		Metadata: map[string]any{"provider": s.name, "sso_subject": identity.Subject, "email": identity.Email},
	})
	s.uc.log.Info("sso: akun ditautkan", map[string]interface{}{"user_id": user.ID, "provider": s.name})
	return user, nil
}

// auditLoginGagal mencatat LOGIN yang gagal atau ditolak beserta metode dan alasannya.
func (s *SSOUseCase) auditLoginGagal(ctx context.Context, aktorID string, identity *sso.Identity, alasan, hasil string) {
	entitasID := aktorID
	if entitasID == "" {
		entitasID = identity.Email
	}
	s.uc.writeAudit(ctx, audit.Entry{
		AktorID: aktorID, Aksi: "LOGIN", Entitas: "pengguna", EntitasID: entitasID, Hasil: hasil,
		Metadata: map[string]any{"metode": MetodeSSOGoogle, "alasan": alasan, "email": identity.Email, "sso_subject": identity.Subject},
	})
}

// auditStateGagal mencatat kegagalan sebelum identitas diketahui (state, penukaran kode, token).
func (s *SSOUseCase) auditStateGagal(ctx context.Context, alasan, hasil string) {
	s.uc.writeAudit(ctx, audit.Entry{
		Aksi: "LOGIN", Entitas: "pengguna", EntitasID: MetodeSSOGoogle, Hasil: hasil,
		Metadata: map[string]any{"metode": MetodeSSOGoogle, "alasan": alasan},
	})
}
