package integration

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	authpg "github.com/garda-inflasi/api/internal/auth/repository/postgres"
	"github.com/garda-inflasi/api/internal/auth/usecase"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Tes pembersihan: basis data sungguhan hasil migrasi, baris dengan waktu eksplisit relatif terhadap
// satu waktu acuan, lalu diperiksa tepat baris mana yang tersisa (bukan sekadar jumlahnya).

var pbrSeq atomic.Int64

var pbrAcuan = time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)

var (
	pbrUserA = uid("pembersihan-user-a")
	pbrUserB = uid("pembersihan-user-b")
)

func pbrDB(t *testing.T) *gorm.DB {
	t.Helper()
	// Nama pendek dan unik: nama dari t.Name() bisa melebihi 63 karakter (terpotong Postgres) dan bertabrakan antar subtes.
	db := pg.NewDatabase(t, fmt.Sprintf("pembersihan_%d", pbrSeq.Add(1)))
	pgtest.MigrateUp(t, db)
	for _, id := range []string{pbrUserA, pbrUserB} {
		require.NoError(t, db.Exec(`
			INSERT INTO pengguna (id, nama, email, peran, lapis, lembaga, status)
			VALUES (?, ?, ?, 'OPS_MAHASISWA', 'OPERASIONAL', 'BPPSDMP', 'AKTIF')`,
			id, id, id+"@gardainflasi.id").Error)
	}
	return db
}

func pbrPembersih(db *gorm.DB, cfg usecase.KonfigPembersihan) *usecase.Pembersihan {
	return usecase.NewPembersihan(authpg.New(db), cfg, nil)
}

func pbrSisa(t *testing.T, db *gorm.DB, query string) []string {
	t.Helper()
	var out []string
	require.NoError(t, db.Raw(query).Scan(&out).Error)
	return out
}

func pbrBlacklist(t *testing.T, db *gorm.DB, jti string, expiresAt time.Time) {
	t.Helper()
	require.NoError(t, db.Exec("INSERT INTO jwt_blacklist (jti, expires_at) VALUES (?, ?)", jti, expiresAt).Error)
}

func pbrRefresh(t *testing.T, db *gorm.DB, nama, userID string, expiresAt time.Time, dicabut bool) {
	t.Helper()
	var revokedAt *time.Time
	if dicabut {
		r := pbrAcuan.Add(-time.Minute)
		revokedAt = &r
	}
	require.NoError(t, db.Exec(
		"INSERT INTO refresh_tokens (user_id, token_hash, family_id, expires_at, revoked_at, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		userID, nama, uuid.NewString(), expiresAt, revokedAt, pbrAcuan.Add(-time.Hour)).Error)
}

func pbrReset(t *testing.T, db *gorm.DB, nama, userID string, expiresAt time.Time, usedAt *time.Time) {
	t.Helper()
	require.NoError(t, db.Exec(
		"INSERT INTO password_reset_tokens (user_id, token_hash, expires_at, used_at) VALUES (?, ?, ?, ?)",
		userID, nama, expiresAt, usedAt).Error)
}

func pbrAttempt(t *testing.T, db *gorm.DB, nama string, umur time.Duration, userID *string, sukses bool) {
	t.Helper()
	// failure_reason membawa nama baris agar mudah dikenali di asersi.
	require.NoError(t, db.Exec(
		"INSERT INTO login_attempts (user_id, ip, success, failure_reason, attempted_at) VALUES (?, '10.0.0.1', ?, ?, ?)",
		userID, sukses, nama, pbrAcuan.Add(-umur)).Error)
}

func pbrWaktu(w time.Time) *time.Time { return &w }

func TestPembersihan_JwtBlacklist_HanyaYangKedaluwarsa(t *testing.T) {
	db := pbrDB(t)
	pbrBlacklist(t, db, "kedaluwarsa-1s", pbrAcuan.Add(-time.Second))
	pbrBlacklist(t, db, "kedaluwarsa-30h", pbrAcuan.Add(-30*time.Hour))
	pbrBlacklist(t, db, "tepat-sekarang", pbrAcuan) // belum "lewat": perbandingan ketat
	pbrBlacklist(t, db, "berlaku-1s", pbrAcuan.Add(time.Second))
	pbrBlacklist(t, db, "berlaku-15m", pbrAcuan.Add(15*time.Minute))
	// Berlaku menurut jam basis data sungguhan (IsJWTBlacklisted memakai now()), bukan hanya menurut waktu acuan.
	pbrBlacklist(t, db, "berlaku-24j-nyata", time.Now().Add(24*time.Hour))

	hasil, err := pbrPembersih(db, usecase.DefaultKonfigPembersihan()).JalankanPada(context.Background(), pbrAcuan)

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"tepat-sekarang", "berlaku-1s", "berlaku-15m", "berlaku-24j-nyata"}, pbrSisa(t, db, "SELECT jti FROM jwt_blacklist"))
	for _, l := range hasil.Langkah {
		if l.Nama == "jwt_blacklist" {
			assert.EqualValues(t, 2, l.Dihapus)
		}
	}
	// Yang tersisa masih diperlakukan sebagai dicabut oleh pembacaan blacklist sungguhan.
	repo := authpg.New(db)
	cabut, err := repo.IsJWTBlacklisted(context.Background(), "berlaku-24j-nyata")
	require.NoError(t, err)
	assert.True(t, cabut)
}

func TestPembersihan_SSOState_HanyaYangKedaluwarsa(t *testing.T) {
	db := pbrDB(t)
	simpan := func(nama string, expiresAt time.Time) {
		t.Helper()
		require.NoError(t, db.Exec(`INSERT INTO sso_login_state (state_hash, binding_hash, provider, nonce, code_verifier, expires_at)
			VALUES (?, 'b', 'google', 'n', 'v', ?)`, nama, expiresAt).Error)
	}
	simpan("ditinggalkan", pbrAcuan.Add(-time.Minute))
	simpan("tepat-sekarang", pbrAcuan)
	simpan("menunggu-callback", pbrAcuan.Add(9*time.Minute))

	_, err := pbrPembersih(db, usecase.DefaultKonfigPembersihan()).JalankanPada(context.Background(), pbrAcuan)

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"tepat-sekarang", "menunggu-callback"}, pbrSisa(t, db, "SELECT state_hash FROM sso_login_state"))
}

func TestPembersihan_KunciLogin_HanyaUsangDanTidakTerkunci(t *testing.T) {
	db := pbrDB(t)
	simpan := func(nama string, diperbarui time.Time, sampai *time.Time) {
		t.Helper()
		require.NoError(t, db.Exec(`INSERT INTO kunci_login (email_hash, gagal, jendela_mulai, terkunci_sampai, diperbarui_pada)
			VALUES (?, 1, ?, ?, ?)`, nama, diperbarui, sampai, diperbarui).Error)
	}
	simpan("usang", pbrAcuan.Add(-2*time.Hour), nil)
	simpan("baru", pbrAcuan.Add(-10*time.Minute), nil)
	simpan("usang-tapi-terkunci", pbrAcuan.Add(-2*time.Hour), pbrWaktu(pbrAcuan.Add(5*time.Minute)))

	_, err := pbrPembersih(db, usecase.DefaultKonfigPembersihan()).JalankanPada(context.Background(), pbrAcuan)

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"baru", "usang-tapi-terkunci"}, pbrSisa(t, db, "SELECT email_hash FROM kunci_login"))
}

func TestPembersihan_RefreshTokens_HapusKedaluwarsa_SimpanDicabutYangMasihBerlaku(t *testing.T) {
	db := pbrDB(t)
	pbrRefresh(t, db, "kedaluwarsa-1s-aktif", pbrUserA, pbrAcuan.Add(-time.Second), false)
	pbrRefresh(t, db, "kedaluwarsa-3d-dicabut", pbrUserA, pbrAcuan.Add(-72*time.Hour), true)
	pbrRefresh(t, db, "kedaluwarsa-1s-dicabut", pbrUserB, pbrAcuan.Add(-time.Second), true)
	pbrRefresh(t, db, "tepat-sekarang-aktif", pbrUserA, pbrAcuan, false)
	pbrRefresh(t, db, "berlaku-1s-aktif", pbrUserA, pbrAcuan.Add(time.Second), false)
	pbrRefresh(t, db, "berlaku-5d-DICABUT", pbrUserB, pbrAcuan.Add(5*24*time.Hour), true)
	pbrRefresh(t, db, "berlaku-6d-aktif", pbrUserB, pbrAcuan.Add(6*24*time.Hour), false)

	_, err := pbrPembersih(db, usecase.DefaultKonfigPembersihan()).JalankanPada(context.Background(), pbrAcuan)

	require.NoError(t, err)
	assert.ElementsMatch(t,
		[]string{"tepat-sekarang-aktif", "berlaku-1s-aktif", "berlaku-5d-DICABUT", "berlaku-6d-aktif"},
		pbrSisa(t, db, "SELECT token_hash FROM refresh_tokens"),
		"baris dicabut-tapi-belum-kedaluwarsa wajib tetap ada untuk deteksi pemakaian ulang")
	var masihDicabut int64
	require.NoError(t, db.Raw("SELECT count(*) FROM refresh_tokens WHERE token_hash = 'berlaku-5d-DICABUT' AND revoked_at IS NOT NULL").Scan(&masihDicabut).Error)
	assert.EqualValues(t, 1, masihDicabut, "baris yang disimpan tidak diubah")
}

func TestPembersihan_ResetTokens_RetensiTujuhHari(t *testing.T) {
	db := pbrDB(t)
	tujuhHari := 7 * 24 * time.Hour
	pbrReset(t, db, "kedaluwarsa-100h-lalu-tak-terpakai", pbrUserA, pbrAcuan.Add(-100*24*time.Hour), nil)
	pbrReset(t, db, "kedaluwarsa-7d+1s-tak-terpakai", pbrUserA, pbrAcuan.Add(-tujuhHari-time.Second), nil)
	pbrReset(t, db, "kedaluwarsa-tepat-7d-tak-terpakai", pbrUserA, pbrAcuan.Add(-tujuhHari), nil)
	pbrReset(t, db, "kedaluwarsa-6d-tak-terpakai", pbrUserA, pbrAcuan.Add(-6*24*time.Hour), nil)
	pbrReset(t, db, "dipakai-8d-lalu", pbrUserB, pbrAcuan.Add(-8*24*time.Hour+30*time.Minute), pbrWaktu(pbrAcuan.Add(-8*24*time.Hour)))
	// Dipakai >7 hari lalu walau expires_at (aneh) di masa depan: tetap mati.
	pbrReset(t, db, "dipakai-8d-lalu-exp-masa-depan", pbrUserB, pbrAcuan.Add(time.Hour), pbrWaktu(pbrAcuan.Add(-8*24*time.Hour)))
	pbrReset(t, db, "dipakai-6d-lalu", pbrUserB, pbrAcuan.Add(-6*24*time.Hour+30*time.Minute), pbrWaktu(pbrAcuan.Add(-6*24*time.Hour)))
	// Token lama yang ikut dihanguskan (used_at baru) oleh ConsumeResetToken, padahal kedaluwarsa lama: tetap dibuang.
	pbrReset(t, db, "kedaluwarsa-100d-dihanguskan-baru", pbrUserB, pbrAcuan.Add(-100*24*time.Hour), pbrWaktu(pbrAcuan.Add(-time.Hour)))
	// Hidup: belum dipakai dan belum kedaluwarsa, atau baru saja dipakai.
	pbrReset(t, db, "hidup-10m", pbrUserA, pbrAcuan.Add(10*time.Minute), nil)
	pbrReset(t, db, "hidup-1s", pbrUserB, pbrAcuan.Add(time.Second), nil)
	pbrReset(t, db, "dipakai-1j-lalu", pbrUserB, pbrAcuan.Add(30*time.Minute), pbrWaktu(pbrAcuan.Add(-time.Hour)))

	_, err := pbrPembersih(db, usecase.DefaultKonfigPembersihan()).JalankanPada(context.Background(), pbrAcuan)

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{
		"kedaluwarsa-tepat-7d-tak-terpakai", // batas ketat
		"kedaluwarsa-6d-tak-terpakai",       // masih dalam retensi (pesan "kedaluwarsa" tetap akurat)
		"dipakai-6d-lalu",
		"hidup-10m", "hidup-1s", "dipakai-1j-lalu",
	}, pbrSisa(t, db, "SELECT token_hash FROM password_reset_tokens"))
}

func TestPembersihan_ResetTokens_RetensiNolMenghapusSemuaYangMati(t *testing.T) {
	db := pbrDB(t)
	pbrReset(t, db, "kedaluwarsa-1s", pbrUserA, pbrAcuan.Add(-time.Second), nil)
	pbrReset(t, db, "dipakai-1s", pbrUserA, pbrAcuan.Add(time.Hour), pbrWaktu(pbrAcuan.Add(-time.Second)))
	pbrReset(t, db, "hidup", pbrUserB, pbrAcuan.Add(time.Minute), nil)
	cfg := usecase.DefaultKonfigPembersihan()
	cfg.RetensiResetToken = 0

	_, err := pbrPembersih(db, cfg).JalankanPada(context.Background(), pbrAcuan)

	require.NoError(t, err)
	assert.Equal(t, []string{"hidup"}, pbrSisa(t, db, "SELECT token_hash FROM password_reset_tokens"))
}

func TestPembersihan_LoginAttempts_RetensiBawaan90Hari(t *testing.T) {
	db := pbrDB(t)
	hari := 24 * time.Hour
	pengguna := pbrUserA
	pbrAttempt(t, db, "umur-120d-gagal", 120*hari, &pengguna, false)
	pbrAttempt(t, db, "umur-90d+1s-sukses", 90*hari+time.Second, &pengguna, true)
	pbrAttempt(t, db, "umur-90d+1s-tanpa-pengguna", 90*hari+time.Second, nil, false)
	pbrAttempt(t, db, "umur-tepat-90d", 90*hari, &pengguna, false)
	pbrAttempt(t, db, "umur-89d", 89*hari, &pengguna, false)
	pbrAttempt(t, db, "umur-2d", 2*hari, nil, false)
	pbrAttempt(t, db, "umur-1j", time.Hour, &pengguna, true)
	pbrAttempt(t, db, "umur-0", 0, &pengguna, false)

	_, err := pbrPembersih(db, usecase.DefaultKonfigPembersihan()).JalankanPada(context.Background(), pbrAcuan)

	require.NoError(t, err)
	assert.ElementsMatch(t,
		[]string{"umur-tepat-90d", "umur-89d", "umur-2d", "umur-1j", "umur-0"},
		pbrSisa(t, db, "SELECT failure_reason FROM login_attempts"))
}

func TestPembersihan_LoginAttempts_LantaiSatuHariBerapaPunKonfigurasi(t *testing.T) {
	jam := time.Hour
	for nama, retensi := range map[string]time.Duration{
		"satu_jam":   jam,
		"satu_menit": time.Minute,
		"negatif":    -10 * jam,
		"satu_ns":    time.Nanosecond,
	} {
		t.Run(nama, func(t *testing.T) {
			db := pbrDB(t)
			pengguna := pbrUserA
			pbrAttempt(t, db, "umur-48j", 48*jam, &pengguna, false)
			pbrAttempt(t, db, "umur-24j+1s", 24*jam+time.Second, &pengguna, false)
			pbrAttempt(t, db, "umur-tepat-24j", 24*jam, &pengguna, false)
			pbrAttempt(t, db, "umur-23j59m", 23*jam+59*time.Minute, &pengguna, false)
			pbrAttempt(t, db, "umur-12j", 12*jam, &pengguna, true)
			pbrAttempt(t, db, "umur-15m", 15*time.Minute, &pengguna, false) // jendela penguncian akun
			pbrAttempt(t, db, "umur-0", 0, nil, false)
			cfg := usecase.DefaultKonfigPembersihan()
			cfg.RetensiLoginAttempt = retensi

			_, err := pbrPembersih(db, cfg).JalankanPada(context.Background(), pbrAcuan)

			require.NoError(t, err)
			assert.ElementsMatch(t,
				[]string{"umur-tepat-24j", "umur-23j59m", "umur-12j", "umur-15m", "umur-0"},
				pbrSisa(t, db, "SELECT failure_reason FROM login_attempts"),
				"tidak ada baris yang lebih muda dari 1 hari yang boleh terhapus")
		})
	}
}

func TestPembersihan_LoginAttempts_RetensiNolTidakMenghapusApaPun(t *testing.T) {
	db := pbrDB(t)
	pengguna := pbrUserA
	pbrAttempt(t, db, "sangat-lama", 2000*24*time.Hour, &pengguna, false)
	pbrAttempt(t, db, "baru", time.Minute, &pengguna, false)
	cfg := usecase.DefaultKonfigPembersihan()
	cfg.RetensiLoginAttempt = 0

	_, err := pbrPembersih(db, cfg).JalankanPada(context.Background(), pbrAcuan)

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"sangat-lama", "baru"}, pbrSisa(t, db, "SELECT failure_reason FROM login_attempts"))
}

func TestPembersihan_BatchBerulangSampaiHabisDanTerpotongMelanjutkan(t *testing.T) {
	db := pbrDB(t)
	const mati, hidup = 25, 5
	for i := 0; i < mati; i++ {
		pbrRefresh(t, db, fmt.Sprintf("mati-%02d", i), pbrUserA, pbrAcuan.Add(-time.Duration(i+1)*time.Minute), i%2 == 0)
	}
	for i := 0; i < hidup; i++ {
		pbrRefresh(t, db, fmt.Sprintf("hidup-%02d", i), pbrUserB, pbrAcuan.Add(time.Duration(i+1)*time.Hour), i%2 == 0)
	}
	pbrSisaMati := func() int {
		return len(pbrSisa(t, db, "SELECT token_hash FROM refresh_tokens WHERE token_hash LIKE 'mati-%'"))
	}
	cfg := usecase.DefaultKonfigPembersihan()
	cfg.UkuranBatch = 10
	cfg.MaksBatch = 2
	p := pbrPembersih(db, cfg)

	// Putaran 1 dipotong di 2 batch (20 baris); putaran 2 menuntaskan sisanya; putaran 3 tidak menemukan apa pun.
	h1, err := p.JalankanPada(context.Background(), pbrAcuan)
	require.NoError(t, err)
	r1 := h1.Langkah[1]
	assert.Equal(t, "refresh_tokens", r1.Nama)
	assert.EqualValues(t, 20, r1.Dihapus)
	assert.True(t, r1.Terpotong)
	assert.Equal(t, 5, pbrSisaMati())

	h2, err := p.JalankanPada(context.Background(), pbrAcuan)
	require.NoError(t, err)
	r2 := h2.Langkah[1]
	assert.EqualValues(t, 5, r2.Dihapus)
	assert.False(t, r2.Terpotong)
	assert.Equal(t, 0, pbrSisaMati())

	h3, err := p.JalankanPada(context.Background(), pbrAcuan)
	require.NoError(t, err)
	assert.EqualValues(t, 0, h3.Total(), "idempoten: putaran ulang tidak menghapus apa pun")
	assert.Len(t, pbrSisa(t, db, "SELECT token_hash FROM refresh_tokens WHERE token_hash LIKE 'hidup-%'"), hidup)
}

func TestPembersihan_BarisDikunciTransaksiLainDilewatiBukanDitunggu(t *testing.T) {
	db := pbrDB(t)
	pbrRefresh(t, db, "mati-terkunci", pbrUserA, pbrAcuan.Add(-time.Hour), false)
	pbrRefresh(t, db, "mati-bebas", pbrUserA, pbrAcuan.Add(-time.Hour), false)

	// Transaksi lain (mis. Logout yang mencabut token) memegang kunci baris "mati-terkunci".
	tx := db.Begin()
	require.NoError(t, tx.Error)
	require.NoError(t, tx.Exec("UPDATE refresh_tokens SET revoked_at = ? WHERE token_hash = 'mati-terkunci'", pbrAcuan).Error)

	selesai := make(chan error, 1)
	go func() {
		_, err := pbrPembersih(db, usecase.DefaultKonfigPembersihan()).JalankanPada(context.Background(), pbrAcuan)
		selesai <- err
	}()
	select {
	case err := <-selesai:
		require.NoError(t, err, "pembersihan tidak boleh menunggu kunci baris")
	case <-time.After(10 * time.Second):
		require.NoError(t, tx.Rollback().Error)
		t.Fatal("pembersihan tersangkut menunggu baris yang dikunci")
	}
	assert.Equal(t, []string{"mati-terkunci"}, pbrSisa(t, db, "SELECT token_hash FROM refresh_tokens"))

	require.NoError(t, tx.Commit().Error)
	_, err := pbrPembersih(db, usecase.DefaultKonfigPembersihan()).JalankanPada(context.Background(), pbrAcuan)
	require.NoError(t, err)
	assert.Empty(t, pbrSisa(t, db, "SELECT token_hash FROM refresh_tokens"), "putaran berikutnya menyapu baris yang tadi dilewati")
}

func TestPembersihan_RepositoriMenolakBatchTidakValid(t *testing.T) {
	db := pbrDB(t)
	repo := authpg.New(db)
	_, err := repo.HapusBlacklistKedaluwarsa(context.Background(), pbrAcuan, 0)
	require.Error(t, err)
	_, err = repo.HapusLoginAttemptLama(context.Background(), pbrAcuan, -1)
	require.Error(t, err)
}

func TestPembersihan_KonteksSelesaiMenghentikanDenganGalat(t *testing.T) {
	db := pbrDB(t)
	pbrRefresh(t, db, "mati", pbrUserA, pbrAcuan.Add(-time.Hour), false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := pbrPembersih(db, usecase.DefaultKonfigPembersihan()).JalankanPada(ctx, pbrAcuan)

	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, []string{"mati"}, pbrSisa(t, db, "SELECT token_hash FROM refresh_tokens"))
}

// Setelah pembersihan, alur sungguhan tetap utuh: token keluarga yang masih berlaku dapat dirotasi, dan
// token lama yang sudah dicabut (belum kedaluwarsa) tetap terdeteksi sebagai pemakaian ulang.
func TestPembersihan_AlurRefreshTetapUtuhSetelahPembersihan(t *testing.T) {
	s := setupSuite(t)
	// Sisipkan baris mati yang harus ikut tersapu bersama baris sungguhan di basis data yang sama.
	pbrRefresh(t, s.db, "mati-lama", uid("test-user-001"), time.Now().Add(-time.Hour), true)
	pbrReset(t, s.db, "reset-mati", uid("test-user-001"), time.Now().Add(-30*24*time.Hour), nil)

	_, lama := s.loginAs(t, "test@gardainflasi.id", "Test1234!")
	rotasi := s.serveRequest(t, http.MethodPost, "/v1/auth/refresh", map[string]any{"refresh_token": lama}, nil)
	require.Equal(t, http.StatusOK, rotasi.Code, rotasi.Body.String())
	baru, _ := s.parseResponse(t, rotasi)["data"].(map[string]any)["refresh_token"].(string)
	require.NotEmpty(t, baru)

	hasil, err := pbrPembersih(s.db, usecase.DefaultKonfigPembersihan()).Jalankan(context.Background())

	require.NoError(t, err)
	assert.EqualValues(t, 2, hasil.Total(), "hanya dua baris mati yang tersapu: %+v", hasil.Langkah)
	var ada int64
	require.NoError(t, s.db.Raw("SELECT count(*) FROM refresh_tokens WHERE token_hash IN ('mati-lama')").Scan(&ada).Error)
	assert.Zero(t, ada)

	// Di luar jendela toleransi tab serentak (lihat 01-E), pemakaian ulang token lama adalah pencurian.
	s.ageRotation(t, lama)
	pakaiLagi := s.serveRequest(t, http.MethodPost, "/v1/auth/refresh", map[string]any{"refresh_token": lama}, nil)
	assert.Equal(t, http.StatusUnauthorized, pakaiLagi.Code, "token lama yang dicabut tetap terdeteksi sebagai pemakaian ulang")
	// Deteksi pemakaian ulang mencabut seluruh keluarga, termasuk token baru.
	lagi := s.serveRequest(t, http.MethodPost, "/v1/auth/refresh", map[string]any{"refresh_token": baru}, nil)
	assert.Equal(t, http.StatusUnauthorized, lagi.Code)
}

func TestPembersihan_ResetTokenSahTetapBerlaku(t *testing.T) {
	s := setupSuite(t)
	s.simpanTokenReset(t, "token-sah", time.Now().Add(30*time.Minute))
	s.simpanTokenReset(t, "token-lama", time.Now().Add(-30*24*time.Hour))

	hasil, err := pbrPembersih(s.db, usecase.DefaultKonfigPembersihan()).Jalankan(context.Background())

	require.NoError(t, err)
	assert.EqualValues(t, 1, hasil.Total())
	w := s.reset(t, "token-sah", passwordBaru)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	lama := s.reset(t, "token-lama", passwordBaru)
	assert.Equal(t, http.StatusUnauthorized, lama.Code)
}
