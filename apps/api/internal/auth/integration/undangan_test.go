package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/garda-inflasi/api/pkg/email/emailtest"
)

// Tes integrasi undang pengguna lewat email (02-D): basis data Postgres sungguhan hasil migrasi, router asli, dan
// server SMTP palsu yang menangkap email.

const (
	baseWebUji    = "https://garda.test"
	sandiAktivasi = "Rahasia#2026Kuat"
)

// setupSuiteSMTP menyiapkan suite yang mengirim email ke server SMTP palsu (rute membaca SMTP_HOST, SMTP_PORT, dan
// APP_PUBLIC_URL dari lingkungan saat didaftarkan).
func setupSuiteSMTP(t *testing.T) (*Suite, *emailtest.Server) {
	t.Helper()
	smtp := emailtest.Start(t)
	t.Setenv("SMTP_HOST", smtp.Host())
	t.Setenv("SMTP_PORT", smtp.Port())
	t.Setenv("APP_PUBLIC_URL", baseWebUji)
	return setupSuite(t), smtp
}

type isianUndang map[string]any

func isianSah(email string) isianUndang {
	return isianUndang{"nama": "Budi Santoso", "email": email, "peran": "OPS_MAHASISWA", "lembaga": "FMIPA USK", "kabupaten_kode": "1171"}
}

func (s *Suite) undang(t *testing.T, access string, isian isianUndang) *httptest.ResponseRecorder {
	t.Helper()
	return s.panggil(t, http.MethodPost, "/v1/users/undang", isian, access)
}

func (s *Suite) aktivasi(t *testing.T, token, password string) *httptest.ResponseRecorder {
	t.Helper()
	return s.serveRequest(t, http.MethodPost, "/v1/auth/aktivasi", map[string]any{"token": token, "password": password}, nil)
}

// dataUndangan membaca data.user.id dari jawaban undangan.
func (s *Suite) dataUndangan(t *testing.T, w *httptest.ResponseRecorder) (id string, data map[string]any) {
	t.Helper()
	data = s.parseResponse(t, w)["data"].(map[string]any)
	return data["user"].(map[string]any)["id"].(string), data
}

func (s *Suite) statusAkun(t *testing.T, userID string) string {
	t.Helper()
	var st string
	require.NoError(t, s.db.Raw("SELECT status::text FROM pengguna WHERE id = ?", userID).Scan(&st).Error)
	return st
}

func (s *Suite) pesan(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	m, _ := s.parseResponse(t, w)["message"].(string)
	return m
}

func (s *Suite) jumlahAudit(t *testing.T, aksi, entitasID string) int64 {
	t.Helper()
	return s.hitung(t, "SELECT count(*) FROM audit_log WHERE aksi = ? AND entitas_id = ?", aksi, entitasID)
}

// ── Alur penuh ────────────────────────────────────────────────────────

func TestUndangan_AlurPenuh_UndangAktivasiMasuk(t *testing.T) {
	s, smtp := setupSuiteSMTP(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)

	w := s.undang(t, admin.access, isianSah("Budi.Santoso@GardaInflasi.id"))
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	id, data := s.dataUndangan(t, w)

	t.Run("jawaban", func(t *testing.T) {
		user := data["user"].(map[string]any)
		assert.Equal(t, "MENUNGGU", user["status"])
		assert.Equal(t, "budi.santoso@gardainflasi.id", user["email"])
		assert.Equal(t, "OPERASIONAL", user["lapis"])
		assert.Equal(t, "1171", user["kabupaten_kode"])
		assert.Equal(t, true, data["email_terkirim"])
		berlaku, err := time.Parse(time.RFC3339, data["berlaku_sampai"].(string))
		require.NoError(t, err)
		assert.WithinDuration(t, time.Now().Add(72*time.Hour), berlaku, time.Minute)
		assert.NotContains(t, w.Body.String(), "password_hash", "tidak ada rahasia di jawaban")
	})

	pesanEmail := smtp.Tunggu(t, "budi.santoso@gardainflasi.id", 1)
	token := pesanEmail[0].Token()
	require.NotEmpty(t, token)

	t.Run("email berisi tautan ke halaman aktivasi", func(t *testing.T) {
		assert.Contains(t, pesanEmail[0].Data, baseWebUji+"/aktivasi?token="+token)
		assert.Contains(t, pesanEmail[0].Data, "Budi Santoso")
	})
	t.Run("basis data: tanpa kata sandi, hanya hash token", func(t *testing.T) {
		var row struct {
			Status       string
			PasswordHash *string
			Wajib        bool
		}
		require.NoError(t, s.db.Raw("SELECT status::text AS status, password_hash, wajib_ganti_password AS wajib FROM pengguna WHERE id = ?", id).Scan(&row).Error)
		assert.Equal(t, "MENUNGGU", row.Status)
		assert.Nil(t, row.PasswordHash)
		assert.False(t, row.Wajib)

		var u struct {
			TokenHash    string
			Email        string
			DiundangOleh string
			UsedAt       *time.Time
		}
		require.NoError(t, s.db.Raw("SELECT token_hash, email, diundang_oleh, used_at FROM undangan_pengguna WHERE user_id = ?", id).Scan(&u).Error)
		assert.Equal(t, hashToken(token), u.TokenHash)
		assert.Regexp(t, `^[0-9a-f]{64}$`, u.TokenHash)
		assert.Equal(t, "budi.santoso@gardainflasi.id", u.Email)
		assert.Equal(t, "test-user-001", u.DiundangOleh)
		assert.Nil(t, u.UsedAt)
		assert.Zero(t, s.hitung(t, "SELECT count(*) FROM undangan_pengguna WHERE token_hash = ? OR email = ?", token, token), "token mentah tidak tersimpan")
	})
	t.Run("belum bisa masuk; jawabannya sama dengan email tak terdaftar", func(t *testing.T) {
		menunggu := s.coba(t, "budi.santoso@gardainflasi.id", sandiAktivasi)
		takAda := s.coba(t, "tidak.ada@gardainflasi.id", sandiAktivasi)
		assert.Equal(t, http.StatusUnauthorized, menunggu.Code)
		assert.Equal(t, takAda.Code, menunggu.Code)
		assert.Equal(t, takAda.Body.String(), menunggu.Body.String(), "tidak ada celah enumerasi akun yang menunggu")
	})
	t.Run("aktivasi menetapkan kata sandi dan mengaktifkan akun", func(t *testing.T) {
		w := s.aktivasi(t, token, sandiAktivasi)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, "AKTIF", s.statusAkun(t, id))
		assert.NotZero(t, s.hitung(t, "SELECT count(*) FROM pengguna WHERE id = ? AND password_hash IS NOT NULL AND wajib_ganti_password = false AND password_diubah_pada IS NOT NULL", id))
		assert.NotZero(t, s.hitung(t, "SELECT count(*) FROM undangan_pengguna WHERE user_id = ? AND used_at IS NOT NULL", id))
	})
	t.Run("tautan yang sama tidak bisa dipakai lagi", func(t *testing.T) {
		w := s.aktivasi(t, token, "LainLagi#2026Kuat")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, s.pesan(t, w), "sudah dipakai")
		s.loginAs(t, "budi.santoso@gardainflasi.id", sandiAktivasi) // kata sandi pertama tetap berlaku
	})
	t.Run("masuk dengan kata sandi baru tanpa kewajiban ganti", func(t *testing.T) {
		access, _ := s.loginAs(t, "budi.santoso@gardainflasi.id", sandiAktivasi)
		me := s.parseResponse(t, s.serveRequest(t, http.MethodGet, "/v1/auth/me", nil, s.authHeader(access)))["data"].(map[string]any)
		assert.Equal(t, false, me["wajib_ganti_password"])
		assert.Equal(t, "AKTIF", me["status"])
	})
	t.Run("audit", func(t *testing.T) {
		assert.EqualValues(t, 1, s.jumlahAudit(t, "UNDANG_PENGGUNA", id))
		assert.EqualValues(t, 1, s.jumlahAudit(t, "AKTIVASI_AKUN", id))
		var aktor string
		require.NoError(t, s.db.Raw("SELECT aktor_id FROM audit_log WHERE aksi = 'UNDANG_PENGGUNA' AND entitas_id = ?", id).Scan(&aktor).Error)
		assert.Equal(t, "test-user-001", aktor)
		require.NoError(t, s.db.Raw("SELECT aktor_id FROM audit_log WHERE aksi = 'AKTIVASI_AKUN' AND entitas_id = ?", id).Scan(&aktor).Error)
		assert.Equal(t, id, aktor, "aktor aktivasi adalah pemilik akun")
		assert.Zero(t, s.hitung(t, "SELECT count(*) FROM audit_log WHERE metadata::text LIKE ? OR sesudah::text LIKE ? OR sebelum::text LIKE ?", "%"+token+"%", "%"+token+"%", "%"+token+"%"), "token tidak masuk audit")
		assert.Zero(t, s.hitung(t, "SELECT count(*) FROM audit_log WHERE sesudah::text LIKE ? OR metadata::text LIKE ?", "%"+sandiAktivasi+"%", "%"+sandiAktivasi+"%"))
	})
}

// ── Izin ──────────────────────────────────────────────────────────────

func TestUndangan_HanyaPenggunaKelola(t *testing.T) {
	s, smtp := setupSuiteSMTP(t)
	biasa := s.mulaiSesi(t, emailBiasa, uaMac)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	w := s.undang(t, admin.access, isianSah("pending@gardainflasi.id"))
	require.Equal(t, http.StatusCreated, w.Code)
	idMenunggu, _ := s.dataUndangan(t, w)
	sebelum := len(smtp.Messages())

	permintaan := map[string]struct {
		metode, path string
		badan        any
	}{
		"undang":       {http.MethodPost, "/v1/users/undang", isianSah("lain@gardainflasi.id")},
		"undang ulang": {http.MethodPost, "/v1/users/" + idMenunggu + "/undang-ulang", nil},
		"impor":        {http.MethodPost, "/v1/users/impor", map[string]any{"csv": headerCSV + "\nA,a@b.id,OPS_MAHASISWA,X,\n", "dry_run": false}},
		"ekspor":       {http.MethodGet, "/v1/users/ekspor", nil},
	}
	for nama, p := range permintaan {
		t.Run(nama+": peran tanpa pengguna.kelola ditolak 403", func(t *testing.T) {
			w := s.panggil(t, p.metode, p.path, p.badan, biasa.access)
			assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
		})
		t.Run(nama+": tanpa token 401", func(t *testing.T) {
			w := s.panggil(t, p.metode, p.path, p.badan, "")
			assert.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
		})
	}
	assert.Len(t, smtp.Messages(), sebelum, "permintaan yang ditolak tidak mengirim email")
	assert.Zero(t, s.hitung(t, "SELECT count(*) FROM pengguna WHERE email IN ('lain@gardainflasi.id', 'a@b.id')"))
	assert.Zero(t, s.hitung(t, "SELECT count(*) FROM audit_log WHERE aksi = 'EKSPOR_DATA'"), "ekspor yang ditolak tidak tercatat sebagai ekspor")
}

func TestUndangan_AktivasiPublikTanpaToken(t *testing.T) {
	s, _ := setupSuiteSMTP(t)

	w := s.aktivasi(t, "token-tak-dikenal", sandiAktivasi)

	assert.Equal(t, http.StatusUnauthorized, w.Code, "publik: tidak 401 karena sesi, tetapi karena tautan")
	assert.Contains(t, s.pesan(t, w), "tidak valid atau sudah dipakai")
}

// ── Validasi ──────────────────────────────────────────────────────────

func TestUndangan_ValidasiDanDuplikat(t *testing.T) {
	s, smtp := setupSuiteSMTP(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)

	ubah := func(k string, v any) isianUndang {
		i := isianSah("valid@gardainflasi.id")
		i[k] = v
		return i
	}
	kasus := map[string]struct {
		isian  isianUndang
		status int
		pesan  string
	}{
		"nama kosong":            {ubah("nama", " "), http.StatusBadRequest, "Nama wajib diisi"},
		"nama diawali rumus":     {ubah("nama", "=1+1"), http.StatusBadRequest, "rumus"},
		"email tidak valid":      {ubah("email", "bukan-email"), http.StatusBadRequest, "Email tidak valid"},
		"peran tak dikenal":      {ubah("peran", "DEWA"), http.StatusBadRequest, "Peran tidak dikenal"},
		"lembaga kosong":         {ubah("lembaga", ""), http.StatusBadRequest, "Validasi gagal"},
		"kabupaten tak dikenal":  {ubah("kabupaten_kode", "9999"), http.StatusBadRequest, "Kode kabupaten tidak dikenal"},
		"kabupaten salah format": {ubah("kabupaten_kode", "abc"), http.StatusBadRequest, "4 digit"},
		"email sudah terdaftar":  {ubah("email", "UJI.BIASA@gardainflasi.id"), http.StatusConflict, "sudah terdaftar"},
		"email admin terdaftar":  {ubah("email", emailAdmin), http.StatusConflict, "sudah terdaftar"},
	}
	for nama, k := range kasus {
		t.Run(nama, func(t *testing.T) {
			w := s.undang(t, admin.access, k.isian)
			assert.Equal(t, k.status, w.Code, w.Body.String())
			assert.Contains(t, s.pesan(t, w), k.pesan)
		})
	}
	assert.Zero(t, s.hitung(t, "SELECT count(*) FROM undangan_pengguna"))
	assert.Empty(t, smtp.Messages())

	t.Run("badan bukan JSON", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodPost, "/v1/users/undang", nil, s.authHeader(admin.access))
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
	t.Run("email kembar saat undangan kedua: akun pertama utuh", func(t *testing.T) {
		require.Equal(t, http.StatusCreated, s.undang(t, admin.access, isianSah("kembar@gardainflasi.id")).Code)
		w := s.undang(t, admin.access, isianSah("KEMBAR@gardainflasi.id"))
		assert.Equal(t, http.StatusConflict, w.Code)
		assert.EqualValues(t, 1, s.hitung(t, "SELECT count(*) FROM pengguna WHERE lower(email) = 'kembar@gardainflasi.id'"))
		assert.EqualValues(t, 1, s.hitung(t, "SELECT count(*) FROM undangan_pengguna u JOIN pengguna p ON p.id = u.user_id WHERE p.email = 'kembar@gardainflasi.id'"))
	})
}

func TestUndangan_EmailGagalAkunTetapDibuatDanBisaDikirimUlang(t *testing.T) {
	s, smtp := setupSuiteSMTP(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	smtp.TolakPenerima(true)

	w := s.undang(t, admin.access, isianSah("gagal@gardainflasi.id"))

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	id, data := s.dataUndangan(t, w)
	assert.Equal(t, false, data["email_terkirim"], "admin diberi tahu bahwa email belum terkirim")
	assert.Equal(t, "MENUNGGU", s.statusAkun(t, id))

	smtp.TolakPenerima(false)
	w = s.panggil(t, http.MethodPost, "/v1/users/"+id+"/undang-ulang", nil, admin.access)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	_, data = s.dataUndangan(t, w)
	assert.Equal(t, true, data["email_terkirim"])
	token := smtp.Tunggu(t, "gagal@gardainflasi.id", 1)[0].Token()
	assert.Equal(t, http.StatusOK, s.aktivasi(t, token, sandiAktivasi).Code)
}

// ── Kirim ulang ───────────────────────────────────────────────────────

func TestUndangan_KirimUlangMencabutTautanLama(t *testing.T) {
	s, smtp := setupSuiteSMTP(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	id, _ := s.dataUndangan(t, s.undang(t, admin.access, isianSah("ulang@gardainflasi.id")))
	lama := smtp.Tunggu(t, "ulang@gardainflasi.id", 1)[0].Token()

	w := s.panggil(t, http.MethodPost, "/v1/users/"+id+"/undang-ulang", nil, admin.access)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	baru := smtp.Tunggu(t, "ulang@gardainflasi.id", 2)[1].Token()
	require.NotEqual(t, lama, baru)

	t.Run("hanya satu undangan hidup", func(t *testing.T) {
		assert.EqualValues(t, 1, s.hitung(t, "SELECT count(*) FROM undangan_pengguna WHERE user_id = ? AND used_at IS NULL AND dicabut_pada IS NULL", id))
		assert.EqualValues(t, 2, s.hitung(t, "SELECT count(*) FROM undangan_pengguna WHERE user_id = ?", id))
		assert.EqualValues(t, 1, s.hitung(t, "SELECT count(*) FROM undangan_pengguna WHERE user_id = ? AND dicabut_pada IS NOT NULL", id))
	})
	t.Run("tautan lama ditolak dan tidak mengaktifkan", func(t *testing.T) {
		w := s.aktivasi(t, lama, sandiAktivasi)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Equal(t, "MENUNGGU", s.statusAkun(t, id))
	})
	t.Run("tautan baru berlaku", func(t *testing.T) {
		assert.Equal(t, http.StatusOK, s.aktivasi(t, baru, sandiAktivasi).Code)
		assert.Equal(t, "AKTIF", s.statusAkun(t, id))
	})
	t.Run("audit", func(t *testing.T) {
		assert.EqualValues(t, 1, s.jumlahAudit(t, "UNDANG_ULANG_PENGGUNA", id))
	})
	t.Run("akun yang sudah aktif tidak bisa diundang ulang", func(t *testing.T) {
		w := s.panggil(t, http.MethodPost, "/v1/users/"+id+"/undang-ulang", nil, admin.access)
		assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	})
}

func TestUndangan_KirimUlangTidakAdaDan409UntukBukanMenunggu(t *testing.T) {
	s, smtp := setupSuiteSMTP(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)

	assert.Equal(t, http.StatusNotFound, s.panggil(t, http.MethodPost, "/v1/users/tidak-ada/undang-ulang", nil, admin.access).Code)
	for _, id := range []string{userBiasa, "test-user-002"} { // aktif dan nonaktif
		w := s.panggil(t, http.MethodPost, "/v1/users/"+id+"/undang-ulang", nil, admin.access)
		assert.Equal(t, http.StatusConflict, w.Code, id)
	}
	assert.Empty(t, smtp.Messages())
}

func TestUndangan_Kedaluwarsa(t *testing.T) {
	s, smtp := setupSuiteSMTP(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	id, _ := s.dataUndangan(t, s.undang(t, admin.access, isianSah("lewat@gardainflasi.id")))
	token := smtp.Tunggu(t, "lewat@gardainflasi.id", 1)[0].Token()

	t.Run("sebelum batas masih berlaku (kata sandi lemah ditolak tanpa menghabiskan tautan)", func(t *testing.T) {
		// Batas dihitung dari jam proses API (yang membandingkannya), bukan now() basis data: jam kontainer
		// Docker bisa bergeser dari jam host setelah laptop tidur.
		require.NoError(t, s.db.Exec("UPDATE undangan_pengguna SET expires_at = ? WHERE user_id = ?", time.Now().Add(time.Minute), id).Error)
		w := s.aktivasi(t, token, "semuahurufkecil")
		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Contains(t, s.pesan(t, w), "huruf kapital", "alasan kebijakan sampai ke pengguna")
		assert.Equal(t, "MENUNGGU", s.statusAkun(t, id))
		assert.Zero(t, s.hitung(t, "SELECT count(*) FROM undangan_pengguna WHERE user_id = ? AND used_at IS NOT NULL", id))
	})
	t.Run("sesudah batas ditolak dengan pesan kedaluwarsa", func(t *testing.T) {
		require.NoError(t, s.db.Exec("UPDATE undangan_pengguna SET expires_at = ? WHERE user_id = ?", time.Now().Add(-time.Second), id).Error)
		w := s.aktivasi(t, token, sandiAktivasi)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, s.pesan(t, w), "kedaluwarsa")
		assert.Equal(t, "MENUNGGU", s.statusAkun(t, id))
	})
	t.Run("kirim ulang menggantikan undangan kedaluwarsa", func(t *testing.T) {
		w := s.panggil(t, http.MethodPost, "/v1/users/"+id+"/undang-ulang", nil, admin.access)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		baru := smtp.Tunggu(t, "lewat@gardainflasi.id", 2)[1].Token()
		assert.Equal(t, http.StatusOK, s.aktivasi(t, baru, sandiAktivasi).Code)
	})
}

func TestUndangan_KataSandiMemenuhiKebijakanYangSama(t *testing.T) {
	s, smtp := setupSuiteSMTP(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	s.undang(t, admin.access, isianSah("kebijakan@gardainflasi.id"))
	token := smtp.Tunggu(t, "kebijakan@gardainflasi.id", 1)[0].Token()

	kasus := map[string]string{
		"terlalu pendek (binding)": "Ab1!",
		"tanpa kapital":            "rahasia#2026kuat",
		"tanpa angka":              "Rahasia#Kuatsekali",
		"tanpa karakter khusus":    "Rahasia2026Kuat",
		"terlalu umum":             "password",
		"lebih dari 72 byte":       "Aa1!" + strings.Repeat("x", 80),
	}
	for nama, sandi := range kasus {
		w := s.aktivasi(t, token, sandi)
		assert.Equal(t, http.StatusBadRequest, w.Code, "%s: %s", nama, w.Body.String())
	}
	assert.Equal(t, http.StatusOK, s.aktivasi(t, token, sandiAktivasi).Code, "tautan masih berlaku setelah semua penolakan")
}

func TestUndangan_AktivasiSerentakHanyaSatuYangBerhasil(t *testing.T) {
	s, smtp := setupSuiteSMTP(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	id, _ := s.dataUndangan(t, s.undang(t, admin.access, isianSah("balapan@gardainflasi.id")))
	token := smtp.Tunggu(t, "balapan@gardainflasi.id", 1)[0].Token()

	const jumlah = 8
	hasil := make([]*httptest.ResponseRecorder, jumlah)
	var wg sync.WaitGroup
	for i := range jumlah {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hasil[i] = s.aktivasi(t, token, fmt.Sprintf("Rahasia#%dKuat", 100+i))
		}()
	}
	wg.Wait()

	var berhasil int
	for _, w := range hasil {
		switch w.Code {
		case http.StatusOK:
			berhasil++
		default:
			assert.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
		}
	}
	assert.Equal(t, 1, berhasil, "tepat satu permintaan menetapkan kata sandi")
	assert.Equal(t, "AKTIF", s.statusAkun(t, id))
	assert.EqualValues(t, 1, s.jumlahAudit(t, "AKTIVASI_AKUN", id))
}

// Mengubah email akun MENUNGGU tidak boleh membiarkan tautan yang terlanjur dikirim ke alamat lama tetap hidup.
func TestUndangan_GantiEmailMenghanguskanTautanLama(t *testing.T) {
	s, smtp := setupSuiteSMTP(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	id, _ := s.dataUndangan(t, s.undang(t, admin.access, isianSah("salah.ketik@gardainflasi.id")))
	lama := smtp.Tunggu(t, "salah.ketik@gardainflasi.id", 1)[0].Token()

	w := s.panggil(t, http.MethodPatch, "/v1/users/"+id, map[string]any{"email": "benar@gardainflasi.id"}, admin.access)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	assert.Equal(t, http.StatusUnauthorized, s.aktivasi(t, lama, sandiAktivasi).Code, "pemilik alamat lama tidak boleh mengambil alih akun")
	assert.Equal(t, "MENUNGGU", s.statusAkun(t, id))
	require.Equal(t, http.StatusOK, s.panggil(t, http.MethodPost, "/v1/users/"+id+"/undang-ulang", nil, admin.access).Code)
	baru := smtp.Tunggu(t, "benar@gardainflasi.id", 1)[0].Token()
	assert.Equal(t, http.StatusOK, s.aktivasi(t, baru, sandiAktivasi).Code)
}

// ── Akun menunggu di jalur lain ───────────────────────────────────────

func TestUndangan_AkunMenungguDiJalurLain(t *testing.T) {
	s, smtp := setupSuiteSMTP(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	id, _ := s.dataUndangan(t, s.undang(t, admin.access, isianSah("pending@gardainflasi.id")))
	smtp.Tunggu(t, "pending@gardainflasi.id", 1)

	t.Run("PATCH status AKTIF atau NONAKTIF ditolak 409", func(t *testing.T) {
		for _, st := range []string{"AKTIF", "NONAKTIF"} {
			w := s.panggil(t, http.MethodPatch, "/v1/users/"+id, map[string]any{"status": st}, admin.access)
			assert.Equal(t, http.StatusConflict, w.Code, "%s: %s", st, w.Body.String())
		}
		assert.Equal(t, "MENUNGGU", s.statusAkun(t, id))
	})
	t.Run("PATCH status MENUNGGU pada akun aktif ditolak 400", func(t *testing.T) {
		w := s.panggil(t, http.MethodPatch, "/v1/users/"+userBiasa, map[string]any{"status": "MENUNGGU"}, admin.access)
		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Equal(t, "AKTIF", s.statusAkun(t, userBiasa))
	})
	t.Run("reset kata sandi oleh admin ditolak 409", func(t *testing.T) {
		w := s.panggil(t, http.MethodPost, "/v1/users/"+id+"/reset-password", map[string]any{"new_password": sandiAktivasi}, admin.access)
		assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
		assert.EqualValues(t, 1, s.hitung(t, "SELECT count(*) FROM pengguna WHERE id = ? AND password_hash IS NULL", id))
	})
	t.Run("aksi massal aktifkan dan nonaktifkan melewati akun menunggu", func(t *testing.T) {
		for _, aksi := range []string{"aktifkan", "nonaktifkan"} {
			w := s.panggil(t, http.MethodPost, "/v1/users/bulk", map[string]any{"action": aksi, "ids": []string{id}}, admin.access)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			assert.EqualValues(t, 0, s.parseResponse(t, w)["data"].(map[string]any)["berhasil"], aksi)
		}
		assert.Equal(t, "MENUNGGU", s.statusAkun(t, id))
	})
	t.Run("lupa kata sandi tidak mengirim apa pun dan tidak mengaktifkan", func(t *testing.T) {
		sebelum := len(smtp.Messages())
		w := s.serveRequest(t, http.MethodPost, "/v1/auth/forgot-password", map[string]any{"email": "pending@gardainflasi.id"}, nil)
		assert.Equal(t, http.StatusOK, w.Code)
		time.Sleep(200 * time.Millisecond) // pengiriman email reset berjalan di latar belakang
		assert.Len(t, smtp.Messages(), sebelum)
		assert.Zero(t, s.hitung(t, "SELECT count(*) FROM password_reset_tokens WHERE user_id = ?", id))
	})
	t.Run("token reset tidak bisa dipakai di jalur aktivasi dan sebaliknya", func(t *testing.T) {
		token := smtp.Untuk("pending@gardainflasi.id")[0].Token()
		w := s.reset(t, token, sandiAktivasi)
		assert.Equal(t, http.StatusUnauthorized, w.Code, "token undangan bukan token reset")
		assert.Equal(t, "MENUNGGU", s.statusAkun(t, id))
		s.simpanTokenReset(t, "token-reset-biasa", time.Now().Add(time.Hour))
		assert.Equal(t, http.StatusUnauthorized, s.aktivasi(t, "token-reset-biasa", sandiAktivasi).Code)
	})
	t.Run("hapus akun membatalkan undangan", func(t *testing.T) {
		token := smtp.Untuk("pending@gardainflasi.id")[0].Token()
		w := s.panggil(t, http.MethodDelete, "/v1/users/"+id, nil, admin.access)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Zero(t, s.hitung(t, "SELECT count(*) FROM undangan_pengguna WHERE user_id = ?", id), "undangan ikut terhapus")
		assert.Equal(t, http.StatusUnauthorized, s.aktivasi(t, token, sandiAktivasi).Code)
	})
}

func TestUndangan_AdminYangMengundangDihapusUndanganTetapAda(t *testing.T) {
	s, smtp := setupSuiteSMTP(t)
	require.NoError(t, s.db.Exec(`INSERT INTO pengguna (id, nama, email, peran, lapis, lembaga, status)
		VALUES ('admin-sementara', 'Admin Sementara', 'adm.sementara@gardainflasi.id', 'ADMIN_SISTEM', 'SISTEM', 'BPPSDMP', 'AKTIF')`).Error)
	require.NoError(t, s.db.Exec("UPDATE pengguna SET password_hash = (SELECT password_hash FROM pengguna WHERE id = 'test-user-001') WHERE id = 'admin-sementara'").Error)
	sementara := s.mulaiSesi(t, "adm.sementara@gardainflasi.id", uaMac)
	id, _ := s.dataUndangan(t, s.undang(t, sementara.access, isianSah("yatim@gardainflasi.id")))
	token := smtp.Tunggu(t, "yatim@gardainflasi.id", 1)[0].Token()

	require.NoError(t, s.db.Exec("DELETE FROM refresh_tokens WHERE user_id = 'admin-sementara'").Error)
	require.NoError(t, s.db.Exec("DELETE FROM login_attempts WHERE user_id = 'admin-sementara'").Error)
	require.NoError(t, s.db.Exec("DELETE FROM pengguna WHERE id = 'admin-sementara'").Error)

	assert.EqualValues(t, 1, s.hitung(t, "SELECT count(*) FROM undangan_pengguna WHERE user_id = ? AND diundang_oleh IS NULL", id), "FK SET NULL: undangan tidak menghalangi penghapusan admin")
	assert.Equal(t, http.StatusOK, s.aktivasi(t, token, sandiAktivasi).Code)
}

// ── Batas laju ────────────────────────────────────────────────────────

func TestUndangan_AktivasiDibatasiPerIPSatuEmberDenganLogin(t *testing.T) {
	s := newSuiteBatas(t, seedData, 3, []string{"192.0.2.0/24"})
	const ip = "198.51.100.30"

	for i := 1; i <= 2; i++ {
		w := s.kirimDariIP(t, ip, http.MethodPost, "/v1/auth/aktivasi", map[string]any{"token": "tebakan", "password": sandiAktivasi})
		require.Equal(t, http.StatusUnauthorized, w.Code, "percobaan %d", i)
	}
	w := s.kirimDariIP(t, ip, http.MethodPost, "/v1/auth/login", map[string]any{"email": emailBiasa, "password": sandiSalahUji})
	require.Equal(t, http.StatusUnauthorized, w.Code, "ember bersama: login ikut menghitung")

	tegas429(t, s.kirimDariIP(t, ip, http.MethodPost, "/v1/auth/aktivasi", map[string]any{"token": "tebakan", "password": sandiAktivasi}))
	w = s.kirimDariIP(t, "198.51.100.31", http.MethodPost, "/v1/auth/aktivasi", map[string]any{"token": "tebakan", "password": sandiAktivasi})
	assert.Equal(t, http.StatusUnauthorized, w.Code, "IP lain tidak terpengaruh")
}

// ── Bentuk token ──────────────────────────────────────────────────────

func TestUndangan_TokenAcakDanTidakBerulang(t *testing.T) {
	s, smtp := setupSuiteSMTP(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	polaToken := regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
	lihat := map[string]bool{}
	for i := range 5 {
		mail := fmt.Sprintf("acak%d@gardainflasi.id", i)
		require.Equal(t, http.StatusCreated, s.undang(t, admin.access, isianSah(mail)).Code)
		token := smtp.Tunggu(t, mail, 1)[0].Token()
		assert.Regexp(t, polaToken, token)
		assert.False(t, lihat[token])
		lihat[token] = true
	}
}
