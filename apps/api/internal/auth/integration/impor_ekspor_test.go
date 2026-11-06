package integration

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/garda-inflasi/api/internal/auth/domain"
)

const (
	headerCSV = "nama,email,peran,lembaga,kabupaten_kode"
	bomUji    = "\xef\xbb\xbf"
)

func (s *Suite) impor(t *testing.T, access, csvTeks string, dryRun bool) *httptest.ResponseRecorder {
	t.Helper()
	return s.panggil(t, http.MethodPost, "/v1/users/impor", map[string]any{"csv": csvTeks, "dry_run": dryRun}, access)
}

type hasilImporUji struct {
	DryRun     bool `json:"dry_run"`
	Total      int
	Valid      int
	Galat      int
	Dibuat     int
	EmailAktif bool `json:"email_aktif"`
	Baris      []struct {
		Baris  int
		Email  string
		Status string
		Galat  []string
		ID     string
	}
}

func (s *Suite) bacaImpor(t *testing.T, w *httptest.ResponseRecorder) hasilImporUji {
	t.Helper()
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var env struct{ Data hasilImporUji }
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	return env.Data
}

// ── Impor ─────────────────────────────────────────────────────────────

func TestImpor_DryRunMemeriksaTanpaMenulis(t *testing.T) {
	s, smtp := setupSuiteSMTP(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	csvTeks := headerCSV + "\n" +
		"Budi Santoso,budi@gardainflasi.id,OPS_MAHASISWA,FMIPA USK,1171\n" +
		",tanpa.nama@gardainflasi.id,OPS_MAHASISWA,FMIPA,\n" +
		"Siti,bukan-email,OPS_MAHASISWA,FMIPA,\n" +
		"Agus,agus@gardainflasi.id,DEWA,FMIPA,\n" +
		"Dedi,dedi@gardainflasi.id,OPS_MAHASISWA,FMIPA,9999\n" +
		"Budi Dua,BUDI@gardainflasi.id,OPS_MAHASISWA,FMIPA,\n" +
		"Sudah,UJI.BIASA@gardainflasi.id,OPS_MAHASISWA,FMIPA,\n"

	h := s.bacaImpor(t, s.impor(t, admin.access, csvTeks, true))

	assert.True(t, h.DryRun)
	assert.Equal(t, 7, h.Total)
	assert.Equal(t, 1, h.Valid)
	assert.Equal(t, 6, h.Galat)
	assert.Zero(t, h.Dibuat)
	require.Len(t, h.Baris, 7)
	assert.Equal(t, "valid", h.Baris[0].Status)
	assert.Equal(t, 2, h.Baris[0].Baris)
	assert.Equal(t, []string{}, h.Baris[0].Galat, "galat selalu larik, tidak pernah null")
	assert.Contains(t, h.Baris[1].Galat, "Nama wajib diisi.")
	assert.Contains(t, h.Baris[2].Galat, "Email tidak valid.")
	assert.Contains(t, h.Baris[3].Galat, "Peran tidak dikenal: DEWA.")
	assert.Contains(t, h.Baris[4].Galat, "Kode kabupaten tidak dikenal: 9999.")
	assert.Contains(t, h.Baris[5].Galat, "Email sama dengan baris 2.")
	assert.Contains(t, h.Baris[6].Galat, "Email sudah terdaftar.")

	assert.Zero(t, s.hitung(t, "SELECT count(*) FROM pengguna WHERE email = 'budi@gardainflasi.id'"), "dry run tidak membuat akun")
	assert.Zero(t, s.hitung(t, "SELECT count(*) FROM undangan_pengguna"))
	assert.Zero(t, s.hitung(t, "SELECT count(*) FROM audit_log WHERE aksi IN ('UNDANG_PENGGUNA', 'IMPOR_PENGGUNA')"), "dry run tidak diaudit")
	assert.Empty(t, smtp.Messages())
}

func TestImpor_KomitMembuatBarisSahSebagaiUndangan(t *testing.T) {
	s, smtp := setupSuiteSMTP(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	csvTeks := headerCSV + "\n" +
		"Budi Santoso,budi@gardainflasi.id,OPS_MAHASISWA,FMIPA USK,1171\n" +
		"Siti Rahma,siti@gardainflasi.id,OPS_AGREGATOR,BPS Aceh,\n" +
		"Salah,bukan-email,OPS_MAHASISWA,FMIPA,\n"

	h := s.bacaImpor(t, s.impor(t, admin.access, csvTeks, false))

	assert.False(t, h.DryRun)
	assert.Equal(t, 2, h.Dibuat)
	assert.Equal(t, 1, h.Galat)
	assert.True(t, h.EmailAktif)
	require.Len(t, h.Baris, 3)
	assert.Equal(t, "dibuat", h.Baris[0].Status)
	assert.NotEmpty(t, h.Baris[0].ID)
	assert.Equal(t, "galat", h.Baris[2].Status)

	t.Run("akun MENUNGGU tanpa kata sandi, tanpa akun untuk baris salah", func(t *testing.T) {
		assert.EqualValues(t, 2, s.hitung(t, "SELECT count(*) FROM pengguna WHERE status = 'MENUNGGU' AND password_hash IS NULL"))
		assert.EqualValues(t, 2, s.hitung(t, "SELECT count(*) FROM undangan_pengguna WHERE used_at IS NULL AND dicabut_pada IS NULL"))
		assert.Zero(t, s.hitung(t, "SELECT count(*) FROM pengguna WHERE email = 'bukan-email'"))
	})
	t.Run("email undangan per akun yang dibuat; tautan bisa dipakai", func(t *testing.T) {
		token := smtp.Tunggu(t, "budi@gardainflasi.id", 1)[0].Token()
		smtp.Tunggu(t, "siti@gardainflasi.id", 1)
		assert.Len(t, smtp.Messages(), 2)
		assert.Equal(t, http.StatusOK, s.aktivasi(t, token, sandiAktivasi).Code)
		s.loginAs(t, "budi@gardainflasi.id", sandiAktivasi)
	})
	t.Run("audit: satu entri per akun dan satu ringkasan", func(t *testing.T) {
		assert.EqualValues(t, 2, s.hitung(t, "SELECT count(*) FROM audit_log WHERE aksi = 'UNDANG_PENGGUNA' AND aktor_id = 'test-user-001' AND metadata->>'sumber' = 'impor'"))
		assert.EqualValues(t, 1, s.hitung(t, "SELECT count(*) FROM audit_log WHERE aksi = 'IMPOR_PENGGUNA' AND (metadata->>'dibuat')::int = 2 AND (metadata->>'ditolak')::int = 1"))
	})
	t.Run("komit ulang berkas yang sama: semua baris kini terdaftar", func(t *testing.T) {
		ulang := s.bacaImpor(t, s.impor(t, admin.access, csvTeks, false))
		assert.Zero(t, ulang.Dibuat)
		assert.Equal(t, 3, ulang.Galat)
		assert.Contains(t, ulang.Baris[0].Galat, "Email sudah terdaftar.")
	})
}

func TestImpor_BOMTitikKomaDanCRLFMelaluiAPI(t *testing.T) {
	s, _ := setupSuiteSMTP(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	csvTeks := bomUji + "nama;email;peran;lembaga;kabupaten_kode\r\n" +
		"\"Santoso; Budi\";excel@gardainflasi.id;OPS_MAHASISWA;\"FMIPA \"\"USK\"\"\";1171\r\n"

	h := s.bacaImpor(t, s.impor(t, admin.access, csvTeks, false))

	assert.Equal(t, 1, h.Dibuat)
	var row struct{ Nama, Lembaga string }
	require.NoError(t, s.db.Raw("SELECT nama, lembaga FROM pengguna WHERE email = 'excel@gardainflasi.id'").Scan(&row).Error)
	assert.Equal(t, "Santoso; Budi", row.Nama)
	assert.Equal(t, "FMIPA \"USK\"", row.Lembaga)
}

func TestImpor_GalatBerkasDan400(t *testing.T) {
	s, smtp := setupSuiteSMTP(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	baris := func(n int) string {
		var sb strings.Builder
		sb.WriteString(headerCSV + "\n")
		for i := range n {
			fmt.Fprintf(&sb, "Nama %d,u%d@gardainflasi.id,OPS_MAHASISWA,FMIPA,\n", i, i)
		}
		return sb.String()
	}
	kasus := map[string]struct {
		csv   string
		pesan string
	}{
		"kolom tak dikenal":   {"nama,email,peran,lembaga,telepon\nA,a@b.id,OPS_MAHASISWA,X,1\n", "Kolom tidak dikenal"},
		"kolom wajib hilang":  {"nama,email\nA,a@b.id\n", "Kolom wajib tidak ada"},
		"hanya header":        {headerCSV + "\n", "tidak ada baris data"},
		"501 baris":           {baris(501), "lebih dari 500 baris"},
		"bukan UTF-8":         {headerCSV + "\nBudi \xff,b@c.id,OPS_MAHASISWA,X,\n", "UTF-8"},
		"kutip tak berpasang": {headerCSV + "\n\"Budi,b@c.id,OPS_MAHASISWA,X,\n", "kutip"},
	}
	for nama, k := range kasus {
		for _, dry := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s (dry_run=%v)", nama, dry), func(t *testing.T) {
				w := s.impor(t, admin.access, k.csv, dry)
				assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
				assert.Contains(t, s.pesan(t, w), k.pesan)
			})
		}
	}
	t.Run("500 baris diterima", func(t *testing.T) {
		h := s.bacaImpor(t, s.impor(t, admin.access, baris(500), true))
		assert.Equal(t, 500, h.Valid)
	})
	t.Run("badan tanpa csv atau dry_run", func(t *testing.T) {
		for _, badan := range []map[string]any{{"dry_run": true}, {"csv": baris(1)}, {"csv": "", "dry_run": true}} {
			w := s.panggil(t, http.MethodPost, "/v1/users/impor", badan, admin.access)
			assert.Equal(t, http.StatusBadRequest, w.Code, "%v: %s", badan, w.Body.String())
		}
	})
	t.Run("dry_run wajib eksplisit: lupa mengisinya tidak berarti komit", func(t *testing.T) {
		w := s.panggil(t, http.MethodPost, "/v1/users/impor", map[string]any{"csv": baris(1)}, admin.access)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Zero(t, s.hitung(t, "SELECT count(*) FROM pengguna WHERE email = 'u0@gardainflasi.id'"))
	})
	t.Run("berkas terlalu besar dijawab 413", func(t *testing.T) {
		besar := headerCSV + "\n" + strings.Repeat("x", 520*1024)
		w := s.impor(t, admin.access, besar, true)
		assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code, w.Body.String())
	})
	t.Run("badan JSON jauh melebihi batas dijawab 413 tanpa dibaca habis", func(t *testing.T) {
		w := s.impor(t, admin.access, strings.Repeat("x", 3*1024*1024), true)
		assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code, w.Body.String())
	})
	assert.Zero(t, s.hitung(t, "SELECT count(*) FROM undangan_pengguna"))
	assert.Empty(t, smtp.Messages())
}

// BuatUndangan menulis semua akun dalam satu transaksi: satu duplikat membatalkan seluruh kelompok.
func TestImpor_PenulisanAtomik(t *testing.T) {
	s := setupSuite(t)
	repo := newRepo(s)
	akun := func(id, mail string) domain.UndanganBaru {
		return domain.UndanganBaru{
			User:     &domain.Pengguna{ID: id, Nama: id, Email: mail, Peran: "OPS_MAHASISWA", Lapis: "OPERASIONAL", Lembaga: "X", Status: "MENUNGGU"},
			Undangan: &domain.Undangan{TokenHash: hashToken(id), Email: mail, ExpiresAt: time.Now().Add(72 * time.Hour)},
		}
	}

	err := repo.BuatUndangan(context.Background(), []domain.UndanganBaru{
		akun("atomik-1", "atomik1@gardainflasi.id"),
		akun("atomik-2", "atomik2@gardainflasi.id"),
		akun("atomik-3", "UJI.BIASA@gardainflasi.id"), // sudah terdaftar (beda huruf)
	})

	require.ErrorIs(t, err, gorm.ErrDuplicatedKey)
	assert.Zero(t, s.hitung(t, "SELECT count(*) FROM pengguna WHERE id LIKE 'atomik-%'"), "akun sebelum duplikat ikut dibatalkan")
	assert.Zero(t, s.hitung(t, "SELECT count(*) FROM undangan_pengguna"), "undangan ikut dibatalkan")

	require.NoError(t, repo.BuatUndangan(context.Background(), []domain.UndanganBaru{akun("atomik-1", "atomik1@gardainflasi.id"), akun("atomik-2", "atomik2@gardainflasi.id")}))
	assert.EqualValues(t, 2, s.hitung(t, "SELECT count(*) FROM undangan_pengguna"))
}

// Dua impor serentak atas berkas yang sama: pemeriksaan mereka sama-sama lolos, tetapi hanya satu yang berhasil
// menulis; yang kalah dibatalkan seluruhnya (409) dan tidak menyisakan akun setengah jadi.
func TestImpor_KomitSerentakHanyaSatuMenang(t *testing.T) {
	s, _ := setupSuiteSMTP(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	csvTeks := headerCSV + "\nA,a1@gardainflasi.id,OPS_MAHASISWA,X,\nB,b1@gardainflasi.id,OPS_MAHASISWA,X,\nC,c1@gardainflasi.id,OPS_MAHASISWA,X,\n"

	const jumlah = 4
	hasil := make([]*httptest.ResponseRecorder, jumlah)
	var wg sync.WaitGroup
	for i := range jumlah {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hasil[i] = s.impor(t, admin.access, csvTeks, false)
		}()
	}
	wg.Wait()

	var menang, kalah409, kalahPeriksa int
	for _, w := range hasil {
		switch w.Code {
		case http.StatusOK:
			var env struct{ Data hasilImporUji }
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
			if env.Data.Dibuat == 3 {
				menang++
			} else {
				kalahPeriksa++ // terlambat: pemeriksaan melihat email sudah terdaftar
				assert.Zero(t, env.Data.Dibuat)
			}
		case http.StatusConflict:
			kalah409++
		default:
			t.Fatalf("status tak terduga %d: %s", w.Code, w.Body.String())
		}
	}
	assert.Equal(t, 1, menang)
	assert.Equal(t, jumlah-1, kalah409+kalahPeriksa)
	assert.EqualValues(t, 3, s.hitung(t, "SELECT count(*) FROM pengguna WHERE status = 'MENUNGGU'"))
	assert.EqualValues(t, 3, s.hitung(t, "SELECT count(*) FROM undangan_pengguna"))
}

// ── Ekspor ────────────────────────────────────────────────────────────

func (s *Suite) ekspor(t *testing.T, access, query string) *httptest.ResponseRecorder {
	t.Helper()
	path := "/v1/users/ekspor"
	if query != "" {
		path += "?" + query
	}
	return s.panggil(t, http.MethodGet, path, nil, access)
}

func bacaEkspor(t *testing.T, w *httptest.ResponseRecorder) [][]string {
	t.Helper()
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := w.Body.Bytes()
	require.True(t, bytes.HasPrefix(body, []byte(bomUji)), "UTF-8 BOM")
	r := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(body, []byte(bomUji))))
	r.Comma = ';'
	rekaman, err := r.ReadAll()
	require.NoError(t, err)
	return rekaman
}

func kolomEkspor(rekaman [][]string, nama string) []string {
	idx := -1
	for i, h := range rekaman[0] {
		if h == nama {
			idx = i
		}
	}
	var out []string
	for _, r := range rekaman[1:] {
		out = append(out, r[idx])
	}
	return out
}

func TestEkspor_IsiHeaderDanAudit(t *testing.T) {
	s, _ := setupSuiteSMTP(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	require.Equal(t, http.StatusCreated, s.undang(t, admin.access, isianSah("menunggu@gardainflasi.id")).Code)

	w := s.ekspor(t, admin.access, "")

	rekaman := bacaEkspor(t, w)
	assert.Equal(t, []string{"id", "nama", "email", "peran", "lapis", "lembaga", "kabupaten_kode", "status", "mfa_aktif", "terakhir_masuk", "dibuat_pada"}, rekaman[0])
	emails := kolomEkspor(rekaman, "email")
	assert.Contains(t, emails, "test@gardainflasi.id")
	assert.Contains(t, emails, "uji.biasa@gardainflasi.id")
	assert.Contains(t, emails, "menunggu@gardainflasi.id")
	assert.Equal(t, "MENUNGGU", kolomEkspor(rekaman, "status")[indeks(emails, "menunggu@gardainflasi.id")])
	assert.Equal(t, "true", kolomEkspor(rekaman, "mfa_aktif")[indeks(emails, "uji.mfa@gardainflasi.id")])

	t.Run("header unduhan", func(t *testing.T) {
		assert.Equal(t, "text/csv; charset=utf-8", w.Header().Get("Content-Type"))
		assert.Regexp(t, `^attachment; filename="pengguna-\d{4}-\d{2}-\d{2}\.csv"$`, w.Header().Get("Content-Disposition"))
		assert.Equal(t, fmt.Sprint(len(rekaman)-1), w.Header().Get("X-Export-Rows"))
		assert.Empty(t, w.Header().Get("X-Export-Truncated"))
		assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	})
	t.Run("tanpa rahasia", func(t *testing.T) {
		body := w.Body.String()
		for _, terlarang := range []string{"$2a$", "$2b$", "v1:", "password", "mfa_secret", "BACKUP0", "JBSWY3DP"} {
			assert.NotContains(t, body, terlarang)
		}
		var hash string
		require.NoError(t, s.db.Raw("SELECT password_hash FROM pengguna WHERE id = 'test-user-001'").Scan(&hash).Error)
		assert.NotContains(t, body, hash)
	})
	t.Run("tercatat sebagai EKSPOR_DATA", func(t *testing.T) {
		var row struct {
			AktorID   string
			EntitasID string
			Hasil     string
			Baris     int
			Terpotong bool
		}
		require.NoError(t, s.db.Raw(`SELECT aktor_id, entitas_id, hasil, (metadata->>'baris')::int AS baris, (metadata->>'terpotong')::boolean AS terpotong
			FROM audit_log WHERE aksi = 'EKSPOR_DATA' AND entitas = 'pengguna'`).Scan(&row).Error)
		assert.Equal(t, "test-user-001", row.AktorID)
		assert.Equal(t, "ekspor", row.EntitasID)
		assert.Equal(t, "BERHASIL", row.Hasil)
		assert.Equal(t, len(rekaman)-1, row.Baris)
		assert.False(t, row.Terpotong)
	})
}

func indeks(daftar []string, nilai string) int {
	for i, v := range daftar {
		if v == nilai {
			return i
		}
	}
	return -1
}

func TestEkspor_FilterSamaDenganDaftar(t *testing.T) {
	s := setupSuite(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)

	t.Run("pencarian", func(t *testing.T) {
		rekaman := bacaEkspor(t, s.ekspor(t, admin.access, "search="+url.QueryEscape("MFA")))
		daftar := s.panggil(t, http.MethodGet, "/v1/users?search="+url.QueryEscape("MFA")+"&limit=100", nil, admin.access)
		require.Equal(t, http.StatusOK, daftar.Code)
		data := s.parseResponse(t, daftar)["data"].(map[string]any)
		var dariDaftar []string
		for _, it := range data["items"].([]any) {
			dariDaftar = append(dariDaftar, it.(map[string]any)["id"].(string))
		}
		assert.ElementsMatch(t, dariDaftar, kolomEkspor(rekaman, "id"), "ekspor memuat persis yang tampil di daftar")
		assert.Equal(t, []string{"test-user-mfa"}, kolomEkspor(rekaman, "id"))
	})
	t.Run("id terpilih", func(t *testing.T) {
		rekaman := bacaEkspor(t, s.ekspor(t, admin.access, "ids="+url.QueryEscape("test-user-biasa, test-user-002 ,test-user-biasa,tidak-ada")))
		assert.ElementsMatch(t, []string{"test-user-biasa", "test-user-002"}, kolomEkspor(rekaman, "id"))
	})
	t.Run("pencarian dan id digabung", func(t *testing.T) {
		rekaman := bacaEkspor(t, s.ekspor(t, admin.access, "search=biasa&ids=test-user-001,test-user-biasa"))
		assert.Equal(t, []string{"test-user-biasa"}, kolomEkspor(rekaman, "id"))
	})
	t.Run("tidak ada yang cocok: hanya header, tetap tercatat", func(t *testing.T) {
		w := s.ekspor(t, admin.access, "search=tidak-ada-yang-cocok")
		rekaman := bacaEkspor(t, w)
		assert.Len(t, rekaman, 1)
		assert.Equal(t, "0", w.Header().Get("X-Export-Rows"))
	})
	t.Run("id terpilih yang kosong tidak berubah menjadi semua pengguna", func(t *testing.T) {
		for _, q := range []string{"ids=,", "ids=%20,%20"} {
			w := s.ekspor(t, admin.access, q)
			assert.Equal(t, http.StatusBadRequest, w.Code, q)
		}
	})
	t.Run("terlalu banyak id ditolak", func(t *testing.T) {
		ids := make([]string, 501)
		for i := range ids {
			ids[i] = fmt.Sprintf("u%d", i)
		}
		w := s.ekspor(t, admin.access, "ids="+strings.Join(ids, ","))
		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	})
}

func TestEkspor_InjeksiRumusDinetralkan(t *testing.T) {
	s := setupSuite(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	require.NoError(t, s.db.Exec(`INSERT INTO pengguna (id, nama, email, peran, lapis, lembaga, status) VALUES
		('inj-1', '=HYPERLINK("http://jahat.test","klik")', 'inj1@gardainflasi.id', 'OPS_MAHASISWA', 'OPERASIONAL', '+62811', 'AKTIF'),
		('inj-2', E'Santoso; "Budi"\nKedua', 'inj2@gardainflasi.id', 'OPS_MAHASISWA', 'OPERASIONAL', '@SUM(A1)', 'AKTIF')`).Error)

	w := s.ekspor(t, admin.access, "ids=inj-1,inj-2")

	rekaman := bacaEkspor(t, w)
	nama := kolomEkspor(rekaman, "nama")
	lembaga := kolomEkspor(rekaman, "lembaga")
	ids := kolomEkspor(rekaman, "id")
	for i, id := range ids {
		switch id {
		case "inj-1":
			assert.Equal(t, `'=HYPERLINK("http://jahat.test","klik")`, nama[i])
			assert.Equal(t, "'+62811", lembaga[i])
		case "inj-2":
			assert.Equal(t, "Santoso; \"Budi\"\nKedua", nama[i], "pemisah, kutip, dan baris baru dikutip dengan benar")
			assert.Equal(t, "'@SUM(A1)", lembaga[i])
		}
	}
	assert.Len(t, ids, 2)
}

// Ekspor yang tidak bisa dicatat tidak boleh menyerahkan data.
func TestEkspor_AuditGagalTidakMenyerahkanData(t *testing.T) {
	s := setupSuite(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	require.NoError(t, s.db.Exec("ALTER TABLE audit_log RENAME TO audit_log_rusak").Error)

	w := s.ekspor(t, admin.access, "")

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.NotContains(t, w.Body.String(), "uji.biasa@gardainflasi.id")
	assert.NotContains(t, w.Header().Get("Content-Type"), "text/csv")
	assert.Empty(t, w.Header().Get("Content-Disposition"))
}

func TestEkspor_PenggunaTanpaIzinTidakMendapatData(t *testing.T) {
	s := setupSuite(t)
	biasa := s.mulaiSesi(t, emailBiasa, uaMac)

	w := s.ekspor(t, biasa.access, "")

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.NotContains(t, w.Body.String(), "@gardainflasi.id")
}
