package integration

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/pkg/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Penulisan per kolom pada tabel pengguna (01 review: lost update). Jalur baca-ubah-tulis tidak lagi menyimpan
// seluruh baris dari salinan yang dibaca lebih dulu (GORM Save): tiap repositori hanya menulis kolom miliknya,
// dan penulisan bersyarat menolak bila keadaan yang diharapkan sudah berubah.

func ptr[T any](v T) *T { return &v }

type barisPengguna struct {
	Nama         string
	Email        string
	Status       string
	Peran        string
	Lapis        string
	Lembaga      string
	PasswordHash *string
	Wajib        bool `gorm:"column:wajib_ganti_password"`
	MfaAktif     bool
	MfaSecret    *string
	MfaLangkah   int64 `gorm:"column:mfa_langkah_terakhir"`
}

func (s *Suite) barisPengguna(t *testing.T, id string) barisPengguna {
	t.Helper()
	var b barisPengguna
	require.NoError(t, s.db.Raw(`SELECT nama, email, status::text AS status, peran::text AS peran, lapis::text AS lapis, lembaga,
		password_hash, wajib_ganti_password, mfa_aktif, mfa_secret, mfa_langkah_terakhir FROM pengguna WHERE id = ?`, id).Scan(&b).Error)
	return b
}

func TestPatchUser_HanyaKolomPatchDanBarisSebelumSesudahDibacaSegar(t *testing.T) {
	s := setupSuite(t)
	repo := newRepo(s)
	ctx := context.Background()
	basi, err := repo.GetUserByID(ctx, idBiasa) // salinan yang dibaca lebih dulu, lalu basi
	require.NoError(t, err)
	require.NoError(t, s.db.Exec("UPDATE pengguna SET status = 'NONAKTIF', peran = 'ADMIN_SISTEM', lapis = 'SISTEM', wajib_ganti_password = true WHERE id = ?", idBiasa).Error)
	time.Sleep(10 * time.Millisecond)

	sebelum, sesudah, err := repo.PatchUser(ctx, idBiasa, domain.PenggunaPatch{Nama: ptr("Nama Baru"), Lembaga: ptr("BPS")})

	require.NoError(t, err)
	assert.Equal(t, "AKTIF", basi.Status, "kontrol: salinan lama memang basi")
	assert.Equal(t, "NONAKTIF", sebelum.Status, "baris sebelum dibaca bersama penulisan, bukan dari salinan lama")
	assert.Equal(t, "ADMIN_SISTEM", sebelum.Peran)
	assert.Equal(t, "Pengguna Biasa", sebelum.Nama)
	assert.Equal(t, "Nama Baru", sesudah.Nama)
	assert.Equal(t, "BPS", sesudah.Lembaga)
	assert.Equal(t, "NONAKTIF", sesudah.Status)
	assert.True(t, sesudah.DiupdatePada.After(sebelum.DiupdatePada), "diupdate_pada ikut maju")

	b := s.barisPengguna(t, idBiasa)
	assert.Equal(t, "NONAKTIF", b.Status, "penonaktifan tidak ditimpa")
	assert.Equal(t, "ADMIN_SISTEM", b.Peran)
	assert.Equal(t, "SISTEM", b.Lapis)
	assert.True(t, b.Wajib)
	assert.Equal(t, "Nama Baru", b.Nama)
	assert.Equal(t, "BPS", b.Lembaga)
}

func TestPatchUser_SemuaKolomProfilTermasukEnumDanKabupaten(t *testing.T) {
	s := setupSuite(t)
	repo := newRepo(s)

	_, sesudah, err := repo.PatchUser(context.Background(), idBiasa, domain.PenggunaPatch{
		Nama: ptr("N"), Email: ptr("Baru.Biasa@gardainflasi.id"), Peran: ptr("MANAJER_DATA"), Lapis: ptr("MANAJERIAL"),
		Lembaga: ptr("L"), KabupatenKode: ptr("1171"), Status: ptr("NONAKTIF"),
	})

	require.NoError(t, err)
	assert.Equal(t, "MANAJER_DATA", sesudah.Peran)
	require.NotNil(t, sesudah.KabupatenKode)
	assert.Equal(t, "1171", *sesudah.KabupatenKode)
	b := s.barisPengguna(t, idBiasa)
	assert.Equal(t, "Baru.Biasa@gardainflasi.id", b.Email)
	assert.Equal(t, "MANAJER_DATA", b.Peran)
	assert.Equal(t, "MANAJERIAL", b.Lapis)
	assert.Equal(t, "NONAKTIF", b.Status)
}

func TestPatchUser_KosongTidakMenulisDanPenggunaHilangDilaporkan(t *testing.T) {
	s := setupSuite(t)
	repo := newRepo(s)
	ctx := context.Background()
	var sebelumTs time.Time
	require.NoError(t, s.db.Raw("SELECT diupdate_pada FROM pengguna WHERE id = ?", idBiasa).Scan(&sebelumTs).Error)

	a, b, err := repo.PatchUser(ctx, idBiasa, domain.PenggunaPatch{})
	require.NoError(t, err)
	assert.Equal(t, a.Nama, b.Nama)
	var sesudahTs time.Time
	require.NoError(t, s.db.Raw("SELECT diupdate_pada FROM pengguna WHERE id = ?", idBiasa).Scan(&sesudahTs).Error)
	assert.True(t, sebelumTs.Equal(sesudahTs), "patch kosong tidak menulis apa pun")

	_, _, err = repo.PatchUser(ctx, uid("tidak-ada"), domain.PenggunaPatch{Nama: ptr("x")})
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestPatchUser_EmailKembarDijawabDuplicatedKey(t *testing.T) {
	s := setupSuite(t)

	_, _, err := newRepo(s).PatchUser(context.Background(), idBiasa, domain.PenggunaPatch{Email: ptr("TEST@gardainflasi.id")})

	require.ErrorIs(t, err, gorm.ErrDuplicatedKey, "indeks unik lower(email) menolak, tanpa menulis apa pun")
	assert.Equal(t, "uji.biasa@gardainflasi.id", s.barisPengguna(t, idBiasa).Email)
}

func TestPatchUser_PenulisSerentakKolomBerbedaSemuanyaTerpakai(t *testing.T) {
	s := setupSuite(t)
	repo := newRepo(s)
	ctx := context.Background()

	var wg sync.WaitGroup
	galat := make(chan error, 40)
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _, err := repo.PatchUser(ctx, idBiasa, domain.PenggunaPatch{Nama: ptr("Nama Serentak")})
			galat <- err
		}()
		go func() {
			defer wg.Done()
			_, _, err := repo.PatchUser(ctx, idBiasa, domain.PenggunaPatch{Status: ptr("NONAKTIF")})
			galat <- err
		}()
	}
	wg.Wait()
	close(galat)

	for err := range galat {
		require.NoError(t, err)
	}
	b := s.barisPengguna(t, idBiasa)
	assert.Equal(t, "NONAKTIF", b.Status, "tidak ada penulis nama yang mengembalikan status")
	assert.Equal(t, "Nama Serentak", b.Nama)
}

func TestSetPassword_HanyaKolomKataSandi(t *testing.T) {
	s := setupSuite(t)
	repo := newRepo(s)
	hash, err := crypto.HashPassword("Baru#12345")
	require.NoError(t, err)
	require.NoError(t, s.db.Exec("UPDATE pengguna SET status = 'NONAKTIF', nama = 'Dijaga' WHERE id = ?", idBiasa).Error)
	at := time.Now().UTC().Truncate(time.Second)

	ok, err := repo.SetPassword(context.Background(), idBiasa, domain.GantiPassword{Hash: hash, WajibGanti: true, At: at})

	require.NoError(t, err)
	assert.True(t, ok)
	b := s.barisPengguna(t, idBiasa)
	require.NotNil(t, b.PasswordHash)
	assert.True(t, crypto.VerifyPassword("Baru#12345", *b.PasswordHash))
	assert.True(t, b.Wajib)
	assert.Equal(t, "NONAKTIF", b.Status)
	assert.Equal(t, "Dijaga", b.Nama)
	assert.EqualValues(t, 1, s.hitung(t, "SELECT count(*) FROM pengguna WHERE id = ? AND password_diubah_pada = ?", idBiasa, at))
}

func TestSetPassword_PenggunaTidakAdaDanSyaratHashLama(t *testing.T) {
	s := setupSuite(t)
	repo := newRepo(s)
	ctx := context.Background()
	hashBaru, err := crypto.HashPassword("Baru#12345")
	require.NoError(t, err)
	saatIni := s.barisPengguna(t, idBiasa).PasswordHash
	require.NotNil(t, saatIni)

	ok, err := repo.SetPassword(ctx, uid("tidak-ada"), domain.GantiPassword{Hash: hashBaru, At: time.Now()})
	require.NoError(t, err)
	assert.False(t, ok, "tidak ada baris yang cocok")

	salah := "$2a$12$bukanhashyangtersimpan"
	ok, err = repo.SetPassword(ctx, idBiasa, domain.GantiPassword{Hash: hashBaru, At: time.Now(), HashSebelumnya: &salah})
	require.NoError(t, err)
	assert.False(t, ok, "hash sebelumnya sudah tidak sama: tidak ada yang ditimpa")
	assert.Equal(t, *saatIni, *s.barisPengguna(t, idBiasa).PasswordHash)

	ok, err = repo.SetPassword(ctx, idBiasa, domain.GantiPassword{Hash: hashBaru, At: time.Now(), HashSebelumnya: saatIni})
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, hashBaru, *s.barisPengguna(t, idBiasa).PasswordHash)
}

func TestSetPassword_PenggantianSerentakDenganSyaratHashSamaHanyaSatuMenang(t *testing.T) {
	s := setupSuite(t)
	repo := newRepo(s)
	saatIni := s.barisPengguna(t, idBiasa).PasswordHash
	require.NotNil(t, saatIni)

	var wg sync.WaitGroup
	var mu sync.Mutex
	menang := 0
	for i := 0; i < 8; i++ {
		hash, err := crypto.HashPassword("Serentak#Ke" + string(rune('A'+i)) + "1")
		require.NoError(t, err)
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := repo.SetPassword(context.Background(), idBiasa, domain.GantiPassword{Hash: hash, At: time.Now(), HashSebelumnya: saatIni})
			assert.NoError(t, err)
			if ok {
				mu.Lock()
				menang++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, 1, menang, "compare-and-swap: tepat satu penulis yang melihat hash lama")
}

func TestSetMFASecret_HanyaBilaMFABelumAktif(t *testing.T) {
	s := setupSuite(t)
	repo := newRepo(s)
	ctx := context.Background()

	ok, err := repo.SetMFASecret(ctx, idBiasa, "v1:rahasia-setup")
	require.NoError(t, err)
	assert.True(t, ok)
	b := s.barisPengguna(t, idBiasa)
	require.NotNil(t, b.MfaSecret)
	assert.Equal(t, "v1:rahasia-setup", *b.MfaSecret)
	assert.False(t, b.MfaAktif)

	// Akun yang MFA-nya sudah aktif (uji.mfa): rahasia aktif tidak boleh diganti rahasia setup.
	sebelum := s.barisPengguna(t, uid("test-user-mfa"))
	ok, err = repo.SetMFASecret(ctx, uid("test-user-mfa"), "v1:rahasia-setup")
	require.NoError(t, err)
	assert.False(t, ok)
	sesudah := s.barisPengguna(t, uid("test-user-mfa"))
	assert.True(t, sesudah.MfaAktif)
	assert.Equal(t, *sebelum.MfaSecret, *sesudah.MfaSecret)

	ok, err = repo.SetMFASecret(ctx, uid("tidak-ada"), "v1:x")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestActivateMFA_BersyaratRahasiaDanBelumAktif(t *testing.T) {
	s := setupSuite(t)
	repo := newRepo(s)
	ctx := context.Background()
	_, err := repo.SetMFASecret(ctx, idBiasa, "v1:rahasia-a")
	require.NoError(t, err)
	kode := []string{"h1", "h2", "h3"}

	ok, err := repo.ActivateMFA(ctx, idBiasa, kode, 7, "v1:rahasia-lain")
	require.NoError(t, err)
	assert.False(t, ok, "rahasia berganti sesudah kode diverifikasi: tidak diaktifkan")
	assert.False(t, s.barisPengguna(t, idBiasa).MfaAktif)
	assert.Zero(t, s.hitung(t, "SELECT count(*) FROM mfa_kode_cadangan WHERE user_id = ?", idBiasa), "kode cadangan tidak ditulis")

	ok, err = repo.ActivateMFA(ctx, idBiasa, kode, 7, "v1:rahasia-a")
	require.NoError(t, err)
	assert.True(t, ok)
	b := s.barisPengguna(t, idBiasa)
	assert.True(t, b.MfaAktif)
	assert.EqualValues(t, 7, b.MfaLangkah)
	assert.EqualValues(t, 3, s.hitung(t, "SELECT count(*) FROM mfa_kode_cadangan WHERE user_id = ?", idBiasa))

	ok, err = repo.ActivateMFA(ctx, idBiasa, []string{"x1", "x2"}, 9, "v1:rahasia-a")
	require.NoError(t, err)
	assert.False(t, ok, "sudah aktif: aktivasi kedua tidak mengganti kode cadangan")
	assert.EqualValues(t, 3, s.hitung(t, "SELECT count(*) FROM mfa_kode_cadangan WHERE user_id = ?", idBiasa))
	assert.EqualValues(t, 7, s.barisPengguna(t, idBiasa).MfaLangkah)
}

func TestActivateMFA_AktivasiSerentakHanyaSatuMenang(t *testing.T) {
	s := setupSuite(t)
	repo := newRepo(s)
	ctx := context.Background()
	_, err := repo.SetMFASecret(ctx, idBiasa, "v1:rahasia-a")
	require.NoError(t, err)

	var wg sync.WaitGroup
	var mu sync.Mutex
	menang := 0
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := repo.ActivateMFA(ctx, idBiasa, []string{"a", "b", "c", "d"}, 5, "v1:rahasia-a")
			assert.NoError(t, err)
			if ok {
				mu.Lock()
				menang++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, 1, menang)
	assert.EqualValues(t, 4, s.hitung(t, "SELECT count(*) FROM mfa_kode_cadangan WHERE user_id = ?", idBiasa), "satu set kode cadangan, bukan 8")
}

// ── Alur HTTP: kolom yang bukan milik aksi tidak tersentuh ────────────

func TestChangePassword_TidakMenyentuhKolomLain(t *testing.T) {
	s := setupSuite(t)
	access, _ := s.loginAs(t, emailBiasa, sandiBenarUji)
	require.NoError(t, s.db.Exec("UPDATE pengguna SET nama = 'Dijaga', lembaga = 'LEMBAGA-X' WHERE id = ?", idBiasa).Error)

	w := s.serveRequest(t, http.MethodPost, "/v1/auth/change-password",
		map[string]any{"old_password": sandiBenarUji, "new_password": "BaruSekali#77"}, s.authHeader(access))

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	b := s.barisPengguna(t, idBiasa)
	require.NotNil(t, b.PasswordHash)
	assert.True(t, crypto.VerifyPassword("BaruSekali#77", *b.PasswordHash))
	assert.False(t, b.Wajib)
	assert.Equal(t, "Dijaga", b.Nama)
	assert.Equal(t, "LEMBAGA-X", b.Lembaga)
	assert.EqualValues(t, 1, s.hitung(t, "SELECT count(*) FROM pengguna WHERE id = ? AND password_diubah_pada IS NOT NULL", idBiasa))
}

func TestAdmin_PatchDanGantiPeranMencatatSnapshotAuditDariBarisYangDitulis(t *testing.T) {
	s := setupSuite(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)

	w := s.panggil(t, http.MethodPatch, "/v1/users/"+userBiasa, map[string]any{"nama": "Nama Audit"}, admin.access)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = s.panggil(t, http.MethodPut, "/v1/users/"+userBiasa+"/roles", map[string]any{"peran": "MANAJER_DATA"}, admin.access)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "MANAJER_DATA", s.dataObj(t, w)["peran"])

	assert.EqualValues(t, 1, s.count(t, `SELECT count(*) FROM audit_log WHERE aksi = 'UPDATE_USER' AND entitas_id = ?
		AND sebelum->>'nama' = 'Pengguna Biasa' AND sesudah->>'nama' = 'Nama Audit' AND sesudah->>'status' = 'AKTIF'`, userBiasa))
	assert.EqualValues(t, 1, s.count(t, `SELECT count(*) FROM audit_log WHERE aksi = 'ASSIGN_ROLE' AND entitas_id = ?
		AND sebelum->>'peran' = 'OPS_MAHASISWA' AND sesudah->>'peran' = 'MANAJER_DATA' AND sesudah->>'nama' = 'Nama Audit'`, userBiasa))
	assert.Equal(t, http.StatusNotFound, s.panggil(t, http.MethodPut, "/v1/users/"+uid("tidak-ada")+"/roles", map[string]any{"peran": "MANAJER_DATA"}, admin.access).Code)
	assert.Equal(t, http.StatusNotFound, s.panggil(t, http.MethodPatch, "/v1/users/"+uid("tidak-ada"), map[string]any{"nama": "x"}, admin.access).Code)
}

func TestAdmin_PatchEmailYangDipakaiPenggunaLainDijawab409(t *testing.T) {
	s := setupSuite(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)

	w := s.panggil(t, http.MethodPatch, "/v1/users/"+userBiasa, map[string]any{"email": "TEST@gardainflasi.id"}, admin.access)

	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
}
