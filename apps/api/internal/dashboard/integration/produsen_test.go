package integration

import (
	"math"
	"net/http"
	"testing"
	"time"

	"github.com/garda-inflasi/api/pkg/seed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Agregasi produsen (08-B): harga produsen sepekan terakhir, harga konsumen pembanding, margin, dan produksi neraca.

const (
	pasarBandaAceh = "PSR-1171-01"
	pasarAcehBesar = "PSR-1108-01"
	pasarAcehTngh  = "PSR-1106-01"
)

// rataDariTabelDasar menghitung rata-rata harga dan jumlah entri langsung dari entri_harga (tanpa agregat harian dan
// tanpa tertimbang): entri DIVALIDASI dan DIPUBLIKASI dalam tujuh hari WIB terakhir, untuk tipe, komoditas, dan
// kabupaten (kosong: semua) yang diberikan.
func (s *suite) rataDariTabelDasar(t *testing.T, tipe, komoditas, kabupaten string) (rata float64, n int) {
	t.Helper()
	h := hariIni()
	var row struct {
		Rata *float64
		N    int
	}
	require.NoError(t, s.db.Raw(`
		SELECT AVG(e.harga)::float8 AS rata, COUNT(*) AS n
		  FROM entri_harga e JOIN pasar p ON p.kode = e.pasar_id
		 WHERE e.tipe = ?::tipe_harga AND e.komoditas_kode = ? AND e.status IN ('DIVALIDASI', 'DIPUBLIKASI')
		   AND e.dicatat_pada >= ? AND e.dicatat_pada < ? AND (?::text = '' OR p.kabupaten_kode = ?)`,
		tipe, komoditas, pukul(h.AddDate(0, 0, -6), 0, 0), pukul(h.AddDate(0, 0, 1), 0, 0), kabupaten, kabupaten).Scan(&row).Error)
	if row.Rata == nil {
		return 0, 0
	}
	return *row.Rata, row.N
}

// Data demo (demo/03_harga_harian.sql dan demo/14_eksekutif_data.sql): harga produsen ada untuk empat komoditas, dan
// neraca provinsi untuk delapan komoditas bulan lalu. Rata-rata API dibandingkan dengan perhitungan langsung dari
// entri_harga yang tidak melewati agregat harian.
func TestProdusen_DataDemoSesuaiTabelDasar(t *testing.T) {
	s := terpisah(t, "dashboard_produsen_demo", true)
	require.NoError(t, seed.SeedFiles(t.Context(), s.db, "demo/14_eksekutif_data.sql"))
	s.segarkan(t)

	r := s.produsenDari(t, emailBerhak, "")

	h := hariIni()
	assert.Equal(t, tanggal(h.AddDate(0, 0, -6)), r.Dari)
	assert.Equal(t, tanggal(h), r.Sampai)
	// Empat komoditas berharga produsen ditambah empat yang hanya punya produksi. Komoditas pilot lebih dulu menurut nama.
	assert.ElementsMatch(t, []string{"BWM", "CBM-KRT", "KTG", "TMT", "BRS-MED", "DAG-AYM", "IKN-TGK", "TLR"}, r.kodeKomoditas())
	assert.Equal(t, []string{"BWM", "BRS-MED", "CBM-KRT"}, r.kodeKomoditas()[:3], "pilot menurut nama: Bawang merah, Beras medium, Cabai merah keriting")

	for _, kode := range []string{"BWM", "CBM-KRT", "KTG", "TMT"} {
		k := r.cari(t, kode)
		rataP, nP := s.rataDariTabelDasar(t, "PRODUSEN", kode, "")
		require.NotNil(t, k.Produsen, kode)
		assert.Equal(t, math.Round(rataP), k.Produsen.Harga, "harga produsen %s = rata-rata entri, dibulatkan", kode)
		assert.Equal(t, nP, k.Produsen.JumlahEntri, "entri produsen %s", kode)

		require.NotNil(t, k.Konsumen, kode)
		rataK, nK := s.rataDariTabelDasar(t, "KONSUMEN", kode, "")
		assert.Equal(t, math.Round(rataK), k.Konsumen.Harga, "harga konsumen %s (kabupaten demo sama di kedua sisi)", kode)
		assert.Equal(t, nK, k.Konsumen.JumlahEntri, kode)

		require.NotNil(t, k.Margin, kode)
		assert.Equal(t, k.Konsumen.Harga-k.Produsen.Harga, k.Margin.Selisih, kode)
		assert.Positive(t, k.Margin.Selisih, "harga konsumen demo di atas harga produsen: %s", kode)
		assert.InDelta(t, 100*k.Margin.Selisih/k.Produsen.Harga, k.Margin.PersenProdusen, 0.051, kode)
		assert.InDelta(t, 100*k.Margin.Selisih/k.Konsumen.Harga, k.Margin.PersenKonsumen, 0.051, kode)
		assert.Equal(t, "kg", k.Satuan)
	}

	// Produksi dari neraca provinsi bulan lalu, berlabel demo.
	cabai := r.cari(t, "CBM-KRT")
	require.NotNil(t, cabai.Produksi)
	assert.Equal(t, 3100.0, cabai.Produksi.Ton)
	assert.Equal(t, tanggal(awalBulan(-1)), cabai.Produksi.Periode)
	assert.Equal(t, "DEMO", cabai.Produksi.Sumber)
	assert.True(t, cabai.Produksi.Demo)

	beras := r.cari(t, "BRS-MED")
	assert.Nil(t, beras.Produsen, "tanpa entri harga produsen")
	assert.Nil(t, beras.Konsumen, "konsumen hanya dari kabupaten yang punya harga produsen")
	assert.Nil(t, beras.Margin, "margin hanya bila kedua sisi ada")
	require.NotNil(t, beras.Produksi)
	assert.Equal(t, 146200.0, beras.Produksi.Ton)
}

// Hitungan tangan. Hari ini = H; jendela H-6..H. Hanya entri DIVALIDASI dan DIPUBLIKASI.
//
//	TMT produsen: 1171 H 5000 (DIVALIDASI), 5200; H-3 5100; H-6 5300 -> 4 entri, jumlah 20.600
//	              1108 H-1 4800, 4900, 5000                            -> 3 entri, jumlah 14.700
//	              (tidak dihitung: H DIAJUKAN 6000, H DRAF 7000, H DITOLAK 9999, H-7 100000)
//	              harga produsen = 35.300 / 7 = 5.042,857 -> 5.043; 7 entri; 2 kabupaten
//	TMT konsumen: 1171 H 6000, 6400; H-2 6200 (18.600/3); 1108 H-1 5600; 1106 H 9000, 9000 (1106 tanpa produsen TMT: bukan pembanding)
//	              harga konsumen = (18.600 + 5.600) / 4 = 6.050; 4 entri; 2 kabupaten
//	              margin: 6.050 - 5.043 = 1.007; terhadap produsen 1.007 / 5.043 = 19,97% -> 20,0; terhadap konsumen 1.007 / 6.050 = 16,64% -> 16,6
//	BWM produsen: 1106 H 30000, 31000 -> 30.500; konsumen hanya di 1171 (tanpa produsen) -> konsumen dan margin null
//	CBM-KRT produsen: 1171 H 40000, 42000; 1106 H-5 38000 x 4 -> 234.000 / 6 = 39.000; 6 entri; 2 kabupaten
//	          konsumen: 1171 H 50000, 52000, 54000 -> 52.000; 3 entri; 1 kabupaten (1106 tidak punya konsumen)
//	          margin: 13.000; terhadap produsen 33,3%; terhadap konsumen 25,0%
//	KTG: hanya konsumen (1171 H 13000) dan produksi neraca -> produsen, konsumen, margin null
//	GLA: hanya konsumen, tanpa produksi -> tidak ada di daftar
func produsenTerkendali(t *testing.T, nama string) (*suite, time.Time) {
	t.Helper()
	s := terpisah(t, nama, false)
	h := hariIni()
	hari := func(n int) time.Time { return h.AddDate(0, 0, n) }

	// TMT produsen
	s.sisipHargaTetap(t, hari(0), pasarBandaAceh, "TMT", "PRODUSEN", "DIVALIDASI", 5000)
	s.sisipEntriPasti(t, "mahasiswa_1", pasarBandaAceh, "TMT", "PRODUSEN", "DIPUBLIKASI", 5200, pukul(hari(0), 9, 1))
	s.sisipHargaTetap(t, hari(-3), pasarBandaAceh, "TMT", "PRODUSEN", "DIPUBLIKASI", 5100)
	s.sisipHargaTetap(t, hari(-6), pasarBandaAceh, "TMT", "PRODUSEN", "DIPUBLIKASI", 5300)
	s.sisipHargaTetap(t, hari(-1), pasarAcehBesar, "TMT", "PRODUSEN", "DIPUBLIKASI", 4800, 4900, 5000)
	// TMT produsen yang tidak boleh dihitung
	s.sisipHargaTetap(t, hari(0), pasarBandaAceh, "TMT", "PRODUSEN", "DIAJUKAN", 6000)
	s.sisipHargaTetap(t, hari(0), pasarBandaAceh, "TMT", "PRODUSEN", "DRAF", 7000)
	s.sisipEntriPasti(t, "mahasiswa_1", pasarBandaAceh, "TMT", "PRODUSEN", "DITOLAK", 9999, pukul(hari(0), 10, 0))
	s.sisipHargaTetap(t, hari(-7), pasarBandaAceh, "TMT", "PRODUSEN", "DIPUBLIKASI", 100000)
	// TMT konsumen
	s.sisipHargaTetap(t, hari(0), pasarBandaAceh, "TMT", "KONSUMEN", "DIPUBLIKASI", 6000, 6400)
	s.sisipHargaTetap(t, hari(-2), pasarBandaAceh, "TMT", "KONSUMEN", "DIPUBLIKASI", 6200)
	s.sisipHargaTetap(t, hari(-1), pasarAcehBesar, "TMT", "KONSUMEN", "DIPUBLIKASI", 5600)
	s.sisipHargaTetap(t, hari(0), pasarAcehTngh, "TMT", "KONSUMEN", "DIPUBLIKASI", 9000, 9000)
	s.sisipHargaTetap(t, hari(0), pasarBandaAceh, "TMT", "KONSUMEN", "DIAJUKAN", 7000)
	s.sisipHargaTetap(t, hari(-7), pasarBandaAceh, "TMT", "KONSUMEN", "DIPUBLIKASI", 1)
	// BWM
	s.sisipHargaTetap(t, hari(0), pasarAcehTngh, "BWM", "PRODUSEN", "DIPUBLIKASI", 30000, 31000)
	s.sisipHargaTetap(t, hari(0), pasarBandaAceh, "BWM", "KONSUMEN", "DIPUBLIKASI", 45000)
	// CBM-KRT
	s.sisipHargaTetap(t, hari(0), pasarBandaAceh, "CBM-KRT", "PRODUSEN", "DIPUBLIKASI", 40000, 42000)
	s.sisipHargaTetap(t, hari(-5), pasarAcehTngh, "CBM-KRT", "PRODUSEN", "DIPUBLIKASI", 38000, 38000, 38000, 38000)
	s.sisipHargaTetap(t, hari(0), pasarBandaAceh, "CBM-KRT", "KONSUMEN", "DIPUBLIKASI", 50000, 52000, 54000)
	// KTG dan GLA: hanya konsumen
	s.sisipHargaTetap(t, hari(0), pasarBandaAceh, "KTG", "KONSUMEN", "DIPUBLIKASI", 13000)
	s.sisipHargaTetap(t, hari(0), pasarBandaAceh, "GLA", "KONSUMEN", "DIPUBLIKASI", 17000)

	// Neraca pangan: provinsi (DEMO, dua periode), satu baris kabupaten resmi untuk TMT.
	p1, p2 := awalBulan(-1), awalBulan(-2)
	s.sisipNeracaPasti(t, "TMT", "11", p1, 4100, "DEMO")
	s.sisipNeracaPasti(t, "TMT", "11", p2, 3000, "DEMO")
	s.sisipNeracaPasti(t, "TMT", "1171", p1, 100, "BPS")
	s.sisipNeracaPasti(t, "BWM", "11", p1, 900, "DEMO")
	s.sisipNeracaPasti(t, "CBM-KRT", "11", p1, 3100, "DEMO")
	// Baris kabupaten bersumber sama (DEMO) dengan baris provinsinya: bila keduanya sampai tercampur (mis. pencocokan
	// wilayah menurut awalan "11"), tampilan provinsi menjumlahkan atau menggandakan CBM-KRT. Di tampilan 1171 baris ini
	// tidak dipakai karena 1171 punya angka resmi (TMT, BPS) dan angka resmi mengalahkan contoh.
	s.sisipNeracaPasti(t, "CBM-KRT", "1171", p1, 50, "DEMO")
	s.sisipNeracaPasti(t, "KTG", "11", p1, 9800, "DEMO")
	s.segarkan(t)
	return s, h
}

func TestProdusen_AngkaHasilHitungTangan(t *testing.T) {
	s, h := produsenTerkendali(t, "dashboard_produsen_terkendali")

	r := s.produsenDari(t, emailBerhak, "")

	assert.Equal(t, tanggal(h.AddDate(0, 0, -6)), r.Dari)
	assert.Equal(t, tanggal(h), r.Sampai)
	assert.Equal(t, []string{"BWM", "CBM-KRT", "KTG", "TMT"}, r.kodeKomoditas(), "pilot (bawang, cabai) lebih dulu, lalu kentang dan tomat; GLA tanpa produsen dan produksi tidak ada")

	tomat := r.cari(t, "TMT")
	assert.Equal(t, &sisiHarga{Harga: 5043, JumlahEntri: 7, JumlahKabupaten: 2}, tomat.Produsen,
		"DIVALIDASI dan DIPUBLIKASI dalam H-6..H; DIAJUKAN, DRAF, DITOLAK, dan H-7 tidak")
	assert.Equal(t, &sisiHarga{Harga: 6050, JumlahEntri: 4, JumlahKabupaten: 2}, tomat.Konsumen,
		"konsumen hanya dari kabupaten yang punya harga produsen tomat: 1106 (9000) tidak ikut")
	assert.Equal(t, &margin{Selisih: 1007, PersenProdusen: 20.0, PersenKonsumen: 16.6}, tomat.Margin)
	assert.Equal(t, 6050.0-5043.0, tomat.Margin.Selisih, "angka yang tampil saling menjumlah")
	assert.Equal(t, &produksi{Ton: 4100, Periode: tanggal(awalBulan(-1)), Sumber: "DEMO", Demo: true}, tomat.Produksi,
		"baris provinsi periode terbaru; baris 1171 (100 t) tidak dijumlahkan, periode lama (3000) tidak dipakai")

	bawang := r.cari(t, "BWM")
	assert.Equal(t, &sisiHarga{Harga: 30500, JumlahEntri: 2, JumlahKabupaten: 1}, bawang.Produsen)
	assert.Nil(t, bawang.Konsumen, "konsumen BWM hanya di kabupaten tanpa produsen")
	assert.Nil(t, bawang.Margin)
	require.NotNil(t, bawang.Produksi)
	assert.Equal(t, 900.0, bawang.Produksi.Ton)

	cabai := r.cari(t, "CBM-KRT")
	assert.Equal(t, &sisiHarga{Harga: 39000, JumlahEntri: 6, JumlahKabupaten: 2}, cabai.Produsen)
	assert.Equal(t, &sisiHarga{Harga: 52000, JumlahEntri: 3, JumlahKabupaten: 1}, cabai.Konsumen,
		"celah cakupan terlihat: dua kabupaten produsen, satu kabupaten konsumen")
	assert.Equal(t, &margin{Selisih: 13000, PersenProdusen: 33.3, PersenKonsumen: 25.0}, cabai.Margin)

	kentang := r.cari(t, "KTG")
	assert.Nil(t, kentang.Produsen)
	assert.Nil(t, kentang.Konsumen)
	assert.Nil(t, kentang.Margin)
	require.NotNil(t, kentang.Produksi, "komoditas dengan produksi tetapi tanpa harga produsen tetap tampil")
	assert.Equal(t, 9800.0, kentang.Produksi.Ton)

	// Rata-rata API sama dengan rata-rata langsung dari tabel dasar.
	for kode, kab := range map[string]string{"TMT": "", "CBM-KRT": "", "BWM": ""} {
		rata, n := s.rataDariTabelDasar(t, "PRODUSEN", kode, kab)
		k := r.cari(t, kode)
		assert.Equal(t, math.Round(rata), k.Produsen.Harga, kode)
		assert.Equal(t, n, k.Produsen.JumlahEntri, kode)
	}
}

// Wilayah neraca: tampilan seluruh Aceh memakai baris provinsi, tampilan satu kabupaten memakai baris kabupaten itu;
// keduanya tidak pernah dijumlahkan. Bukan baris provinsi bagi pemegang kabupaten.
func TestProdusen_WilayahProduksiTidakPernahDijumlahkan(t *testing.T) {
	s, _ := produsenTerkendali(t, "dashboard_produsen_wilayah")

	provinsi := s.produsenDari(t, emailBerhak, "")
	assert.Equal(t, 4100.0, provinsi.cari(t, "TMT").Produksi.Ton, "bukan 4100 + 100")
	assert.Equal(t, 3100.0, provinsi.cari(t, "CBM-KRT").Produksi.Ton, "bukan 3100 + 50, dan satu baris saja")
	assert.Equal(t, []string{"BWM", "CBM-KRT", "KTG", "TMT"}, provinsi.kodeKomoditas(), "tidak ada baris ganda")

	t.Run("filter kabupaten pada peran provinsi", func(t *testing.T) {
		r := s.produsenDari(t, emailBerhak, "?kabupaten_kode="+kabBanda)

		assert.Equal(t, []string{"CBM-KRT", "TMT"}, r.kodeKomoditas(), "komoditas berharga produsen di 1171; kentang tidak (produksinya hanya tingkat provinsi)")
		tomat := r.cari(t, "TMT")
		assert.Equal(t, &sisiHarga{Harga: 5150, JumlahEntri: 4, JumlahKabupaten: 1}, tomat.Produsen, "20.600 / 4")
		assert.Equal(t, &sisiHarga{Harga: 6200, JumlahEntri: 3, JumlahKabupaten: 1}, tomat.Konsumen, "18.600 / 3")
		assert.Equal(t, &margin{Selisih: 1050, PersenProdusen: 20.4, PersenKonsumen: 16.9}, tomat.Margin)
		assert.Equal(t, &produksi{Ton: 100, Periode: tanggal(awalBulan(-1)), Sumber: "BPS", Demo: false}, tomat.Produksi,
			"baris kabupaten 1171, resmi; bukan baris provinsi")
		cabai := r.cari(t, "CBM-KRT")
		assert.Equal(t, &sisiHarga{Harga: 41000, JumlahEntri: 2, JumlahKabupaten: 1}, cabai.Produsen)
		assert.Equal(t, &sisiHarga{Harga: 52000, JumlahEntri: 3, JumlahKabupaten: 1}, cabai.Konsumen)
		assert.Equal(t, &margin{Selisih: 11000, PersenProdusen: 26.8, PersenKonsumen: 21.2}, cabai.Margin)
		assert.Nil(t, cabai.Produksi, "1171 punya angka resmi (TMT, BPS): baris DEMO-nya (50 t) tidak dipakai, dan baris provinsi (3100 t) bukan miliknya")
	})
	t.Run("Koordinator dan Agregator sama dengan filter kabupaten tugasnya", func(t *testing.T) {
		filter := s.produsenDari(t, emailBerhak, "?kabupaten_kode="+kabBanda)
		for _, email := range []string{emailKoordinator, emailAgregator} {
			assert.Equal(t, filter, s.produsenDari(t, email, ""), email)
		}
	})
	t.Run("kabupaten tanpa baris neraca", func(t *testing.T) {
		r := s.produsenDari(t, emailBerhak, "?kabupaten_kode="+kabSelatan)
		assert.Equal(t, []string{"TMT"}, r.kodeKomoditas())
		tomat := r.cari(t, "TMT")
		assert.Equal(t, &sisiHarga{Harga: 4900, JumlahEntri: 3, JumlahKabupaten: 1}, tomat.Produsen, "14.700 / 3")
		assert.Equal(t, &margin{Selisih: 700, PersenProdusen: 14.3, PersenKonsumen: 12.5}, tomat.Margin)
		assert.Nil(t, tomat.Produksi, "kosong, bukan nol dan bukan produksi provinsi")
	})
}

// Angka resmi lebih diutamakan daripada contoh (aturan yang sama dengan dashboard eksekutif): begitu ada neraca resmi
// untuk provinsi, baris DEMO tidak lagi dipakai, juga untuk komoditas yang tidak punya baris resmi.
func TestProdusen_NeracaResmiMengalahkanDemo(t *testing.T) {
	s, _ := produsenTerkendali(t, "dashboard_produsen_resmi")
	require.NotNil(t, s.produsenDari(t, emailBerhak, "").cari(t, "TMT").Produksi, "sebelum ada angka resmi")

	s.sisipNeracaPasti(t, "BRS-MED", "11", awalBulan(-1), 140000, "BPS")
	s.segarkan(t)
	r := s.produsenDari(t, emailBerhak, "")

	assert.Nil(t, r.cari(t, "TMT").Produksi, "baris DEMO tidak dipakai bila ada angka resmi untuk wilayah itu")
	assert.Nil(t, r.cari(t, "CBM-KRT").Produksi)
	beras := r.cari(t, "BRS-MED")
	require.NotNil(t, beras.Produksi)
	assert.Equal(t, &produksi{Ton: 140000, Periode: tanggal(awalBulan(-1)), Sumber: "BPS", Demo: false}, beras.Produksi)
	assert.NotContains(t, r.kodeKomoditas(), "KTG", "KTG hanya punya produksi DEMO dan tidak punya harga produsen")
}

func TestProdusen_FilterKomoditasDanKabupaten(t *testing.T) {
	s, _ := produsenTerkendali(t, "dashboard_produsen_filter")

	tomat := s.produsenDari(t, emailBerhak, "?komoditas_kode=TMT")
	assert.Equal(t, []string{"TMT"}, tomat.kodeKomoditas())
	assert.Equal(t, s.produsenDari(t, emailBerhak, "").cari(t, "TMT"), tomat.Komoditas[0], "filter adalah irisan tampilan penuh")

	bawang := s.produsenDari(t, emailBerhak, "?kabupaten_kode=1106&komoditas_kode=BWM")
	require.Len(t, bawang.Komoditas, 1)
	assert.Equal(t, &sisiHarga{Harga: 30500, JumlahEntri: 2, JumlahKabupaten: 1}, bawang.Komoditas[0].Produsen)
	assert.Nil(t, bawang.Komoditas[0].Konsumen, "1106 tidak punya harga konsumen BWM")

	for _, q := range []string{"?komoditas_kode=TIDAK-ADA", "?kabupaten_kode=9999", "?kabupaten_kode=1101"} {
		kosong := s.produsenDari(t, emailBerhak, q)
		assert.NotNil(t, kosong.Komoditas, q)
		assert.Empty(t, kosong.Komoditas, q)
	}
}

func TestProdusen_CakupanKoordinatorFilterDiLuarCakupanKosong(t *testing.T) {
	s, _ := produsenTerkendali(t, "dashboard_produsen_cakupan")

	for _, email := range []string{emailKoordinator, emailAgregator} {
		lain := s.produsenDari(t, email, "?kabupaten_kode="+kabSelatan)
		assert.NotNil(t, lain.Komoditas)
		assert.Empty(t, lain.Komoditas, "%s: 1108 punya data, tetapi di luar cakupan", email)
		assert.Equal(t, []string{"CBM-KRT", "TMT"}, s.produsenDari(t, email, "?kabupaten_kode="+kabBanda).kodeKomoditas())
	}

	// Dipindah ke 1108: tampilan berpindah, tanpa login ulang.
	s.pindahKabupaten(t, emailKoordinator, kabSelatan)
	r := s.produsenDari(t, emailKoordinator, "")
	assert.Equal(t, []string{"TMT"}, r.kodeKomoditas())
	assert.Equal(t, 4900.0, r.cari(t, "TMT").Produsen.Harga, "harga 1108, bukan 1171 dari cache")

	s.pindahKabupaten(t, emailKoordinator, "")
	tertutup := s.produsenDari(t, emailKoordinator, "")
	assert.NotNil(t, tertutup.Komoditas)
	assert.Empty(t, tertutup.Komoditas, "tanpa kabupaten tugas: tertutup")
}

// Hasil cache satu cakupan tidak boleh tersaji ke cakupan lain.
func TestProdusen_HasilCacheTidakTersajiKeCakupanLain(t *testing.T) {
	s, _ := produsenTerkendali(t, "dashboard_produsen_cache")

	for range 3 {
		for _, email := range []string{emailBerhak, emailKoordinator, emailManajerData, emailAgregator} {
			provinsi := email == emailBerhak || email == emailManajerData
			tomat := s.produsenDari(t, email, "?komoditas_kode=TMT").cari(t, "TMT")
			assert.Equal(t, map[bool]float64{true: 5043, false: 5150}[provinsi], tomat.Produsen.Harga, email)
		}
	}
}

// Penyegaran agregat (publikasi harga) membuang hasil lama tanpa menunggu TTL.
func TestProdusen_DataBaruTampilSetelahPenyegaran(t *testing.T) {
	s, h := produsenTerkendali(t, "dashboard_produsen_segar")
	sebelum := s.produsenDari(t, emailBerhak, "?komoditas_kode=BWM").cari(t, "BWM").Produsen
	require.Equal(t, 30500.0, sebelum.Harga)

	s.sisipHargaTetap(t, h, pasarAcehTngh, "BWM", "PRODUSEN", "DIPUBLIKASI", 32500, 32500)
	assert.Equal(t, 30500.0, s.produsenDari(t, emailBerhak, "?komoditas_kode=BWM").cari(t, "BWM").Produsen.Harga, "dalam TTL dan sebelum penyegaran")

	s.segarkan(t)
	sesudah := s.produsenDari(t, emailBerhak, "?komoditas_kode=BWM").cari(t, "BWM").Produsen
	assert.Equal(t, 31500.0, sesudah.Harga, "(30000 + 31000 + 32500 + 32500) / 4")
	assert.Equal(t, 4, sesudah.JumlahEntri)
}

func TestProdusen_ParameterTidakValidDijawab400(t *testing.T) {
	s := sharedSuite(t)
	for _, q := range []string{"?kabupaten_kode=abc", "?kabupaten_kode=117", "?komoditas_kode=cabai%20merah", "?komoditas_kode=A"} {
		assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodGet, awalan+"/produsen"+q, emailBerhak, nil).Code, q)
	}
}

func TestProdusen_Otorisasi(t *testing.T) {
	s := sharedSuite(t)
	jalur := awalan + "/produsen"

	assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodGet, jalur, "", nil).Code)
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, jalur, emailTakBerhak, nil).Code)
	for _, email := range []string{emailBerhak, emailKoordinator, emailAgregator, emailManajerData} {
		assert.Equal(t, http.StatusOK, s.call(t, http.MethodGet, jalur, email, nil).Code, email)
	}
}

func TestProdusen_BasisDataKosongTidakGalat(t *testing.T) {
	s := terpisah(t, "dashboard_produsen_kosong", false)

	r := s.produsenDari(t, emailBerhak, "")

	assert.NotNil(t, r.Komoditas)
	assert.Empty(t, r.Komoditas)
	assert.Contains(t, string(s.mentah(t, awalan+"/produsen", emailBerhak)), `"komoditas":[]`)
}
