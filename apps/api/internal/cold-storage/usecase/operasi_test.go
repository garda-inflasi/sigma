package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/audit/audittest"
	"github.com/garda-inflasi/api/internal/cold-storage/domain"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubOps adalah OperasiRepository minimal untuk menguji validasi usecase. fasilitas dikunci menurut UUID; kode
// memetakan kode fasilitas (alias CS-01) ke UUID-nya.
type stubOps struct {
	domain.OperasiRepository
	fasilitas map[string]domain.FasilitasView
	kode      map[string]string
	saved     []domain.TelemetriSuhu
	trx       []domain.TransaksiInput
	trxErr    map[idx.ID]error
	diubah    []string
	cariKode  []string
}

func (s *stubOps) IDFasilitasDariKode(_ context.Context, kode string) (string, error) {
	s.cariKode = append(s.cariKode, kode)
	id, ok := s.kode[kode]
	if !ok {
		return "", domain.ErrFasilitasTidakAda
	}
	return id, nil
}

func (s *stubOps) GetFasilitas(_ context.Context, id string) (*domain.FasilitasView, error) {
	f, ok := s.fasilitas[id]
	if !ok {
		return nil, domain.ErrFasilitasTidakAda
	}
	return &f, nil
}

func (s *stubOps) UpdateFasilitas(_ context.Context, id string, _ domain.FasilitasPatch) (before, after domain.FasilitasView, err error) {
	s.diubah = append(s.diubah, id)
	f, ok := s.fasilitas[id]
	if !ok {
		return before, after, domain.ErrFasilitasTidakAda
	}
	return f, f, nil
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
	// Basis data yang memberi ID: transaksi berurutan, lot MASUK mendapat ID baru.
	hasil := domain.TransaksiHasil{TransaksiID: idx.ID(len(s.trx)), Jenis: in.Jenis, LotID: in.LotID, VolumeTon: 10}
	if in.Jenis == domain.TransaksiMasuk {
		hasil.LotID = 500
	}
	return hasil, nil
}

func newOps(repo *stubOps) (*Operasi, *audittest.Recorder) {
	rec := &audittest.Recorder{}
	return NewOperasi(repo, logger.NewDefault(), rec).WithClock(func() time.Time { return fixedNow }), rec
}

// repoFasilitas membuat stub dengan fasilitas yang dikunci menurut UUID dan dapat dicari menurut kode.
func repoFasilitas(f map[string]domain.FasilitasView) *stubOps {
	repo := &stubOps{fasilitas: map[string]domain.FasilitasView{}, kode: map[string]string{}}
	for kode, v := range f {
		v.ID = fas(kode)
		repo.fasilitas[v.ID] = v
		repo.kode[kode] = v.ID
	}
	return repo
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
	repo := repoFasilitas(map[string]domain.FasilitasView{
		"CS-01": {SuhuTargetC: 4, Status: domain.StatusFasilitasOperasional},
		"CS-05": {SuhuTargetC: 4, Status: domain.StatusFasilitasRencana},
	})
	ops, _ := newOps(repo)
	ok := Pembacaan{FasilitasID: "CS-01", Waktu: fixedNow.Add(-time.Minute), SuhuC: 4}

	cases := map[string]struct {
		in   []Pembacaan
		want error
	}{
		"kosong":               {nil, nil},
		"fasilitas tak ada":    {[]Pembacaan{ok, {FasilitasID: "CS-99", Waktu: fixedNow, SuhuC: 4}}, domain.ErrFasilitasTidakAda},
		"uuid tak ada":         {[]Pembacaan{{FasilitasID: fas("CS-99"), Waktu: fixedNow, SuhuC: 4}}, domain.ErrFasilitasTidakAda},
		"fasilitas rencana":    {[]Pembacaan{{FasilitasID: "CS-05", Waktu: fixedNow, SuhuC: 4}}, domain.ErrFasilitasTidakAktif},
		"fasilitas kosong":     {[]Pembacaan{{FasilitasID: "  ", Waktu: fixedNow, SuhuC: 4}}, nil},
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
	repo := repoFasilitas(map[string]domain.FasilitasView{
		"CS-01": {SuhuTargetC: 4, Status: domain.StatusFasilitasOperasional},
	})
	ops, rec := newOps(repo)

	// Satu fasilitas ditulis dengan kode pada satu pembacaan dan dengan UUID pada pembacaan lain.
	out, err := ops.Ingest(context.Background(), IngestInput{ActorID: uid("u1"), Pembacaan: []Pembacaan{
		{FasilitasID: "CS-01", Waktu: fixedNow.Add(-2 * time.Minute), SuhuC: 7.5},
		{FasilitasID: " " + fas("CS-01") + " ", Waktu: fixedNow.Add(-time.Minute), SuhuC: 4.0},
	}})

	require.NoError(t, err)
	assert.Equal(t, 2, out.Diterima)
	require.Len(t, out.DiLuarRentang, 1)
	assert.Equal(t, 7.5, out.DiLuarRentang[0].SuhuC)
	assert.Equal(t, fas("CS-01"), out.DiLuarRentang[0].FasilitasID, "alarm memakai UUID, bukan teks isian")
	assert.Equal(t, domain.SumberSensor, repo.saved[0].Sumber, "sumber bawaan SENSOR")
	for _, row := range repo.saved {
		assert.Equal(t, fas("CS-01"), row.FasilitasID, "baris tersimpan memakai UUID, termasuk saat dikirim dengan kode")
	}
	assert.Equal(t, []string{"CS-01"}, repo.cariKode, "kode dicari sekali, UUID tidak menyentuh basis data untuk pencarian")
	last, found := rec.Last()
	require.True(t, found)
	assert.Equal(t, "INGEST_TELEMETRI", last.Aksi)
	assert.Equal(t, uid("u1"), last.AktorID)
	assert.Equal(t, fas("CS-01"), last.EntitasID, "satu fasilitas: entitas_id adalah UUID-nya")
}

func TestIngest_AuditMultiFacilityUsesSummaryWithIDsInMetadata(t *testing.T) {
	repo := repoFasilitas(map[string]domain.FasilitasView{
		"CS-01": {SuhuTargetC: 4, Status: domain.StatusFasilitasOperasional},
		"CS-02": {SuhuTargetC: -20, Status: domain.StatusFasilitasOperasional},
	})
	ops, rec := newOps(repo)

	_, err := ops.Ingest(context.Background(), IngestInput{Pembacaan: []Pembacaan{
		{FasilitasID: "CS-02", Waktu: fixedNow.Add(-time.Minute), SuhuC: -20},
		{FasilitasID: "CS-01", Waktu: fixedNow.Add(-time.Minute), SuhuC: 4},
	}})

	require.NoError(t, err)
	last, found := rec.Last()
	require.True(t, found)
	assert.Equal(t, "2 fasilitas", last.EntitasID, "kolom entitas_id terbatas 255 karakter; daftar UUID di metadata")
	ids := []string{fas("CS-01"), fas("CS-02")}
	if ids[0] > ids[1] {
		ids[0], ids[1] = ids[1], ids[0]
	}
	assert.Equal(t, ids, last.Metadata["fasilitas_ids"])
}

func TestDetail_ResolvesCodeAliasAndUnknownCodeIsNotFound(t *testing.T) {
	repo := repoFasilitas(map[string]domain.FasilitasView{
		"CS-01": {SuhuTargetC: 4, Status: domain.StatusFasilitasOperasional},
	})
	repo.OperasiRepository = &repoKosong{}
	ops, _ := newOps(repo)

	_, err := ops.Detail(context.Background(), idx.RujukanUUID{Kode: "CS-99"}, 0)
	assert.ErrorIs(t, err, domain.ErrFasilitasTidakAda)

	_, err = ops.Detail(context.Background(), idx.RujukanUUID{}, 0)
	assert.ErrorIs(t, err, domain.ErrFasilitasTidakAda, "rujukan kosong tidak pernah sampai ke basis data")

	d, err := ops.Detail(context.Background(), idx.RujukanUUID{Kode: "CS-01"}, 0)
	require.NoError(t, err)
	assert.Equal(t, fas("CS-01"), d.Fasilitas.ID)
	d, err = ops.Detail(context.Background(), idx.RujukanUUID{ID: fas("CS-01")}, 0)
	require.NoError(t, err)
	assert.Equal(t, fas("CS-01"), d.Fasilitas.ID)
}

// repoKosong melengkapi bagian OperasiRepository yang dibaca Detail (telemetri, stok, transaksi) dengan hasil kosong.
type repoKosong struct{ domain.OperasiRepository }

func (*repoKosong) ListTelemetri(context.Context, string, time.Time, int) ([]domain.TelemetriSuhu, error) {
	return nil, nil
}
func (*repoKosong) StokFasilitas(context.Context, string) ([]domain.StokView, error) { return nil, nil }
func (*repoKosong) ListTransaksi(context.Context, string, int) ([]domain.TransaksiView, error) {
	return nil, nil
}

func TestTransaksi_RequestValidation(t *testing.T) {
	repo := repoFasilitas(map[string]domain.FasilitasView{"CS-01": {Status: domain.StatusFasilitasOperasional}})
	ops, rec := newOps(repo)
	past := fixedNow.Add(-time.Hour)

	cases := map[string]TransaksiRequest{
		"jenis tak dikenal":       {Jenis: "BUANG", VolumeTon: 1},
		"masuk tanpa pemilik":     {Jenis: "MASUK", FasilitasID: "CS-01", KomoditasKode: "TLR", VolumeTon: 1},
		"masuk volume nol":        {Jenis: "MASUK", FasilitasID: "CS-01", KomoditasKode: "TLR", Pemilik: "x"},
		"masuk kedaluwarsa lalu":  {Jenis: "MASUK", FasilitasID: "CS-01", KomoditasKode: "TLR", Pemilik: "x", VolumeTon: 1, KedaluwarsaPada: &past},
		"keluar tanpa lot":        {Jenis: "KELUAR", Alasan: "x", VolumeTon: 1},
		"keluar tanpa alasan":     {Jenis: "KELUAR", LotID: 1, VolumeTon: 1},
		"keluar tanpa volume":     {Jenis: "KELUAR", LotID: 1, Alasan: "x"},
		"pindah tanpa tujuan":     {Jenis: "PINDAH", LotID: 1, VolumeTon: 1},
		"pindah tanpa lot":        {Jenis: "PINDAH", FasilitasTujuanID: "CS-01", VolumeTon: 1},
		"volume tak masuk akal":   {Jenis: "KELUAR", LotID: 1, Alasan: "x", VolumeTon: 1e9},
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

func TestTransaksi_UnknownFacilityCodeIsRejectedBeforeTheLedger(t *testing.T) {
	repo := repoFasilitas(nil)
	ops, rec := newOps(repo)

	_, err := ops.Transaksi(context.Background(), TransaksiRequest{
		Jenis: "MASUK", FasilitasID: "CS-99", KomoditasKode: "TLR", Pemilik: "x", VolumeTon: 1,
	})
	assert.ErrorIs(t, err, domain.ErrFasilitasTidakAda)
	_, err = ops.Transaksi(context.Background(), TransaksiRequest{Jenis: "PINDAH", LotID: 1, FasilitasTujuanID: "CS-99", VolumeTon: 1})
	assert.ErrorIs(t, err, domain.ErrFasilitasTidakAda)

	assert.Empty(t, repo.trx, "buku besar tidak disentuh")
	_, ada := rec.Last()
	assert.False(t, ada)
}

func TestTransaksi_BuildsInputWithoutAppIDsAndAuditsDomainResult(t *testing.T) {
	repo := repoFasilitas(map[string]domain.FasilitasView{
		"CS-03": {Status: domain.StatusFasilitasOperasional},
	})
	ops, rec := newOps(repo)

	hasil, err := ops.Transaksi(context.Background(), TransaksiRequest{
		Jenis: "PINDAH", LotID: 7, FasilitasTujuanID: "CS-03", VolumeTon: 10, ActorID: uid("u1"),
	})

	require.NoError(t, err)
	require.Len(t, repo.trx, 1)
	in := repo.trx[0]
	assert.Equal(t, fixedNow, in.Waktu)
	assert.Equal(t, uid("u1"), in.OlehID)
	assert.Equal(t, idx.ID(7), in.LotID)
	assert.Equal(t, fas("CS-03"), in.FasilitasTujuanID, "alias kode dipetakan ke UUID sebelum ke repositori")
	assert.Equal(t, idx.ID(1), hasil.TransaksiID, "ID transaksi datang dari repositori (RETURNING), bukan dari aplikasi")
	last, _ := rec.Last()
	assert.Equal(t, "TRANSAKSI_STOK_PINDAH", last.Aksi)
	assert.Equal(t, "7", last.EntitasID)
	assert.Equal(t, "1", last.Metadata["transaksi_id"])
}

func TestTransaksi_MasukUsesFacilityUUIDAndLotIDFromDatabase(t *testing.T) {
	repo := repoFasilitas(map[string]domain.FasilitasView{"CS-01": {Status: domain.StatusFasilitasOperasional}})
	ops, rec := newOps(repo)

	// UUID huruf besar dinormalkan; lot_id yang dikirim klien pada MASUK diabaikan.
	hasil, err := ops.Transaksi(context.Background(), TransaksiRequest{
		Jenis: "MASUK", LotID: 99, FasilitasID: " " + upperUUID(fas("CS-01")) + " ", KomoditasKode: "TLR", Pemilik: "KUD", VolumeTon: 3, ActorID: uid("u1"),
	})

	require.NoError(t, err)
	require.Len(t, repo.trx, 1)
	assert.Equal(t, fas("CS-01"), repo.trx[0].FasilitasID)
	assert.Zero(t, repo.trx[0].LotID, "lot baru dibuat repositori")
	assert.Equal(t, idx.ID(500), hasil.LotID)
	assert.Empty(t, repo.cariKode, "UUID tidak dicari menurut kode")
	last, _ := rec.Last()
	assert.Equal(t, "500", last.EntitasID)
}

func upperUUID(s string) string {
	out := []byte(s)
	for i, c := range out {
		if c >= 'a' && c <= 'f' {
			out[i] = c - 32
		}
	}
	return string(out)
}

func TestTransaksi_DomainRejectionIsPassedThroughWithoutAudit(t *testing.T) {
	repo := &stubOps{trxErr: map[idx.ID]error{1: domain.ErrStokTidakCukup}}
	ops, rec := newOps(repo)

	_, err := ops.Transaksi(context.Background(), TransaksiRequest{Jenis: "KELUAR", LotID: 1, VolumeTon: 99, Alasan: "x"})

	assert.ErrorIs(t, err, domain.ErrStokTidakCukup)
	_, ada := rec.Last()
	assert.False(t, ada)
}

func TestBulkMutasi_BuangReportsPerLot(t *testing.T) {
	repo := &stubOps{trxErr: map[idx.ID]error{
		2: domain.ErrLotHabis,
		3: errors.New("koneksi putus"),
	}}
	ops, rec := newOps(repo)

	out, err := ops.BulkMutasi(context.Background(), BulkInput{Action: "buang", IDs: []idx.ID{1, 2, 3, 1}, ActorID: uid("u1")})

	require.NoError(t, err)
	assert.Equal(t, 1, out.Berhasil)
	assert.Equal(t, 2, out.Gagal)
	assert.Equal(t, 10.0, out.VolumeTon)
	require.Len(t, out.Baris, 3, "id ganda dihitung sekali")
	assert.True(t, out.Baris[0].Berhasil)
	assert.NotZero(t, out.Baris[0].TransaksiID)
	assert.Contains(t, out.Baris[1].Pesan, "habis")
	assert.Equal(t, "Gagal memproses lot", out.Baris[2].Pesan, "galat sistem tidak dibocorkan ke pengguna")
	assert.Equal(t, alasanBuang, repo.trx[0].Alasan)
	assert.True(t, repo.trx[0].Semua)
	last, _ := rec.Last()
	assert.Equal(t, "BULK_buang", last.Aksi)
	assert.Equal(t, []string{"1"}, last.Metadata["transaksi_ids"])
}

func TestBulkMutasi_RejectsInvalidInput(t *testing.T) {
	ops, _ := newOps(&stubOps{})

	_, err := ops.BulkMutasi(context.Background(), BulkInput{Action: "rekomendasi_olahan", IDs: []idx.ID{1}})
	assert.ErrorIs(t, err, ErrInvalidAction, "aksi advisori bukan urusan modul ini")
	_, err = ops.BulkMutasi(context.Background(), BulkInput{Action: "buang", IDs: []idx.ID{0, 0}})
	assert.ErrorIs(t, err, ErrNoIDs)
}

func TestUbahFasilitas_Validation(t *testing.T) {
	ops, _ := newOps(&stubOps{})
	nol, status := 0, "TUTUP"
	ref := idx.RujukanUUID{Kode: "CS-01"}

	_, err := ops.UbahFasilitas(context.Background(), ref, domain.FasilitasPatch{}, uid("u1"))
	var input *InputError
	assert.ErrorAs(t, err, &input)
	_, err = ops.UbahFasilitas(context.Background(), ref, domain.FasilitasPatch{KapasitasTon: &nol}, uid("u1"))
	assert.ErrorAs(t, err, &input)
	_, err = ops.UbahFasilitas(context.Background(), ref, domain.FasilitasPatch{Status: &status}, uid("u1"))
	assert.ErrorAs(t, err, &input)
}

func TestUbahFasilitas_AliasResolvedAndAuditUsesUUID(t *testing.T) {
	repo := repoFasilitas(map[string]domain.FasilitasView{"CS-01": {Status: domain.StatusFasilitasOperasional, KapasitasTon: 100}})
	ops, rec := newOps(repo)
	kapasitas := 150

	_, err := ops.UbahFasilitas(context.Background(), idx.RujukanUUID{Kode: "CS-01"}, domain.FasilitasPatch{KapasitasTon: &kapasitas}, uid("u1"))

	require.NoError(t, err)
	assert.Equal(t, []string{fas("CS-01")}, repo.diubah, "repositori menerima UUID, bukan kode")
	last, _ := rec.Last()
	assert.Equal(t, fas("CS-01"), last.EntitasID, "audit mencatat UUID walau URL memakai kode")

	_, err = ops.UbahFasilitas(context.Background(), idx.RujukanUUID{Kode: "CS-99"}, domain.FasilitasPatch{KapasitasTon: &kapasitas}, uid("u1"))
	assert.ErrorIs(t, err, domain.ErrFasilitasTidakAda)
	assert.Len(t, repo.diubah, 1, "kode yang tidak ada tidak sampai ke UPDATE")
}
