package usecase

import (
	"context"
	"testing"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/audit/audittest"
	"github.com/garda-inflasi/api/internal/intervensi/domain"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	kabBandaAceh = "1171"
	kabPidie     = "1107"
)

func cakupanUC(peta map[string]domain.PenggunaCakupan) (*UseCase, *mockRepository, *mockWorkflow, *audittest.Recorder) {
	repo, wf, rec := new(mockRepository), new(mockWorkflow), &audittest.Recorder{}
	return New(repo, wf, stubCakupan{peta: peta}, rec, logger.NewDefault()), repo, wf, rec
}

func kartuDi(kab string) *domain.Intervensi {
	return &domain.Intervensi{ID: kid("INT-301"), Siklus: "TEKANAN", Tahap: "t-ews", KabupatenKode: kab}
}

func koordinator(kab string) map[string]domain.PenggunaCakupan {
	return map[string]domain.PenggunaCakupan{uid("koordinator"): {Peran: "OPS_KOORDINATOR", KabupatenKode: kab}}
}

// Pelaku bercakupan wilayah (Koordinator, Agregator, Mahasiswa, peran tak dikenal) hanya menjangkau kartu di
// kabupatennya; kartu lain dijawab tidak ditemukan, sama dengan kartu yang tidak ada.
func TestMove_CakupanWilayah(t *testing.T) {
	izin := allow("intervensi.kelola", "intervensi.setujui")
	skenario := []struct {
		nama    string
		peta    map[string]domain.PenggunaCakupan
		kartu   string
		diterus bool
	}{
		{"koordinator di kabupatennya", koordinator(kabBandaAceh), kabBandaAceh, true},
		{"koordinator di kabupaten lain", koordinator(kabBandaAceh), kabPidie, false},
		{"koordinator pada kartu tingkat provinsi", koordinator(kabBandaAceh), "", false},
		{"koordinator tanpa kabupaten tugas", koordinator(""), kabBandaAceh, false},
		{"mahasiswa", map[string]domain.PenggunaCakupan{uid("koordinator"): {Peran: "OPS_MAHASISWA", KabupatenKode: kabBandaAceh}}, kabBandaAceh, false},
		{"peran tak dikenal", map[string]domain.PenggunaCakupan{uid("koordinator"): {Peran: "PERAN_BARU"}}, kabBandaAceh, false},
		{"manajer tingkat provinsi", nil, kabPidie, true},
	}
	for _, sk := range skenario {
		t.Run(sk.nama, func(t *testing.T) {
			uc, repo, _, _ := cakupanUC(sk.peta)
			repo.On("GetByID", mock.Anything, rujukanID("INT-301")).Return(kartuDi(sk.kartu), nil)
			repo.On("ListTahap", mock.Anything).Return(defs(), nil)
			repo.On("Move", mock.Anything, kid("INT-301"), "t-petakan", 0, false, 1, uid("koordinator")).Return(2, nil)

			_, err := uc.Move(context.Background(), rujukanID("INT-301"), "t-petakan", 0, 1, uid("koordinator"), izin)

			if sk.diterus {
				require.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, ErrNotFound)
			repo.AssertNotCalled(t, "Move", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func TestMove_PenggunaTidakAdaGagalTutup(t *testing.T) {
	uc, repo, _, _ := cakupanUC(nil)
	uc.cakupan = penggunaHilang{}
	repo.On("GetByID", mock.Anything, rujukanID("INT-301")).Return(kartuDi(kabBandaAceh), nil)

	_, err := uc.Move(context.Background(), rujukanID("INT-301"), "t-petakan", 0, 1, uid("hantu"), allow("intervensi.setujui"))

	assert.ErrorIs(t, err, ErrNotFound)
}

type penggunaHilang struct{}

func (penggunaHilang) PenggunaCakupan(context.Context, string) (domain.PenggunaCakupan, error) {
	return domain.PenggunaCakupan{}, domain.ErrPenggunaTidakAda
}

func TestAjukan_DiLuarCakupanDitolakSebelumWorkflow(t *testing.T) {
	uc, repo, wf, _ := cakupanUC(koordinator(kabBandaAceh))
	repo.On("GetByID", mock.Anything, rujukanID("INT-301")).Return(kartuDi(kabPidie), nil)

	_, err := uc.Ajukan(context.Background(), rujukanID("INT-301"), "t-petakan", "", uid("koordinator"))

	assert.ErrorIs(t, err, ErrNotFound)
	wf.AssertNotCalled(t, "Ajukan", mock.Anything, mock.Anything)
}

func TestPutus_DiLuarCakupanDijawabTidakDitemukan(t *testing.T) {
	uc, _, wf, rec := cakupanUC(koordinator(kabBandaAceh))
	p := permintaan(domain.PersetujuanMenunggu, 1, "kelola")
	p.KartuKabupaten = kabPidie
	wf.On("GetPersetujuan", mock.Anything, idx.ID(7)).Return(p, nil)

	_, err := uc.Putus(context.Background(), 7, "SETUJU", "", uid("koordinator"), allow("intervensi.setujui"))

	assert.ErrorIs(t, err, ErrPersetujuanNotFound)
	wf.AssertNotCalled(t, "Putus", mock.Anything, mock.Anything)
	assert.Empty(t, rec.Entries())
}

func TestBulkAksi_KartuDiLuarCakupanDilaporkanTidakDitemukan(t *testing.T) {
	uc, repo, _, rec := cakupanUC(koordinator(kabBandaAceh))
	dalam, luar, hilang := kid("INT-1"), kid("INT-2"), kid("INT-3")
	repo.On("GetByID", mock.Anything, idx.RujukanUUID{ID: dalam}).Return(&domain.Intervensi{ID: dalam, KabupatenKode: kabBandaAceh}, nil)
	repo.On("GetByID", mock.Anything, idx.RujukanUUID{ID: luar}).Return(&domain.Intervensi{ID: luar, KabupatenKode: kabPidie}, nil)
	repo.On("GetByID", mock.Anything, idx.RujukanUUID{ID: hilang}).Return(nil, ErrNotFound)
	// Repository hanya menerima kartu yang lolos cakupan.
	repo.On("BulkAksi", mock.Anything, domain.BulkTugaskan, []string{dalam}, uid("koordinator")).
		Return([]domain.HasilKartu{{ID: dalam, Berhasil: true}}, nil)

	res, err := uc.BulkAksi(context.Background(), "tugaskan", []string{luar, dalam, hilang}, uid("koordinator"))

	require.NoError(t, err)
	assert.Equal(t, 1, res.Berhasil)
	assert.Equal(t, 2, res.Gagal)
	assert.Equal(t, []domain.HasilKartu{
		{ID: luar, Alasan: "Kartu tidak ditemukan"},
		{ID: dalam, Berhasil: true},
		{ID: hilang, Alasan: "Kartu tidak ditemukan"},
	}, res.Hasil, "urutan hasil mengikuti urutan permintaan")
	entry, _ := rec.Last()
	assert.Equal(t, map[string]any{"ids": []string{dalam}, "diminta": 3}, entry.Metadata)
}

func TestBulkAksi_PeranProvinsiTidakMembacaKartuSatuPerSatu(t *testing.T) {
	uc, repo, _, _ := cakupanUC(nil)
	id := kid("INT-1")
	repo.On("BulkAksi", mock.Anything, domain.BulkEskalasi, []string{id}, uid("manajer")).Return([]domain.HasilKartu{{ID: id, Berhasil: true}}, nil)

	_, err := uc.BulkAksi(context.Background(), "eskalasi", []string{id}, uid("manajer"))

	require.NoError(t, err)
	repo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
}

// Setiap penolakan pemisahan tugas dicatat sebagai DITOLAK di audit, baik yang tertangkap di usecase maupun
// yang baru terdeteksi di dalam transaksi repository (balapan).
func TestPutus_PemisahanTugasDiaudit(t *testing.T) {
	t.Run("tertangkap di usecase", func(t *testing.T) {
		uc, _, wf, rec := cakupanUC(nil)
		wf.On("GetPersetujuan", mock.Anything, idx.ID(7)).Return(permintaan(domain.PersetujuanMenunggu, 1, "pengaju"), nil)

		_, err := uc.Putus(context.Background(), 7, "SETUJU", "", "pengaju", allow("intervensi.setujui"))

		assert.ErrorIs(t, err, ErrPemisahanTugas)
		wf.AssertNotCalled(t, "Putus", mock.Anything, mock.Anything)
		entry, ok := rec.Last()
		require.True(t, ok)
		assert.Equal(t, "PUTUS_PERSETUJUAN", entry.Aksi)
		assert.Equal(t, audit.HasilDitolak, entry.Hasil)
		assert.Equal(t, "pemisahan_tugas", entry.Metadata["sebab"])
		assert.Equal(t, "pengaju", entry.AktorID)
	})

	t.Run("terdeteksi di dalam transaksi", func(t *testing.T) {
		uc, _, wf, rec := cakupanUC(nil)
		// Potret usecase belum memuat keterlibatan pemutus; repository menemukannya di bawah kunci.
		wf.On("GetPersetujuan", mock.Anything, idx.ID(7)).Return(permintaan(domain.PersetujuanMenunggu, 1, "pengaju"), nil)
		wf.On("Putus", mock.Anything, mock.Anything).Return(nil, domain.ErrPemisahanTugas)

		_, err := uc.Putus(context.Background(), 7, "SETUJU", "", "penyetuju", allow("intervensi.setujui"))

		assert.ErrorIs(t, err, ErrPemisahanTugas)
		entry, _ := rec.Last()
		assert.Equal(t, audit.HasilDitolak, entry.Hasil)
	})
}

func TestMelibatkanPelaku(t *testing.T) {
	p := permintaan(domain.PersetujuanMenunggu, 2, "pengaju", "pemutus1")

	assert.True(t, p.MelibatkanPelaku("pengaju"), "pengaju")
	assert.True(t, p.MelibatkanPelaku("pemutus1"), "pemutus langkah sebelumnya")
	assert.False(t, p.MelibatkanPelaku("orang-ketiga"))
	assert.False(t, p.MelibatkanPelaku(""), "sistem tidak pernah dianggap terlibat")
	sistem := permintaan(domain.PersetujuanMenunggu, 1, "x")
	sistem.DiajukanOleh = nil
	assert.False(t, sistem.MelibatkanPelaku("siapa-pun"), "permintaan buatan sistem tanpa pengaju")
}
