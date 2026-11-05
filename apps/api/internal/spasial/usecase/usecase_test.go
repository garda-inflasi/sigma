package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/spasial/domain"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRepository struct {
	markers  []domain.Marker
	neraca   []domain.Neraca
	rute     []domain.Rute
	kandidat []domain.Kandidat
	geometri []domain.GeometriWilayah
	bbox     *domain.BBox
	query    domain.NeracaQuery
	err      error
}

func (f *fakeRepository) ListMarker(context.Context) ([]domain.Marker, error) {
	return f.markers, f.err
}

func (f *fakeRepository) ListGeometri(_ context.Context, bbox *domain.BBox) ([]domain.GeometriWilayah, error) {
	f.bbox = bbox
	return f.geometri, f.err
}

func (f *fakeRepository) ListNeraca(_ context.Context, q domain.NeracaQuery) ([]domain.Neraca, error) {
	f.query = q
	return f.neraca, f.err
}

func (f *fakeRepository) ListRute(context.Context) ([]domain.Rute, error) { return f.rute, f.err }

func (f *fakeRepository) ListKandidat(context.Context) ([]domain.Kandidat, error) {
	return f.kandidat, f.err
}

func TestUseCase_PassesDataThrough(t *testing.T) {
	repo := &fakeRepository{
		markers:  []domain.Marker{{Wilayah: domain.Wilayah{Kode: "1171"}}},
		neraca:   []domain.Neraca{{KomoditasNama: "Cabai"}},
		rute:     []domain.Rute{{AsalNama: "Takengon"}},
		kandidat: []domain.Kandidat{{KabupatenNama: "Langsa"}},
	}
	uc := New(repo, logger.NewDefault())
	ctx := context.Background()

	markers, err := uc.ListMarker(ctx)
	require.NoError(t, err)
	assert.Equal(t, repo.markers, markers)

	query := domain.NeracaQuery{KomoditasKode: "BWM", Periode: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}
	neraca, err := uc.ListNeraca(ctx, query)
	require.NoError(t, err)
	assert.Equal(t, repo.neraca, neraca)
	assert.Equal(t, query, repo.query, "filter diteruskan ke repository")

	rute, err := uc.ListRute(ctx)
	require.NoError(t, err)
	assert.Equal(t, repo.rute, rute)

	kandidat, err := uc.ListKandidat(ctx)
	require.NoError(t, err)
	assert.Equal(t, repo.kandidat, kandidat)
}

func TestUseCase_RepositoryErrorsAreReturned(t *testing.T) {
	uc := New(&fakeRepository{err: assert.AnError}, logger.NewDefault())
	ctx := context.Background()

	_, err := uc.ListMarker(ctx)
	assert.ErrorIs(t, err, assert.AnError)
	_, err = uc.ListNeraca(ctx, domain.NeracaQuery{})
	assert.ErrorIs(t, err, assert.AnError)
	_, err = uc.ListRute(ctx)
	assert.ErrorIs(t, err, assert.AnError)
	_, err = uc.ListKandidat(ctx)
	assert.ErrorIs(t, err, assert.AnError)
}
