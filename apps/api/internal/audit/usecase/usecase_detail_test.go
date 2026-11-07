package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/audit/domain"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr(s string) *string { return &s }

func TestGet_ReturnsRowWithDiff(t *testing.T) {
	repo := &fakeRepo{rows: []domain.AuditLog{{
		ID: 7, Kode: ptr("a1-lama"), Sebelum: ptr(`{"peran":"A","aktif":true}`), Sesudah: ptr(`{"peran":"B","aktif":true}`),
	}}}

	detail, err := newUseCase(repo).Get(context.Background(), idx.RujukanID{ID: 7})

	require.NoError(t, err)
	assert.Equal(t, idx.ID(7), detail.Row.ID)
	require.Len(t, detail.Perubahan, 1)
	assert.Equal(t, "peran", detail.Perubahan[0].Path)
	assert.Equal(t, domain.PerubahanDiubah, detail.Perubahan[0].Jenis)
}

func TestGet_AliasKodeIDLama(t *testing.T) {
	repo := &fakeRepo{rows: []domain.AuditLog{{ID: 7, Kode: ptr("a1-lama"), Sesudah: ptr(`{"peran":"B"}`)}}}

	detail, err := newUseCase(repo).Get(context.Background(), idx.RujukanID{Kode: "a1-lama"})

	require.NoError(t, err)
	assert.Equal(t, idx.ID(7), detail.Row.ID)
}

func TestGet_NotFoundAndInvalidState(t *testing.T) {
	repo := &fakeRepo{rows: []domain.AuditLog{{ID: 9, Kode: ptr("rusak"), Sebelum: ptr(`{bukan json`)}}}
	uc := newUseCase(repo)

	_, err := uc.Get(context.Background(), idx.RujukanID{Kode: "tidak-ada"})
	assert.ErrorIs(t, err, domain.ErrNotFound)

	_, err = uc.Get(context.Background(), idx.RujukanID{ID: 9})
	require.Error(t, err)
	assert.NotErrorIs(t, err, domain.ErrNotFound)
}

// pagedRepo menghormati cursor seperti repositori sungguhan: baris r0, r1, ...
// terurut menurun menurut waktu.
type pagedRepo struct {
	fakeRepo
	total int
	calls int
}

func (p *pagedRepo) List(_ context.Context, _ domain.Filter, after *domain.Cursor, limit int) ([]domain.AuditLog, error) {
	p.calls++
	start := 0
	if after != nil {
		// Baris r0, r1, ... memakai ID total..1 (menurun menurut waktu, seperti identity yang menaik).
		start = p.total - int(after.ID) + 1
	}
	var out []domain.AuditLog
	for i := start; i < p.total && len(out) < limit; i++ {
		out = append(out, domain.AuditLog{ID: idx.ID(p.total - i), Waktu: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Add(-time.Duration(i) * time.Minute)})
	}
	return out, nil
}

func exportAll(t *testing.T, total int) (n int, truncated bool, ids []idx.ID, calls int) {
	t.Helper()
	repo := &pagedRepo{total: total}
	n, truncated, err := New(repo).Export(context.Background(), domain.Filter{}, func(row domain.AuditLog) error {
		ids = append(ids, row.ID)
		return nil
	})
	require.NoError(t, err)
	return n, truncated, ids, repo.calls
}

func TestExport_WalksBatchesWithoutDuplicates(t *testing.T) {
	n, truncated, ids, calls := exportAll(t, ExportBatch*2+7)

	assert.Equal(t, ExportBatch*2+7, n)
	assert.False(t, truncated)
	assert.Len(t, ids, n)
	seen := map[idx.ID]bool{}
	for _, id := range ids {
		assert.False(t, seen[id], "baris dobel: %s", id)
		seen[id] = true
	}
	assert.Equal(t, 3, calls)
}

func TestExport_ExactBatchBoundaryIsNotTruncated(t *testing.T) {
	n, truncated, _, _ := exportAll(t, ExportBatch)

	assert.Equal(t, ExportBatch, n)
	assert.False(t, truncated)
}

func TestExport_CapsAtMaxRows(t *testing.T) {
	n, truncated, _, _ := exportAll(t, MaxExportRows+1)
	assert.Equal(t, MaxExportRows, n)
	assert.True(t, truncated)

	n, truncated, _, _ = exportAll(t, MaxExportRows)
	assert.Equal(t, MaxExportRows, n)
	assert.False(t, truncated, "tepat sebatas batas tidak dianggap terpotong")
}

func TestExport_StopsOnEmitError(t *testing.T) {
	boom := errors.New("tulis gagal")

	n, _, err := New(&pagedRepo{total: 10}).Export(context.Background(), domain.Filter{}, func(domain.AuditLog) error { return boom })

	assert.ErrorIs(t, err, boom)
	assert.Zero(t, n)
}
