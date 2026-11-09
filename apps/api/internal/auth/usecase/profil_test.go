package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/audit/audittest"
	"github.com/garda-inflasi/api/internal/auth/domain"
)

func ptrStr(s string) *string { return &s }

func TestNormalisasiTelepon(t *testing.T) {
	cases := []struct {
		name   string
		mentah string
		want   string
		galat  error
	}{
		// Bentuk yang diterima menjadi "+62" + nomor nasional.
		{"E.164 apa adanya", "+6281234567890", "+6281234567890", nil},
		{"awalan 0", "081234567890", "+6281234567890", nil},
		{"awalan 62", "6281234567890", "+6281234567890", nil},
		{"tanda hubung", "0812-3456-7890", "+6281234567890", nil},
		{"titik", "0812.3456.7890", "+6281234567890", nil},
		{"spasi", "0812 3456 7890", "+6281234567890", nil},
		{"kurung", "(0812) 3456 7890", "+6281234567890", nil},
		{"+62 dengan spasi dan tanda hubung", "+62 812-3456-7890", "+6281234567890", nil},
		{"+62 dengan kurung", "+62(812)3456-7890", "+6281234567890", nil},
		{"62 dengan spasi", "62 812 3456 7890", "+6281234567890", nil},
		{"dipangkas", "  0812 3456 7890\t\n", "+6281234567890", nil},
		{"spasi tak terputus", "0812 3456 7890", "+6281234567890", nil},
		{"telepon rumah", "(0651) 123-456", "+62651123456", nil},
		{"batas bawah 8 digit nasional", "012345678", "+6212345678", nil},
		{"batas atas 13 digit nasional", "01234567890123", "+621234567890123", nil},

		// Kosong menghapus nomor.
		{"kosong", "", "", nil},
		{"hanya spasi", "   ", "", nil},
		{"hanya tab dan baris baru", "\t\n ", "", nil},

		// Tidak valid.
		{"kode negara lain", "+1 415 555 2671", "", ErrTeleponTidakValid},
		{"kode negara lain 44", "+44 20 7946 0958", "", ErrTeleponTidakValid},
		{"tanpa awalan", "81234567890", "", ErrTeleponTidakValid},
		{"tanpa awalan, 10 digit", "8123456789", "", ErrTeleponTidakValid},
		{"+620 (nol sesudah kode negara)", "+6208123456789", "", ErrTeleponTidakValid},
		{"620 (nol sesudah kode negara)", "6208123456789", "", ErrTeleponTidakValid},
		{"00 internasional", "0062812345678", "", ErrTeleponTidakValid},
		{"hanya awalan 0", "0", "", ErrTeleponTidakValid},
		{"hanya +62", "+62", "", ErrTeleponTidakValid},
		{"hanya 62", "62", "", ErrTeleponTidakValid},
		{"7 digit nasional (terlalu pendek)", "01234567", "", ErrTeleponTidakValid},
		{"terlalu pendek", "0812345", "", ErrTeleponTidakValid},
		{"14 digit nasional (terlalu panjang)", "012345678901234", "", ErrTeleponTidakValid},
		{"terlalu panjang", "+6281234567890123", "", ErrTeleponTidakValid},
		{"huruf di tengah", "0812abc34567", "", ErrTeleponTidakValid},
		{"huruf di akhir", "08123456789a", "", ErrTeleponTidakValid},
		{"hanya pemisah, bukan kosong", "-", "", ErrTeleponTidakValid},
		{"hanya kurung", "()", "", ErrTeleponTidakValid},
		{"plus di tengah", "0812+3456789", "", ErrTeleponTidakValid},
		{"garis miring", "0812/3456789", "", ErrTeleponTidakValid},
		{"digit non-ASCII", "０８１２３４５６７８９０", "", ErrTeleponTidakValid},
		{"awalan + selain +62", "+0812345678", "", ErrTeleponTidakValid},
		{"NUL", "0812345\x00678", "", ErrTeleponTidakValid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalisasiTelepon(tc.mentah)

			if tc.galat != nil {
				require.ErrorIs(t, err, tc.galat)
				assert.Empty(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// Hasil normalisasi harus stabil: menormalkan ulang nomor yang sudah E.164 tidak mengubahnya.
func TestNormalisasiTelepon_Idempoten(t *testing.T) {
	for _, mentah := range []string{"0812-3456-7890", "62 812 3456 7890", "(0651) 123-456", "+62 81234567890"} {
		sekali, err := NormalisasiTelepon(mentah)
		require.NoError(t, err, mentah)
		dua, err := NormalisasiTelepon(sekali)
		require.NoError(t, err, sekali)
		assert.Equal(t, sekali, dua, mentah)
	}
}

func TestNormalisasiNama(t *testing.T) {
	cases := []struct {
		name   string
		mentah string
		want   string
		galat  error
	}{
		{"biasa", "Budi Santoso", "Budi Santoso", nil},
		{"dipangkas", "  Budi Santoso \t", "Budi Santoso", nil},
		{"batas bawah 3 rune", "Ani", "Ani", nil},
		{"3 rune multibita", "日本語", "日本語", nil},
		{"spasi di tengah dihitung", " a b ", "a b", nil},
		{"100 rune", strings.Repeat("a", 100), strings.Repeat("a", 100), nil},
		{"100 rune multibita (200 byte)", strings.Repeat("é", 100), strings.Repeat("é", 100), nil},
		{"100 emoji (400 byte)", strings.Repeat("😀", 100), strings.Repeat("😀", 100), nil},
		{"NUL dan UTF-8 rusak dibuang", "Bu\x00di\xff Santoso", "Budi Santoso", nil},

		{"kosong", "", "", ErrNamaTerlaluPendek},
		{"spasi semua", "      ", "", ErrNamaTerlaluPendek},
		{"tab dan baris baru semua", "\t\n\r", "", ErrNamaTerlaluPendek},
		{"2 rune", "Al", "", ErrNamaTerlaluPendek},
		{"2 rune dengan spasi tepi", "   Al   ", "", ErrNamaTerlaluPendek},
		{"2 rune multibita", "日本", "", ErrNamaTerlaluPendek},
		{"hanya NUL", "\x00\x00\x00", "", ErrNamaTerlaluPendek},
		{"101 rune", strings.Repeat("a", 101), "", ErrNamaTerlaluPanjang},
		{"101 rune multibita", strings.Repeat("é", 101), "", ErrNamaTerlaluPanjang},
		{"101 rune sesudah dipangkas", "  " + strings.Repeat("a", 101) + "  ", "", ErrNamaTerlaluPanjang},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalisasiNama(tc.mentah)

			if tc.galat != nil {
				require.ErrorIs(t, err, tc.galat)
				assert.Empty(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// ── UpdateProfil ──────────────────────────────────────────────────────

// penggunaProfil adalah akun uji dengan telepon tersimpan.
func penggunaProfil(telepon *string) *domain.Pengguna {
	u := activeUser()
	u.Telepon = telepon
	return u
}

// patchDitulis mengambil PenggunaPatch dari panggilan PatchUser (harus tepat satu).
func patchDitulis(t *testing.T, repo *mockRepository) domain.PenggunaPatch {
	t.Helper()
	var patches []domain.PenggunaPatch
	for _, c := range repo.Calls {
		if c.Method == "PatchUser" {
			patches = append(patches, c.Arguments.Get(2).(domain.PenggunaPatch))
		}
	}
	require.Len(t, patches, 1, "PatchUser harus dipanggil tepat sekali")
	return patches[0]
}

// tegaskanHanyaProfil memastikan patch tidak menyentuh kolom di luar kewenangan pemilik akun.
func tegaskanHanyaProfil(t *testing.T, p domain.PenggunaPatch) {
	t.Helper()
	assert.Nil(t, p.Email)
	assert.Nil(t, p.Peran)
	assert.Nil(t, p.Lapis)
	assert.Nil(t, p.Lembaga)
	assert.Nil(t, p.KabupatenKode)
	assert.Nil(t, p.Status)
}

func tegaskanTidakMenulis(t *testing.T, repo *mockRepository, recorder *audittest.Recorder) {
	t.Helper()
	repo.AssertNotCalled(t, "PatchUser", mock.Anything, mock.Anything, mock.Anything)
	assert.Empty(t, recorder.Entries(), "tanpa tulis berarti tanpa audit")
}

func TestUpdateProfil_UbahNamaSaja(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	repo.On("GetUserByID", mock.Anything, uid("user-123")).Return(penggunaProfil(ptrStr("+6281200000001")), nil)
	repo.On("PatchUser", mock.Anything, uid("user-123"), mock.Anything).Return(nil)

	hasil, err := uc.UpdateProfil(context.Background(), uid("user-123"), ProfilPatch{Nama: ptrStr("  Nama Baru  ")})

	require.NoError(t, err)
	patch := patchDitulis(t, repo)
	if assert.NotNil(t, patch.Nama) {
		assert.Equal(t, "Nama Baru", *patch.Nama, "nama dipangkas")
	}
	assert.Nil(t, patch.Telepon, "telepon tidak dikirim, tidak disentuh")
	tegaskanHanyaProfil(t, patch)
	assert.Equal(t, "Nama Baru", hasil.Nama)
	require.NotNil(t, hasil.Telepon)
	assert.Equal(t, "+6281200000001", *hasil.Telepon)
	entry, ok := recorder.Last()
	require.True(t, ok)
	assert.Equal(t, "UPDATE_PROFIL", entry.Aksi)
	// Snapshot audit menyamarkan telepon (UU PDP): audit_log append-only dan dibaca banyak pihak.
	assert.Equal(t, "+62********001", *entry.Sebelum.(map[string]any)["telepon"].(*string))
	assert.Equal(t, "+62********001", *entry.Sesudah.(map[string]any)["telepon"].(*string), "telepon tidak berubah")
	assert.NotContains(t, entry.Metadata, "telepon_diubah", "nomor tidak berganti, tidak ada penanda")
}

func TestUpdateProfil_UbahTeleponSaja(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCaseWithAudit(repo)
	repo.On("GetUserByID", mock.Anything, uid("user-123")).Return(penggunaProfil(nil), nil)
	repo.On("PatchUser", mock.Anything, uid("user-123"), mock.Anything).Return(nil)

	hasil, err := uc.UpdateProfil(context.Background(), uid("user-123"), ProfilPatch{Telepon: ptrStr("0812-3456-7890")})

	require.NoError(t, err)
	patch := patchDitulis(t, repo)
	assert.Nil(t, patch.Nama, "nama tidak dikirim, tidak disentuh")
	if assert.NotNil(t, patch.Telepon) {
		assert.Equal(t, "+6281234567890", *patch.Telepon, "telepon dinormalkan sebelum ditulis")
	}
	tegaskanHanyaProfil(t, patch)
	assert.Equal(t, "Test User", hasil.Nama)
	require.NotNil(t, hasil.Telepon)
	assert.Equal(t, "+6281234567890", *hasil.Telepon)
}

func TestUpdateProfil_UbahKeduanya(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCaseWithAudit(repo)
	repo.On("GetUserByID", mock.Anything, uid("user-123")).Return(penggunaProfil(ptrStr("+6281200000001")), nil)
	repo.On("PatchUser", mock.Anything, uid("user-123"), mock.Anything).Return(nil)

	hasil, err := uc.UpdateProfil(context.Background(), uid("user-123"),
		ProfilPatch{Nama: ptrStr("Siti Aminah"), Telepon: ptrStr("+62 857-1234-5678")})

	require.NoError(t, err)
	patch := patchDitulis(t, repo)
	require.NotNil(t, patch.Nama)
	require.NotNil(t, patch.Telepon)
	assert.Equal(t, "Siti Aminah", *patch.Nama)
	assert.Equal(t, "+6285712345678", *patch.Telepon)
	tegaskanHanyaProfil(t, patch)
	assert.Equal(t, "Siti Aminah", hasil.Nama)
	assert.Equal(t, "+6285712345678", *hasil.Telepon)
}

func TestUpdateProfil_HapusTeleponMenulisNULL(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	repo.On("GetUserByID", mock.Anything, uid("user-123")).Return(penggunaProfil(ptrStr("+6281200000001")), nil)
	repo.On("PatchUser", mock.Anything, uid("user-123"), mock.Anything).Return(nil)

	hasil, err := uc.UpdateProfil(context.Background(), uid("user-123"), ProfilPatch{Telepon: ptrStr("")})

	require.NoError(t, err)
	assert.Nil(t, hasil.Telepon, "nomor dihapus: nil, bukan pointer ke string kosong")
	patch := patchDitulis(t, repo)
	if assert.NotNil(t, patch.Telepon) {
		assert.Empty(t, *patch.Telepon, "patch membawa \"\" yang oleh repositori ditulis sebagai NULL")
	}
	assert.Equal(t, map[string]any{"telepon": nil}, patch.Kolom(), "kolom ditulis NULL, bukan string kosong")
	entry, ok := recorder.Last()
	require.True(t, ok)
	assert.NotNil(t, entry.Sebelum.(map[string]any)["telepon"])
	assert.Nil(t, entry.Sesudah.(map[string]any)["telepon"])
}

func TestUpdateProfil_HapusTeleponDenganSpasiSemuaJugaMenghapus(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCaseWithAudit(repo)
	repo.On("GetUserByID", mock.Anything, uid("user-123")).Return(penggunaProfil(ptrStr("+6281200000001")), nil)
	repo.On("PatchUser", mock.Anything, uid("user-123"), mock.Anything).Return(nil)

	hasil, err := uc.UpdateProfil(context.Background(), uid("user-123"), ProfilPatch{Telepon: ptrStr("   ")})

	require.NoError(t, err)
	assert.Nil(t, hasil.Telepon)
}

func TestUpdateProfil_ValidasiMenolakTanpaMenyentuhBasisData(t *testing.T) {
	cases := []struct {
		name  string
		patch ProfilPatch
		galat error
	}{
		{"badan kosong", ProfilPatch{}, ErrTidakAdaPerubahan},
		{"nama kosong", ProfilPatch{Nama: ptrStr("")}, ErrNamaTerlaluPendek},
		{"nama spasi semua", ProfilPatch{Nama: ptrStr("     ")}, ErrNamaTerlaluPendek},
		{"nama 2 karakter", ProfilPatch{Nama: ptrStr("Al")}, ErrNamaTerlaluPendek},
		{"nama 2 karakter berspasi tepi", ProfilPatch{Nama: ptrStr("  Al  ")}, ErrNamaTerlaluPendek},
		{"nama 101 karakter", ProfilPatch{Nama: ptrStr(strings.Repeat("x", 101))}, ErrNamaTerlaluPanjang},
		{"telepon kode negara lain", ProfilPatch{Telepon: ptrStr("+1 415 555 2671")}, ErrTeleponTidakValid},
		{"telepon tanpa awalan", ProfilPatch{Telepon: ptrStr("8123456789")}, ErrTeleponTidakValid},
		{"telepon terlalu pendek", ProfilPatch{Telepon: ptrStr("0812345")}, ErrTeleponTidakValid},
		{"telepon terlalu panjang", ProfilPatch{Telepon: ptrStr("0812345678901234")}, ErrTeleponTidakValid},
		{"telepon berhuruf", ProfilPatch{Telepon: ptrStr("0812abc34567")}, ErrTeleponTidakValid},
		{"telepon +620", ProfilPatch{Telepon: ptrStr("+6208123456789")}, ErrTeleponTidakValid},
		{"nama sah tetapi telepon tidak: tidak ada yang ditulis", ProfilPatch{Nama: ptrStr("Nama Sah"), Telepon: ptrStr("abc")}, ErrTeleponTidakValid},
		{"telepon sah tetapi nama tidak: tidak ada yang ditulis", ProfilPatch{Nama: ptrStr("ab"), Telepon: ptrStr("081234567890")}, ErrNamaTerlaluPendek},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := new(mockRepository)
			uc, recorder := newTestUseCaseWithAudit(repo)

			hasil, err := uc.UpdateProfil(context.Background(), uid("user-123"), tc.patch)

			require.ErrorIs(t, err, tc.galat)
			assert.Nil(t, hasil)
			repo.AssertNotCalled(t, "GetUserByID", mock.Anything, mock.Anything)
			tegaskanTidakMenulis(t, repo, recorder)
		})
	}
}

func TestUpdateProfil_NilaiSamaTidakMenulisDanTidakMengaudit(t *testing.T) {
	cases := []struct {
		name  string
		patch ProfilPatch
	}{
		{"nama sama", ProfilPatch{Nama: ptrStr("Test User")}},
		{"nama sama sesudah dipangkas", ProfilPatch{Nama: ptrStr("  Test User  ")}},
		{"telepon sama", ProfilPatch{Telepon: ptrStr("+6281200000001")}},
		{"telepon sama sesudah normalisasi", ProfilPatch{Telepon: ptrStr("0812-0000-0001")}},
		{"keduanya sama", ProfilPatch{Nama: ptrStr("Test User"), Telepon: ptrStr("62 812 0000 0001")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := new(mockRepository)
			uc, recorder := newTestUseCaseWithAudit(repo)
			repo.On("GetUserByID", mock.Anything, uid("user-123")).Return(penggunaProfil(ptrStr("+6281200000001")), nil)

			hasil, err := uc.UpdateProfil(context.Background(), uid("user-123"), tc.patch)

			require.NoError(t, err)
			require.NotNil(t, hasil)
			assert.Equal(t, "Test User", hasil.Nama)
			require.NotNil(t, hasil.Telepon)
			assert.Equal(t, "+6281200000001", *hasil.Telepon)
			tegaskanTidakMenulis(t, repo, recorder)
		})
	}
}

func TestUpdateProfil_HapusTeleponYangMemangKosongTidakMenulis(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	repo.On("GetUserByID", mock.Anything, uid("user-123")).Return(penggunaProfil(nil), nil)

	hasil, err := uc.UpdateProfil(context.Background(), uid("user-123"), ProfilPatch{Telepon: ptrStr("")})

	require.NoError(t, err)
	assert.Nil(t, hasil.Telepon)
	tegaskanTidakMenulis(t, repo, recorder)
}

func TestUpdateProfil_HanyaKolomYangBerbedaDitulis(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	repo.On("GetUserByID", mock.Anything, uid("user-123")).Return(penggunaProfil(ptrStr("+6281200000001")), nil)
	repo.On("PatchUser", mock.Anything, uid("user-123"), mock.Anything).Return(nil)

	// Nama sama dengan yang tersimpan, telepon berbeda: patch hanya memuat telepon.
	_, err := uc.UpdateProfil(context.Background(), uid("user-123"),
		ProfilPatch{Nama: ptrStr("Test User"), Telepon: ptrStr("081299998888")})

	require.NoError(t, err)
	patch := patchDitulis(t, repo)
	assert.Nil(t, patch.Nama, "nama tidak berubah, tidak ditulis")
	require.NotNil(t, patch.Telepon)
	assert.Equal(t, "+6281299998888", *patch.Telepon)
	entry, _ := recorder.Last()
	assert.Equal(t, "Test User", entry.Sebelum.(map[string]any)["nama"])
	assert.Equal(t, "Test User", entry.Sesudah.(map[string]any)["nama"])
}

func TestUpdateProfil_AuditMemuatSebelumDanSesudahDenganTelepon(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	repo.On("GetUserByID", mock.Anything, uid("user-123")).Return(penggunaProfil(ptrStr("+6281200000001")), nil)
	repo.On("PatchUser", mock.Anything, uid("user-123"), mock.Anything).Return(nil)

	_, err := uc.UpdateProfil(context.Background(), uid("user-123"),
		ProfilPatch{Nama: ptrStr("Nama Baru"), Telepon: ptrStr("0857 1234 5678")})

	require.NoError(t, err)
	entries := recorder.Entries()
	require.Len(t, entries, 1)
	e := entries[0]
	assert.Equal(t, "UPDATE_PROFIL", e.Aksi)
	assert.Equal(t, "pengguna", e.Entitas)
	assert.Equal(t, uid("user-123"), e.EntitasID, "entitas = pengguna itu sendiri")
	assert.Equal(t, uid("user-123"), e.AktorID, "pelaku = pemilik akun")
	assert.Equal(t, audit.HasilBerhasil, e.Hasil)

	sebelum, sesudah := e.Sebelum.(map[string]any), e.Sesudah.(map[string]any)
	assert.Equal(t, "Test User", sebelum["nama"])
	assert.Equal(t, "Nama Baru", sesudah["nama"])
	require.IsType(t, (*string)(nil), sebelum["telepon"])
	// Nomor di snapshot audit tersamar (UU PDP); perubahannya ditandai metadata telepon_diubah.
	assert.Equal(t, "+62********001", *sebelum["telepon"].(*string))
	assert.Equal(t, "+62********678", *sesudah["telepon"].(*string))
	assert.Equal(t, true, e.Metadata["telepon_diubah"], "nomor berganti, ditandai di metadata audit")
	// Kolom lain tidak berubah antara sebelum dan sesudah.
	for _, kolom := range []string{"email", "peran", "lapis", "lembaga", "status"} {
		assert.Equal(t, sebelum[kolom], sesudah[kolom], kolom)
	}
}

func TestUpdateProfil_PenggunaTidakAda(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	repo.On("GetUserByID", mock.Anything, uid("hilang")).Return(nil, fmt.Errorf("get user by id: %w", gorm.ErrRecordNotFound))

	_, err := uc.UpdateProfil(context.Background(), uid("hilang"), ProfilPatch{Nama: ptrStr("Nama Baru")})

	require.ErrorIs(t, err, ErrUserNotFound)
	tegaskanTidakMenulis(t, repo, recorder)
}

func TestUpdateProfil_PenggunaHilangDiTengahPenulisan(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	repo.On("GetUserByID", mock.Anything, uid("user-123")).Return(penggunaProfil(nil), nil)
	repo.On("PatchUser", mock.Anything, uid("user-123"), mock.Anything).Return(fmt.Errorf("get user for update: %w", gorm.ErrRecordNotFound))

	_, err := uc.UpdateProfil(context.Background(), uid("user-123"), ProfilPatch{Nama: ptrStr("Nama Baru")})

	require.ErrorIs(t, err, ErrUserNotFound)
	assert.Empty(t, recorder.Entries(), "penulisan gagal tidak diaudit")
}

func TestUpdateProfil_GalatBasisDataBukanPenggunaHilang(t *testing.T) {
	t.Run("saat membaca", func(t *testing.T) {
		repo := new(mockRepository)
		uc, recorder := newTestUseCaseWithAudit(repo)
		repo.On("GetUserByID", mock.Anything, uid("user-123")).Return(nil, errors.New("koneksi putus"))

		_, err := uc.UpdateProfil(context.Background(), uid("user-123"), ProfilPatch{Nama: ptrStr("Nama Baru")})

		require.Error(t, err)
		assert.NotErrorIs(t, err, ErrUserNotFound, "galat basis data bukan pengguna hilang")
		tegaskanTidakMenulis(t, repo, recorder)
	})
	t.Run("saat menulis", func(t *testing.T) {
		repo := new(mockRepository)
		uc, recorder := newTestUseCaseWithAudit(repo)
		repo.On("GetUserByID", mock.Anything, uid("user-123")).Return(penggunaProfil(nil), nil)
		repo.On("PatchUser", mock.Anything, uid("user-123"), mock.Anything).Return(errors.New("koneksi putus"))

		_, err := uc.UpdateProfil(context.Background(), uid("user-123"), ProfilPatch{Nama: ptrStr("Nama Baru")})

		require.Error(t, err)
		assert.NotErrorIs(t, err, ErrUserNotFound)
		assert.Empty(t, recorder.Entries(), "penulisan gagal tidak diaudit")
	})
}

func TestUpdateProfil_KegagalanAuditTidakMenggagalkanPerubahan(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	recorder.Err = errors.New("db mati")
	repo.On("GetUserByID", mock.Anything, uid("user-123")).Return(penggunaProfil(nil), nil)
	repo.On("PatchUser", mock.Anything, uid("user-123"), mock.Anything).Return(nil)

	hasil, err := uc.UpdateProfil(context.Background(), uid("user-123"), ProfilPatch{Nama: ptrStr("Nama Baru")})

	require.NoError(t, err, "perubahan sudah tersimpan; kegagalan audit hanya dilaporkan di log")
	assert.Equal(t, "Nama Baru", hasil.Nama)
}
