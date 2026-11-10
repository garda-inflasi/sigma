package usecase

import (
	"errors"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/dashboard/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func gerak(kom, nama, jenis string, jumlah int, ton float64) domain.GerakanAgregat {
	return domain.GerakanAgregat{KomoditasKode: kom, KomoditasNama: nama, Jenis: jenis, Jumlah: jumlah, VolumeTon: ton}
}

// Hitungan tangan:
//
//	BWM      MASUK 1 / 162,1000002 t                              -> total 162,1
//	CBM-KRT  MASUK 2 / 74,2, KELUAR 1 / 10,04, PINDAH 1 / 5,0     -> total 89,24
//	TMT      PINDAH 3 / 0,04                                       -> total 0,04
//	ringkasan: MASUK 3 / 236,3000002 -> 236,3; KELUAR 1 / 10,0; PINDAH 4 / 5,04 -> 5,0
func TestRakitLogistik_RingkasanHitunganTangan(t *testing.T) {
	l := RakitLogistik(tgl("2026-10-11"), hariUji, domain.LogistikFakta{Agregat: []domain.GerakanAgregat{
		gerak("TMT", "Tomat", domain.JenisPindah, 3, 0.04),
		gerak("CBM-KRT", "Cabai merah keriting", domain.JenisMasuk, 2, 74.2),
		gerak("BWM", "Bawang merah", domain.JenisMasuk, 1, 162.1000002),
		gerak("CBM-KRT", "Cabai merah keriting", domain.JenisKeluar, 1, 10.04),
		gerak("CBM-KRT", "Cabai merah keriting", domain.JenisPindah, 1, 5.0),
	}})

	assert.Equal(t, tgl("2026-10-11"), l.Dari)
	assert.Equal(t, hariUji, l.Sampai)
	assert.Equal(t, domain.RingkasanGerakan{
		Masuk:  domain.GerakanJenis{Jumlah: 3, VolumeTon: 236.3},
		Keluar: domain.GerakanJenis{Jumlah: 1, VolumeTon: 10.0},
		Pindah: domain.GerakanJenis{Jumlah: 4, VolumeTon: 5.0},
	}, l.Ringkasan)

	// Tonase seluruh jenis, terbesar dulu. Ketiga jenis selalu ada, bernilai nol bila tak ada transaksi.
	require.Len(t, l.Komoditas, 3)
	assert.Equal(t, domain.GerakanKomoditas{KomoditasKode: "BWM", KomoditasNama: "Bawang merah",
		Masuk: domain.GerakanJenis{Jumlah: 1, VolumeTon: 162.1}}, l.Komoditas[0])
	assert.Equal(t, domain.GerakanKomoditas{KomoditasKode: "CBM-KRT", KomoditasNama: "Cabai merah keriting",
		Masuk:  domain.GerakanJenis{Jumlah: 2, VolumeTon: 74.2},
		Keluar: domain.GerakanJenis{Jumlah: 1, VolumeTon: 10.0},
		Pindah: domain.GerakanJenis{Jumlah: 1, VolumeTon: 5.0}}, l.Komoditas[1])
	assert.Equal(t, domain.GerakanKomoditas{KomoditasKode: "TMT", KomoditasNama: "Tomat",
		Pindah: domain.GerakanJenis{Jumlah: 3, VolumeTon: 0}}, l.Komoditas[2], "0,04 t pada tonase agregat dibulatkan satu desimal")
}

func TestRakitLogistik_RingkasanSamaDenganJumlahKomoditas(t *testing.T) {
	fakta := domain.LogistikFakta{Agregat: []domain.GerakanAgregat{
		gerak("A1", "A", domain.JenisMasuk, 5, 10.3),
		gerak("B1", "B", domain.JenisMasuk, 7, 20.4),
		gerak("A1", "A", domain.JenisKeluar, 2, 3.2),
		gerak("B1", "B", domain.JenisKeluar, 4, 8.1),
	}}

	l := RakitLogistik(tgl("2026-10-11"), hariUji, fakta)

	var jumlahMasuk, jumlahKeluar int
	for _, k := range l.Komoditas {
		jumlahMasuk += k.Masuk.Jumlah
		jumlahKeluar += k.Keluar.Jumlah
	}
	assert.Equal(t, jumlahMasuk, l.Ringkasan.Masuk.Jumlah)
	assert.Equal(t, jumlahKeluar, l.Ringkasan.Keluar.Jumlah)
	assert.Equal(t, 30.7, l.Ringkasan.Masuk.VolumeTon)
	assert.Equal(t, 11.3, l.Ringkasan.Keluar.VolumeTon)
	assert.Equal(t, domain.GerakanJenis{}, l.Ringkasan.Pindah)
}

func TestRakitLogistik_SeriDiputusKodeDanJenisAsingDiabaikan(t *testing.T) {
	l := RakitLogistik(tgl("2026-10-11"), hariUji, domain.LogistikFakta{Agregat: []domain.GerakanAgregat{
		gerak("TMT", "Tomat", domain.JenisMasuk, 1, 10),
		gerak("BWM", "Bawang merah", domain.JenisMasuk, 1, 10),
		gerak("KTG", "Kentang", "SUSUT", 9, 999), // jenis di luar CHECK basis data: tidak dikarang menjadi MASUK
	}})

	require.Len(t, l.Komoditas, 3)
	assert.Equal(t, "BWM", l.Komoditas[0].KomoditasKode)
	assert.Equal(t, "TMT", l.Komoditas[1].KomoditasKode)
	assert.Equal(t, 20.0, l.Ringkasan.Masuk.VolumeTon)
	assert.Zero(t, l.Ringkasan.Keluar.Jumlah+l.Ringkasan.Pindah.Jumlah)
}

func TestRakitLogistik_KosongBukanNil(t *testing.T) {
	l := RakitLogistik(tgl("2026-10-11"), hariUji, domain.LogistikFakta{})

	assert.Equal(t, domain.RingkasanGerakan{}, l.Ringkasan)
	assert.NotNil(t, l.Komoditas)
	assert.NotNil(t, l.Terbaru)
	assert.NotNil(t, l.Rute.Items)
	assert.Empty(t, l.Komoditas)
	assert.Empty(t, l.Terbaru)
	assert.Empty(t, l.Rute.Items)
	assert.Zero(t, l.Rute.Jumlah)
}

func TestRakitLogistik_TerbaruUrutWaktuMenurunLaluIDDanBulatKilogram(t *testing.T) {
	t0 := time.Date(2026, 10, 16, 3, 0, 0, 0, time.UTC)
	l := RakitLogistik(tgl("2026-10-11"), hariUji, domain.LogistikFakta{Terbaru: []domain.Pergerakan{
		{ID: 1, Waktu: t0.Add(-time.Hour), Jenis: domain.JenisMasuk, VolumeTon: 10.0449},
		{ID: 7, Waktu: t0, Jenis: domain.JenisKeluar, VolumeTon: 0.04},
		{ID: 9, Waktu: t0, Jenis: domain.JenisPindah, VolumeTon: 12.3456},
	}})

	require.Len(t, l.Terbaru, 3)
	assert.Equal(t, []int64{9, 7, 1}, []int64{int64(l.Terbaru[0].ID), int64(l.Terbaru[1].ID), int64(l.Terbaru[2].ID)},
		"waktu menurun; waktu sama diurut ID menurun")
	assert.Equal(t, 12.346, l.Terbaru[0].VolumeTon)
	assert.Equal(t, 0.04, l.Terbaru[1].VolumeTon, "transaksi 40 kg tidak tampil 0,0 ton")
	assert.Equal(t, 10.045, l.Terbaru[2].VolumeTon)
}

func TestRakitLogistik_RuteUrutBiayaMenurunLaluIDDanTidakMengubahFakta(t *testing.T) {
	asli := []domain.RuteAktif{
		{ID: 3, Biaya: 100}, {ID: 1, Biaya: 300}, {ID: 2, Biaya: 300}, {ID: 4, Biaya: 200},
	}
	l := RakitLogistik(tgl("2026-10-11"), hariUji, domain.LogistikFakta{Rute: domain.RuteRingkas{
		Jumlah: 25, VolumeTon: 166.04, Biaya: 137905543, Items: asli,
	}})

	id := make([]int64, len(l.Rute.Items))
	for i, r := range l.Rute.Items {
		id[i] = int64(r.ID)
	}
	assert.Equal(t, []int64{1, 2, 4, 3}, id)
	assert.Equal(t, 25, l.Rute.Jumlah, "jumlah atas semua rute aktif, bukan hanya yang ditampilkan")
	assert.Equal(t, 166.0, l.Rute.VolumeTon)
	assert.EqualValues(t, 137905543, l.Rute.Biaya)
	assert.EqualValues(t, 3, asli[0].ID, "irisan milik repository tidak diubah urutannya")
}

func TestGetLogistik_JendelaTujuhHariWaktuAcehDanBatasJumlah(t *testing.T) {
	repo := &fakeRepo{}

	l, err := newUC(repo).GetLogistik(t.Context(), banda, domain.PermintaanSaringan{KabupatenKode: "1171", KomoditasKode: "TMT"})

	require.NoError(t, err)
	q := repo.qLogistik
	assert.Equal(t, banda, q.Cakupan)
	assert.Equal(t, "1171", q.KabupatenKode)
	assert.Equal(t, "TMT", q.KomoditasKode)
	// [11 Okt 00.00 WIB, 18 Okt 00.00 WIB): tujuh hari kalender Aceh yang berakhir 17 Okt, hari ini termasuk.
	assert.True(t, time.Date(2026, 10, 11, 0, 0, 0, 0, wib).Equal(q.Awal), "awal: %s", q.Awal)
	assert.True(t, time.Date(2026, 10, 18, 0, 0, 0, 0, wib).Equal(q.Akhir), "akhir: %s", q.Akhir)
	assert.Equal(t, 7*24*time.Hour, q.Akhir.Sub(q.Awal))
	assert.Equal(t, domain.MaksPergerakanTerbaru, q.MaksTerbaru)
	assert.Equal(t, domain.MaksRuteDitampilkan, q.MaksRute)
	assert.Equal(t, tgl("2026-10-11"), l.Dari)
	assert.Equal(t, hariUji, l.Sampai)
}

func TestGetLogistik_CakupanTertutupDanIrisanKosongTanpaBasisData(t *testing.T) {
	cases := []struct {
		nama    string
		cakupan domain.Cakupan
		req     domain.PermintaanSaringan
	}{
		{"cakupan tertutup", tertutup, domain.PermintaanSaringan{}},
		{"cakupan tertutup dengan filter", tertutup, domain.PermintaanSaringan{KabupatenKode: "1171"}},
		{"kabupaten di luar cakupan", banda, domain.PermintaanSaringan{KabupatenKode: "1108"}},
	}
	for _, c := range cases {
		t.Run(c.nama, func(t *testing.T) {
			repo := &fakeRepo{logistik: func(domain.LogistikQuery) (domain.LogistikFakta, error) {
				return domain.LogistikFakta{Agregat: []domain.GerakanAgregat{gerak("TMT", "Tomat", domain.JenisMasuk, 1, 5)}}, nil
			}}
			uc := newUC(repo)

			l, err := uc.GetLogistik(t.Context(), c.cakupan, c.req)

			require.NoError(t, err)
			assert.Equal(t, domain.RingkasanGerakan{}, l.Ringkasan, "pergerakan cakupan lain tidak boleh tampil")
			assert.Empty(t, l.Komoditas)
			assert.NotNil(t, l.Terbaru)
			assert.NotNil(t, l.Rute.Items)
			assert.Zero(t, repo.totalPanggilanData())
			assert.Zero(t, uc.cache.Ukuran())
		})
	}
}

func TestGetLogistik_ParameterTidakValidDanGalatRepository(t *testing.T) {
	repo := &fakeRepo{}
	_, err := newUC(repo).GetLogistik(t.Context(), semuaAceh, domain.PermintaanSaringan{KabupatenKode: "117"})
	require.ErrorIs(t, err, ErrValidasi)
	assert.Zero(t, repo.totalPanggilanData())

	boom := errors.New("db mati")
	repo = &fakeRepo{logistik: func(domain.LogistikQuery) (domain.LogistikFakta, error) { return domain.LogistikFakta{}, boom }}
	uc := newUC(repo)
	_, err = uc.GetLogistik(t.Context(), semuaAceh, domain.PermintaanSaringan{})
	require.ErrorIs(t, err, boom)
	assert.NotErrorIs(t, err, ErrValidasi)
	assert.Zero(t, uc.cache.Ukuran())
}
