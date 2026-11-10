package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/dashboard/domain"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 16 Oktober 2026 20.00 UTC = 17 Oktober 03.00 WIB: "hari ini" menurut waktu Aceh adalah 17 Oktober.
var (
	sekarangUji = time.Date(2026, 10, 16, 20, 0, 0, 0, time.UTC)
	hariUji     = time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC)
)

func newUC(repo *fakeRepo, opts ...Option) *UseCase {
	uc := New(repo, logger.NewDefault(), opts...)
	uc.now = func() time.Time { return sekarangUji }
	return uc
}

func tgl(s string) time.Time {
	t, err := time.Parse(domain.TanggalLayout, s)
	if err != nil {
		panic(err)
	}
	return t
}

var (
	semuaAceh = domain.CakupanSemua()
	banda     = domain.CakupanKabupaten("1171")
	tertutup  = domain.Cakupan{}
)

func TestGetKpiOperasional_MerakitDariFaktaDanTarget(t *testing.T) {
	repo := &fakeRepo{
		deretKpi: func(domain.DeretKpiQuery) ([]domain.TitikKpi, error) {
			return []domain.TitikKpi{{Indikator: domain.IndikatorEntri, Tanggal: hariUji, Pembilang: 10}}, nil
		},
		targets: []domain.TargetKpi{{Indikator: "waspada", Target: 3, Arah: domain.ArahTurun}},
	}

	kpis, err := newUC(repo).GetKpiOperasional(t.Context(), semuaAceh, domain.PermintaanFilter{})

	require.NoError(t, err)
	require.Len(t, kpis, 4)
	assert.Equal(t, 10.0, kartu(kpis, "entri").Nilai)
	assert.Equal(t, "normal", kartu(kpis, "waspada").Tone, "nol komoditas di atas ambang, target turun 3")
	assert.Equal(t, hariUji, repo.qTarget, "target yang berlaku pada tanggal acuan")
	assert.Equal(t, domain.DeretKpiQuery{
		Cakupan: semuaAceh,
		// 14 hari tren berakhir hari ini (tren mulai 28 Okt... 4 Okt), ditambah 6 hari jendela validasi di belakangnya.
		Awal:  hariUji.AddDate(0, 0, -(domain.BandingHari + domain.JendelaValidasiHari - 1)),
		Akhir: hariUji,
	}, repo.qKpi)
}

func TestGetKpiOperasional_MembacaFaktaSejauhYangDibutuhkanDeltaDanJendelaValidasi(t *testing.T) {
	cases := []struct {
		nama     string
		req      domain.PermintaanFilter
		wantAwal string
	}{
		{"bawaan 14 hari: 13 hari banding + 6 hari jendela", domain.PermintaanFilter{}, "2026-09-28"},
		{"periode satu hari: tetap 13 hari banding untuk delta", domain.PermintaanFilter{Dari: "2026-10-17"}, "2026-09-28"},
		{"periode pendek berakhir lampau", domain.PermintaanFilter{Dari: "2026-10-10", Sampai: "2026-10-12"}, "2026-09-23"},
		{"periode 30 hari: dari - 6 hari jendela", domain.PermintaanFilter{Dari: "2026-09-18"}, "2026-09-12"},
		{"periode terpanjang", domain.PermintaanFilter{Dari: "2026-07-20"}, "2026-07-14"},
	}
	for _, c := range cases {
		t.Run(c.nama, func(t *testing.T) {
			repo := &fakeRepo{}
			_, err := newUC(repo).GetKpiOperasional(t.Context(), semuaAceh, c.req)
			require.NoError(t, err)
			assert.Equal(t, tgl(c.wantAwal), repo.qKpi.Awal)
		})
	}
}

func TestGetKpiOperasional_GalatRepositoryDibungkus(t *testing.T) {
	boom := errors.New("db mati")

	_, err := newUC(&fakeRepo{deretKpi: func(domain.DeretKpiQuery) ([]domain.TitikKpi, error) { return nil, boom }}).
		GetKpiOperasional(t.Context(), semuaAceh, domain.PermintaanFilter{})
	require.ErrorIs(t, err, boom)
	assert.NotErrorIs(t, err, ErrValidasi, "galat basis data bukan galat validasi (500, bukan 400)")

	_, err = newUC(&fakeRepo{targetErr: boom}).GetKpiOperasional(t.Context(), semuaAceh, domain.PermintaanFilter{})
	require.ErrorIs(t, err, boom)
}

func TestGetKpiOperasional_ParameterTidakValidTidakMenyentuhBasisData(t *testing.T) {
	for _, req := range []domain.PermintaanFilter{
		{KabupatenKode: "abc"}, {KomoditasKode: "cabai"}, {Dari: "kemarin"}, {Sampai: "2026-10-18"},
		{Dari: "2026-10-10", Sampai: "2026-10-01"}, {Dari: "2025-01-01"},
	} {
		repo := &fakeRepo{}
		uc := newUC(repo)

		_, err := uc.GetKpiOperasional(t.Context(), semuaAceh, req)
		require.ErrorIs(t, err, ErrValidasi, "%+v", req)
		_, err = uc.GetHeatmap(t.Context(), semuaAceh, req)
		require.ErrorIs(t, err, ErrValidasi, "%+v", req)

		assert.Zero(t, repo.totalPanggilanData(), "%+v", req)
		assert.Zero(t, uc.cache.Ukuran(), "permintaan tidak valid tidak membuat entri cache")
	}
}

func TestGetKpiOperasional_CakupanTertutupDanIrisanKosongMenghasilkanKartuNolTanpaBasisData(t *testing.T) {
	cases := []struct {
		nama    string
		cakupan domain.Cakupan
		req     domain.PermintaanFilter
	}{
		{"cakupan tertutup", tertutup, domain.PermintaanFilter{}},
		{"cakupan tertutup dengan filter kabupaten", tertutup, domain.PermintaanFilter{KabupatenKode: "1171"}},
		{"kabupaten di luar cakupan", banda, domain.PermintaanFilter{KabupatenKode: "1108"}},
	}
	for _, c := range cases {
		t.Run(c.nama, func(t *testing.T) {
			repo := &fakeRepo{}
			uc := newUC(repo)

			kpis, err := uc.GetKpiOperasional(t.Context(), c.cakupan, c.req)

			require.NoError(t, err)
			require.Len(t, kpis, 4)
			for _, k := range kpis {
				assert.Zero(t, k.Nilai, k.ID)
				assert.Equal(t, "data", k.Tone, k.ID)
				assert.Len(t, k.Tren, 14, k.ID)
			}
			assert.Zero(t, repo.totalPanggilanData())
			assert.Zero(t, uc.cache.Ukuran())
		})
	}
}

func TestGetKpiOperasional_CakupanDanFilterDiteruskanKeRepository(t *testing.T) {
	t.Run("cakupan kabupaten tanpa filter", func(t *testing.T) {
		repo := &fakeRepo{}
		_, err := newUC(repo).GetKpiOperasional(t.Context(), banda, domain.PermintaanFilter{})
		require.NoError(t, err)
		assert.Equal(t, banda, repo.qKpi.Cakupan, "repository menerima cakupan dan menjadikannya FALSE bila tertutup")
		assert.Empty(t, repo.qKpi.KabupatenKode)
	})
	t.Run("cakupan kabupaten dengan filter kabupaten yang sama", func(t *testing.T) {
		repo := &fakeRepo{}
		_, err := newUC(repo).GetKpiOperasional(t.Context(), banda, domain.PermintaanFilter{KabupatenKode: "1171", KomoditasKode: "TMT"})
		require.NoError(t, err)
		assert.Equal(t, banda, repo.qKpi.Cakupan)
		assert.Equal(t, "1171", repo.qKpi.KabupatenKode)
		assert.Equal(t, "TMT", repo.qKpi.KomoditasKode)
	})
	t.Run("seluruh Aceh dengan filter", func(t *testing.T) {
		repo := &fakeRepo{}
		_, err := newUC(repo).GetKpiOperasional(t.Context(), semuaAceh, domain.PermintaanFilter{KabupatenKode: "1108", KomoditasKode: "CBM-KRT", Dari: "2026-10-01", Sampai: "2026-10-10"})
		require.NoError(t, err)
		assert.Equal(t, semuaAceh, repo.qKpi.Cakupan)
		assert.Equal(t, "1108", repo.qKpi.KabupatenKode)
		assert.Equal(t, "CBM-KRT", repo.qKpi.KomoditasKode)
		assert.Equal(t, tgl("2026-10-10"), repo.qKpi.Akhir)
	})
}

func TestGetKpiOperasional_NadaHanyaUntukTingkatAceh(t *testing.T) {
	// Fakta yang membuat validasi 100% (jauh di atas target 85) dan waspada 0 (di bawah target 3): bila target
	// dipakai, kedua kartu "normal".
	fakta := func(domain.DeretKpiQuery) ([]domain.TitikKpi, error) {
		var titik []domain.TitikKpi
		for d := range 20 {
			hari := hariUji.AddDate(0, 0, -d)
			titik = append(titik, domain.TitikKpi{Indikator: domain.IndikatorValidasi, Tanggal: hari, Pembilang: 10, Penyebut: 10})
		}
		return titik, nil
	}
	targets := []domain.TargetKpi{
		{Indikator: "validasi", Target: 85, Arah: domain.ArahNaik},
		{Indikator: "waspada", Target: 3, Arah: domain.ArahTurun},
		{Indikator: "cs", Target: 75, Arah: domain.ArahNaik},
	}

	t.Run("seluruh Aceh tanpa filter memakai target", func(t *testing.T) {
		repo := &fakeRepo{deretKpi: fakta, targets: targets}
		kpis, err := newUC(repo).GetKpiOperasional(t.Context(), semuaAceh, domain.PermintaanFilter{})
		require.NoError(t, err)
		assert.Equal(t, 1, repo.panggilan("TargetKpi"))
		assert.Equal(t, "normal", kartu(kpis, "validasi").Tone)
		assert.Equal(t, "normal", kartu(kpis, "waspada").Tone)
		assert.Equal(t, "defisit", kartu(kpis, "cs").Tone, "cs 0% terhadap target 75%")
		assert.Equal(t, "data", kartu(kpis, "entri").Tone, "entri tidak punya target")
	})

	netral := []struct {
		nama    string
		cakupan domain.Cakupan
		req     domain.PermintaanFilter
	}{
		{"filter kabupaten", semuaAceh, domain.PermintaanFilter{KabupatenKode: "1171"}},
		{"filter komoditas", semuaAceh, domain.PermintaanFilter{KomoditasKode: "CBM-KRT"}},
		{"cakupan kabupaten (Koordinator)", banda, domain.PermintaanFilter{}},
		{"cakupan kabupaten dengan filter komoditas", banda, domain.PermintaanFilter{KomoditasKode: "TMT"}},
	}
	for _, c := range netral {
		t.Run(c.nama+": nada netral, target tidak dipakai", func(t *testing.T) {
			repo := &fakeRepo{deretKpi: fakta, targets: targets}
			kpis, err := newUC(repo).GetKpiOperasional(t.Context(), c.cakupan, c.req)
			require.NoError(t, err)
			assert.Zero(t, repo.panggilan("TargetKpi"), "target provinsi tidak dibaca untuk tampilan yang bukan provinsi")
			for _, k := range kpis {
				assert.Equal(t, "data", k.Tone, k.ID)
			}
			assert.Equal(t, 100.0, kartu(kpis, "validasi").Nilai, "nilainya tetap dihitung")
		})
	}
}

func TestGetKpiOperasional_LabelEntriUntukTanggalLampau(t *testing.T) {
	kpis, err := newUC(&fakeRepo{}).GetKpiOperasional(t.Context(), semuaAceh, domain.PermintaanFilter{Sampai: "2026-10-10"})
	require.NoError(t, err)
	assert.Equal(t, "Entri harga 10 Okt 2026", kartu(kpis, "entri").Label)

	kpis, err = newUC(&fakeRepo{}).GetKpiOperasional(t.Context(), semuaAceh, domain.PermintaanFilter{})
	require.NoError(t, err)
	assert.Equal(t, "Entri harga hari ini", kartu(kpis, "entri").Label)
}

func TestGetHeatmap_MeneruskanCakupanFilterDanTanggalAcuan(t *testing.T) {
	t.Run("tanpa periode: jalur bawaan dari view, acuan hari ini", func(t *testing.T) {
		repo := &fakeRepo{}
		_, err := newUC(repo).GetHeatmap(t.Context(), banda, domain.PermintaanFilter{KomoditasKode: "CBM-KRT"})
		require.NoError(t, err)
		assert.Equal(t, domain.HeatmapQuery{
			Cakupan: banda, KomoditasKode: "CBM-KRT", Acuan: hariUji, Langsung: false, Dari: hariUji.AddDate(0, 0, -13),
		}, repo.qHeatmap)
	})
	t.Run("sampai: dihitung langsung untuk tanggal itu", func(t *testing.T) {
		repo := &fakeRepo{}
		_, err := newUC(repo).GetHeatmap(t.Context(), semuaAceh, domain.PermintaanFilter{KabupatenKode: "1108", Sampai: "2026-10-05"})
		require.NoError(t, err)
		assert.True(t, repo.qHeatmap.Langsung)
		assert.Equal(t, tgl("2026-10-05"), repo.qHeatmap.Acuan)
		assert.Equal(t, "1108", repo.qHeatmap.KabupatenKode)
	})
	t.Run("dari menjadi batas awal tanggal harga terakhir", func(t *testing.T) {
		repo := &fakeRepo{}
		_, err := newUC(repo).GetHeatmap(t.Context(), semuaAceh, domain.PermintaanFilter{Dari: "2026-10-17"})
		require.NoError(t, err)
		assert.True(t, repo.qHeatmap.Langsung)
		assert.Equal(t, hariUji, repo.qHeatmap.Dari)
		assert.Equal(t, hariUji, repo.qHeatmap.Acuan)
	})
}

func TestGetHeatmap_CakupanTertutupDanIrisanKosongHeatmapKosongTanpaBasisData(t *testing.T) {
	for _, c := range []struct {
		cakupan domain.Cakupan
		req     domain.PermintaanFilter
	}{
		{tertutup, domain.PermintaanFilter{}},
		{banda, domain.PermintaanFilter{KabupatenKode: "1108"}},
	} {
		repo := &fakeRepo{}
		data, err := newUC(repo).GetHeatmap(t.Context(), c.cakupan, c.req)
		require.NoError(t, err)
		assert.Empty(t, data.Sel)
		assert.Empty(t, data.Kabupaten)
		assert.Empty(t, data.Komoditas)
		assert.Zero(t, repo.totalPanggilanData())
	}
}

func TestGetHeatmap(t *testing.T) {
	repo := &fakeRepo{heatmap: func(domain.HeatmapQuery) (*domain.HeatmapData, error) {
		return &domain.HeatmapData{Sel: []domain.SelHeatmap{{KomoditasKode: "BWM"}}}, nil
	}}
	data, err := newUC(repo).GetHeatmap(t.Context(), semuaAceh, domain.PermintaanFilter{})
	require.NoError(t, err)
	assert.Len(t, data.Sel, 1)

	_, err = newUC(&fakeRepo{heatmap: func(domain.HeatmapQuery) (*domain.HeatmapData, error) { return nil, errors.New("db mati") }}).
		GetHeatmap(t.Context(), semuaAceh, domain.PermintaanFilter{})
	assert.Error(t, err)
}

func TestGetAktivitasLapangan_UsesTodayInAcehTime(t *testing.T) {
	repo := &fakeRepo{aktivitas: func(awal, akhir time.Time, c domain.Cakupan) ([]domain.AktivitasKabupaten, error) {
		return []domain.AktivitasKabupaten{{KabupatenKode: "1171", Entri: 10, Target: 14}}, nil
	}}
	uc := newUC(repo)
	// 16 Oktober 19.00 UTC = 17 Oktober 02.00 WIB: "hari ini" adalah 17 Oktober WIB.
	uc.now = func() time.Time { return time.Date(2026, 10, 16, 19, 0, 0, 0, time.UTC) }

	items, err := uc.GetAktivitasLapangan(t.Context(), banda)

	require.NoError(t, err)
	require.Len(t, items, 1)
	awal := time.Date(2026, 10, 17, 0, 0, 0, 0, wib)
	assert.True(t, awal.Equal(repo.qAwal), "awal hari WIB: %s", repo.qAwal)
	assert.True(t, awal.AddDate(0, 0, 1).Equal(repo.qAkhir))
	assert.Equal(t, banda, repo.qCakupan)
}

func TestGetPembandingHarga_UrutSelisihTerbesarDanHitungSemuaPasangan(t *testing.T) {
	pasangan := make([]domain.PembandingHarga, 0, 25)
	for i := range 25 {
		pasangan = append(pasangan, domain.PembandingHarga{KomoditasKode: "K" + string(rune('A'+i)), KabupatenKode: "1171", SelisihPct: float64(i) - 12})
	}
	repo := &fakeRepo{pembanding: func(time.Time, domain.Cakupan) ([]domain.PembandingHarga, error) { return pasangan, nil }}
	uc := newUC(repo)
	uc.now = func() time.Time { return time.Date(2026, 10, 16, 23, 0, 0, 0, time.UTC) } // 17 Oktober 06.00 WIB

	res, err := uc.GetPembandingHarga(context.Background(), semuaAceh)

	require.NoError(t, err)
	assert.Equal(t, 25, res.Pasangan)
	assert.Equal(t, 21, res.Sepakat, "selisih -10..10 persen dihitung sepakat")
	require.Len(t, res.Items, 20)
	assert.Equal(t, -12.0, res.Items[0].SelisihPct, "selisih absolut terbesar dulu; seri diurutkan menurut kode komoditas")
	assert.Equal(t, 12.0, res.Items[1].SelisihPct)
	assert.Equal(t, time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC), repo.qSejak, "jendela 7 hari dimulai 10 Oktober")
	assert.Equal(t, semuaAceh, repo.qCakupan)
}

func TestGetPembandingHarga_KosongBukanGalat(t *testing.T) {
	res, err := newUC(&fakeRepo{}).GetPembandingHarga(context.Background(), semuaAceh)

	require.NoError(t, err)
	assert.Zero(t, res.Pasangan)
	assert.Empty(t, res.Items)
}

func TestGetPembandingHarga_GalatRepositoryDibungkus(t *testing.T) {
	repo := &fakeRepo{pembanding: func(time.Time, domain.Cakupan) ([]domain.PembandingHarga, error) { return nil, errors.New("db down") }}

	_, err := newUC(repo).GetPembandingHarga(context.Background(), semuaAceh)

	assert.Error(t, err)
}

func TestGetCuaca_AnomaliDanCurahTerbesarDulu(t *testing.T) {
	repo := &fakeRepo{cuaca: func(time.Time, time.Time, domain.Cakupan) ([]domain.CuacaKabupaten, error) {
		return []domain.CuacaKabupaten{
			{KabupatenKode: "1101", CurahHujanMm: 5},
			{KabupatenKode: "1102", CurahHujanMm: 20},
			{KabupatenKode: "1103", CurahHujanMm: 55, Anomali: true},
			{KabupatenKode: "1104", CurahHujanMm: 70, Anomali: true},
		}, nil
	}}
	uc := newUC(repo)
	uc.now = func() time.Time { return time.Date(2026, 10, 16, 23, 0, 0, 0, time.UTC) } // 17 Oktober WIB

	items, err := uc.GetCuaca(context.Background(), semuaAceh)

	require.NoError(t, err)
	kode := make([]string, len(items))
	for i, it := range items {
		kode[i] = it.KabupatenKode
	}
	assert.Equal(t, []string{"1104", "1103", "1102", "1101"}, kode)
	assert.Equal(t, hariUji.AddDate(0, 0, -3), repo.qSejak)
	assert.Equal(t, hariUji, repo.qSampai)
}

// Semua endpoint agregat memperlakukan cakupan tertutup sama: hasil kosong (bukan seluruh Aceh, bukan galat) tanpa
// menyentuh basis data.
func TestSemuaEndpoint_CakupanTertutupHasilKosongTanpaBasisData(t *testing.T) {
	repo := &fakeRepo{}
	uc := newUC(repo)

	akt, err := uc.GetAktivitasLapangan(t.Context(), tertutup)
	require.NoError(t, err)
	assert.Empty(t, akt)
	assert.NotNil(t, akt, "JSON [] bukan null")

	pb, err := uc.GetPembandingHarga(t.Context(), tertutup)
	require.NoError(t, err)
	assert.Zero(t, pb.Pasangan)
	assert.Empty(t, pb.Items)

	cu, err := uc.GetCuaca(t.Context(), tertutup)
	require.NoError(t, err)
	assert.Empty(t, cu)
	assert.NotNil(t, cu)

	assert.Zero(t, repo.totalPanggilanData())
}

func TestCakupan_DariPeranDanKabupatenDiBasisData(t *testing.T) {
	cases := map[string]struct {
		pengguna domain.PenggunaCakupan
		want     domain.Cakupan
	}{
		"koordinator":              {domain.PenggunaCakupan{Peran: "OPS_KOORDINATOR", KabupatenKode: "1171"}, banda},
		"agregator":                {domain.PenggunaCakupan{Peran: "OPS_AGREGATOR", KabupatenKode: "1108"}, domain.CakupanKabupaten("1108")},
		"koordinator tanpa kab":    {domain.PenggunaCakupan{Peran: "OPS_KOORDINATOR"}, tertutup},
		"mahasiswa":                {domain.PenggunaCakupan{Peran: "OPS_MAHASISWA", KabupatenKode: "1171"}, tertutup},
		"manajer":                  {domain.PenggunaCakupan{Peran: "MANAJER_DATA"}, semuaAceh},
		"strategis":                {domain.PenggunaCakupan{Peran: "STRATEGIS_BI"}, semuaAceh},
		"admin":                    {domain.PenggunaCakupan{Peran: "ADMIN_SISTEM"}, semuaAceh},
		"peran asing tidak semua":  {domain.PenggunaCakupan{Peran: "PERAN_BARU"}, tertutup},
		"peran kosong tidak semua": {domain.PenggunaCakupan{}, tertutup},
	}
	for nama, c := range cases {
		t.Run(nama, func(t *testing.T) {
			repo := &fakeRepo{pengguna: c.pengguna}
			got, err := newUC(repo).Cakupan(t.Context(), "uuid-1")
			require.NoError(t, err)
			assert.Equal(t, c.want, got)
			assert.Equal(t, "uuid-1", repo.qPengguna)
		})
	}
}

func TestCakupan_AkunTidakAdaTertutup(t *testing.T) {
	got, err := newUC(&fakeRepo{penggunaErr: domain.ErrPenggunaTidakAda}).Cakupan(t.Context(), "uuid-hilang")

	require.NoError(t, err, "akun yang sudah dihapus bukan galat server")
	assert.True(t, got.Tertutup())
}

func TestCakupan_GalatBasisDataGagalTutup(t *testing.T) {
	// Galat dikembalikan sebagai galat; tidak pernah dijawab dengan cakupan yang lebih luas.
	got, err := newUC(&fakeRepo{pengguna: domain.PenggunaCakupan{Peran: "MANAJER_DATA"}, penggunaErr: errors.New("koneksi putus")}).
		Cakupan(t.Context(), "uuid-1")

	require.Error(t, err)
	assert.False(t, got.Semua, "galat tidak boleh menghasilkan seluruh Aceh")
	assert.True(t, got.Tertutup())
}

// Cakupan dibaca ulang dari basis data pada setiap permintaan, bukan di-cache: kabupaten yang diubah admin berlaku
// seketika, bukan setelah TTL atau setelah token lama habis.
func TestCakupan_DibacaUlangTiapPermintaan(t *testing.T) {
	repo := &fakeRepo{pengguna: domain.PenggunaCakupan{Peran: "OPS_KOORDINATOR", KabupatenKode: "1171"}}
	uc := newUC(repo)

	a, _ := uc.Cakupan(t.Context(), "uuid-1")
	repo.mu.Lock()
	repo.pengguna.KabupatenKode = "1108" // admin memindahkan Koordinator
	repo.mu.Unlock()
	b, _ := uc.Cakupan(t.Context(), "uuid-1")

	assert.Equal(t, domain.CakupanKabupaten("1171"), a)
	assert.Equal(t, domain.CakupanKabupaten("1108"), b)
	assert.Equal(t, 2, repo.panggilan("PenggunaCakupan"))
}
