package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/audit/audittest"
	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/pkg/crypto"
	"github.com/garda-inflasi/api/pkg/email"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/token"
	"github.com/garda-inflasi/api/pkg/totp"
)

// Regresi: jalur baca-ubah-tulis selain login dulu menyimpan SELURUH baris pengguna (UpdateUser = GORM Save)
// dari salinan yang dibaca lebih dulu. Perubahan admin yang terjadi di sela-sela baca dan tulis tertimpa nilai
// lama: akun yang baru dinonaktifkan kembali AKTIF, peran kembali ke yang lama, hash hasil reset admin kembali
// ke yang lama, MFA yang baru aktif dimatikan. Kini tiap jalur hanya menulis kolom miliknya (PatchUser,
// SetPassword, SetMFASecret, ActivateMFA), dengan syarat keadaan yang diharapkan bila perlu.
// Gayanya mengikuti TestLogin_PerubahanAdminSelamaLoginTidakTertimpa.

// menyelaTulis menjalankan sela tepat sebelum penulisan pertama ke tabel pengguna, yaitu sesudah usecase
// selesai membaca baris dan memeriksa kata sandi: celah yang dimanfaatkan admin lain.
type menyelaTulis struct {
	*memRepo
	sela func()
}

func (r *menyelaTulis) jalankan() {
	if f := r.sela; f != nil {
		r.sela = nil
		f()
	}
}

func (r *menyelaTulis) PatchUser(ctx context.Context, id string, p domain.PenggunaPatch) (*domain.Pengguna, *domain.Pengguna, error) {
	r.jalankan()
	return r.memRepo.PatchUser(ctx, id, p)
}

func (r *menyelaTulis) SetPassword(ctx context.Context, id string, p domain.GantiPassword) (bool, error) {
	r.jalankan()
	return r.memRepo.SetPassword(ctx, id, p)
}

func (r *menyelaTulis) SetMFASecret(ctx context.Context, id, sealed string) (bool, error) {
	r.jalankan()
	return r.memRepo.SetMFASecret(ctx, id, sealed)
}

func (r *menyelaTulis) ActivateMFA(ctx context.Context, id string, kode []string, langkah int64, sealed string) (bool, error) {
	r.jalankan()
	return r.memRepo.ActivateMFA(ctx, id, kode, langkah, sealed)
}

// fixtureKolom menyiapkan usecase di atas penyimpanan dalam memori yang bisa disela. Perubahan admin
// ditulis langsung ke penyimpanan lewat ubah.
type fixtureKolom struct {
	mem   *memRepo
	repo  *menyelaTulis
	uc    *UseCase
	audit *audittest.Recorder
}

func newFixtureKolom(t *testing.T, user *domain.Pengguna) *fixtureKolom {
	t.Helper()
	jamBeku(t, awalUji)
	mem := newMemRepo(user)
	repo := &menyelaTulis{memRepo: mem}
	rec := &audittest.Recorder{}
	cfg := DefaultConfig()
	cfg.MFAKey = testKeyring()
	uc := New(repo, token.NewManager("test-secret-key-32-chars!!"), cfg, logger.NewDefault(), email.New(email.Config{}), rec)
	mem.On("RevokeAllUserRefreshTokens", mock.Anything, mock.Anything).Return(nil)
	return &fixtureKolom{mem: mem, repo: repo, uc: uc, audit: rec}
}

// ubah menerapkan perubahan admin langsung ke baris tersimpan, seperti UPDATE dari permintaan lain.
func (f *fixtureKolom) ubah(id string, fn func(u *domain.Pengguna)) {
	f.mem.mu.Lock()
	defer f.mem.mu.Unlock()
	fn(f.mem.users[id])
}

func (f *fixtureKolom) disela(fn func(u *domain.Pengguna)) {
	f.repo.sela = func() { f.ubah("user-123", fn) }
}

// ── Ganti kata sandi ──────────────────────────────────────────────────

func TestChangePassword_PenonaktifanDanGantiPeranAdminSelamaProsesTidakTertimpa(t *testing.T) {
	f := newFixtureKolom(t, activeUser())
	f.disela(func(u *domain.Pengguna) { u.Status, u.Peran = "NONAKTIF", "ADMIN_SISTEM" })

	_, _, err := f.uc.ChangePassword(context.Background(), "user-123", "Test1234!", "NewTest1@", "10.0.0.1")

	require.NoError(t, err)
	stored := f.mem.stored("user-123")
	assert.Equal(t, "NONAKTIF", stored.Status, "penonaktifan admin tidak boleh dibalikkan ganti kata sandi")
	assert.Equal(t, "ADMIN_SISTEM", stored.Peran)
	assert.True(t, crypto.VerifyPassword("NewTest1@", *stored.PasswordHash))
	require.NotNil(t, stored.PasswordDiubahPada)
	assert.False(t, stored.WajibGantiPassword)
}

func TestChangePassword_ResetAdminSelamaProsesTidakDitimpaDanDitolak(t *testing.T) {
	user := activeUser()
	user.WajibGantiPassword = false
	f := newFixtureKolom(t, user)
	hashAdmin, err := crypto.HashPassword("Admin#Reset1")
	require.NoError(t, err)
	f.disela(func(u *domain.Pengguna) { u.PasswordHash, u.WajibGantiPassword = &hashAdmin, true })

	_, _, err = f.uc.ChangePassword(context.Background(), "user-123", "Test1234!", "NewTest1@", "10.0.0.1")

	require.ErrorIs(t, err, ErrPenggunaBerubah, "kata sandi lama yang baru saja diverifikasi sudah tidak berlaku")
	stored := f.mem.stored("user-123")
	assert.True(t, crypto.VerifyPassword("Admin#Reset1", *stored.PasswordHash), "kata sandi hasil reset admin tetap")
	assert.True(t, stored.WajibGantiPassword, "penanda wajib ganti dari admin tetap")
	assert.Equal(t, 0, len(f.mem.refresh), "tidak ada sesi baru untuk perubahan yang ditolak")
	assert.Empty(t, f.audit.Entries(), "tidak dicatat berhasil")
}

func TestChangePassword_PenggantianSerentakSatuMenang(t *testing.T) {
	f := newFixtureKolom(t, activeUser())
	hashLain, err := crypto.HashPassword("Lain#Serentak9")
	require.NoError(t, err)
	f.disela(func(u *domain.Pengguna) { u.PasswordHash = &hashLain })

	_, _, err = f.uc.ChangePassword(context.Background(), "user-123", "Test1234!", "NewTest1@", "10.0.0.1")

	require.ErrorIs(t, err, ErrPenggunaBerubah)
	assert.True(t, crypto.VerifyPassword("Lain#Serentak9", *f.mem.stored("user-123").PasswordHash))
}

// ── Reset kata sandi ──────────────────────────────────────────────────

func TestResetPassword_PenonaktifanAdminSelamaProsesTidakTertimpa(t *testing.T) {
	f := newFixtureKolom(t, activeUser())
	f.mem.On("GetResetToken", mock.Anything, mock.Anything).
		Return(&domain.ResetToken{ID: 1, UserID: "user-123", ExpiresAt: time.Now().Add(30 * time.Minute)}, nil)
	f.mem.On("ConsumeResetToken", mock.Anything, int64(1), mock.Anything).Return(true, nil)
	f.disela(func(u *domain.Pengguna) { u.Status, u.Peran = "NONAKTIF", "MANAJER_DATA" })

	require.NoError(t, f.uc.ResetPassword(context.Background(), "token", "NewPass1@"))

	stored := f.mem.stored("user-123")
	assert.Equal(t, "NONAKTIF", stored.Status)
	assert.Equal(t, "MANAJER_DATA", stored.Peran)
	assert.True(t, crypto.VerifyPassword("NewPass1@", *stored.PasswordHash))
	assert.False(t, stored.WajibGantiPassword)
}

func TestResetPasswordAdmin_PerubahanAdminLainSelamaProsesTidakTertimpa(t *testing.T) {
	f := newFixtureKolom(t, activeUser())
	f.disela(func(u *domain.Pengguna) { u.Status, u.Peran, u.MfaWajib = "NONAKTIF", "ADMIN_SISTEM", true })

	require.NoError(t, f.uc.ResetPasswordAdmin(context.Background(), "user-123", "Baru#12345", "admin-1"))

	stored := f.mem.stored("user-123")
	assert.Equal(t, "NONAKTIF", stored.Status)
	assert.Equal(t, "ADMIN_SISTEM", stored.Peran)
	assert.True(t, stored.MfaWajib)
	assert.True(t, crypto.VerifyPassword("Baru#12345", *stored.PasswordHash))
	assert.True(t, stored.WajibGantiPassword)
}

func TestResetPasswordAdmin_PenggunaDihapusSelamaProsesDijawabTidakAda(t *testing.T) {
	f := newFixtureKolom(t, activeUser())
	f.repo.sela = func() {
		f.mem.mu.Lock()
		defer f.mem.mu.Unlock()
		delete(f.mem.users, "user-123")
	}

	err := f.uc.ResetPasswordAdmin(context.Background(), "user-123", "Baru#12345", "admin-1")

	require.ErrorIs(t, err, ErrUserNotFound)
	assert.Empty(t, f.audit.Entries())
}

// ── Admin: ubah pengguna dan ganti peran ──────────────────────────────

func TestUpdateUser_PenonaktifanAdminLainSelamaProsesTidakTertimpaDanAuditAkurat(t *testing.T) {
	f := newFixtureKolom(t, activeUser())
	f.disela(func(u *domain.Pengguna) { u.Status = "NONAKTIF" })

	updated, err := f.uc.UpdateUser(context.Background(), "user-123", UserPatch{Nama: ptr("Nama Baru")}, "admin-1")

	require.NoError(t, err)
	stored := f.mem.stored("user-123")
	assert.Equal(t, "Nama Baru", stored.Nama)
	assert.Equal(t, "NONAKTIF", stored.Status, "PATCH nama tidak boleh mengaktifkan kembali akun yang baru dinonaktifkan")
	assert.Equal(t, "NONAKTIF", updated.Status, "respons memuat keadaan sebenarnya, bukan salinan basi")

	entri, ok := f.audit.Last()
	require.True(t, ok)
	assert.Equal(t, "UPDATE_USER", entri.Aksi)
	sebelum, sesudah := entri.Sebelum.(map[string]any), entri.Sesudah.(map[string]any)
	assert.Equal(t, "NONAKTIF", sebelum["status"], "sebelum dibaca bersama penulisan, jadi tidak seolah-olah PATCH ini yang menonaktifkan")
	assert.Equal(t, "NONAKTIF", sesudah["status"])
	assert.Equal(t, "Test User", sebelum["nama"])
	assert.Equal(t, "Nama Baru", sesudah["nama"])
	f.mem.AssertNotCalled(t, "RevokeAllUserRefreshTokens", mock.Anything, mock.Anything)
}

func TestUpdateUser_HanyaKolomDiminta(t *testing.T) {
	f := newFixtureKolom(t, activeUser())
	f.disela(func(u *domain.Pengguna) {
		hash, _ := crypto.HashPassword("Admin#Reset1")
		u.PasswordHash, u.WajibGantiPassword, u.MfaAktif = &hash, true, true
	})

	_, err := f.uc.UpdateUser(context.Background(), "user-123", UserPatch{Lembaga: ptr("BPS")}, "admin-1")

	require.NoError(t, err)
	stored := f.mem.stored("user-123")
	assert.Equal(t, "BPS", stored.Lembaga)
	assert.True(t, crypto.VerifyPassword("Admin#Reset1", *stored.PasswordHash), "kata sandi hasil reset serentak tetap")
	assert.True(t, stored.WajibGantiPassword)
	assert.True(t, stored.MfaAktif)
}

func TestAssignRole_PenonaktifanAdminLainSelamaProsesTidakTertimpaDanAuditAkurat(t *testing.T) {
	f := newFixtureKolom(t, activeUser())
	f.disela(func(u *domain.Pengguna) { u.Status = "NONAKTIF" })

	updated, err := f.uc.AssignRole(context.Background(), "user-123", "MANAJER_DATA", "admin-1")

	require.NoError(t, err)
	stored := f.mem.stored("user-123")
	assert.Equal(t, "MANAJER_DATA", stored.Peran)
	assert.Equal(t, roleLapis["MANAJER_DATA"], stored.Lapis)
	assert.Equal(t, "NONAKTIF", stored.Status, "ganti peran tidak boleh mengaktifkan kembali akun yang baru dinonaktifkan")
	assert.Equal(t, "NONAKTIF", updated.Status)
	f.mem.AssertCalled(t, "RevokeAllUserRefreshTokens", mock.Anything, "user-123")

	entri, ok := f.audit.Last()
	require.True(t, ok)
	assert.Equal(t, "ASSIGN_ROLE", entri.Aksi)
	sebelum, sesudah := entri.Sebelum.(map[string]any), entri.Sesudah.(map[string]any)
	assert.Equal(t, "OPS_MAHASISWA", sebelum["peran"])
	assert.Equal(t, "MANAJER_DATA", sesudah["peran"])
	assert.Equal(t, "NONAKTIF", sebelum["status"])
	assert.Equal(t, "NONAKTIF", sesudah["status"])
}

func TestAssignRole_PenggunaTidakAdaDijawabTidakAda(t *testing.T) {
	f := newFixtureKolom(t, activeUser())

	_, err := f.uc.AssignRole(context.Background(), "hantu", "MANAJER_DATA", "admin-1")

	require.ErrorIs(t, err, ErrUserNotFound)
	assert.Empty(t, f.audit.Entries())
	f.mem.AssertNotCalled(t, "RevokeAllUserRefreshTokens", mock.Anything, mock.Anything)
}

// ── MFA ───────────────────────────────────────────────────────────────

func TestSetupMFA_AktivasiSerentakTidakDimatikan(t *testing.T) {
	f := newFixtureKolom(t, activeUser())
	rahasiaAktif, err := f.uc.cfg.MFAKey.Seal(totp.GenerateSecret())
	require.NoError(t, err)
	f.disela(func(u *domain.Pengguna) { u.MfaAktif, u.MfaSecret, u.MfaLangkahTerakhir = true, &rahasiaAktif, 99 })

	_, err = f.uc.SetupMFA(context.Background(), "user-123")

	require.ErrorIs(t, err, ErrMFAAlreadyActive)
	stored := f.mem.stored("user-123")
	assert.True(t, stored.MfaAktif, "MFA yang baru diaktifkan tidak boleh dimatikan setup lain")
	assert.Equal(t, rahasiaAktif, *stored.MfaSecret, "rahasia yang sudah aktif tidak diganti rahasia setup")
	assert.EqualValues(t, 99, stored.MfaLangkahTerakhir)
	assert.Empty(t, f.audit.Entries())
}

func TestSetupMFA_PenonaktifanAdminSelamaProsesTidakTertimpa(t *testing.T) {
	f := newFixtureKolom(t, activeUser())
	f.disela(func(u *domain.Pengguna) { u.Status, u.Peran = "NONAKTIF", "ADMIN_SISTEM" })

	setup, err := f.uc.SetupMFA(context.Background(), "user-123")

	require.NoError(t, err)
	stored := f.mem.stored("user-123")
	assert.Equal(t, "NONAKTIF", stored.Status)
	assert.Equal(t, "ADMIN_SISTEM", stored.Peran)
	require.NotNil(t, stored.MfaSecret)
	rahasia, err := f.uc.cfg.MFAKey.Open(*stored.MfaSecret)
	require.NoError(t, err)
	assert.Equal(t, setup.Secret, rahasia)
	assert.False(t, stored.MfaAktif)
}

// userSetupMFA adalah pengguna yang sudah memanggil SetupMFA: rahasia bersegel tersimpan, belum aktif.
func userSetupMFA(t *testing.T, f *fixtureKolom) (secret, sealed string) {
	t.Helper()
	secret = totp.GenerateSecret()
	var err error
	sealed, err = f.uc.cfg.MFAKey.Seal(secret)
	require.NoError(t, err)
	f.ubah("user-123", func(u *domain.Pengguna) { u.MfaSecret = &sealed })
	return secret, sealed
}

func TestVerifyMFA_SetupDiulangDiTabLainSelamaVerifikasiTidakMengaktifkanRahasiaBaru(t *testing.T) {
	f := newFixtureKolom(t, activeUser())
	secret, _ := userSetupMFA(t, f)
	rahasiaBaru, err := f.uc.cfg.MFAKey.Seal(totp.GenerateSecret())
	require.NoError(t, err)
	// Tab lain memanggil SetupMFA lagi tepat sebelum aktivasi: rahasia berganti, belum pernah dipindai pengguna.
	f.disela(func(u *domain.Pengguna) { u.MfaSecret = &rahasiaBaru })
	kode, err := totp.GenerateTOTP(secret, time.Now())
	require.NoError(t, err)

	_, err = f.uc.VerifyMFA(context.Background(), "user-123", kode, "10.0.0.1")

	require.ErrorIs(t, err, ErrMFASetupBerubah)
	stored := f.mem.stored("user-123")
	assert.False(t, stored.MfaAktif, "MFA tidak boleh aktif dengan rahasia yang kodenya tidak pernah diverifikasi")
	assert.Equal(t, rahasiaBaru, *stored.MfaSecret)
	assert.Empty(t, f.audit.Entries())
	f.mem.AssertNotCalled(t, "RevokeAllUserRefreshTokens", mock.Anything, mock.Anything)
}

func TestVerifyMFA_AktivasiSerentakHanyaSatuYangMenerbitkanKode(t *testing.T) {
	f := newFixtureKolom(t, activeUser())
	secret, _ := userSetupMFA(t, f)
	// Permintaan lain mengaktifkan MFA lebih dulu dengan rahasia yang sama.
	f.disela(func(u *domain.Pengguna) { u.MfaAktif = true })
	kode, err := totp.GenerateTOTP(secret, time.Now())
	require.NoError(t, err)

	hasil, err := f.uc.VerifyMFA(context.Background(), "user-123", kode, "10.0.0.1")

	require.ErrorIs(t, err, ErrMFAAlreadyActive)
	assert.Nil(t, hasil, "kode cadangan dari aktivasi yang kalah tidak boleh sampai ke pengguna")
	assert.Empty(t, f.audit.Entries())
}

func TestVerifyMFA_AktivasiNormalMenulisHanyaKolomMFA(t *testing.T) {
	f := newFixtureKolom(t, activeUser())
	secret, _ := userSetupMFA(t, f)
	f.mem.On("RevokeAllUserRefreshTokens", mock.Anything, mock.Anything).Return(nil).Maybe()
	f.disela(func(u *domain.Pengguna) { u.Peran = "MANAJER_DATA" })
	kode, err := totp.GenerateTOTP(secret, time.Now())
	require.NoError(t, err)

	hasil, err := f.uc.VerifyMFA(context.Background(), "user-123", kode, "10.0.0.1")

	require.NoError(t, err)
	assert.Len(t, hasil.KodeCadangan, JumlahKodeCadangan)
	stored := f.mem.stored("user-123")
	assert.True(t, stored.MfaAktif)
	assert.Equal(t, "MANAJER_DATA", stored.Peran, "peran dari admin yang menyela tidak tertimpa")
	entri, ok := f.audit.Last()
	require.True(t, ok)
	assert.Equal(t, "MFA_VERIFY", entri.Aksi)
	assert.Equal(t, audit.HasilBerhasil, entri.Hasil)
}
