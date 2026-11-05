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

// mockWorkflow mengimplementasi domain.WorkflowRepository untuk metode yang dipakai usecase.
type mockWorkflow struct {
	domain.WorkflowRepository
	mock.Mock
}

func (m *mockWorkflow) ListAlur(ctx context.Context) ([]domain.Alur, error) {
	args := m.Called(ctx)
	return args.Get(0).([]domain.Alur), args.Error(1)
}

func (m *mockWorkflow) ListPersetujuan(ctx context.Context, status string) ([]domain.Persetujuan, error) {
	args := m.Called(ctx, status)
	return args.Get(0).([]domain.Persetujuan), args.Error(1)
}

func (m *mockWorkflow) Ajukan(ctx context.Context, in domain.AjukanInput) (*domain.Persetujuan, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Persetujuan), args.Error(1)
}

func (m *mockWorkflow) GetPersetujuan(ctx context.Context, id int64) (*domain.Persetujuan, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Persetujuan), args.Error(1)
}

func (m *mockWorkflow) Putus(ctx context.Context, in domain.PutusInput) (*domain.Persetujuan, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Persetujuan), args.Error(1)
}

func newWorkflowUseCase() (*UseCase, *mockRepository, *mockWorkflow, *audittest.Recorder) {
	repo, wf, rec := new(mockRepository), new(mockWorkflow), &audittest.Recorder{}
	return New(repo, wf, rec, logger.NewDefault()), repo, wf, rec
}

func strPtr(s string) *string { return &s }

func permintaan(status string, langkahKini int, pengaju string, keputusan ...string) *domain.Persetujuan {
	p := &domain.Persetujuan{IntervensiPersetujuan: domain.IntervensiPersetujuan{
		ID: 7, IntervensiID: "INT-301", Jenis: domain.JenisPindahTahap, TahapTujuan: strPtr("t-fiskal"),
		Status: status, LangkahKini: langkahKini, DiajukanOleh: strPtr(pengaju), DiajukanPada: time.Now(),
	}}
	izin := []string{"intervensi.setujui", "intervensi.setujui_fiskal"}
	for i, iz := range izin {
		l := domain.LangkahPersetujuan{PersetujuanLangkah: domain.PersetujuanLangkah{PersetujuanID: 7, Urutan: i + 1, Label: "L", IzinDibutuhkan: iz}}
		if i < len(keputusan) {
			l.Keputusan, l.Oleh = strPtr(domain.KeputusanSetuju), strPtr(keputusan[i])
		}
		p.Langkah = append(p.Langkah, l)
	}
	return p
}

func TestAjukan(t *testing.T) {
	card := &domain.Intervensi{ID: "INT-301", Siklus: "TEKANAN", Tahap: "t-ews"}

	t.Run("membuat permintaan dan mengaudit", func(t *testing.T) {
		uc, repo, wf, rec := newWorkflowUseCase()
		repo.On("GetByID", mock.Anything, "INT-301").Return(card, nil)
		repo.On("ListTahap", mock.Anything).Return(defs(), nil)
		in := domain.AjukanInput{KartuID: "INT-301", Jenis: domain.JenisPindahTahap, TahapTujuan: "t-petakan", Oleh: "u1", Catatan: "siap"}
		wf.On("Ajukan", mock.Anything, in).Return(permintaan(domain.PersetujuanMenunggu, 1, "u1"), nil)

		p, err := uc.Ajukan(context.Background(), "INT-301", "t-petakan", "siap", "u1")

		require.NoError(t, err)
		assert.Equal(t, int64(7), p.ID)
		entry, _ := rec.Last()
		assert.Equal(t, "AJUKAN_PERSETUJUAN", entry.Aksi)
		assert.Equal(t, "u1", entry.AktorID)
		assert.Equal(t, 2, entry.Metadata["langkah"])
	})

	t.Run("tahap di luar siklus ditolak sebelum menyentuh workflow", func(t *testing.T) {
		uc, repo, wf, _ := newWorkflowUseCase()
		repo.On("GetByID", mock.Anything, "INT-301").Return(card, nil)
		repo.On("ListTahap", mock.Anything).Return(defs(), nil)

		_, err := uc.Ajukan(context.Background(), "INT-301", "n-pantau", "", "u1")

		assert.ErrorIs(t, err, ErrInvalidTahap)
		wf.AssertNotCalled(t, "Ajukan", mock.Anything, mock.Anything)
	})

	t.Run("kartu tidak ada", func(t *testing.T) {
		uc, repo, _, _ := newWorkflowUseCase()
		repo.On("GetByID", mock.Anything, "X").Return(nil, gorm.ErrRecordNotFound)
		_, err := uc.Ajukan(context.Background(), "X", "t-petakan", "", "u1")
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("galat bisnis diteruskan tanpa audit", func(t *testing.T) {
		for _, bizErr := range []error{ErrTanpaPersetujuan, ErrPersetujuanTerbuka, ErrKartuSelesai} {
			uc, repo, wf, rec := newWorkflowUseCase()
			repo.On("GetByID", mock.Anything, "INT-301").Return(card, nil)
			repo.On("ListTahap", mock.Anything).Return(defs(), nil)
			wf.On("Ajukan", mock.Anything, mock.Anything).Return(nil, bizErr)

			_, err := uc.Ajukan(context.Background(), "INT-301", "t-petakan", "", "u1")

			assert.ErrorIs(t, err, bizErr)
			assert.Empty(t, rec.Entries())
		}
	})
}

func TestPutus(t *testing.T) {
	setujui := allow("intervensi.setujui")

	t.Run("penyetuju berizin memutus langkah yang menunggu", func(t *testing.T) {
		uc, _, wf, rec := newWorkflowUseCase()
		wf.On("GetPersetujuan", mock.Anything, int64(7)).Return(permintaan(domain.PersetujuanMenunggu, 1, "kelola"), nil)
		want := domain.PutusInput{PersetujuanID: 7, LangkahKini: 1, Keputusan: "SETUJU", Oleh: "setuju1", Catatan: "ok"}
		after := permintaan(domain.PersetujuanMenunggu, 2, "kelola", "setuju1")
		wf.On("Putus", mock.Anything, want).Return(after, nil)

		got, err := uc.Putus(context.Background(), 7, "SETUJU", " ok ", "setuju1", setujui)

		require.NoError(t, err)
		assert.Equal(t, 2, got.LangkahKini)
		entry, _ := rec.Last()
		assert.Equal(t, "PUTUS_PERSETUJUAN", entry.Aksi)
		assert.Equal(t, audit.HasilBerhasil, entry.Hasil)
		assert.Equal(t, "MENUNGGU", entry.Metadata["status"])
	})

	t.Run("tanpa izin langkah ditolak, dicatat, dan tidak diteruskan", func(t *testing.T) {
		uc, _, wf, rec := newWorkflowUseCase()
		wf.On("GetPersetujuan", mock.Anything, int64(7)).Return(permintaan(domain.PersetujuanMenunggu, 2, "kelola", "setuju1"), nil)

		_, err := uc.Putus(context.Background(), 7, "SETUJU", "", "setuju2", setujui) // langkah 2 butuh setujui_fiskal

		var forbidden *ForbiddenError
		require.ErrorAs(t, err, &forbidden)
		assert.Equal(t, "intervensi.setujui_fiskal", forbidden.Izin)
		wf.AssertNotCalled(t, "Putus", mock.Anything, mock.Anything)
		entry, _ := rec.Last()
		assert.Equal(t, audit.HasilDitolak, entry.Hasil)
	})

	t.Run("pemisahan tugas: pengaju atau pemutus sebelumnya", func(t *testing.T) {
		uc, _, wf, _ := newWorkflowUseCase()
		wf.On("GetPersetujuan", mock.Anything, int64(7)).Return(permintaan(domain.PersetujuanMenunggu, 1, "setuju1"), nil)
		_, err := uc.Putus(context.Background(), 7, "SETUJU", "", "setuju1", setujui)
		assert.ErrorIs(t, err, ErrPemisahanTugas, "pengaju memutus sendiri")

		uc, _, wf, _ = newWorkflowUseCase()
		wf.On("GetPersetujuan", mock.Anything, int64(7)).Return(permintaan(domain.PersetujuanMenunggu, 2, "kelola", "setuju1"), nil)
		_, err = uc.Putus(context.Background(), 7, "SETUJU", "", "setuju1", allow("intervensi.setujui", "intervensi.setujui_fiskal"))
		assert.ErrorIs(t, err, ErrPemisahanTugas, "memutus dua langkah")
		wf.AssertNotCalled(t, "Putus", mock.Anything, mock.Anything)
	})

	t.Run("penolakan wajib bercatatan", func(t *testing.T) {
		uc, _, wf, _ := newWorkflowUseCase()
		_, err := uc.Putus(context.Background(), 7, "TOLAK", "   ", "setuju1", setujui)
		assert.ErrorIs(t, err, ErrCatatanWajib)
		wf.AssertNotCalled(t, "GetPersetujuan", mock.Anything, mock.Anything)
	})

	t.Run("permintaan sudah diputuskan", func(t *testing.T) {
		uc, _, wf, _ := newWorkflowUseCase()
		wf.On("GetPersetujuan", mock.Anything, int64(7)).Return(permintaan(domain.PersetujuanDisetujui, 2, "kelola"), nil)
		_, err := uc.Putus(context.Background(), 7, "SETUJU", "", "setuju1", setujui)
		assert.ErrorIs(t, err, ErrPersetujuanSelesai)
	})

	t.Run("tidak ditemukan dan balapan keputusan", func(t *testing.T) {
		uc, _, wf, rec := newWorkflowUseCase()
		wf.On("GetPersetujuan", mock.Anything, int64(9)).Return(nil, domain.ErrPersetujuanNotFound)
		_, err := uc.Putus(context.Background(), 9, "SETUJU", "", "u", setujui)
		assert.ErrorIs(t, err, ErrPersetujuanNotFound)

		wf.On("GetPersetujuan", mock.Anything, int64(7)).Return(permintaan(domain.PersetujuanMenunggu, 1, "kelola"), nil)
		wf.On("Putus", mock.Anything, mock.Anything).Return(nil, domain.ErrPersetujuanSelesai)
		_, err = uc.Putus(context.Background(), 7, "SETUJU", "", "setuju1", setujui)
		assert.ErrorIs(t, err, ErrPersetujuanSelesai)
		assert.Empty(t, rec.Entries(), "keputusan yang kalah balapan tidak diaudit sebagai keputusan")
	})
}

func TestListAlurDanPersetujuan(t *testing.T) {
	uc, _, wf, _ := newWorkflowUseCase()
	wf.On("ListAlur", mock.Anything).Return([]domain.Alur{{AlurPersetujuan: domain.AlurPersetujuan{Kode: "eskalasi"}}}, nil)
	wf.On("ListPersetujuan", mock.Anything, "MENUNGGU").Return([]domain.Persetujuan{{}}, nil)

	alur, err := uc.ListAlur(context.Background())
	require.NoError(t, err)
	assert.Len(t, alur, 1)
	items, err := uc.ListPersetujuan(context.Background(), "MENUNGGU")
	require.NoError(t, err)
	assert.Len(t, items, 1)
}

func TestIzinLangkahKini(t *testing.T) {
	p := permintaan(domain.PersetujuanMenunggu, 2, "x")
	assert.Equal(t, "intervensi.setujui_fiskal", p.IzinLangkahKini())
	p.LangkahKini = 9
	assert.Empty(t, p.IzinLangkahKini())
}
