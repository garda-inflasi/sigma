package integration

import (
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/auth/usecase"
	"github.com/garda-inflasi/api/pkg/crypto"
	"github.com/garda-inflasi/api/pkg/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Reset MFA oleh admin (POST /v1/users/:id/reset-mfa). Admin: test-user-001 (ADMIN_SISTEM, tanpa MFA).
// Korban: test-user-mfa (OPS_KOORDINATOR, MFA aktif, tiga kode cadangan) dan test-user-biasa (tanpa MFA).

var idMFA = uid("test-user-mfa")

type hasilReset struct {
	ID                 string `json:"id"`
	MFAAktifSebelumnya bool   `json:"mfa_aktif_sebelumnya"`
	SesiDicabut        int    `json:"sesi_dicabut"`
}

func (s *Suite) resetMFA(t *testing.T, token, id string) *httptest.ResponseRecorder {
	t.Helper()
	return s.panggil(t, http.MethodPost, "/v1/users/"+id+"/reset-mfa", nil, token)
}

func (s *Suite) resetMFAOk(t *testing.T, token, id string) hasilReset {
	t.Helper()
	w := s.resetMFA(t, token, id)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var h hasilReset
	s.decodeData(t, w.Body.Bytes(), &h)
	return h
}

// validasiMFA menukar token tantangan dan kode dengan sesi penuh.
func (s *Suite) validasiMFA(t *testing.T, email, kode string) sesiUji {
	t.Helper()
	challenge := s.mulaiLoginMFA(t, email)
	w := s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/validate", map[string]any{"mfa_token": challenge, "code": kode}, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var data struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	s.decodeData(t, w.Body.Bytes(), &data)
	claims, err := s.tokenMgr.VerifyToken(data.AccessToken)
	require.NoError(t, err)
	return sesiUji{access: data.AccessToken, refresh: data.RefreshToken, sid: claims.SessionID}
}

type keadaanMFA struct {
	MfaAktif           bool
	AdaRahasia         bool
	MfaLangkahTerakhir int64
	MfaWajib           bool
	Diupdate           time.Time
	KodeCadangan       int64
}

func (s *Suite) keadaanMFA(t *testing.T, id string) keadaanMFA {
	t.Helper()
	var k keadaanMFA
	require.NoError(t, s.db.Raw(`SELECT mfa_aktif, (mfa_secret IS NOT NULL) AS ada_rahasia, mfa_langkah_terakhir, mfa_wajib, diupdate_pada AS diupdate,
		(SELECT count(*) FROM mfa_kode_cadangan WHERE user_id = pengguna.id) AS kode_cadangan FROM pengguna WHERE id = ?`, id).Scan(&k).Error)
	return k
}

func TestResetMFA_MenghapusSemuaBahanMFADanMencabutSemuaSesi(t *testing.T) {
	s := setupSuite(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)

	// Dua sesi aktif korban: satu lewat OTP, satu lewat kode cadangan.
	s1 := s.validasiMFA(t, emailMFA, kodeOTP(t, mfaSecretUji, time.Now()))
	s2 := s.validasiMFA(t, emailMFA, "BACKUP01")
	require.Equal(t, http.StatusOK, s.status(t, s1.access))
	require.Equal(t, http.StatusOK, s.status(t, s2.access))
	awal := s.keadaanMFA(t, idMFA)
	require.True(t, awal.MfaAktif)
	require.True(t, awal.AdaRahasia)
	require.Positive(t, awal.MfaLangkahTerakhir, "langkah TOTP anti-ulang terisi setelah login OTP")
	require.EqualValues(t, 3, awal.KodeCadangan)

	hasil := s.resetMFAOk(t, admin.access, idMFA)

	assert.Equal(t, hasilReset{ID: idMFA, MFAAktifSebelumnya: true, SesiDicabut: 2}, hasil)

	t.Run("rahasia, langkah TOTP, dan kode cadangan terhapus", func(t *testing.T) {
		k := s.keadaanMFA(t, idMFA)
		assert.False(t, k.MfaAktif)
		assert.False(t, k.AdaRahasia, "rahasia TOTP terenkripsi dikosongkan")
		assert.Zero(t, k.MfaLangkahTerakhir, "langkah anti-ulang direset")
		assert.Zero(t, k.KodeCadangan, "kode cadangan (terpakai maupun belum) dihapus")
	})
	t.Run("semua sesi korban mati seketika", func(t *testing.T) {
		for _, sesi := range []sesiUji{s1, s2} {
			assert.Equal(t, http.StatusUnauthorized, s.status(t, sesi.access), "access token lama langsung ditolak")
			assert.Equal(t, http.StatusUnauthorized, s.refreshRaw(t, sesi.refresh).Code, "refresh token lama ditolak")
			assert.EqualValues(t, 0, s.liveTokens(t, sesi.sid))
		}
		assert.Equal(t, http.StatusOK, s.status(t, admin.access), "sesi admin tidak ikut tercabut")
	})
	t.Run("tercatat di audit dengan sebelum dan sesudah", func(t *testing.T) {
		var baris []struct {
			AktorID   string
			EntitasID string
			Hasil     string
			Sebelum   string
			Sesudah   string
			Metadata  string
		}
		require.NoError(t, s.db.Raw(`SELECT aktor_id, entitas_id, hasil, sebelum::text AS sebelum, sesudah::text AS sesudah, metadata::text AS metadata
			FROM audit_log WHERE aksi = 'RESET_MFA'`).Scan(&baris).Error)
		require.Len(t, baris, 1)
		assert.Equal(t, uid("test-user-001"), baris[0].AktorID)
		assert.Equal(t, idMFA, baris[0].EntitasID)
		assert.Equal(t, "BERHASIL", baris[0].Hasil)
		assert.JSONEq(t, `{"mfa_aktif": true}`, baris[0].Sebelum)
		assert.JSONEq(t, `{"mfa_aktif": false}`, baris[0].Sesudah)
		assert.JSONEq(t, `{"mfa_aktif_sebelumnya": true, "rahasia_dihapus": true, "kode_cadangan_dihapus": 3, "sesi_dicabut": 2}`, baris[0].Metadata)
		assert.NotContains(t, baris[0].Metadata+baris[0].Sebelum+baris[0].Sesudah, mfaSecretUji, "rahasia tidak masuk audit")
	})
	t.Run("daftar dan detail menampilkan MFA mati", func(t *testing.T) {
		w := s.panggil(t, http.MethodGet, "/v1/users/"+idMFA, nil, admin.access)
		require.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, false, s.dataObj(t, w)["mfa_aktif"])
		for _, filter := range []struct {
			query string
			ada   bool
		}{{"mfa=true", false}, {"mfa=false", true}} {
			ids := s.daftarSemua(t, admin.access, filter.query).ids()
			assert.Equal(t, filter.ada, containsStr(ids, idMFA), filter.query)
		}
	})
	t.Run("login berikutnya cukup kata sandi", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodPost, "/v1/auth/login", map[string]any{"email": emailMFA, "password": "Test1234!"}, nil)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var data loginMFA
		s.decodeData(t, w.Body.Bytes(), &data)
		assert.False(t, data.MFARequired)
		assert.NotEmpty(t, data.AccessToken)
	})
	t.Run("pengguna dapat mendaftarkan autentikator baru", func(t *testing.T) {
		akses, _ := s.loginAs(t, emailMFA, "Test1234!")
		w := s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/setup", nil, s.authHeader(akses))
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var setup struct {
			Secret string `json:"secret"`
		}
		s.decodeData(t, w.Body.Bytes(), &setup)
		assert.NotEqual(t, mfaSecretUji, setup.Secret, "rahasia baru, bukan yang lama")

		w = s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/verify", map[string]any{"code": kodeOTP(t, setup.Secret, time.Now())}, s.authHeader(akses))
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.True(t, s.keadaanMFA(t, idMFA).MfaAktif)
		assert.EqualValues(t, usecase.JumlahKodeCadangan, s.keadaanMFA(t, idMFA).KodeCadangan)
	})
}

func containsStr(daftar []string, nilai string) bool {
	for _, d := range daftar {
		if d == nilai {
			return true
		}
	}
	return false
}

func TestResetMFA_Idempoten(t *testing.T) {
	s := setupSuite(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	s1 := s.validasiMFA(t, emailMFA, kodeOTP(t, mfaSecretUji, time.Now()))

	pertama := s.resetMFAOk(t, admin.access, idMFA)
	assert.True(t, pertama.MFAAktifSebelumnya)
	assert.Equal(t, 1, pertama.SesiDicabut)
	setelahPertama := s.keadaanMFA(t, idMFA)

	// Pengguna masuk lagi (tanpa MFA) lalu admin mengulang reset: tidak ada yang berubah, sesi barunya selamat.
	baru := s.mulaiSesi(t, emailMFA, uaIPhone)
	kedua := s.resetMFAOk(t, admin.access, idMFA)

	assert.Equal(t, hasilReset{ID: idMFA, MFAAktifSebelumnya: false, SesiDicabut: 0}, kedua)
	assert.Equal(t, http.StatusOK, s.status(t, baru.access), "reset kedua tidak mencabut sesi pengguna tanpa MFA")
	assert.Equal(t, http.StatusUnauthorized, s.status(t, s1.access))
	assert.Equal(t, setelahPertama, s.keadaanMFA(t, idMFA), "reset kedua tidak menulis ulang baris (diupdate_pada tetap)")

	var aktif []bool
	require.NoError(t, s.db.Raw(`SELECT (metadata->>'mfa_aktif_sebelumnya')::boolean FROM audit_log WHERE aksi = 'RESET_MFA' ORDER BY waktu`).Scan(&aktif).Error)
	assert.Equal(t, []bool{true, false}, aktif, "keduanya tercatat; yang kedua sebagai tanpa perubahan")
}

func TestResetMFA_PenggunaTanpaMFATidakDiganggu(t *testing.T) {
	s := setupSuite(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	biasa := s.mulaiSesi(t, emailBiasa, uaIPhone)
	sebelum := s.keadaanMFA(t, userBiasa)

	hasil := s.resetMFAOk(t, admin.access, userBiasa)

	assert.Equal(t, hasilReset{ID: userBiasa, MFAAktifSebelumnya: false, SesiDicabut: 0}, hasil)
	assert.Equal(t, http.StatusOK, s.status(t, biasa.access), "tanpa MFA yang dilepas, sesi tidak dicabut")
	assert.Equal(t, sebelum, s.keadaanMFA(t, userBiasa))
	assert.EqualValues(t, 1, s.auditCount(t, "RESET_MFA", uid("test-user-001"), userBiasa))
}

func TestResetMFA_SetupYangBelumDiverifikasiDibersihkan(t *testing.T) {
	s := setupSuite(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	biasa := s.mulaiSesi(t, emailBiasa, uaIPhone)

	w := s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/setup", nil, s.authHeader(biasa.access))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var setup struct {
		Secret string `json:"secret"`
	}
	s.decodeData(t, w.Body.Bytes(), &setup)
	require.True(t, s.keadaanMFA(t, userBiasa).AdaRahasia)

	hasil := s.resetMFAOk(t, admin.access, userBiasa)

	assert.False(t, hasil.MFAAktifSebelumnya)
	assert.Equal(t, 0, hasil.SesiDicabut)
	k := s.keadaanMFA(t, userBiasa)
	assert.False(t, k.AdaRahasia, "rahasia setup yang menggantung ikut dihapus")
	assert.Equal(t, http.StatusOK, s.status(t, biasa.access))
	// Verifikasi dengan rahasia lama tidak lagi bisa mengaktifkan MFA.
	w = s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/verify", map[string]any{"code": kodeOTP(t, setup.Secret, time.Now())}, s.authHeader(biasa.access))
	assert.Equal(t, http.StatusBadRequest, w.Code, "MFA belum disiapkan: mulai setup lagi")
	assert.False(t, s.keadaanMFA(t, userBiasa).MfaAktif)
}

func TestResetMFA_MenolakAkunSendiriTanpaMengubahApaPun(t *testing.T) {
	s := setupSuite(t)
	// Admin yang MFA-nya aktif: tidak boleh melucutinya lewat endpoint admin (jalur pemilik meminta kata sandi).
	ring, err := crypto.KeyringFromEnv(os.Getenv)
	require.NoError(t, err)
	sealed, err := ring.Seal(mfaSecretUji)
	require.NoError(t, err)
	require.NoError(t, s.db.Exec("UPDATE pengguna SET mfa_aktif = true, mfa_secret = ? WHERE id = garda_uuid_stabil('pengguna', 'test-user-001')", sealed).Error)
	require.NoError(t, s.db.Exec("INSERT INTO mfa_kode_cadangan (user_id, kode_hash) VALUES (garda_uuid_stabil('pengguna', 'test-user-001'), 'h1')").Error)
	admin := s.validasiMFA(t, emailAdmin, kodeOTP(t, mfaSecretUji, time.Now()))
	sebelum := s.keadaanMFA(t, uid("test-user-001"))

	w := s.resetMFA(t, admin.access, uid("test-user-001"))

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, s.parseResponse(t, w)["message"], "akun sendiri")
	assert.Equal(t, sebelum, s.keadaanMFA(t, uid("test-user-001")), "tidak ada yang berubah")
	assert.Equal(t, http.StatusOK, s.status(t, admin.access), "sesi admin tetap hidup")
	assert.Zero(t, s.hitung(t, "SELECT count(*) FROM audit_log WHERE aksi = 'RESET_MFA'"), "penolakan tidak tercatat sebagai reset")
}

func TestResetMFA_PenggunaTidakAdaDanOtorisasi(t *testing.T) {
	s := setupSuite(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	biasa := s.mulaiSesi(t, emailBiasa, uaIPhone)

	assert.Equal(t, http.StatusNotFound, s.resetMFA(t, admin.access, uid("tidak-ada")).Code)
	assert.Equal(t, http.StatusForbidden, s.resetMFA(t, biasa.access, idMFA).Code, "tanpa pengguna.kelola")
	assert.Equal(t, http.StatusUnauthorized, s.resetMFA(t, "", idMFA).Code, "tanpa token")
	assert.True(t, s.keadaanMFA(t, idMFA).MfaAktif, "korban tidak tersentuh oleh permintaan yang ditolak")
	assert.Zero(t, s.hitung(t, "SELECT count(*) FROM audit_log WHERE aksi = 'RESET_MFA'"))
}

func TestResetMFA_PermintaanSerentakHanyaSatuYangMelepas(t *testing.T) {
	s := setupSuite(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	s.validasiMFA(t, emailMFA, kodeOTP(t, mfaSecretUji, time.Now()))

	const n = 4
	hasil := make([]hasilReset, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := s.resetMFA(t, admin.access, idMFA)
			assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
			s.decodeData(t, w.Body.Bytes(), &hasil[i])
		}()
	}
	wg.Wait()

	var melepas, sesi int
	for _, h := range hasil {
		if h.MFAAktifSebelumnya {
			melepas++
		}
		sesi += h.SesiDicabut
	}
	assert.Equal(t, 1, melepas, "baris dikunci: tepat satu permintaan yang mendapati MFA aktif")
	assert.Equal(t, 1, sesi, "sesi dicabut sekali saja")
	assert.EqualValues(t, n, s.hitung(t, "SELECT count(*) FROM audit_log WHERE aksi = 'RESET_MFA'"))
}

// seedDuaAdminMFA menambah dua ADMIN_SISTEM ber-MFA (adm-a pelaku, adm-b korban) di atas seedData.
func seedDuaAdminMFA(t *testing.T, db *gorm.DB) {
	t.Helper()
	seedData(t, db)
	for _, slug := range []string{"adm-a", "adm-b"} {
		require.NoError(t, db.Exec(`INSERT INTO pengguna (id, nama, email, password_hash, peran, lapis, lembaga, status, mfa_aktif, mfa_secret)
			SELECT ?, ?, ?, password_hash, 'ADMIN_SISTEM', 'SISTEM', 'BPPSDMP', 'AKTIF', true, mfa_secret FROM pengguna WHERE id = garda_uuid_stabil('pengguna', 'test-user-mfa')`,
			uid(slug), "Admin "+slug, slug+"@gardainflasi.id").Error)
	}
}

// Peran wajib MFA tidak dilepas oleh reset: bila MFA diwajibkan lingkungan, login berikutnya korban masuk ke
// setup MFA (klaim mfa_setup_diwajibkan) dan hanya endpoint setup yang terbuka sampai ia mendaftarkan ulang.
func TestResetMFA_PeranWajibMFAMasukSetupSaatLoginBerikutnya(t *testing.T) {
	s := newSuite(t, seedDuaAdminMFA, penegakan(t))
	pelaku := s.validasiMFA(t, "adm-a@gardainflasi.id", kodeOTP(t, mfaSecretUji, time.Now()))
	korban := s.validasiMFA(t, "adm-b@gardainflasi.id", kodeOTP(t, mfaSecretUji, time.Now()))
	require.Equal(t, http.StatusOK, s.panggil(t, http.MethodGet, "/v1/users", nil, korban.access).Code, "korban admin penuh sebelum reset")

	hasil := s.resetMFAOk(t, pelaku.access, uid("adm-b"))
	assert.True(t, hasil.MFAAktifSebelumnya)
	assert.Equal(t, http.StatusUnauthorized, s.status(t, korban.access))

	w := s.serveRequest(t, http.MethodPost, "/v1/auth/login", map[string]any{"email": "adm-b@gardainflasi.id", "password": "Test1234!"}, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var login struct {
		MFARequired        bool   `json:"mfa_required"`
		AccessToken        string `json:"access_token"`
		MFASetupDiwajibkan bool   `json:"mfa_setup_diwajibkan"`
	}
	s.decodeData(t, w.Body.Bytes(), &login)
	assert.False(t, login.MFARequired, "tidak ada faktor kedua untuk ditantang")
	assert.True(t, login.MFASetupDiwajibkan, "peran wajib MFA diarahkan ke setup")

	terbatas := s.panggil(t, http.MethodGet, "/v1/users", nil, login.AccessToken)
	assert.Equal(t, http.StatusForbidden, terbatas.Code)
	assert.Equal(t, middleware.MFASetupRequiredCode, s.parseResponse(t, terbatas)["code"])
	assert.Equal(t, http.StatusOK, s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/setup", nil, s.authHeader(login.AccessToken)).Code, "endpoint setup terbuka")
}
