package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Regresi: konfirmasi kata sandi di nonaktifkan-MFA dan kode-cadangan-baru dulu memeriksa bcrypt langsung
// (confirmPassword) tanpa lockout per email, sedangkan ChangePassword menghitungnya. Pemegang access token
// curian dapat menebak kata sandi di dua endpoint itu tanpa batas selain batas laju API. Kini semuanya
// lewat periksaKataSandiPemilik: terkunci -> 423, salah -> terhitung, galat infrastruktur -> tidak dihitung.

// aksiKonfirmasi adalah aksi yang meminta konfirmasi kata sandi beserta mock "berhasil" miliknya.
type aksiKonfirmasi struct {
	nama     string
	jalankan func(uc *UseCase, sandi string) error
	// siap memasang mock untuk jalur sukses aksi ini dan mengembalikan nama metode yang tidak boleh terpanggil
	// bila konfirmasi ditolak.
	siap func(repo *mockRepository) (terlarang string)
}

func daftarAksiKonfirmasi() []aksiKonfirmasi {
	return []aksiKonfirmasi{
		{
			nama: "nonaktifkan MFA",
			jalankan: func(uc *UseCase, sandi string) error {
				return uc.DisableMFA(context.Background(), uid("user-123"), sandi, "10.0.0.1")
			},
			siap: func(repo *mockRepository) string {
				repo.On("ClearMFA", mock.Anything, uid("user-123")).Return(nil)
				return "ClearMFA"
			},
		},
		{
			nama: "kode cadangan baru",
			jalankan: func(uc *UseCase, sandi string) error {
				_, err := uc.RegenerateBackupCodes(context.Background(), uid("user-123"), sandi, "10.0.0.1")
				return err
			},
			siap: func(repo *mockRepository) string {
				repo.On("ReplaceBackupCodes", mock.Anything, uid("user-123"), mock.Anything).Return(nil)
				return "ReplaceBackupCodes"
			},
		},
	}
}

// jumlahPanggilan menghitung berapa kali metode mock dipanggil.
func jumlahPanggilan(repo *mockRepository, metode string) int {
	n := 0
	for _, c := range repo.Calls {
		if c.Method == metode {
			n++
		}
	}
	return n
}

// jumlahGagal adalah hitungan kegagalan lockout untuk emailUji; 0 bila belum ada barisnya.
func jumlahGagal(repo *mockRepository) int {
	if k := repo.keadaan(emailUji); k != nil {
		return k.Gagal
	}
	return 0
}

func siapKonfirmasi(t *testing.T, aksi aksiKonfirmasi) (*mockRepository, *UseCase, string) {
	t.Helper()
	jamBeku(t, awalUji)
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	user, _ := userMFA(t, uc)
	repo.On("GetUserByID", mock.Anything, uid("user-123")).Return(user, nil)
	repo.On("CreateLoginAttempt", mock.Anything, mock.Anything).Return(nil)
	terlarang := aksi.siap(repo)
	return repo, uc, terlarang
}

func TestKonfirmasiKataSandi_SalahBerulangMengunciAkunDanAksiTidakJalan(t *testing.T) {
	for _, aksi := range daftarAksiKonfirmasi() {
		t.Run(aksi.nama, func(t *testing.T) {
			repo, uc, terlarang := siapKonfirmasi(t, aksi)

			for i := 1; i <= 5; i++ {
				require.ErrorIs(t, aksi.jalankan(uc, sandiSalah), ErrWrongPassword, "percobaan %d", i)
			}
			assert.Equal(t, 5, jumlahGagal(repo), "tiap tebakan salah dihitung")

			// Kata sandi yang benar pun ditolak: akun terkunci, kata sandi tidak diperiksa.
			err := aksi.jalankan(uc, sandiBenar)
			require.ErrorIs(t, err, ErrAccountLocked, "pada kode lama aksi lolos tanpa batas")
			var terkunci *AkunTerkunciError
			require.ErrorAs(t, err, &terkunci)
			assert.Equal(t, 15*60, terkunci.SisaDetik)
			assert.Zero(t, jumlahPanggilan(repo, terlarang), "aksi sensitif tidak boleh jalan saat terkunci")
			assert.Equal(t, 5, jumlahGagal(repo), "percobaan yang ditolak tidak mengubah hitungan")
		})
	}
}

func TestKonfirmasiKataSandi_BenarTidakDihitungDanKataSandiSalahDicatat(t *testing.T) {
	for _, aksi := range daftarAksiKonfirmasi() {
		t.Run(aksi.nama, func(t *testing.T) {
			repo, uc, _ := siapKonfirmasi(t, aksi)

			for i := 0; i < 4; i++ {
				require.ErrorIs(t, aksi.jalankan(uc, sandiSalah), ErrWrongPassword)
			}
			require.NoError(t, aksi.jalankan(uc, sandiBenar))
			assert.Equal(t, 4, jumlahGagal(repo), "kata sandi benar mengembalikan percobaannya dan tidak menghapus hitungan")

			// Percobaan salah berikutnya (yang kelima) masih dijawab 'salah', baru yang keenam terkunci.
			require.ErrorIs(t, aksi.jalankan(uc, sandiSalah), ErrWrongPassword)
			require.ErrorIs(t, aksi.jalankan(uc, sandiBenar), ErrAccountLocked)
		})
	}
}

func TestKonfirmasiKataSandi_SalahDicatatDiLoginAttemptsDenganAlasanTersendiri(t *testing.T) {
	repo := new(mockRepository)
	jamBeku(t, awalUji)
	uc := newTestUseCase(repo)
	user, _ := userMFA(t, uc)
	repo.On("GetUserByID", mock.Anything, uid("user-123")).Return(user, nil)
	repo.On("ClearMFA", mock.Anything, uid("user-123")).Return(nil)
	var dicatat []*domain.LoginAttempt
	repo.On("CreateLoginAttempt", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { dicatat = append(dicatat, args.Get(1).(*domain.LoginAttempt)) }).Return(nil)

	require.ErrorIs(t, uc.DisableMFA(context.Background(), uid("user-123"), sandiSalah, "10.0.0.7"), ErrWrongPassword)
	require.NoError(t, uc.DisableMFA(context.Background(), uid("user-123"), sandiBenar, "10.0.0.7"))

	require.Len(t, dicatat, 1, "hanya tebakan salah yang dicatat")
	assert.False(t, dicatat[0].Success)
	require.NotNil(t, dicatat[0].FailureReason)
	assert.Equal(t, "wrong_password_confirm", *dicatat[0].FailureReason)
	require.NotNil(t, dicatat[0].UserID)
	assert.Equal(t, uid("user-123"), *dicatat[0].UserID)
	require.NotNil(t, dicatat[0].IP)
	assert.Equal(t, "10.0.0.7", *dicatat[0].IP)
}

func TestKonfirmasiKataSandi_MemakaiKunciYangSamaDenganLoginDanGantiKataSandi(t *testing.T) {
	jamBeku(t, awalUji)
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	user, _ := userMFA(t, uc)
	siapLogin(repo, user)
	repo.On("GetUserByID", mock.Anything, uid("user-123")).Return(user, nil)

	// Tiga tebakan lewat login, dua lewat konfirmasi kata sandi: satu hitungan per email.
	for i := 0; i < 3; i++ {
		require.ErrorIs(t, login(uc, emailUji, sandiSalah), ErrInvalidCredentials)
	}
	for i := 0; i < 2; i++ {
		require.ErrorIs(t, uc.DisableMFA(context.Background(), uid("user-123"), sandiSalah, "10.0.0.1"), ErrWrongPassword)
	}

	assert.ErrorIs(t, login(uc, emailUji, sandiBenar), ErrAccountLocked)
	_, _, err := uc.ChangePassword(context.Background(), uid("user-123"), sandiBenar, "BaruLagi99!", "10.0.0.1")
	assert.ErrorIs(t, err, ErrAccountLocked)
	repo.AssertNotCalled(t, "ClearMFA", mock.Anything, mock.Anything)
}

func TestKonfirmasiKataSandi_GalatInfrastrukturDitolakTertutupDanTidakDihitung(t *testing.T) {
	for _, aksi := range daftarAksiKonfirmasi() {
		t.Run(aksi.nama+": gagal memesan percobaan", func(t *testing.T) {
			repo, uc, terlarang := siapKonfirmasi(t, aksi)
			repo.errMulai = errors.New("db mati")

			err := aksi.jalankan(uc, sandiBenar)

			require.Error(t, err)
			assert.NotErrorIs(t, err, ErrWrongPassword)
			assert.NotErrorIs(t, err, ErrAccountLocked)
			assert.Zero(t, jumlahPanggilan(repo, terlarang), "gagal tertutup: aksi sensitif tidak jalan")
			assert.Zero(t, jumlahPanggilan(repo, "CreateLoginAttempt"))
			assert.Nil(t, repo.keadaan(emailUji), "tidak ada percobaan yang tercatat")
		})
		t.Run(aksi.nama+": gagal mengembalikan percobaan setelah kata sandi benar", func(t *testing.T) {
			repo, uc, _ := siapKonfirmasi(t, aksi)
			repo.errKembali = errors.New("db lambat")

			// Kegagalan hanya dilaporkan di log: aksinya tetap berjalan.
			require.NoError(t, aksi.jalankan(uc, sandiBenar))
		})
	}
}

func TestKonfirmasiKataSandi_AkunTanpaKataSandiTidakDihitung(t *testing.T) {
	jamBeku(t, awalUji)
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	user, _ := userMFA(t, uc)
	user.PasswordHash = nil
	repo.On("GetUserByID", mock.Anything, uid("user-123")).Return(user, nil)

	for i := 0; i < 8; i++ {
		require.ErrorIs(t, uc.DisableMFA(context.Background(), uid("user-123"), "apa saja", "10.0.0.1"), ErrWrongPassword)
	}

	assert.Nil(t, repo.keadaan(emailUji), "sama dengan ChangePassword: akun SSO tanpa kata sandi tidak mengunci dirinya")
}

func TestKonfirmasiKataSandi_KunciBerakhirAksiJalanLagi(t *testing.T) {
	sekarang := jamBeku(t, awalUji)
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	user, _ := userMFA(t, uc)
	repo.On("GetUserByID", mock.Anything, uid("user-123")).Return(user, nil)
	repo.On("CreateLoginAttempt", mock.Anything, mock.Anything).Return(nil)
	repo.On("ClearMFA", mock.Anything, uid("user-123")).Return(nil)
	for i := 0; i < 5; i++ {
		require.ErrorIs(t, uc.DisableMFA(context.Background(), uid("user-123"), sandiSalah, "10.0.0.1"), ErrWrongPassword)
	}
	require.ErrorIs(t, uc.DisableMFA(context.Background(), uid("user-123"), sandiBenar, "10.0.0.1"), ErrAccountLocked)

	*sekarang = sekarang.Add(15 * time.Minute)

	require.NoError(t, uc.DisableMFA(context.Background(), uid("user-123"), sandiBenar, "10.0.0.1"))
}
