package http

import (
	"errors"
	"fmt"
	nethttp "net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/internal/auth/usecase"
	"github.com/garda-inflasi/api/pkg/email"
	"github.com/garda-inflasi/api/pkg/email/emailtest"
)

// Tes handler undang pengguna, kirim ulang undangan, aktivasi publik, impor CSV, dan ekspor CSV: validasi
// binding, pemetaan galat → status, dan hal yang tidak boleh bocor (token, hash, kata sandi).

const badanUndang = `{"nama":"Budi","email":"Budi@Garda.Test","peran":"OPS_MAHASISWA","lembaga":"BPPSDMP"}`

// ── POST /users/undang ────────────────────────────────────────────────

func TestUndangPengguna_ValidasiBindingDanIsian(t *testing.T) {
	r, rec := routerPengguna(t, &repoUji{})
	cases := map[string]struct {
		badan string
		muat  string
	}{
		"JSON rusak":     {`{"nama":`, ""},
		"nama hilang":    {`{"email":"b@garda.test","peran":"OPS_MAHASISWA","lembaga":"X"}`, ""},
		"email hilang":   {`{"nama":"B","peran":"OPS_MAHASISWA","lembaga":"X"}`, ""},
		"peran hilang":   {`{"nama":"B","email":"b@garda.test","lembaga":"X"}`, ""},
		"lembaga hilang": {`{"nama":"B","email":"b@garda.test","peran":"OPS_MAHASISWA"}`, ""},
		// Validasi isi ada di usecase (pesan yang sama dengan impor CSV) dan digabung dalam satu jawaban.
		"email tak valid":       {`{"nama":"B","email":"bukan email","peran":"OPS_MAHASISWA","lembaga":"X"}`, "Email tidak valid"},
		"peran tak dikenal":     {`{"nama":"B","email":"b@garda.test","peran":"SUPERADMIN","lembaga":"X"}`, "Peran tidak dikenal"},
		"rumus di nama":         {`{"nama":"=1+1","email":"b@garda.test","peran":"OPS_MAHASISWA","lembaga":"X"}`, "rumus"},
		"kabupaten tak sah":     {`{"nama":"B","email":"b@garda.test","peran":"OPS_MAHASISWA","lembaga":"X","kabupaten_kode":"12"}`, "4 digit"},
		"kabupaten tak dikenal": {`{"nama":"B","email":"b@garda.test","peran":"OPS_MAHASISWA","lembaga":"X","kabupaten_kode":"9999"}`, "tidak dikenal"},
	}
	for nama, tc := range cases {
		t.Run(nama, func(t *testing.T) {
			h := panggilJSON(t, r, nethttp.MethodPost, "/users/undang", tc.badan)
			assert.Equal(t, nethttp.StatusBadRequest, h.Code, h.Body.String())
			assert.Contains(t, h.pesan(), tc.muat)
		})
	}
	assert.Empty(t, rec.Entries())
}

func TestUndangPengguna_PemetaanGalat(t *testing.T) {
	t.Run("email sudah terdaftar", func(t *testing.T) {
		r, rec := routerPengguna(t, &repoUji{buatUndangan: func([]domain.UndanganBaru) error {
			return fmt.Errorf("buat: %w", gorm.ErrDuplicatedKey)
		}})
		h := panggilJSON(t, r, nethttp.MethodPost, "/users/undang", badanUndang)
		assert.Equal(t, nethttp.StatusConflict, h.Code)
		assert.Contains(t, h.pesan(), "sudah terdaftar")
		assert.Empty(t, rec.Entries())
	})
	t.Run("galat basis data", func(t *testing.T) {
		r, rec := routerPengguna(t, &repoUji{buatUndangan: func([]domain.UndanganBaru) error { return galatDB() }})
		h := panggilJSON(t, r, nethttp.MethodPost, "/users/undang", badanUndang)
		assert.Equal(t, nethttp.StatusInternalServerError, h.Code)
		tanpaBocor(t, h)
		assert.Empty(t, rec.Entries())
	})
	t.Run("galat memeriksa kabupaten", func(t *testing.T) {
		r, _ := routerPengguna(t, &repoUji{kabupaten: func([]string) (map[string]bool, error) { return nil, galatDB() }})
		h := panggilJSON(t, r, nethttp.MethodPost, "/users/undang",
			`{"nama":"B","email":"b@garda.test","peran":"MANAJER_DATA","lembaga":"X","kabupaten_kode":"1171"}`)
		assert.Equal(t, nethttp.StatusInternalServerError, h.Code)
		tanpaBocor(t, h)
	})
}

func TestUndangPengguna_BerhasilTanpaSMTPTidakMembocorkanTokenAtauHash(t *testing.T) {
	var tersimpan domain.UndanganBaru
	r, rec := routerPengguna(t, &repoUji{
		kabupaten: func([]string) (map[string]bool, error) { return map[string]bool{"1171": true}, nil },
		buatUndangan: func(b []domain.UndanganBaru) error {
			require.Len(t, b, 1)
			tersimpan = b[0]
			return nil
		},
	})

	h := panggilJSON(t, r, nethttp.MethodPost, "/users/undang",
		`{"nama":" Budi ","email":" Budi@Garda.Test ","peran":"ops_koordinator","lembaga":"BPPSDMP","kabupaten_kode":"1171"}`)

	require.Equal(t, nethttp.StatusCreated, h.Code, h.Body.String())
	d := h.data(t)
	assert.Equal(t, false, d["email_terkirim"], "SMTP belum dikonfigurasi: akun dan undangan tetap ada")
	assert.NotEmpty(t, d["berlaku_sampai"])
	user := d["user"].(map[string]any)
	assert.Equal(t, "budi@garda.test", user["email"], "email dirapikan ke huruf kecil")
	assert.Equal(t, "Budi", user["nama"])
	assert.Equal(t, "MENUNGGU", user["status"])
	assert.True(t, strings.HasPrefix(user["id"].(string), "USR-"), "id dibuat server")
	assert.Equal(t, "OPS_KOORDINATOR", user["peran"], "peran dinormalkan ke huruf besar")

	// Yang tersimpan: akun tanpa kata sandi dan hanya hash token (token mentah tidak punya jalan keluar).
	assert.Nil(t, tersimpan.User.PasswordHash)
	assert.Len(t, tersimpan.Undangan.TokenHash, 64)
	assert.NotContains(t, h.Body.String(), tersimpan.Undangan.TokenHash)
	assert.NotContains(t, h.Body.String(), "token")

	e, ok := rec.Last()
	require.True(t, ok)
	assert.Equal(t, "UNDANG_PENGGUNA", e.Aksi)
	assert.NotContains(t, fmt.Sprint(e), tersimpan.Undangan.TokenHash, "audit tanpa hash token")
}

func TestUndangPengguna_EmailTerkirimLewatSMTPPalsu(t *testing.T) {
	smtp := emailtest.Start(t)
	r, _ := routerPengguna(t, &repoUji{
		pengirim:     email.New(smtp.Config("https://garda.test")),
		buatUndangan: func([]domain.UndanganBaru) error { return nil },
	})

	h := panggilJSON(t, r, nethttp.MethodPost, "/users/undang", badanUndang)

	require.Equal(t, nethttp.StatusCreated, h.Code, h.Body.String())
	assert.Equal(t, true, h.data(t)["email_terkirim"])
	pesan := smtp.Tunggu(t, "budi@garda.test", 1)[0]
	token := pesan.Token()
	require.NotEmpty(t, token)
	assert.NotContains(t, h.Body.String(), token, "token hanya ada di email, tidak di jawaban API")
}

// ── POST /users/:id/undang-ulang ──────────────────────────────────────

func TestUndangUlang_Handler(t *testing.T) {
	menunggu := penggunaAktif("u-1")
	menunggu.Status = domain.StatusMenunggu
	cases := []struct {
		nama   string
		repo   *repoUji
		status int
	}{
		{"pengguna tidak ada", &repoUji{gantiUndangan: func(string) (*domain.Pengguna, error) { return nil, fmt.Errorf("x: %w", gorm.ErrRecordNotFound) }}, 404},
		{"bukan akun menunggu", &repoUji{gantiUndangan: func(string) (*domain.Pengguna, error) { return nil, fmt.Errorf("x: %w", domain.ErrBukanMenunggu) }}, 409},
		{"galat basis data", &repoUji{gantiUndangan: func(string) (*domain.Pengguna, error) { return nil, galatDB() }}, 500},
		{"berhasil", &repoUji{gantiUndangan: func(string) (*domain.Pengguna, error) { return menunggu, nil }}, 200},
	}
	for _, tc := range cases {
		t.Run(tc.nama, func(t *testing.T) {
			r, rec := routerPengguna(t, tc.repo)
			h := panggilJSON(t, r, nethttp.MethodPost, "/users/u-1/undang-ulang", "")
			assert.Equal(t, tc.status, h.Code, h.Body.String())
			tanpaBocor(t, h)
			if tc.status == 200 {
				d := h.data(t)
				assert.Equal(t, false, d["email_terkirim"])
				assert.Equal(t, "MENUNGGU", d["user"].(map[string]any)["status"])
				e, ok := rec.Last()
				require.True(t, ok)
				assert.Equal(t, "UNDANG_ULANG_PENGGUNA", e.Aksi)
			} else {
				assert.Empty(t, rec.Entries())
			}
		})
	}
}

// ── POST /auth/aktivasi ───────────────────────────────────────────────

func TestAktivasiAkun_Handler(t *testing.T) {
	const sandi = "Baru#Sandi123"
	hidup := func(string) (*domain.Undangan, error) {
		return &domain.Undangan{UserID: "u-1", ExpiresAt: time.Now().Add(time.Hour)}, nil
	}
	cases := []struct {
		nama   string
		repo   *repoUji
		badan  string
		status int
		muat   string
	}{
		{"JSON rusak", &repoUji{}, `{"token":`, 400, ""},
		{"token hilang", &repoUji{}, `{"password":"` + sandi + `"}`, 400, ""},
		{"kata sandi hilang", &repoUji{}, `{"token":"abc"}`, 400, ""},
		{"kata sandi pendek", &repoUji{}, `{"token":"abc","password":"abc"}`, 400, ""},
		{"token kepanjangan", &repoUji{}, `{"token":"` + strings.Repeat("a", 513) + `","password":"` + sandi + `"}`, 400, ""},
		{"token tidak ada", &repoUji{getUndangan: func(string) (*domain.Undangan, error) { return nil, fmt.Errorf("x: %w", gorm.ErrRecordNotFound) }},
			`{"token":"abc","password":"` + sandi + `"}`, 401, "tidak valid"},
		{"token sudah dipakai", &repoUji{getUndangan: func(string) (*domain.Undangan, error) {
			dipakai := time.Now()
			return &domain.Undangan{ExpiresAt: time.Now().Add(time.Hour), UsedAt: &dipakai}, nil
		}}, `{"token":"abc","password":"` + sandi + `"}`, 401, "tidak valid"},
		{"token dicabut karena undangan baru", &repoUji{getUndangan: func(string) (*domain.Undangan, error) {
			dicabut := time.Now()
			return &domain.Undangan{ExpiresAt: time.Now().Add(time.Hour), DicabutPada: &dicabut}, nil
		}}, `{"token":"abc","password":"` + sandi + `"}`, 401, "tidak valid"},
		{"token kedaluwarsa", &repoUji{getUndangan: func(string) (*domain.Undangan, error) {
			return &domain.Undangan{ExpiresAt: time.Now().Add(-time.Minute)}, nil
		}}, `{"token":"abc","password":"` + sandi + `"}`, 401, "kedaluwarsa"},
		{"kata sandi lemah tidak menghabiskan tautan", &repoUji{getUndangan: hidup},
			`{"token":"abc","password":"12345678"}`, 400, ""},
		{"kata sandi lebih dari batas bcrypt", &repoUji{getUndangan: hidup},
			`{"token":"abc","password":"` + strings.Repeat("Aa1#", 19) + `"}`, 400, "72"},
		{"akun berubah di sela aktivasi", &repoUji{getUndangan: hidup, aktifkan: func() (*domain.Pengguna, error) { return nil, nil }},
			`{"token":"abc","password":"` + sandi + `"}`, 401, "tidak valid"},
		{"galat membaca undangan", &repoUji{getUndangan: func(string) (*domain.Undangan, error) { return nil, galatDB() }},
			`{"token":"abc","password":"` + sandi + `"}`, 500, ""},
		{"galat mengaktifkan", &repoUji{getUndangan: hidup, aktifkan: func() (*domain.Pengguna, error) { return nil, galatDB() }},
			`{"token":"abc","password":"` + sandi + `"}`, 500, ""},
	}
	for _, tc := range cases {
		t.Run(tc.nama, func(t *testing.T) {
			r, rec := routerPengguna(t, tc.repo)
			h := panggilJSON(t, r, nethttp.MethodPost, "/auth/aktivasi", tc.badan)
			assert.Equal(t, tc.status, h.Code, h.Body.String())
			if tc.muat != "" {
				assert.Contains(t, h.pesan(), tc.muat)
			}
			tanpaBocor(t, h)
			assert.NotContains(t, h.Body.String(), sandi)
			assert.Empty(t, rec.Entries(), "aktivasi yang gagal tidak diaudit sebagai berhasil")
		})
	}

	t.Run("berhasil tanpa menerbitkan sesi", func(t *testing.T) {
		aktif := penggunaAktif("u-1")
		r, rec := routerPengguna(t, &repoUji{getUndangan: hidup, aktifkan: func() (*domain.Pengguna, error) { return aktif, nil }})
		h := panggilJSON(t, r, nethttp.MethodPost, "/auth/aktivasi", `{"token":"abc","password":"`+sandi+`"}`)
		require.Equal(t, nethttp.StatusOK, h.Code, h.Body.String())
		assert.NotContains(t, h.Body.String(), "access_token")
		assert.NotContains(t, h.Body.String(), "refresh_token")
		e, ok := rec.Last()
		require.True(t, ok)
		assert.Equal(t, "AKTIVASI_AKUN", e.Aksi)
		assert.Equal(t, "u-1", e.AktorID)
		assert.NotContains(t, fmt.Sprint(e), sandi)
		assert.NotContains(t, fmt.Sprint(e), "abc", "audit tanpa token")
	})
}

// ── POST /users/impor ─────────────────────────────────────────────────

const csvSah = "nama,email,peran,lembaga,kabupaten_kode\nAni,ani@garda.test,OPS_MAHASISWA,BPPSDMP,\nBudi,budi@garda.test,MANAJER_DATA,BPS,\n"

func badanImpor(csv string, dryRun any) string {
	return js(map[string]any{"csv": csv, "dry_run": dryRun})
}

func TestImporPengguna_ValidasiBinding(t *testing.T) {
	r, _ := routerPengguna(t, &repoUji{})
	for nama, badan := range map[string]string{
		"JSON rusak":              `{"csv":`,
		"csv hilang":              `{"dry_run":true}`,
		"csv kosong":              `{"csv":"","dry_run":true}`,
		"dry_run wajib eksplisit": js(map[string]any{"csv": csvSah}),
		"dry_run bukan boolean":   `{"csv":"a","dry_run":"true"}`,
		"dry_run null":            `{"csv":"a","dry_run":null}`,
	} {
		h := panggilJSON(t, r, nethttp.MethodPost, "/users/impor", badan)
		assert.Equal(t, nethttp.StatusBadRequest, h.Code, nama)
		assert.Equal(t, "400", h.kode(), nama)
	}
}

func TestImporPengguna_BadanTerlaluBesarDijawab413(t *testing.T) {
	r, _ := routerPengguna(t, &repoUji{})
	besar := strings.Repeat("a", usecase.ImporMaksByte*2+8192)

	h := panggilJSON(t, r, nethttp.MethodPost, "/users/impor", badanImpor(besar, true))

	assert.Equal(t, nethttp.StatusRequestEntityTooLarge, h.Code)
	assert.Contains(t, h.pesan(), "terlalu besar")
}

func TestImporPengguna_TeksMelebihiBatasDijawab413DenganBatasKB(t *testing.T) {
	r, _ := routerPengguna(t, &repoUji{})
	// Di bawah batas badan JSON tetapi di atas batas teks CSV: ditolak usecase dengan pesan batas dalam KB.
	csv := "nama,email,peran,lembaga\n" + strings.Repeat("a", usecase.ImporMaksByte+10)

	h := panggilJSON(t, r, nethttp.MethodPost, "/users/impor", badanImpor(csv, true))

	assert.Equal(t, nethttp.StatusRequestEntityTooLarge, h.Code)
	assert.Contains(t, h.pesan(), "512 KB")
}

func TestImporPengguna_GalatBerkasDijawab400(t *testing.T) {
	r, _ := routerPengguna(t, &repoUji{})
	for nama, csv := range map[string]string{
		"header tanpa kolom wajib": "nama,email\nAni,ani@garda.test\n",
		"kolom asing":              "nama,email,peran,lembaga,password\nAni,ani@garda.test,OPS_MAHASISWA,X,rahasia\n",
		"hanya header":             "nama,email,peran,lembaga\n",
		"bukan UTF-8":              "nama,email,peran,lembaga\nAni\xff,ani@garda.test,OPS_MAHASISWA,X\n",
	} {
		h := panggilJSON(t, r, nethttp.MethodPost, "/users/impor", badanImpor(csv, true))
		assert.Equal(t, nethttp.StatusBadRequest, h.Code, nama+": "+h.Body.String())
	}
}

func TestImporPengguna_DryRunMelaporkanPerBarisTanpaMenulis(t *testing.T) {
	csv := "nama,email,peran,lembaga,kabupaten_kode\n" +
		"Ani,ani@garda.test,OPS_MAHASISWA,BPPSDMP,\n" +
		"Salah,salah,PERAN_X,,12\n" +
		"Ani lagi,ANI@garda.test,OPS_MAHASISWA,BPPSDMP,\n"
	r, rec := routerPengguna(t, &repoUji{}) // BuatUndangan tidak diisi: tulis akan menggagalkan tes

	h := panggilJSON(t, r, nethttp.MethodPost, "/users/impor", badanImpor(csv, true))

	require.Equal(t, nethttp.StatusOK, h.Code, h.Body.String())
	d := h.data(t)
	assert.Equal(t, true, d["dry_run"])
	assert.EqualValues(t, 3, d["total"])
	assert.EqualValues(t, 1, d["valid"])
	assert.EqualValues(t, 2, d["galat"])
	assert.EqualValues(t, 0, d["dibuat"])
	assert.Equal(t, false, d["email_aktif"])
	baris := d["baris"].([]any)
	require.Len(t, baris, 3)
	assert.Equal(t, "valid", baris[0].(map[string]any)["status"])
	assert.Equal(t, []any{}, baris[0].(map[string]any)["galat"], "baris sah: larik galat kosong, bukan null")
	assert.Equal(t, "galat", baris[1].(map[string]any)["status"])
	assert.NotEmpty(t, baris[1].(map[string]any)["galat"])
	assert.Contains(t, fmt.Sprint(baris[2].(map[string]any)["galat"]), "baris 2")
	assert.EqualValues(t, 2, baris[0].(map[string]any)["baris"], "nomor baris = nomor di spreadsheet (header 1)")
	assert.Empty(t, rec.Entries(), "mode uji tidak diaudit")
}

func TestImporPengguna_KomitMembuatAkunMenungguDanMengaudit(t *testing.T) {
	var ditulis []domain.UndanganBaru
	r, rec := routerPengguna(t, &repoUji{buatUndangan: func(b []domain.UndanganBaru) error { ditulis = b; return nil }})

	h := panggilJSON(t, r, nethttp.MethodPost, "/users/impor", badanImpor(csvSah, false))

	require.Equal(t, nethttp.StatusOK, h.Code, h.Body.String())
	d := h.data(t)
	assert.Equal(t, false, d["dry_run"])
	assert.EqualValues(t, 2, d["dibuat"])
	for _, b := range d["baris"].([]any) {
		baris := b.(map[string]any)
		assert.Equal(t, "dibuat", baris["status"])
		assert.True(t, strings.HasPrefix(baris["id"].(string), "USR-"))
	}
	require.Len(t, ditulis, 2, "semua baris sah ditulis dalam satu pemanggilan (satu transaksi)")
	for _, b := range ditulis {
		assert.Equal(t, domain.StatusMenunggu, b.User.Status)
		assert.Nil(t, b.User.PasswordHash)
		assert.NotContains(t, h.Body.String(), b.Undangan.TokenHash)
	}
	aksi := []string{}
	for _, e := range rec.Entries() {
		aksi = append(aksi, e.Aksi)
	}
	assert.Equal(t, []string{"UNDANG_PENGGUNA", "UNDANG_PENGGUNA", "IMPOR_PENGGUNA"}, aksi, "satu entri per akun (riwayat peran) dan satu ringkasan")
	assert.Equal(t, "MANAJER_DATA", snap(rec.Entries()[1].Sesudah)["peran"], "riwayat peran dapat membaca peran awal dari impor")
}

func TestImporPengguna_KomitTanpaBarisSahTidakMenulisApaPun(t *testing.T) {
	r, rec := routerPengguna(t, &repoUji{}) // BuatUndangan tidak diisi
	csv := "nama,email,peran,lembaga\n,bukan,X,\n"

	h := panggilJSON(t, r, nethttp.MethodPost, "/users/impor", badanImpor(csv, false))

	require.Equal(t, nethttp.StatusOK, h.Code, h.Body.String())
	assert.EqualValues(t, 0, h.data(t)["dibuat"])
	assert.Empty(t, rec.Entries())
}

func TestImporPengguna_PemetaanGalatKomit(t *testing.T) {
	t.Run("email keburu didaftarkan pihak lain", func(t *testing.T) {
		r, rec := routerPengguna(t, &repoUji{buatUndangan: func([]domain.UndanganBaru) error { return fmt.Errorf("x: %w", gorm.ErrDuplicatedKey) }})
		h := panggilJSON(t, r, nethttp.MethodPost, "/users/impor", badanImpor(csvSah, false))
		assert.Equal(t, nethttp.StatusConflict, h.Code)
		assert.Contains(t, h.pesan(), "Tidak ada akun yang dibuat")
		assert.Empty(t, rec.Entries())
	})
	t.Run("galat basis data", func(t *testing.T) {
		r, _ := routerPengguna(t, &repoUji{buatUndangan: func([]domain.UndanganBaru) error { return galatDB() }})
		h := panggilJSON(t, r, nethttp.MethodPost, "/users/impor", badanImpor(csvSah, false))
		assert.Equal(t, nethttp.StatusInternalServerError, h.Code)
		tanpaBocor(t, h)
	})
	t.Run("galat memeriksa email terdaftar", func(t *testing.T) {
		r, _ := routerPengguna(t, &repoUji{emailAda: func([]string) (map[string]bool, error) { return nil, galatDB() }})
		h := panggilJSON(t, r, nethttp.MethodPost, "/users/impor", badanImpor(csvSah, true))
		assert.Equal(t, nethttp.StatusInternalServerError, h.Code)
		tanpaBocor(t, h)
	})
	t.Run("email yang sudah terdaftar dilaporkan per baris", func(t *testing.T) {
		r, _ := routerPengguna(t, &repoUji{emailAda: func([]string) (map[string]bool, error) { return map[string]bool{"ani@garda.test": true}, nil }})
		h := panggilJSON(t, r, nethttp.MethodPost, "/users/impor", badanImpor(csvSah, true))
		require.Equal(t, nethttp.StatusOK, h.Code, h.Body.String())
		baris := h.data(t)["baris"].([]any)
		assert.Contains(t, fmt.Sprint(baris[0].(map[string]any)["galat"]), "sudah terdaftar")
		assert.Equal(t, "valid", baris[1].(map[string]any)["status"])
	})
}

// ── GET /users/ekspor ─────────────────────────────────────────────────

func TestEksporPengguna_FilterTidakValidDijawab400(t *testing.T) {
	r, _ := routerPengguna(t, &repoUji{}) // ListUsersUntukEkspor tidak boleh dipanggil
	for nama, q := range map[string]string{
		"mfa bukan boolean":      "mfa=1",
		"terkunci bukan boolean": "terkunci=ya",
		"status tak dikenal":     "status=aktif",
		"lapis tak dikenal":      "lapis=OPS",
		"peran tak dikenal":      "peran=SUPERADMIN",
		"search kepanjangan":     "search=" + strings.Repeat("a", 101),
		"ids hanya pemisah":      "ids=,,%20,",
		"ids terlalu banyak":     "ids=" + strings.Join(seqID(usecase.EksporMaksID+1), ","),
	} {
		h := panggilJSON(t, r, nethttp.MethodGet, "/users/ekspor?"+q, "")
		assert.Equal(t, nethttp.StatusBadRequest, h.Code, nama+": "+h.Body.String())
		assert.Equal(t, "400", h.kode(), nama)
	}
}

func seqID(n int) []string {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("u-%d", i)
	}
	return ids
}

func TestEksporPengguna_MeneruskanSemuaFilterDaftarDanIds(t *testing.T) {
	var diterima domain.FilterPengguna
	r, _ := routerPengguna(t, &repoUji{ekspor: func(f domain.FilterPengguna, _ int) ([]domain.Pengguna, bool, error) {
		diterima = f
		return nil, false, nil
	}})

	h := panggilJSON(t, r, nethttp.MethodGet,
		"/users/ekspor?search=+ani+&peran=ADMIN_SISTEM&status=AKTIF&lapis=SISTEM&mfa=true&terkunci=false&ids=u-1,+u-2+,,u-1", "")

	require.Equal(t, nethttp.StatusOK, h.Code, h.Body.String())
	assert.Equal(t, "ani", diterima.Cari)
	assert.Equal(t, "ADMIN_SISTEM", diterima.Peran)
	assert.Equal(t, "AKTIF", diterima.Status)
	assert.Equal(t, "SISTEM", diterima.Lapis)
	require.NotNil(t, diterima.MFA)
	assert.True(t, *diterima.MFA)
	require.NotNil(t, diterima.Terkunci)
	assert.False(t, *diterima.Terkunci)
	assert.Equal(t, []string{"u-1", "u-2"}, diterima.IDs, "id dirapikan dan duplikat dibuang")
}

func TestEksporPengguna_BerhasilHeaderIsiDanAudit(t *testing.T) {
	sandi, rahasia := "hash-rahasia", "v1:rahasia-mfa"
	u := penggunaAktif("u-1")
	u.Nama = "=HYPERLINK(\"http://x\")"
	u.PasswordHash, u.MfaSecret = &sandi, &rahasia
	r, rec := routerPengguna(t, &repoUji{ekspor: func(domain.FilterPengguna, int) ([]domain.Pengguna, bool, error) {
		return []domain.Pengguna{*u}, true, nil
	}})

	h := panggilJSON(t, r, nethttp.MethodGet, "/users/ekspor?search=ani", "")

	require.Equal(t, nethttp.StatusOK, h.Code, h.Body.String())
	assert.Equal(t, "text/csv; charset=utf-8", h.Header().Get("Content-Type"))
	assert.Equal(t, "no-store", h.Header().Get("Cache-Control"))
	assert.Equal(t, "1", h.Header().Get("X-Export-Rows"))
	assert.Equal(t, "true", h.Header().Get("X-Export-Truncated"))
	assert.Regexp(t, `^attachment; filename="pengguna-\d{4}-\d{2}-\d{2}\.csv"$`, h.Header().Get("Content-Disposition"))
	body := h.Body.String()
	assert.True(t, strings.HasPrefix(body, "\xef\xbb\xbf"), "UTF-8 BOM supaya terbuka benar di Excel")
	assert.Contains(t, body, "id;nama;email;peran;lapis;lembaga;kabupaten_kode;status;mfa_aktif;terakhir_masuk;dibuat_pada")
	assert.Contains(t, body, "'=HYPERLINK", "sel berawalan rumus dinetralkan")
	assert.NotContains(t, body, sandi)
	assert.NotContains(t, body, rahasia)

	e, ok := rec.Last()
	require.True(t, ok)
	assert.Equal(t, "EKSPOR_DATA", e.Aksi)
	assert.Equal(t, "ani", e.Metadata["search"])
	assert.EqualValues(t, 1, e.Metadata["baris"])
	assert.Equal(t, true, e.Metadata["terpotong"])
}

func TestEksporPengguna_TanpaTerpotongTidakMenyetelHeaderTerpotong(t *testing.T) {
	r, _ := routerPengguna(t, &repoUji{ekspor: func(domain.FilterPengguna, int) ([]domain.Pengguna, bool, error) {
		return []domain.Pengguna{*penggunaAktif("u-1")}, false, nil
	}})
	h := panggilJSON(t, r, nethttp.MethodGet, "/users/ekspor", "")
	require.Equal(t, nethttp.StatusOK, h.Code)
	assert.Empty(t, h.Header().Get("X-Export-Truncated"))
}

func TestEksporPengguna_AuditGagalTidakMenyerahkanData(t *testing.T) {
	r, rec := routerPengguna(t, &repoUji{ekspor: func(domain.FilterPengguna, int) ([]domain.Pengguna, bool, error) {
		return []domain.Pengguna{*penggunaAktif("u-1")}, false, nil
	}})
	rec.Err = errors.New("tabel audit rusak")

	h := panggilJSON(t, r, nethttp.MethodGet, "/users/ekspor", "")

	assert.Equal(t, nethttp.StatusInternalServerError, h.Code)
	assert.NotContains(t, h.Body.String(), "u-1@garda.test", "data pribadi tidak keluar bila jejaknya tak tercatat")
	assert.Empty(t, h.Header().Get("Content-Disposition"))
}

func TestEksporPengguna_GalatBacaDijawab500(t *testing.T) {
	r, rec := routerPengguna(t, &repoUji{ekspor: func(domain.FilterPengguna, int) ([]domain.Pengguna, bool, error) {
		return nil, false, galatDB()
	}})

	h := panggilJSON(t, r, nethttp.MethodGet, "/users/ekspor", "")

	assert.Equal(t, nethttp.StatusInternalServerError, h.Code)
	tanpaBocor(t, h)
	assert.Empty(t, rec.Entries())
}
