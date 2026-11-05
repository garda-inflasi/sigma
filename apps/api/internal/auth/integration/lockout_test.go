package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	authpg "github.com/garda-inflasi/api/internal/auth/repository/postgres"
	"github.com/garda-inflasi/api/internal/auth/routes"
	"github.com/garda-inflasi/api/internal/auth/usecase"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tes lockout (01-F). Korban tes memakai emailBiasa (uji.biasa) supaya admin uji (test@gardainflasi.id)
// tetap bisa masuk dan membuka kunci.

const (
	sandiBenarUji = "Test1234!"
	sandiSalahUji = "SalahTotal9!"
	idBiasa       = "test-user-biasa"
	idAdminUji    = "test-user-001"
)

func newRepo(s *Suite) *authpg.Repository { return authpg.New(s.db) }

// kunciConfig adalah konfigurasi produksi dengan batas dan durasi kunci yang diubah.
func kunciConfig(t *testing.T, maks int, durasi time.Duration) routes.Options {
	t.Helper()
	cfg := routes.ConfigFromEnv(os.Getenv)
	cfg.MaxFailedAttempts, cfg.LockoutDuration = maks, durasi
	return routes.Options{UseCaseConfig: cfg}
}

// coba mengirim satu percobaan login dan mengembalikan respons mentah.
func (s *Suite) coba(t *testing.T, email, sandi string) *httptest.ResponseRecorder {
	t.Helper()
	return s.serveRequest(t, http.MethodPost, "/v1/auth/login", map[string]any{"email": email, "password": sandi}, nil)
}

// galat adalah bentuk amplop galat dengan data kunci.
type galat struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    *struct {
		TerkunciSampai time.Time `json:"terkunci_sampai"`
		SisaDetik      int       `json:"sisa_detik"`
	} `json:"data"`
}

func (s *Suite) galatDari(t *testing.T, w *httptest.ResponseRecorder) galat {
	t.Helper()
	var g galat
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &g), w.Body.String())
	return g
}

// gagalkan mengirim n kata sandi salah dan menegaskan semuanya dijawab 401 INVALID_CREDENTIALS.
func (s *Suite) gagalkan(t *testing.T, email string, n int) {
	t.Helper()
	for i := 1; i <= n; i++ {
		w := s.coba(t, email, sandiSalahUji)
		require.Equal(t, http.StatusUnauthorized, w.Code, "percobaan gagal ke-%d: %s", i, w.Body.String())
		require.Equal(t, "INVALID_CREDENTIALS", s.galatDari(t, w).Code)
	}
}

type barisKunci struct {
	Gagal          int
	TerkunciSampai *time.Time
}

// kunciDi membaca baris kunci_login untuk email; nil bila tidak ada.
func (s *Suite) kunciDi(t *testing.T, email string) *barisKunci {
	t.Helper()
	var baris []barisKunci
	require.NoError(t, s.db.Raw("SELECT gagal, terkunci_sampai FROM kunci_login WHERE email_hash = ?", usecase.HashEmailKunci(email)).Scan(&baris).Error)
	if len(baris) == 0 {
		return nil
	}
	return &baris[0]
}

func (s *Suite) hitung(t *testing.T, sql string, args ...any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, s.db.Raw(sql, args...).Scan(&n).Error)
	return n
}

// bukaKunci memanggil endpoint admin dan mengembalikan respons.
func (s *Suite) bukaKunci(t *testing.T, token, id string) *httptest.ResponseRecorder {
	t.Helper()
	return s.serveRequest(t, http.MethodPost, "/v1/users/"+id+"/buka-kunci", nil, s.authHeader(token))
}

func TestIntegration_Lockout_AmbangAkhirKunciDanPesan(t *testing.T) {
	s := setupSuite(t)

	s.gagalkan(t, emailBiasa, 5)

	w := s.coba(t, emailBiasa, sandiBenarUji)
	require.Equal(t, http.StatusLocked, w.Code, w.Body.String())
	g := s.galatDari(t, w)
	assert.Equal(t, "ACCOUNT_LOCKED", g.Code)
	assert.NotEmpty(t, g.Message)
	require.NotNil(t, g.Data)
	assert.InDelta(t, 900, g.Data.SisaDetik, 5, "kunci 15 menit sejak kegagalan ke-5")
	assert.Equal(t, strconv.Itoa(g.Data.SisaDetik), w.Header().Get("Retry-After"))
	assert.WithinDuration(t, time.Now().Add(15*time.Minute), g.Data.TerkunciSampai, 10*time.Second)

	t.Run("percobaan selama terkunci tidak memperpanjang kunci", func(t *testing.T) {
		sebelum := s.kunciDi(t, emailBiasa)
		require.NotNil(t, sebelum)
		require.NotNil(t, sebelum.TerkunciSampai)

		for i := 0; i < 3; i++ {
			assert.Equal(t, http.StatusLocked, s.coba(t, emailBiasa, sandiSalahUji).Code)
			assert.Equal(t, http.StatusLocked, s.coba(t, emailBiasa, sandiBenarUji).Code)
		}

		sesudah := s.kunciDi(t, emailBiasa)
		require.NotNil(t, sesudah)
		assert.Equal(t, 5, sesudah.Gagal, "percobaan yang ditolak tidak menambah hitungan")
		assert.True(t, sebelum.TerkunciSampai.Equal(*sesudah.TerkunciSampai), "akhir kunci tidak bergeser")
		again := s.galatDari(t, s.coba(t, emailBiasa, sandiBenarUji))
		assert.True(t, again.Data.TerkunciSampai.Equal(g.Data.TerkunciSampai), "dan akhir yang dilaporkan sama")
	})

	t.Run("akun lain tidak terpengaruh", func(t *testing.T) {
		assert.Equal(t, http.StatusOK, s.coba(t, emailUji, sandiBenarUji).Code)
	})

	t.Run("kunci berakhir: kata sandi benar berhasil dan hitungan dihapus", func(t *testing.T) {
		require.NoError(t, s.db.Exec("UPDATE kunci_login SET terkunci_sampai = now() - interval '1 second'").Error)

		assert.Equal(t, http.StatusOK, s.coba(t, emailBiasa, sandiBenarUji).Code)
		assert.Nil(t, s.kunciDi(t, emailBiasa))
	})
}

func TestIntegration_Lockout_KunciBerakhirMemulaiHitunganBaru(t *testing.T) {
	s := newSuite(t, seedData, kunciConfig(t, 3, 2*time.Second))

	s.gagalkan(t, emailBiasa, 3)
	require.Equal(t, http.StatusLocked, s.coba(t, emailBiasa, sandiBenarUji).Code)

	time.Sleep(2200 * time.Millisecond)

	// Hitungan dari nol: dua kegagalan baru belum mengunci.
	s.gagalkan(t, emailBiasa, 2)
	assert.Equal(t, 2, s.kunciDi(t, emailBiasa).Gagal)
	assert.Equal(t, http.StatusOK, s.coba(t, emailBiasa, sandiBenarUji).Code)
}

func TestIntegration_Lockout_LoginBerhasilMengembalikanHitunganKeNol(t *testing.T) {
	s := setupSuite(t)

	s.gagalkan(t, emailBiasa, 4)
	require.Equal(t, http.StatusOK, s.coba(t, emailBiasa, sandiBenarUji).Code)
	assert.Nil(t, s.kunciDi(t, emailBiasa))

	// Tanpa reset, kegagalan ke-5 di sini sudah mengunci. Dengan reset, empat lagi masih aman.
	s.gagalkan(t, emailBiasa, 4)
	assert.Equal(t, http.StatusOK, s.coba(t, emailBiasa, sandiBenarUji).Code)
}

func TestIntegration_Lockout_KataSandiBenarPadaPercobaanKeLimaBerhasil(t *testing.T) {
	s := setupSuite(t)

	s.gagalkan(t, emailBiasa, 4)

	assert.Equal(t, http.StatusOK, s.coba(t, emailBiasa, sandiBenarUji).Code)
}

func TestIntegration_Lockout_EmailTakTerdaftarMengikutiUrutanJawabanYangSama(t *testing.T) {
	s := setupSuite(t)
	const hantu = "tidak.terdaftar@gardainflasi.id"

	type jawaban struct {
		status  int
		code    string
		message string
		sisa    int
		retry   int
	}
	rangkaian := func(email string) []jawaban {
		var hasil []jawaban
		for i := 0; i < 8; i++ {
			sandi := sandiSalahUji
			if i == 6 { // kata sandi benar (bagi yang terdaftar) di tengah kunci tetap ditolak sama
				sandi = sandiBenarUji
			}
			w := s.coba(t, email, sandi)
			g := s.galatDari(t, w)
			j := jawaban{status: w.Code, code: g.Code, message: g.Message}
			if g.Data != nil {
				j.sisa = g.Data.SisaDetik
			}
			j.retry, _ = strconv.Atoi(w.Header().Get("Retry-After"))
			hasil = append(hasil, j)
		}
		return hasil
	}

	terdaftar := rangkaian(emailBiasa)
	takTerdaftar := rangkaian(hantu)

	require.Len(t, terdaftar, len(takTerdaftar))
	for i := range terdaftar {
		assert.Equal(t, terdaftar[i].status, takTerdaftar[i].status, "status percobaan %d", i+1)
		assert.Equal(t, terdaftar[i].code, takTerdaftar[i].code, "kode percobaan %d", i+1)
		assert.Equal(t, terdaftar[i].message, takTerdaftar[i].message, "pesan percobaan %d", i+1)
		assert.InDelta(t, terdaftar[i].sisa, takTerdaftar[i].sisa, 5, "sisa detik percobaan %d", i+1)
		assert.InDelta(t, terdaftar[i].retry, takTerdaftar[i].retry, 5, "Retry-After percobaan %d", i+1)
	}
	for i := 0; i < 5; i++ {
		assert.Equal(t, http.StatusUnauthorized, takTerdaftar[i].status, "lima percobaan pertama 401")
	}
	for i := 5; i < 8; i++ {
		assert.Equal(t, http.StatusLocked, takTerdaftar[i].status, "sesudahnya 423")
	}

	t.Run("email tak terdaftar hanya tersimpan sebagai hash", func(t *testing.T) {
		require.NotNil(t, s.kunciDi(t, hantu))
		assert.EqualValues(t, 0, s.hitung(t, "SELECT count(*) FROM kunci_login WHERE email_hash LIKE '%@%'"))
		assert.EqualValues(t, 0, s.hitung(t, "SELECT count(*) FROM kunci_login WHERE length(email_hash) <> 64"))
	})
}

func TestIntegration_Lockout_AkunNonaktif(t *testing.T) {
	s := setupSuite(t)
	const nonaktif = "inactive@gardainflasi.id"

	t.Run("kata sandi salah dijawab 401 yang sama dengan akun aktif", func(t *testing.T) {
		s.gagalkan(t, nonaktif, 1)
	})
	t.Run("kata sandi benar menyingkap nonaktif tanpa menambah hitungan", func(t *testing.T) {
		for i := 0; i < 8; i++ {
			w := s.coba(t, nonaktif, sandiBenarUji)
			require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
			require.Equal(t, "ACCOUNT_INACTIVE", s.galatDari(t, w).Code)
		}
		assert.Equal(t, 1, s.kunciDi(t, nonaktif).Gagal)
	})
	t.Run("tebakan salah tetap mengunci akun nonaktif seperti akun lain", func(t *testing.T) {
		s.gagalkan(t, nonaktif, 4)
		w := s.coba(t, nonaktif, sandiBenarUji)
		assert.Equal(t, http.StatusLocked, w.Code, "terkunci, bukan 403: nonaktif tidak bocor lewat kunci")
	})
}

func TestIntegration_Lockout_KegagalanSerentakTidakMelewatiBatas(t *testing.T) {
	s := setupSuite(t)
	const serentak = 24

	kode := make(chan int, serentak)
	var wg sync.WaitGroup
	for i := 0; i < serentak; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body, _ := json.Marshal(map[string]any{"email": emailBiasa, "password": sandiSalahUji})
			req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			s.router.ServeHTTP(w, req)
			kode <- w.Code
		}()
	}
	wg.Wait()
	close(kode)

	hitungan := map[int]int{}
	for c := range kode {
		hitungan[c]++
	}
	assert.Equal(t, 5, hitungan[http.StatusUnauthorized], "tepat lima kata sandi yang sempat diperiksa: %v", hitungan)
	assert.Equal(t, serentak-5, hitungan[http.StatusLocked], "sisanya ditolak tanpa diperiksa: %v", hitungan)
	assert.Equal(t, 5, s.kunciDi(t, emailBiasa).Gagal)
	assert.EqualValues(t, 5, s.hitung(t, "SELECT count(*) FROM login_attempts WHERE failure_reason = 'wrong_password' AND user_id = ?", idBiasa))
}

func TestIntegration_Lockout_MFA(t *testing.T) {
	t.Run("kata sandi benar tidak menyentuh hitungan, kode salah menghitung, kode benar menghapus", func(t *testing.T) {
		s := setupSuite(t)
		s.gagalkan(t, emailMFA, 2)

		challenge := s.mulaiLoginMFA(t, emailMFA)
		assert.Equal(t, 2, s.kunciDi(t, emailMFA).Gagal, "kata sandi benar yang menunggu OTP tidak mengubah hitungan")

		w := s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/validate", map[string]any{"mfa_token": challenge, "code": "000000"}, nil)
		require.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Equal(t, "MFA_CODE_INVALID", s.galatDari(t, w).Code)
		assert.Equal(t, 3, s.kunciDi(t, emailMFA).Gagal)

		w = s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/validate",
			map[string]any{"mfa_token": challenge, "code": kodeOTP(t, mfaSecretUji, time.Now())}, nil)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Nil(t, s.kunciDi(t, emailMFA), "login lengkap menghapus hitungan")
	})

	t.Run("kata sandi salah dan kode salah memakai satu kunci", func(t *testing.T) {
		s := setupSuite(t)
		challenge := s.mulaiLoginMFA(t, emailMFA)
		s.gagalkan(t, emailMFA, 3)
		for i := 0; i < 2; i++ {
			w := s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/validate", map[string]any{"mfa_token": challenge, "code": "000000"}, nil)
			require.Equal(t, http.StatusUnauthorized, w.Code)
		}

		// Terkunci untuk langkah kata sandi maupun langkah OTP.
		w := s.coba(t, emailMFA, sandiBenarUji)
		assert.Equal(t, http.StatusLocked, w.Code)
		assert.Equal(t, "ACCOUNT_LOCKED", s.galatDari(t, w).Code)
		w = s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/validate",
			map[string]any{"mfa_token": challenge, "code": kodeOTP(t, mfaSecretUji, time.Now())}, nil)
		assert.Equal(t, http.StatusLocked, w.Code)
		g := s.galatDari(t, w)
		assert.Equal(t, "ACCOUNT_LOCKED", g.Code)
		require.NotNil(t, g.Data)
		assert.InDelta(t, 900, g.Data.SisaDetik, 5)
	})

	t.Run("token tantangan tidak valid: MFA_CHALLENGE_EXPIRED dan tidak dihitung", func(t *testing.T) {
		s := setupSuite(t)
		for i := 0; i < 8; i++ {
			w := s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/validate", map[string]any{"mfa_token": "palsu", "code": "123456"}, nil)
			require.Equal(t, http.StatusUnauthorized, w.Code)
			require.Equal(t, "MFA_CHALLENGE_EXPIRED", s.galatDari(t, w).Code)
		}
		assert.Nil(t, s.kunciDi(t, emailMFA))
	})

	t.Run("kode setup yang salah memakai kunci yang sama", func(t *testing.T) {
		s := setupSuite(t)
		access, _ := s.loginAs(t, emailBiasa, sandiBenarUji)
		w := s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/setup", nil, s.authHeader(access))
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())

		for i := 0; i < 5; i++ {
			w = s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/verify", map[string]any{"code": "000000"}, s.authHeader(access))
			require.Equal(t, http.StatusBadRequest, w.Code)
			require.Equal(t, "MFA_CODE_INVALID", s.galatDari(t, w).Code)
		}
		w = s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/verify", map[string]any{"code": "000000"}, s.authHeader(access))
		assert.Equal(t, http.StatusLocked, w.Code)
		assert.Equal(t, http.StatusLocked, s.coba(t, emailBiasa, sandiBenarUji).Code, "login ikut terkunci")
	})
}

func TestIntegration_Lockout_GantiKataSandi(t *testing.T) {
	s := setupSuite(t)
	access, _ := s.loginAs(t, emailBiasa, sandiBenarUji)

	for i := 0; i < 5; i++ {
		w := s.serveRequest(t, http.MethodPost, "/v1/auth/change-password",
			map[string]any{"old_password": sandiSalahUji, "new_password": "BaruSekali#77"}, s.authHeader(access))
		require.Equal(t, http.StatusUnauthorized, w.Code, "percobaan %d: %s", i+1, w.Body.String())
	}

	w := s.serveRequest(t, http.MethodPost, "/v1/auth/change-password",
		map[string]any{"old_password": sandiBenarUji, "new_password": "BaruSekali#77"}, s.authHeader(access))
	require.Equal(t, http.StatusLocked, w.Code, w.Body.String())
	g := s.galatDari(t, w)
	assert.Equal(t, "ACCOUNT_LOCKED", g.Code)
	assert.NotEmpty(t, w.Header().Get("Retry-After"))
	assert.Equal(t, http.StatusLocked, s.coba(t, emailBiasa, sandiBenarUji).Code, "login memakai kunci yang sama")
}

func TestIntegration_Lockout_BukaKunciOlehAdmin(t *testing.T) {
	s := setupSuite(t)
	admin, _ := s.loginAs(t, emailUji, sandiBenarUji)
	biasa, _ := s.loginAs(t, emailBiasa, sandiBenarUji)
	s.gagalkan(t, emailBiasa, 5)
	require.Equal(t, http.StatusLocked, s.coba(t, emailBiasa, sandiBenarUji).Code)

	t.Run("daftar dan detail menampilkan terkunci_sampai", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodGet, "/v1/users/"+idBiasa, nil, s.authHeader(admin))
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var detail struct {
			Data struct {
				TerkunciSampai *time.Time `json:"terkunci_sampai"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &detail))
		require.NotNil(t, detail.Data.TerkunciSampai, w.Body.String())
		assert.WithinDuration(t, time.Now().Add(15*time.Minute), *detail.Data.TerkunciSampai, 10*time.Second)

		w = s.serveRequest(t, http.MethodGet, "/v1/users?limit=100", nil, s.authHeader(admin))
		require.Equal(t, http.StatusOK, w.Code)
		var daftar struct {
			Data struct {
				Items []struct {
					ID             string     `json:"id"`
					TerkunciSampai *time.Time `json:"terkunci_sampai"`
				} `json:"items"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &daftar))
		terkunci := map[string]bool{}
		for _, item := range daftar.Data.Items {
			terkunci[item.ID] = item.TerkunciSampai != nil
		}
		assert.True(t, terkunci[idBiasa], "pengguna yang terkunci ditandai")
		assert.False(t, terkunci[idAdminUji], "yang lain tidak")
		assert.Contains(t, terkunci, "test-user-mfa")
	})

	t.Run("hanya pengguna.kelola", func(t *testing.T) {
		assert.Equal(t, http.StatusForbidden, s.bukaKunci(t, biasa, idBiasa).Code)
		assert.Equal(t, http.StatusUnauthorized, s.bukaKunci(t, "", idBiasa).Code)
		assert.NotNil(t, s.kunciDi(t, emailBiasa), "ditolak = tidak membuka apa pun")
	})

	t.Run("pengguna tidak ada dijawab 404 dan tidak diaudit", func(t *testing.T) {
		w := s.bukaKunci(t, admin, "tidak-ada")
		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.EqualValues(t, 0, s.hitung(t, "SELECT count(*) FROM audit_log WHERE aksi = 'UNLOCK_USER' AND entitas_id = 'tidak-ada'"))
	})

	t.Run("admin membuka kunci: dijawab, diaudit, dan pengguna bisa masuk", func(t *testing.T) {
		w := s.bukaKunci(t, admin, idBiasa)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var hasil struct {
			Data struct {
				ID                 string `json:"id"`
				TerkunciSebelumnya bool   `json:"terkunci_sebelumnya"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &hasil))
		assert.Equal(t, idBiasa, hasil.Data.ID)
		assert.True(t, hasil.Data.TerkunciSebelumnya)

		assert.Nil(t, s.kunciDi(t, emailBiasa))
		assert.EqualValues(t, 1, s.hitung(t,
			"SELECT count(*) FROM audit_log WHERE aksi = 'UNLOCK_USER' AND aktor_id = ? AND entitas = 'pengguna' AND entitas_id = ? AND hasil = 'BERHASIL' AND metadata->>'terkunci_sebelumnya' = 'true'",
			idAdminUji, idBiasa))
		assert.Equal(t, http.StatusOK, s.coba(t, emailBiasa, sandiBenarUji).Code)

		detailW := s.serveRequest(t, http.MethodGet, "/v1/users/"+idBiasa, nil, s.authHeader(admin))
		assert.NotContains(t, detailW.Body.String(), "terkunci_sampai", "tidak lagi terkunci")
	})

	t.Run("akun yang tidak terkunci: idempoten, terkunci_sebelumnya false, tetap diaudit", func(t *testing.T) {
		w := s.bukaKunci(t, admin, idBiasa)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), `"terkunci_sebelumnya":false`)
		assert.EqualValues(t, 1, s.hitung(t,
			"SELECT count(*) FROM audit_log WHERE aksi = 'UNLOCK_USER' AND entitas_id = ? AND metadata->>'terkunci_sebelumnya' = 'false'", idBiasa))
	})

	t.Run("membuka kunci akun MFA yang terkunci kode salah", func(t *testing.T) {
		challenge := s.mulaiLoginMFA(t, emailMFA)
		for i := 0; i < 5; i++ {
			s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/validate", map[string]any{"mfa_token": challenge, "code": "000000"}, nil)
		}
		require.Equal(t, http.StatusLocked, s.coba(t, emailMFA, sandiBenarUji).Code)

		require.Equal(t, http.StatusOK, s.bukaKunci(t, admin, "test-user-mfa").Code)

		w := s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/validate",
			map[string]any{"mfa_token": challenge, "code": kodeOTP(t, mfaSecretUji, time.Now())}, nil)
		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	})
}

func TestIntegration_Lockout_KesalahanTerlindungiSamaDiSemuaJalur(t *testing.T) {
	// Peta status/kode yang stabil untuk klien (BFF memetakannya ke pesan).
	s := setupSuite(t)

	t.Run("kredensial salah", func(t *testing.T) {
		w := s.coba(t, emailBiasa, sandiSalahUji)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Equal(t, "INVALID_CREDENTIALS", s.galatDari(t, w).Code)
	})
	t.Run("validasi masukan tetap 400 tanpa menghitung", func(t *testing.T) {
		for i := 0; i < 8; i++ {
			w := s.coba(t, emailBiasa, "pendek")
			require.Equal(t, http.StatusBadRequest, w.Code)
		}
		assert.Equal(t, 1, s.kunciDi(t, emailBiasa).Gagal, "hanya satu yang tadi (kredensial salah)")
	})
	t.Run("kode MFA setup salah", func(t *testing.T) {
		access, _ := s.loginAs(t, emailUji, sandiBenarUji)
		require.Equal(t, http.StatusOK, s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/setup", nil, s.authHeader(access)).Code)
		w := s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/verify", map[string]any{"code": "000000"}, s.authHeader(access))
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, "MFA_CODE_INVALID", s.galatDari(t, w).Code)
	})
}

func TestIntegration_Lockout_PangkasKunciUsang(t *testing.T) {
	s := setupSuite(t)
	s.gagalkan(t, emailBiasa, 2)
	s.gagalkan(t, "hantu.lama@gardainflasi.id", 1)
	require.NoError(t, s.db.Exec("UPDATE kunci_login SET diperbarui_pada = now() - interval '2 hours' WHERE email_hash = ?",
		usecase.HashEmailKunci("hantu.lama@gardainflasi.id")).Error)

	repo := newRepo(s)
	n, err := repo.HapusKunciUsang(t.Context(), time.Now().Add(-time.Hour))

	require.NoError(t, err)
	assert.EqualValues(t, 1, n)
	assert.Nil(t, s.kunciDi(t, "hantu.lama@gardainflasi.id"))
	assert.NotNil(t, s.kunciDi(t, emailBiasa), "baris yang baru diperbarui tetap")
}
