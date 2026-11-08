package usecase

import (
	"context"
	"testing"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/audit/audittest"
	"github.com/garda-inflasi/api/internal/master/domain"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ── Mock Repository ───────────────────────────────────────────────────

type mockRepository struct {
	mock.Mock
}

func (m *mockRepository) ListKomoditas(ctx context.Context, q domain.ListQuery) ([]domain.Komoditas, int64, error) {
	args := m.Called(ctx, q)
	return args.Get(0).([]domain.Komoditas), args.Get(1).(int64), args.Error(2)
}

func (m *mockRepository) ListWilayah(ctx context.Context, provinsiKode string, q domain.ListQuery) ([]domain.Kabupaten, int64, error) {
	args := m.Called(ctx, provinsiKode, q)
	return args.Get(0).([]domain.Kabupaten), args.Get(1).(int64), args.Error(2)
}

func (m *mockRepository) ListPasar(ctx context.Context, kabupatenKode string, q domain.ListQuery) ([]domain.Pasar, int64, error) {
	args := m.Called(ctx, kabupatenKode, q)
	return args.Get(0).([]domain.Pasar), args.Get(1).(int64), args.Error(2)
}

func (m *mockRepository) StatusKomoditas(ctx context.Context, kode []string) (map[string]string, error) {
	args := m.Called(ctx, kode)
	status, _ := args.Get(0).(map[string]string)
	return status, args.Error(1)
}

func (m *mockRepository) UbahStatusKomoditas(ctx context.Context, kode string, t domain.Transisi) (bool, error) {
	args := m.Called(ctx, kode, t)
	return args.Bool(0), args.Error(1)
}

// ── Test Helpers ──────────────────────────────────────────────────────

func newTestUseCase(repo *mockRepository) (*UseCase, *audittest.Recorder) {
	recorder := &audittest.Recorder{}
	return New(repo, recorder, logger.NewDefault()), recorder
}

// ── List Tests ────────────────────────────────────────────────────────

func TestListKomoditas_Success(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	expected := []domain.Komoditas{{Kode: "BRS-MED", Nama: "Beras medium", Kategori: "Pangan pokok", Satuan: "kg", Status: "AKTIF"}}
	query := domain.ListQuery{Search: "beras", Page: 2, Limit: 5}
	repo.On("ListKomoditas", mock.Anything, query).Return(expected, int64(11), nil)

	result, total, err := uc.ListKomoditas(context.Background(), query)

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "BRS-MED", result[0].Kode)
	assert.EqualValues(t, 11, total)
	repo.AssertExpectations(t)
}

func TestListWilayah_Success(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	expected := []domain.Kabupaten{{Kode: "1171", Nama: "KOTA BANDA ACEH", ProvinsiKode: "11"}}
	repo.On("ListWilayah", mock.Anything, "11", domain.ListQuery{}).Return(expected, int64(1), nil)

	result, total, err := uc.ListWilayah(context.Background(), "11", domain.ListQuery{})

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "1171", result[0].Kode)
	assert.EqualValues(t, 1, total)
	repo.AssertExpectations(t)
}

func TestListWilayah_Empty(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	repo.On("ListWilayah", mock.Anything, "", domain.ListQuery{}).Return([]domain.Kabupaten{}, int64(0), nil)

	result, _, err := uc.ListWilayah(context.Background(), "", domain.ListQuery{})

	require.NoError(t, err)
	assert.Empty(t, result)
	repo.AssertExpectations(t)
}

func TestListPasar_Success(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	expected := []domain.Pasar{{ID: 1, Kode: "P1", Nama: "Pasar Induk", KabupatenKode: "1171", Aktif: true}}
	repo.On("ListPasar", mock.Anything, "1171", domain.ListQuery{}).Return(expected, int64(1), nil)

	result, _, err := uc.ListPasar(context.Background(), "1171", domain.ListQuery{})

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, idx.ID(1), result[0].ID)
	assert.Equal(t, "P1", result[0].Kode)
	repo.AssertExpectations(t)
}

// ── BulkKomoditas Tests ───────────────────────────────────────────────

func TestBulkKomoditas_Transitions(t *testing.T) {
	cases := []struct {
		action    string
		domain    domain.BulkAction
		from      string
		naikVersi bool
	}{
		{"ajukan_perubahan", domain.BulkAjukanPerubahan, "AKTIF", false},
		{"setujui", domain.BulkSetujui, "MENUNGGU_PERSETUJUAN", true},
		{"arsipkan", domain.BulkArsipkan, "AKTIF", false},
		{"arsipkan", domain.BulkArsipkan, "MENUNGGU_PERSETUJUAN", false},
	}
	for _, tc := range cases {
		t.Run(tc.action+"/"+tc.from, func(t *testing.T) {
			repo := new(mockRepository)
			uc, recorder := newTestUseCase(repo)
			kode := []string{"BRS-MED", "CBM-KRT"}
			transisi, _ := domain.TransisiUntuk(tc.domain)
			assert.Equal(t, tc.naikVersi, transisi.NaikVersi)
			repo.On("StatusKomoditas", mock.Anything, kode).Return(map[string]string{"BRS-MED": tc.from, "CBM-KRT": tc.from}, nil)
			repo.On("UbahStatusKomoditas", mock.Anything, mock.Anything, transisi).Return(true, nil).Twice()

			hasil, err := uc.BulkKomoditas(context.Background(), kode, tc.action, uid("admin-1"))

			require.NoError(t, err)
			assert.Equal(t, 2, hasil.Berhasil)
			assert.Zero(t, hasil.Gagal)
			assert.Equal(t, []domain.RincianBulk{{Kode: "BRS-MED", Berhasil: true}, {Kode: "CBM-KRT", Berhasil: true}}, hasil.Rincian)
			entry, ok := recorder.Last()
			require.True(t, ok, "aksi yang mengubah data tercatat di audit")
			assert.Equal(t, "BULK_"+tc.action, entry.Aksi)
			assert.Equal(t, uid("admin-1"), entry.AktorID)
			assert.Equal(t, audit.HasilBerhasil, entry.Hasil)
			assert.Equal(t, kode, entry.Metadata["kode"])
			repo.AssertExpectations(t)
		})
	}
}

func TestBulkKomoditas_PerItemFailures(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	kode := []string{"BRS-MED", "TIDAK-ADA", "CBM-KRT", "BRS-MED", ""}
	unik := []string{"BRS-MED", "TIDAK-ADA", "CBM-KRT"}
	transisi, _ := domain.TransisiUntuk(domain.BulkAjukanPerubahan)
	repo.On("StatusKomoditas", mock.Anything, unik).Return(map[string]string{"BRS-MED": "AKTIF", "CBM-KRT": "DIARSIPKAN"}, nil)
	repo.On("UbahStatusKomoditas", mock.Anything, "BRS-MED", transisi).Return(true, nil).Once()

	hasil, err := uc.BulkKomoditas(context.Background(), kode, "ajukan_perubahan", uid("admin-1"))

	require.NoError(t, err)
	assert.Equal(t, 1, hasil.Berhasil)
	assert.Equal(t, 2, hasil.Gagal)
	require.Len(t, hasil.Rincian, 3, "kode ganda dan kosong tidak diproses ulang")
	assert.True(t, hasil.Rincian[0].Berhasil)
	assert.Equal(t, "TIDAK-ADA", hasil.Rincian[1].Kode)
	assert.Contains(t, hasil.Rincian[1].Pesan, "tidak ditemukan")
	assert.False(t, hasil.Rincian[2].Berhasil)
	assert.Contains(t, hasil.Rincian[2].Pesan, "DIARSIPKAN")
	repo.AssertNotCalled(t, "UbahStatusKomoditas", mock.Anything, "CBM-KRT", mock.Anything)
	entry, _ := recorder.Last()
	assert.Equal(t, []string{"BRS-MED"}, entry.Metadata["kode"], "audit hanya memuat kode yang berubah")
	assert.Equal(t, []string{"TIDAK-ADA", "CBM-KRT"}, entry.Metadata["gagal"])
}

func TestBulkKomoditas_AllFailedWritesNoAudit(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	kode := []string{"BRS-MED"}
	repo.On("StatusKomoditas", mock.Anything, kode).Return(map[string]string{"BRS-MED": "AKTIF"}, nil)

	hasil, err := uc.BulkKomoditas(context.Background(), kode, "setujui", uid("admin-1"))

	require.NoError(t, err)
	assert.Equal(t, 1, hasil.Gagal)
	assert.Empty(t, recorder.Entries(), "tidak ada data yang berubah, tidak ada audit")
}

func TestBulkKomoditas_RaceLostIsFailedItem(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	kode := []string{"BRS-MED"}
	repo.On("StatusKomoditas", mock.Anything, kode).Return(map[string]string{"BRS-MED": "AKTIF"}, nil)
	repo.On("UbahStatusKomoditas", mock.Anything, "BRS-MED", mock.Anything).Return(false, nil)

	hasil, err := uc.BulkKomoditas(context.Background(), kode, "arsipkan", uid("admin-1"))

	require.NoError(t, err)
	assert.Equal(t, 1, hasil.Gagal)
	assert.Contains(t, hasil.Rincian[0].Pesan, "berubah")
}

func TestBulkKomoditas_InvalidAction(t *testing.T) {
	uc, recorder := newTestUseCase(new(mockRepository))

	_, err := uc.BulkKomoditas(context.Background(), []string{"BRS-MED"}, "hapus", uid("admin-1"))

	assert.ErrorIs(t, err, ErrInvalidAction)
	assert.Empty(t, recorder.Entries())
}

func TestBulkKomoditas_RepoError(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	kode := []string{"BRS-MED"}
	repo.On("StatusKomoditas", mock.Anything, kode).Return(nil, assert.AnError)

	_, err := uc.BulkKomoditas(context.Background(), kode, "arsipkan", uid("admin-1"))

	assert.Error(t, err)
	assert.Empty(t, recorder.Entries(), "aksi yang gagal tidak dicatat sebagai berhasil")
	repo.AssertExpectations(t)
}

func TestBulkKomoditas_UpdateErrorAborts(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	kode := []string{"BRS-MED"}
	repo.On("StatusKomoditas", mock.Anything, kode).Return(map[string]string{"BRS-MED": "AKTIF"}, nil)
	repo.On("UbahStatusKomoditas", mock.Anything, "BRS-MED", mock.Anything).Return(false, assert.AnError)

	_, err := uc.BulkKomoditas(context.Background(), kode, "arsipkan", uid("admin-1"))

	assert.Error(t, err)
}

func TestBulkKomoditas_AuditFailureDoesNotFailAction(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	recorder.Err = assert.AnError
	kode := []string{"BRS-MED"}
	repo.On("StatusKomoditas", mock.Anything, kode).Return(map[string]string{"BRS-MED": "MENUNGGU_PERSETUJUAN"}, nil)
	repo.On("UbahStatusKomoditas", mock.Anything, "BRS-MED", mock.Anything).Return(true, nil)

	hasil, err := uc.BulkKomoditas(context.Background(), kode, "setujui", uid("admin-1"))

	require.NoError(t, err)
	assert.Equal(t, 1, hasil.Berhasil)
}

func TestIzinUntukAksi(t *testing.T) {
	assert.Equal(t, "master.setujui", domain.BulkSetujui.IzinUntuk())
	assert.Equal(t, "master.ajukan", domain.BulkAjukanPerubahan.IzinUntuk())
	assert.Equal(t, "master.ajukan", domain.BulkArsipkan.IzinUntuk())
}
