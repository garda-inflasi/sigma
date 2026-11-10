package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/audit/audittest"
	"github.com/garda-inflasi/api/internal/siklus/domain"
	"github.com/garda-inflasi/api/internal/siklus/dto"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRepository struct {
	wilayah string
	siklus  *domain.SiklusEkonomi
	aturan  []domain.AturanSiklus
	err     error
}

func (f *fakeRepository) GetActiveByWilayah(_ context.Context, wilayahKode string) (*domain.SiklusEkonomi, error) {
	f.wilayah = wilayahKode
	return f.siklus, f.err
}

func (f *fakeRepository) ListAturan(context.Context) ([]domain.AturanSiklus, error) {
	return f.aturan, f.err
}

type fakeEvaluator struct {
	wilayah string
	opsi    domain.OpsiEvaluasi
	hasil   domain.HasilEvaluasi
	err     error
}

func (f *fakeEvaluator) Evaluasi(_ context.Context, wilayahKode string, opsi domain.OpsiEvaluasi) (domain.HasilEvaluasi, error) {
	f.wilayah, f.opsi = wilayahKode, opsi
	return f.hasil, f.err
}

func baru(repo domain.Repository, ev domain.Evaluator) (*UseCase, *audittest.Recorder) {
	rec := &audittest.Recorder{}
	return New(repo, ev, rec, logger.NewDefault()), rec
}

func TestGetAktif_DefaultsToAceh(t *testing.T) {
	repo := &fakeRepository{siklus: &domain.SiklusEkonomi{Siklus: domain.SiklusTekanan}}
	uc, _ := baru(repo, &fakeEvaluator{})

	got, err := uc.GetAktif(t.Context(), "")

	require.NoError(t, err)
	assert.Equal(t, domain.WilayahBawaan, repo.wilayah)
	assert.Equal(t, domain.SiklusTekanan, got.Siklus)
}

func TestGetAktif_NotFoundVersusFailure(t *testing.T) {
	uc, _ := baru(&fakeRepository{err: domain.ErrNotFound}, &fakeEvaluator{})
	_, err := uc.GetAktif(t.Context(), "99")
	assert.ErrorIs(t, err, ErrSiklusNotFound)

	uc, _ = baru(&fakeRepository{err: errors.New("db mati")}, &fakeEvaluator{})
	_, err = uc.GetAktif(t.Context(), "11")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrSiklusNotFound, "galat basis data bukan 404")
}

func TestToSiklusAktifResponse_LowercaseAndAcehDate(t *testing.T) {
	// 26 September 20.00 UTC sudah 27 September 03.00 WIB.
	res := dto.ToSiklusAktifResponse("TEKANAN", time.Date(2026, 9, 26, 20, 0, 0, 0, time.UTC))

	assert.Equal(t, "tekanan", res.Siklus)
	assert.Equal(t, "2026-09-27", res.Sejak)
}

func TestListAturan_PassesThroughAndWrapsErrors(t *testing.T) {
	aturan := []domain.AturanSiklus{{Kode: "SKL-A"}}
	uc, _ := baru(&fakeRepository{aturan: aturan}, &fakeEvaluator{})
	got, err := uc.ListAturan(t.Context())
	require.NoError(t, err)
	assert.Equal(t, aturan, got)

	uc, _ = baru(&fakeRepository{err: errors.New("db mati")}, &fakeEvaluator{})
	_, err = uc.ListAturan(t.Context())
	assert.Error(t, err)
}

// Evaluasi manual selalu untuk provinsi bawaan, meneruskan pemintanya ke mesin (supaya pergantian siklus tercatat atas
// namanya), dan meninggalkan catatan EVALUASI walau tidak ada yang berubah.
func TestEvaluasi_RecordsManualRequestWithActor(t *testing.T) {
	aktif := &domain.SiklusEkonomi{Siklus: domain.SiklusTekanan}
	ev := &fakeEvaluator{hasil: domain.HasilEvaluasi{WilayahKode: "11", Aktif: aktif, Ringkasan: "Siklus tetap TEKANAN: ..."}}
	uc, rec := baru(&fakeRepository{}, ev)

	hasil, err := uc.Evaluasi(t.Context(), "u-1")

	require.NoError(t, err)
	assert.Equal(t, "11", ev.wilayah)
	assert.Equal(t, domain.OpsiEvaluasi{AktorID: "u-1", Pemicu: domain.PemicuManual}, ev.opsi)
	assert.Equal(t, "Siklus tetap TEKANAN: ...", hasil.Ringkasan)

	entry, ok := rec.Last()
	require.True(t, ok, "evaluasi manual harus diaudit")
	assert.Equal(t, "u-1", entry.AktorID)
	assert.Equal(t, "EVALUASI", entry.Aksi)
	assert.Equal(t, "siklus_ekonomi", entry.Entitas)
	assert.Equal(t, audit.HasilBerhasil, entry.Hasil)
	assert.Equal(t, domain.SiklusTekanan, entry.Metadata["siklus"])
	assert.Equal(t, false, entry.Metadata["berubah"])
}

func TestEvaluasi_ErrorsAreNotAuditedAndBusyIsDistinguished(t *testing.T) {
	ev := &fakeEvaluator{err: domain.ErrEvaluasiSibuk}
	uc, rec := baru(&fakeRepository{}, ev)
	_, err := uc.Evaluasi(t.Context(), "u-1")
	assert.ErrorIs(t, err, ErrEvaluasiSibuk)

	ev.err = errors.New("db mati")
	_, err = uc.Evaluasi(t.Context(), "u-1")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrEvaluasiSibuk)

	assert.Empty(t, rec.Entries(), "evaluasi yang gagal tidak meninggalkan catatan berhasil")
}

func TestEvaluasi_AuditFailureDoesNotFailTheEvaluation(t *testing.T) {
	uc, rec := baru(&fakeRepository{}, &fakeEvaluator{hasil: domain.HasilEvaluasi{WilayahKode: "11"}})
	rec.Err = errors.New("audit mati")

	_, err := uc.Evaluasi(t.Context(), "u-1")

	assert.NoError(t, err, "pola proyek: kegagalan audit hanya dilog, aksinya sudah terjadi")
}
