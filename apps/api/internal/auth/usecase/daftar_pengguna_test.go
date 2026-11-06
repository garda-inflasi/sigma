package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestListUsers_MenormalkanQueryDanMengisiJamKunci(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	sekarang := time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)
	asli := jam
	jam = func() time.Time { return sekarang }
	t.Cleanup(func() { jam = asli })

	var diterima domain.DaftarPenggunaQuery
	repo.On("ListUsers", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { diterima = args.Get(1).(domain.DaftarPenggunaQuery) }).
		Return([]domain.Pengguna{}, int64(0), nil)

	_, _, err := uc.ListUsers(context.Background(), domain.DaftarPenggunaQuery{Filter: domain.FilterPengguna{Cari: "  ani  ", Peran: "ADMIN_SISTEM"}})

	require.NoError(t, err)
	assert.Equal(t, "ani", diterima.Filter.Cari)
	assert.Equal(t, domain.UrutDibuatPada, diterima.Urut)
	assert.Equal(t, domain.ArahTurun, diterima.Arah)
	assert.Equal(t, 1, diterima.Page)
	assert.Equal(t, domain.LimitBawaanPengguna, diterima.Limit)
	assert.Equal(t, sekarang, diterima.Sekarang, "filter terkunci memakai jam usecase")
}

func TestListUsers_MenghormatiSekarangDariPemanggil(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	tetap := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	repo.On("ListUsers", mock.Anything, mock.MatchedBy(func(q domain.DaftarPenggunaQuery) bool { return q.Sekarang.Equal(tetap) })).
		Return([]domain.Pengguna{}, int64(0), nil)

	_, _, err := uc.ListUsers(context.Background(), domain.DaftarPenggunaQuery{Sekarang: tetap})

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestListUsers_MasukanSalahDitolakSebelumKeBasisData(t *testing.T) {
	cases := map[string]struct {
		q     domain.DaftarPenggunaQuery
		param string
	}{
		"peran tak dikenal":  {domain.DaftarPenggunaQuery{Filter: domain.FilterPengguna{Peran: "SUPERADMIN"}}, "peran"},
		"peran huruf kecil":  {domain.DaftarPenggunaQuery{Filter: domain.FilterPengguna{Peran: "admin_sistem"}}, "peran"},
		"status tak dikenal": {domain.DaftarPenggunaQuery{Filter: domain.FilterPengguna{Status: "BEKU"}}, "status"},
		"lapis tak dikenal":  {domain.DaftarPenggunaQuery{Filter: domain.FilterPengguna{Lapis: "OPS"}}, "lapis"},
		"sort tak dikenal":   {domain.DaftarPenggunaQuery{Urut: "password_hash"}, "sort"},
		"arah tak dikenal":   {domain.DaftarPenggunaQuery{Arah: "menurun"}, "arah"},
	}
	for nama, tc := range cases {
		t.Run(nama, func(t *testing.T) {
			repo := new(mockRepository)
			uc := newTestUseCase(repo)

			_, _, err := uc.ListUsers(context.Background(), tc.q)

			var galat *domain.FilterPenggunaError
			require.ErrorAs(t, err, &galat)
			assert.Equal(t, tc.param, galat.Param)
			repo.AssertNotCalled(t, "ListUsers", mock.Anything, mock.Anything)
		})
	}
}

func TestListUsers_SemuaPeranKatalogDiterima(t *testing.T) {
	for _, peran := range roleOrder {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		repo.On("ListUsers", mock.Anything, mock.Anything).Return([]domain.Pengguna{}, int64(0), nil)

		_, _, err := uc.ListUsers(context.Background(), domain.DaftarPenggunaQuery{Filter: domain.FilterPengguna{Peran: peran}})

		assert.NoError(t, err, peran)
	}
}

func TestListUsers_GalatRepositoriDiteruskanApaAdanya(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	repo.On("ListUsers", mock.Anything, mock.Anything).Return([]domain.Pengguna{}, int64(0), errors.New("db putus"))

	_, _, err := uc.ListUsers(context.Background(), domain.DaftarPenggunaQuery{})

	require.Error(t, err)
	var galat *domain.FilterPenggunaError
	assert.False(t, errors.As(err, &galat), "galat basis data bukan salah masukan (bukan 400)")
}

// Pengguna dengan peran dari enum basis data harus selalu lolos validasi: katalog peran backend sama dengan
// enum yang dipakai filter daftar.
func TestPeranDikenal_MengikutiKatalogPeran(t *testing.T) {
	for _, peran := range roleOrder {
		assert.True(t, peranDikenal(peran), peran)
	}
	assert.False(t, peranDikenal(""))
	assert.False(t, peranDikenal("ADMIN"))
}
