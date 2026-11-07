package usecase

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/garda-inflasi/api/internal/auth/domain"
)

// Pemetaan galat usecase manajemen pengguna (buat, ubah, hapus, reset kata sandi, ganti peran, ekspor, katalog,
// izin langsung): galat basis data tidak boleh menyamar sebagai "tidak ada" atau "ditolak", dan aksi yang
// ditolak tidak menulis apa pun maupun mengaudit.

var errKoneksiPutus = errors.New("koneksi putus")

// Regresi: UpdateUser, DeleteUser, dan ResetPasswordAdmin dulu menjawab ErrUserNotFound untuk SETIAP galat
// GetUserByID, sehingga basis data yang mati tampil sebagai "Pengguna tidak ditemukan" (404) di UI dan admin
// mengira akunnya sudah hilang. GetUser, BukaKunci, ForceLogout, dan RiwayatPeran sudah membedakannya.
func TestMuatPengguna_GalatBasisDataBukanPenggunaTakAda(t *testing.T) {
	ctx := context.Background()
	aksi := map[string]func(uc *UseCase) error{
		"GetUser": func(uc *UseCase) error { _, err := uc.GetUser(ctx, uid("user-123")); return err },
		"UpdateUser": func(uc *UseCase) error {
			_, err := uc.UpdateUser(ctx, uid("user-123"), UserPatch{Nama: ptr("x")}, uid("admin-1"))
			return err
		},
		"DeleteUser": func(uc *UseCase) error { return uc.DeleteUser(ctx, uid("user-123"), uid("admin-1")) },
		"ResetPasswordAdmin": func(uc *UseCase) error {
			return uc.ResetPasswordAdmin(ctx, uid("user-123"), "Baru#Sandi123", uid("admin-1"))
		},
	}
	for nama, jalankan := range aksi {
		t.Run(nama, func(t *testing.T) {
			repo := new(mockRepository)
			uc, recorder := newTestUseCaseWithAudit(repo)
			repo.On("GetUserByID", mock.Anything, uid("user-123")).Return(nil, errKoneksiPutus)

			err := jalankan(uc)

			require.Error(t, err)
			assert.NotErrorIs(t, err, ErrUserNotFound, "galat basis data bukan 'pengguna tidak ada'")
			assert.ErrorIs(t, err, errKoneksiPutus, "penyebab asli tetap terbaca di log")
			for _, tulis := range []string{"PatchUser", "DeleteUser", "SetPassword", "RevokeAllUserRefreshTokens"} {
				repo.AssertNotCalled(t, tulis, mock.Anything, mock.Anything, mock.Anything)
			}
			assert.Empty(t, recorder.Entries())
		})
	}
}

func TestMuatPengguna_TidakAdaTetapErrUserNotFoundMeski_Terbungkus(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	repo.On("GetUserByID", mock.Anything, "hilang").Return(nil, fmt.Errorf("baca: %w", gorm.ErrRecordNotFound))

	_, err := uc.GetUser(context.Background(), "hilang")
	require.ErrorIs(t, err, ErrUserNotFound)
	require.ErrorIs(t, uc.DeleteUser(context.Background(), "hilang", uid("admin-1")), ErrUserNotFound)
	require.ErrorIs(t, uc.ResetPasswordAdmin(context.Background(), "hilang", "Baru#Sandi123", uid("admin-1")), ErrUserNotFound)
}

// ── CreateUser ────────────────────────────────────────────────────────

func penggunaBaruUji() *domain.Pengguna {
	return &domain.Pengguna{ID: uid("u-baru"), Nama: "Baru", Email: "baru@garda.test", Peran: "OPS_MAHASISWA", Lapis: "OPERASIONAL", Lembaga: "BPPSDMP"}
}

func TestCreateUser_Penolakan(t *testing.T) {
	ctx := context.Background()

	t.Run("kata sandi lemah ditolak sebelum menyentuh repositori", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		err := uc.CreateUser(ctx, penggunaBaruUji(), "12345678", uid("admin-1"))
		require.ErrorIs(t, err, ErrWeakPassword)
	})
	t.Run("email sudah terdaftar", func(t *testing.T) {
		repo := new(mockRepository)
		uc, recorder := newTestUseCaseWithAudit(repo)
		repo.On("GetUserByEmail", mock.Anything, "baru@garda.test").Return(activeUser(), nil)
		err := uc.CreateUser(ctx, penggunaBaruUji(), "Baru#Sandi123", uid("admin-1"))
		require.ErrorIs(t, err, ErrEmailExists)
		repo.AssertNotCalled(t, "CreateUser", mock.Anything, mock.Anything)
		assert.Empty(t, recorder.Entries())
	})
	t.Run("galat menyimpan dibungkus dan tidak diaudit", func(t *testing.T) {
		repo := new(mockRepository)
		uc, recorder := newTestUseCaseWithAudit(repo)
		repo.On("GetUserByEmail", mock.Anything, "baru@garda.test").Return(nil, gorm.ErrRecordNotFound)
		repo.On("CreateUser", mock.Anything, mock.Anything).Return(errKoneksiPutus)
		err := uc.CreateUser(ctx, penggunaBaruUji(), "Baru#Sandi123", uid("admin-1"))
		require.ErrorIs(t, err, errKoneksiPutus)
		assert.NotErrorIs(t, err, ErrEmailExists)
		assert.Empty(t, recorder.Entries())
	})
}

func TestCreateUser_BerhasilWajibGantiDanAuditTanpaRahasia(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	repo.On("GetUserByEmail", mock.Anything, "baru@garda.test").Return(nil, gorm.ErrRecordNotFound)
	var disimpan *domain.Pengguna
	repo.On("CreateUser", mock.Anything, mock.Anything).Run(func(a mock.Arguments) { disimpan = a.Get(1).(*domain.Pengguna) }).Return(nil)

	require.NoError(t, uc.CreateUser(context.Background(), penggunaBaruUji(), "Baru#Sandi123", uid("admin-1")))

	require.NotNil(t, disimpan.PasswordHash)
	assert.NotEqual(t, "Baru#Sandi123", *disimpan.PasswordHash, "hanya hash yang disimpan")
	assert.True(t, disimpan.WajibGantiPassword, "kata sandi sementara dari admin wajib diganti pemiliknya")
	e, ok := recorder.Last()
	require.True(t, ok)
	assert.Equal(t, "CREATE_USER", e.Aksi)
	assert.Equal(t, "OPS_MAHASISWA", e.Sesudah.(map[string]any)["peran"], "riwayat peran membaca peran awal dari sini")
	assert.NotContains(t, fmt.Sprint(e), "Baru#Sandi123")
	assert.NotContains(t, fmt.Sprint(e), *disimpan.PasswordHash)
}

// ── AssignRole & UpdateUser: galat repositori ─────────────────────────

func TestAssignRole_Penolakan(t *testing.T) {
	ctx := context.Background()
	t.Run("peran di luar katalog ditolak tanpa menyentuh repositori", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		_, err := uc.AssignRole(ctx, uid("user-123"), "SUPERADMIN", uid("admin-1"))
		require.ErrorIs(t, err, ErrRoleNotFound)
	})
	for nama, tc := range map[string]struct {
		dari  error
		ingin error
	}{
		"pengguna hilang saat menulis": {fmt.Errorf("patch: %w", gorm.ErrRecordNotFound), ErrUserNotFound},
		"pemegang terakhir":            {fmt.Errorf("patch: %w", domain.ErrPemegangTerakhir), ErrPemegangTerakhir},
		"galat basis data":             {errKoneksiPutus, errKoneksiPutus},
	} {
		t.Run(nama, func(t *testing.T) {
			repo := new(mockRepository)
			uc, recorder := newTestUseCaseWithAudit(repo)
			repo.On("PatchUser", mock.Anything, uid("user-123"), mock.Anything).Return(tc.dari)

			_, err := uc.AssignRole(ctx, uid("user-123"), "MANAJER_DATA", uid("admin-1"))

			require.ErrorIs(t, err, tc.ingin)
			if tc.ingin == errKoneksiPutus {
				assert.NotErrorIs(t, err, ErrUserNotFound)
				assert.NotErrorIs(t, err, ErrPemegangTerakhir)
			}
			repo.AssertNotCalled(t, "RevokeAllUserRefreshTokens", mock.Anything, mock.Anything)
			assert.Empty(t, recorder.Entries(), "peran tidak berubah: tidak ada yang diaudit")
		})
	}
}

func TestUpdateUser_GalatPenulisanDipetakan(t *testing.T) {
	ctx := context.Background()
	for nama, tc := range map[string]struct {
		dari  error
		ingin error
	}{
		"email kembar dari basis data": {fmt.Errorf("patch: %w", gorm.ErrDuplicatedKey), ErrEmailExists},
		"pemegang terakhir":            {fmt.Errorf("patch: %w", domain.ErrPemegangTerakhir), ErrPemegangTerakhir},
		"pengguna terhapus di sela":    {fmt.Errorf("patch: %w", gorm.ErrRecordNotFound), ErrUserNotFound},
	} {
		t.Run(nama, func(t *testing.T) {
			repo := new(mockRepository)
			uc, recorder := newTestUseCaseWithAudit(repo)
			repo.On("GetUserByID", mock.Anything, uid("user-123")).Return(activeUser(), nil)
			repo.On("PatchUser", mock.Anything, uid("user-123"), mock.Anything).Return(tc.dari)

			_, err := uc.UpdateUser(ctx, uid("user-123"), UserPatch{Nama: ptr("x")}, uid("admin-1"))

			require.ErrorIs(t, err, tc.ingin)
			assert.Empty(t, recorder.Entries())
			repo.AssertNotCalled(t, "RevokeAllUserRefreshTokens", mock.Anything, mock.Anything)
		})
	}
}

// ── Ekspor ────────────────────────────────────────────────────────────

func TestEksporPengguna_MasukanSalahDitolakSebelumMembacaData(t *testing.T) {
	ctx := context.Background()
	for nama, filter := range map[string]domain.FilterPengguna{
		"status tak dikenal": {Status: "aktif"},
		"lapis tak dikenal":  {Lapis: "OPS"},
		"peran tak dikenal":  {Peran: "SUPERADMIN"},
	} {
		t.Run(nama, func(t *testing.T) {
			f := newUndanganFixture(t)
			_, err := f.uc.EksporPengguna(ctx, filter, adminUndangID)
			var salah *domain.FilterPenggunaError
			require.ErrorAs(t, err, &salah)
			assert.Empty(t, entriAudit(f, "EKSPOR_DATA"), "ekspor yang ditolak tidak dicatat sebagai keluarnya data")
		})
	}
}

func TestMetadataEkspor_MencatatHanyaFilterYangDiisi(t *testing.T) {
	ya, tidak := true, false

	kosong := metadataEkspor(domain.FilterPengguna{}, 3, false)
	assert.Equal(t, map[string]any{"baris": 3, "terpotong": false, "search": "", "jumlah_id": 0}, kosong)

	penuh := metadataEkspor(domain.FilterPengguna{
		Cari: "ani", Peran: "ADMIN_SISTEM", Status: "AKTIF", Lapis: "SISTEM", MFA: &ya, Terkunci: &tidak, IDs: []string{"a", "b"},
	}, 2, true)
	assert.Equal(t, "ani", penuh["search"])
	assert.Equal(t, "ADMIN_SISTEM", penuh["peran"])
	assert.Equal(t, "AKTIF", penuh["status"])
	assert.Equal(t, "SISTEM", penuh["lapis"])
	assert.Equal(t, true, penuh["mfa"])
	assert.Equal(t, false, penuh["terkunci"], "false tetap dicatat: bedakan 'tanpa filter' dari 'hanya yang tidak terkunci'")
	assert.Equal(t, 2, penuh["jumlah_id"], "jumlah id, bukan daftar id (audit tidak menimbun id)")
	assert.Equal(t, true, penuh["terpotong"])
	assert.NotContains(t, penuh, "ids")
}

// ── Galat berkas pembantu ─────────────────────────────────────────────

func TestGalatBerkasPembantu(t *testing.T) {
	assert.Equal(t, "Nama wajib diisi.", (&GalatValidasi{Pesan: "Nama wajib diisi."}).Error())

	konflik := &KonflikVersiPeran{Saat: RoleInfo{ID: "ADMIN_SISTEM", Versi: 5}}
	assert.ErrorIs(t, konflik, ErrVersiPeranBerubah)
	assert.Equal(t, ErrVersiPeranBerubah.Error(), konflik.Error())
}

// ── Katalog & izin langsung: galat penyimpanan ────────────────────────

// izinBermasalah membuat setiap metode penyimpanan izin gagal dengan err (kecuali yang ditimpa tes).
type izinBermasalah struct {
	*izinFake
	err error
}

func (i izinBermasalah) ListPeran(context.Context) ([]domain.PeranDetail, error) { return nil, i.err }
func (i izinBermasalah) ListIzin(context.Context) ([]domain.Izin, error)         { return nil, i.err }
func (i izinBermasalah) SetIzinPeran(context.Context, domain.SetIzinPeran) (*domain.HasilSetIzinPeran, error) {
	return nil, i.err
}
func (i izinBermasalah) ListIzinLangsung(context.Context, string) (*domain.IzinPengguna, error) {
	return nil, i.err
}
func (i izinBermasalah) BerikanIzinLangsung(context.Context, domain.BerikanIzin) (*domain.IzinLangsungDetail, *domain.IzinLangsung, error) {
	return nil, nil, i.err
}
func (i izinBermasalah) CabutIzinLangsung(context.Context, string, string) (*domain.IzinLangsung, error) {
	return nil, i.err
}

type repoIzinBermasalah struct {
	*mockRepository
	izinBermasalah
}

func ucIzinBermasalah(err error) *UseCase {
	jam := &jamTes{t: time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC)}
	repo := &repoIzinBermasalah{mockRepository: new(mockRepository), izinBermasalah: izinBermasalah{izinFake: newIzinFake(jam.Now), err: err}}
	uc := newTestUseCase(new(mockRepository))
	uc.izin = repo
	return uc
}

func TestKatalogDanIzinLangsung_GalatPenyimpananTidakMenyamarSebagaiMasukanSalah(t *testing.T) {
	ctx := context.Background()
	uc := ucIzinBermasalah(errKoneksiPutus)
	semua := map[string]func() error{
		"ListRoles":       func() error { _, err := uc.ListRoles(ctx); return err },
		"ListPermissions": func() error { _, err := uc.ListPermissions(ctx); return err },
		"UpdateRolePermissions": func() error {
			_, err := uc.UpdateRolePermissions(ctx, "ADMIN_SISTEM", 1, []string{"harga.lihat"}, uid("admin-1"))
			return err
		},
		"ListUserPermissions": func() error { _, err := uc.ListUserPermissions(ctx, uid("u-1")); return err },
		"GrantUserPermission": func() error {
			_, err := uc.GrantUserPermission(ctx, GrantInput{UserID: uid("u-1"), Izin: "harga.ekspor", Alasan: "audit"}, uid("admin-1"))
			return err
		},
		"RevokeUserPermission": func() error { return uc.RevokeUserPermission(ctx, uid("u-1"), "harga.ekspor", uid("admin-1")) },
	}
	for nama, jalankan := range semua {
		t.Run(nama, func(t *testing.T) {
			err := jalankan()
			require.ErrorIs(t, err, errKoneksiPutus)
			for _, bukan := range []error{ErrUserNotFound, ErrRoleNotFound, ErrIzinTidakDikenal, ErrIzinDariPeran, ErrIzinLangsungAda, ErrIzinLangsungTidakAda, ErrPemegangTerakhir} {
				assert.NotErrorIs(t, err, bukan, nama)
			}
		})
	}
}

func TestUpdateRolePermissions_PeranHilangDanIzinDitolakPenyimpanan(t *testing.T) {
	ctx := context.Background()
	t.Run("peran tidak ada di basis data", func(t *testing.T) {
		uc := ucIzinBermasalah(fmt.Errorf("baca peran: %w", gorm.ErrRecordNotFound))
		_, err := uc.UpdateRolePermissions(ctx, "ADMIN_SISTEM", 1, nil, uid("admin-1"))
		require.ErrorIs(t, err, ErrRoleNotFound)
	})
	t.Run("nama izin ditolak kunci asing", func(t *testing.T) {
		uc := ucIzinBermasalah(domain.ErrIzinTidakDikenal)
		_, err := uc.UpdateRolePermissions(ctx, "ADMIN_SISTEM", 1, []string{"harga.lihat"}, uid("admin-1"))
		require.ErrorIs(t, err, ErrIzinTidakDikenal)
	})
}

func TestGrantUserPermission_PenolakanPenyimpanan(t *testing.T) {
	ctx := context.Background()
	in := GrantInput{UserID: uid("u-1"), Izin: "harga.ekspor", Alasan: "audit"}
	for nama, tc := range map[string]struct {
		dari  error
		ingin error
	}{
		"pengguna tidak ada":       {gorm.ErrRecordNotFound, ErrUserNotFound},
		"izin sudah dari peran":    {domain.ErrIzinDariPeran, ErrIzinDariPeran},
		"izin langsung masih ada":  {domain.ErrIzinLangsungAda, ErrIzinLangsungAda},
		"nama ditolak kunci asing": {domain.ErrIzinTidakDikenal, ErrIzinTidakDikenal},
	} {
		t.Run(nama, func(t *testing.T) {
			uc := ucIzinBermasalah(tc.dari)
			_, err := uc.GrantUserPermission(ctx, in, uid("admin-1"))
			require.ErrorIs(t, err, tc.ingin)
		})
	}
	t.Run("pencabutan tanpa izin langsung", func(t *testing.T) {
		uc := ucIzinBermasalah(domain.ErrIzinLangsungTidakAda)
		require.ErrorIs(t, uc.RevokeUserPermission(ctx, uid("u-1"), "harga.ekspor", uid("admin-1")), ErrIzinLangsungTidakAda)
	})
	t.Run("daftar izin pengguna yang tidak ada", func(t *testing.T) {
		uc := ucIzinBermasalah(gorm.ErrRecordNotFound)
		_, err := uc.ListUserPermissions(ctx, "hilang")
		require.ErrorIs(t, err, ErrUserNotFound)
	})
}

func TestCreateUser_PeranDanLapisHarusDikenalSebelumMenyentuhRepositori(t *testing.T) {
	ctx := context.Background()
	t.Run("peran di luar katalog", func(t *testing.T) {
		repo := new(mockRepository) // GetUserByEmail/CreateUser tidak diharapkan: mock panik bila dipanggil
		uc := newTestUseCase(repo)
		u := penggunaBaruUji()
		u.Peran = "SUPERADMIN"
		require.ErrorIs(t, uc.CreateUser(ctx, u, "Baru#Sandi123", uid("admin-1")), ErrRoleNotFound)
	})
	t.Run("lapis di luar enum", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		u := penggunaBaruUji()
		u.Lapis = "OPS"
		err := uc.CreateUser(ctx, u, "Baru#Sandi123", uid("admin-1"))
		var validasi *GalatValidasi
		require.ErrorAs(t, err, &validasi)
		assert.Contains(t, validasi.Pesan, "Lapis tidak dikenal")
	})
	t.Run("lapis eksplisit yang sah boleh berbeda dari lapis bawaan peran", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		repo.On("GetUserByEmail", mock.Anything, "baru@garda.test").Return(nil, gorm.ErrRecordNotFound)
		repo.On("CreateUser", mock.Anything, mock.Anything).Return(nil)
		u := penggunaBaruUji()
		u.Lapis = "SISTEM"
		require.NoError(t, uc.CreateUser(ctx, u, "Baru#Sandi123", uid("admin-1")))
	})
}
