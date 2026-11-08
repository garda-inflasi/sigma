package integration

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/auth/usecase"
	"github.com/garda-inflasi/api/pkg/crypto"
	"github.com/garda-inflasi/api/pkg/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Profil mandiri (20-A): PATCH /v1/auth/me mengubah nama dan telepon pemilik akun, tidak ada yang lain. Akun uji
// "uji.biasa" (OPS_MAHASISWA) bernama "Pengguna Biasa" dan belum punya telepon.

const (
	namaAwalBiasa = "Pengguna Biasa"
	teleponBaru   = "+6281234567890"
)

// patchMe mengirim PATCH /v1/auth/me dengan badan JSON.
func (s *Suite) patchMe(t *testing.T, access string, body any) *httptest.ResponseRecorder {
	t.Helper()
	return s.panggil(t, http.MethodPatch, "/v1/auth/me", body, access)
}

// patchMeMentah mengirim PATCH /v1/auth/me dengan badan apa adanya (mis. JSON rusak).
func (s *Suite) patchMeMentah(t *testing.T, access, mentah string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch, "/v1/auth/me", strings.NewReader(mentah))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+access)
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	return w
}

type barisProfil struct {
	Nama    string
	Telepon *string
}

func (s *Suite) profilDiDB(t *testing.T, id string) barisProfil {
	t.Helper()
	var b barisProfil
	require.NoError(t, s.db.Raw("SELECT nama, telepon FROM pengguna WHERE id = ?", id).Scan(&b).Error)
	return b
}

// barisJSON adalah seluruh baris pengguna sebagai JSON teks (bandingkan sebelum dan sesudah); kolom yang
// dikecualikan dibuang dari perbandingan.
func (s *Suite) barisJSON(t *testing.T, id string, kecuali ...string) string {
	t.Helper()
	ekspresi := "to_jsonb(p)"
	for _, k := range kecuali {
		ekspresi += " - '" + k + "'"
	}
	var teks string
	require.NoError(t, s.db.Raw("SELECT ("+ekspresi+")::text FROM pengguna p WHERE p.id = ?", id).Scan(&teks).Error)
	return teks
}

func (s *Suite) jumlahAuditProfil(t *testing.T) int64 {
	t.Helper()
	return s.count(t, "SELECT count(*) FROM audit_log WHERE aksi = 'UPDATE_PROFIL'")
}

func (s *Suite) diupdatePada(t *testing.T, id string) time.Time {
	t.Helper()
	var ts time.Time
	require.NoError(t, s.db.Raw("SELECT diupdate_pada FROM pengguna WHERE id = ?", id).Scan(&ts).Error)
	return ts
}

func TestProfil_PatchMengubahNamaDanTeleponLaluGetMemuatNilaiBaru(t *testing.T) {
	s := setupSuite(t)
	biasa := s.mulaiSesi(t, emailBiasa, uaMac)

	w := s.patchMe(t, biasa.access, map[string]any{"nama": "  Nama Baru Biasa  ", "telepon": "0812-3456-7890"})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := s.parseResponse(t, w)
	assert.Equal(t, "200", body["code"])
	assert.Equal(t, "OK", body["message"])
	data := body["data"].(map[string]any)
	assert.Equal(t, userBiasa, data["id"])
	assert.Equal(t, "Nama Baru Biasa", data["nama"], "nama dipangkas")
	assert.Equal(t, teleponBaru, data["telepon"], "telepon dinormalkan ke E.164")
	assert.Equal(t, emailBiasa, data["email"])
	assert.Equal(t, "OPS_MAHASISWA", data["peran"])
	assert.Equal(t, "OPERASIONAL", data["lapis"])
	assert.Equal(t, "BPPSDMP", data["lembaga"])
	assert.Equal(t, "AKTIF", data["status"])
	assert.Equal(t, false, data["mfa_aktif"])
	assert.Equal(t, false, data["mfa_setup_diwajibkan"])
	assert.Equal(t, false, data["mfa_wajib"])
	assert.Equal(t, false, data["wajib_ganti_password"])
	assert.NotContains(t, data, "kode_cadangan_tersisa", "hanya ada bila MFA aktif")
	assert.NotContains(t, data, "password_hash")

	t.Run("GET /v1/auth/me sesudahnya memuat nilai baru, bentuknya sama dengan jawaban PATCH", func(t *testing.T) {
		gw := s.panggil(t, http.MethodGet, "/v1/auth/me", nil, biasa.access)
		require.Equal(t, http.StatusOK, gw.Code, "token yang sama tetap berlaku: PATCH tidak mencabut sesi")
		me := s.dataObj(t, gw)
		assert.Equal(t, "Nama Baru Biasa", me["nama"])
		assert.Equal(t, teleponBaru, me["telepon"])
		assert.Equal(t, data, me)
	})

	t.Run("tersimpan di basis data", func(t *testing.T) {
		b := s.profilDiDB(t, userBiasa)
		assert.Equal(t, "Nama Baru Biasa", b.Nama)
		require.NotNil(t, b.Telepon)
		assert.Equal(t, teleponBaru, *b.Telepon)
	})

	t.Run("tercatat di audit sebagai UPDATE_PROFIL dengan sebelum dan sesudah", func(t *testing.T) {
		assert.EqualValues(t, 1, s.count(t, `SELECT count(*) FROM audit_log
			WHERE aksi = 'UPDATE_PROFIL' AND entitas = 'pengguna' AND entitas_id = ? AND aktor_id = ? AND hasil = 'BERHASIL'
			  AND sebelum->>'nama' = ? AND jsonb_typeof(sebelum->'telepon') = 'null'
			  AND sesudah->>'nama' = 'Nama Baru Biasa' AND sesudah->>'telepon' = ?
			  AND sesudah->>'email' = ? AND sesudah->>'peran' = 'OPS_MAHASISWA'`,
			userBiasa, userBiasa, namaAwalBiasa, teleponBaru, emailBiasa))
		assert.EqualValues(t, 1, s.jumlahAuditProfil(t))
	})

	t.Run("refresh menerbitkan access token dengan nama baru (dipakai BFF web)", func(t *testing.T) {
		w := s.refreshRaw(t, biasa.refresh)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var data struct {
			AccessToken string `json:"access_token"`
		}
		s.decodeData(t, w.Body.Bytes(), &data)
		assert.Equal(t, "Nama Baru Biasa", mustClaims(t, s.tokenMgr, data.AccessToken).Name)
	})
}

func TestProfil_UbahSatuFieldSajaTidakMenyentuhYangLain(t *testing.T) {
	s := setupSuite(t)
	biasa := s.mulaiSesi(t, emailBiasa, uaMac)

	t.Run("telepon saja", func(t *testing.T) {
		w := s.patchMe(t, biasa.access, map[string]any{"telepon": "+62 857-1234-5678"})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())

		b := s.profilDiDB(t, userBiasa)
		assert.Equal(t, namaAwalBiasa, b.Nama, "nama tidak dikirim, tidak disentuh")
		require.NotNil(t, b.Telepon)
		assert.Equal(t, "+6285712345678", *b.Telepon)
	})
	t.Run("nama saja", func(t *testing.T) {
		w := s.patchMe(t, biasa.access, map[string]any{"nama": "Hanya Nama"})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())

		b := s.profilDiDB(t, userBiasa)
		assert.Equal(t, "Hanya Nama", b.Nama)
		require.NotNil(t, b.Telepon)
		assert.Equal(t, "+6285712345678", *b.Telepon, "telepon tidak dikirim, tidak disentuh")
	})
	t.Run("null sama dengan tidak dikirim", func(t *testing.T) {
		w := s.patchMe(t, biasa.access, map[string]any{"nama": "Nama Dari Null", "telepon": nil})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())

		b := s.profilDiDB(t, userBiasa)
		assert.Equal(t, "Nama Dari Null", b.Nama)
		require.NotNil(t, b.Telepon)
		assert.Equal(t, "+6285712345678", *b.Telepon)
	})
}

func TestProfil_NormalisasiTeleponKeBentukSama(t *testing.T) {
	s := setupSuite(t)
	biasa := s.mulaiSesi(t, emailBiasa, uaMac)

	for _, mentah := range []string{"081234567890", "6281234567890", "+6281234567890", "0812 3456 7890", "(0812) 3456-7890", "+62.812.3456.7890"} {
		w := s.patchMe(t, biasa.access, map[string]any{"telepon": mentah})

		require.Equal(t, http.StatusOK, w.Code, "%s: %s", mentah, w.Body.String())
		assert.Equal(t, "+6281234567890", s.dataObj(t, w)["telepon"], mentah)
		b := s.profilDiDB(t, userBiasa)
		require.NotNil(t, b.Telepon, mentah)
		assert.Equal(t, "+6281234567890", *b.Telepon, mentah)
	}
	assert.EqualValues(t, 1, s.jumlahAuditProfil(t), "enam permintaan menghasilkan nilai yang sama: hanya yang pertama menulis dan diaudit")
}

func TestProfil_HapusTeleponMenyimpanNULLBukanStringKosong(t *testing.T) {
	s := setupSuite(t)
	biasa := s.mulaiSesi(t, emailBiasa, uaMac)
	require.Equal(t, http.StatusOK, s.patchMe(t, biasa.access, map[string]any{"telepon": "0812-3456-7890"}).Code)

	w := s.patchMe(t, biasa.access, map[string]any{"telepon": ""})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.NotContains(t, s.dataObj(t, w), "telepon", "tanpa nomor, field dihilangkan dari jawaban")
	assert.NotContains(t, s.dataObj(t, s.panggil(t, http.MethodGet, "/v1/auth/me", nil, biasa.access)), "telepon")
	assert.EqualValues(t, 1, s.count(t, "SELECT count(*) FROM pengguna WHERE id = ? AND telepon IS NULL", userBiasa), "kolom NULL")
	assert.Zero(t, s.count(t, "SELECT count(*) FROM pengguna WHERE telepon = ''"), "tidak pernah string kosong")
	assert.EqualValues(t, 1, s.count(t, `SELECT count(*) FROM audit_log
		WHERE aksi = 'UPDATE_PROFIL' AND entitas_id = ? AND sebelum->>'telepon' = ? AND jsonb_typeof(sesudah->'telepon') = 'null'`,
		userBiasa, teleponBaru))

	t.Run("menghapus lagi saat sudah kosong tidak menulis", func(t *testing.T) {
		sebelumTs := s.diupdatePada(t, userBiasa)
		jumlahAudit := s.jumlahAuditProfil(t)

		w := s.patchMe(t, biasa.access, map[string]any{"telepon": "   "})

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.True(t, sebelumTs.Equal(s.diupdatePada(t, userBiasa)))
		assert.Equal(t, jumlahAudit, s.jumlahAuditProfil(t))
	})
}

func TestProfil_NilaiSamaTidakMenulisDanTidakMengaudit(t *testing.T) {
	s := setupSuite(t)
	biasa := s.mulaiSesi(t, emailBiasa, uaMac)
	require.Equal(t, http.StatusOK, s.patchMe(t, biasa.access, map[string]any{"nama": "Nama Tetap", "telepon": "0812-3456-7890"}).Code)
	time.Sleep(10 * time.Millisecond)
	sebelumTs := s.diupdatePada(t, userBiasa)
	sebelumAudit := s.jumlahAuditProfil(t)

	w := s.patchMe(t, biasa.access, map[string]any{"nama": "  Nama Tetap ", "telepon": "+62 812 3456 7890"})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	data := s.dataObj(t, w)
	assert.Equal(t, "Nama Tetap", data["nama"])
	assert.Equal(t, teleponBaru, data["telepon"])
	assert.True(t, sebelumTs.Equal(s.diupdatePada(t, userBiasa)), "tidak ada tulis ke basis data (diupdate_pada tidak maju)")
	assert.Equal(t, sebelumAudit, s.jumlahAuditProfil(t), "tanpa audit")
}

// Pemilik hanya dapat mengubah nama dan telepon: field lain di badan diabaikan, dan baris pengguna (juga baris
// pengguna lain) tidak berubah sedikit pun selain kolom itu.
func TestProfil_FieldDiLuarNamaDanTeleponDiabaikan(t *testing.T) {
	s := setupSuite(t)
	biasa := s.mulaiSesi(t, emailBiasa, uaMac)
	lainSebelum := s.barisJSON(t, idAdminUji)
	biasaSebelum := s.barisJSON(t, userBiasa, "nama", "diupdate_pada", "telepon")

	t.Run("bersama nama: hanya nama yang berubah", func(t *testing.T) {
		w := s.patchMe(t, biasa.access, map[string]any{
			"nama": "Nama Aman Abc", "email": "bajak@gardainflasi.id", "peran": "ADMIN_SISTEM", "lapis": "SISTEM",
			"lembaga": "LEMBAGA-PALSU", "kabupaten_kode": "1171", "status": "NONAKTIF", "mfa_aktif": true, "mfa_wajib": true,
			"wajib_ganti_password": true, "password_hash": "x", "id": idAdminUji, "terakhir_masuk": "2000-01-01T00:00:00Z",
			"dibuat_pada": "2000-01-01T00:00:00Z", "permissions": []string{"pengguna.kelola"},
		})

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		data := s.dataObj(t, w)
		assert.Equal(t, userBiasa, data["id"])
		assert.Equal(t, "Nama Aman Abc", data["nama"])
		assert.Equal(t, emailBiasa, data["email"])
		assert.Equal(t, "OPS_MAHASISWA", data["peran"])
		assert.Equal(t, "OPERASIONAL", data["lapis"])
		assert.Equal(t, "BPPSDMP", data["lembaga"])
		assert.Equal(t, "AKTIF", data["status"])
		assert.NotContains(t, data, "kabupaten_kode")
		assert.Equal(t, biasaSebelum, s.barisJSON(t, userBiasa, "nama", "diupdate_pada", "telepon"), "selain nama, seluruh baris utuh")
		assert.Equal(t, lainSebelum, s.barisJSON(t, idAdminUji), "baris pengguna lain tidak tersentuh")
		assert.Equal(t, "Nama Aman Abc", s.profilDiDB(t, userBiasa).Nama)
	})

	t.Run("tanpa nama dan telepon: 400 tidak ada perubahan, tidak ada yang berubah", func(t *testing.T) {
		antara := s.barisJSON(t, userBiasa)
		w := s.patchMe(t, biasa.access, map[string]any{
			"email": "bajak2@gardainflasi.id", "peran": "ADMIN_SISTEM", "lembaga": "LEMBAGA-PALSU", "status": "NONAKTIF",
		})

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Equal(t, "Tidak ada perubahan", s.parseResponse(t, w)["message"])
		assert.Equal(t, antara, s.barisJSON(t, userBiasa))
		assert.Equal(t, lainSebelum, s.barisJSON(t, idAdminUji))
	})

	t.Run("izin tidak bertambah: token tetap milik peran semula", func(t *testing.T) {
		assert.Equal(t, http.StatusForbidden, s.panggil(t, http.MethodGet, "/v1/users", nil, biasa.access).Code)
	})
}

func TestProfil_TanpaTokenDijawab401(t *testing.T) {
	s := setupSuite(t)
	sebelum := s.barisJSON(t, userBiasa, "terakhir_masuk") // subtes ketiga login (terakhir_masuk maju)

	t.Run("tanpa token", func(t *testing.T) {
		w := s.patchMe(t, "", map[string]any{"nama": "Nama Peretas"})
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
	t.Run("token sembarang", func(t *testing.T) {
		w := s.patchMe(t, "bukan.token.sah", map[string]any{"nama": "Nama Peretas"})
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
	t.Run("token yang sudah dicabut", func(t *testing.T) {
		biasa := s.mulaiSesi(t, emailBiasa, uaMac)
		require.Equal(t, http.StatusOK, s.panggil(t, http.MethodPost, "/v1/auth/logout", nil, biasa.access).Code)

		w := s.patchMe(t, biasa.access, map[string]any{"nama": "Nama Peretas"})
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	assert.Equal(t, sebelum, s.barisJSON(t, userBiasa, "terakhir_masuk"), "tidak ada yang berubah")
	assert.Zero(t, s.jumlahAuditProfil(t))
}

func TestProfil_GalatValidasiDijawab400DenganPesanKontrak(t *testing.T) {
	s := setupSuite(t)
	biasa := s.mulaiSesi(t, emailBiasa, uaMac)
	require.Equal(t, http.StatusOK, s.patchMe(t, biasa.access, map[string]any{"telepon": "0812-3456-7890"}).Code)
	awal := s.barisJSON(t, userBiasa)
	auditAwal := s.jumlahAuditProfil(t)

	cases := []struct {
		name    string
		body    any
		mentah  *string
		pesan   string
		tanpaJS bool // pesan hanya diawali (galat bawaan binding)
	}{
		{name: "nama kosong", body: map[string]any{"nama": ""}, pesan: "Nama minimal 3 karakter"},
		{name: "nama spasi semua", body: map[string]any{"nama": "      "}, pesan: "Nama minimal 3 karakter"},
		{name: "nama 2 karakter", body: map[string]any{"nama": "Al"}, pesan: "Nama minimal 3 karakter"},
		{name: "nama 2 karakter berspasi tepi", body: map[string]any{"nama": "   Al   "}, pesan: "Nama minimal 3 karakter"},
		{name: "nama 101 karakter", body: map[string]any{"nama": strings.Repeat("a", 101)}, pesan: "Nama maksimal 100 karakter"},
		{name: "nama 101 karakter multibita", body: map[string]any{"nama": strings.Repeat("é", 101)}, pesan: "Nama maksimal 100 karakter"},
		{name: "telepon kode negara lain", body: map[string]any{"telepon": "+1 415 555 2671"}, pesan: "Nomor telepon tidak valid"},
		{name: "telepon tanpa awalan", body: map[string]any{"telepon": "8123456789"}, pesan: "Nomor telepon tidak valid"},
		{name: "telepon terlalu pendek", body: map[string]any{"telepon": "0812345"}, pesan: "Nomor telepon tidak valid"},
		{name: "telepon terlalu panjang", body: map[string]any{"telepon": "0812345678901234"}, pesan: "Nomor telepon tidak valid"},
		{name: "telepon berhuruf", body: map[string]any{"telepon": "0812abc34567"}, pesan: "Nomor telepon tidak valid"},
		{name: "telepon +620", body: map[string]any{"telepon": "+6208123456789"}, pesan: "Nomor telepon tidak valid"},
		{name: "nama sah, telepon tidak: tidak ada yang ditulis", body: map[string]any{"nama": "Nama Sah Saja", "telepon": "abc"}, pesan: "Nomor telepon tidak valid"},
		{name: "telepon sah, nama tidak: tidak ada yang ditulis", body: map[string]any{"nama": "ab", "telepon": "081299998888"}, pesan: "Nama minimal 3 karakter"},
		{name: "objek kosong", body: map[string]any{}, pesan: "Tidak ada perubahan"},
		{name: "keduanya null", body: map[string]any{"nama": nil, "telepon": nil}, pesan: "Tidak ada perubahan"},
		{name: "badan kosong", body: nil, pesan: "Tidak ada perubahan"},
		{name: "JSON rusak", mentah: ptr(`{"nama": `), pesan: "Validasi gagal", tanpaJS: true},
		{name: "tipe salah", body: map[string]any{"nama": 123}, pesan: "Validasi gagal", tanpaJS: true},
		{name: "pagar mentah nama", body: map[string]any{"nama": strings.Repeat("a", 1001)}, pesan: "Validasi gagal", tanpaJS: true},
		{name: "pagar mentah telepon", body: map[string]any{"telepon": strings.Repeat("1", 101)}, pesan: "Validasi gagal", tanpaJS: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var w *httptest.ResponseRecorder
			if tc.mentah != nil {
				w = s.patchMeMentah(t, biasa.access, *tc.mentah)
			} else {
				w = s.patchMe(t, biasa.access, tc.body)
			}

			assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
			jawab := s.parseResponse(t, w)
			assert.Equal(t, "400", jawab["code"])
			if tc.tanpaJS {
				assert.True(t, strings.HasPrefix(jawab["message"].(string), tc.pesan), "%v", jawab["message"])
			} else {
				assert.Equal(t, tc.pesan, jawab["message"])
			}
			assert.NotContains(t, jawab, "data")
		})
	}

	assert.Equal(t, awal, s.barisJSON(t, userBiasa), "galat validasi tidak mengubah apa pun")
	assert.Equal(t, auditAwal, s.jumlahAuditProfil(t), "galat validasi tidak diaudit")
}

func TestProfil_NamaSeratusKarakterMultibitaDiterima(t *testing.T) {
	s := setupSuite(t)
	biasa := s.mulaiSesi(t, emailBiasa, uaMac)
	nama := strings.Repeat("😀", 100) // 100 rune, 400 byte

	w := s.patchMe(t, biasa.access, map[string]any{"nama": nama})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, nama, s.profilDiDB(t, userBiasa).Nama, "batas dihitung dalam rune, bukan byte")
}

func TestProfil_NamaMengandungNULDibersihkanBukanGalatServer(t *testing.T) {
	s := setupSuite(t)
	biasa := s.mulaiSesi(t, emailBiasa, uaMac)

	w := s.patchMe(t, biasa.access, map[string]any{"nama": "Bu\u0000di Santoso"})

	require.Equal(t, http.StatusOK, w.Code, "PostgreSQL menolak NUL; usecase membuangnya lebih dulu: %s", w.Body.String())
	assert.Equal(t, "Budi Santoso", s.profilDiDB(t, userBiasa).Nama)
}

// Semua peran boleh mengubah profil sendiri (tidak ada izin khusus).
func TestProfil_PatchBerhasilUntukSemuaPeran(t *testing.T) {
	hash, err := crypto.HashPassword("Test1234!")
	require.NoError(t, err)
	peran := usecase.DaftarPeran()
	require.Len(t, peran, 14)
	s := setupSuiteWith(t, func(t *testing.T, db *gorm.DB) {
		seedData(t, db)
		for _, p := range peran {
			require.NoError(t, db.Exec(`INSERT INTO pengguna (id, nama, email, password_hash, peran, lapis, lembaga, status)
				VALUES (garda_uuid_stabil('pengguna', ?), ?, ?, ?, ?, ?, 'BPPSDMP', 'AKTIF')`,
				"profil-"+p.Kunci, "Pengguna "+p.Kunci, strings.ToLower(p.Kunci)+".profil@gardainflasi.id", hash, p.Kunci, p.Lapis).Error)
		}
	})

	for i, p := range peran {
		t.Run(p.Kunci, func(t *testing.T) {
			email := strings.ToLower(p.Kunci) + ".profil@gardainflasi.id"
			sesi := s.mulaiSesi(t, email, uaMac)
			id := uid("profil-" + p.Kunci)
			telepon := "+62812000" + strings.Repeat("1", 4) + string(rune('0'+i%10))

			w := s.patchMe(t, sesi.access, map[string]any{"nama": "Diubah " + p.Kunci, "telepon": telepon})

			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			data := s.dataObj(t, w)
			assert.Equal(t, id, data["id"])
			assert.Equal(t, p.Kunci, data["peran"], "peran tidak berubah")
			assert.Equal(t, "Diubah "+p.Kunci, data["nama"])
			assert.Equal(t, telepon, data["telepon"])
			assert.EqualValues(t, 1, s.auditCount(t, "UPDATE_PROFIL", id, id))
		})
	}
}

// Sesi terbatas (wajib ganti kata sandi atau wajib setup MFA) boleh membaca profil, bukan mengubahnya.
func TestProfil_SesiTerbatasHanyaBolehMembaca(t *testing.T) {
	t.Run("wajib ganti kata sandi", func(t *testing.T) {
		s := newSuite(t, seedAdminAwal)
		access, _ := s.loginAs(t, adminEmail, adminTempSandi)
		sebelum := s.barisJSON(t, uid("admin-awal"))

		assert.Equal(t, http.StatusOK, s.panggil(t, http.MethodGet, "/v1/auth/me", nil, access).Code, "membaca tetap boleh")
		w := s.patchMe(t, access, map[string]any{"nama": "Admin Baru Nama", "telepon": "081234567890"})

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, middleware.PasswordChangeRequiredCode, s.parseResponse(t, w)["code"])
		assert.Equal(t, sebelum, s.barisJSON(t, uid("admin-awal")))

		cp := s.serveRequest(t, http.MethodPost, "/v1/auth/change-password",
			map[string]any{"old_password": adminTempSandi, "new_password": adminNewPassword}, s.authHeader(access))
		require.Equal(t, http.StatusOK, cp.Code, cp.Body.String())
		var sesi struct {
			AccessToken string `json:"access_token"`
		}
		s.decodeData(t, cp.Body.Bytes(), &sesi)
		assert.Equal(t, http.StatusOK, s.patchMe(t, sesi.AccessToken, map[string]any{"nama": "Admin Baru Nama"}).Code, "setelah kewajiban selesai boleh")
		assert.Equal(t, "Admin Baru Nama", s.profilDiDB(t, uid("admin-awal")).Nama)
	})
	t.Run("wajib setup MFA", func(t *testing.T) {
		s := newSuite(t, seedData, penegakan(t))
		access, _ := s.loginAs(t, "test@gardainflasi.id", "Test1234!") // ADMIN_SISTEM: peran wajib MFA
		sebelum := s.barisJSON(t, idAdminUji)

		assert.Equal(t, http.StatusOK, s.panggil(t, http.MethodGet, "/v1/auth/me", nil, access).Code, "membaca tetap boleh")
		w := s.patchMe(t, access, map[string]any{"nama": "Nama Tanpa MFA"})

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, middleware.MFASetupRequiredCode, s.parseResponse(t, w)["code"])
		assert.Equal(t, sebelum, s.barisJSON(t, idAdminUji))
	})
}

// Akun ber-MFA aktif: jawaban PATCH memuat kode_cadangan_tersisa seperti GET /v1/auth/me.
func TestProfil_JawabanMemuatStatusMFASepertiGetMe(t *testing.T) {
	s := setupSuite(t)
	sesi := s.sesiMFA(t)

	w := s.patchMe(t, sesi, map[string]any{"nama": "Nama Akun MFA"})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	data := s.dataObj(t, w)
	assert.Equal(t, true, data["mfa_aktif"])
	assert.EqualValues(t, len(kodeCadanganUji), data["kode_cadangan_tersisa"])
	assert.Equal(t, data, s.dataObj(t, s.panggil(t, http.MethodGet, "/v1/auth/me", nil, sesi)))
}
