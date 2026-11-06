package usecase

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/audit/audittest"
	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/pkg/crypto"
	"github.com/garda-inflasi/api/pkg/email"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/token"
)

// Tes aksi admin atas pengguna (ubah, hapus, reset, aksi massal, katalog peran) dan jalur galat yang tidak
// tersentuh tes alur masuk. Yang diperiksa adalah akibatnya: sesi tercabut atau tidak, audit, dan galat.

func ptr[T any](v T) *T { return &v }

// ── UpdateUser ────────────────────────────────────────────────────────

func TestUpdateUser_PergantianPeranMencabutSemuaSesi(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	user := activeUser() // OPS_MAHASISWA / OPS
	repo.On("GetUserByID", mock.Anything, "user-123").Return(user, nil)
	repo.On("PatchUser", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("RevokeAllUserRefreshTokens", mock.Anything, "user-123").Return(nil)

	updated, err := uc.UpdateUser(context.Background(), "user-123", UserPatch{Peran: ptr("MANAJER_DATA")}, "admin-1")

	require.NoError(t, err)
	repo.AssertCalled(t, "RevokeAllUserRefreshTokens", mock.Anything, "user-123")
	assert.Equal(t, "MANAJER_DATA", updated.Peran)
	assert.Equal(t, roleLapis["MANAJER_DATA"], updated.Lapis, "lapis mengikuti peran bila tidak ditimpa")
	entry, ok := recorder.Last()
	require.True(t, ok)
	assert.Equal(t, "UPDATE_USER", entry.Aksi)
}

func TestUpdateUser_TidakMencabutSesiBilaPeranDanStatusTidakBerubah(t *testing.T) {
	cases := map[string]UserPatch{
		"hanya nama":                {Nama: ptr("Nama Baru")},
		"peran yang sama":           {Peran: ptr("OPS_MAHASISWA")},
		"status tetap aktif":        {Status: ptr("AKTIF")},
		"lembaga dan kabupaten":     {Lembaga: ptr("BPS"), KabupatenKode: ptr("1171")},
		"lapis ditimpa tanpa peran": {Lapis: ptr("MANAJERIAL")},
	}
	for name, patch := range cases {
		t.Run(name, func(t *testing.T) {
			repo := new(mockRepository)
			uc := newTestUseCase(repo)
			repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)
			repo.On("PatchUser", mock.Anything, mock.Anything, mock.Anything).Return(nil)

			_, err := uc.UpdateUser(context.Background(), "user-123", patch, "admin-1")

			require.NoError(t, err)
			repo.AssertNotCalled(t, "RevokeAllUserRefreshTokens", mock.Anything, mock.Anything)
		})
	}
}

func TestUpdateUser_StatusSelainAktifMencabutSesi(t *testing.T) {
	// MENUNGGU tidak bisa dipasang lewat PATCH (hanya alur undangan); lihat undangan_test.go.
	for _, status := range []string{"NONAKTIF"} {
		t.Run(status, func(t *testing.T) {
			repo := new(mockRepository)
			uc := newTestUseCase(repo)
			repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)
			repo.On("PatchUser", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			repo.On("RevokeAllUserRefreshTokens", mock.Anything, "user-123").Return(nil)

			_, err := uc.UpdateUser(context.Background(), "user-123", UserPatch{Status: ptr(status)}, "admin-1")

			require.NoError(t, err)
			repo.AssertCalled(t, "RevokeAllUserRefreshTokens", mock.Anything, "user-123")
		})
	}
}

func TestUpdateUser_KegagalanMencabutSesiDilaporkanDanTidakDiauditSebagaiBerhasil(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)
	repo.On("PatchUser", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("RevokeAllUserRefreshTokens", mock.Anything, "user-123").Return(errors.New("db putus"))

	_, err := uc.UpdateUser(context.Background(), "user-123", UserPatch{Peran: ptr("OPS_KOORDINATOR")}, "admin-1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "revoke sessions")
	assert.Empty(t, recorder.Entries(), "perubahan yang sesinya gagal dicabut tidak dicatat berhasil")
}

func TestUpdateUser_LapisEksplisitMengalahkanTurunanPeran(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)
	repo.On("PatchUser", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("RevokeAllUserRefreshTokens", mock.Anything, "user-123").Return(nil)

	updated, err := uc.UpdateUser(context.Background(), "user-123", UserPatch{Peran: ptr("MANAJER_DATA"), Lapis: ptr("KHUSUS")}, "admin-1")

	require.NoError(t, err)
	assert.Equal(t, "KHUSUS", updated.Lapis)
}

func TestUpdateUser_PeranTakDikenalDitolakTanpaMenulisApaPun(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)

	_, err := uc.UpdateUser(context.Background(), "user-123", UserPatch{Peran: ptr("SUPER_ADMIN")}, "admin-1")

	require.ErrorIs(t, err, ErrRoleNotFound)
	repo.AssertNotCalled(t, "PatchUser", mock.Anything, mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "RevokeAllUserRefreshTokens", mock.Anything, mock.Anything)
}

func TestUpdateUser_EmailDipakaiPenggunaLainDitolak(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	lain := activeUser()
	lain.ID = "user-lain"
	repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)
	repo.On("GetUserByEmail", mock.Anything, "lain@example.com").Return(lain, nil)

	_, err := uc.UpdateUser(context.Background(), "user-123", UserPatch{Email: ptr("lain@example.com")}, "admin-1")

	require.ErrorIs(t, err, ErrEmailExists)
	repo.AssertNotCalled(t, "PatchUser", mock.Anything, mock.Anything, mock.Anything)
}

func TestUpdateUser_EmailMilikSendiriBukanKonflik(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	sendiri := activeUser()
	repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)
	repo.On("GetUserByEmail", mock.Anything, "Test@Example.com").Return(sendiri, nil) // ID sama, beda huruf
	repo.On("PatchUser", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	updated, err := uc.UpdateUser(context.Background(), "user-123", UserPatch{Email: ptr("Test@Example.com")}, "admin-1")

	require.NoError(t, err)
	assert.Equal(t, "Test@Example.com", updated.Email)
}

func TestUpdateUser_PenggunaTakAdaDanGalatSimpan(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	repo.On("GetUserByID", mock.Anything, "hilang").Return(nil, gorm.ErrRecordNotFound)
	_, err := uc.UpdateUser(context.Background(), "hilang", UserPatch{Nama: ptr("x")}, "admin-1")
	require.ErrorIs(t, err, ErrUserNotFound)

	repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)
	repo.On("PatchUser", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("db putus"))
	_, err = uc.UpdateUser(context.Background(), "user-123", UserPatch{Nama: ptr("x")}, "admin-1")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrUserNotFound)
	assert.Empty(t, recorder.Entries())
}

// ── DeleteUser ────────────────────────────────────────────────────────

func TestDeleteUser_TidakBolehMenghapusDiriSendiri(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	err := uc.DeleteUser(context.Background(), "admin-1", "admin-1")

	require.ErrorIs(t, err, ErrCannotDeleteSelf)
	repo.AssertNotCalled(t, "GetUserByID", mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "DeleteUser", mock.Anything, mock.Anything)
}

func TestDeleteUser_BerhasilMengauditDanMenjagaSesiTerputus(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)
	repo.On("DeleteUser", mock.Anything, "user-123").Return(nil)
	repo.On("RevokeAllUserRefreshTokens", mock.Anything, "user-123").Return(nil)

	require.NoError(t, uc.DeleteUser(context.Background(), "user-123", "admin-1"))

	repo.AssertCalled(t, "RevokeAllUserRefreshTokens", mock.Anything, "user-123")
	entry, ok := recorder.Last()
	require.True(t, ok)
	assert.Equal(t, "DELETE_USER", entry.Aksi)
	assert.Equal(t, "admin-1", entry.AktorID)
	assert.Equal(t, "user-123", entry.EntitasID)
}

// Regresi: penghapusan ditolak basis data (pengguna masih dirujuk) tidak boleh sudah mengeluarkan pengguna dari
// semua sesinya. Dulu sesi dicabut lebih dulu, jadi "Nonaktifkan akun sebagai gantinya" sudah terlambat.
func TestDeleteUser_PenghapusanDitolakTidakMencabutSesi(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)
	repo.On("DeleteUser", mock.Anything, "user-123").Return(fmt.Errorf("hapus: %w", gorm.ErrForeignKeyViolated))

	err := uc.DeleteUser(context.Background(), "user-123", "admin-1")

	require.ErrorIs(t, err, ErrUserHasReferences)
	repo.AssertNotCalled(t, "RevokeAllUserRefreshTokens", mock.Anything, mock.Anything)
	assert.Empty(t, recorder.Entries(), "penghapusan yang gagal tidak diaudit sebagai berhasil")
}

func TestDeleteUser_PenggunaTakAdaDanGalatLain(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	repo.On("GetUserByID", mock.Anything, "hilang").Return(nil, gorm.ErrRecordNotFound)
	require.ErrorIs(t, uc.DeleteUser(context.Background(), "hilang", "admin-1"), ErrUserNotFound)

	repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)
	repo.On("DeleteUser", mock.Anything, "user-123").Return(errors.New("koneksi putus"))
	err := uc.DeleteUser(context.Background(), "user-123", "admin-1")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrUserHasReferences, "galat umum bukan 'masih dirujuk'")
}

func TestDeleteUser_GagalMencabutSesiSesudahHapusTidakMenggagalkan(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)
	repo.On("DeleteUser", mock.Anything, "user-123").Return(nil)
	repo.On("RevokeAllUserRefreshTokens", mock.Anything, "user-123").Return(errors.New("tabel sudah kosong"))

	require.NoError(t, uc.DeleteUser(context.Background(), "user-123", "admin-1"))
	_, ok := recorder.Last()
	assert.True(t, ok, "pengguna sudah terhapus: audit tetap dicatat")
}

// ── ResetPasswordAdmin ────────────────────────────────────────────────

func TestResetPasswordAdmin_MengubahKataSandiWajibGantiDanMencabutSesi(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	user := activeUser()
	repo.On("GetUserByID", mock.Anything, "user-123").Return(user, nil)
	repo.On("SetPassword", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("RevokeAllUserRefreshTokens", mock.Anything, "user-123").Return(nil)

	require.NoError(t, uc.ResetPasswordAdmin(context.Background(), "user-123", "Baru#12345", "admin-1"))

	require.NotNil(t, user.PasswordHash)
	assert.True(t, crypto.VerifyPassword("Baru#12345", *user.PasswordHash))
	assert.False(t, crypto.VerifyPassword("Test1234!", *user.PasswordHash), "kata sandi lama tidak berlaku lagi")
	assert.True(t, user.WajibGantiPassword, "kata sandi yang ditetapkan admin wajib diganti pemiliknya")
	require.NotNil(t, user.PasswordDiubahPada)
	repo.AssertCalled(t, "RevokeAllUserRefreshTokens", mock.Anything, "user-123")
	entry, ok := recorder.Last()
	require.True(t, ok)
	assert.Equal(t, "RESET_PASSWORD_ADMIN", entry.Aksi)
	assert.Equal(t, "admin-1", entry.AktorID)
	assert.Equal(t, "user-123", entry.EntitasID)
}

func TestResetPasswordAdmin_KataSandiLemahDitolakSebelumMenyentuhPengguna(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	err := uc.ResetPasswordAdmin(context.Background(), "user-123", "lemah", "admin-1")

	require.ErrorIs(t, err, ErrWeakPassword)
	repo.AssertNotCalled(t, "GetUserByID", mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "SetPassword", mock.Anything, mock.Anything, mock.Anything)
}

func TestResetPasswordAdmin_JalurGalat(t *testing.T) {
	t.Run("pengguna tak ada", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		repo.On("GetUserByID", mock.Anything, "hilang").Return(nil, gorm.ErrRecordNotFound)
		require.ErrorIs(t, uc.ResetPasswordAdmin(context.Background(), "hilang", "Baru#12345", "admin-1"), ErrUserNotFound)
	})
	t.Run("gagal menyimpan: sesi tidak dicabut dan tidak diaudit", func(t *testing.T) {
		repo := new(mockRepository)
		uc, recorder := newTestUseCaseWithAudit(repo)
		repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)
		repo.On("SetPassword", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("db putus"))

		require.Error(t, uc.ResetPasswordAdmin(context.Background(), "user-123", "Baru#12345", "admin-1"))

		repo.AssertNotCalled(t, "RevokeAllUserRefreshTokens", mock.Anything, mock.Anything)
		assert.Empty(t, recorder.Entries())
	})
	t.Run("gagal mencabut sesi dilaporkan", func(t *testing.T) {
		repo := new(mockRepository)
		uc, recorder := newTestUseCaseWithAudit(repo)
		repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)
		repo.On("SetPassword", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		repo.On("RevokeAllUserRefreshTokens", mock.Anything, "user-123").Return(errors.New("db putus"))

		err := uc.ResetPasswordAdmin(context.Background(), "user-123", "Baru#12345", "admin-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "revoke sessions")
		assert.Empty(t, recorder.Entries())
	})
}

// ── BulkUserAction ────────────────────────────────────────────────────

func TestBulkUserAction_AksiTakDikenalDitolakTanpaMenyentuhRepositori(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	n, err := uc.BulkUserAction(context.Background(), []string{"a"}, "hapus_semua", "admin-1")

	require.ErrorIs(t, err, ErrInvalidAction)
	assert.Zero(t, n)
	repo.AssertNotCalled(t, "BulkUpdateUsers", mock.Anything, mock.Anything, mock.Anything)
}

func TestBulkUserAction_SetiapAksiDiteruskanDanDiaudit(t *testing.T) {
	cases := map[string]domain.BulkUserAction{
		"aktifkan":     domain.BulkAktifkan,
		"nonaktifkan":  domain.BulkNonaktifkan,
		"wajibkan_mfa": domain.BulkWajibkanMFA,
		"akhiri_sesi":  domain.BulkAkhiriSesi,
	}
	for aksi, domainAksi := range cases {
		t.Run(aksi, func(t *testing.T) {
			repo := new(mockRepository)
			uc, recorder := newTestUseCaseWithAudit(repo)
			ids := []string{"a", "b", "c"}
			repo.On("BulkUpdateUsers", mock.Anything, ids, domainAksi).Return(int64(2), nil)

			n, err := uc.BulkUserAction(context.Background(), ids, aksi, "admin-1")

			require.NoError(t, err)
			assert.EqualValues(t, 2, n)
			entry, ok := recorder.Last()
			require.True(t, ok)
			assert.Equal(t, "BULK_"+aksi, entry.Aksi)
			assert.Equal(t, "admin-1", entry.AktorID)
			assert.Equal(t, "2 pengguna", entry.EntitasID, "jumlah yang benar-benar berubah, bukan yang diminta")
			assert.Equal(t, ids, entry.Metadata["ids"])
			assert.Equal(t, audit.HasilBerhasil, entry.Hasil)
		})
	}
}

func TestBulkUserAction_GalatRepositoriDibungkusDanTidakDiaudit(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	repo.On("BulkUpdateUsers", mock.Anything, mock.Anything, mock.Anything).Return(int64(0), errors.New("db putus"))

	_, err := uc.BulkUserAction(context.Background(), []string{"a"}, "nonaktifkan", "admin-1")

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrInvalidAction, "galat basis data bukan aksi tak valid")
	assert.Empty(t, recorder.Entries())
}

// ── Katalog dan daftar ────────────────────────────────────────────────

func TestListUsers_MeneruskanPaginasiDanPencarian(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	want := []domain.Pengguna{*activeUser()}
	repo.On("ListUsers", mock.Anything, 3, 25, "ani").Return(want, int64(61), nil)

	got, total, err := uc.ListUsers(context.Background(), 3, 25, "ani")

	require.NoError(t, err)
	assert.EqualValues(t, 61, total)
	assert.Equal(t, want, got)
}

func TestListRoles_UrutanBakuDanIzinSamaDenganMatriks(t *testing.T) {
	roles := newTestUseCase(new(mockRepository)).ListRoles()

	require.Len(t, roles, len(roleOrder))
	for i, r := range roles {
		assert.Equal(t, roleOrder[i], r.ID, "urutan peran mengikuti roleOrder")
		assert.Equal(t, rolePermissions[r.ID], r.Permissions)
		assert.NotEmpty(t, r.Permissions, r.ID)
	}
	assert.Equal(t, "ADMIN_SISTEM", roles[len(roles)-1].ID)
}

func TestListPermissions_UrutDanTanpaDuplikatDanMencakupSemuaIzinPeran(t *testing.T) {
	perms := newTestUseCase(new(mockRepository)).ListPermissions()

	assert.True(t, sort.StringsAreSorted(perms))
	seen := map[string]bool{}
	for _, p := range perms {
		assert.False(t, seen[p], "duplikat: %s", p)
		seen[p] = true
	}
	for role, ps := range rolePermissions {
		for _, p := range ps {
			assert.True(t, seen[p], "izin %s milik %s hilang dari katalog", p, role)
		}
	}
	assert.Contains(t, perms, "pengguna.kelola")
}

// ── Login: jalur yang tidak boleh menggagalkan atau menyesatkan ──────

// Regresi: completeLogin dulu memanggil UpdateUser (GORM Save seluruh baris) dengan salinan pengguna yang dibaca
// SEBELUM bcrypt. Bila admin menonaktifkan akun, mereset kata sandi, atau mengganti peran selama login berjalan,
// login yang sedang berlangsung menimpanya kembali ke nilai lama. Kini hanya terakhir_masuk yang ditulis.
type adminMenyela struct {
	*memRepo
	sela func()
}

func (r *adminMenyela) CreateRefreshToken(ctx context.Context, tk *domain.RefreshToken) error {
	if r.sela != nil {
		r.sela()
		r.sela = nil
	}
	return r.memRepo.CreateRefreshToken(ctx, tk)
}

func TestLogin_PerubahanAdminSelamaLoginTidakTertimpa(t *testing.T) {
	user := activeUser()
	mem := newMemRepo(user)
	hashBaru, err := crypto.HashPassword("Admin#Reset1")
	require.NoError(t, err)
	repo := &adminMenyela{memRepo: mem}
	repo.sela = func() {
		// Admin bertindak setelah kata sandi diperiksa dan sebelum login selesai.
		mem.mu.Lock()
		defer mem.mu.Unlock()
		u := mem.users[user.ID]
		u.Status, u.Peran, u.PasswordHash, u.WajibGantiPassword = "NONAKTIF", "ADMIN_SISTEM", &hashBaru, true
	}
	cfg := DefaultConfig()
	cfg.MFAKey = testKeyring()
	uc := New(repo, token.NewManager("test-secret-key-32-chars!!"), cfg, logger.NewDefault(), email.New(email.Config{}), &audittest.Recorder{})

	res, err := uc.Login(context.Background(), user.Email, "Test1234!", "10.0.0.1")

	require.NoError(t, err)
	require.NotNil(t, res)
	stored := mem.stored(user.ID)
	assert.Equal(t, "NONAKTIF", stored.Status, "penonaktifan admin tidak boleh dibatalkan login yang sedang berjalan")
	assert.Equal(t, "ADMIN_SISTEM", stored.Peran)
	assert.True(t, stored.WajibGantiPassword)
	assert.True(t, crypto.VerifyPassword("Admin#Reset1", *stored.PasswordHash), "kata sandi hasil reset admin tidak kembali ke yang lama")
	require.NotNil(t, stored.TerakhirMasuk, "terakhir_masuk tetap dicatat")
}

func TestLogin_GagalMencatatMasukTerakhirTidakMenggagalkanLogin(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	repo.On("GetUserByEmail", mock.Anything, "test@example.com").Return(activeUser(), nil)
	repo.On("CreateRefreshToken", mock.Anything, mock.Anything).Return(nil)
	repo.On("CreateLoginAttempt", mock.Anything, mock.Anything).Return(nil)
	repo.On("TouchLastLogin", mock.Anything, "user-123", mock.Anything).Return(errors.New("db lambat"))

	res, err := uc.Login(context.Background(), "test@example.com", "Test1234!", "10.0.0.1")

	require.NoError(t, err)
	assert.NotEmpty(t, res.AccessToken)
}

func TestLogin_GagalMenyimpanRefreshTokenMenggagalkanLoginTanpaMenghapusHitungan(t *testing.T) {
	jamBeku(t, awalUji)
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	repo.On("GetUserByEmail", mock.Anything, mock.Anything).Return(activeUser(), nil)
	repo.On("CreateLoginAttempt", mock.Anything, mock.Anything).Return(nil)
	repo.On("CreateRefreshToken", mock.Anything, mock.Anything).Return(errors.New("db penuh"))
	// Satu kegagalan sebelumnya: login yang gagal menerbitkan sesi tidak boleh menghapusnya.
	require.ErrorIs(t, login(uc, emailUji, sandiSalah), ErrInvalidCredentials)

	err := login(uc, emailUji, "Test1234!")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "save refresh token")
	state := repo.keadaan(emailUji)
	require.NotNil(t, state)
	assert.Equal(t, 2, state.Gagal, "kata sandi yang benar tetapi sesi gagal terbit tidak menghapus catatan kegagalan")
	for _, e := range recorder.Entries() {
		assert.False(t, e.Aksi == "LOGIN" && e.Hasil == audit.HasilBerhasil, "tidak ada audit LOGIN berhasil tanpa sesi")
	}
}

func TestLogin_GagalMenghapusHitunganTidakMenggagalkanLogin(t *testing.T) {
	jamBeku(t, awalUji)
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	siapLogin(repo, activeUser())
	repo.errHapus = errors.New("db lambat")

	res, err := uc.Login(context.Background(), emailUji, "Test1234!", "10.0.0.1")

	require.NoError(t, err)
	assert.NotEmpty(t, res.AccessToken)
}

func TestLogin_GagalMengembalikanPercobaanTidakMenggagalkanTantanganMFA(t *testing.T) {
	jamBeku(t, awalUji)
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	user, _ := userMFA(t, uc)
	repo.On("GetUserByEmail", mock.Anything, mock.Anything).Return(user, nil)
	repo.errKembali = errors.New("db lambat")

	res, err := uc.Login(context.Background(), emailUji, "Test1234!", "10.0.0.1")

	require.NoError(t, err)
	assert.True(t, res.MFARequired)
	assert.Empty(t, res.AccessToken, "tantangan MFA belum membawa sesi")
}

// ── Refresh: galat basis data tidak boleh menyamar sebagai token tidak sah ──

func TestRefresh_PemilikTokenTakTerbacaBukanTokenTidakSah(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	stored := &domain.RefreshToken{ID: 7, UserID: "user-123", FamilyID: "fam-1", ExpiresAt: time.Now().Add(time.Hour)}
	repo.On("GetRefreshTokenByHash", mock.Anything, mock.Anything).Return(stored, nil)
	repo.On("GetUserByID", mock.Anything, "user-123").Return(nil, errors.New("koneksi putus"))

	_, _, err := uc.Refresh(context.Background(), "rt", "10.0.0.1")

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrInvalidToken, "klien akan mengeluarkan pengguna bila menerima 401; galat sementara harus 5xx")
	repo.AssertNotCalled(t, "RevokeRefreshTokenFamily", mock.Anything, mock.Anything)
}

func TestRefresh_PemilikTokenSudahDihapusDianggapTidakSah(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	stored := &domain.RefreshToken{ID: 7, UserID: "user-123", FamilyID: "fam-1", ExpiresAt: time.Now().Add(time.Hour)}
	repo.On("GetRefreshTokenByHash", mock.Anything, mock.Anything).Return(stored, nil)
	repo.On("GetUserByID", mock.Anything, "user-123").Return(nil, gorm.ErrRecordNotFound)

	_, _, err := uc.Refresh(context.Background(), "rt", "10.0.0.1")

	require.ErrorIs(t, err, ErrInvalidToken)
}

func TestRefresh_GagalMenyimpanPenggantiTidakMengeluarkanSesi(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	stored := &domain.RefreshToken{ID: 7, UserID: "user-123", FamilyID: "fam-1", ExpiresAt: time.Now().Add(time.Hour)}
	repo.On("GetRefreshTokenByHash", mock.Anything, mock.Anything).Return(stored, nil)
	repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)
	repo.On("RotateRefreshToken", mock.Anything, int64(7), mock.Anything, mock.Anything).Return(false, errors.New("deadlock"))

	_, _, err := uc.Refresh(context.Background(), "rt", "10.0.0.1")

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrInvalidToken)
	repo.AssertNotCalled(t, "RevokeRefreshTokenFamily", mock.Anything, mock.Anything)
}

// ── ForgotPassword ────────────────────────────────────────────────────

// Regresi (enumerasi dan ketersediaan): SMTP disebut tanpa batas waktu, dan hanya email terdaftar yang sampai ke
// langkah kirim. Menunggunya membuat lama jawaban membedakan email terdaftar dari yang tidak, dan SMTP yang
// macet menggantung permintaan. Pengiriman kini di latar belakang.
func TestForgotPassword_PengirimanEmailTidakMenahanPermintaan(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	var terhubung atomic.Int32
	tahan := make(chan struct{})
	t.Cleanup(func() { close(tahan) })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			terhubung.Add(1)
			go func() { <-tahan; _ = conn.Close() }() // server SMTP yang menerima koneksi tetapi tidak pernah menyapa
		}
	}()
	addr := ln.Addr().(*net.TCPAddr)

	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	uc.emailSender = email.New(email.Config{Host: "127.0.0.1", Port: addr.Port, From: "noreply@garda.test"})
	repo.On("GetUserByEmail", mock.Anything, "test@example.com").Return(activeUser(), nil)
	repo.On("SaveResetToken", mock.Anything, "user-123", mock.Anything, mock.Anything).Return(nil)

	mulai := time.Now()
	err = uc.ForgotPassword(context.Background(), "test@example.com")

	require.NoError(t, err)
	assert.Less(t, time.Since(mulai), 2*time.Second, "permintaan tidak menunggu SMTP")
	entry, ok := recorder.Last()
	require.True(t, ok)
	assert.Equal(t, "FORGOT_PASSWORD", entry.Aksi, "token tetap disimpan dan diaudit")
	assert.Eventually(t, func() bool { return terhubung.Load() > 0 }, 3*time.Second, 10*time.Millisecond,
		"email tetap dicoba dikirim di latar belakang")
}

func TestForgotPassword_GagalMenyimpanTokenDilaporkan(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	repo.On("GetUserByEmail", mock.Anything, "test@example.com").Return(activeUser(), nil)
	repo.On("SaveResetToken", mock.Anything, "user-123", mock.Anything, mock.Anything).Return(errors.New("db putus"))

	err := uc.ForgotPassword(context.Background(), "test@example.com")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "save reset token")
}

// ── ResetPassword dan ChangePassword: cabang galat ───────────────────

func TestResetPassword_JalurGalat(t *testing.T) {
	tokenSah := &domain.ResetToken{ID: 4, UserID: "user-123", ExpiresAt: time.Now().Add(time.Hour)}

	t.Run("pemilik token sudah dihapus: token tidak dipakai", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		repo.On("GetResetToken", mock.Anything, mock.Anything).Return(tokenSah, nil)
		repo.On("GetUserByID", mock.Anything, "user-123").Return(nil, gorm.ErrRecordNotFound)

		require.ErrorIs(t, uc.ResetPassword(context.Background(), "tkn", "Baru#12345"), ErrUserNotFound)
		repo.AssertNotCalled(t, "ConsumeResetToken", mock.Anything, mock.Anything, mock.Anything)
	})
	t.Run("kata sandi sama dengan yang lama ditolak tanpa memakai token", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		repo.On("GetResetToken", mock.Anything, mock.Anything).Return(tokenSah, nil)
		repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)

		require.ErrorIs(t, uc.ResetPassword(context.Background(), "tkn", "Test1234!"), ErrPasswordReuse)
		repo.AssertNotCalled(t, "ConsumeResetToken", mock.Anything, mock.Anything, mock.Anything)
	})
	t.Run("galat basis data saat memakai token dilaporkan, bukan token tidak sah", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		repo.On("GetResetToken", mock.Anything, mock.Anything).Return(tokenSah, nil)
		repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)
		repo.On("ConsumeResetToken", mock.Anything, int64(4), mock.Anything).Return(false, errors.New("db putus"))

		err := uc.ResetPassword(context.Background(), "tkn", "Baru#12345")

		require.Error(t, err)
		assert.NotErrorIs(t, err, ErrResetTokenInvalid)
		repo.AssertNotCalled(t, "RevokeAllUserRefreshTokens", mock.Anything, mock.Anything)
	})
	t.Run("gagal menyimpan kata sandi sesudah token dipakai dilaporkan dan sesi tidak dicabut", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		repo.On("GetResetToken", mock.Anything, mock.Anything).Return(tokenSah, nil)
		repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)
		repo.On("ConsumeResetToken", mock.Anything, int64(4), mock.Anything).Return(true, nil)
		repo.On("SetPassword", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("db putus"))

		require.Error(t, uc.ResetPassword(context.Background(), "tkn", "Baru#12345"))
		repo.AssertNotCalled(t, "RevokeAllUserRefreshTokens", mock.Anything, mock.Anything)
	})
}

func TestChangePassword_JalurGalat(t *testing.T) {
	t.Run("pengguna tak ada", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		repo.On("GetUserByID", mock.Anything, "hilang").Return(nil, gorm.ErrRecordNotFound)
		_, _, err := uc.ChangePassword(context.Background(), "hilang", "Test1234!", "Baru#12345", "10.0.0.1")
		require.ErrorIs(t, err, ErrUserNotFound)
	})
	t.Run("akun tanpa kata sandi (SSO) tidak punya kata sandi lama dan tidak menghitung percobaan", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		user := activeUser()
		user.PasswordHash = nil
		repo.On("GetUserByID", mock.Anything, "user-123").Return(user, nil)

		_, _, err := uc.ChangePassword(context.Background(), "user-123", "apa-saja", "Baru#12345", "10.0.0.1")

		require.ErrorIs(t, err, ErrWrongPassword)
		assert.Nil(t, repo.keadaan(emailUji), "tidak ada rahasia yang diperiksa, jadi tidak ada percobaan yang dipesan")
	})
	t.Run("akun terkunci: kata sandi lama tidak diperiksa", func(t *testing.T) {
		jamBeku(t, awalUji)
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)
		repo.kunciTerkunci(emailUji)

		_, _, err := uc.ChangePassword(context.Background(), "user-123", "Test1234!", "Baru#12345", "10.0.0.1")

		require.ErrorIs(t, err, ErrAccountLocked)
		repo.AssertNotCalled(t, "SetPassword", mock.Anything, mock.Anything, mock.Anything)
	})
	t.Run("gagal memesan percobaan: ditolak, bukan dilewati", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)
		repo.errMulai = errors.New("db putus")

		_, _, err := uc.ChangePassword(context.Background(), "user-123", "Test1234!", "Baru#12345", "10.0.0.1")

		require.Error(t, err)
		repo.AssertNotCalled(t, "SetPassword", mock.Anything, mock.Anything, mock.Anything)
	})
	t.Run("gagal menyimpan kata sandi baru", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)
		repo.On("CreateLoginAttempt", mock.Anything, mock.Anything).Return(nil)
		repo.On("SetPassword", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("db putus"))

		_, _, err := uc.ChangePassword(context.Background(), "user-123", "Test1234!", "Baru#12345", "10.0.0.1")

		require.Error(t, err)
		repo.AssertNotCalled(t, "RevokeAllUserRefreshTokens", mock.Anything, mock.Anything)
	})
}
