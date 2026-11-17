package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func sesiRiwayat(id string, mulai time.Time, status string) domain.RiwayatSesi {
	return domain.RiwayatSesi{ID: id, MulaiPada: mulai, TerakhirDipakaiPada: mulai, Status: status}
}

func TestListSessionHistory_BatasDanHalamanBerikutnya(t *testing.T) {
	repo := new(mockRepository)
	uc := &UseCase{repo: repo}
	t0 := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	s1, s2, s3 := "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222", "33333333-3333-4333-8333-333333333333"
	repo.On("ListSessionHistory", mock.Anything, "u1", mock.MatchedBy(func(f domain.RiwayatFilter) bool {
		return f.Limit == 3 && f.Status == domain.RiwayatSemua && f.After == nil
	}), mock.Anything).Return([]domain.RiwayatSesi{
		sesiRiwayat(s1, t0.Add(2*time.Hour), domain.SesiAktif),
		sesiRiwayat(s2, t0.Add(time.Hour), domain.SesiDicabut),
		sesiRiwayat(s3, t0, domain.SesiKedaluwarsa),
	}, nil)

	got, err := uc.ListSessionHistory(WithSession(context.Background(), s1), "u1", "", 2, "")

	require.NoError(t, err)
	require.Len(t, got.Items, 2, "baris ekstra dibuang")
	assert.True(t, got.Items[0].Current)
	assert.False(t, got.Items[1].Current)
	require.NotEmpty(t, got.NextCursor)

	// Cursor menunjuk ke sesi terakhir halaman ini.
	cursor, err := decodeCursorRiwayat(got.NextCursor)
	require.NoError(t, err)
	assert.Equal(t, s2, cursor.ID)
	assert.True(t, cursor.MulaiPada.Equal(t0.Add(time.Hour)))
}

func TestListSessionHistory_HalamanTerakhirTanpaCursor(t *testing.T) {
	repo := new(mockRepository)
	uc := &UseCase{repo: repo}
	repo.On("ListSessionHistory", mock.Anything, "u1", mock.Anything, mock.Anything).
		Return([]domain.RiwayatSesi{sesiRiwayat("11111111-1111-4111-8111-111111111111", time.Now(), domain.SesiAktif)}, nil)

	got, err := uc.ListSessionHistory(context.Background(), "u1", domain.RiwayatAktif, 0, "")

	require.NoError(t, err)
	assert.Empty(t, got.NextCursor)
	assert.False(t, got.Items[0].Current, "tanpa sid di konteks tidak ada sesi yang ditandai")
	repo.AssertCalled(t, "ListSessionHistory", mock.Anything, "u1", mock.MatchedBy(func(f domain.RiwayatFilter) bool {
		return f.Limit == DefaultLimitRiwayatSesi+1 && f.Status == domain.RiwayatAktif
	}), mock.Anything)
}

func TestListSessionHistory_MasukanTidakValidDitolakTanpaMenyentuhRepository(t *testing.T) {
	repo := new(mockRepository)
	uc := &UseCase{repo: repo}

	for _, tc := range []struct{ status, cursor string }{
		{"selesai", ""},
		{"", "bukan-cursor"},
		{"", "MjAyNi0xMS0wMVQxMDowMDowMFp8Ym9rYW4tdXVpZA"}, // waktu sah, id bukan UUID
	} {
		_, err := uc.ListSessionHistory(context.Background(), "u1", tc.status, 10, tc.cursor)
		assert.ErrorIs(t, err, ErrRiwayatSesiTidakValid, "%+v", tc)
	}
	repo.AssertNotCalled(t, "ListSessionHistory", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}
