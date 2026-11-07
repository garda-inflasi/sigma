package integration

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/auth/routes"
	"github.com/garda-inflasi/api/internal/auth/usecase"
	"github.com/garda-inflasi/api/pkg/middleware"
	"github.com/garda-inflasi/api/pkg/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	emailMFA   = "uji.mfa@gardainflasi.id"
	emailBiasa = "uji.biasa@gardainflasi.id"
)

// decodeData membaca isi data dari amplop {code, message, data} ke out.
func (s *Suite) decodeData(t *testing.T, body []byte, out any) {
	t.Helper()
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope), string(body))
	require.NoError(t, json.Unmarshal(envelope.Data, out), string(body))
}

// kodeOTP membuat kode TOTP untuk rahasia pada waktu t.
func kodeOTP(t *testing.T, secret string, waktu time.Time) string {
	t.Helper()
	code, err := totp.GenerateTOTP(secret, waktu)
	require.NoError(t, err)
	return code
}

type loginMFA struct {
	MFARequired bool   `json:"mfa_required"`
	MFAToken    string `json:"mfa_token"`
	AccessToken string `json:"access_token"`
}

// mulaiLoginMFA login dengan kata sandi dan mengembalikan token tantangan.
func (s *Suite) mulaiLoginMFA(t *testing.T, email string) string {
	t.Helper()
	w := s.serveRequest(t, http.MethodPost, "/v1/auth/login", map[string]any{"email": email, "password": "Test1234!"}, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var data loginMFA
	s.decodeData(t, w.Body.Bytes(), &data)
	require.True(t, data.MFARequired, w.Body.String())
	require.NotEmpty(t, data.MFAToken)
	require.Empty(t, data.AccessToken, "belum ada sesi sebelum faktor kedua")
	return data.MFAToken
}

func TestIntegration_MFALogin_ChallengeThenSession(t *testing.T) {
	s := setupSuite(t)
	challenge := s.mulaiLoginMFA(t, emailMFA)

	t.Run("token tantangan bukan kredensial sesi", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodGet, "/v1/auth/me", nil, s.authHeader(challenge))
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
	t.Run("tanpa token tantangan atau kode ditolak", func(t *testing.T) {
		assert.Equal(t, http.StatusBadRequest, s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/validate", map[string]any{"code": "123456"}, nil).Code)
		assert.Equal(t, http.StatusUnauthorized, s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/validate", map[string]any{"mfa_token": "palsu", "code": "123456"}, nil).Code)
	})
	t.Run("kode TOTP yang benar menerbitkan sesi penuh", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/validate",
			map[string]any{"mfa_token": challenge, "code": kodeOTP(t, mfaSecretUji, time.Now())}, nil)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var data struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			User         struct {
				Peran string `json:"peran"`
			} `json:"user"`
		}
		s.decodeData(t, w.Body.Bytes(), &data)
		require.NotEmpty(t, data.AccessToken)
		assert.NotEmpty(t, data.RefreshToken)
		assert.Equal(t, "OPS_KOORDINATOR", data.User.Peran)
		assert.Equal(t, http.StatusOK, s.serveRequest(t, http.MethodGet, "/v1/auth/me", nil, s.authHeader(data.AccessToken)).Code)
	})
	t.Run("kode yang sama tidak berlaku dua kali", func(t *testing.T) {
		ulang := s.mulaiLoginMFA(t, emailMFA)
		w := s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/validate",
			map[string]any{"mfa_token": ulang, "code": kodeOTP(t, mfaSecretUji, time.Now())}, nil)
		assert.Equal(t, http.StatusUnauthorized, w.Code, "langkah waktu itu sudah terpakai")

		berikut := s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/validate",
			map[string]any{"mfa_token": ulang, "code": kodeOTP(t, mfaSecretUji, time.Now().Add(totp.DefaultPeriod*time.Second))}, nil)
		assert.Equal(t, http.StatusOK, berikut.Code, "kode langkah berikutnya diterima: "+berikut.Body.String())
	})
	t.Run("login dan MFA tercatat di audit", func(t *testing.T) {
		var n int64
		require.NoError(t, s.db.Raw("SELECT count(*) FROM audit_log WHERE aksi = 'LOGIN' AND aktor_id = garda_uuid_stabil('pengguna', 'test-user-mfa') AND hasil = 'BERHASIL' AND metadata->>'mfa' = 'totp'").Scan(&n).Error)
		assert.EqualValues(t, 2, n)
	})
}

func TestIntegration_MFALogin_WrongCodesLockTheAccount(t *testing.T) {
	s := setupSuite(t)
	challenge := s.mulaiLoginMFA(t, emailMFA)

	for i := 0; i < 5; i++ {
		w := s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/validate", map[string]any{"mfa_token": challenge, "code": "000000"}, nil)
		require.Equal(t, http.StatusUnauthorized, w.Code, "percobaan %d", i+1)
	}

	w := s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/validate",
		map[string]any{"mfa_token": challenge, "code": kodeOTP(t, mfaSecretUji, time.Now())}, nil)
	assert.Equal(t, http.StatusLocked, w.Code, "kode benar pun ditolak saat akun terkunci")
}

func TestIntegration_MFALogin_BackupCodes(t *testing.T) {
	s := setupSuite(t)

	pakai := func(kode string) int {
		challenge := s.mulaiLoginMFA(t, emailMFA)
		return s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/validate", map[string]any{"mfa_token": challenge, "code": kode}, nil).Code
	}

	assert.Equal(t, http.StatusOK, pakai("BACKUP01"))
	assert.Equal(t, http.StatusUnauthorized, pakai("BACKUP01"), "kode cadangan hanya sekali pakai")
	assert.Equal(t, http.StatusOK, pakai("backup02"), "huruf kecil diterima")
	assert.Equal(t, http.StatusUnauthorized, pakai("TIDAKADA99"))

	var terpakai int64
	require.NoError(t, s.db.Raw("SELECT count(*) FROM mfa_kode_cadangan WHERE user_id = garda_uuid_stabil('pengguna', 'test-user-mfa') AND dipakai_pada IS NOT NULL").Scan(&terpakai).Error)
	assert.EqualValues(t, 2, terpakai)
}

// Kode cadangan yang terbit sebelum migrasi 000110 di-hash dari "<id lama>:<kode>" dan tidak bisa dihitung ulang
// saat ID menjadi UUID. Akun hasil migrasi punya baris di pengguna_id_lama; kode lamanya tetap berlaku, sekali pakai.
func TestIntegration_MFALogin_KodeCadanganSebelumMigrasiUUIDTetapBerlaku(t *testing.T) {
	s := setupSuite(t)
	require.NoError(t, s.db.Exec("DELETE FROM mfa_kode_cadangan WHERE user_id = garda_uuid_stabil('pengguna', 'test-user-mfa')").Error)
	require.NoError(t, s.db.Exec("INSERT INTO pengguna_id_lama (id, id_lama) VALUES (garda_uuid_stabil('pengguna', 'test-user-mfa'), 'test-user-mfa')").Error)
	require.NoError(t, s.db.Exec("INSERT INTO mfa_kode_cadangan (user_id, kode_hash) VALUES (garda_uuid_stabil('pengguna', 'test-user-mfa'), ?)",
		usecase.HashKodeCadangan("test-user-mfa", "LAMA0001")).Error)

	pakai := func(kode string) int {
		challenge := s.mulaiLoginMFA(t, emailMFA)
		return s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/validate", map[string]any{"mfa_token": challenge, "code": kode}, nil).Code
	}

	assert.Equal(t, http.StatusOK, pakai("LAMA0001"), "kode lama dicocokkan dengan ID lama akun")
	assert.Equal(t, http.StatusUnauthorized, pakai("LAMA0001"), "tetap sekali pakai")
	assert.Equal(t, http.StatusUnauthorized, pakai("LAMA9999"), "kode yang bukan miliknya ditolak")
}

func TestIntegration_MFASetup_ActivationFlow(t *testing.T) {
	s := setupSuite(t)
	access, _ := s.loginAs(t, "test@gardainflasi.id", "Test1234!")

	var setup struct {
		Secret string `json:"secret"`
		URI    string `json:"provisioning_uri"`
		QRSVG  string `json:"qr_svg"`
	}
	w := s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/setup", nil, s.authHeader(access))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	s.decodeData(t, w.Body.Bytes(), &setup)
	assert.NotEmpty(t, setup.Secret)
	assert.Contains(t, setup.URI, "otpauth://totp/")
	assert.True(t, strings.HasPrefix(setup.QRSVG, "<svg "), "QR berupa SVG")

	t.Run("rahasia tersimpan bersegel, bukan polos", func(t *testing.T) {
		var tersimpan string
		require.NoError(t, s.db.Raw("SELECT mfa_secret FROM pengguna WHERE id = garda_uuid_stabil('pengguna', 'test-user-001')").Scan(&tersimpan).Error)
		assert.True(t, strings.HasPrefix(tersimpan, "v1:"), tersimpan)
		assert.NotContains(t, tersimpan, setup.Secret)
	})
	t.Run("kode salah ditolak dan MFA belum aktif", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/verify", map[string]any{"code": "000000"}, s.authHeader(access))
		assert.Equal(t, http.StatusBadRequest, w.Code)
		s.loginAs(t, "test@gardainflasi.id", "Test1234!")
	})

	w = s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/verify", map[string]any{"code": kodeOTP(t, setup.Secret, time.Now())}, s.authHeader(access))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var aktif struct {
		AccessToken  string   `json:"access_token"`
		RefreshToken string   `json:"refresh_token"`
		KodeCadangan []string `json:"kode_cadangan"`
	}
	s.decodeData(t, w.Body.Bytes(), &aktif)
	require.Len(t, aktif.KodeCadangan, 10)
	assert.NotEmpty(t, aktif.AccessToken)

	t.Run("kode cadangan tersimpan sebagai hash", func(t *testing.T) {
		var hashes []string
		require.NoError(t, s.db.Raw("SELECT kode_hash FROM mfa_kode_cadangan WHERE user_id = garda_uuid_stabil('pengguna', 'test-user-001')").Scan(&hashes).Error)
		require.Len(t, hashes, 10)
		for _, h := range hashes {
			assert.Len(t, h, 64)
			for _, kode := range aktif.KodeCadangan {
				assert.NotContains(t, h, strings.ReplaceAll(kode, "-", ""))
			}
		}
	})
	t.Run("sesi lama dicabut, sesi baru berlaku", func(t *testing.T) {
		assert.Equal(t, http.StatusOK, s.serveRequest(t, http.MethodGet, "/v1/auth/me", nil, s.authHeader(aktif.AccessToken)).Code)
	})
	t.Run("profil menampilkan MFA aktif dan sisa kode cadangan", func(t *testing.T) {
		var me struct {
			MfaAktif            bool `json:"mfa_aktif"`
			KodeCadanganTersisa int  `json:"kode_cadangan_tersisa"`
		}
		s.decodeData(t, s.serveRequest(t, http.MethodGet, "/v1/auth/me", nil, s.authHeader(aktif.AccessToken)).Body.Bytes(), &me)
		assert.True(t, me.MfaAktif)
		assert.Equal(t, 10, me.KodeCadanganTersisa)
	})
	t.Run("login berikutnya meminta kode", func(t *testing.T) {
		s.mulaiLoginMFA(t, "test@gardainflasi.id")
	})
	t.Run("setup ulang ditolak saat sudah aktif", func(t *testing.T) {
		assert.Equal(t, http.StatusConflict, s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/setup", nil, s.authHeader(aktif.AccessToken)).Code)
	})
	t.Run("kode cadangan baru menghanguskan yang lama", func(t *testing.T) {
		salah := s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/backup-codes", map[string]any{"password": "SalahSandi1!"}, s.authHeader(aktif.AccessToken))
		assert.Equal(t, http.StatusUnauthorized, salah.Code)

		w := s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/backup-codes", map[string]any{"password": "Test1234!"}, s.authHeader(aktif.AccessToken))
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var baru struct {
			KodeCadangan []string `json:"kode_cadangan"`
		}
		s.decodeData(t, w.Body.Bytes(), &baru)
		require.Len(t, baru.KodeCadangan, 10)

		challenge := s.mulaiLoginMFA(t, "test@gardainflasi.id")
		lama := s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/validate", map[string]any{"mfa_token": challenge, "code": aktif.KodeCadangan[0]}, nil)
		assert.Equal(t, http.StatusUnauthorized, lama.Code)
		baruOK := s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/validate", map[string]any{"mfa_token": challenge, "code": baru.KodeCadangan[0]}, nil)
		assert.Equal(t, http.StatusOK, baruOK.Code, baruOK.Body.String())
	})
	t.Run("nonaktifkan memerlukan kata sandi", func(t *testing.T) {
		assert.Equal(t, http.StatusUnauthorized, s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/disable", map[string]any{"password": "SalahSandi1!"}, s.authHeader(aktif.AccessToken)).Code)
		assert.Equal(t, http.StatusOK, s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/disable", map[string]any{"password": "Test1234!"}, s.authHeader(aktif.AccessToken)).Code)

		s.loginAs(t, "test@gardainflasi.id", "Test1234!")
		var sisa int64
		require.NoError(t, s.db.Raw("SELECT count(*) FROM mfa_kode_cadangan WHERE user_id = garda_uuid_stabil('pengguna', 'test-user-001')").Scan(&sisa).Error)
		assert.Zero(t, sisa, "kode cadangan ikut dihapus")
	})
}

func TestIntegration_MFASetup_RequiresAuthentication(t *testing.T) {
	s := setupSuite(t)

	assert.Equal(t, http.StatusUnauthorized, s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/setup", nil, nil).Code)
	assert.Equal(t, http.StatusUnauthorized, s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/verify", map[string]any{"code": "123456"}, nil).Code)
	assert.Equal(t, http.StatusUnauthorized, s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/disable", map[string]any{"password": "x"}, nil).Code)
	assert.Equal(t, http.StatusUnauthorized, s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/backup-codes", map[string]any{"password": "x"}, nil).Code)
}

// penegakan adalah konfigurasi produksi: MFA wajib bagi peran wajib dan akun bertanda mfa_wajib.
func penegakan(t *testing.T) routes.Options {
	t.Helper()
	t.Setenv("MFA_ENFORCE", "true")
	return routes.Options{UseCaseConfig: routes.ConfigFromEnv(os.Getenv)}
}

func TestIntegration_MFAEnforced_OnlySetupEndpointsBeforeSetup(t *testing.T) {
	s := newSuite(t, seedData, penegakan(t))

	// ADMIN_SISTEM termasuk peran wajib MFA.
	access, refresh := s.loginAs(t, "test@gardainflasi.id", "Test1234!")

	t.Run("login menandai wajib setup", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodPost, "/v1/auth/login", map[string]any{"email": "test@gardainflasi.id", "password": "Test1234!"}, nil)
		var data struct {
			MFASetupDiwajibkan bool `json:"mfa_setup_diwajibkan"`
		}
		s.decodeData(t, w.Body.Bytes(), &data)
		assert.True(t, data.MFASetupDiwajibkan)
	})
	t.Run("endpoint lain dijawab 403 MFA_SETUP_REQUIRED", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodGet, "/v1/users", nil, s.authHeader(access))
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, middleware.MFASetupRequiredCode, s.parseResponse(t, w)["code"])
	})
	t.Run("profil tetap terbuka dan menyebut kewajiban", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodGet, "/v1/auth/me", nil, s.authHeader(access))
		require.Equal(t, http.StatusOK, w.Code)
		data := s.parseResponse(t, w)["data"].(map[string]any)
		assert.Equal(t, true, data["mfa_setup_diwajibkan"])
		assert.Equal(t, true, data["mfa_wajib"])
	})
	t.Run("refresh mempertahankan pembatasan", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodPost, "/v1/auth/refresh", map[string]any{"refresh_token": refresh}, nil)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var data struct {
			AccessToken string `json:"access_token"`
		}
		s.decodeData(t, w.Body.Bytes(), &data)
		users := s.serveRequest(t, http.MethodGet, "/v1/users", nil, s.authHeader(data.AccessToken))
		assert.Equal(t, http.StatusForbidden, users.Code)
	})

	var setup struct {
		Secret string `json:"secret"`
	}
	w := s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/setup", nil, s.authHeader(access))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	s.decodeData(t, w.Body.Bytes(), &setup)

	w = s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/verify", map[string]any{"code": kodeOTP(t, setup.Secret, time.Now())}, s.authHeader(access))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var aktif struct {
		AccessToken string `json:"access_token"`
	}
	s.decodeData(t, w.Body.Bytes(), &aktif)

	t.Run("setelah aktivasi sesi baru membuka semua endpoint", func(t *testing.T) {
		assert.Equal(t, http.StatusOK, s.serveRequest(t, http.MethodGet, "/v1/users", nil, s.authHeader(aktif.AccessToken)).Code)
	})
	t.Run("MFA yang diwajibkan tidak bisa dinonaktifkan", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/disable", map[string]any{"password": "Test1234!"}, s.authHeader(aktif.AccessToken))
		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}

func TestIntegration_MFAEnforced_NonMandatoryRolesAreUnaffected(t *testing.T) {
	s := newSuite(t, seedData, penegakan(t))

	access, _ := s.loginAs(t, emailBiasa, "Test1234!")

	assert.Equal(t, http.StatusOK, s.serveRequest(t, http.MethodGet, "/v1/harga/saya", nil, s.authHeader(access)).Code)
	me := s.parseResponse(t, s.serveRequest(t, http.MethodGet, "/v1/auth/me", nil, s.authHeader(access)))["data"].(map[string]any)
	assert.Equal(t, false, me["mfa_setup_diwajibkan"])
}

func TestIntegration_MFAEnforced_BulkWajibkanMFAAppliesToAnyRole(t *testing.T) {
	s := newSuite(t, seedData, penegakan(t))
	require.NoError(t, s.db.Exec("UPDATE pengguna SET mfa_wajib = true WHERE id = garda_uuid_stabil('pengguna', 'test-user-biasa')").Error)

	access, _ := s.loginAs(t, emailBiasa, "Test1234!")

	w := s.serveRequest(t, http.MethodGet, "/v1/harga/saya", nil, s.authHeader(access))
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Equal(t, middleware.MFASetupRequiredCode, s.parseResponse(t, w)["code"])
}

func TestIntegration_MFAEnforced_PasswordChangeAndMFASetupCompose(t *testing.T) {
	s := newSuite(t, seedAdminAwal, penegakan(t))
	access, _ := s.loginAs(t, adminEmail, adminTempSandi)

	blocked := s.serveRequest(t, http.MethodGet, "/v1/users", nil, s.authHeader(access))
	assert.Equal(t, middleware.PasswordChangeRequiredCode, s.parseResponse(t, blocked)["code"], "ganti kata sandi didahulukan")
	assert.Equal(t, http.StatusForbidden, s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/setup", nil, s.authHeader(access)).Code)

	w := s.serveRequest(t, http.MethodPost, "/v1/auth/change-password", map[string]any{"old_password": adminTempSandi, "new_password": adminNewPassword}, s.authHeader(access))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var sesi struct {
		AccessToken string `json:"access_token"`
	}
	s.decodeData(t, w.Body.Bytes(), &sesi)

	stillBlocked := s.serveRequest(t, http.MethodGet, "/v1/users", nil, s.authHeader(sesi.AccessToken))
	assert.Equal(t, middleware.MFASetupRequiredCode, s.parseResponse(t, stillBlocked)["code"], "setelah ganti kata sandi tinggal setup MFA")
	assert.Equal(t, http.StatusOK, s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/setup", nil, s.authHeader(sesi.AccessToken)).Code)
}

func TestConfigFromEnv(t *testing.T) {
	env := func(values map[string]string) func(string) string {
		return func(key string) string { return values[key] }
	}

	assert.False(t, routes.ConfigFromEnv(env(nil)).MFAEnforce, "lokal/demo: tidak dipaksa")
	assert.True(t, routes.ConfigFromEnv(env(map[string]string{"APP_ENV": "production", "AES_KEY": "abcdefghijklmnopqrstuvwxyz012345"})).MFAEnforce, "produksi: dipaksa")
	assert.False(t, routes.ConfigFromEnv(env(map[string]string{"APP_ENV": "production", "MFA_ENFORCE": "false", "AES_KEY": "abcdefghijklmnopqrstuvwxyz012345"})).MFAEnforce, "ditimpa eksplisit")
	assert.True(t, routes.ConfigFromEnv(env(map[string]string{"MFA_ENFORCE": "true"})).MFAEnforce)
	assert.NotNil(t, routes.ConfigFromEnv(env(nil)).MFAKey)

	assert.Panics(t, func() { routes.ConfigFromEnv(env(map[string]string{"MFA_ENFORCE": "mungkin"})) }, "nilai tidak dikenal menghentikan start")
	assert.Panics(t, func() { routes.ConfigFromEnv(env(map[string]string{"APP_ENV": "production"})) }, "produksi tanpa AES_KEY menghentikan start")
}
