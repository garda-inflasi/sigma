package http

import (
	"fmt"
	nethttp "net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/garda-inflasi/api/internal/auth/domain"
)

// Tes handler RBAC berbasis basis data (02-C): edit matriks peran, daftar/beri/cabut izin langsung. Yang
// diperiksa: kode galat mesin yang dibaca UI (LAST_PERMISSION_HOLDER, ROLE_VERSION_CONFLICT, ...), status
// HTTP, dan bahwa audit hanya ditulis untuk perubahan yang benar-benar terjadi.

func peranContoh(kunci string, versi int64, izin ...string) domain.PeranDetail {
	return domain.PeranDetail{
		Peran: domain.Peran{Kunci: kunci, Label: kunci, Lapis: "SISTEM", Versi: versi, DiperbaruiPada: time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC)},
		Izin:  izin,
	}
}

// ── PUT /roles/:id/permissions ────────────────────────────────────────

func TestUpdateRolePermissions_ValidasiBindingDijawab400(t *testing.T) {
	r, _ := routerPengguna(t, &repoUji{})
	for nama, badan := range map[string]string{
		"JSON rusak":        `{"permissions":`,
		"izin hilang":       `{"versi":1}`,
		"versi hilang":      `{"permissions":[]}`,
		"versi nol":         `{"permissions":[],"versi":0}`,
		"versi negatif":     `{"permissions":[],"versi":-1}`,
		"izin bukan larik":  `{"permissions":"pengguna.kelola","versi":1}`,
		"versi bukan angka": `{"permissions":[],"versi":"1"}`,
		"izin berisi angka": `{"permissions":[1],"versi":1}`,
		"versi pecahan":     `{"permissions":[],"versi":1.5}`,
	} {
		h := panggilJSON(t, r, nethttp.MethodPut, "/roles/ADMIN_SISTEM/permissions", badan)
		assert.Equal(t, nethttp.StatusBadRequest, h.Code, nama+": "+h.Body.String())
	}
}

func TestUpdateRolePermissions_PemetaanGalat(t *testing.T) {
	const badan = `{"permissions":["harga.lihat"],"versi":2}`
	cases := []struct {
		nama   string
		id     string
		repo   *repoUji
		badan  string
		status int
		kode   string
	}{
		{"peran tidak dikenal", "SUPERADMIN", &repoUji{}, badan, 404, "404"},
		{"izin di luar katalog", "ADMIN_SISTEM", &repoUji{}, `{"permissions":["sistem.rusak","harga.lihat"],"versi":2}`, 400, "UNKNOWN_PERMISSION"},
		{"peran hilang di basis data", "ADMIN_SISTEM", &repoUji{setIzinPeran: func(domain.SetIzinPeran) (*domain.HasilSetIzinPeran, error) {
			return nil, fmt.Errorf("baca peran: %w", gorm.ErrRecordNotFound)
		}}, badan, 404, "404"},
		{"pemegang terakhir pengguna.kelola", "ADMIN_SISTEM", &repoUji{setIzinPeran: func(domain.SetIzinPeran) (*domain.HasilSetIzinPeran, error) {
			return nil, domain.ErrPemegangTerakhir
		}}, badan, 409, "LAST_PERMISSION_HOLDER"},
		{"nama izin ditolak basis data", "ADMIN_SISTEM", &repoUji{setIzinPeran: func(domain.SetIzinPeran) (*domain.HasilSetIzinPeran, error) {
			return nil, domain.ErrIzinTidakDikenal
		}}, badan, 400, "UNKNOWN_PERMISSION"},
		{"galat basis data", "ADMIN_SISTEM", &repoUji{setIzinPeran: func(domain.SetIzinPeran) (*domain.HasilSetIzinPeran, error) {
			return nil, galatDB()
		}}, badan, 500, "500"},
	}
	for _, tc := range cases {
		t.Run(tc.nama, func(t *testing.T) {
			r, rec := routerPengguna(t, tc.repo)
			h := panggilJSON(t, r, nethttp.MethodPut, "/roles/"+tc.id+"/permissions", tc.badan)
			assert.Equal(t, tc.status, h.Code, h.Body.String())
			assert.Equal(t, tc.kode, h.kode())
			tanpaBocor(t, h)
			assert.Empty(t, rec.Entries(), "perubahan yang ditolak tidak diaudit")
		})
	}
}

func TestUpdateRolePermissions_KonflikVersiMembawaPeranSaatIni(t *testing.T) {
	r, rec := routerPengguna(t, &repoUji{
		setIzinPeran: func(domain.SetIzinPeran) (*domain.HasilSetIzinPeran, error) { return nil, domain.ErrVersiPeranBerubah },
		listPeran: func() ([]domain.PeranDetail, error) {
			return []domain.PeranDetail{peranContoh("MANAJER_DATA", 1), peranContoh("ADMIN_SISTEM", 5, "pengguna.kelola", "harga.lihat")}, nil
		},
	})

	h := panggilJSON(t, r, nethttp.MethodPut, "/roles/ADMIN_SISTEM/permissions", `{"permissions":["harga.lihat"],"versi":2}`)

	require.Equal(t, nethttp.StatusConflict, h.Code, h.Body.String())
	assert.Equal(t, "ROLE_VERSION_CONFLICT", h.kode())
	saat := h.data(t)
	assert.Equal(t, "ADMIN_SISTEM", saat["id"], "data berisi peran saat ini supaya UI memuat ulang tanpa permintaan lagi")
	assert.EqualValues(t, 5, saat["versi"])
	assert.Equal(t, []any{"pengguna.kelola", "harga.lihat"}, saat["permissions"])
	assert.Empty(t, rec.Entries())
}

func TestUpdateRolePermissions_BerhasilDanTanpaPerubahanTidakDiaudit(t *testing.T) {
	t.Run("perubahan nyata diaudit dengan selisih", func(t *testing.T) {
		var diterima domain.SetIzinPeran
		r, rec := routerPengguna(t, &repoUji{setIzinPeran: func(p domain.SetIzinPeran) (*domain.HasilSetIzinPeran, error) {
			diterima = p
			return &domain.HasilSetIzinPeran{
				Peran:   domain.Peran{Kunci: "ADMIN_SISTEM", Label: "Admin", Lapis: "SISTEM", Versi: 3},
				Sebelum: []string{"harga.lihat"}, Sesudah: []string{"harga.lihat", "harga.ekspor"}, Berubah: true,
			}, nil
		}})
		h := panggilJSON(t, r, nethttp.MethodPut, "/roles/ADMIN_SISTEM/permissions", `{"permissions":["harga.lihat","harga.ekspor"],"versi":2}`)
		require.Equal(t, nethttp.StatusOK, h.Code, h.Body.String())
		assert.Equal(t, "ADMIN_SISTEM", diterima.Peran)
		assert.EqualValues(t, 2, diterima.Versi)
		assert.Equal(t, "admin-1", diterima.Oleh)
		assert.Equal(t, []any{"harga.lihat", "harga.ekspor"}, h.data(t)["permissions"])
		e, ok := rec.Last()
		require.True(t, ok)
		assert.Equal(t, "UPDATE_ROLE_PERMISSIONS", e.Aksi)
		assert.Equal(t, "peran", e.Entitas)
		assert.Equal(t, []string{"harga.ekspor"}, e.Metadata["ditambah"])
		assert.Equal(t, []string{}, e.Metadata["dicabut"])
	})
	t.Run("himpunan sama: tidak menulis dan tidak diaudit", func(t *testing.T) {
		r, rec := routerPengguna(t, &repoUji{setIzinPeran: func(domain.SetIzinPeran) (*domain.HasilSetIzinPeran, error) {
			return &domain.HasilSetIzinPeran{
				Peran:   domain.Peran{Kunci: "ADMIN_SISTEM", Versi: 2},
				Sebelum: []string{"harga.lihat"}, Sesudah: []string{"harga.lihat"},
			}, nil
		}})
		h := panggilJSON(t, r, nethttp.MethodPut, "/roles/ADMIN_SISTEM/permissions", `{"permissions":["harga.lihat"],"versi":2}`)
		require.Equal(t, nethttp.StatusOK, h.Code, h.Body.String())
		assert.EqualValues(t, 2, h.data(t)["versi"], "versi tidak naik")
		assert.Empty(t, rec.Entries())
	})
}

// ── GET /users/:id/permissions ────────────────────────────────────────

func TestListUserPermissions_Handler(t *testing.T) {
	lewat := time.Now().Add(-time.Hour)
	depan := time.Now().Add(time.Hour)
	pemberi := "admin-1"
	nama := "Admin Satu"
	t.Run("berhasil: peran, langsung aktif dan kedaluwarsa, efektif", func(t *testing.T) {
		r, _ := routerPengguna(t, &repoUji{listLangsung: func(string) (*domain.IzinPengguna, error) {
			return &domain.IzinPengguna{
				Peran: "OPS_MAHASISWA", IzinPeran: []string{"harga.lihat"},
				Langsung: []domain.IzinLangsungDetail{
					{IzinLangsung: domain.IzinLangsung{PenggunaID: "u-1", PermissionNama: "harga.ekspor", BerlakuSampai: &depan, Alasan: "audit", DiberikanOleh: &pemberi, DiberikanPada: time.Now()}, Deskripsi: "Ekspor", Kelompok: "Harga", DiberikanOlehNama: &nama},
					{IzinLangsung: domain.IzinLangsung{PenggunaID: "u-1", PermissionNama: "harga.validasi", BerlakuSampai: &lewat, Alasan: "dulu", DiberikanPada: time.Now()}},
				},
			}, nil
		}})
		h := panggilJSON(t, r, nethttp.MethodGet, "/users/u-1/permissions", "")
		require.Equal(t, nethttp.StatusOK, h.Code, h.Body.String())
		d := h.data(t)
		assert.Equal(t, "OPS_MAHASISWA", d["peran"])
		assert.Equal(t, []any{"harga.lihat"}, d["izin_peran"])
		assert.Equal(t, []any{"harga.ekspor", "harga.lihat"}, d["efektif"], "kedaluwarsa tidak ikut izin efektif; terurut")
		langsung := d["langsung"].([]any)
		require.Len(t, langsung, 2)
		assert.Equal(t, true, langsung[0].(map[string]any)["aktif"])
		assert.Equal(t, "Admin Satu", langsung[0].(map[string]any)["diberikan_oleh_nama"])
		assert.Equal(t, false, langsung[1].(map[string]any)["aktif"])
	})
	t.Run("tanpa izin langsung: larik kosong, bukan null", func(t *testing.T) {
		r, _ := routerPengguna(t, &repoUji{listLangsung: func(string) (*domain.IzinPengguna, error) {
			return &domain.IzinPengguna{Peran: "OPS_MAHASISWA"}, nil
		}})
		h := panggilJSON(t, r, nethttp.MethodGet, "/users/u-1/permissions", "")
		require.Equal(t, nethttp.StatusOK, h.Code, h.Body.String())
		d := h.data(t)
		assert.Equal(t, []any{}, d["langsung"])
		assert.Equal(t, []any{}, d["efektif"])
	})
	t.Run("pengguna tidak ada", func(t *testing.T) {
		r, _ := routerPengguna(t, &repoUji{listLangsung: func(string) (*domain.IzinPengguna, error) { return nil, gorm.ErrRecordNotFound }})
		assert.Equal(t, nethttp.StatusNotFound, panggilJSON(t, r, nethttp.MethodGet, "/users/hilang/permissions", "").Code)
	})
	t.Run("galat basis data", func(t *testing.T) {
		r, _ := routerPengguna(t, &repoUji{listLangsung: func(string) (*domain.IzinPengguna, error) { return nil, galatDB() }})
		h := panggilJSON(t, r, nethttp.MethodGet, "/users/u-1/permissions", "")
		assert.Equal(t, nethttp.StatusInternalServerError, h.Code)
		tanpaBocor(t, h)
	})
}

// ── POST /users/:id/permissions ───────────────────────────────────────

func TestGrantUserPermission_ValidasiDanPemetaanGalat(t *testing.T) {
	besok := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	kemarin := time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339)
	galatRepo := func(err error) *repoUji {
		return &repoUji{berikanIzin: func(domain.BerikanIzin) (*domain.IzinLangsungDetail, *domain.IzinLangsung, error) {
			return nil, nil, err
		}}
	}
	cases := []struct {
		nama   string
		repo   *repoUji
		badan  string
		status int
		kode   string
	}{
		{"JSON rusak", &repoUji{}, `{"izin":`, 400, "400"},
		{"izin hilang", &repoUji{}, `{"alasan":"x"}`, 400, "400"},
		{"alasan hilang", &repoUji{}, `{"izin":"harga.lihat"}`, 400, "400"},
		{"izin kepanjangan", &repoUji{}, `{"izin":"` + strings.Repeat("a", 65) + `","alasan":"x"}`, 400, "400"},
		{"waktu bukan RFC 3339", &repoUji{}, `{"izin":"harga.lihat","alasan":"x","berlaku_sampai":"besok"}`, 400, "400"},
		{"izin di luar katalog", &repoUji{}, `{"izin":"sistem.rusak","alasan":"x"}`, 400, "UNKNOWN_PERMISSION"},
		{"alasan hanya spasi", &repoUji{}, `{"izin":"harga.ekspor","alasan":"   "}`, 400, "400"},
		{"alasan lebih dari 500 karakter", &repoUji{}, `{"izin":"harga.ekspor","alasan":"` + strings.Repeat("a", 501) + `"}`, 400, "400"},
		{"masa berlaku sudah lewat", &repoUji{}, `{"izin":"harga.ekspor","alasan":"x","berlaku_sampai":"` + kemarin + `"}`, 400, "400"},
		{"pengguna tidak ada", galatRepo(gorm.ErrRecordNotFound), `{"izin":"harga.ekspor","alasan":"x"}`, 404, "404"},
		{"izin sudah dimiliki lewat peran", galatRepo(domain.ErrIzinDariPeran), `{"izin":"harga.ekspor","alasan":"x"}`, 409, "PERMISSION_ALREADY_VIA_ROLE"},
		{"izin langsung masih berlaku", galatRepo(domain.ErrIzinLangsungAda), `{"izin":"harga.ekspor","alasan":"x"}`, 409, "DIRECT_PERMISSION_EXISTS"},
		{"nama ditolak basis data", galatRepo(domain.ErrIzinTidakDikenal), `{"izin":"harga.ekspor","alasan":"x"}`, 400, "UNKNOWN_PERMISSION"},
		{"galat basis data", galatRepo(galatDB()), `{"izin":"harga.ekspor","alasan":"x","berlaku_sampai":"` + besok + `"}`, 500, "500"},
	}
	for _, tc := range cases {
		t.Run(tc.nama, func(t *testing.T) {
			r, rec := routerPengguna(t, tc.repo)
			h := panggilJSON(t, r, nethttp.MethodPost, "/users/u-1/permissions", tc.badan)
			assert.Equal(t, tc.status, h.Code, h.Body.String())
			assert.Equal(t, tc.kode, h.kode())
			tanpaBocor(t, h)
			assert.Empty(t, rec.Entries(), "pemberian yang ditolak tidak diaudit")
		})
	}
}

func TestGrantUserPermission_BerhasilMengauditSebelumSesudah(t *testing.T) {
	depan := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)
	lama := time.Now().Add(-time.Hour)
	var diterima domain.BerikanIzin
	r, rec := routerPengguna(t, &repoUji{berikanIzin: func(g domain.BerikanIzin) (*domain.IzinLangsungDetail, *domain.IzinLangsung, error) {
		diterima = g
		pemberi := g.Oleh
		return &domain.IzinLangsungDetail{
				IzinLangsung: domain.IzinLangsung{PenggunaID: g.PenggunaID, PermissionNama: g.Izin, BerlakuSampai: g.BerlakuSampai, Alasan: g.Alasan, DiberikanOleh: &pemberi, DiberikanPada: time.Now()},
				Deskripsi:    "Ekspor harga", Kelompok: "Harga",
			}, // baris kedaluwarsa yang digantikan ikut tercatat sebagai "sebelum"
			&domain.IzinLangsung{PenggunaID: g.PenggunaID, PermissionNama: g.Izin, BerlakuSampai: &lama, Alasan: "dulu", DiberikanPada: lama.Add(-time.Hour)}, nil
	}})

	h := panggilJSON(t, r, nethttp.MethodPost, "/users/u-1/permissions",
		`{"izin":"harga.ekspor","alasan":"  tugas audit  ","berlaku_sampai":"`+depan.Format(time.RFC3339)+`"}`)

	require.Equal(t, nethttp.StatusCreated, h.Code, h.Body.String())
	d := h.data(t)
	assert.Equal(t, "harga.ekspor", d["izin"])
	assert.Equal(t, "tugas audit", d["alasan"], "alasan dipangkas")
	assert.Equal(t, true, d["aktif"])
	assert.Equal(t, "admin-1", d["diberikan_oleh"], "pemberi dari token, bukan dari badan permintaan")
	assert.Equal(t, "u-1", diterima.PenggunaID)
	require.NotNil(t, diterima.BerlakuSampai)
	assert.True(t, diterima.BerlakuSampai.Equal(depan))

	e, ok := rec.Last()
	require.True(t, ok)
	assert.Equal(t, "GRANT_USER_PERMISSION", e.Aksi)
	assert.Equal(t, "u-1", e.EntitasID)
	assert.NotNil(t, e.Sebelum, "baris kedaluwarsa yang diganti ikut tercatat")
	assert.Equal(t, "tugas audit", snap(e.Sesudah)["alasan"])
}

func TestGrantUserPermission_TanpaMasaBerlakuAdalahTanpaBatas(t *testing.T) {
	r, _ := routerPengguna(t, &repoUji{berikanIzin: func(g domain.BerikanIzin) (*domain.IzinLangsungDetail, *domain.IzinLangsung, error) {
		assert.Nil(t, g.BerlakuSampai)
		return &domain.IzinLangsungDetail{IzinLangsung: domain.IzinLangsung{PenggunaID: g.PenggunaID, PermissionNama: g.Izin, Alasan: g.Alasan, DiberikanPada: time.Now()}}, nil, nil
	}})

	h := panggilJSON(t, r, nethttp.MethodPost, "/users/u-1/permissions", `{"izin":"harga.ekspor","alasan":"permanen"}`)

	require.Equal(t, nethttp.StatusCreated, h.Code, h.Body.String())
	assert.Nil(t, h.data(t)["berlaku_sampai"])
	assert.Equal(t, true, h.data(t)["aktif"])
}

// ── DELETE /users/:id/permissions/:izin ───────────────────────────────

func TestRevokeUserPermission_Handler(t *testing.T) {
	t.Run("berhasil", func(t *testing.T) {
		var u, izin string
		r, rec := routerPengguna(t, &repoUji{cabutIzin: func(id, i string) (*domain.IzinLangsung, error) {
			u, izin = id, i
			return &domain.IzinLangsung{PenggunaID: id, PermissionNama: i, Alasan: "audit", DiberikanPada: time.Now()}, nil
		}})
		h := panggilJSON(t, r, nethttp.MethodDelete, "/users/u-1/permissions/harga.ekspor", "")
		require.Equal(t, nethttp.StatusOK, h.Code, h.Body.String())
		assert.Equal(t, map[string]any{"id": "u-1", "izin": "harga.ekspor"}, h.data(t))
		assert.Equal(t, "u-1", u)
		assert.Equal(t, "harga.ekspor", izin)
		e, ok := rec.Last()
		require.True(t, ok)
		assert.Equal(t, "REVOKE_USER_PERMISSION", e.Aksi)
		assert.Equal(t, "audit", snap(e.Sebelum)["alasan"])
	})
	t.Run("tidak ada izin langsung untuk dicabut", func(t *testing.T) {
		r, rec := routerPengguna(t, &repoUji{cabutIzin: func(string, string) (*domain.IzinLangsung, error) { return nil, domain.ErrIzinLangsungTidakAda }})
		h := panggilJSON(t, r, nethttp.MethodDelete, "/users/u-1/permissions/harga.ekspor", "")
		assert.Equal(t, nethttp.StatusNotFound, h.Code)
		assert.Equal(t, "DIRECT_PERMISSION_NOT_FOUND", h.kode())
		assert.Empty(t, rec.Entries())
	})
	t.Run("galat basis data", func(t *testing.T) {
		r, rec := routerPengguna(t, &repoUji{cabutIzin: func(string, string) (*domain.IzinLangsung, error) { return nil, galatDB() }})
		h := panggilJSON(t, r, nethttp.MethodDelete, "/users/u-1/permissions/harga.ekspor", "")
		assert.Equal(t, nethttp.StatusInternalServerError, h.Code)
		tanpaBocor(t, h)
		assert.Empty(t, rec.Entries())
	})
}
