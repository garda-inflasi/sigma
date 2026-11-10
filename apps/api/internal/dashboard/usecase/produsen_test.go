package usecase

import (
	"errors"
	"math"
	"testing"

	"github.com/garda-inflasi/api/internal/dashboard/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func f64(v float64) *float64 { return &v }
func str(v string) *string   { return &v }

func faktaHarga(kode, nama string, pilot bool, produsen, konsumen *float64) domain.ProdusenFakta {
	f := domain.ProdusenFakta{KomoditasKode: kode, KomoditasNama: nama, Satuan: "kg", Pilot: pilot, HargaProdusen: produsen, HargaKonsumen: konsumen}
	if produsen != nil {
		f.EntriProdusen, f.KabProdusen = 50, 10
	}
	if konsumen != nil {
		f.EntriKonsumen, f.KabKonsumen = 47, 10
	}
	return f
}

func cariProdusen(p *domain.Produsen, kode string) domain.ProdusenKomoditas {
	for _, k := range p.Komoditas {
		if k.KomoditasKode == kode {
			return k
		}
	}
	return domain.ProdusenKomoditas{}
}

// Hitungan tangan CBM-KRT: produsen 41.682,4 -> 41.682; konsumen 57.470,51 -> 57.471;
// selisih 57.471 - 41.682 = 15.789; terhadap produsen 15.789 / 41.682 = 37,88% -> 37,9;
// terhadap konsumen 15.789 / 57.471 = 27,47% -> 27,5. Margin dihitung dari harga yang sudah dibulatkan.
func TestRakitProdusen_MarginHitunganTangan(t *testing.T) {
	periode := tgl("2026-10-01")
	f := faktaHarga("CBM-KRT", "Cabai merah keriting", true, f64(41682.4), f64(57470.51))
	f.ProduksiTon, f.ProduksiPeriode, f.ProduksiSumber = f64(3100.04), &periode, str("DEMO")

	p := RakitProdusen(tgl("2026-10-11"), hariUji, []domain.ProdusenFakta{f})

	assert.Equal(t, tgl("2026-10-11"), p.Dari)
	assert.Equal(t, hariUji, p.Sampai)
	require.Len(t, p.Komoditas, 1)
	k := p.Komoditas[0]
	assert.Equal(t, "kg", k.Satuan)
	assert.Equal(t, &domain.SisiHarga{Harga: 41682, JumlahEntri: 50, JumlahKabupaten: 10}, k.Produsen)
	assert.Equal(t, &domain.SisiHarga{Harga: 57471, JumlahEntri: 47, JumlahKabupaten: 10}, k.Konsumen)
	assert.Equal(t, &domain.Margin{Selisih: 15789, PersenProdusen: 37.9, PersenKonsumen: 27.5}, k.Margin)
	assert.Equal(t, k.Konsumen.Harga-k.Produsen.Harga, k.Margin.Selisih, "angka yang tampil saling menjumlah")
	require.NotNil(t, k.Produksi)
	assert.Equal(t, 3100.0, k.Produksi.Ton)
	assert.Equal(t, periode, k.Produksi.Periode)
	assert.True(t, k.Produksi.Demo())
}

func TestRakitProdusen_MarginHanyaBilaKeduaSisiAda(t *testing.T) {
	p := RakitProdusen(tgl("2026-10-11"), hariUji, []domain.ProdusenFakta{
		faktaHarga("BWM", "Bawang merah", true, f64(30000), nil),
		faktaHarga("TMT", "Tomat", false, nil, f64(11000)),
		faktaHarga("KTG", "Kentang", false, nil, nil),
	})

	bwm := cariProdusen(p, "BWM")
	require.NotNil(t, bwm.Produsen)
	assert.Nil(t, bwm.Konsumen, "tanpa harga konsumen pembanding")
	assert.Nil(t, bwm.Margin, "margin tidak dikarang dari satu sisi")

	tmt := cariProdusen(p, "TMT")
	assert.Nil(t, tmt.Produsen)
	assert.Nil(t, tmt.Margin)

	ktg := cariProdusen(p, "KTG")
	assert.Nil(t, ktg.Produsen)
	assert.Nil(t, ktg.Konsumen)
	assert.Nil(t, ktg.Margin)
	assert.Nil(t, ktg.Produksi, "tanpa neraca: tidak ada produksi (bukan nol)")
}

func TestRakitProdusen_MarginNegatifDipertahankan(t *testing.T) {
	// Harga konsumen di bawah harga produsen (data menyimpang atau komoditas dijual rugi): angkanya apa adanya.
	p := RakitProdusen(tgl("2026-10-11"), hariUji, []domain.ProdusenFakta{faktaHarga("TMT", "Tomat", false, f64(12000), f64(10000))})

	m := p.Komoditas[0].Margin
	require.NotNil(t, m)
	assert.Equal(t, &domain.Margin{Selisih: -2000, PersenProdusen: -16.7, PersenKonsumen: -20}, m)
}

func TestRakitProdusen_NolTidakPernahNegatif(t *testing.T) {
	cases := map[string]struct{ produsen, konsumen float64 }{
		"selisih persis nol":                         {10000, 10000},
		"selisih -1 rupiah: persen -0,01 jadi nol":   {10000, 9999},
		"selisih -1 rupiah pada harga kecil: -0,04%": {2500, 2499},
		"selisih +1 rupiah: persen 0,01 jadi 0,0":    {10000, 10001},
	}
	for nama, c := range cases {
		t.Run(nama, func(t *testing.T) {
			p := RakitProdusen(tgl("2026-10-11"), hariUji, []domain.ProdusenFakta{faktaHarga("TMT", "Tomat", false, f64(c.produsen), f64(c.konsumen))})

			m := p.Komoditas[0].Margin
			require.NotNil(t, m)
			// Nol negatif ditulis JSON sebagai "-0".
			assert.False(t, m.PersenProdusen == 0 && math.Signbit(m.PersenProdusen), "persen produsen %v", m.PersenProdusen)
			assert.False(t, m.PersenKonsumen == 0 && math.Signbit(m.PersenKonsumen), "persen konsumen %v", m.PersenKonsumen)
			assert.False(t, m.Selisih == 0 && math.Signbit(m.Selisih))
		})
	}
}

func TestRakitProdusen_HargaNolTidakMembagiDenganNol(t *testing.T) {
	// Harga entri selalu positif (CHECK harga > 0), tetapi pembagi nol tidak boleh menghasilkan Inf atau NaN bila data menyimpang.
	p := RakitProdusen(tgl("2026-10-11"), hariUji, []domain.ProdusenFakta{
		faktaHarga("TMT", "Tomat", false, f64(0.3), f64(11000)), // dibulatkan 0
		faktaHarga("BWM", "Bawang merah", false, f64(10000), f64(0.2)),
	})

	for _, k := range p.Komoditas {
		assert.Nil(t, k.Margin, k.KomoditasKode)
	}
}

func TestRakitProdusen_ProduksiTanpaHarga(t *testing.T) {
	periode := tgl("2026-10-01")
	f := faktaHarga("BRS-MED", "Beras medium", true, nil, nil)
	f.ProduksiTon, f.ProduksiPeriode, f.ProduksiSumber = f64(146200), &periode, str("BPS")

	k := RakitProdusen(tgl("2026-10-11"), hariUji, []domain.ProdusenFakta{f}).Komoditas[0]

	assert.Nil(t, k.Produsen)
	assert.Nil(t, k.Margin)
	require.NotNil(t, k.Produksi)
	assert.False(t, k.Produksi.Demo(), "sumber resmi")
}

func TestRakitProdusen_UrutPilotLaluNamaLaluKode(t *testing.T) {
	p := RakitProdusen(tgl("2026-10-11"), hariUji, []domain.ProdusenFakta{
		faktaHarga("TMT", "Tomat", false, f64(1), nil),
		faktaHarga("CBM-KRT", "cabai merah", true, f64(1), nil),
		faktaHarga("BWM", "Bawang merah", true, f64(1), nil),
		faktaHarga("KTG", "Kentang", false, f64(1), nil),
		faktaHarga("KTG-2", "Kentang", false, f64(1), nil),
	})

	kode := make([]string, len(p.Komoditas))
	for i, k := range p.Komoditas {
		kode[i] = k.KomoditasKode
	}
	assert.Equal(t, []string{"BWM", "CBM-KRT", "KTG", "KTG-2", "TMT"}, kode)
}

func TestRakitProdusen_KosongBukanNil(t *testing.T) {
	p := RakitProdusen(tgl("2026-10-11"), hariUji, nil)

	assert.NotNil(t, p.Komoditas, "JSON [] bukan null")
	assert.Empty(t, p.Komoditas)
	assert.Equal(t, tgl("2026-10-11"), p.Dari)
	assert.Equal(t, hariUji, p.Sampai)
}

func TestGetProdusen_JendelaTujuhHariTermasukHariIni(t *testing.T) {
	repo := &fakeRepo{}

	p, err := newUC(repo).GetProdusen(t.Context(), semuaAceh, domain.PermintaanSaringan{KabupatenKode: "1108", KomoditasKode: "CBM-KRT"})

	require.NoError(t, err)
	q := repo.qProdusen
	assert.Equal(t, tgl("2026-10-11"), q.Dari, "17 Okt dan enam hari sebelumnya")
	assert.Equal(t, hariUji, q.Sampai)
	assert.Equal(t, domain.HariAntara(q.Dari, q.Sampai), domain.JendelaOperasionalHari)
	assert.Equal(t, semuaAceh, q.Cakupan)
	assert.Equal(t, "1108", q.KabupatenKode)
	assert.Equal(t, "CBM-KRT", q.KomoditasKode)
	assert.Equal(t, q.Dari, p.Dari)
	assert.Equal(t, q.Sampai, p.Sampai)
}

func TestGetProdusen_CakupanTertutupDanIrisanKosongTanpaBasisData(t *testing.T) {
	cases := []struct {
		nama    string
		cakupan domain.Cakupan
		req     domain.PermintaanSaringan
	}{
		{"cakupan tertutup", tertutup, domain.PermintaanSaringan{}},
		{"kabupaten di luar cakupan", banda, domain.PermintaanSaringan{KabupatenKode: "1108"}},
	}
	for _, c := range cases {
		t.Run(c.nama, func(t *testing.T) {
			repo := &fakeRepo{produsen: func(domain.ProdusenQuery) ([]domain.ProdusenFakta, error) {
				return []domain.ProdusenFakta{faktaHarga("TMT", "Tomat", false, f64(1), f64(2))}, nil
			}}
			uc := newUC(repo)

			p, err := uc.GetProdusen(t.Context(), c.cakupan, c.req)

			require.NoError(t, err)
			assert.NotNil(t, p.Komoditas)
			assert.Empty(t, p.Komoditas, "harga cakupan lain tidak boleh tampil")
			assert.Equal(t, tgl("2026-10-11"), p.Dari, "jendela tetap diisi")
			assert.Zero(t, repo.totalPanggilanData())
			assert.Zero(t, uc.cache.Ukuran())
		})
	}
}

func TestGetProdusen_ParameterTidakValidDanGalatRepository(t *testing.T) {
	repo := &fakeRepo{}
	_, err := newUC(repo).GetProdusen(t.Context(), semuaAceh, domain.PermintaanSaringan{KomoditasKode: "cabai"})
	require.ErrorIs(t, err, ErrValidasi)
	assert.Zero(t, repo.totalPanggilanData())

	boom := errors.New("db mati")
	repo = &fakeRepo{produsen: func(domain.ProdusenQuery) ([]domain.ProdusenFakta, error) { return nil, boom }}
	uc := newUC(repo)
	_, err = uc.GetProdusen(t.Context(), semuaAceh, domain.PermintaanSaringan{})
	require.ErrorIs(t, err, boom)
	assert.NotErrorIs(t, err, ErrValidasi)
	assert.Zero(t, uc.cache.Ukuran())
}
