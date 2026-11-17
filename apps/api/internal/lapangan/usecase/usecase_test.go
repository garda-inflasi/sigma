package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/audit/audittest"
	hargadomain "github.com/garda-inflasi/api/internal/harga/domain"
	"github.com/garda-inflasi/api/internal/lapangan/domain"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockRepository struct {
	mock.Mock
}

func (m *mockRepository) PenggunaCakupan(ctx context.Context, userID string) (string, string, bool, error) {
	args := m.Called(ctx, userID)
	return args.String(0), args.String(1), args.Bool(2), args.Error(3)
}

func (m *mockRepository) ListSaya(ctx context.Context, cakupan hargadomain.Cakupan) ([]domain.TugasView, error) {
	args := m.Called(ctx, cakupan)
	items, _ := args.Get(0).([]domain.TugasView)
	return items, args.Error(1)
}

func (m *mockRepository) GetByID(ctx context.Context, ref idx.RujukanID, cakupan hargadomain.Cakupan) (*domain.TugasView, error) {
	args := m.Called(ctx, ref, cakupan)
	item, _ := args.Get(0).(*domain.TugasView)
	return item, args.Error(1)
}

func (m *mockRepository) Rincian(ctx context.Context, tugasID idx.ID, kode []string) ([]domain.RincianKomoditas, error) {
	args := m.Called(ctx, tugasID, kode)
	items, _ := args.Get(0).([]domain.RincianKomoditas)
	return items, args.Error(1)
}

func (m *mockRepository) Aksesibel(ctx context.Context, refs []idx.RujukanID, cakupan hargadomain.Cakupan) ([]domain.TugasRef, error) {
	args := m.Called(ctx, refs, cakupan)
	found, _ := args.Get(0).([]domain.TugasRef)
	return found, args.Error(1)
}

func (m *mockRepository) TandaiSelesai(ctx context.Context, refs []idx.RujukanID, cakupan hargadomain.Cakupan, waktu time.Time) ([]domain.TugasRef, error) {
	args := m.Called(ctx, refs, cakupan, waktu)
	changed, _ := args.Get(0).([]domain.TugasRef)
	return changed, args.Error(1)
}

func (m *mockRepository) Acuan(ctx context.Context, sejak time.Time, minSampel int) (domain.DataAcuan, error) {
	args := m.Called(ctx, sejak, minSampel)
	return args.Get(0).(domain.DataAcuan), args.Error(1)
}

func (m *mockRepository) EntriMilikPetugas(ctx context.Context, entriID idx.ID) (string, string, bool, error) {
	args := m.Called(ctx, entriID)
	return args.String(0), args.String(1), args.Bool(2), args.Error(3)
}

func (m *mockRepository) SimpanLampiranEntri(ctx context.Context, lampiran *domain.LampiranEntri, fotoURL string) error {
	args := m.Called(ctx, lampiran, fotoURL)
	return args.Error(0)
}

func (m *mockRepository) LampiranTerbaru(ctx context.Context, entriID idx.ID) (*domain.LampiranEntri, error) {
	args := m.Called(ctx, entriID)
	item, _ := args.Get(0).(*domain.LampiranEntri)
	return item, args.Error(1)
}

func (m *mockRepository) KodeEtikAktif(ctx context.Context, userID string) (*domain.KodeEtikStatus, error) {
	args := m.Called(ctx, userID)
	item, _ := args.Get(0).(*domain.KodeEtikStatus)
	return item, args.Error(1)
}

func (m *mockRepository) SetujuiKodeEtik(ctx context.Context, userID, versi string) (*domain.PersetujuanKodeEtik, error) {
	args := m.Called(ctx, userID, versi)
	item, _ := args.Get(0).(*domain.PersetujuanKodeEtik)
	return item, args.Error(1)
}

func (m *mockRepository) ListKinerja(ctx context.Context, cakupan hargadomain.Cakupan) ([]domain.KinerjaPetugas, error) {
	args := m.Called(ctx, cakupan)
	items, _ := args.Get(0).([]domain.KinerjaPetugas)
	return items, args.Error(1)
}

var fixedNow = time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)

func newTestUseCase(repo *mockRepository) (*UseCase, *audittest.Recorder) {
	recorder := &audittest.Recorder{}
	uc := New(repo, recorder, logger.NewDefault())
	uc.now = func() time.Time { return fixedNow }
	return uc, recorder
}

func tugas(id idx.ID, status string, tenggat time.Duration, selesai int) domain.TugasView {
	return domain.TugasView{ID: id, Status: status, Tenggat: fixedNow.Add(tenggat), Target: 4, JumlahSelesai: selesai}
}

// refKode dan refID membuat rujukan tugas menurut kode lama atau ID BIGINT.
func refKode(kode string) idx.RujukanID { return idx.RujukanID{Kode: kode} }
func refID(id idx.ID) idx.RujukanID     { return idx.RujukanID{ID: id} }

// tugasRef membuat baris tugas hasil kueri: ID BIGINT beserta kode lamanya (kosong untuk tugas baru).
func tugasRef(id idx.ID, kode string) domain.TugasRef {
	if kode == "" {
		return domain.TugasRef{ID: id}
	}
	return domain.TugasRef{ID: id, Kode: &kode}
}

func TestListSaya_ComputesDisplayedStatus(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	repo.On("ListSaya", mock.Anything, hargadomain.CakupanMilik(uid("mahasiswa_1"))).Return([]domain.TugasView{
		tugas(1, domain.StatusBelum, 2*time.Hour, 0),
		tugas(2, domain.StatusBelum, 2*time.Hour, 1),
		tugas(3, domain.StatusBerjalan, -time.Hour, 1),
		tugas(4, domain.StatusSelesai, -time.Hour, 4),
	}, nil)

	got, err := uc.ListSaya(t.Context(), Pemanggil{ID: uid("mahasiswa_1")})

	require.NoError(t, err)
	status := map[idx.ID]string{}
	for _, item := range got {
		status[item.ID] = item.StatusTampil
	}
	assert.Equal(t, map[idx.ID]string{
		1: domain.StatusBelum, 2: domain.StatusBerjalan, 3: domain.StatusTerlambat, 4: domain.StatusSelesai,
	}, status)
}

func TestListSaya_RepoError(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	repo.On("ListSaya", mock.Anything, hargadomain.CakupanMilik("u")).Return(nil, errors.New("db mati"))

	_, err := uc.ListSaya(t.Context(), Pemanggil{ID: "u"})

	require.Error(t, err)
}

func TestGetByID(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	item := tugas(11, domain.StatusBelum, time.Hour, 0)
	repo.On("GetByID", mock.Anything, refID(11), hargadomain.CakupanMilik("u")).Return(&item, nil)
	repo.On("GetByID", mock.Anything, refKode("TGS-mahasiswa_1-1"), hargadomain.CakupanMilik("u")).Return(&item, nil)
	repo.On("GetByID", mock.Anything, refID(12), hargadomain.CakupanMilik("u")).Return(nil, domain.ErrNotFound)

	got, err := uc.GetByID(t.Context(), refID(11), Pemanggil{ID: "u"})
	require.NoError(t, err)
	assert.Equal(t, idx.ID(11), got.ID)

	alias, err := uc.GetByID(t.Context(), refKode("TGS-mahasiswa_1-1"), Pemanggil{ID: "u"})
	require.NoError(t, err)
	assert.Equal(t, idx.ID(11), alias.ID, "kode lama menunjuk tugas yang sama")

	_, err = uc.GetByID(t.Context(), refID(12), Pemanggil{ID: "u"})
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestGetDetail_RincianMengikutiUrutanTugasDanMelengkapiKodeTakDikenal(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	item := tugas(11, domain.StatusBelum, time.Hour, 1)
	item.Komoditas = domain.KodeKomoditas{"TLR", "XXX", "BWM"}
	repo.On("GetByID", mock.Anything, refID(11), hargadomain.CakupanMilik("u")).Return(&item, nil)
	repo.On("GetByID", mock.Anything, refID(12), hargadomain.CakupanMilik("u")).Return(nil, domain.ErrNotFound)
	repo.On("Rincian", mock.Anything, idx.ID(11), []string{"TLR", "XXX", "BWM"}).Return([]domain.RincianKomoditas{
		{Kode: "BWM", Nama: "Bawang merah", Terisi: true},
		{Kode: "TLR", Nama: "Telur ayam ras", Terisi: false},
	}, nil)

	got, err := uc.GetDetail(t.Context(), refID(11), Pemanggil{ID: "u"})
	require.NoError(t, err)
	assert.Equal(t, []domain.RincianKomoditas{
		{Kode: "TLR", Nama: "Telur ayam ras", Terisi: false},
		{Kode: "XXX", Nama: "XXX", Terisi: false},
		{Kode: "BWM", Nama: "Bawang merah", Terisi: true},
	}, got.Rincian)

	_, err = uc.GetDetail(t.Context(), refID(12), Pemanggil{ID: "u"})
	assert.ErrorIs(t, err, ErrNotFound)
	repo.AssertNotCalled(t, "Rincian", mock.Anything, idx.ID(12), mock.Anything)
}

func TestGetDetail_GalatRepositoriDiteruskan(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	item := tugas(11, domain.StatusBelum, time.Hour, 0)
	repo.On("GetByID", mock.Anything, refID(11), hargadomain.CakupanMilik("u")).Return(&item, nil)
	repo.On("Rincian", mock.Anything, idx.ID(11), mock.Anything).Return(nil, errors.New("db mati"))

	_, err := uc.GetDetail(t.Context(), refID(11), Pemanggil{ID: "u"})
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrNotFound)
}

func TestBulkAction_TandaiSelesaiAudits(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	repo.On("TandaiSelesai", mock.Anything, []idx.RujukanID{refID(1), refID(2)}, hargadomain.CakupanMilik("u"), fixedNow).Return([]domain.TugasRef{tugasRef(1, "")}, nil)

	hasil, err := uc.BulkAction(t.Context(), []idx.RujukanID{refID(1), refID(2), refID(1), {}}, "tandai_selesai", Pemanggil{ID: "u"})

	require.NoError(t, err)
	assert.Equal(t, HasilBulk{Berhasil: 1, Gagal: 1}, hasil, "ganda dan kosong disaring")
	entry, ok := recorder.Last()
	require.True(t, ok, "aksi yang mengubah data tercatat di audit")
	assert.Equal(t, "TUGAS_TANDAI_SELESAI", entry.Aksi)
	assert.Equal(t, "u", entry.AktorID)
	assert.Equal(t, "1 tugas", entry.EntitasID)
	assert.Equal(t, map[string]any{"ids": []idx.ID{1}, "gagal": 1}, entry.Metadata)
}

// Antrean dan tampilan PWA yang dibuat sebelum migrasi 000111 memegang kode lama (TGS-...) sebagai ID: aksi massal
// tetap melayani kode itu dan ID baru dalam satu permintaan, dan menghitung per rujukan.
func TestBulkAction_KodeLamaDanIDBaruDilayaniBersama(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	refs := []idx.RujukanID{refKode("TGS-mahasiswa_1-1"), refID(7), refKode("TGS-hilang-9")}
	repo.On("TandaiSelesai", mock.Anything, refs, hargadomain.CakupanMilik("u"), fixedNow).
		Return([]domain.TugasRef{tugasRef(3, "TGS-mahasiswa_1-1"), tugasRef(7, "")}, nil)

	hasil, err := uc.BulkAction(t.Context(), refs, "tandai_selesai", Pemanggil{ID: "u"})

	require.NoError(t, err)
	assert.Equal(t, HasilBulk{Berhasil: 2, Gagal: 1}, hasil)
	entry, _ := recorder.Last()
	assert.Equal(t, map[string]any{"ids": []idx.ID{3, 7}, "gagal": 1}, entry.Metadata, "audit mencatat ID BIGINT, bukan kode")
}

func TestBulkAction_UnduhIsReadOnly(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	repo.On("Aksesibel", mock.Anything, []idx.RujukanID{refID(1), refKode("TGS-x-1")}, hargadomain.CakupanMilik("u")).Return([]domain.TugasRef{tugasRef(1, "")}, nil)

	hasil, err := uc.BulkAction(t.Context(), []idx.RujukanID{refID(1), refKode("TGS-x-1")}, "unduh_data_acuan", Pemanggil{ID: "u"})

	require.NoError(t, err)
	assert.Equal(t, HasilBulk{Berhasil: 1, Gagal: 1}, hasil)
	assert.Empty(t, recorder.Entries(), "unduh tidak mengubah data")
	repo.AssertNotCalled(t, "TandaiSelesai", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestBulkAction_Validation(t *testing.T) {
	uc, _ := newTestUseCase(new(mockRepository))

	_, err := uc.BulkAction(t.Context(), []idx.RujukanID{refID(1)}, "hapus", Pemanggil{ID: "u"})
	assert.ErrorIs(t, err, ErrInvalidAction)

	_, err = uc.BulkAction(t.Context(), []idx.RujukanID{{}, {}}, "tandai_selesai", Pemanggil{ID: "u"})
	assert.ErrorIs(t, err, ErrValidasi)

	banyak := make([]idx.RujukanID, MaxBulkIDs+1)
	for i := range banyak {
		banyak[i] = refID(idx.ID(i + 1))
	}
	_, err = uc.BulkAction(t.Context(), banyak, "tandai_selesai", Pemanggil{ID: "u"})
	assert.ErrorIs(t, err, ErrValidasi)
}

func TestBulkAction_RepoErrorSkipsAudit(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	repo.On("TandaiSelesai", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("db mati"))

	_, err := uc.BulkAction(t.Context(), []idx.RujukanID{refID(1)}, "tandai_selesai", Pemanggil{ID: "u"})

	require.Error(t, err)
	assert.Empty(t, recorder.Entries())
}

func TestBulkAction_NothingChangedIsNotAudited(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	repo.On("TandaiSelesai", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]domain.TugasRef{}, nil)

	hasil, err := uc.BulkAction(t.Context(), []idx.RujukanID{refID(1)}, "tandai_selesai", Pemanggil{ID: "u"})

	require.NoError(t, err)
	assert.Equal(t, HasilBulk{Gagal: 1}, hasil)
	assert.Empty(t, recorder.Entries())
}

func TestAcuan_ComputesRangeAndStableVersion(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	raw := domain.DataAcuan{
		Komoditas: []domain.AcuanKomoditas{{Kode: "CBM-KRT", Nama: "Cabai Merah Keriting", Satuan: "kg", HargaAcuan: 50000}},
		Pasar:     []domain.AcuanPasar{{ID: "PSR-1171-01", Nama: "Pasar Aceh", KabupatenKode: "1171", RadiusMeter: 400}},
		Median:    []domain.MedianHarga{{KomoditasKode: "CBM-KRT", KabupatenKode: "1171", Tipe: "KONSUMEN", Median: 52349.6, Sampel: 8}},
	}
	repo.On("Acuan", mock.Anything, fixedNow.AddDate(0, 0, -HariAcuan), MinSampelAcuan).Return(raw, nil)

	first, err := uc.Acuan(t.Context())
	require.NoError(t, err)
	second, err := uc.Acuan(t.Context())
	require.NoError(t, err)

	require.Len(t, first.HargaAcuan, 1)
	assert.Equal(t, HargaAcuan{KomoditasKode: "CBM-KRT", KabupatenKode: "1171", Tipe: "KONSUMEN", Median: 52350, Bawah: 39300, Atas: 65400, Sampel: 8}, first.HargaAcuan[0])
	assert.Equal(t, ToleransiAcuan, first.Toleransi)
	assert.NotEmpty(t, first.Versi)
	assert.Equal(t, first.Versi, second.Versi, "isi sama menghasilkan versi sama")

	raw.Median[0].Median = 60000
	repo.ExpectedCalls = nil
	repo.On("Acuan", mock.Anything, mock.Anything, mock.Anything).Return(raw, nil)
	changed, err := uc.Acuan(t.Context())
	require.NoError(t, err)
	assert.NotEqual(t, first.Versi, changed.Versi, "isi berubah menghasilkan versi baru")
}

func TestAcuan_RepoError(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	repo.On("Acuan", mock.Anything, mock.Anything, mock.Anything).Return(domain.DataAcuan{}, errors.New("db mati"))

	_, err := uc.Acuan(t.Context())

	require.Error(t, err)
}
