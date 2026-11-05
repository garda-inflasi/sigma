package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/audit/audittest"
	"github.com/garda-inflasi/api/internal/policy-brief/domain"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRepo menyimpan brief di memori; id yang sudah ada tidak ditimpa.
type fakeRepo struct {
	rows      map[string]domain.PolicyBrief
	createErr error
}

func newFakeRepo() *fakeRepo { return &fakeRepo{rows: map[string]domain.PolicyBrief{}} }

func (r *fakeRepo) List(context.Context) ([]domain.BriefView, error) { return nil, nil }

func (r *fakeRepo) Get(_ context.Context, id string) (*domain.BriefView, error) {
	pb := r.rows[id]
	return &domain.BriefView{ID: pb.ID, Periode: pb.Periode, Judul: pb.Judul, Status: pb.Status, Ringkasan: pb.Ringkasan, DisusunOleh: "Penyusun"}, nil
}

func (r *fakeRepo) Transisi(_ context.Context, id, dari, ke string) (bool, error) {
	pb, ada := r.rows[id]
	if !ada || pb.Status != dari {
		return false, nil
	}
	pb.Status = ke
	r.rows[id] = pb
	return true, nil
}

func (r *fakeRepo) Create(_ context.Context, pb *domain.PolicyBrief) (bool, error) {
	if r.createErr != nil {
		return false, r.createErr
	}
	if _, ada := r.rows[pb.ID]; ada {
		return false, nil
	}
	r.rows[pb.ID] = *pb
	return true, nil
}

func newUseCase(repo *fakeRepo) (*UseCase, *audittest.Recorder) {
	rec := &audittest.Recorder{}
	return New(repo, rec, logger.NewDefault()), rec
}

func TestGenerate_SavesValidDraftFromTemplate(t *testing.T) {
	repo := newFakeRepo()
	uc, _ := newUseCase(repo)

	got, err := uc.Generate(t.Context(), "manajer_kebijakan", "2026-10")

	require.NoError(t, err)
	assert.Equal(t, "PB-2026-10", got.ID)
	stored := repo.rows["PB-2026-10"]
	assert.Equal(t, "Oktober 2026", stored.Periode)
	assert.Equal(t, domain.StatusDraf, stored.Status)
	assert.Equal(t, "manajer_kebijakan", stored.DisusunOlehID)
	assert.JSONEq(t, "[]", stored.Rekomendasi)
	assert.Contains(t, stored.Ringkasan, "non-LLM")
	assert.Contains(t, stored.Judul, "draf otomatis")
	assert.Positive(t, stored.Halaman)
}

func TestGenerate_IsDeterministic(t *testing.T) {
	a, b := newFakeRepo(), newFakeRepo()
	ucA, _ := newUseCase(a)
	ucB, _ := newUseCase(b)

	_, errA := ucA.Generate(t.Context(), "u1", "2026-03")
	_, errB := ucB.Generate(t.Context(), "u1", "2026-03")

	require.NoError(t, errA)
	require.NoError(t, errB)
	assert.Equal(t, a.rows, b.rows)
}

func TestGenerate_RejectsDuplicatePeriodWithoutOverwriting(t *testing.T) {
	repo := newFakeRepo()
	repo.rows["PB-2026-10"] = domain.PolicyBrief{ID: "PB-2026-10", Judul: "Asli", Status: domain.StatusDisetujui}
	uc, rec := newUseCase(repo)

	_, err := uc.Generate(t.Context(), "u1", "2026-10")

	assert.ErrorIs(t, err, ErrPeriodeSudahAda)
	assert.Equal(t, "Asli", repo.rows["PB-2026-10"].Judul)
	assert.Empty(t, rec.Entries(), "tidak ada perubahan data, tidak ada audit")
}

func TestGenerate_RejectsInvalidPeriod(t *testing.T) {
	for _, periode := range []string{"", "Oktober 2026", "2026-13", "2026-00", "2026-1", "2026/10"} {
		repo := newFakeRepo()
		uc, _ := newUseCase(repo)

		_, err := uc.Generate(t.Context(), "u1", periode)

		assert.ErrorIs(t, err, ErrPeriodeTidakValid, periode)
		assert.Empty(t, repo.rows, periode)
	}
}

func TestGenerate_WritesAuditWithActor(t *testing.T) {
	uc, rec := newUseCase(newFakeRepo())

	_, err := uc.Generate(t.Context(), "manajer_kebijakan", "2026-10")

	require.NoError(t, err)
	entry, ok := rec.Last()
	require.True(t, ok)
	assert.Equal(t, "manajer_kebijakan", entry.AktorID)
	assert.Equal(t, "GENERATE_BRIEF", entry.Aksi)
	assert.Equal(t, "policy_brief", entry.Entitas)
	assert.Equal(t, "PB-2026-10", entry.EntitasID)
	assert.Equal(t, audit.HasilBerhasil, entry.Hasil)
}

func TestGenerate_AuditFailureDoesNotFailTheAction(t *testing.T) {
	uc, rec := newUseCase(newFakeRepo())
	rec.Err = errors.New("audit down")

	got, err := uc.Generate(t.Context(), "u1", "2026-10")

	require.NoError(t, err)
	assert.Equal(t, "PB-2026-10", got.ID)
}

func TestGenerate_PropagatesRepositoryError(t *testing.T) {
	repo := newFakeRepo()
	repo.createErr = errors.New("db down")
	uc, rec := newUseCase(repo)

	_, err := uc.Generate(t.Context(), "u1", "2026-10")

	assert.Error(t, err)
	assert.Empty(t, rec.Entries())
}

func TestBulkAction_MemajukanStatusDanMencatatAudit(t *testing.T) {
	repo := newFakeRepo()
	repo.rows["PB-2026-09"] = domain.PolicyBrief{ID: "PB-2026-09", Status: domain.StatusDraf}
	repo.rows["PB-2026-08"] = domain.PolicyBrief{ID: "PB-2026-08", Status: domain.StatusDisetujui}
	uc, rec := newUseCase(repo)

	hasil, err := uc.BulkAction(context.Background(), []string{"PB-2026-09", "PB-2026-08", "tidak-ada"}, domain.AksiTelaah, "u1")

	require.NoError(t, err)
	assert.Equal(t, BulkResult{Berhasil: 1, Gagal: 2}, hasil)
	assert.Equal(t, domain.StatusTelaah, repo.rows["PB-2026-09"].Status)
	assert.Equal(t, domain.StatusDisetujui, repo.rows["PB-2026-08"].Status)
	assert.Len(t, rec.Entries(), 1)
}

func TestBulkAction_SetujuiHanyaDariTelaah(t *testing.T) {
	repo := newFakeRepo()
	repo.rows["PB-2026-09"] = domain.PolicyBrief{ID: "PB-2026-09", Status: domain.StatusDraf}
	uc, _ := newUseCase(repo)

	hasil, err := uc.BulkAction(context.Background(), []string{"PB-2026-09"}, domain.AksiSetujui, "u1")

	require.NoError(t, err)
	assert.Equal(t, BulkResult{Gagal: 1}, hasil)
}

func TestBulkAction_AksiTidakValid(t *testing.T) {
	uc, _ := newUseCase(newFakeRepo())

	_, err := uc.BulkAction(context.Background(), []string{"x"}, "hapus", "u1")

	assert.ErrorIs(t, err, ErrAksiTidakValid)
}
