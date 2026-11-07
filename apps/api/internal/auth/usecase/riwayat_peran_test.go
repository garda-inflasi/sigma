package usecase

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func entriPeran(n int) []domain.PerubahanPeran {
	out := make([]domain.PerubahanPeran, n)
	for i := range out {
		out[i] = domain.PerubahanPeran{AuditID: fmt.Sprintf("a-%d", i), Waktu: time.Unix(int64(1000-i), 0), Aksi: "ASSIGN_ROLE", Ke: "MANAJER_DATA"}
	}
	return out
}

func TestRiwayatPeran_PenggunaTidakAdaDanGalatBasisDataDibedakan(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	repo.On("GetUserByID", mock.Anything, "hilang").Return(nil, fmt.Errorf("get user by id: %w", gorm.ErrRecordNotFound))
	repo.On("GetUserByID", mock.Anything, "rusak").Return(nil, errors.New("koneksi putus"))

	_, err := uc.RiwayatPeran(context.Background(), "hilang", 0)
	assert.ErrorIs(t, err, ErrUserNotFound)

	_, err = uc.RiwayatPeran(context.Background(), "rusak", 0)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrUserNotFound, "galat basis data bukan pengguna hilang")
	repo.AssertNotCalled(t, "RiwayatPeran", mock.Anything, mock.Anything, mock.Anything)
}

func TestRiwayatPeran_MembatasiDanMenandaiTerpotong(t *testing.T) {
	cases := []struct {
		nama          string
		limit         int
		wantRepoLimit int // limit + 1: satu baris ekstra penanda terpotong
		dikembalikan  int
		wantItems     int
		wantTerpotong bool
	}{
		{"bawaan 50 bila limit kosong", 0, 51, 3, 3, false},
		{"tepat habis bukan terpotong", 3, 4, 3, 3, false},
		{"ada lebih banyak", 3, 4, 4, 3, true},
		{"dibatasi maksimum 200", 5000, 201, 201, 200, true},
		{"limit negatif memakai bawaan", -4, 51, 0, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.nama, func(t *testing.T) {
			repo := new(mockRepository)
			uc := newTestUseCase(repo)
			repo.On("GetUserByID", mock.Anything, uid("u-1")).Return(activeUser(), nil)
			repo.On("RiwayatPeran", mock.Anything, uid("u-1"), tc.wantRepoLimit).Return(entriPeran(tc.dikembalikan), nil)

			hasil, err := uc.RiwayatPeran(context.Background(), uid("u-1"), tc.limit)

			require.NoError(t, err)
			assert.Len(t, hasil.Items, tc.wantItems)
			assert.Equal(t, tc.wantTerpotong, hasil.Terpotong)
			assert.NotNil(t, hasil.Items, "tanpa riwayat tetap larik kosong, bukan null di JSON")
			repo.AssertExpectations(t)
		})
	}
}

func TestRiwayatPeran_GalatRepositoriDibungkus(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	repo.On("GetUserByID", mock.Anything, uid("u-1")).Return(activeUser(), nil)
	repo.On("RiwayatPeran", mock.Anything, uid("u-1"), mock.Anything).Return(nil, errors.New("db putus"))

	_, err := uc.RiwayatPeran(context.Background(), uid("u-1"), 10)

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrUserNotFound)
}
