package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/audit/audittest"
	"github.com/garda-inflasi/api/internal/cold-storage/domain"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubOps adalah OperasiRepository minimal untuk menguji validasi usecase.
type stubOps struct {
	domain.OperasiRepository
	fasilitas map[string]domain.FasilitasView
	saved     []domain.TelemetriSuhu
	trx       []domain.TransaksiInput
	trxErr    map[string]error
}

func (s *stubOps) GetFasilitas(_ context.Context, id string) (*domain.FasilitasView, error) {
	f, ok := s.fasilitas[id]
	if !ok {
		return nil, domain.ErrFasilitasTidakAda
	}
	return &f, nil
}

func (s *stubOps) SimpanTelemetri(_ context.Context, rows []domain.TelemetriSuhu) (int64, error) {
	s.saved = append(s.saved, rows...)
	return int64(len(rows)), nil
}

func (s *stubOps) CatatTransaksi(_ context.Context, in domain.TransaksiInput) (domain.TransaksiHasil, error) {
	s.trx = append(s.trx, in)
	if err := s.trxErr[in.LotID]; err != nil {
		return domain.TransaksiHasil{}, err
	}
	return domain.TransaksiHasil{TransaksiID: in.ID, Jenis: in.Jenis, LotID: in.LotID, VolumeTon: 10}, nil
}

func newOps(repo *stubOps) (*Operasi, *audittest.Recorder) {
	rec := &audittest.Recorder{}
	return NewOperasi(repo, logger.NewDefault(), rec).WithClock(func() time.Time { return fixedNow }), rec
}

func ptr(f float64) *float64 { return &f }

func TestBatasSuhu(t *testing.T) {
	lo, hi := domain.BatasSuhu(4, nil, nil)
	assert.Equal(t, [2]float64{2, 6}, [2]float64{lo, hi}, "target ± toleransi bawaan")

	lo, hi = domain.BatasSuhu(4, ptr(3), nil)
	assert.Equal(t, [2]float64{3, 6}, [2]float64{lo, hi}, "hanya batas yang diisi menggantikan bawaan")

	assert.False(t, domain.DiLuarRentang(6, 2, 6), "batas inklusif")
	assert.True(t, domain.DiLuarRentang(6.01, 2, 6))
	assert.True(t, domain.DiLuarRentang(1.99, 2, 6))
}

func TestIsiTurunan_AlarmFollowsActualTemperature(t *testing.T) {
	f := domain.FasilitasView{KapasitasTon: 200, TerisiTon: 99.96, SuhuTargetC: -20, SuhuAktualC: ptr(-17.5)}
	isiTurunan(&f)
	assert.True(t, f.Alarm, "-17,5 di atas batas -18")
	assert.Equal(t, 100.0, f.TerisiTon)
	assert.Equal(t, 50.0, f.UtilisasiPersen)

	f = domain.FasilitasView{SuhuTargetC: 4}
	isiTurunan(&f)
	assert.False(t, f.Alarm, "tanpa telemetri tidak ada alarm")
}

func TestIngest_ValidatesBeforeStoring(t *testing.T) {
	repo := &stubOps{fasilitas: map[string]domain.FasilitasView{
		"CS-01": {ID: "CS-01", SuhuTargetC: 4, Status: domain.StatusFasilitasOperasional},
		"CS-05": {ID: "CS-05", SuhuTargetC: 4, Status: domain.StatusFasilitasRencana},
	}}
	ops, _ := newOps(repo)
	ok := Pembacaan{FasilitasID: "CS-01", Waktu: fixedNow.Add(-time.Minute), SuhuC: 4}

	cases := map[string]struct {
		in   []Pembacaan
		want error
	}{
		"kosong":               {nil, nil},
		"fasilitas tak ada":    {[]Pembacaan{ok, {FasilitasID: "CS-99", Waktu: fixedNow, SuhuC: 4}}, domain.ErrFasilitasTidakAda},
		"fasilitas rencana":    {[]Pembacaan{{FasilitasID: "CS-05", Waktu: fixedNow, SuhuC: 4}}, domain.ErrFasilitasTidakAktif},
		"suhu di luar fisik":   {[]Pembacaan{{FasilitasID: "CS-01", Waktu: fixedNow, SuhuC: -90}}, nil},
		"waktu masa depan":     {[]Pembacaan{{FasilitasID: "CS-01", Waktu: fixedNow.Add(time.Hour), SuhuC: 4}}, nil},
		"waktu kosong":         {[]Pembacaan{{FasilitasID: "CS-01", SuhuC: 4}}, nil},
		"kelembapan > 100":     {[]Pembacaan{{FasilitasID: "CS-01", Waktu: fixedNow, SuhuC: 4, KelembapanPct: ptr(101)}}, nil},
		"sumber tidak dikenal": {[]Pembacaan{{FasilitasID: "CS-01", Waktu: fixedNow, SuhuC: 4, Sumber: "DEMO"}}, nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ops.Ingest(context.Background(), IngestInput{Pembacaan: tc.in})

			require.Error(t, err)
			if tc.want != nil {
				assert.ErrorIs(t, err, tc.want)
			} else {
				var input *InputError
				assert.ErrorAs(t, err, &input)
			}
		})
	}
	assert.Empty(t, repo.saved, "kiriman yang ditolak tidak menyimpan apa pun")
}

func TestIngest_ReportsAlarmsAndAudits(t *testing.T) {
	repo := &stubOps{fasilitas: map[string]domain.FasilitasView{
		"CS-01": {ID: "CS-01", SuhuTargetC: 4, Status: domain.StatusFasilitasOperasional},
	}}
	ops, rec := newOps(repo)

	out, err := ops.Ingest(context.Background(), IngestInput{ActorID: uid("u1"), Pembacaan: []Pembacaan{
		{FasilitasID: "CS-01", Waktu: fixedNow.Add(-2 * time.Minute), SuhuC: 7.5},
		{FasilitasID: "CS-01", Waktu: fixedNow.Add(-time.Minute), SuhuC: 4.0},
	}})

	require.NoError(t, err)
	assert.Equal(t, 2, out.Diterima)
	require.Len(t, out.DiLuarRentang, 1)
	assert.Equal(t, 7.5, out.DiLuarRentang[0].SuhuC)
	assert.Equal(t, domain.SumberSensor, repo.saved[0].Sumber, "sumber bawaan SENSOR")
	last, found := rec.Last()
	require.True(t, found)
	assert.Equal(t, "INGEST_TELEMETRI", last.Aksi)
	assert.Equal(t, uid("u1"), last.AktorID)
}

func TestTransaksi_RequestValidation(t *testing.T) {
	repo := &stubOps{}
	ops, rec := newOps(repo)
	past := fixedNow.Add(-time.Hour)

	cases := map[string]TransaksiRequest{
		"jenis tak dikenal":       {Jenis: "BUANG", VolumeTon: 1},
		"masuk tanpa pemilik":     {Jenis: "MASUK", FasilitasID: "CS-01", KomoditasKode: "TLR", VolumeTon: 1},
		"masuk volume nol":        {Jenis: "MASUK", FasilitasID: "CS-01", KomoditasKode: "TLR", Pemilik: "x"},
		"masuk kedaluwarsa lalu":  {Jenis: "MASUK", FasilitasID: "CS-01", KomoditasKode: "TLR", Pemilik: "x", VolumeTon: 1, KedaluwarsaPada: &past},
		"keluar tanpa alasan":     {Jenis: "KELUAR", LotID: "L", VolumeTon: 1},
		"keluar tanpa volume":     {Jenis: "KELUAR", LotID: "L", Alasan: "x"},
		"pindah tanpa tujuan":     {Jenis: "PINDAH", LotID: "L", VolumeTon: 1},
		"volume tak masuk akal":   {Jenis: "KELUAR", LotID: "L", Alasan: "x", VolumeTon: 1e9},
		"masuk dengan semua=true": {Jenis: "MASUK", FasilitasID: "CS-01", KomoditasKode: "TLR", Pemilik: "x", Semua: true},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ops.Transaksi(context.Background(), req)

			var input *InputError
			assert.ErrorAs(t, err, &input)
		})
	}
	assert.Empty(t, repo.trx)
	_, ada := rec.Last()
	assert.False(t, ada, "validasi gagal tidak menulis audit")
}

func TestTransaksi_BuildsInputAndAuditsDomainResult(t *testing.T) {
	repo := &stubOps{}
	ops, rec := newOps(repo)

	hasil, err := ops.Transaksi(context.Background(), TransaksiRequest{
		Jenis: "PINDAH", LotID: "STK-1", FasilitasTujuanID: "CS-03", VolumeTon: 10, ActorID: uid("u1"),
	})

	require.NoError(t, err)
	require.Len(t, repo.trx, 1)
	in := repo.trx[0]
	assert.Equal(t, fixedNow, in.Waktu)
	assert.Equal(t, uid("u1"), in.OlehID)
	assert.Regexp(t, `^TRX-[0-9A-F]{12}$`, in.ID)
	assert.Regexp(t, `^STK-[0-9A-F]{12}$`, in.LotBaruID, "id lot tujuan dibuat bila pindah sebagian")
	last, _ := rec.Last()
	assert.Equal(t, "TRANSAKSI_STOK_PINDAH", last.Aksi)
	assert.Equal(t, hasil.LotID, last.EntitasID)
}

func TestTransaksi_DomainRejectionIsPassedThroughWithoutAudit(t *testing.T) {
	repo := &stubOps{trxErr: map[string]error{"STK-1": domain.ErrStokTidakCukup}}
	ops, rec := newOps(repo)

	_, err := ops.Transaksi(context.Background(), TransaksiRequest{Jenis: "KELUAR", LotID: "STK-1", VolumeTon: 99, Alasan: "x"})

	assert.ErrorIs(t, err, domain.ErrStokTidakCukup)
	_, ada := rec.Last()
	assert.False(t, ada)
}

func TestBulkMutasi_BuangReportsPerLot(t *testing.T) {
	repo := &stubOps{trxErr: map[string]error{
		"HABIS": domain.ErrLotHabis,
		"RUSAK": errors.New("koneksi putus"),
	}}
	ops, rec := newOps(repo)

	out, err := ops.BulkMutasi(context.Background(), BulkInput{Action: "buang", IDs: []string{"A", "HABIS", "RUSAK", "A"}, ActorID: uid("u1")})

	require.NoError(t, err)
	assert.Equal(t, 1, out.Berhasil)
	assert.Equal(t, 2, out.Gagal)
	assert.Equal(t, 10.0, out.VolumeTon)
	require.Len(t, out.Baris, 3, "id ganda dihitung sekali")
	assert.True(t, out.Baris[0].Berhasil)
	assert.NotEmpty(t, out.Baris[0].TransaksiID)
	assert.Contains(t, out.Baris[1].Pesan, "habis")
	assert.Equal(t, "Gagal memproses lot", out.Baris[2].Pesan, "galat sistem tidak dibocorkan ke pengguna")
	assert.Equal(t, alasanBuang, repo.trx[0].Alasan)
	assert.True(t, repo.trx[0].Semua)
	last, _ := rec.Last()
	assert.Equal(t, "BULK_buang", last.Aksi)
}

func TestBulkMutasi_RejectsInvalidInput(t *testing.T) {
	ops, _ := newOps(&stubOps{})

	_, err := ops.BulkMutasi(context.Background(), BulkInput{Action: "rekomendasi_olahan", IDs: []string{"A"}})
	assert.ErrorIs(t, err, ErrInvalidAction, "aksi advisori bukan urusan modul ini")
	_, err = ops.BulkMutasi(context.Background(), BulkInput{Action: "buang", IDs: []string{" ", ""}})
	assert.ErrorIs(t, err, ErrNoIDs)
}

func TestUbahFasilitas_Validation(t *testing.T) {
	ops, _ := newOps(&stubOps{})
	nol, status := 0, "TUTUP"

	_, err := ops.UbahFasilitas(context.Background(), "CS-01", domain.FasilitasPatch{}, uid("u1"))
	var input *InputError
	assert.ErrorAs(t, err, &input)
	_, err = ops.UbahFasilitas(context.Background(), "CS-01", domain.FasilitasPatch{KapasitasTon: &nol}, uid("u1"))
	assert.ErrorAs(t, err, &input)
	_, err = ops.UbahFasilitas(context.Background(), "CS-01", domain.FasilitasPatch{Status: &status}, uid("u1"))
	assert.ErrorAs(t, err, &input)
}
