package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/audit/audittest"
	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/pkg/email"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/sso"
	"github.com/garda-inflasi/api/pkg/token"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ── Penyimpanan dalam memori ──────────────────────────────────────────────
//
// memRepo menggantikan basis data untuk seluruh jalur SSO: pengguna (dengan aturan UNIQUE(sso_provider,
// sso_subject) dan UpdateUser yang menimpa seluruh baris seperti GORM Save), sesi, percobaan masuk, dan
// state SSO. Metode lain milik mockRepository dibiarkan: bila terpanggil tanpa diharapkan, tes gagal.

type memRepo struct {
	*mockRepository
	mu       sync.Mutex
	users    map[string]*domain.Pengguna
	states   map[string]domain.SSOLoginState
	attempts []domain.LoginAttempt
	refresh  []domain.RefreshToken

	saveStateErr error
	// Mengatur hasil LinkSSO bila tidak nil (simulasi balapan).
	linkOverride func(r *memRepo, userID string) (bool, error)
}

func newMemRepo(users ...*domain.Pengguna) *memRepo {
	r := &memRepo{mockRepository: new(mockRepository), users: map[string]*domain.Pengguna{}, states: map[string]domain.SSOLoginState{}}
	for _, u := range users {
		c := *u
		r.users[u.ID] = &c
	}
	return r
}

func (r *memRepo) clone(u *domain.Pengguna) *domain.Pengguna { c := *u; return &c }

func (r *memRepo) GetUserByID(_ context.Context, id string) (*domain.Pengguna, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if u, ok := r.users[id]; ok {
		return r.clone(u), nil
	}
	return nil, fmt.Errorf("get user by id: %w", gorm.ErrRecordNotFound)
}

func (r *memRepo) GetUserByEmail(_ context.Context, mail string) (*domain.Pengguna, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range r.users {
		if strings.EqualFold(u.Email, mail) {
			return r.clone(u), nil
		}
	}
	return nil, fmt.Errorf("get user by email: %w", gorm.ErrRecordNotFound)
}

func (r *memRepo) UpdateUser(_ context.Context, u *domain.Pengguna) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.users[u.ID] = r.clone(u) // seperti GORM Save: seluruh kolom ditimpa
	return nil
}

func (r *memRepo) CreateRefreshToken(_ context.Context, t *domain.RefreshToken) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.refresh = append(r.refresh, *t)
	return nil
}

func (r *memRepo) CreateLoginAttempt(_ context.Context, a *domain.LoginAttempt) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.attempts = append(r.attempts, *a)
	return nil
}

func (r *memRepo) SaveLoginState(_ context.Context, s *domain.SSOLoginState) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.saveStateErr != nil {
		return r.saveStateErr
	}
	r.states[s.StateHash] = *s
	return nil
}

func (r *memRepo) ConsumeLoginState(_ context.Context, hash string) (*domain.SSOLoginState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.states[hash]
	if !ok {
		return nil, fmt.Errorf("pakai state SSO: %w", gorm.ErrRecordNotFound)
	}
	delete(r.states, hash)
	return &s, nil
}

func (r *memRepo) GetUserBySSO(_ context.Context, provider, subject string) (*domain.Pengguna, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range r.users {
		if u.SSOProvider != nil && *u.SSOProvider == provider && *u.SSOSubject == subject {
			return r.clone(u), nil
		}
	}
	return nil, fmt.Errorf("get user by sso: %w", gorm.ErrRecordNotFound)
}

func (r *memRepo) LinkSSO(_ context.Context, userID, provider, subject string, at time.Time) (bool, error) {
	if r.linkOverride != nil {
		return r.linkOverride(r, userID)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range r.users {
		if u.SSOProvider != nil && *u.SSOProvider == provider && *u.SSOSubject == subject {
			return false, gorm.ErrDuplicatedKey
		}
	}
	u := r.users[userID]
	if u == nil || u.SSOSubject != nil {
		return false, nil
	}
	u.SSOProvider, u.SSOSubject, u.SSOTerhubungPada = &provider, &subject, &at
	return true, nil
}

func (r *memRepo) stored(id string) *domain.Pengguna {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.users[id]
}

// ── Penyedia palsu ────────────────────────────────────────────────────────

type fakeProvider struct {
	authErr     error
	exchangeErr error
	identity    *sso.Identity

	authCalls     int
	exchangeCalls int
	gotState      string
	gotNonce      string
	gotVerifier   string
	exCode        string
	exVerifier    string
	exNonce       string
}

func (f *fakeProvider) AuthCodeURL(_ context.Context, state, nonce, verifier string) (string, error) {
	f.authCalls++
	f.gotState, f.gotNonce, f.gotVerifier = state, nonce, verifier
	if f.authErr != nil {
		return "", f.authErr
	}
	return "https://idp.example/authorize?state=" + state, nil
}

func (f *fakeProvider) Exchange(_ context.Context, code, verifier, nonce string) (*sso.Identity, error) {
	f.exchangeCalls++
	f.exCode, f.exVerifier, f.exNonce = code, verifier, nonce
	if f.exchangeErr != nil {
		return nil, f.exchangeErr
	}
	id := *f.identity
	return &id, nil
}

// ── Fixture ───────────────────────────────────────────────────────────────

type ssoFixture struct {
	t        *testing.T
	repo     *memRepo
	provider *fakeProvider
	uc       *UseCase
	sso      *SSOUseCase
	audit    *audittest.Recorder
	tokens   *token.Manager
	now      time.Time
}

func ssoUser(id, mail string) *domain.Pengguna {
	return &domain.Pengguna{ID: id, Nama: "Pengguna " + id, Email: mail, Peran: "OPS_MAHASISWA", Lapis: "OPERASIONAL", Status: "AKTIF"}
}

func newSSOFixture(t *testing.T, users ...*domain.Pengguna) *ssoFixture {
	t.Helper()
	f := &ssoFixture{
		t:        t,
		repo:     newMemRepo(users...),
		provider: &fakeProvider{identity: &sso.Identity{Subject: "sub-1", Email: "ani@kemenkeu.go.id", EmailVerified: true, Name: "Ani"}},
		audit:    &audittest.Recorder{},
		tokens:   token.NewManager("test-secret-key-32-chars!!"),
		now:      time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC),
	}
	cfg := DefaultConfig()
	cfg.MFAKey = testKeyring()
	f.uc = New(f.repo, f.tokens, cfg, logger.NewDefault(), email.New(email.Config{}), f.audit)
	f.sso = NewSSO(f.uc, f.repo, SSOConfig{Provider: f.provider, Now: func() time.Time { return f.now }})
	return f
}

// mulai menjalankan Authorize dan mengembalikan state serta binding yang dikirim klien pada callback.
func (f *ssoFixture) mulai() (state, binding string) {
	f.t.Helper()
	res, err := f.sso.Authorize(context.Background())
	require.NoError(f.t, err)
	return f.provider.gotState, res.Binding
}

func (f *ssoFixture) callback(state, binding string) (*LoginResult, error) {
	return f.sso.Callback(context.Background(), SSOCallbackInput{Code: "kode-1", State: state, Binding: binding, IP: "10.0.0.1"})
}

// masuk menjalankan alur penuh: Authorize lalu Callback.
func (f *ssoFixture) masuk() (*LoginResult, error) {
	state, binding := f.mulai()
	return f.callback(state, binding)
}

func (f *ssoFixture) lastAudit() audit.Entry {
	e, ok := f.audit.Last()
	require.True(f.t, ok, "tidak ada entri audit")
	return e
}

func (f *ssoFixture) auditAksi(aksi string) []audit.Entry {
	var out []audit.Entry
	for _, e := range f.audit.Entries() {
		if e.Aksi == aksi {
			out = append(out, e)
		}
	}
	return out
}

func alasanAudit(e audit.Entry) any { return e.Metadata["alasan"] }

// ── Authorize ─────────────────────────────────────────────────────────────

func TestSSOAuthorize_MenyimpanStateSebagaiHashDanTidakMembocorkanRahasia(t *testing.T) {
	f := newSSOFixture(t)

	res, err := f.sso.Authorize(context.Background())
	require.NoError(t, err)

	state, nonce, verifier := f.provider.gotState, f.provider.gotNonce, f.provider.gotVerifier
	assert.Equal(t, "https://idp.example/authorize?state="+state, res.URL)
	assert.NotEmpty(t, res.Binding)
	assert.GreaterOrEqual(t, len(state), 43, "state minimal 32 byte acak")
	assert.GreaterOrEqual(t, len(nonce), 43)
	assert.GreaterOrEqual(t, len(verifier), 43, "verifier PKCE minimal 43 karakter")
	assert.NotEqual(t, state, res.Binding, "binding dan state adalah rahasia yang berbeda")
	assert.NotEqual(t, state, nonce)

	require.Len(t, f.repo.states, 1)
	row, ok := f.repo.states[sha256Hex(state)]
	require.True(t, ok, "baris disimpan dengan kunci hash state, bukan state polos")
	assert.Equal(t, sha256Hex(res.Binding), row.BindingHash)
	assert.Equal(t, nonce, row.Nonce)
	assert.Equal(t, verifier, row.CodeVerifier)
	assert.Equal(t, domain.ProviderGoogle, row.Provider)
	assert.Equal(t, f.now.Add(10*time.Minute), row.ExpiresAt, "TTL bawaan 10 menit")
	assert.NotContains(t, []string{row.StateHash, row.BindingHash}, state)
	assert.NotContains(t, []string{row.StateHash, row.BindingHash}, res.Binding)
}

func TestSSOAuthorize_SetiapAlurPunyaRahasiaBaru(t *testing.T) {
	f := newSSOFixture(t)
	seen := map[string]bool{}
	for i := 0; i < 20; i++ {
		res, err := f.sso.Authorize(context.Background())
		require.NoError(t, err)
		for _, v := range []string{f.provider.gotState, f.provider.gotNonce, f.provider.gotVerifier, res.Binding} {
			assert.False(t, seen[v], "nilai acak tidak boleh berulang antar alur")
			seen[v] = true
		}
	}
	assert.Len(t, f.repo.states, 20)
}

func TestSSOAuthorize_Nonaktif(t *testing.T) {
	f := newSSOFixture(t)
	disabled := NewSSO(f.uc, f.repo, SSOConfig{})
	assert.False(t, disabled.Enabled())

	_, err := disabled.Authorize(context.Background())
	assert.ErrorIs(t, err, ErrSSODisabled)
	_, err = disabled.Callback(context.Background(), SSOCallbackInput{Code: "c", State: "s", Binding: "b"})
	assert.ErrorIs(t, err, ErrSSODisabled)
	assert.Empty(t, f.repo.states)
}

func TestSSOAuthorize_PenyediaTidakTerjangkauTidakMenulisState(t *testing.T) {
	f := newSSOFixture(t)
	f.provider.authErr = fmt.Errorf("%w: dial tcp: connection refused", sso.ErrUnavailable)

	_, err := f.sso.Authorize(context.Background())
	assert.ErrorIs(t, err, ErrSSOUnavailable)
	assert.Empty(t, f.repo.states)
}

func TestSSOAuthorize_GalatLainDanGalatPenyimpanan(t *testing.T) {
	f := newSSOFixture(t)
	f.provider.authErr = errors.New("rusak")
	_, err := f.sso.Authorize(context.Background())
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrSSOUnavailable)

	f.provider.authErr = nil
	f.repo.saveStateErr = errors.New("disk penuh")
	_, err = f.sso.Authorize(context.Background())
	require.Error(t, err)
}

// ── Callback: state, binding, TTL ─────────────────────────────────────────

func TestSSOCallback_StateHanyaSekaliPakai(t *testing.T) {
	f := newSSOFixture(t, ssoUser("u1", "ani@kemenkeu.go.id"))
	state, binding := f.mulai()

	_, err := f.callback(state, binding)
	require.NoError(t, err)

	// Pengulangan callback yang sama (replay): state sudah terhapus.
	_, err = f.callback(state, binding)
	assert.ErrorIs(t, err, ErrSSOStateInvalid)
	assert.Equal(t, 1, f.provider.exchangeCalls, "kode tidak ditukar dua kali")
	assert.Equal(t, "state_tidak_dikenal", alasanAudit(f.lastAudit()))
	assert.Equal(t, audit.HasilDitolak, f.lastAudit().Hasil)
}

func TestSSOCallback_BindingSalahMenolakDanMenghanguskanState(t *testing.T) {
	f := newSSOFixture(t, ssoUser("u1", "ani@kemenkeu.go.id"))
	state, binding := f.mulai()

	// Login CSRF: korban membawa state penyerang tetapi cookie binding-nya sendiri (atau tanpa cookie).
	_, err := f.callback(state, "binding-milik-browser-lain")
	assert.ErrorIs(t, err, ErrSSOStateInvalid)
	assert.Equal(t, 0, f.provider.exchangeCalls, "kode tidak boleh ditukar tanpa binding yang cocok")
	assert.Equal(t, "binding_tidak_cocok", alasanAudit(f.lastAudit()))
	assert.Equal(t, audit.HasilDitolak, f.lastAudit().Hasil)

	// Percobaan dengan binding benar pun gagal: state sudah hangus pada percobaan pertama.
	_, err = f.callback(state, binding)
	assert.ErrorIs(t, err, ErrSSOStateInvalid)
	assert.Equal(t, 0, f.provider.exchangeCalls)
	assert.Nil(t, f.repo.stored("u1").SSOSubject, "akun tidak boleh tertaut")
}

func TestSSOCallback_StateKedaluwarsa(t *testing.T) {
	f := newSSOFixture(t, ssoUser("u1", "ani@kemenkeu.go.id"))
	state, binding := f.mulai()

	f.now = f.now.Add(10*time.Minute + time.Second)
	_, err := f.callback(state, binding)
	assert.ErrorIs(t, err, ErrSSOStateInvalid)
	assert.Equal(t, 0, f.provider.exchangeCalls)
	assert.Equal(t, "state_kedaluwarsa", alasanAudit(f.lastAudit()))
}

func TestSSOCallback_StateMasihBerlakuTepatSebelumBatas(t *testing.T) {
	f := newSSOFixture(t, ssoUser("u1", "ani@kemenkeu.go.id"))
	state, binding := f.mulai()
	f.now = f.now.Add(10*time.Minute - time.Second)
	_, err := f.callback(state, binding)
	assert.NoError(t, err)
}

func TestSSOCallback_StateProviderBerbeda(t *testing.T) {
	f := newSSOFixture(t)
	state, binding := f.mulai()
	row := f.repo.states[sha256Hex(state)]
	row.Provider = "lain"
	f.repo.states[sha256Hex(state)] = row

	_, err := f.callback(state, binding)
	assert.ErrorIs(t, err, ErrSSOStateInvalid)
}

func TestSSOCallback_ParameterKosongAtauTerlaluPanjang(t *testing.T) {
	f := newSSOFixture(t)
	cases := map[string]SSOCallbackInput{
		"tanpa kode":    {State: "s", Binding: "b"},
		"tanpa state":   {Code: "c", Binding: "b"},
		"tanpa binding": {Code: "c", State: "s"},
		"kode panjang":  {Code: strings.Repeat("c", maxSSOCodeLen+1), State: "s", Binding: "b"},
		"state panjang": {Code: "c", State: strings.Repeat("s", maxSSOStateLen+1), Binding: "b"},
	}
	for name, in := range cases {
		_, err := f.sso.Callback(context.Background(), in)
		assert.ErrorIs(t, err, ErrSSOStateInvalid, name)
	}
	assert.Equal(t, 0, f.provider.exchangeCalls)
}

func TestSSOCallback_PenukaranMemakaiVerifierDanNonceTersimpanBukanDariRequest(t *testing.T) {
	f := newSSOFixture(t, ssoUser("u1", "ani@kemenkeu.go.id"))
	state, binding := f.mulai()
	verifier, nonce := f.provider.gotVerifier, f.provider.gotNonce

	_, err := f.callback(state, binding)
	require.NoError(t, err)
	assert.Equal(t, "kode-1", f.provider.exCode)
	assert.Equal(t, verifier, f.provider.exVerifier, "code_verifier PKCE berasal dari penyimpanan server")
	assert.Equal(t, nonce, f.provider.exNonce, "nonce berasal dari penyimpanan server")
}

// ── Callback: galat penukaran kode ────────────────────────────────────────

func TestSSOCallback_PemetaanGalatPenukaran(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		want       error
		wantHasil  string
		wantAlasan string
	}{
		{"penyedia tidak tersedia", fmt.Errorf("%w: timeout", sso.ErrUnavailable), ErrSSOUnavailable, audit.HasilGagal, "sso_penyedia_tidak_tersedia"},
		{"domain ditolak", sso.ErrHostedDomain, ErrSSODomainNotAllowed, audit.HasilDitolak, "sso_domain_ditolak"},
		{"nonce berbeda", sso.ErrNonceMismatch, ErrSSOExchangeFailed, audit.HasilDitolak, "sso_nonce_tidak_cocok"},
		{"token tidak valid", fmt.Errorf("%w: tanda tangan", sso.ErrInvalidToken), ErrSSOExchangeFailed, audit.HasilDitolak, "sso_token_ditolak"},
		{"kode ditolak", fmt.Errorf("%w: invalid_grant", sso.ErrCodeRejected), ErrSSOExchangeFailed, audit.HasilDitolak, "sso_token_ditolak"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newSSOFixture(t, ssoUser("u1", "ani@kemenkeu.go.id"))
			f.provider.exchangeErr = tc.err

			res, err := f.masuk()
			assert.ErrorIs(t, err, tc.want)
			assert.Nil(t, res)
			assert.Equal(t, tc.wantHasil, f.lastAudit().Hasil)
			assert.Equal(t, tc.wantAlasan, alasanAudit(f.lastAudit()))
			assert.Equal(t, MetodeSSOGoogle, f.lastAudit().Metadata["metode"])
			assert.Nil(t, f.repo.stored("u1").SSOSubject, "tidak ada penautan pada galat")
			assert.Empty(t, f.repo.refresh, "tidak ada sesi")
		})
	}
}

// ── Callback: pemetaan akun ───────────────────────────────────────────────

func TestSSOCallback_MasukPertamaMenautkanAkunDanMenerbitkanSesi(t *testing.T) {
	f := newSSOFixture(t, ssoUser("u1", "ani@kemenkeu.go.id"))

	res, err := f.masuk()
	require.NoError(t, err)
	assert.Equal(t, "u1", res.User.ID)
	assert.NotEmpty(t, res.AccessToken)
	assert.NotEmpty(t, res.RefreshToken)
	assert.False(t, res.MFARequired)

	claims, err := f.tokens.VerifyToken(res.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, "u1", claims.UserID)
	assert.Equal(t, "OPS_MAHASISWA", claims.Role)

	stored := f.repo.stored("u1")
	require.NotNil(t, stored.SSOSubject, "tautan harus bertahan setelah UpdateUser milik completeLogin")
	assert.Equal(t, "sub-1", *stored.SSOSubject)
	assert.Equal(t, domain.ProviderGoogle, *stored.SSOProvider)
	require.NotNil(t, stored.SSOTerhubungPada)
	assert.Equal(t, f.now, *stored.SSOTerhubungPada)
	require.NotNil(t, stored.TerakhirMasuk)

	require.Len(t, f.repo.refresh, 1)
	require.Len(t, f.repo.attempts, 1)
	assert.True(t, f.repo.attempts[0].Success)

	link := f.auditAksi("SSO_LINK")
	require.Len(t, link, 1)
	assert.Equal(t, "u1", link[0].AktorID)
	assert.Equal(t, audit.HasilBerhasil, link[0].Hasil)
	assert.Equal(t, "sub-1", link[0].Metadata["sso_subject"])

	login := f.auditAksi("LOGIN")
	require.Len(t, login, 1, "tepat satu LOGIN")
	assert.Equal(t, audit.HasilBerhasil, login[0].Hasil)
	assert.Equal(t, MetodeSSOGoogle, login[0].Metadata["metode"], "audit LOGIN mencatat metode sso_google")
}

func TestSSOCallback_MasukBerikutnyaMencocokkanSubjekBukanEmail(t *testing.T) {
	f := newSSOFixture(t, ssoUser("u1", "ani@kemenkeu.go.id"))
	_, err := f.masuk()
	require.NoError(t, err)

	// Email di penyedia berubah, subjek tetap: tetap akun yang sama dan tidak ditautkan ulang.
	f.provider.identity.Email = "ani.baru@kemenkeu.go.id"
	res, err := f.masuk()
	require.NoError(t, err)
	assert.Equal(t, "u1", res.User.ID)
	assert.Len(t, f.auditAksi("SSO_LINK"), 1, "penautan hanya pada masuk pertama")
	assert.Equal(t, "ani@kemenkeu.go.id", f.repo.stored("u1").Email, "email akun tidak berubah")
}

func TestSSOCallback_EmailTakPekaHurufBesarKecil(t *testing.T) {
	f := newSSOFixture(t, ssoUser("u1", "ani@kemenkeu.go.id"))
	f.provider.identity.Email = "Ani@Kemenkeu.GO.id"
	res, err := f.masuk()
	require.NoError(t, err)
	assert.Equal(t, "u1", res.User.ID)
}

func TestSSOCallback_EmailTakDikenalDitolakTanpaMembuatAkun(t *testing.T) {
	f := newSSOFixture(t, ssoUser("u1", "lain@kemenkeu.go.id"))

	res, err := f.masuk()
	assert.ErrorIs(t, err, ErrSSOAccountNotFound)
	assert.Nil(t, res)
	assert.Len(t, f.repo.users, 1, "tidak ada pendaftaran mandiri")
	assert.Empty(t, f.repo.refresh)

	e := f.lastAudit()
	assert.Equal(t, "LOGIN", e.Aksi)
	assert.Equal(t, audit.HasilGagal, e.Hasil)
	assert.Equal(t, "sso_akun_tidak_ditemukan", alasanAudit(e))
	assert.Equal(t, "ani@kemenkeu.go.id", e.Metadata["email"])
	assert.Equal(t, MetodeSSOGoogle, e.Metadata["metode"])
	assert.Empty(t, e.AktorID)
}

func TestSSOCallback_EmailTerhubungKeSubjekLainDitolakDanDiaudit(t *testing.T) {
	user := ssoUser("u1", "ani@kemenkeu.go.id")
	asing, google := "sub-lain", domain.ProviderGoogle
	user.SSOProvider, user.SSOSubject = &google, &asing
	f := newSSOFixture(t, user)

	res, err := f.masuk()
	assert.ErrorIs(t, err, ErrSSOAccountConflict)
	assert.Nil(t, res)
	assert.Equal(t, "sub-lain", *f.repo.stored("u1").SSOSubject, "tautan lama tidak boleh ditimpa")
	assert.Empty(t, f.repo.refresh)
	assert.Empty(t, f.auditAksi("SSO_LINK"))

	e := f.lastAudit()
	assert.Equal(t, "LOGIN", e.Aksi)
	assert.Equal(t, audit.HasilDitolak, e.Hasil)
	assert.Equal(t, "u1", e.AktorID)
	assert.Equal(t, "sso_subjek_berbeda", alasanAudit(e))
	assert.Equal(t, "sub-1", e.Metadata["sso_subject"], "subjek yang dicoba dicatat")
}

func TestSSOCallback_EmailTakTerverifikasiDitolak(t *testing.T) {
	for name, mutate := range map[string]func(id *sso.Identity){
		"email_verified false": func(id *sso.Identity) { id.EmailVerified = false },
		"email kosong":         func(id *sso.Identity) { id.Email = "" },
	} {
		t.Run(name, func(t *testing.T) {
			f := newSSOFixture(t, ssoUser("u1", "ani@kemenkeu.go.id"))
			mutate(f.provider.identity)

			_, err := f.masuk()
			assert.ErrorIs(t, err, ErrSSOEmailUnverified)
			assert.Nil(t, f.repo.stored("u1").SSOSubject, "email tak terverifikasi tidak boleh menaut akun")
			assert.Empty(t, f.repo.refresh)
			assert.Equal(t, "sso_email_belum_terverifikasi", alasanAudit(f.lastAudit()))
		})
	}
}

func TestSSOCallback_AkunTidakAktifDitolakSepertiLoginKataSandi(t *testing.T) {
	for _, status := range []string{"NONAKTIF", "MENUNGGU"} {
		t.Run(status, func(t *testing.T) {
			u := ssoUser("u1", "ani@kemenkeu.go.id")
			u.Status = status
			f := newSSOFixture(t, u)

			res, err := f.masuk()
			assert.ErrorIs(t, err, ErrAccountInactive)
			assert.Nil(t, res)
			assert.Nil(t, f.repo.stored("u1").SSOSubject, "akun tidak aktif tidak ditautkan")
			assert.Empty(t, f.repo.refresh)
			require.Len(t, f.repo.attempts, 1)
			assert.False(t, f.repo.attempts[0].Success)
			assert.Equal(t, "account_inactive", *f.repo.attempts[0].FailureReason)
			assert.Equal(t, "account_inactive", alasanAudit(f.lastAudit()))
		})
	}
}

func TestSSOCallback_AkunBertautTidakAktifJugaDitolak(t *testing.T) {
	u := ssoUser("u1", "ani@kemenkeu.go.id")
	u.Status = "NONAKTIF"
	sub, google := "sub-1", domain.ProviderGoogle
	u.SSOProvider, u.SSOSubject = &google, &sub
	f := newSSOFixture(t, u)

	_, err := f.masuk()
	assert.ErrorIs(t, err, ErrAccountInactive)
	assert.Empty(t, f.repo.refresh)
}

func TestSSOCallback_AkunTerkunciDitolakTanpaMenaut(t *testing.T) {
	f := newSSOFixture(t, ssoUser("u1", "ani@kemenkeu.go.id"))
	f.repo.kunciTerkunci("ANI@kemenkeu.go.id ")

	res, err := f.masuk()
	assert.ErrorIs(t, err, ErrAccountLocked)
	assert.Nil(t, res)
	assert.Nil(t, f.repo.stored("u1").SSOSubject)
	assert.Empty(t, f.repo.refresh)
	assert.Equal(t, "account_locked", alasanAudit(f.lastAudit()))
}

func TestSSOCallback_AkunSSOSajaTanpaKataSandi(t *testing.T) {
	u := ssoUser("u1", "ani@kemenkeu.go.id")
	require.Nil(t, u.PasswordHash)
	f := newSSOFixture(t, u)
	res, err := f.masuk()
	require.NoError(t, err)
	assert.NotEmpty(t, res.AccessToken)
}

// ── Callback: MFA ─────────────────────────────────────────────────────────

func TestSSOCallback_AkunMFAAktifMenerimaTantanganBukanSesi(t *testing.T) {
	u := ssoUser("u1", "ani@kemenkeu.go.id")
	u.MfaAktif = true
	f := newSSOFixture(t, u)

	res, err := f.masuk()
	require.NoError(t, err)
	assert.True(t, res.MFARequired)
	assert.Empty(t, res.AccessToken, "belum ada sesi sebelum OTP")
	assert.Empty(t, res.RefreshToken)
	require.NotEmpty(t, res.MFAToken)

	userID, err := f.tokens.VerifyMFAChallengeToken(res.MFAToken)
	require.NoError(t, err, "token tantangan sama dengan alur kata sandi")
	assert.Equal(t, "u1", userID)
	_, err = f.tokens.VerifyToken(res.MFAToken)
	assert.Error(t, err, "token tantangan bukan access token")

	assert.Empty(t, f.repo.refresh)
	assert.Empty(t, f.repo.attempts, "belum ada login yang selesai")
	require.NotNil(t, f.repo.stored("u1").SSOSubject, "identitas sudah terbukti sehingga akun tertaut")

	assert.Empty(t, f.auditAksi("LOGIN"), "LOGIN dicatat setelah OTP")
	sso := f.auditAksi("SSO_LOGIN")
	require.Len(t, sso, 1)
	assert.Equal(t, MetodeSSOGoogle, sso[0].Metadata["metode"])
	assert.Equal(t, "menunggu_mfa", sso[0].Metadata["tahap"])
}

func TestSSOCallback_AkunWajibMFABelumSetupMenerimaKlaimSetup(t *testing.T) {
	u := ssoUser("u1", "ani@kemenkeu.go.id")
	u.Peran, u.Lapis = "STRATEGIS_FMIPA", "STRATEGIS"
	f := newSSOFixture(t, u)
	f.uc.cfg.MFAEnforce = true

	res, err := f.masuk()
	require.NoError(t, err)
	assert.True(t, res.MFASetupRequired)
	claims, err := f.tokens.VerifyToken(res.AccessToken)
	require.NoError(t, err)
	assert.True(t, claims.MFASetupRequired, "klaim sama dengan login kata sandi")
}

func TestSSOCallback_WajibGantiPasswordMembawaKlaim(t *testing.T) {
	u := ssoUser("u1", "ani@kemenkeu.go.id")
	u.WajibGantiPassword = true
	f := newSSOFixture(t, u)
	res, err := f.masuk()
	require.NoError(t, err)
	claims, err := f.tokens.VerifyToken(res.AccessToken)
	require.NoError(t, err)
	assert.True(t, claims.PasswordChangeRequired)
}

// ── Penautan: balapan dan keunikan ────────────────────────────────────────

func TestSSOCallback_SubjekSudahDipakaiAkunLain(t *testing.T) {
	pemilik := ssoUser("u-pemilik", "pemilik@kemenkeu.go.id")
	sub, google := "sub-1", domain.ProviderGoogle
	pemilik.SSOProvider, pemilik.SSOSubject = &google, &sub
	f := newSSOFixture(t, pemilik, ssoUser("u1", "ani@kemenkeu.go.id"))

	// Subjek sub-1 sudah milik u-pemilik: masuk sebagai u-pemilik, bukan u1 (subjek mengalahkan email).
	res, err := f.masuk()
	require.NoError(t, err)
	assert.Equal(t, "u-pemilik", res.User.ID)
	assert.Nil(t, f.repo.stored("u1").SSOSubject, "akun dengan email cocok tidak diambil alih")
}

func TestSSOCallback_PenautanSerentakDenganSubjekSama(t *testing.T) {
	f := newSSOFixture(t, ssoUser("u1", "ani@kemenkeu.go.id"))
	f.repo.linkOverride = func(r *memRepo, userID string) (bool, error) {
		// Permintaan lain menaut lebih dulu dengan subjek yang sama.
		sub, google, at := "sub-1", domain.ProviderGoogle, time.Now()
		r.mu.Lock()
		u := r.users[userID]
		u.SSOProvider, u.SSOSubject, u.SSOTerhubungPada = &google, &sub, &at
		r.mu.Unlock()
		return false, nil
	}
	res, err := f.masuk()
	require.NoError(t, err)
	assert.Equal(t, "u1", res.User.ID)
	assert.Empty(t, f.auditAksi("SSO_LINK"), "penautan dicatat oleh permintaan yang menang")
}

func TestSSOCallback_PenautanSerentakDenganSubjekLainDitolak(t *testing.T) {
	f := newSSOFixture(t, ssoUser("u1", "ani@kemenkeu.go.id"))
	f.repo.linkOverride = func(r *memRepo, userID string) (bool, error) {
		sub, google := "sub-lain", domain.ProviderGoogle
		r.mu.Lock()
		r.users[userID].SSOProvider, r.users[userID].SSOSubject = &google, &sub
		r.mu.Unlock()
		return false, nil
	}
	_, err := f.masuk()
	assert.ErrorIs(t, err, ErrSSOAccountConflict)
	assert.Equal(t, audit.HasilDitolak, f.lastAudit().Hasil)
	assert.Empty(t, f.repo.refresh)
}

func TestSSOCallback_KunciUnikDilanggarSaatMenaut(t *testing.T) {
	f := newSSOFixture(t, ssoUser("u1", "ani@kemenkeu.go.id"))
	f.repo.linkOverride = func(*memRepo, string) (bool, error) { return false, gorm.ErrDuplicatedKey }
	_, err := f.masuk()
	assert.ErrorIs(t, err, ErrSSOAccountConflict)
	assert.Equal(t, "sso_subjek_dipakai_akun_lain", alasanAudit(f.lastAudit()))
}

func TestSSOCallback_GalatBasisDataBukanAkunTidakDitemukan(t *testing.T) {
	f := newSSOFixture(t, ssoUser("u1", "ani@kemenkeu.go.id"))
	f.repo.linkOverride = func(*memRepo, string) (bool, error) { return false, errors.New("koneksi putus") }
	_, err := f.masuk()
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrSSOAccountConflict)
	assert.NotErrorIs(t, err, ErrSSOAccountNotFound)
}

// ── Audit metode masuk ────────────────────────────────────────────────────

func TestWithLoginMethodMetadata(t *testing.T) {
	assert.Nil(t, withLoginMethodMetadata(context.Background(), nil), "login kata sandi: metadata tidak berubah")
	base := map[string]any{"mfa": "totp"}
	assert.Equal(t, map[string]any{"mfa": "totp"}, withLoginMethodMetadata(context.Background(), base))

	ctx := withLoginMethod(context.Background(), MetodeSSOGoogle)
	assert.Equal(t, map[string]any{"metode": "sso_google"}, withLoginMethodMetadata(ctx, nil))
	assert.Equal(t, map[string]any{"mfa": "totp", "metode": "sso_google"}, withLoginMethodMetadata(ctx, base))
}
