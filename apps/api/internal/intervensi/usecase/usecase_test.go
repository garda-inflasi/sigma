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

func (m *mockRepository) Move(ctx context.Context, id, tahap string, index int, selesai bool, versi int, oleh string) (int, error) {
	args := m.Called(ctx, id, tahap, index, selesai, versi, oleh)
	return args.Int(0), args.Error(1)
}

func (m *mockRepository) BulkAksi(ctx context.Context, aksi domain.BulkAksi, ids []string, oleh string) ([]domain.HasilKartu, error) {
	args := m.Called(ctx, aksi, ids, oleh)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.HasilKartu), args.Error(1)
}

func (m *mockRepository) ListRiwayat(ctx context.Context, id string) ([]domain.Riwayat, error) {
	args := m.Called(ctx, id)
	return args.Get(0).([]domain.Riwayat), args.Error(1)
}

func newTestUseCase(repo *mockRepository) (*UseCase, *audittest.Recorder) {
	recorder := &audittest.Recorder{}
	return New(repo, nil, recorder, logger.NewDefault()), recorder
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
	in := domain.NewIntervensi{Siklus: "TEKANAN", Judul: "Operasi pasar", Prioritas: "TINGGI", Tenggat: time.Now(), DibuatOlehID: uid("u1")}
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
	card := &domain.Intervensi{ID: "INT-301", Siklus: "TEKANAN", Tahap: "t-ews", Urutan: 0, Versi: 3}
	noMove := func(repo *mockRepository) {
		repo.AssertNotCalled(t, "Move", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	}

	t.Run("pindah ke tahap biasa dengan izin kelola mengembalikan versi baru", func(t *testing.T) {
		repo := new(mockRepository)
		uc, recorder := newTestUseCase(repo)
		repo.On("GetByID", mock.Anything, "INT-301").Return(card, nil)
		repo.On("ListTahap", mock.Anything).Return(defs(), nil)
		repo.On("Move", mock.Anything, "INT-301", "t-ews", 2, false, 3, uid("u1")).Return(4, nil)

		versi, err := uc.Move(context.Background(), "INT-301", "t-ews", 2, 3, uid("u1"), allow(domain.IzinBawaan))

		require.NoError(t, err)
		assert.Equal(t, 4, versi)
		entry, _ := recorder.Last()
		assert.Equal(t, "PINDAH_TAHAP", entry.Aksi)
		assert.Equal(t, audit.HasilBerhasil, entry.Hasil)
		assert.Equal(t, 3, entry.Sebelum.(map[string]any)["versi"])
		assert.Equal(t, 4, entry.Sesudah.(map[string]any)["versi"])
	})

	t.Run("tahap penutup menandai selesai", func(t *testing.T) {
		repo := new(mockRepository)
		uc, _ := newTestUseCase(repo)
		repo.On("GetByID", mock.Anything, "INT-301").Return(card, nil)
		repo.On("ListTahap", mock.Anything).Return(defs(), nil)
		repo.On("Move", mock.Anything, "INT-301", "t-selesai", 0, true, 3, uid("u1")).Return(4, nil)

		_, err := uc.Move(context.Background(), "INT-301", "t-selesai", 0, 3, uid("u1"), allow("intervensi.setujui"))

		require.NoError(t, err)
		repo.AssertExpectations(t)
	})

	t.Run("izin tahap tujuan tidak terpenuhi ditolak dan dicatat", func(t *testing.T) {
		repo := new(mockRepository)
		uc, recorder := newTestUseCase(repo)
		repo.On("GetByID", mock.Anything, "INT-301").Return(card, nil)
		repo.On("ListTahap", mock.Anything).Return(defs(), nil)

		_, err := uc.Move(context.Background(), "INT-301", "t-petakan", 0, 3, uid("u1"), allow(domain.IzinBawaan))

		var forbidden *ForbiddenError
		require.ErrorAs(t, err, &forbidden)
		assert.Equal(t, "intervensi.setujui", forbidden.Izin)
		assert.Empty(t, forbidden.Tahap)
		assert.Equal(t, "tahap tujuan memerlukan izin intervensi.setujui", forbidden.Error())
		noMove(repo)
		entry, _ := recorder.Last()
		assert.Equal(t, audit.HasilDitolak, entry.Hasil)
	})

	t.Run("tahap persetujuan tidak boleh dilompati tanpa izinnya", func(t *testing.T) {
		// t-ews (1) -> t-akhir (3) melompati t-petakan (2, butuh setujui); tahap tujuan sendiri tidak butuh izin khusus.
		tahap := []domain.TahapDef{
			{Siklus: "TEKANAN", KodeTahap: "t-ews", Urutan: 1, IzinDibutuhkan: domain.IzinBawaan},
			{Siklus: "TEKANAN", KodeTahap: "t-petakan", Urutan: 2, Label: "Petakan suplai", IzinDibutuhkan: "intervensi.setujui"},
			{Siklus: "TEKANAN", KodeTahap: "t-akhir", Urutan: 3, IzinDibutuhkan: domain.IzinBawaan},
		}
		repo := new(mockRepository)
		uc, recorder := newTestUseCase(repo)
		repo.On("GetByID", mock.Anything, "INT-301").Return(card, nil)
		repo.On("ListTahap", mock.Anything).Return(tahap, nil)

		_, err := uc.Move(context.Background(), "INT-301", "t-akhir", 0, 3, uid("u1"), allow(domain.IzinBawaan))

		var forbidden *ForbiddenError
		require.ErrorAs(t, err, &forbidden)
		assert.Equal(t, "intervensi.setujui", forbidden.Izin)
		assert.Equal(t, "Petakan suplai", forbidden.Tahap)
		assert.Contains(t, forbidden.Error(), `melewati tahap "Petakan suplai"`)
		noMove(repo)
		entry, _ := recorder.Last()
		assert.Equal(t, audit.HasilDitolak, entry.Hasil)
		assert.Equal(t, "Petakan suplai", entry.Metadata["dilewati"])

		// Pemegang izin tahap itu boleh melompatinya.
		repo.On("Move", mock.Anything, "INT-301", "t-akhir", 0, true, 3, uid("u1")).Return(4, nil)
		_, err = uc.Move(context.Background(), "INT-301", "t-akhir", 0, 3, uid("u1"), allow(domain.IzinBawaan, "intervensi.setujui"))
		assert.NoError(t, err)
	})

	t.Run("mundur melewati tahap persetujuan tidak dianggap melompat", func(t *testing.T) {
		repo := new(mockRepository)
		uc, _ := newTestUseCase(repo)
		maju := &domain.Intervensi{ID: "INT-302", Siklus: "TEKANAN", Tahap: "t-selesai", Versi: 1}
		repo.On("GetByID", mock.Anything, "INT-302").Return(maju, nil)
		repo.On("ListTahap", mock.Anything).Return(defs(), nil)
		repo.On("Move", mock.Anything, "INT-302", "t-ews", 0, false, 1, uid("u1")).Return(2, nil)

		_, err := uc.Move(context.Background(), "INT-302", "t-ews", 0, 1, uid("u1"), allow(domain.IzinBawaan))

		require.NoError(t, err)
	})

	t.Run("versi usang diteruskan sebagai konflik tanpa audit", func(t *testing.T) {
		repo := new(mockRepository)
		uc, recorder := newTestUseCase(repo)
		repo.On("GetByID", mock.Anything, "INT-301").Return(card, nil)
		repo.On("ListTahap", mock.Anything).Return(defs(), nil)
		repo.On("Move", mock.Anything, "INT-301", "t-ews", 0, false, 2, uid("u1")).Return(0, domain.ErrVersiUsang)

		_, err := uc.Move(context.Background(), "INT-301", "t-ews", 0, 2, uid("u1"), allow(domain.IzinBawaan))

		assert.ErrorIs(t, err, ErrVersiUsang)
		assert.Empty(t, recorder.Entries())
	})

	t.Run("tahap milik siklus lain ditolak", func(t *testing.T) {
		repo := new(mockRepository)
		uc, _ := newTestUseCase(repo)
		repo.On("GetByID", mock.Anything, "INT-301").Return(card, nil)
		repo.On("ListTahap", mock.Anything).Return(defs(), nil)

		_, err := uc.Move(context.Background(), "INT-301", "n-pantau", 0, 3, uid("u1"), allow(domain.IzinBawaan))

		assert.ErrorIs(t, err, ErrInvalidTahap)
	})

	t.Run("kartu tidak ada", func(t *testing.T) {
		repo := new(mockRepository)
		uc, _ := newTestUseCase(repo)
		repo.On("GetByID", mock.Anything, "X").Return(nil, gorm.ErrRecordNotFound)

		_, err := uc.Move(context.Background(), "X", "t-ews", 0, 1, uid("u1"), allow(domain.IzinBawaan))

		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("kartu hilang saat dipindah menjadi tidak ditemukan", func(t *testing.T) {
		repo := new(mockRepository)
		uc, recorder := newTestUseCase(repo)
		repo.On("GetByID", mock.Anything, "INT-301").Return(card, nil)
		repo.On("ListTahap", mock.Anything).Return(defs(), nil)
		repo.On("Move", mock.Anything, "INT-301", "t-ews", 0, false, 3, uid("u1")).Return(0, domain.ErrNotFound)

		_, err := uc.Move(context.Background(), "INT-301", "t-ews", 0, 3, uid("u1"), allow(domain.IzinBawaan))

		assert.ErrorIs(t, err, ErrNotFound)
		assert.Empty(t, recorder.Entries())
	})

	t.Run("galat repository diteruskan tanpa audit", func(t *testing.T) {
		repo := new(mockRepository)
		uc, recorder := newTestUseCase(repo)
		repo.On("GetByID", mock.Anything, "INT-301").Return(card, nil)
		repo.On("ListTahap", mock.Anything).Return(defs(), nil)
		repo.On("Move", mock.Anything, "INT-301", "t-ews", 0, false, 3, uid("u1")).Return(0, assert.AnError)

		_, err := uc.Move(context.Background(), "INT-301", "t-ews", 0, 3, uid("u1"), allow(domain.IzinBawaan))

		assert.ErrorIs(t, err, assert.AnError)
		assert.Empty(t, recorder.Entries())
	})
}

func TestBulkAksi(t *testing.T) {
	t.Run("hasil per kartu dihitung dan hanya yang berhasil diaudit", func(t *testing.T) {
		repo := new(mockRepository)
		uc, recorder := newTestUseCase(repo)
		hasil := []domain.HasilKartu{{ID: "INT-1", Berhasil: true}, {ID: "INT-2", Alasan: "Kartu sudah selesai"}, {ID: "INT-3", Berhasil: true}}
		repo.On("BulkAksi", mock.Anything, domain.BulkTugaskan, []string{"INT-1", "INT-2", "INT-3"}, uid("u1")).Return(hasil, nil)

		res, err := uc.BulkAksi(context.Background(), "tugaskan", []string{"INT-1", "INT-2", "INT-1", "INT-3"}, uid("u1"))

		require.NoError(t, err)
		assert.Equal(t, 2, res.Berhasil)
		assert.Equal(t, 1, res.Gagal)
		assert.Equal(t, hasil, res.Hasil)
		entry, ok := recorder.Last()
		require.True(t, ok)
		assert.Equal(t, "BULK_TUGASKAN", entry.Aksi)
		assert.Equal(t, "2 kartu", entry.EntitasID)
		assert.Equal(t, []string{"INT-1", "INT-3"}, entry.Metadata["ids"])
	})

	t.Run("tanpa perubahan tidak diaudit", func(t *testing.T) {
		repo := new(mockRepository)
		uc, recorder := newTestUseCase(repo)
		repo.On("BulkAksi", mock.Anything, domain.BulkEskalasi, []string{"INT-9"}, uid("u1")).Return([]domain.HasilKartu{{ID: "INT-9", Alasan: "Kartu tidak ditemukan"}}, nil)

		res, err := uc.BulkAksi(context.Background(), "eskalasi", []string{"INT-9"}, uid("u1"))

		require.NoError(t, err)
		assert.Equal(t, 1, res.Gagal)
		assert.Empty(t, recorder.Entries())
	})

	t.Run("aksi tidak dikenal ditolak", func(t *testing.T) {
		uc, _ := newTestUseCase(new(mockRepository))
		_, err := uc.BulkAksi(context.Background(), "hapus", []string{"INT-1"}, uid("u1"))
		assert.ErrorIs(t, err, ErrInvalidAction)
	})

	t.Run("galat repository diteruskan", func(t *testing.T) {
		repo := new(mockRepository)
		uc, recorder := newTestUseCase(repo)
		repo.On("BulkAksi", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, assert.AnError)

		_, err := uc.BulkAksi(context.Background(), "tugaskan", []string{"INT-1"}, uid("u1"))

		assert.ErrorIs(t, err, assert.AnError)
		assert.Empty(t, recorder.Entries())
	})
}

func TestListRiwayat(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	repo.On("GetByID", mock.Anything, "X").Return(nil, gorm.ErrRecordNotFound)
	_, err := uc.ListRiwayat(context.Background(), "X")
	assert.ErrorIs(t, err, ErrNotFound)

	repo.On("GetByID", mock.Anything, "INT-1").Return(&domain.Intervensi{ID: "INT-1"}, nil)
	repo.On("ListRiwayat", mock.Anything, "INT-1").Return([]domain.Riwayat{{IntervensiRiwayat: domain.IntervensiRiwayat{Aksi: "BUAT"}}}, nil)
	items, err := uc.ListRiwayat(context.Background(), "INT-1")
	require.NoError(t, err)
	assert.Len(t, items, 1)
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
