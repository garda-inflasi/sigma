package http

import (
	"errors"
	"fmt"
	nethttp "net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/internal/auth/usecase"
	"github.com/garda-inflasi/api/pkg/idx"
)

// Tes pemetaan galat usecase → status HTTP untuk handler pengguna (buat, daftar, detail, ubah, hapus, reset
// kata sandi, ganti peran, aksi massal, buka kunci, paksa keluar, katalog peran/izin). Repositori ditiru
// (repoUji): yang diuji adalah apa yang dilihat klien, bukan SQL.

const rahasiaInternal = "pq: rahasia-internal-basis-data"

func galatDB() error { return errors.New(rahasiaInternal) }

// tidakAdaPengguna meniru "tidak ada" dari repositori (terbungkus, seperti di repositori Postgres).
func tidakAdaPengguna(string) (*domain.Pengguna, error) {
	return nil, fmt.Errorf("get user: %w", gorm.ErrRecordNotFound)
}

func tanpaBocor(t *testing.T, h hasil) {
	t.Helper()
	assert.NotContains(t, h.Body.String(), "rahasia-internal", "galat internal tidak boleh sampai ke klien")
}

// ── POST /users ───────────────────────────────────────────────────────

func TestCreateUser_ValidasiBindingDijawab400(t *testing.T) {
	r, _ := routerPengguna(t, &repoUji{})
	valid := map[string]any{
		"id": uid("u-baru"), "nama": "Baru", "email": "baru@garda.test", "password": "Kata#Sandi1",
		"peran": "OPS_MAHASISWA", "lapis": "OPERASIONAL", "lembaga": "BPPSDMP",
	}
	ubah := func(k string, v any) string {
		m := map[string]any{}
		for kk, vv := range valid {
			m[kk] = vv
		}
		if v == nil {
			delete(m, k)
		} else {
			m[k] = v
		}
		return js(m)
	}
	cases := map[string]string{
		"JSON rusak":      `{"id":`,
		"badan kosong":    ``,
		"nama hilang":     ubah("nama", nil),
		"email tak valid": ubah("email", "bukan-email"),
		"sandi pendek":    ubah("password", "pendek"),
		"peran hilang":    ubah("peran", nil),
		"lapis hilang":    ubah("lapis", nil),
		"lembaga hilang":  ubah("lembaga", nil),
	}
	for nama, badan := range cases {
		t.Run(nama, func(t *testing.T) {
			h := panggilJSON(t, r, nethttp.MethodPost, "/users", badan)
			assert.Equal(t, nethttp.StatusBadRequest, h.Code, h.Body.String())
			assert.Equal(t, "400", h.kode())
		})
	}
}

func TestCreateUser_PemetaanGalat(t *testing.T) {
	cases := []struct {
		nama   string
		repo   *repoUji
		status int
	}{
		{"email sudah terdaftar", &repoUji{byEmail: func(string) (*domain.Pengguna, error) { return penggunaAktif("lain"), nil }}, nethttp.StatusConflict},
		{"galat menyimpan", &repoUji{
			byEmail: func(string) (*domain.Pengguna, error) { return nil, gorm.ErrRecordNotFound },
			buat:    func(*domain.Pengguna) error { return galatDB() },
		}, nethttp.StatusInternalServerError},
		{"berhasil", &repoUji{
			byEmail: func(string) (*domain.Pengguna, error) { return nil, gorm.ErrRecordNotFound },
			buat:    func(*domain.Pengguna) error { return nil },
		}, nethttp.StatusCreated},
	}
	body := js(map[string]any{
		"id": uid("u-baru"), "nama": "Baru", "email": "baru@garda.test", "password": "Kata#Sandi1",
		"peran": "OPS_MAHASISWA", "lapis": "OPERASIONAL", "lembaga": "BPPSDMP",
	})
	for _, tc := range cases {
		t.Run(tc.nama, func(t *testing.T) {
			r, rec := routerPengguna(t, tc.repo)
			h := panggilJSON(t, r, nethttp.MethodPost, "/users", body)
			assert.Equal(t, tc.status, h.Code, h.Body.String())
			tanpaBocor(t, h)
			if tc.status == nethttp.StatusCreated {
				e, ok := rec.Last()
				require.True(t, ok)
				assert.Equal(t, "CREATE_USER", e.Aksi)
			} else {
				assert.Empty(t, rec.Entries(), "pembuatan yang gagal tidak diaudit sebagai berhasil")
			}
		})
	}
}

// ID pengguna adalah UUID (000110). Klien tidak wajib mengirim id: server membuat UUID acak. Bila dikirim, id
// harus UUID yang sah (dinormalkan ke huruf kecil), selain itu 400; id yang sudah dipakai 409.
func TestCreateUser_IDOpsionalDanHarusUUID(t *testing.T) {
	badan := func(id any) string {
		m := map[string]any{
			"nama": "Baru", "email": "baru@garda.test", "password": "Kata#Sandi1",
			"peran": "OPS_MAHASISWA", "lapis": "OPERASIONAL", "lembaga": "BPPSDMP",
		}
		if id != nil {
			m["id"] = id
		}
		return js(m)
	}
	baru := func(simpan *string) *repoUji {
		return &repoUji{
			byEmail: func(string) (*domain.Pengguna, error) { return nil, gorm.ErrRecordNotFound },
			buat:    func(p *domain.Pengguna) error { *simpan = p.ID; return nil },
		}
	}

	t.Run("tanpa id: server membuat UUID acak", func(t *testing.T) {
		var disimpan string
		r, rec := routerPengguna(t, baru(&disimpan))
		h := panggilJSON(t, r, nethttp.MethodPost, "/users", badan(nil))
		require.Equal(t, nethttp.StatusCreated, h.Code, h.Body.String())
		assert.True(t, idx.ValidUUID(disimpan), "id dibuat server: %q", disimpan)
		e, ok := rec.Last()
		require.True(t, ok)
		assert.Equal(t, disimpan, e.EntitasID, "audit mencatat id yang dibuat")
	})
	t.Run("id kosong sama dengan tanpa id", func(t *testing.T) {
		var disimpan string
		r, _ := routerPengguna(t, baru(&disimpan))
		h := panggilJSON(t, r, nethttp.MethodPost, "/users", badan(""))
		require.Equal(t, nethttp.StatusCreated, h.Code, h.Body.String())
		assert.True(t, idx.ValidUUID(disimpan))
	})
	t.Run("id UUID sah dipakai apa adanya, huruf kecil", func(t *testing.T) {
		var disimpan string
		r, _ := routerPengguna(t, baru(&disimpan))
		h := panggilJSON(t, r, nethttp.MethodPost, "/users", badan(strings.ToUpper(uid("u-baru"))))
		require.Equal(t, nethttp.StatusCreated, h.Code, h.Body.String())
		assert.Equal(t, uid("u-baru"), disimpan)
	})
	t.Run("id bukan UUID: 400 tanpa menyentuh basis data", func(t *testing.T) {
		for _, id := range []string{"u-baru", "USR-2024-0001", "superadmin", "123", "urn:uuid:" + uid("u-baru"), "{" + uid("u-baru") + "}", " " + uid("u-baru")} {
			r, rec := routerPengguna(t, &repoUji{})
			h := panggilJSON(t, r, nethttp.MethodPost, "/users", badan(id))
			assert.Equal(t, nethttp.StatusBadRequest, h.Code, "%q: %s", id, h.Body.String())
			assert.Equal(t, "400", h.kode())
			assert.Empty(t, rec.Entries())
		}
	})
	t.Run("id sudah dipakai: 409", func(t *testing.T) {
		r, rec := routerPengguna(t, &repoUji{
			byEmail:  func(string) (*domain.Pengguna, error) { return nil, gorm.ErrRecordNotFound },
			buat:     func(*domain.Pengguna) error { return fmt.Errorf("create: %w", gorm.ErrDuplicatedKey) },
			pengguna: ada,
		})
		h := panggilJSON(t, r, nethttp.MethodPost, "/users", badan(uid("u-baru")))
		assert.Equal(t, nethttp.StatusConflict, h.Code, h.Body.String())
		assert.Contains(t, h.Body.String(), "ID pengguna")
		assert.Empty(t, rec.Entries())
	})
}

// Semua rute /users/:id... memperlakukan id yang bukan UUID sebagai pengguna yang tidak ada (404), tanpa
// menyentuh repositori (repoUji gagal bila dipanggil tanpa diharapkan) dan tanpa menulis audit.
func TestRuteUserID_IDSalahBentukDijawab404TanpaKeBasisData(t *testing.T) {
	rute := []struct{ metode, pola, badan string }{
		{nethttp.MethodGet, "/users/%s", ""},
		{nethttp.MethodPatch, "/users/%s", `{"nama":"Baru"}`},
		{nethttp.MethodDelete, "/users/%s", ""},
		{nethttp.MethodPost, "/users/%s/reset-password", `{"new_password":"Kata#Sandi1"}`},
		{nethttp.MethodPost, "/users/%s/buka-kunci", ""},
		{nethttp.MethodPost, "/users/%s/revoke-sessions", ""},
		{nethttp.MethodPut, "/users/%s/roles", `{"peran":"OPS_MAHASISWA"}`},
		{nethttp.MethodPost, "/users/%s/reset-mfa", ""},
		{nethttp.MethodGet, "/users/%s/riwayat-peran", ""},
		{nethttp.MethodGet, "/users/%s/permissions", ""},
		{nethttp.MethodPost, "/users/%s/permissions", `{"izin":"harga.lihat","alasan":"uji"}`},
		{nethttp.MethodDelete, "/users/%s/permissions/harga.lihat", ""},
		{nethttp.MethodPost, "/users/%s/undang-ulang", ""},
	}
	for _, bentuk := range []string{"bukan-uuid", "superadmin", "USR-2024-0001", "123", "urn:uuid:" + uid("u-1"), "%7B" + uid("u-1") + "%7D"} {
		for _, rt := range rute {
			r, rec := routerPengguna(t, &repoUji{})
			path := fmt.Sprintf(rt.pola, bentuk)
			h := panggilJSON(t, r, rt.metode, path, rt.badan)
			assert.Equal(t, nethttp.StatusNotFound, h.Code, "%s %s: %s", rt.metode, path, h.Body.String())
			assert.Empty(t, rec.Entries(), "%s %s", rt.metode, path)
		}
	}
}

// Aksi massal menolak id yang bukan UUID (400) alih-alih membiarkannya jadi galat kolom uuid (500), dan merapikan
// id (huruf kecil, ganda dibuang) sebelum sampai ke repositori.
func TestBulkUser_IDHarusUUIDDanDirapikan(t *testing.T) {
	var diterima []string
	r, _ := routerPengguna(t, &repoUji{
		bulk: func(ids []string, _ domain.BulkUserAction) (int64, error) {
			diterima = ids
			return int64(len(ids)), nil
		},
	})

	h := panggilJSON(t, r, nethttp.MethodPost, "/users/bulk", js(map[string]any{"action": "nonaktifkan", "ids": []string{uid("u-1"), "u-2"}}))
	assert.Equal(t, nethttp.StatusBadRequest, h.Code, h.Body.String())
	assert.Nil(t, diterima, "tidak ada yang ditulis bila satu id salah")

	h = panggilJSON(t, r, nethttp.MethodPost, "/users/bulk",
		js(map[string]any{"action": "nonaktifkan", "ids": []string{strings.ToUpper(uid("u-1")), uid("u-2"), uid("u-1")}}))
	require.Equal(t, nethttp.StatusOK, h.Code, h.Body.String())
	assert.Equal(t, []string{uid("u-1"), uid("u-2")}, diterima)
}

// ── GET /users ────────────────────────────────────────────────────────

func TestListUsers_MenyertakanAkhirKunciHanyaUntukAkunTerkunci(t *testing.T) {
	terkunci := penggunaAktif(uid("u-kunci"))
	bebas := penggunaAktif(uid("u-bebas"))
	sampai := time.Now().Add(10 * time.Minute).UTC().Truncate(time.Second)
	repo := &repoUji{
		daftar: func(domain.DaftarPenggunaQuery) ([]domain.Pengguna, int64, error) {
			return []domain.Pengguna{*terkunci, *bebas}, 2, nil
		},
		kunciAktif: func(hash []string) (map[string]time.Time, error) {
			require.Len(t, hash, 2)
			return map[string]time.Time{usecase.HashEmailKunci(terkunci.Email): sampai}, nil
		},
	}
	r, _ := routerPengguna(t, repo)

	h := panggilJSON(t, r, nethttp.MethodGet, "/users", "")

	require.Equal(t, nethttp.StatusOK, h.Code, h.Body.String())
	items := h.data(t)["items"].([]any)
	require.Len(t, items, 2)
	assert.Equal(t, sampai.Format(time.RFC3339), items[0].(map[string]any)["terkunci_sampai"])
	assert.NotContains(t, items[1], "terkunci_sampai")
	assert.NotContains(t, h.Body.String(), "password_hash")
}

func TestListUsers_GalatBacaKunciDijawab500(t *testing.T) {
	repo := &repoUji{
		daftar: func(domain.DaftarPenggunaQuery) ([]domain.Pengguna, int64, error) {
			return []domain.Pengguna{*penggunaAktif(uid("u-1"))}, 1, nil
		},
		kunciAktif: func([]string) (map[string]time.Time, error) { return nil, galatDB() },
	}
	r, _ := routerPengguna(t, repo)

	h := panggilJSON(t, r, nethttp.MethodGet, "/users", "")

	assert.Equal(t, nethttp.StatusInternalServerError, h.Code)
	tanpaBocor(t, h)
}

func TestListUsers_ParameterBukanAngkaDijawab400(t *testing.T) {
	r, _ := routerPengguna(t, &repoUji{})
	for _, q := range []string{"page=abc", "limit=xyz", "page=1.5", "limit=0x10"} {
		h := panggilJSON(t, r, nethttp.MethodGet, "/users?"+q, "")
		assert.Equal(t, nethttp.StatusBadRequest, h.Code, q)
	}
}

func TestListUsers_PeranDiluarKatalogDitolakUsecaseDengan400(t *testing.T) {
	// Peran divalidasi terhadap katalog di usecase (bukan hanya dto): pintunya sama dengan ekspor.
	repo := &repoUji{daftar: func(domain.DaftarPenggunaQuery) ([]domain.Pengguna, int64, error) {
		t.Fatal("repositori tidak boleh dipanggil untuk peran tak dikenal")
		return nil, 0, nil
	}}
	r, _ := routerPengguna(t, repo)

	h := panggilJSON(t, r, nethttp.MethodGet, "/users?peran=SUPERADMIN", "")

	assert.Equal(t, nethttp.StatusBadRequest, h.Code)
	assert.Contains(t, h.pesan(), "peran")
}

// ── GET /users/:id ────────────────────────────────────────────────────

func TestGetUser_PemetaanDanTanpaRahasia(t *testing.T) {
	sandi, rahasia := "hash-bcrypt-rahasia", "v1:rahasia-mfa"
	u := penggunaAktif(uid("u-1"))
	u.PasswordHash, u.MfaSecret, u.MfaAktif = &sandi, &rahasia, true
	sampai := time.Now().Add(5 * time.Minute).UTC().Truncate(time.Second)

	t.Run("berhasil dengan akhir kunci", func(t *testing.T) {
		r, _ := routerPengguna(t, &repoUji{
			pengguna: func(string) (*domain.Pengguna, error) { return u, nil },
			kunciAktif: func([]string) (map[string]time.Time, error) {
				return map[string]time.Time{usecase.HashEmailKunci(u.Email): sampai}, nil
			},
		})
		h := panggilJSON(t, r, nethttp.MethodGet, "/users/"+uid("u-1"), "")
		require.Equal(t, nethttp.StatusOK, h.Code, h.Body.String())
		d := h.data(t)
		assert.Equal(t, uid("u-1"), d["id"])
		assert.Equal(t, true, d["mfa_aktif"])
		assert.Equal(t, sampai.Format(time.RFC3339), d["terkunci_sampai"])
		assert.NotContains(t, h.Body.String(), sandi)
		assert.NotContains(t, h.Body.String(), rahasia)
	})
	t.Run("tidak ada", func(t *testing.T) {
		r, _ := routerPengguna(t, &repoUji{pengguna: tidakAdaPengguna})
		h := panggilJSON(t, r, nethttp.MethodGet, "/users/hilang", "")
		assert.Equal(t, nethttp.StatusNotFound, h.Code)
	})
	t.Run("galat basis data bukan 404", func(t *testing.T) {
		r, _ := routerPengguna(t, &repoUji{pengguna: func(string) (*domain.Pengguna, error) { return nil, galatDB() }})
		h := panggilJSON(t, r, nethttp.MethodGet, "/users/"+uid("u-1"), "")
		assert.Equal(t, nethttp.StatusInternalServerError, h.Code)
		tanpaBocor(t, h)
	})
	t.Run("galat baca kunci", func(t *testing.T) {
		r, _ := routerPengguna(t, &repoUji{
			pengguna:   ada,
			kunciAktif: func([]string) (map[string]time.Time, error) { return nil, galatDB() },
		})
		h := panggilJSON(t, r, nethttp.MethodGet, "/users/"+uid("u-1"), "")
		assert.Equal(t, nethttp.StatusInternalServerError, h.Code)
		tanpaBocor(t, h)
	})
}

// ── PATCH /users/:id ──────────────────────────────────────────────────

func TestUpdateUser_ValidasiBindingDijawab400(t *testing.T) {
	r, _ := routerPengguna(t, &repoUji{})
	for nama, badan := range map[string]string{
		"JSON rusak":         `{"nama":`,
		"email tak valid":    `{"email":"bukan-email"}`,
		"status tak dikenal": `{"status":"BEKU"}`,
		"status huruf kecil": `{"status":"aktif"}`,
		"tipe nama salah":    `{"nama":123}`,
	} {
		h := panggilJSON(t, r, nethttp.MethodPatch, "/users/"+uid("u-1"), badan)
		assert.Equal(t, nethttp.StatusBadRequest, h.Code, nama)
		assert.Equal(t, "400", h.kode(), nama)
	}
}

func TestUpdateUser_PemetaanGalat(t *testing.T) {
	menunggu := func(string) (*domain.Pengguna, error) {
		u := penggunaAktif(uid("u-1"))
		u.Status = domain.StatusMenunggu
		return u, nil
	}
	cases := []struct {
		nama       string
		repo       *repoUji
		badan      string
		status     int
		kode       string
		pesanMuat  string
		auditAksi  string
		tanpaAudit bool
	}{
		{nama: "pengguna tidak ada", repo: &repoUji{pengguna: tidakAdaPengguna}, badan: `{"nama":"X"}`, status: 404},
		// Regresi: galat basis data saat memuat pengguna bukan "pengguna tidak ada" (GetUser sudah begitu).
		{nama: "galat basis data saat memuat", repo: &repoUji{pengguna: func(string) (*domain.Pengguna, error) { return nil, galatDB() }}, badan: `{"nama":"X"}`, status: 500},
		{nama: "akun menunggu tidak boleh diaktifkan lewat PATCH", repo: &repoUji{pengguna: menunggu}, badan: `{"status":"AKTIF"}`, status: 409, pesanMuat: "undangan"},
		{nama: "akun menunggu tidak boleh dinonaktifkan lewat PATCH", repo: &repoUji{pengguna: menunggu}, badan: `{"status":"NONAKTIF"}`, status: 409},
		{nama: "status menunggu tidak boleh dipasang manual", repo: &repoUji{pengguna: ada}, badan: `{"status":"MENUNGGU"}`, status: 400, pesanMuat: "undangan"},
		{nama: "email dipakai pengguna lain", repo: &repoUji{
			pengguna: ada,
			byEmail:  func(string) (*domain.Pengguna, error) { return penggunaAktif("lain"), nil },
		}, badan: `{"email":"lain@garda.test"}`, status: 409},
		{nama: "peran tidak dikenal", repo: &repoUji{pengguna: ada}, badan: `{"peran":"SUPERADMIN"}`, status: 400, pesanMuat: "Peran"},
		{nama: "pemegang terakhir pengguna.kelola", repo: &repoUji{
			pengguna: ada,
			patch: func(string, domain.PenggunaPatch) (*domain.Pengguna, *domain.Pengguna, error) {
				return nil, nil, fmt.Errorf("patch: %w", domain.ErrPemegangTerakhir)
			},
		}, badan: `{"status":"NONAKTIF"}`, status: 409, kode: "LAST_PERMISSION_HOLDER"},
		{nama: "email kembar dari basis data", repo: &repoUji{
			pengguna: ada,
			patch: func(string, domain.PenggunaPatch) (*domain.Pengguna, *domain.Pengguna, error) {
				return nil, nil, fmt.Errorf("patch: %w", gorm.ErrDuplicatedKey)
			},
		}, badan: `{"nama":"X"}`, status: 409},
		{nama: "terhapus di sela penulisan", repo: &repoUji{
			pengguna: ada,
			patch: func(string, domain.PenggunaPatch) (*domain.Pengguna, *domain.Pengguna, error) {
				return nil, nil, fmt.Errorf("patch: %w", gorm.ErrRecordNotFound)
			},
		}, badan: `{"nama":"X"}`, status: 404},
		{nama: "galat menyimpan", repo: &repoUji{
			pengguna: ada,
			patch: func(string, domain.PenggunaPatch) (*domain.Pengguna, *domain.Pengguna, error) {
				return nil, nil, galatDB()
			},
		}, badan: `{"nama":"X"}`, status: 500},
	}
	for _, tc := range cases {
		t.Run(tc.nama, func(t *testing.T) {
			r, rec := routerPengguna(t, tc.repo)
			h := panggilJSON(t, r, nethttp.MethodPatch, "/users/"+uid("u-1"), tc.badan)
			assert.Equal(t, tc.status, h.Code, h.Body.String())
			if tc.kode != "" {
				assert.Equal(t, tc.kode, h.kode())
			}
			if tc.pesanMuat != "" {
				assert.Contains(t, h.pesan(), tc.pesanMuat)
			}
			tanpaBocor(t, h)
			assert.Empty(t, rec.Entries(), "perubahan yang ditolak tidak diaudit")
		})
	}
}

func TestUpdateUser_BerhasilDanGagalMencabutSesi(t *testing.T) {
	sebelum := penggunaAktif(uid("u-1"))
	sesudah := penggunaAktif(uid("u-1"))
	sesudah.Peran, sesudah.Lapis = "ADMIN_SISTEM", "SISTEM"
	patch := func(string, domain.PenggunaPatch) (*domain.Pengguna, *domain.Pengguna, error) {
		return sebelum, sesudah, nil
	}

	t.Run("berhasil mencabut sesi karena peran berganti", func(t *testing.T) {
		var dicabut []string
		r, rec := routerPengguna(t, &repoUji{
			pengguna: ada, patch: patch,
			cabutSemua: func(id string) error { dicabut = append(dicabut, id); return nil },
		})
		h := panggilJSON(t, r, nethttp.MethodPatch, "/users/"+uid("u-1"), `{"peran":"ADMIN_SISTEM"}`)
		require.Equal(t, nethttp.StatusOK, h.Code, h.Body.String())
		assert.Equal(t, "ADMIN_SISTEM", h.data(t)["peran"])
		assert.Equal(t, []string{uid("u-1")}, dicabut)
		e, ok := rec.Last()
		require.True(t, ok)
		assert.Equal(t, "UPDATE_USER", e.Aksi)
		assert.Equal(t, "OPS_MAHASISWA", snap(e.Sebelum)["peran"])
		assert.Equal(t, "ADMIN_SISTEM", snap(e.Sesudah)["peran"])
	})
	t.Run("gagal mencabut sesi: 500 tetapi perubahan tetap tercatat", func(t *testing.T) {
		r, rec := routerPengguna(t, &repoUji{
			pengguna: ada, patch: patch,
			cabutSemua: func(string) error { return galatDB() },
		})
		h := panggilJSON(t, r, nethttp.MethodPatch, "/users/"+uid("u-1"), `{"peran":"ADMIN_SISTEM"}`)
		assert.Equal(t, nethttp.StatusInternalServerError, h.Code)
		tanpaBocor(t, h)
		e, ok := rec.Last()
		require.True(t, ok, "peran sudah tersimpan: riwayat peran tidak boleh kehilangan perubahannya")
		assert.Equal(t, true, e.Metadata["pencabutan_sesi_gagal"])
	})
}

// ── DELETE /users/:id ─────────────────────────────────────────────────

func TestDeleteUser_PemetaanGalat(t *testing.T) {
	cases := []struct {
		nama   string
		id     string
		repo   *repoUji
		status int
		kode   string
	}{
		{"akun sendiri", uid("admin-1"), &repoUji{}, 400, ""},
		{"tidak ada", uid("hilang"), &repoUji{pengguna: tidakAdaPengguna}, 404, ""},
		// Regresi: galat basis data saat memuat pengguna bukan "pengguna tidak ada".
		{"galat basis data saat memuat", uid("u-1"), &repoUji{pengguna: func(string) (*domain.Pengguna, error) { return nil, galatDB() }}, 500, ""},
		{"masih dirujuk data lain", uid("u-1"), &repoUji{pengguna: ada, hapus: func(string) error { return fmt.Errorf("hapus: %w", gorm.ErrForeignKeyViolated) }}, 409, "409"},
		{"pemegang terakhir pengguna.kelola", uid("u-1"), &repoUji{pengguna: ada, hapus: func(string) error { return fmt.Errorf("hapus: %w", domain.ErrPemegangTerakhir) }}, 409, "LAST_PERMISSION_HOLDER"},
		{"galat basis data saat menghapus", uid("u-1"), &repoUji{pengguna: ada, hapus: func(string) error { return galatDB() }}, 500, ""},
	}
	for _, tc := range cases {
		t.Run(tc.nama, func(t *testing.T) {
			r, rec := routerPengguna(t, tc.repo)
			h := panggilJSON(t, r, nethttp.MethodDelete, "/users/"+tc.id, "")
			assert.Equal(t, tc.status, h.Code, h.Body.String())
			if tc.kode != "" {
				assert.Equal(t, tc.kode, h.kode())
			}
			tanpaBocor(t, h)
			assert.Empty(t, rec.Entries(), "penghapusan yang ditolak tidak diaudit")
		})
	}
}

func TestDeleteUser_BerhasilMengauditDanMenjawabId(t *testing.T) {
	r, rec := routerPengguna(t, &repoUji{
		pengguna: ada, hapus: func(string) error { return nil }, cabutSemua: func(string) error { return nil },
	})

	h := panggilJSON(t, r, nethttp.MethodDelete, "/users/"+uid("u-1"), "")

	require.Equal(t, nethttp.StatusOK, h.Code, h.Body.String())
	assert.Equal(t, uid("u-1"), h.data(t)["id"])
	e, ok := rec.Last()
	require.True(t, ok)
	assert.Equal(t, "DELETE_USER", e.Aksi)
	assert.Equal(t, uid("admin-1"), e.AktorID)
}

// ── POST /users/:id/reset-password ────────────────────────────────────

func TestResetPasswordAdmin_Handler(t *testing.T) {
	menunggu := func(string) (*domain.Pengguna, error) {
		u := penggunaAktif(uid("u-1"))
		u.Status = domain.StatusMenunggu
		return u, nil
	}
	const baik = `{"new_password":"Baru#Sandi123"}`
	cases := []struct {
		nama   string
		repo   *repoUji
		badan  string
		status int
		muat   string
	}{
		{"JSON rusak", &repoUji{}, `{"new_password":`, 400, ""},
		{"kata sandi hilang", &repoUji{}, `{}`, 400, ""},
		{"kata sandi terlalu pendek", &repoUji{}, `{"new_password":"abc"}`, 400, ""},
		{"kata sandi lemah menurut kebijakan", &repoUji{}, `{"new_password":"12345678"}`, 400, ""},
		{"pengguna tidak ada", &repoUji{pengguna: tidakAdaPengguna}, baik, 404, ""},
		// Regresi: galat basis data saat memuat pengguna bukan "pengguna tidak ada".
		{"galat basis data saat memuat", &repoUji{pengguna: func(string) (*domain.Pengguna, error) { return nil, galatDB() }}, baik, 500, ""},
		{"akun menunggu: pakai kirim ulang undangan", &repoUji{pengguna: menunggu}, baik, 409, "undangan"},
		{"galat menyimpan kata sandi", &repoUji{pengguna: ada, setSandi: func(string) (bool, error) { return false, galatDB() }}, baik, 500, ""},
		{"baris hilang saat menyimpan", &repoUji{pengguna: ada, setSandi: func(string) (bool, error) { return false, nil }}, baik, 404, ""},
		{"gagal mencabut sesi", &repoUji{
			pengguna: ada, setSandi: func(string) (bool, error) { return true, nil },
			cabutSemua: func(string) error { return galatDB() },
		}, baik, 500, ""},
	}
	for _, tc := range cases {
		t.Run(tc.nama, func(t *testing.T) {
			r, rec := routerPengguna(t, tc.repo)
			h := panggilJSON(t, r, nethttp.MethodPost, "/users/"+uid("u-1")+"/reset-password", tc.badan)
			assert.Equal(t, tc.status, h.Code, h.Body.String())
			if tc.muat != "" {
				assert.Contains(t, h.pesan(), tc.muat)
			}
			tanpaBocor(t, h)
			assert.NotContains(t, h.Body.String(), "Baru#Sandi123", "kata sandi tidak dipantulkan")
			assert.Empty(t, rec.Entries())
		})
	}

	t.Run("berhasil", func(t *testing.T) {
		r, rec := routerPengguna(t, &repoUji{
			pengguna: ada, setSandi: func(string) (bool, error) { return true, nil },
			cabutSemua: func(string) error { return nil },
		})
		h := panggilJSON(t, r, nethttp.MethodPost, "/users/"+uid("u-1")+"/reset-password", baik)
		require.Equal(t, nethttp.StatusOK, h.Code, h.Body.String())
		assert.Equal(t, uid("u-1"), h.data(t)["id"])
		e, ok := rec.Last()
		require.True(t, ok)
		assert.Equal(t, "RESET_PASSWORD_ADMIN", e.Aksi)
		assert.NotContains(t, fmt.Sprint(e), "Baru#Sandi123", "audit tanpa kata sandi")
	})
}

// ── PUT /users/:id/roles ──────────────────────────────────────────────

func TestAssignRole_Handler(t *testing.T) {
	sebelum, sesudah := penggunaAktif(uid("u-1")), penggunaAktif(uid("u-1"))
	sesudah.Peran, sesudah.Lapis = "MANAJER_DATA", "MANAJERIAL"
	berhasil := func(string, domain.PenggunaPatch) (*domain.Pengguna, *domain.Pengguna, error) {
		return sebelum, sesudah, nil
	}
	galatPatch := func(err error) func(string, domain.PenggunaPatch) (*domain.Pengguna, *domain.Pengguna, error) {
		return func(string, domain.PenggunaPatch) (*domain.Pengguna, *domain.Pengguna, error) { return nil, nil, err }
	}
	cases := []struct {
		nama   string
		repo   *repoUji
		badan  string
		status int
		kode   string
	}{
		{"JSON rusak", &repoUji{}, `{"peran":`, 400, "400"},
		{"peran hilang", &repoUji{}, `{}`, 400, "400"},
		{"peran tidak dikenal", &repoUji{}, `{"peran":"SUPERADMIN"}`, 400, "400"},
		{"peran huruf kecil tidak dikenal", &repoUji{}, `{"peran":"manajer_data"}`, 400, "400"},
		{"pengguna tidak ada", &repoUji{patch: galatPatch(fmt.Errorf("patch: %w", gorm.ErrRecordNotFound))}, `{"peran":"MANAJER_DATA"}`, 404, "404"},
		{"pemegang terakhir pengguna.kelola", &repoUji{patch: galatPatch(domain.ErrPemegangTerakhir)}, `{"peran":"MANAJER_DATA"}`, 409, "LAST_PERMISSION_HOLDER"},
		{"galat basis data", &repoUji{patch: galatPatch(galatDB())}, `{"peran":"MANAJER_DATA"}`, 500, "500"},
		{"gagal mencabut sesi", &repoUji{patch: berhasil, cabutSemua: func(string) error { return galatDB() }}, `{"peran":"MANAJER_DATA"}`, 500, "500"},
	}
	for _, tc := range cases {
		t.Run(tc.nama, func(t *testing.T) {
			r, _ := routerPengguna(t, tc.repo)
			h := panggilJSON(t, r, nethttp.MethodPut, "/users/"+uid("u-1")+"/roles", tc.badan)
			assert.Equal(t, tc.status, h.Code, h.Body.String())
			assert.Equal(t, tc.kode, h.kode())
			tanpaBocor(t, h)
		})
	}

	t.Run("berhasil mencabut sesi dan mencatat sebelum/sesudah", func(t *testing.T) {
		var dicabut []string
		r, rec := routerPengguna(t, &repoUji{patch: berhasil, cabutSemua: func(id string) error { dicabut = append(dicabut, id); return nil }})
		h := panggilJSON(t, r, nethttp.MethodPut, "/users/"+uid("u-1")+"/roles", `{"peran":"MANAJER_DATA"}`)
		require.Equal(t, nethttp.StatusOK, h.Code, h.Body.String())
		d := h.data(t)
		assert.Equal(t, "MANAJER_DATA", d["peran"])
		assert.Equal(t, "MANAJERIAL", d["lapis"])
		assert.Equal(t, []string{uid("u-1")}, dicabut)
		e, ok := rec.Last()
		require.True(t, ok)
		assert.Equal(t, "ASSIGN_ROLE", e.Aksi)
		assert.Equal(t, "OPS_MAHASISWA", snap(e.Sebelum)["peran"])
		assert.Equal(t, "MANAJER_DATA", snap(e.Sesudah)["peran"])
	})
}

// ── POST /users/bulk ──────────────────────────────────────────────────

func TestBulkUser_ValidasiBindingDijawab400(t *testing.T) {
	r, _ := routerPengguna(t, &repoUji{})
	for nama, badan := range map[string]string{
		"JSON rusak":       `{"action":`,
		"aksi hilang":      `{"ids":["u-1"]}`,
		"aksi tak dikenal": `{"action":"hapus_semua","ids":["u-1"]}`,
		"id hilang":        `{"action":"aktifkan"}`,
		"id kosong":        `{"action":"aktifkan","ids":[]}`,
		"id bukan larik":   `{"action":"aktifkan","ids":"u-1"}`,
		"aksi huruf besar": `{"action":"AKTIFKAN","ids":["u-1"]}`,
	} {
		h := panggilJSON(t, r, nethttp.MethodPost, "/users/bulk", badan)
		assert.Equal(t, nethttp.StatusBadRequest, h.Code, nama)
	}
}

func TestBulkUser_PemetaanGalatDanKeberhasilan(t *testing.T) {
	badan := js(map[string]any{"action": "nonaktifkan", "ids": []string{uid("u-1"), uid("u-2")}})

	t.Run("pemegang terakhir ditolak seluruhnya", func(t *testing.T) {
		r, rec := routerPengguna(t, &repoUji{bulk: func([]string, domain.BulkUserAction) (int64, error) {
			return 0, fmt.Errorf("bulk: %w", domain.ErrPemegangTerakhir)
		}})
		h := panggilJSON(t, r, nethttp.MethodPost, "/users/bulk", badan)
		assert.Equal(t, nethttp.StatusConflict, h.Code)
		assert.Equal(t, "LAST_PERMISSION_HOLDER", h.kode())
		assert.Empty(t, rec.Entries())
	})
	t.Run("galat basis data tidak bocor", func(t *testing.T) {
		r, _ := routerPengguna(t, &repoUji{bulk: func([]string, domain.BulkUserAction) (int64, error) { return 0, galatDB() }})
		h := panggilJSON(t, r, nethttp.MethodPost, "/users/bulk", badan)
		assert.Equal(t, nethttp.StatusInternalServerError, h.Code)
		tanpaBocor(t, h)
	})
	t.Run("berhasil melaporkan jumlah yang diproses dan mengaudit", func(t *testing.T) {
		var aksi domain.BulkUserAction
		r, rec := routerPengguna(t, &repoUji{bulk: func(_ []string, a domain.BulkUserAction) (int64, error) { aksi = a; return 1, nil }})
		h := panggilJSON(t, r, nethttp.MethodPost, "/users/bulk", badan)
		require.Equal(t, nethttp.StatusOK, h.Code, h.Body.String())
		assert.EqualValues(t, 1, h.data(t)["berhasil"], "akun MENUNGGU dilewati repositori: jumlah mengikuti yang benar-benar berubah")
		assert.Equal(t, domain.BulkNonaktifkan, aksi)
		e, ok := rec.Last()
		require.True(t, ok)
		assert.Equal(t, "BULK_nonaktifkan", e.Aksi)
	})
}

// ── GET /roles, GET /permissions ──────────────────────────────────────

func TestListRoles_DanListPermissions(t *testing.T) {
	waktu := time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC)
	repo := &repoUji{
		listPeran: func() ([]domain.PeranDetail, error) {
			return []domain.PeranDetail{
				{Peran: domain.Peran{Kunci: "ADMIN_SISTEM", Label: "Admin Sistem", Lapis: "SISTEM", Versi: 3, DiperbaruiPada: waktu}, Izin: []string{"pengguna.kelola"}},
				{Peran: domain.Peran{Kunci: "OPS_MAHASISWA", Label: "Mahasiswa", Lapis: "OPERASIONAL", Versi: 1, DiperbaruiPada: waktu}, Izin: []string{}},
			}, nil
		},
		listIzin: func() ([]domain.Izin, error) {
			return []domain.Izin{{Nama: "pengguna.kelola", Deskripsi: "Kelola pengguna", Kelompok: "Pengguna"}}, nil
		},
	}
	r, _ := routerPengguna(t, repo)

	t.Run("peran", func(t *testing.T) {
		h := panggilJSON(t, r, nethttp.MethodGet, "/roles", "")
		require.Equal(t, nethttp.StatusOK, h.Code, h.Body.String())
		items := h.JSON["data"].([]any)
		require.Len(t, items, 2)
		admin := items[0].(map[string]any)
		assert.Equal(t, "ADMIN_SISTEM", admin["id"])
		assert.Equal(t, "Admin Sistem", admin["label"])
		assert.EqualValues(t, 3, admin["versi"])
		assert.Equal(t, []any{"pengguna.kelola"}, admin["permissions"])
		assert.NotEmpty(t, admin["bawaan"], "matriks bawaan dari kode ikut sebagai pembanding")
		assert.Equal(t, []any{}, items[1].(map[string]any)["permissions"], "peran tanpa izin: larik kosong, bukan null")
	})
	t.Run("izin", func(t *testing.T) {
		h := panggilJSON(t, r, nethttp.MethodGet, "/permissions", "")
		require.Equal(t, nethttp.StatusOK, h.Code, h.Body.String())
		items := h.JSON["data"].([]any)
		require.Len(t, items, 1)
		assert.Equal(t, map[string]any{"id": "pengguna.kelola", "deskripsi": "Kelola pengguna", "kelompok": "Pengguna"}, items[0])
	})
	t.Run("galat basis data", func(t *testing.T) {
		rusak, _ := routerPengguna(t, &repoUji{
			listPeran: func() ([]domain.PeranDetail, error) { return nil, galatDB() },
			listIzin:  func() ([]domain.Izin, error) { return nil, galatDB() },
		})
		for _, path := range []string{"/roles", "/permissions"} {
			h := panggilJSON(t, rusak, nethttp.MethodGet, path, "")
			assert.Equal(t, nethttp.StatusInternalServerError, h.Code, path)
			tanpaBocor(t, h)
		}
	})
}

// ── POST /users/:id/buka-kunci, POST /users/:id/revoke-sessions ───────

func TestBukaKunci_Handler(t *testing.T) {
	t.Run("berhasil membuka akun terkunci", func(t *testing.T) {
		sampai := time.Now().Add(10 * time.Minute)
		var hash string
		r, rec := routerPengguna(t, &repoUji{
			pengguna: ada,
			hapusKunci: func(h string) (*domain.KunciLogin, error) {
				hash = h
				return &domain.KunciLogin{Gagal: 5, TerkunciSampai: &sampai}, nil
			},
		})
		h := panggilJSON(t, r, nethttp.MethodPost, "/users/"+uid("u-1")+"/buka-kunci", "")
		require.Equal(t, nethttp.StatusOK, h.Code, h.Body.String())
		d := h.data(t)
		assert.Equal(t, uid("u-1"), d["id"])
		assert.Equal(t, true, d["terkunci_sebelumnya"])
		assert.Equal(t, usecase.HashEmailKunci(uid("u-1")+"@garda.test"), hash, "yang dihapus adalah kunci email akun, bukan id")
		e, ok := rec.Last()
		require.True(t, ok)
		assert.Equal(t, "UNLOCK_USER", e.Aksi)
	})
	t.Run("idempoten untuk akun yang tidak terkunci", func(t *testing.T) {
		r, _ := routerPengguna(t, &repoUji{pengguna: ada, hapusKunci: func(string) (*domain.KunciLogin, error) { return nil, nil }})
		h := panggilJSON(t, r, nethttp.MethodPost, "/users/"+uid("u-1")+"/buka-kunci", "")
		require.Equal(t, nethttp.StatusOK, h.Code)
		assert.Equal(t, false, h.data(t)["terkunci_sebelumnya"])
	})
	t.Run("pengguna tidak ada", func(t *testing.T) {
		r, _ := routerPengguna(t, &repoUji{pengguna: tidakAdaPengguna})
		assert.Equal(t, nethttp.StatusNotFound, panggilJSON(t, r, nethttp.MethodPost, "/users/hilang/buka-kunci", "").Code)
	})
	t.Run("galat basis data", func(t *testing.T) {
		for nama, repo := range map[string]*repoUji{
			"memuat pengguna": {pengguna: func(string) (*domain.Pengguna, error) { return nil, galatDB() }},
			"menghapus kunci": {pengguna: ada, hapusKunci: func(string) (*domain.KunciLogin, error) { return nil, galatDB() }},
		} {
			r, _ := routerPengguna(t, repo)
			h := panggilJSON(t, r, nethttp.MethodPost, "/users/"+uid("u-1")+"/buka-kunci", "")
			assert.Equal(t, nethttp.StatusInternalServerError, h.Code, nama)
			tanpaBocor(t, h)
		}
	})
}

func TestForceLogoutUser_Handler(t *testing.T) {
	t.Run("berhasil", func(t *testing.T) {
		r, rec := routerPengguna(t, &repoUji{pengguna: ada, cabutSesi: func(string) ([]string, error) { return []string{"f1", "f2"}, nil }})
		h := panggilJSON(t, r, nethttp.MethodPost, "/users/"+uid("u-1")+"/revoke-sessions", "")
		require.Equal(t, nethttp.StatusOK, h.Code, h.Body.String())
		assert.EqualValues(t, 2, h.data(t)["dicabut"])
		e, ok := rec.Last()
		require.True(t, ok)
		assert.Equal(t, "FORCE_LOGOUT", e.Aksi)
	})
	t.Run("pengguna tidak ada", func(t *testing.T) {
		r, _ := routerPengguna(t, &repoUji{pengguna: tidakAdaPengguna})
		assert.Equal(t, nethttp.StatusNotFound, panggilJSON(t, r, nethttp.MethodPost, "/users/hilang/revoke-sessions", "").Code)
	})
	t.Run("galat basis data", func(t *testing.T) {
		for nama, repo := range map[string]*repoUji{
			"memuat pengguna": {pengguna: func(string) (*domain.Pengguna, error) { return nil, galatDB() }},
			"mencabut sesi":   {pengguna: ada, cabutSesi: func(string) ([]string, error) { return nil, galatDB() }},
		} {
			r, _ := routerPengguna(t, repo)
			h := panggilJSON(t, r, nethttp.MethodPost, "/users/"+uid("u-1")+"/revoke-sessions", "")
			assert.Equal(t, nethttp.StatusInternalServerError, h.Code, nama)
			tanpaBocor(t, h)
		}
	})
}

// ── jawabAkunTerkunci ─────────────────────────────────────────────────

func TestJawabAkunTerkunci_DenganDanTanpaRincian(t *testing.T) {
	rinci := routerKecil(func(c *gin.Context) {
		jawabAkunTerkunci(c, &usecase.AkunTerkunciError{Sampai: time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC), SisaDetik: 90})
	})
	h := panggilJSON(t, rinci, nethttp.MethodGet, "/x", "")
	assert.Equal(t, nethttp.StatusLocked, h.Code)
	assert.Equal(t, "90", h.Header().Get("Retry-After"))
	assert.EqualValues(t, 90, h.data(t)["sisa_detik"])

	polos := routerKecil(func(c *gin.Context) { jawabAkunTerkunci(c, usecase.ErrAccountLocked) })
	h = panggilJSON(t, polos, nethttp.MethodGet, "/x", "")
	assert.Equal(t, nethttp.StatusLocked, h.Code)
	assert.Empty(t, h.Header().Get("Retry-After"), "tanpa rincian: tidak ada Retry-After yang bisa dipercaya")
	assert.False(t, strings.Contains(h.Body.String(), "sisa_detik"))
}

// Regresi: peran atau lapis di luar enum dulu lolos validasi, lalu ditolak basis data (enum PostgreSQL) dan
// dijawab 500 "Gagal membuat pengguna" untuk kesalahan klien. UpdateUser dan AssignRole sudah menjawab 400.
func TestCreateUser_PeranDanLapisDiluarKatalogDijawab400(t *testing.T) {
	buat := func(peran, lapis string) string {
		return js(map[string]any{
			"id": uid("u-baru"), "nama": "Baru", "email": "baru@garda.test", "password": "Kata#Sandi1",
			"peran": peran, "lapis": lapis, "lembaga": "BPPSDMP",
		})
	}
	repo := &repoUji{
		byEmail: func(string) (*domain.Pengguna, error) { return nil, gorm.ErrRecordNotFound },
		buat:    func(*domain.Pengguna) error { t.Error("pengguna tidak boleh disimpan"); return nil },
	}
	for nama, tc := range map[string]struct{ badan, muat string }{
		"peran tak dikenal": {buat("SUPERADMIN", "SISTEM"), "Peran"},
		"peran huruf kecil": {buat("admin_sistem", "SISTEM"), "Peran"},
		"lapis tak dikenal": {buat("OPS_MAHASISWA", "OPS"), "Lapis"},
		"lapis huruf kecil": {buat("OPS_MAHASISWA", "operasional"), "Lapis"},
	} {
		t.Run(nama, func(t *testing.T) {
			r, rec := routerPengguna(t, repo)
			h := panggilJSON(t, r, nethttp.MethodPost, "/users", tc.badan)
			assert.Equal(t, nethttp.StatusBadRequest, h.Code, h.Body.String())
			assert.Contains(t, h.pesan(), tc.muat)
			assert.Empty(t, rec.Entries())
		})
	}
}
