package http

import (
	"errors"
	"fmt"
	nethttp "net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/garda-inflasi/api/internal/auth/domain"
)

// PATCH /v1/auth/me dan GET /v1/auth/me: pemetaan galat usecase → status HTTP dan bentuk jawabannya. Pemanggil
// adalah uid("admin-1") (router tes memasang user_id); repositori ditiru (repoUji).

const pemilikProfil = "admin-1"

func ptrS(s string) *string { return &s }

func TestUpdateMe_GalatValidasiDijawab400DenganPesanKontrakTanpaMenyentuhRepositori(t *testing.T) {
	cases := []struct {
		nama, badan, pesan string
	}{
		{"nama kosong", `{"nama":""}`, "Nama minimal 3 karakter"},
		{"nama spasi semua", `{"nama":"     "}`, "Nama minimal 3 karakter"},
		{"nama 2 karakter", `{"nama":"Al"}`, "Nama minimal 3 karakter"},
		{"nama 101 karakter", js(map[string]any{"nama": strings.Repeat("a", 101)}), "Nama maksimal 100 karakter"},
		{"telepon kode negara lain", `{"telepon":"+1 415 555 2671"}`, "Nomor telepon tidak valid"},
		{"telepon tanpa awalan", `{"telepon":"8123456789"}`, "Nomor telepon tidak valid"},
		{"telepon berhuruf", `{"telepon":"0812abc34567"}`, "Nomor telepon tidak valid"},
		{"objek kosong", `{}`, "Tidak ada perubahan"},
		{"hanya null", `{"nama":null,"telepon":null}`, "Tidak ada perubahan"},
		{"hanya field terlarang", `{"email":"x@garda.test","peran":"ADMIN_SISTEM","lembaga":"X"}`, "Tidak ada perubahan"},
		{"badan kosong", ``, "Tidak ada perubahan"},
	}
	for _, tc := range cases {
		t.Run(tc.nama, func(t *testing.T) {
			// repoUji kosong: sentuhan apa pun ke repositori menggagalkan tes.
			r, rec := routerPengguna(t, &repoUji{})

			h := panggilJSON(t, r, nethttp.MethodPatch, "/auth/me", tc.badan)

			assert.Equal(t, nethttp.StatusBadRequest, h.Code, h.Body.String())
			assert.Equal(t, "400", h.kode())
			assert.Equal(t, tc.pesan, h.pesan())
			assert.Empty(t, rec.Entries())
		})
	}
}

func TestUpdateMe_BadanTakLayakDijawab400(t *testing.T) {
	r, rec := routerPengguna(t, &repoUji{})

	for nama, badan := range map[string]string{
		"JSON rusak":    `{"nama":`,
		"bukan objek":   `["Nama"]`,
		"tipe salah":    `{"nama":123}`,
		"pagar nama":    js(map[string]any{"nama": strings.Repeat("a", 1001)}),
		"pagar telepon": js(map[string]any{"telepon": strings.Repeat("1", 101)}),
	} {
		h := panggilJSON(t, r, nethttp.MethodPatch, "/auth/me", badan)

		assert.Equal(t, nethttp.StatusBadRequest, h.Code, "%s: %s", nama, h.Body.String())
		assert.Equal(t, "400", h.kode(), nama)
		assert.True(t, strings.HasPrefix(h.pesan(), "Validasi gagal"), "%s: %s", nama, h.pesan())
	}
	assert.Empty(t, rec.Entries())
}

func TestUpdateMe_PenggunaTidakAdaDijawab404(t *testing.T) {
	r, rec := routerPengguna(t, &repoUji{pengguna: tidakAdaPengguna})

	h := panggilJSON(t, r, nethttp.MethodPatch, "/auth/me", `{"nama":"Nama Baru"}`)

	assert.Equal(t, nethttp.StatusNotFound, h.Code, h.Body.String())
	assert.Equal(t, "404", h.kode())
	assert.Equal(t, "Pengguna tidak ditemukan", h.pesan())
	assert.Empty(t, rec.Entries())
}

func TestUpdateMe_PenggunaHilangDiTengahPenulisanDijawab404(t *testing.T) {
	r, rec := routerPengguna(t, &repoUji{
		pengguna: ada,
		patch: func(string, domain.PenggunaPatch) (*domain.Pengguna, *domain.Pengguna, error) {
			return nil, nil, fmt.Errorf("get user for update: %w", gorm.ErrRecordNotFound)
		},
	})

	h := panggilJSON(t, r, nethttp.MethodPatch, "/auth/me", `{"nama":"Nama Baru"}`)

	assert.Equal(t, nethttp.StatusNotFound, h.Code, h.Body.String())
	assert.Empty(t, rec.Entries())
}

func TestUpdateMe_GalatBasisDataDijawab500TanpaMembocorkanDetail(t *testing.T) {
	t.Run("saat membaca", func(t *testing.T) {
		r, rec := routerPengguna(t, &repoUji{pengguna: func(string) (*domain.Pengguna, error) { return nil, galatDB() }})

		h := panggilJSON(t, r, nethttp.MethodPatch, "/auth/me", `{"nama":"Nama Baru"}`)

		assert.Equal(t, nethttp.StatusInternalServerError, h.Code, h.Body.String())
		assert.Equal(t, "500", h.kode())
		tanpaBocor(t, h)
		assert.Empty(t, rec.Entries())
	})
	t.Run("saat menulis", func(t *testing.T) {
		r, rec := routerPengguna(t, &repoUji{
			pengguna: ada,
			patch: func(string, domain.PenggunaPatch) (*domain.Pengguna, *domain.Pengguna, error) {
				return nil, nil, galatDB()
			},
		})

		h := panggilJSON(t, r, nethttp.MethodPatch, "/auth/me", `{"nama":"Nama Baru"}`)

		assert.Equal(t, nethttp.StatusInternalServerError, h.Code, h.Body.String())
		tanpaBocor(t, h)
		assert.Empty(t, rec.Entries(), "penulisan gagal tidak diaudit sebagai berhasil")
	})
}

// Hanya nama dan telepon milik pemanggil (user_id dari token, bukan dari badan) yang sampai ke repositori.
func TestUpdateMe_HanyaNamaDanTeleponPemilikYangDitulis(t *testing.T) {
	var idDitulis string
	var patch domain.PenggunaPatch
	pemilik := penggunaAktif(uid(pemilikProfil))
	r, rec := routerPengguna(t, &repoUji{
		pengguna: ada,
		patch: func(id string, p domain.PenggunaPatch) (*domain.Pengguna, *domain.Pengguna, error) {
			idDitulis, patch = id, p
			sebelum := *pemilik
			sesudah := *pemilik
			p.Terapkan(&sesudah)
			return &sebelum, &sesudah, nil
		},
	})

	h := panggilJSON(t, r, nethttp.MethodPatch, "/auth/me", js(map[string]any{
		"nama": "  Nama Baru  ", "telepon": "0812-3456-7890",
		"id": uid("korban"), "email": "bajak@garda.test", "peran": "ADMIN_SISTEM", "lapis": "SISTEM", "lembaga": "X",
		"kabupaten_kode": "1171", "status": "NONAKTIF",
	}))

	require.Equal(t, nethttp.StatusOK, h.Code, h.Body.String())
	assert.Equal(t, uid(pemilikProfil), idDitulis, "yang ditulis adalah akun pemegang token")
	require.NotNil(t, patch.Nama)
	require.NotNil(t, patch.Telepon)
	assert.Equal(t, "Nama Baru", *patch.Nama)
	assert.Equal(t, "+6281234567890", *patch.Telepon)
	assert.Nil(t, patch.Email)
	assert.Nil(t, patch.Peran)
	assert.Nil(t, patch.Lapis)
	assert.Nil(t, patch.Lembaga)
	assert.Nil(t, patch.KabupatenKode)
	assert.Nil(t, patch.Status)

	d := h.data(t)
	assert.Equal(t, uid(pemilikProfil), d["id"])
	assert.Equal(t, "Nama Baru", d["nama"])
	assert.Equal(t, "+6281234567890", d["telepon"])
	assert.Equal(t, "OPS_MAHASISWA", d["peran"], "peran tidak berubah")
	assert.Equal(t, false, d["mfa_setup_diwajibkan"])
	assert.Equal(t, false, d["mfa_wajib"])
	assert.NotContains(t, d, "kode_cadangan_tersisa")

	e, ok := rec.Last()
	require.True(t, ok)
	assert.Equal(t, "UPDATE_PROFIL", e.Aksi)
	assert.Equal(t, uid(pemilikProfil), e.AktorID)
	assert.Equal(t, uid(pemilikProfil), e.EntitasID)
	assert.Equal(t, "Ani", snap(e.Sebelum)["nama"])
	assert.Equal(t, "Nama Baru", snap(e.Sesudah)["nama"])
}

func TestUpdateMe_NilaiSamaDijawab200TanpaMenulisDanTanpaAudit(t *testing.T) {
	pemilik := penggunaAktif(uid(pemilikProfil))
	pemilik.Telepon = ptrS("+6281234567890")
	// patch tidak diisi: PatchUser dipanggil = tes gagal.
	r, rec := routerPengguna(t, &repoUji{pengguna: func(string) (*domain.Pengguna, error) { return pemilik, nil }})

	h := panggilJSON(t, r, nethttp.MethodPatch, "/auth/me", `{"nama":" Ani ","telepon":"+62 812-3456-7890"}`)

	require.Equal(t, nethttp.StatusOK, h.Code, h.Body.String())
	d := h.data(t)
	assert.Equal(t, "Ani", d["nama"])
	assert.Equal(t, "+6281234567890", d["telepon"])
	assert.Empty(t, rec.Entries())
}

func TestUpdateMe_HapusTeleponMenghilangkanFieldDariJawaban(t *testing.T) {
	pemilik := penggunaAktif(uid(pemilikProfil))
	pemilik.Telepon = ptrS("+6281234567890")
	var patch domain.PenggunaPatch
	r, _ := routerPengguna(t, &repoUji{
		pengguna: func(string) (*domain.Pengguna, error) { return pemilik, nil },
		patch: func(_ string, p domain.PenggunaPatch) (*domain.Pengguna, *domain.Pengguna, error) {
			patch = p
			sebelum := *pemilik
			sesudah := *pemilik
			p.Terapkan(&sesudah)
			return &sebelum, &sesudah, nil
		},
	})

	h := panggilJSON(t, r, nethttp.MethodPatch, "/auth/me", `{"telepon":""}`)

	require.Equal(t, nethttp.StatusOK, h.Code, h.Body.String())
	assert.Equal(t, map[string]any{"telepon": nil}, patch.Kolom(), "ditulis NULL")
	assert.NotContains(t, h.data(t), "telepon")
}

func TestUpdateMe_AkunMFAAktifMemuatSisaKodeCadangan(t *testing.T) {
	pemilik := penggunaAktif(uid(pemilikProfil))
	pemilik.MfaAktif = true
	patch := func(_ string, p domain.PenggunaPatch) (*domain.Pengguna, *domain.Pengguna, error) {
		sebelum := *pemilik
		sesudah := *pemilik
		p.Terapkan(&sesudah)
		return &sebelum, &sesudah, nil
	}

	t.Run("sisa kode dimuat", func(t *testing.T) {
		r, _ := routerPengguna(t, &repoUji{
			pengguna:   func(string) (*domain.Pengguna, error) { return pemilik, nil },
			patch:      patch,
			hitungKode: func(string) (int64, error) { return 7, nil },
		})

		h := panggilJSON(t, r, nethttp.MethodPatch, "/auth/me", `{"nama":"Nama Baru"}`)

		require.Equal(t, nethttp.StatusOK, h.Code, h.Body.String())
		d := h.data(t)
		assert.Equal(t, true, d["mfa_aktif"])
		assert.EqualValues(t, 7, d["kode_cadangan_tersisa"])
	})
	t.Run("galat membaca status MFA dijawab 500 tanpa membocorkan detail", func(t *testing.T) {
		r, _ := routerPengguna(t, &repoUji{
			pengguna:   func(string) (*domain.Pengguna, error) { return pemilik, nil },
			patch:      patch,
			hitungKode: func(string) (int64, error) { return 0, errors.New(rahasiaInternal) },
		})

		h := panggilJSON(t, r, nethttp.MethodPatch, "/auth/me", `{"nama":"Nama Baru"}`)

		assert.Equal(t, nethttp.StatusInternalServerError, h.Code, h.Body.String())
		tanpaBocor(t, h)
	})
}

// GET /v1/auth/me memuat telepon bila ada, dan menghilangkannya bila kosong.
func TestMe_MemuatTeleponBilaAda(t *testing.T) {
	ada := penggunaAktif(uid(pemilikProfil))
	ada.Telepon = ptrS("+6281200000001")
	kosong := penggunaAktif(uid(pemilikProfil))

	r, _ := routerPengguna(t, &repoUji{pengguna: func(string) (*domain.Pengguna, error) { return ada, nil }})
	h := panggilJSON(t, r, nethttp.MethodGet, "/auth/me", ``)
	require.Equal(t, nethttp.StatusOK, h.Code, h.Body.String())
	assert.Equal(t, "+6281200000001", h.data(t)["telepon"])

	r, _ = routerPengguna(t, &repoUji{pengguna: func(string) (*domain.Pengguna, error) { return kosong, nil }})
	h = panggilJSON(t, r, nethttp.MethodGet, "/auth/me", ``)
	require.Equal(t, nethttp.StatusOK, h.Code, h.Body.String())
	assert.NotContains(t, h.data(t), "telepon")
}
