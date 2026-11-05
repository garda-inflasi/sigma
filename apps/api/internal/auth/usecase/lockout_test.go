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
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/pkg/crypto"
	"github.com/garda-inflasi/api/pkg/totp"
)

// jamBeku membekukan jam lockout di awal dan mengembalikan penunjuknya; geser dengan *p = p.Add(...).
func jamBeku(t *testing.T, awal time.Time) *time.Time {
	t.Helper()
	sekarang := awal
	jam = func() time.Time { return sekarang }
	t.Cleanup(func() { jam = time.Now })
	return &sekarang
}

var awalUji = time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)

const (
	emailUji   = "test@example.com"
	sandiBenar = "Test1234!"
	sandiSalah = "Salah1234!"
)

// siapLogin menyiapkan mock untuk alur login penuh (sukses maupun gagal) bagi satu pengguna.
func siapLogin(repo *mockRepository, user *domain.Pengguna) {
	repo.On("GetUserByEmail", mock.Anything, mock.Anything).Return(user, nil)
	repo.On("CreateLoginAttempt", mock.Anything, mock.Anything).Return(nil)
	repo.On("CreateRefreshToken", mock.Anything, mock.Anything).Return(nil)
	repo.On("UpdateUser", mock.Anything, mock.Anything).Return(nil)
}

func login(uc *UseCase, email, sandi string) error {
	_, err := uc.Login(context.Background(), email, sandi, "10.0.0.1")
	return err
}

func TestLockout_AmbangDanAkhirKunci(t *testing.T) {
	sekarang := jamBeku(t, awalUji)
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	siapLogin(repo, activeUser())

	// Lima kata sandi salah, satu menit terpisah. Yang kelima masih dijawab 401 (kunci terpasang sesudahnya).
	for i := 1; i <= 5; i++ {
		require.ErrorIs(t, login(uc, emailUji, sandiSalah), ErrInvalidCredentials, "percobaan %d", i)
		if i < 5 {
			*sekarang = sekarang.Add(time.Minute)
		}
	}
	terakhirGagal := *sekarang
	sampai := terakhirGagal.Add(15 * time.Minute)
	keadaan := repo.keadaan(emailUji)
	require.NotNil(t, keadaan)
	require.NotNil(t, keadaan.TerkunciSampai)
	assert.Equal(t, sampai, *keadaan.TerkunciSampai, "kunci berlaku 15 menit sejak kegagalan ke-5")

	// Kata sandi yang benar pun ditolak, dengan akhir kunci dan sisa detik.
	*sekarang = terakhirGagal.Add(time.Minute)
	err := login(uc, emailUji, sandiBenar)
	require.ErrorIs(t, err, ErrAccountLocked)
	var terkunci *AkunTerkunciError
	require.ErrorAs(t, err, &terkunci)
	assert.Equal(t, sampai, terkunci.Sampai)
	assert.Equal(t, 14*60, terkunci.SisaDetik)

	// Percobaan selama terkunci tidak memperpanjang kunci, seberapa pun sering dan dengan kata sandi apa pun.
	for _, selisih := range []time.Duration{2 * time.Minute, 7 * time.Minute, 14 * time.Minute} {
		*sekarang = terakhirGagal.Add(selisih)
		for _, sandi := range []string{sandiSalah, sandiBenar} {
			err := login(uc, emailUji, sandi)
			require.ErrorAs(t, err, &terkunci)
			assert.Equal(t, sampai, terkunci.Sampai, "kunci tetap berakhir di waktu yang sama")
		}
	}
	assert.Equal(t, 5, repo.keadaan(emailUji).Gagal, "percobaan yang ditolak tidak menambah hitungan")

	// Tepat pada akhir kunci akun terbuka lagi dan kata sandi benar berhasil.
	*sekarang = sampai
	_, err = uc.Login(context.Background(), emailUji, sandiBenar, "10.0.0.1")
	require.NoError(t, err)
}

func TestLockout_KedaluwarsaMemulaiHitunganBaru(t *testing.T) {
	sekarang := jamBeku(t, awalUji)
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	siapLogin(repo, activeUser())

	for i := 0; i < 5; i++ {
		require.ErrorIs(t, login(uc, emailUji, sandiSalah), ErrInvalidCredentials)
	}
	*sekarang = sekarang.Add(15 * time.Minute) // kunci berakhir

	// Hitungan dari nol: empat kegagalan baru belum mengunci, yang kelima baru mengunci.
	for i := 1; i <= 4; i++ {
		require.ErrorIs(t, login(uc, emailUji, sandiSalah), ErrInvalidCredentials, "percobaan %d setelah kunci berakhir", i)
	}
	require.ErrorIs(t, login(uc, emailUji, sandiSalah), ErrInvalidCredentials)
	require.ErrorIs(t, login(uc, emailUji, sandiBenar), ErrAccountLocked)
}

func TestLockout_JendelaKedaluwarsaMenghapusKegagalanLama(t *testing.T) {
	sekarang := jamBeku(t, awalUji)
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	siapLogin(repo, activeUser())

	for i := 0; i < 4; i++ {
		require.ErrorIs(t, login(uc, emailUji, sandiSalah), ErrInvalidCredentials)
	}
	*sekarang = sekarang.Add(15 * time.Minute)
	// Empat kegagalan lama sudah di luar jendela: satu kegagalan baru tidak mengunci, dan kata sandi benar berhasil.
	require.ErrorIs(t, login(uc, emailUji, sandiSalah), ErrInvalidCredentials)
	require.NoError(t, login(uc, emailUji, sandiBenar))
}

func TestLockout_LoginBerhasilMenghapusHitungan(t *testing.T) {
	jamBeku(t, awalUji)
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	siapLogin(repo, activeUser())

	for i := 0; i < 4; i++ {
		require.ErrorIs(t, login(uc, emailUji, sandiSalah), ErrInvalidCredentials)
	}
	require.NoError(t, login(uc, emailUji, sandiBenar))
	assert.Nil(t, repo.keadaan(emailUji), "login berhasil menghapus hitungan")

	// Empat kegagalan lagi tidak mengunci (tanpa reset, kegagalan kelima di atas akan mengunci).
	for i := 0; i < 4; i++ {
		require.ErrorIs(t, login(uc, emailUji, sandiSalah), ErrInvalidCredentials)
	}
	require.NoError(t, login(uc, emailUji, sandiBenar))
}

func TestLockout_PercobaanKeLimaDenganKataSandiBenarBerhasil(t *testing.T) {
	jamBeku(t, awalUji)
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	siapLogin(repo, activeUser())

	for i := 0; i < 4; i++ {
		require.ErrorIs(t, login(uc, emailUji, sandiSalah), ErrInvalidCredentials)
	}
	require.NoError(t, login(uc, emailUji, sandiBenar), "kata sandi benar pada percobaan ke-5 bukan kegagalan")
	assert.Nil(t, repo.keadaan(emailUji))
}

func TestLockout_EmailTerdaftarDanTakTerdaftarBerjawabanSama(t *testing.T) {
	sekarang := jamBeku(t, awalUji)

	rangkaian := func(email string, ada bool) []string {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		repo.On("CreateLoginAttempt", mock.Anything, mock.Anything).Return(nil)
		if ada {
			repo.On("GetUserByEmail", mock.Anything, mock.Anything).Return(activeUser(), nil)
		} else {
			repo.On("GetUserByEmail", mock.Anything, mock.Anything).Return(nil, gorm.ErrRecordNotFound)
		}
		*sekarang = awalUji

		var hasil []string
		for i := 1; i <= 8; i++ {
			err := login(uc, email, sandiSalah)
			var terkunci *AkunTerkunciError
			switch {
			case errors.As(err, &terkunci):
				hasil = append(hasil, fmt.Sprintf("terkunci sampai %s sisa %d", terkunci.Sampai.Format(time.RFC3339), terkunci.SisaDetik))
			case errors.Is(err, ErrInvalidCredentials):
				hasil = append(hasil, "kredensial salah")
			default:
				hasil = append(hasil, "lain: "+err.Error())
			}
			*sekarang = sekarang.Add(10 * time.Second)
		}
		return hasil
	}

	terdaftar := rangkaian(emailUji, true)
	takTerdaftar := rangkaian("hantu@example.com", false)

	assert.Equal(t, terdaftar, takTerdaftar, "urutan jawaban harus identik")
	assert.Equal(t, "kredensial salah", terdaftar[4], "percobaan ke-5 masih 401")
	assert.Contains(t, terdaftar[5], "terkunci sampai", "percobaan ke-6 terkunci")
}

func TestLockout_EmailTakTerdaftarMembandingkanDenganHashPalsu(t *testing.T) {
	// Waktu jawab email tak terdaftar harus mendekati yang terdaftar: keduanya memeriksa satu hash bcrypt.
	jamBeku(t, awalUji)
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	repo.On("CreateLoginAttempt", mock.Anything, mock.Anything).Return(nil)
	repo.On("GetUserByEmail", mock.Anything, mock.Anything).Return(nil, gorm.ErrRecordNotFound)

	mulai := time.Now()
	require.ErrorIs(t, login(uc, "hantu@example.com", sandiSalah), ErrInvalidCredentials)
	assert.Greater(t, time.Since(mulai), 50*time.Millisecond, "ada pekerjaan bcrypt untuk email tak dikenal")
}

func TestHashPalsuSamaCostDenganHashAsli(t *testing.T) {
	asli, err := crypto.HashPassword("apa-saja")
	require.NoError(t, err)
	costAsli, err := bcrypt.Cost([]byte(asli))
	require.NoError(t, err)
	costPalsu, err := bcrypt.Cost([]byte(hashPalsu))
	require.NoError(t, err)
	assert.Equal(t, costAsli, costPalsu, "hashPalsu harus dibuat ulang bila cost bcrypt berubah")
	assert.False(t, crypto.VerifyPassword(sandiBenar, hashPalsu))
}

func TestLockout_AkunNonaktif(t *testing.T) {
	t.Run("kata sandi salah dijawab sama dengan akun aktif dan dihitung", func(t *testing.T) {
		jamBeku(t, awalUji)
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		nonaktif := activeUser()
		nonaktif.Status = "NONAKTIF"
		siapLogin(repo, nonaktif)

		for i := 0; i < 5; i++ {
			require.ErrorIs(t, login(uc, emailUji, sandiSalah), ErrInvalidCredentials, "percobaan %d", i+1)
		}
		require.ErrorIs(t, login(uc, emailUji, sandiBenar), ErrAccountLocked, "akun nonaktif pun terkunci")
	})

	t.Run("kata sandi benar menyingkap nonaktif dan tidak dihitung", func(t *testing.T) {
		jamBeku(t, awalUji)
		repo := new(mockRepository)
		uc, recorder := newTestUseCaseWithAudit(repo)
		nonaktif := activeUser()
		nonaktif.Status = "NONAKTIF"
		siapLogin(repo, nonaktif)

		require.ErrorIs(t, login(uc, emailUji, sandiSalah), ErrInvalidCredentials)
		for i := 0; i < 10; i++ {
			require.ErrorIs(t, login(uc, emailUji, sandiBenar), ErrAccountInactive)
		}
		assert.Equal(t, 1, repo.keadaan(emailUji).Gagal, "hanya tebakan salah yang dihitung")
		last, _ := recorder.Last()
		assert.Equal(t, "account_inactive", last.Metadata["alasan"])
	})
}

func TestLockout_AkunTanpaKataSandiDiperlakukanSamaDenganYangLain(t *testing.T) {
	jamBeku(t, awalUji)
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	sso := activeUser()
	sso.PasswordHash = nil
	siapLogin(repo, sso)

	for i := 0; i < 5; i++ {
		require.ErrorIs(t, login(uc, emailUji, sandiBenar), ErrInvalidCredentials)
	}
	require.ErrorIs(t, login(uc, emailUji, sandiBenar), ErrAccountLocked)
}

func TestLockout_EmailDinormalisasi(t *testing.T) {
	jamBeku(t, awalUji)
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	siapLogin(repo, activeUser())

	// Variasi huruf dan spasi tepi menghitung ke kunci yang sama.
	for _, email := range []string{"test@example.com", "TEST@example.com", " Test@Example.com ", "test@EXAMPLE.com", "test@example.com"} {
		require.ErrorIs(t, login(uc, email, sandiSalah), ErrInvalidCredentials, email)
	}
	require.ErrorIs(t, login(uc, "TeSt@example.com", sandiBenar), ErrAccountLocked)
	assert.Equal(t, HashEmailKunci("test@example.com"), HashEmailKunci("  TEST@Example.COM"))
	assert.NotEqual(t, HashEmailKunci("test@example.com"), HashEmailKunci("lain@example.com"))
}

func TestLockout_KegagalanPenyimpananDitolakTertutup(t *testing.T) {
	t.Run("memesan percobaan gagal", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		repo.errMulai = errors.New("db mati")

		err := login(uc, emailUji, sandiBenar)

		require.Error(t, err)
		assert.NotErrorIs(t, err, ErrInvalidCredentials)
		assert.NotErrorIs(t, err, ErrAccountLocked)
		repo.AssertNotCalled(t, "GetUserByEmail", mock.Anything, mock.Anything)
	})
	t.Run("galat basis data saat mencari akun bukan kredensial salah dan tidak dihitung", func(t *testing.T) {
		jamBeku(t, awalUji)
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		repo.On("GetUserByEmail", mock.Anything, mock.Anything).Return(nil, errors.New("koneksi putus"))

		err := login(uc, emailUji, sandiBenar)

		require.Error(t, err)
		assert.NotErrorIs(t, err, ErrInvalidCredentials)
		assert.Equal(t, 0, repo.keadaan(emailUji).Gagal, "percobaan dikembalikan")
	})
}

func TestLockout_KonfigurasiNolMemakaiBawaan(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	uc.cfg.MaxFailedAttempts, uc.cfg.LockoutDuration = 0, 0

	maks, durasi := uc.batasKunci()

	assert.Equal(t, 5, maks)
	assert.Equal(t, 15*time.Minute, durasi)
	uc.cfg.MaxFailedAttempts, uc.cfg.LockoutDuration = 3, time.Minute
	maks, durasi = uc.batasKunci()
	assert.Equal(t, 3, maks)
	assert.Equal(t, time.Minute, durasi)
}

func TestLockout_BatasDariKonfigurasi(t *testing.T) {
	jamBeku(t, awalUji)
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	uc.cfg.MaxFailedAttempts = 2
	siapLogin(repo, activeUser())

	require.ErrorIs(t, login(uc, emailUji, sandiSalah), ErrInvalidCredentials)
	require.ErrorIs(t, login(uc, emailUji, sandiSalah), ErrInvalidCredentials)
	require.ErrorIs(t, login(uc, emailUji, sandiBenar), ErrAccountLocked)
}

// ── MFA ───────────────────────────────────────────────────────────────

func TestLockout_MFA_KataSandiBenarTidakMengubahHitungan(t *testing.T) {
	jamBeku(t, awalUji)
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	user, _ := userMFA(t, uc)
	siapLogin(repo, user)

	require.ErrorIs(t, login(uc, emailUji, sandiSalah), ErrInvalidCredentials)
	for i := 0; i < 10; i++ {
		res, err := uc.Login(context.Background(), emailUji, sandiBenar, "10.0.0.1")
		require.NoError(t, err)
		require.True(t, res.MFARequired)
	}

	keadaan := repo.keadaan(emailUji)
	require.NotNil(t, keadaan, "kata sandi benar yang menunggu OTP tidak menghapus hitungan")
	assert.Equal(t, 1, keadaan.Gagal, "dan tidak menambahnya")
}

func TestLockout_MFA_KodeSalahDanKataSandiSalahMemakaiKunciYangSama(t *testing.T) {
	sekarang := jamBeku(t, awalUji)
	repo := new(mockRepository)
	uc, _, _, _, challenge := siapMFALogin(t, repo)
	repo.On("GetUserByEmail", mock.Anything, mock.Anything).Return(repo.mfaUser(t, uc), nil)

	// Tiga kata sandi salah + dua kode OTP salah = lima kegagalan pada satu kunci.
	for i := 0; i < 3; i++ {
		require.ErrorIs(t, login(uc, emailUji, sandiSalah), ErrInvalidCredentials)
	}
	for i := 0; i < 2; i++ {
		_, err := uc.ValidateMFALogin(context.Background(), challenge, "000000", "10.0.0.1")
		require.ErrorIs(t, err, ErrMFACodeInvalid, "kode salah ke-%d", i+1)
	}

	_, err := uc.ValidateMFALogin(context.Background(), challenge, "000000", "10.0.0.1")
	require.ErrorIs(t, err, ErrAccountLocked)
	require.ErrorIs(t, login(uc, emailUji, sandiBenar), ErrAccountLocked, "login kata sandi ikut terkunci")

	var terkunci *AkunTerkunciError
	require.ErrorAs(t, err, &terkunci)
	assert.Equal(t, awalUji.Add(15*time.Minute), terkunci.Sampai)
	_ = sekarang
}

func TestLockout_MFA_KodeBenarKetikaTerkunciTetapDitolak(t *testing.T) {
	jamBeku(t, awalUji)
	repo := new(mockRepository)
	uc, _, _, secret, challenge := siapMFALogin(t, repo)
	repo.On("AdvanceMFAStep", mock.Anything, "user-123", mock.Anything).Return(true, nil).Maybe()

	for i := 0; i < 5; i++ {
		_, err := uc.ValidateMFALogin(context.Background(), challenge, "000000", "10.0.0.1")
		require.ErrorIs(t, err, ErrMFACodeInvalid, "kode salah ke-%d", i+1)
	}
	_, err := uc.ValidateMFALogin(context.Background(), challenge, kodeSekarang(t, secret), "10.0.0.1")

	require.ErrorIs(t, err, ErrAccountLocked)
	repo.AssertNotCalled(t, "AdvanceMFAStep", mock.Anything, mock.Anything, mock.Anything)
}

func TestLockout_MFA_LoginLengkapMenghapusHitungan(t *testing.T) {
	jamBeku(t, awalUji)
	repo := new(mockRepository)
	uc, _, _, secret, challenge := siapMFALogin(t, repo)
	repo.On("AdvanceMFAStep", mock.Anything, "user-123", mock.Anything).Return(true, nil)

	for i := 0; i < 3; i++ {
		_, err := uc.ValidateMFALogin(context.Background(), challenge, "000000", "10.0.0.1")
		require.ErrorIs(t, err, ErrMFACodeInvalid)
	}
	require.NotNil(t, repo.keadaan(emailUji))

	_, err := uc.ValidateMFALogin(context.Background(), challenge, kodeSekarang(t, secret), "10.0.0.1")

	require.NoError(t, err)
	assert.Nil(t, repo.keadaan(emailUji), "OTP lolos = login lengkap = hitungan dihapus")
}

func TestLockout_MFA_GalatBukanTebakanTidakDihitung(t *testing.T) {
	jamBeku(t, awalUji)
	repo := new(mockRepository)
	uc, _, _, secret, challenge := siapMFALogin(t, repo)
	repo.On("AdvanceMFAStep", mock.Anything, "user-123", mock.Anything).Return(false, errors.New("db putus"))

	_, err := uc.ValidateMFALogin(context.Background(), challenge, kodeSekarang(t, secret), "10.0.0.1")

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrMFACodeInvalid)
	assert.Equal(t, 0, repo.keadaan(emailUji).Gagal)
}

func TestLockout_MFA_SetupDanGantiKataSandi(t *testing.T) {
	t.Run("kode setup yang salah dihitung dan mengunci", func(t *testing.T) {
		jamBeku(t, awalUji)
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		user := activeUser()
		sealed, _ := uc.cfg.MFAKey.Seal(totp.GenerateSecret())
		user.MfaSecret = &sealed
		repo.On("GetUserByID", mock.Anything, "user-123").Return(user, nil)
		repo.On("CreateLoginAttempt", mock.Anything, mock.Anything).Return(nil)

		for i := 0; i < 5; i++ {
			_, err := uc.VerifyMFA(context.Background(), "user-123", "000000", "10.0.0.1")
			require.ErrorIs(t, err, ErrMFACodeInvalid)
		}
		_, err := uc.VerifyMFA(context.Background(), "user-123", "000000", "10.0.0.1")
		require.ErrorIs(t, err, ErrAccountLocked)
	})
	t.Run("kata sandi lama yang salah dihitung dan mengunci; yang benar tidak dihitung", func(t *testing.T) {
		jamBeku(t, awalUji)
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)
		repo.On("CreateLoginAttempt", mock.Anything, mock.Anything).Return(nil)

		for i := 0; i < 4; i++ {
			_, _, err := uc.ChangePassword(context.Background(), "user-123", sandiSalah, "BaruLagi99!", "10.0.0.1")
			require.ErrorIs(t, err, ErrWrongPassword)
		}
		// Kata sandi lama benar (kata sandi baru lemah supaya berhenti sebelum menulis): tidak menambah hitungan.
		_, _, err := uc.ChangePassword(context.Background(), "user-123", sandiBenar, "lemah", "10.0.0.1")
		require.ErrorIs(t, err, ErrWeakPassword)
		assert.Equal(t, 4, repo.keadaan(emailUji).Gagal)

		_, _, err = uc.ChangePassword(context.Background(), "user-123", sandiSalah, "BaruLagi99!", "10.0.0.1")
		require.ErrorIs(t, err, ErrWrongPassword)
		_, _, err = uc.ChangePassword(context.Background(), "user-123", sandiBenar, "BaruLagi99!", "10.0.0.1")
		require.ErrorIs(t, err, ErrAccountLocked)
	})
}

// mfaUser mengembalikan pengguna ber-MFA yang sama dengan yang dipakai siapMFALogin (GetUserByID).
func (m *mockRepository) mfaUser(t *testing.T, uc *UseCase) *domain.Pengguna {
	t.Helper()
	pengguna, err := m.GetUserByID(context.Background(), "user-123")
	require.NoError(t, err)
	return pengguna
}

// ── Audit ─────────────────────────────────────────────────────────────

func TestLockout_PercobaanSelamaTerkunciDiaudit(t *testing.T) {
	jamBeku(t, awalUji)
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	siapLogin(repo, activeUser())
	repo.kunciTerkunci(emailUji)

	require.ErrorIs(t, login(uc, emailUji, sandiBenar), ErrAccountLocked)

	entries := recorder.Entries()
	require.Len(t, entries, 1)
	assert.Equal(t, "LOGIN", entries[0].Aksi)
	assert.Equal(t, audit.HasilGagal, entries[0].Hasil)
	assert.Equal(t, "user-123", entries[0].AktorID)
	assert.Equal(t, "account_locked", entries[0].Metadata["alasan"])
}

func TestLockout_AuditSamaUntukEmailTakTerdaftarSelamaTerkunci(t *testing.T) {
	jamBeku(t, awalUji)
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	repo.On("GetUserByEmail", mock.Anything, mock.Anything).Return(nil, gorm.ErrRecordNotFound)
	repo.kunciTerkunci("hantu@example.com")

	require.ErrorIs(t, login(uc, "hantu@example.com", sandiBenar), ErrAccountLocked)

	entries := recorder.Entries()
	require.Len(t, entries, 1)
	assert.Empty(t, entries[0].AktorID)
	assert.Equal(t, "account_locked", entries[0].Metadata["alasan"])
}

// ── Buka kunci oleh admin ─────────────────────────────────────────────

func TestBukaKunciPengguna(t *testing.T) {
	t.Run("membuka kunci yang aktif dan mengaudit", func(t *testing.T) {
		jamBeku(t, awalUji)
		repo := new(mockRepository)
		uc, recorder := newTestUseCaseWithAudit(repo)
		repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)
		repo.kunciTerkunci(emailUji)

		hasil, err := uc.BukaKunciPengguna(context.Background(), "user-123", "admin-1")

		require.NoError(t, err)
		assert.True(t, hasil.TerkunciSebelumnya)
		assert.Nil(t, repo.keadaan(emailUji))
		entry, ok := recorder.Last()
		require.True(t, ok)
		assert.Equal(t, "UNLOCK_USER", entry.Aksi)
		assert.Equal(t, "admin-1", entry.AktorID)
		assert.Equal(t, "pengguna", entry.Entitas)
		assert.Equal(t, "user-123", entry.EntitasID)
		assert.Equal(t, audit.HasilBerhasil, entry.Hasil)
		assert.Equal(t, true, entry.Metadata["terkunci_sebelumnya"])
	})
	t.Run("pengguna yang bisa masuk lagi sesudah dibuka", func(t *testing.T) {
		jamBeku(t, awalUji)
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		siapLogin(repo, activeUser())
		repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)
		for i := 0; i < 5; i++ {
			require.ErrorIs(t, login(uc, emailUji, sandiSalah), ErrInvalidCredentials)
		}
		require.ErrorIs(t, login(uc, emailUji, sandiBenar), ErrAccountLocked)

		_, err := uc.BukaKunciPengguna(context.Background(), "user-123", "admin-1")
		require.NoError(t, err)

		require.NoError(t, login(uc, emailUji, sandiBenar))
	})
	t.Run("akun yang tidak terkunci: tetap berhasil, hitungan dihapus", func(t *testing.T) {
		jamBeku(t, awalUji)
		repo := new(mockRepository)
		uc, recorder := newTestUseCaseWithAudit(repo)
		repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)

		hasil, err := uc.BukaKunciPengguna(context.Background(), "user-123", "admin-1")

		require.NoError(t, err)
		assert.False(t, hasil.TerkunciSebelumnya)
		entry, _ := recorder.Last()
		assert.Equal(t, false, entry.Metadata["terkunci_sebelumnya"])
	})
	t.Run("kunci yang sudah berakhir bukan terkunci", func(t *testing.T) {
		sekarang := jamBeku(t, awalUji)
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)
		repo.kunciTerkunci(emailUji)
		*sekarang = sekarang.Add(time.Hour)

		hasil, err := uc.BukaKunciPengguna(context.Background(), "user-123", "admin-1")

		require.NoError(t, err)
		assert.False(t, hasil.TerkunciSebelumnya)
	})
	t.Run("pengguna tidak ada", func(t *testing.T) {
		repo := new(mockRepository)
		uc, recorder := newTestUseCaseWithAudit(repo)
		repo.On("GetUserByID", mock.Anything, "hilang").Return(nil, gorm.ErrRecordNotFound)

		_, err := uc.BukaKunciPengguna(context.Background(), "hilang", "admin-1")

		assert.ErrorIs(t, err, ErrUserNotFound)
		assert.Empty(t, recorder.Entries())
	})
	t.Run("galat basis data bukan 'tidak ada'", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		repo.On("GetUserByID", mock.Anything, "user-123").Return(nil, errors.New("db mati"))

		_, err := uc.BukaKunciPengguna(context.Background(), "user-123", "admin-1")

		require.Error(t, err)
		assert.NotErrorIs(t, err, ErrUserNotFound)
	})
	t.Run("galat penyimpanan dilaporkan dan tidak diaudit", func(t *testing.T) {
		repo := new(mockRepository)
		uc, recorder := newTestUseCaseWithAudit(repo)
		repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)
		repo.errHapus = errors.New("db mati")

		_, err := uc.BukaKunciPengguna(context.Background(), "user-123", "admin-1")

		require.Error(t, err)
		assert.Empty(t, recorder.Entries())
	})
}

func TestKunciPengguna(t *testing.T) {
	jamBeku(t, awalUji)
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	repo.kunciTerkunci("a@example.com")
	repo.kunciSampai("c@example.com", awalUji.Add(-time.Minute)) // sudah berakhir
	users := []domain.Pengguna{
		{ID: "u-a", Email: "A@example.com"},
		{ID: "u-b", Email: "b@example.com"},
		{ID: "u-c", Email: "c@example.com"},
	}

	hasil, err := uc.KunciPengguna(context.Background(), users)

	require.NoError(t, err)
	assert.Len(t, hasil, 1)
	assert.Equal(t, awalUji.Add(10*time.Minute), hasil["u-a"])

	kosong, err := uc.KunciPengguna(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, kosong)

	repo.errAktif = errors.New("db mati")
	_, err = uc.KunciPengguna(context.Background(), users)
	assert.Error(t, err)
}

func TestAkunTerkunciError(t *testing.T) {
	err := error(&AkunTerkunciError{Sampai: awalUji})
	assert.ErrorIs(t, err, ErrAccountLocked)
	assert.NotErrorIs(t, err, ErrInvalidCredentials)
	assert.Equal(t, ErrAccountLocked.Error(), err.Error())

	assert.Equal(t, 0, sisaDetik(awalUji, awalUji))
	assert.Equal(t, 0, sisaDetik(awalUji, awalUji.Add(time.Second)))
	assert.Equal(t, 1, sisaDetik(awalUji.Add(time.Millisecond), awalUji), "dibulatkan ke atas")
	assert.Equal(t, 900, sisaDetik(awalUji.Add(15*time.Minute), awalUji))
}
