package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/audit/audittest"
	"github.com/garda-inflasi/api/internal/intervensi/domain"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type mockRepository struct {
	mock.Mock
}

func (m *mockRepository) List(ctx context.Context, q domain.ListQuery) ([]domain.Kartu, error) {
	args := m.Called(ctx, q)
	return args.Get(0).([]domain.Kartu), args.Error(1)
}

func (m *mockRepository) ListTahap(ctx context.Context) ([]domain.TahapDef, error) {
	args := m.Called(ctx)
	return args.Get(0).([]domain.TahapDef), args.Error(1)
}

func (m *mockRepository) GetByID(ctx context.Context, id string) (*domain.Intervensi, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Intervensi), args.Error(1)
}

func (m *mockRepository) Create(ctx context.Context, in domain.NewIntervensi, tahap string) (*domain.Intervensi, error) {
	args := m.Called(ctx, in, tahap)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Intervensi), args.Error(1)
}

func (m *mockRepository) Move(ctx context.Context, id, tahap string, index int, selesai bool) error {
	return m.Called(ctx, id, tahap, index, selesai).Error(0)
}

func newTestUseCase(repo *mockRepository) (*UseCase, *audittest.Recorder) {
	recorder := &audittest.Recorder{}
	return New(repo, recorder, logger.NewDefault()), recorder
}

func defs() []domain.TahapDef {
	return []domain.TahapDef{
		{Siklus: "TEKANAN", KodeTahap: "t-ews", Urutan: 1, IzinDibutuhkan: domain.IzinBawaan},
		{Siklus: "TEKANAN", KodeTahap: "t-petakan", Urutan: 2, IzinDibutuhkan: "intervensi.setujui"},
		{Siklus: "TEKANAN", KodeTahap: "t-selesai", Urutan: 3, IzinDibutuhkan: "intervensi.setujui"},
		{Siklus: "NORMAL", KodeTahap: "n-pantau", Urutan: 1, IzinDibutuhkan: domain.IzinBawaan},
	}
}

func allow(perms ...string) func(string) bool {
	return func(p string) bool {
		for _, x := range perms {
			if x == p {
				return true
			}
		}
		return false
	}
}

func TestCreate_PutsCardInFirstTahapOfItsSiklusAndAudits(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	in := domain.NewIntervensi{Siklus: "TEKANAN", Judul: "Operasi pasar", Prioritas: "TINGGI", Tenggat: time.Now(), DibuatOlehID: "u1"}
	repo.On("ListTahap", mock.Anything).Return(defs(), nil)
	repo.On("Create", mock.Anything, in, "t-ews").Return(&domain.Intervensi{ID: "INT-1000", Siklus: "TEKANAN", Tahap: "t-ews", Judul: "Operasi pasar", Prioritas: "TINGGI"}, nil)

	created, err := uc.Create(context.Background(), in)

	require.NoError(t, err)
	assert.Equal(t, "INT-1000", created.ID)
	entry, ok := recorder.Last()
	require.True(t, ok)
	assert.Equal(t, "BUAT", entry.Aksi)
	assert.Equal(t, "INT-1000", entry.EntitasID)
	assert.Equal(t, audit.HasilBerhasil, entry.Hasil)
}

func TestCreate_SiklusWithoutTahapIsRejected(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	repo.On("ListTahap", mock.Anything).Return(defs(), nil)

	_, err := uc.Create(context.Background(), domain.NewIntervensi{Siklus: "SURPLUS"})

	assert.ErrorIs(t, err, ErrInvalidTahap)
	assert.Empty(t, recorder.Entries())
}

func TestCreate_InvalidReferenceIsPassedThroughWithoutAudit(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	repo.On("ListTahap", mock.Anything).Return(defs(), nil)
	repo.On("Create", mock.Anything, mock.Anything, "t-ews").Return(nil, domain.ErrInvalidReference)

	_, err := uc.Create(context.Background(), domain.NewIntervensi{Siklus: "TEKANAN"})

	assert.ErrorIs(t, err, ErrInvalidReference)
	assert.Empty(t, recorder.Entries())
}

func TestMove(t *testing.T) {
	card := &domain.Intervensi{ID: "INT-301", Siklus: "TEKANAN", Tahap: "t-ews", Urutan: 0}

	t.Run("pindah ke tahap biasa dengan izin kelola", func(t *testing.T) {
		repo := new(mockRepository)
		uc, recorder := newTestUseCase(repo)
		repo.On("GetByID", mock.Anything, "INT-301").Return(card, nil)
		repo.On("ListTahap", mock.Anything).Return(defs(), nil)
		repo.On("Move", mock.Anything, "INT-301", "t-ews", 2, false).Return(nil)

		err := uc.Move(context.Background(), "INT-301", "t-ews", 2, "u1", allow(domain.IzinBawaan))

		require.NoError(t, err)
		entry, _ := recorder.Last()
		assert.Equal(t, "PINDAH_TAHAP", entry.Aksi)
		assert.Equal(t, audit.HasilBerhasil, entry.Hasil)
	})

	t.Run("tahap penutup menandai selesai", func(t *testing.T) {
		repo := new(mockRepository)
		uc, _ := newTestUseCase(repo)
		repo.On("GetByID", mock.Anything, "INT-301").Return(card, nil)
		repo.On("ListTahap", mock.Anything).Return(defs(), nil)
		repo.On("Move", mock.Anything, "INT-301", "t-selesai", 0, true).Return(nil)

		require.NoError(t, uc.Move(context.Background(), "INT-301", "t-selesai", 0, "u1", allow("intervensi.setujui")))
		repo.AssertExpectations(t)
	})

	t.Run("izin tahap tujuan tidak terpenuhi ditolak dan dicatat", func(t *testing.T) {
		repo := new(mockRepository)
		uc, recorder := newTestUseCase(repo)
		repo.On("GetByID", mock.Anything, "INT-301").Return(card, nil)
		repo.On("ListTahap", mock.Anything).Return(defs(), nil)

		err := uc.Move(context.Background(), "INT-301", "t-petakan", 0, "u1", allow(domain.IzinBawaan))

		var forbidden *ForbiddenError
		require.ErrorAs(t, err, &forbidden)
		assert.Equal(t, "intervensi.setujui", forbidden.Izin)
		repo.AssertNotCalled(t, "Move", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		entry, _ := recorder.Last()
		assert.Equal(t, audit.HasilDitolak, entry.Hasil)
	})

	t.Run("tahap milik siklus lain ditolak", func(t *testing.T) {
		repo := new(mockRepository)
		uc, _ := newTestUseCase(repo)
		repo.On("GetByID", mock.Anything, "INT-301").Return(card, nil)
		repo.On("ListTahap", mock.Anything).Return(defs(), nil)

		err := uc.Move(context.Background(), "INT-301", "n-pantau", 0, "u1", allow(domain.IzinBawaan))

		assert.ErrorIs(t, err, ErrInvalidTahap)
	})

	t.Run("kartu tidak ada", func(t *testing.T) {
		repo := new(mockRepository)
		uc, _ := newTestUseCase(repo)
		repo.On("GetByID", mock.Anything, "X").Return(nil, gorm.ErrRecordNotFound)

		err := uc.Move(context.Background(), "X", "t-ews", 0, "u1", allow(domain.IzinBawaan))

		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("kartu hilang saat dipindah menjadi tidak ditemukan", func(t *testing.T) {
		repo := new(mockRepository)
		uc, recorder := newTestUseCase(repo)
		repo.On("GetByID", mock.Anything, "INT-301").Return(card, nil)
		repo.On("ListTahap", mock.Anything).Return(defs(), nil)
		repo.On("Move", mock.Anything, "INT-301", "t-ews", 0, false).Return(domain.ErrNotFound)

		err := uc.Move(context.Background(), "INT-301", "t-ews", 0, "u1", allow(domain.IzinBawaan))

		assert.ErrorIs(t, err, ErrNotFound)
		assert.Empty(t, recorder.Entries())
	})

	t.Run("galat repository diteruskan tanpa audit", func(t *testing.T) {
		repo := new(mockRepository)
		uc, recorder := newTestUseCase(repo)
		repo.On("GetByID", mock.Anything, "INT-301").Return(card, nil)
		repo.On("ListTahap", mock.Anything).Return(defs(), nil)
		repo.On("Move", mock.Anything, "INT-301", "t-ews", 0, false).Return(assert.AnError)

		err := uc.Move(context.Background(), "INT-301", "t-ews", 0, "u1", allow(domain.IzinBawaan))

		assert.ErrorIs(t, err, assert.AnError)
		assert.Empty(t, recorder.Entries())
	})
}

func TestListAndListTahap(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	q := domain.ListQuery{Siklus: "TEKANAN"}
	repo.On("List", mock.Anything, q).Return([]domain.Kartu{{Intervensi: domain.Intervensi{ID: "INT-301"}}}, nil)
	repo.On("ListTahap", mock.Anything).Return(defs(), nil)

	cards, err := uc.List(context.Background(), q)
	require.NoError(t, err)
	assert.Len(t, cards, 1)

	tahap, err := uc.ListTahap(context.Background())
	require.NoError(t, err)
	assert.Len(t, tahap, 4)
}
