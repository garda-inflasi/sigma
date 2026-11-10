package usecase

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/dashboard/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func barisStok(kom, nama, fas, kelas string, lot int, ton, dekat, risiko float64) domain.BarisStok {
	return domain.BarisStok{
		KomoditasKode: kom, KomoditasNama: nama, FasilitasID: fas, Kelas: kelas,
		JumlahLot: lot, VolumeTon: ton, MendekatiTon: dekat, RisikoTinggiTon: risiko,
	}
}

func komoditasStok(s *domain.Stok, kode string) (domain.StokKomoditas, bool) {
	for _, k := range s.Komoditas {
		if k.KomoditasKode == kode {
			return k, true
		}
	}
	return domain.StokKomoditas{}, false
}

// Data demo (CS-01..CS-04) dengan derau pecahan biner seperti hasil SUM double precision di basis data.
//
// Hitungan tangan, hanya kelas BERLAKU:
//
//	BWM      162,1 t (3 lot, F1)         CBM-KRT 136,9 + 14,4 = 151,3 t (5 lot, F1 dan F3)
//	IKN-TGK  240,0 t (5 lot, F2)         TMT      79,5 t (3 lot, F3)
//	total    162,1 + 151,3 + 240 + 79,5 = 632,9 t; 16 lot; 3 fasilitas berbeda (bukan 1+2+1+1 = 5)
//	mendekati kedaluwarsa 34,9 + 79,5 = 114,4 t; risiko tinggi 34,9 + 106,8 + 31 = 172,7 t
//	utilisasi 632,9 / 950 = 66,6%
func faktaStokDemo() domain.FaktaStok {
	return domain.FaktaStok{
		Baris: []domain.BarisStok{
			barisStok("BWM", "Bawang merah", "F1", domain.KelasStokBerlaku, 3, 162.10000000000002, 0, 0),
			barisStok("CBM-KRT", "Cabai merah keriting", "F1", domain.KelasStokBerlaku, 4, 136.9, 34.9, 34.9),
			barisStok("CBM-KRT", "Cabai merah keriting", "F3", domain.KelasStokBerlaku, 1, 14.4, 0, 0),
			barisStok("IKN-TGK", "Ikan tongkol", "F2", domain.KelasStokBerlaku, 5, 239.99999999999997, 0, 106.8),
			barisStok("TMT", "Tomat", "F3", domain.KelasStokBerlaku, 3, 79.5, 79.5, 31),
			// Bukan stok berlaku: fasilitas perawatan, lot lewat tanggal kedaluwarsa, dan lot yang belum masuk.
			barisStok("BWM", "Bawang merah", "F4", domain.KelasStokLuarOperasional, 1, 20.1, 0, 0),
			barisStok("TLR", "Telur ayam ras", "F4", domain.KelasStokLuarOperasional, 1, 12.9, 0, 0),
			barisStok("CBR-RWT", "Cabai rawit merah", "F1", domain.KelasStokKedaluwarsa, 2, 7.25, 7.25, 7.25),
			barisStok("CBR-RWT", "Cabai rawit merah", "F1", domain.KelasStokBelumMasuk, 9, 900, 900, 900),
		},
		Kapasitas: domain.KapasitasStok{Fasilitas: 3, Ton: 950},
	}
}

func TestRakitStok_HitunganTangan(t *testing.T) {
	s := RakitStok(hariUji, faktaStokDemo())

	r := s.Ringkasan
	assert.Equal(t, hariUji, s.Tanggal)
	assert.Equal(t, 632.9, r.TotalTon)
	assert.Equal(t, 16, r.JumlahLot)
	assert.Equal(t, 3, r.JumlahFasilitas, "fasilitas berbeda, bukan jumlah fasilitas per komoditas")
	assert.EqualValues(t, 950, r.KapasitasTon)
	assert.Equal(t, 3, r.FasilitasOperasional)
	assert.Equal(t, 66.6, r.UtilisasiPct)
	assert.Equal(t, 114.4, r.MendekatiKedaluwarsaTon)
	assert.Equal(t, 172.7, r.RisikoTinggiTon)
	assert.Equal(t, 2, r.KedaluwarsaLot)
	assert.Equal(t, 7.3, r.KedaluwarsaTon, "7,25 dibulatkan ke atas pada setengah")
	assert.Equal(t, 33.0, r.DiLuarOperasionalTon)

	// Terbesar lebih dulu, termasuk 239,99999999999997 yang dibulatkan 240,0.
	kode := make([]string, len(s.Komoditas))
	for i, k := range s.Komoditas {
		kode[i] = k.KomoditasKode
	}
	assert.Equal(t, []string{"IKN-TGK", "BWM", "CBM-KRT", "TMT"}, kode)

	ikan, _ := komoditasStok(s, "IKN-TGK")
	assert.Equal(t, domain.StokKomoditas{KomoditasKode: "IKN-TGK", KomoditasNama: "Ikan tongkol", TotalTon: 240, JumlahLot: 5, JumlahFasilitas: 1, RisikoTinggiTon: 106.8}, ikan)
	cabai, _ := komoditasStok(s, "CBM-KRT")
	assert.Equal(t, domain.StokKomoditas{KomoditasKode: "CBM-KRT", KomoditasNama: "Cabai merah keriting", TotalTon: 151.3, JumlahLot: 5, JumlahFasilitas: 2, MendekatiKedaluwarsaTon: 34.9, RisikoTinggiTon: 34.9}, cabai)
	tomat, _ := komoditasStok(s, "TMT")
	assert.Equal(t, domain.StokKomoditas{KomoditasKode: "TMT", KomoditasNama: "Tomat", TotalTon: 79.5, JumlahLot: 3, JumlahFasilitas: 1, MendekatiKedaluwarsaTon: 79.5, RisikoTinggiTon: 31}, tomat)

	// Komoditas yang hanya punya stok di luar kelas berlaku tidak tampil sebagai baris.
	for _, kode := range []string{"TLR", "CBR-RWT"} {
		_, ada := komoditasStok(s, kode)
		assert.False(t, ada, kode)
	}
}

// Kelas selain BERLAKU tidak boleh bocor ke angka stok berlaku: lot yang belum masuk di fakta uji berukuran 900 ton dan
// menandai bila ia ikut terhitung.
func TestRakitStok_KelasLainTidakMasukStokBerlaku(t *testing.T) {
	f := domain.FaktaStok{
		Baris: []domain.BarisStok{
			barisStok("TMT", "Tomat", "F1", domain.KelasStokKedaluwarsa, 1, 50, 50, 50),
			barisStok("TMT", "Tomat", "F2", domain.KelasStokLuarOperasional, 1, 60, 60, 60),
			barisStok("TMT", "Tomat", "F3", domain.KelasStokBelumMasuk, 1, 70, 70, 70),
		},
		Kapasitas: domain.KapasitasStok{Fasilitas: 3, Ton: 1000},
	}

	s := RakitStok(hariUji, f)

	assert.Zero(t, s.Ringkasan.TotalTon)
	assert.Zero(t, s.Ringkasan.JumlahLot)
	assert.Zero(t, s.Ringkasan.JumlahFasilitas)
	assert.Zero(t, s.Ringkasan.UtilisasiPct, "tanpa stok berlaku utilisasinya nol walau ada kapasitas")
	assert.Zero(t, s.Ringkasan.MendekatiKedaluwarsaTon, "mendekati kedaluwarsa hanya bermakna untuk lot berlaku")
	assert.Zero(t, s.Ringkasan.RisikoTinggiTon, "risiko tinggi hanya dihitung untuk lot berlaku")
	assert.Equal(t, 50.0, s.Ringkasan.KedaluwarsaTon)
	assert.Equal(t, 60.0, s.Ringkasan.DiLuarOperasionalTon)
	assert.NotNil(t, s.Komoditas)
	assert.Empty(t, s.Komoditas)
}

func TestRakitStok_Kosong(t *testing.T) {
	s := RakitStok(hariUji, domain.FaktaStok{})

	assert.Equal(t, domain.RingkasanStok{}, s.Ringkasan)
	assert.NotNil(t, s.Komoditas, "JSON [] bukan null")
	assert.Empty(t, s.Komoditas)
}

func TestRakitStok_UtilisasiTanpaKapasitasNolBukanTakHingga(t *testing.T) {
	// Kapasitas nol dengan stok berlaku (mis. filter kabupaten yang tidak punya fasilitas operasional, atau data
	// kapasitas yang menyimpang): tidak ada pembagian dengan nol.
	f := domain.FaktaStok{Baris: []domain.BarisStok{barisStok("TMT", "Tomat", "F1", domain.KelasStokBerlaku, 1, 10, 0, 0)}}

	s := RakitStok(hariUji, f)

	assert.Zero(t, s.Ringkasan.UtilisasiPct)
	assert.False(t, math.IsInf(s.Ringkasan.UtilisasiPct, 0) || math.IsNaN(s.Ringkasan.UtilisasiPct))
	assert.Equal(t, 10.0, s.Ringkasan.TotalTon)
}

func TestRakitStok_UtilisasiDiAtasSeratusTidakDipotong(t *testing.T) {
	// Pengisian berlebih adalah informasi yang sama pentingnya dengan kapasitas yang sepi.
	f := domain.FaktaStok{
		Baris:     []domain.BarisStok{barisStok("TMT", "Tomat", "F1", domain.KelasStokBerlaku, 1, 120, 0, 0)},
		Kapasitas: domain.KapasitasStok{Fasilitas: 1, Ton: 100},
	}

	assert.Equal(t, 120.0, RakitStok(hariUji, f).Ringkasan.UtilisasiPct)
}

// Total dibulatkan dari jumlah tak terbulatkan, bukan dari jumlah angka yang sudah dibulatkan.
func TestRakitStok_TotalDariJumlahTakTerbulatkan(t *testing.T) {
	f := domain.FaktaStok{Baris: []domain.BarisStok{
		barisStok("TMT", "Tomat", "F1", domain.KelasStokBerlaku, 1, 0.04, 0, 0),
		barisStok("BWM", "Bawang merah", "F1", domain.KelasStokBerlaku, 1, 0.04, 0, 0),
		barisStok("KTG", "Kentang", "F1", domain.KelasStokBerlaku, 1, 0.04, 0, 0),
	}}

	s := RakitStok(hariUji, f)

	assert.Equal(t, 0.1, s.Ringkasan.TotalTon, "0,12 dibulatkan 0,1; menjumlah tiga 0,0 memberi 0,0")
	for _, k := range s.Komoditas {
		assert.Zero(t, k.TotalTon)
	}
}

func TestRakitStok_SeriDiputusKode(t *testing.T) {
	f := domain.FaktaStok{Baris: []domain.BarisStok{
		barisStok("TMT", "Tomat", "F1", domain.KelasStokBerlaku, 1, 10, 0, 0),
		barisStok("BWM", "Bawang merah", "F1", domain.KelasStokBerlaku, 1, 10, 0, 0),
		barisStok("KTG", "Kentang", "F1", domain.KelasStokBerlaku, 1, 10, 0, 0),
	}}

	for range 20 {
		s := RakitStok(hariUji, f)
		assert.Equal(t, []string{"BWM", "KTG", "TMT"}, []string{s.Komoditas[0].KomoditasKode, s.Komoditas[1].KomoditasKode, s.Komoditas[2].KomoditasKode})
	}
}

func TestRakitStok_FasilitasBerlakuTidakMenghitungFasilitasLain(t *testing.T) {
	// F9 hanya punya lot kedaluwarsa: ia bukan fasilitas yang menyimpan stok berlaku.
	f := domain.FaktaStok{Baris: []domain.BarisStok{
		barisStok("TMT", "Tomat", "F1", domain.KelasStokBerlaku, 1, 10, 0, 0),
		barisStok("TMT", "Tomat", "F9", domain.KelasStokKedaluwarsa, 1, 5, 0, 0),
	}}

	assert.Equal(t, 1, RakitStok(hariUji, f).Ringkasan.JumlahFasilitas)
}

func TestGetStok_BatasLotBerlakuDiTengahMalamSesudahHariIni(t *testing.T) {
	repo := &fakeRepo{}

	_, err := newUC(repo).GetStok(t.Context(), banda, domain.PermintaanSaringan{KabupatenKode: "1171", KomoditasKode: "TMT"})

	require.NoError(t, err)
	q := repo.qStok
	assert.Equal(t, banda, q.Cakupan)
	assert.Equal(t, "1171", q.KabupatenKode)
	assert.Equal(t, "TMT", q.KomoditasKode)
	// Hari ini 17 Okt (WIB): berlaku bila masuk < 18 Okt 00.00 WIB <= kedaluwarsa; mendekati bila kedaluwarsa < 25 Okt 00.00 WIB.
	assert.True(t, time.Date(2026, 10, 18, 0, 0, 0, 0, wib).Equal(q.Batas), "batas: %s", q.Batas)
	assert.True(t, time.Date(2026, 10, 25, 0, 0, 0, 0, wib).Equal(q.BatasDekat), "batas dekat: %s", q.BatasDekat)
}

func TestGetStok_MerakitDariFakta(t *testing.T) {
	repo := &fakeRepo{stok: func(domain.StokQuery) (domain.FaktaStok, error) { return faktaStokDemo(), nil }}

	s, err := newUC(repo).GetStok(t.Context(), semuaAceh, domain.PermintaanSaringan{})

	require.NoError(t, err)
	assert.Equal(t, 632.9, s.Ringkasan.TotalTon)
	assert.Equal(t, hariUji, s.Tanggal)
}

func TestGetStok_CakupanTertutupDanIrisanKosongTanpaBasisData(t *testing.T) {
	cases := []struct {
		nama    string
		cakupan domain.Cakupan
		req     domain.PermintaanSaringan
	}{
		{"cakupan tertutup", tertutup, domain.PermintaanSaringan{}},
		{"cakupan tertutup dengan filter", tertutup, domain.PermintaanSaringan{KabupatenKode: "1171", KomoditasKode: "TMT"}},
		{"kabupaten di luar cakupan", banda, domain.PermintaanSaringan{KabupatenKode: "1108"}},
	}
	for _, c := range cases {
		t.Run(c.nama, func(t *testing.T) {
			repo := &fakeRepo{stok: func(domain.StokQuery) (domain.FaktaStok, error) { return faktaStokDemo(), nil }}
			uc := newUC(repo)

			s, err := uc.GetStok(t.Context(), c.cakupan, c.req)

			require.NoError(t, err)
			assert.Equal(t, domain.RingkasanStok{}, s.Ringkasan, "angka milik cakupan lain tidak boleh tampil")
			assert.NotNil(t, s.Komoditas)
			assert.Empty(t, s.Komoditas)
			assert.Zero(t, repo.totalPanggilanData())
			assert.Zero(t, uc.cache.Ukuran())
		})
	}
}

func TestGetStok_ParameterTidakValidTidakMenyentuhBasisData(t *testing.T) {
	for _, req := range []domain.PermintaanSaringan{{KabupatenKode: "abc"}, {KomoditasKode: "cabai"}, {KabupatenKode: "1171", KomoditasKode: "x"}} {
		repo := &fakeRepo{}
		uc := newUC(repo)

		_, err := uc.GetStok(t.Context(), semuaAceh, req)

		require.ErrorIs(t, err, ErrValidasi, "%+v", req)
		assert.Zero(t, repo.totalPanggilanData())
		assert.Zero(t, uc.cache.Ukuran())
	}
}

func TestGetStok_GalatRepositoryDibungkusDanBukanValidasi(t *testing.T) {
	boom := errors.New("db mati")
	repo := &fakeRepo{stok: func(domain.StokQuery) (domain.FaktaStok, error) { return domain.FaktaStok{}, boom }}
	uc := newUC(repo)

	_, err := uc.GetStok(t.Context(), semuaAceh, domain.PermintaanSaringan{})

	require.ErrorIs(t, err, boom)
	assert.NotErrorIs(t, err, ErrValidasi)
	assert.Zero(t, uc.cache.Ukuran(), "galat tidak disimpan")
}
