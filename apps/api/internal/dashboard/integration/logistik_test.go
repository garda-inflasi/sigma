package integration

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/garda-inflasi/api/pkg/seed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Pergerakan logistik (08-B): transaksi stok sepekan terakhir per jenis dan komoditas, transaksi terbaru, dan rute
// distribusi aktif.

// Data demo: empat lot masuk dalam tujuh hari (CS01-02, -04, -05, -06) dan lima rute usulan (demo/09_spasial_neraca.sql).
func TestLogistik_DataDemo(t *testing.T) {
	s := terpisah(t, "dashboard_logistik_demo", true)
	require.NoError(t, seed.SeedFiles(t.Context(), s.db, "demo/09_spasial_neraca.sql"))

	r := s.logistikDari(t, emailBerhak, "")

	h := hariIni()
	assert.Equal(t, tanggal(h.AddDate(0, 0, -6)), r.Dari)
	assert.Equal(t, tanggal(h), r.Sampai)
	assert.Equal(t, gerakRingkasan{Masuk: gerakJenis{Jumlah: 4, VolumeTon: 122.2}}, r.Ringkasan,
		"29,4 + 38,2 + 18,6 + 36,0 t; demo hanya punya penerimaan awal (MASUK)")
	require.Len(t, r.Komoditas, 2)
	assert.Equal(t, gerakKomoditas{KomoditasKode: "CBM-KRT", KomoditasNama: "Cabai merah keriting", Masuk: gerakJenis{Jumlah: 2, VolumeTon: 74.2}}, r.Komoditas[0])
	assert.Equal(t, gerakKomoditas{KomoditasKode: "CBR-RWT", KomoditasNama: "Cabai rawit merah", Masuk: gerakJenis{Jumlah: 2, VolumeTon: 48}}, r.Komoditas[1])

	require.Len(t, r.Terbaru, 4)
	for i, p := range r.Terbaru {
		assert.Equal(t, "MASUK", p.Jenis)
		assert.Nil(t, p.Asal)
		require.NotNil(t, p.Tujuan)
		assert.Equal(t, "CS-01", *p.Tujuan.Kode)
		assert.Equal(t, "Cold Storage Lambaro", p.Tujuan.Nama)
		assert.Equal(t, kabSelatan, p.Tujuan.KabupatenKode)
		assert.NotEmpty(t, p.Tujuan.KabupatenNama)
		if i > 0 {
			assert.False(t, p.Waktu.After(r.Terbaru[i-1].Waktu), "terbaru lebih dulu")
		}
	}

	assert.Equal(t, 5, r.Rute.Jumlah)
	assert.Equal(t, 166.0, r.Rute.VolumeTon, "42 + 39 + 34 + 28 + 23")
	assert.EqualValues(t, 47275123+33450682+25760039+15245072+16174627, r.Rute.Biaya)
	require.Len(t, r.Rute.Items, 5)
	assert.Equal(t, "RT-1106-1171", *r.Rute.Items[0].Kode, "biaya terbesar lebih dulu")
	assert.Equal(t, "USULAN", r.Rute.Items[0].Status)
	assert.Equal(t, "KABUPATEN ACEH TENGAH", r.Rute.Items[0].Asal)
	for i := 1; i < len(r.Rute.Items); i++ {
		assert.LessOrEqual(t, r.Rute.Items[i].Biaya, r.Rute.Items[i-1].Biaya)
	}

	// Koordinator Banda Aceh: tidak ada fasilitas di kabupatennya, tetapi satu rute tujuan 1171.
	k := s.logistikDari(t, emailKoordinator, "")
	assert.Equal(t, gerakRingkasan{}, k.Ringkasan)
	assert.Empty(t, k.Komoditas)
	assert.Empty(t, k.Terbaru)
	assert.Equal(t, 1, k.Rute.Jumlah)
	assert.Equal(t, "RT-1106-1171", *k.Rute.Items[0].Kode)
}

// Hitungan tangan. Hari ini = H; jendela [H-6 00.00 WIB, H+1 00.00 WIB).
//
//	#  jenis   komoditas  ton    asal -> tujuan     waktu
//	1  MASUK   TMT        20,0   -     -> CS-01     H 09.00
//	2  MASUK   BWM        30,0   -     -> CS-03     H-3 10.00
//	3  KELUAR  TMT         5,0   CS-01 -            H-1 09.00
//	4  PINDAH  TMT         8,0   CS-01 -> CS-03     H-2 09.00
//	5  PINDAH  BWM         4,0   CS-02 -> CS-01     H-1 12.00
//	6  MASUK   CBM-KRT    12,5   -     -> CS-02     H-6 00.00.00   (batas awal: termasuk)
//	7  MASUK   TMT         9,0   -     -> CS-01     H-7 23.59.59   (di luar jendela)
//	8  KELUAR  BWM         1,0   CS-03 -            H+1 00.00.00   (batas akhir eksklusif: besok)
//	9  MASUK   TMT         0,04  -     -> CS-04     H 10.00        (fasilitas perawatan; transaksi tetap dihitung)
//
//	CS-01 = Aceh Besar (1108), CS-02 = Lhokseumawe (1174), CS-03 = Aceh Tengah (1106), CS-04 = Aceh Barat (1107).
//
// Seluruh Aceh: MASUK #1 #2 #6 #9 = 4 transaksi, 62,54 -> 62,5 t; KELUAR #3 = 1, 5,0; PINDAH #4 #5 = 2, 12,0.
//
//	BWM      masuk 1/30,0  pindah 1/4,0           -> total 34,0
//	TMT      masuk 2/20,04 keluar 1/5,0 pindah 1/8,0 -> total 33,04
//	CBM-KRT  masuk 1/12,5                          -> total 12,5
//
// Rute: lima baris, satu DIGANTIKAN (tidak aktif). Rute menghubungkan kabupaten, bukan fasilitas.
//
//	A 1106 -> 1171 CBM-KRT 42,0 t Rp47.275.123;  B 1117 -> 1108 CBM-KRT 39,0 t Rp33.450.682;  C 1109 -> 1174 BWM 34,0 t Rp25.760.039
//	D 1106 -> 1107 CBM-KRT 28,0 t Rp15.245.072 DIGANTIKAN;  E 1108 -> 1171 TMT 5,5 t Rp1.000.000
type kunciGerak map[int]int64

func logistikTerkendali(t *testing.T, nama string) (*suite, kunciGerak) {
	t.Helper()
	s := terpisah(t, nama, false)
	h := hariIni()
	hari := func(n int, jam, menit int) time.Time { return pukul(h.AddDate(0, 0, n), jam, menit) }
	sedia := hari(-30, 8, 0)
	kedaluwarsa := hari(60, 8, 0)

	lotTmt01 := s.sisipLotPasti(t, "CS-01", "TMT", 100, sedia, kedaluwarsa, 0.1, "AKTIF")
	lotBwm03 := s.sisipLotPasti(t, "CS-03", "BWM", 100, sedia, kedaluwarsa, 0.1, "AKTIF")
	lotBwm02 := s.sisipLotPasti(t, "CS-02", "BWM", 100, sedia, kedaluwarsa, 0.1, "AKTIF")
	lotCbm02 := s.sisipLotPasti(t, "CS-02", "CBM-KRT", 100, sedia, kedaluwarsa, 0.1, "AKTIF")
	lotTmt04 := s.sisipLotPasti(t, "CS-04", "TMT", 100, sedia, kedaluwarsa, 0.1, "AKTIF")

	id := kunciGerak{}
	// #1 membawa pelaku dan alasan bebas yang memuat nama orang: keduanya tidak boleh sampai ke respons.
	id[1] = s.sisipTransaksiPasti(t, lotTmt01, "MASUK", 20, "", "CS-01", hari(0, 9, 0), "mahasiswa_1", "Diterima oleh Aulia Rahman dari KUD Rahasia")
	id[2] = s.sisipTransaksiPasti(t, lotBwm03, "MASUK", 30, "", "CS-03", hari(-3, 10, 0), "", "")
	id[3] = s.sisipTransaksiPasti(t, lotTmt01, "KELUAR", 5, "CS-01", "", hari(-1, 9, 0), "", "")
	id[4] = s.sisipTransaksiPasti(t, lotTmt01, "PINDAH", 8, "CS-01", "CS-03", hari(-2, 9, 0), "", "")
	id[5] = s.sisipTransaksiPasti(t, lotBwm02, "PINDAH", 4, "CS-02", "CS-01", hari(-1, 12, 0), "", "")
	id[6] = s.sisipTransaksiPasti(t, lotCbm02, "MASUK", 12.5, "", "CS-02", hari(-6, 0, 0), "", "")
	id[7] = s.sisipTransaksiPasti(t, lotTmt01, "MASUK", 9, "", "CS-01", time.Date(h.Year(), h.Month(), h.Day()-7, 23, 59, 59, 0, wibTes), "", "")
	id[8] = s.sisipTransaksiPasti(t, lotBwm03, "KELUAR", 1, "CS-03", "", hari(1, 0, 0), "", "")
	id[9] = s.sisipTransaksiPasti(t, lotTmt04, "MASUK", 0.04, "", "CS-04", hari(0, 10, 0), "", "")

	s.sisipRutePasti(t, "RT-A", "1106", "1171", "CBM-KRT", 42.0, 47275123, "USULAN")
	s.sisipRutePasti(t, "RT-B", "1117", "1108", "CBM-KRT", 39.0, 33450682, "USULAN")
	s.sisipRutePasti(t, "RT-C", "1109", "1174", "BWM", 34.0, 25760039, "USULAN")
	s.sisipRutePasti(t, "RT-D", "1106", "1107", "CBM-KRT", 28.0, 15245072, "DIGANTIKAN")
	s.sisipRutePasti(t, "RT-E", "1108", "1171", "TMT", 5.5, 1000000, "USULAN")
	return s, id
}

func (k kunciGerak) urut(nomor ...int) []string {
	out := make([]string, len(nomor))
	for i, n := range nomor {
		out[i] = strconv.FormatInt(k[n], 10)
	}
	return out
}

func kodeRute(r ruteRingkas) []string {
	kode := make([]string, len(r.Items))
	for i, it := range r.Items {
		kode[i] = *it.Kode
	}
	return kode
}

func TestLogistik_AngkaHasilHitungTangan(t *testing.T) {
	s, id := logistikTerkendali(t, "dashboard_logistik_terkendali")

	r := s.logistikDari(t, emailBerhak, "")

	assert.Equal(t, gerakRingkasan{
		Masuk:  gerakJenis{Jumlah: 4, VolumeTon: 62.5},
		Keluar: gerakJenis{Jumlah: 1, VolumeTon: 5.0},
		Pindah: gerakJenis{Jumlah: 2, VolumeTon: 12.0},
	}, r.Ringkasan, "#7 (H-7 23.59.59) dan #8 (besok 00.00) di luar jendela; #6 (H-6 00.00) di dalam")

	require.Len(t, r.Komoditas, 3)
	assert.Equal(t, gerakKomoditas{KomoditasKode: "BWM", KomoditasNama: "Bawang merah",
		Masuk: gerakJenis{1, 30}, Pindah: gerakJenis{1, 4}}, r.Komoditas[0], "tonase terbesar lebih dulu; KELUAR BWM besok tidak ikut")
	assert.Equal(t, gerakKomoditas{KomoditasKode: "TMT", KomoditasNama: "Tomat",
		Masuk: gerakJenis{2, 20}, Keluar: gerakJenis{1, 5}, Pindah: gerakJenis{1, 8}}, r.Komoditas[1], "20,04 -> 20,0")
	assert.Equal(t, gerakKomoditas{KomoditasKode: "CBM-KRT", KomoditasNama: "Cabai merah keriting", Masuk: gerakJenis{1, 12.5}}, r.Komoditas[2])

	assert.Equal(t, id.urut(9, 1, 5, 3, 4, 2, 6), r.idTerbaru(), "waktu menurun: #9 #1 (hari ini), #5 #3, #4, #2, #6")
	assert.Equal(t, 0.04, r.Terbaru[0].VolumeTon, "satu transaksi dalam ketelitian kilogram")

	// Pindah antarfasilitas membawa kedua ujungnya dengan nama dan kabupaten; MASUK tanpa asal.
	pindah := r.Terbaru[2] // #5: CS-02 (Lhokseumawe) -> CS-01 (Aceh Besar)
	assert.Equal(t, "PINDAH", pindah.Jenis)
	require.NotNil(t, pindah.Asal)
	require.NotNil(t, pindah.Tujuan)
	assert.Equal(t, "CS-02", *pindah.Asal.Kode)
	assert.Equal(t, "1174", pindah.Asal.KabupatenKode)
	assert.NotEmpty(t, pindah.Asal.Nama)
	assert.NotEmpty(t, pindah.Asal.KabupatenNama)
	assert.Equal(t, "CS-01", *pindah.Tujuan.Kode)
	assert.Equal(t, kabSelatan, pindah.Tujuan.KabupatenKode)
	keluar := r.Terbaru[3] // #3
	assert.Equal(t, "KELUAR", keluar.Jenis)
	require.NotNil(t, keluar.Asal)
	assert.Nil(t, keluar.Tujuan)

	assert.Equal(t, 4, r.Rute.Jumlah, "RT-D DIGANTIKAN tidak aktif")
	assert.Equal(t, 120.5, r.Rute.VolumeTon)
	assert.EqualValues(t, 107485844, r.Rute.Biaya)
	assert.Equal(t, []string{"RT-A", "RT-B", "RT-C", "RT-E"}, kodeRute(r.Rute), "biaya menurun")
	assert.Equal(t, "KOTA BANDA ACEH", r.Rute.Items[0].Tujuan)
	assert.Equal(t, "CBM-KRT", r.Rute.Items[0].KomoditasKode)
	assert.Equal(t, 42.0, r.Rute.Items[0].VolumeTon)
	assert.Equal(t, 100, r.Rute.Items[0].JarakKm)
	assert.Equal(t, 2.5, r.Rute.Items[0].JamTempuh)
}

// Tanpa data pribadi: pelaku (oleh_id) dan alasan bebas transaksi tidak dikembalikan; ID transaksi berupa string desimal.
func TestLogistik_TanpaPelakuDanAlasan(t *testing.T) {
	s, id := logistikTerkendali(t, "dashboard_logistik_pii")

	mentah := string(s.mentah(t, awalan+"/logistik", emailBerhak))

	for _, larangan := range []string{"Aulia", "Rahasia", "mahasiswa", "oleh", "alasan", "pelaku", "@gardainflasi"} {
		assert.NotContains(t, mentah, larangan)
	}
	assert.Contains(t, mentah, `"id":"`+strconv.FormatInt(id[1], 10)+`"`, "ID BIGINT sebagai string desimal")
	r := s.logistikDari(t, emailBerhak, "")
	for _, p := range r.Terbaru {
		assert.NotZero(t, p.ID)
	}
}

// Cakupan baris (02-B): pergerakan masuk cakupan bila fasilitas asal ATAU tujuannya di kabupaten; rute bila kabupaten
// asal ATAU tujuannya di sana. Satu PINDAH antarkabupaten tampak di kedua kabupaten dan satu kali di seluruh Aceh.
func TestLogistik_CakupanAsalAtauTujuan(t *testing.T) {
	s, id := logistikTerkendali(t, "dashboard_logistik_cakupan")

	tabel := []struct {
		nama        string
		query       string
		ringkasan   gerakRingkasan
		terbaru     []int
		kodeRute    []string
		jumlahRute  int
		volumeRute  float64
		biayaRute   int64
		komoditasDi []string
	}{
		{
			nama: "Aceh Besar 1108 (CS-01): masuk, keluar, dan kedua PINDAH (asal dan tujuan)", query: "?kabupaten_kode=1108",
			ringkasan: gerakRingkasan{Masuk: gerakJenis{1, 20}, Keluar: gerakJenis{1, 5}, Pindah: gerakJenis{2, 12}},
			terbaru:   []int{1, 5, 3, 4}, kodeRute: []string{"RT-B", "RT-E"}, jumlahRute: 2, volumeRute: 44.5, biayaRute: 34450682,
			komoditasDi: []string{"TMT", "BWM"},
		},
		{
			nama: "Aceh Tengah 1106 (CS-03): masuk #2 dan tujuan PINDAH #4 (asalnya di luar)", query: "?kabupaten_kode=1106",
			ringkasan: gerakRingkasan{Masuk: gerakJenis{1, 30}, Pindah: gerakJenis{1, 8}},
			terbaru:   []int{4, 2}, kodeRute: []string{"RT-A"}, jumlahRute: 1, volumeRute: 42, biayaRute: 47275123,
			komoditasDi: []string{"BWM", "TMT"},
		},
		{
			nama: "Lhokseumawe 1174 (CS-02): asal PINDAH #5 dan masuk #6", query: "?kabupaten_kode=1174",
			ringkasan: gerakRingkasan{Masuk: gerakJenis{1, 12.5}, Pindah: gerakJenis{1, 4}},
			terbaru:   []int{5, 6}, kodeRute: []string{"RT-C"}, jumlahRute: 1, volumeRute: 34, biayaRute: 25760039,
			komoditasDi: []string{"CBM-KRT", "BWM"},
		},
		{
			nama: "Aceh Barat 1107 (CS-04, perawatan): transaksinya tetap tercatat", query: "?kabupaten_kode=1107",
			ringkasan: gerakRingkasan{Masuk: gerakJenis{1, 0}},
			terbaru:   []int{9}, kodeRute: []string{}, jumlahRute: 0, volumeRute: 0, biayaRute: 0,
			komoditasDi: []string{"TMT"},
		},
		{
			nama: "Banda Aceh 1171 tanpa fasilitas: tanpa pergerakan, tetapi dua rute bertujuan ke sana", query: "?kabupaten_kode=1171",
			ringkasan: gerakRingkasan{}, terbaru: []int{}, kodeRute: []string{"RT-A", "RT-E"}, jumlahRute: 2, volumeRute: 47.5, biayaRute: 48275123,
			komoditasDi: []string{},
		},
	}
	for _, c := range tabel {
		t.Run(c.nama, func(t *testing.T) {
			r := s.logistikDari(t, emailBerhak, c.query)

			assert.Equal(t, c.ringkasan, r.Ringkasan)
			assert.Equal(t, id.urut(c.terbaru...), r.idTerbaru())
			assert.Equal(t, c.kodeRute, kodeRute(r.Rute))
			assert.Equal(t, c.jumlahRute, r.Rute.Jumlah)
			assert.Equal(t, c.volumeRute, r.Rute.VolumeTon)
			assert.Equal(t, c.biayaRute, r.Rute.Biaya)
			kode := make([]string, len(r.Komoditas))
			for i, k := range r.Komoditas {
				kode[i] = k.KomoditasKode
			}
			assert.Equal(t, c.komoditasDi, kode)
			assert.NotNil(t, r.Terbaru)
			assert.NotNil(t, r.Rute.Items)
		})
	}
}

func TestLogistik_CakupanKoordinatorDanAgregator(t *testing.T) {
	s, id := logistikTerkendali(t, "dashboard_logistik_peran")

	t.Run("Banda Aceh: tidak ada pergerakan, dua rute bertujuan ke sana", func(t *testing.T) {
		for _, email := range []string{emailKoordinator, emailAgregator} {
			r := s.logistikDari(t, email, "")
			assert.Equal(t, gerakRingkasan{}, r.Ringkasan, email)
			assert.Empty(t, r.Terbaru)
			assert.Equal(t, []string{"RT-A", "RT-E"}, kodeRute(r.Rute), email)
		}
	})
	t.Run("pindah ke Aceh Besar: sama dengan filter kabupaten", func(t *testing.T) {
		s.pindahKabupaten(t, emailKoordinator, kabSelatan)
		s.pindahKabupaten(t, emailAgregator, kabSelatan)
		filter := s.logistikDari(t, emailBerhak, "?kabupaten_kode="+kabSelatan)

		for _, email := range []string{emailKoordinator, emailAgregator} {
			r := s.logistikDari(t, email, "")
			assert.Equal(t, filter, r, email)
			assert.Equal(t, id.urut(1, 5, 3, 4), r.idTerbaru(), "PINDAH #5 dari Lhokseumawe tampak di kabupaten tujuannya")

			// Filter kabupaten lain: di luar cakupan, kosong (bukan 403, bukan data seluruh Aceh).
			lain := s.logistikDari(t, email, "?kabupaten_kode=1106")
			assert.Equal(t, gerakRingkasan{}, lain.Ringkasan, email)
			assert.Empty(t, lain.Terbaru)
			assert.Empty(t, lain.Rute.Items)
			assert.Zero(t, lain.Rute.Jumlah)
			// Filter komoditas di dalam cakupan tetap bekerja.
			assert.Equal(t, id.urut(1, 3, 4), s.logistikDari(t, email, "?komoditas_kode=TMT").idTerbaru())
		}
	})
	t.Run("tanpa kabupaten tugas: tertutup", func(t *testing.T) {
		s.pindahKabupaten(t, emailKoordinator, "")
		r := s.logistikDari(t, emailKoordinator, "")
		assert.Equal(t, gerakRingkasan{}, r.Ringkasan)
		assert.Empty(t, r.Terbaru)
		assert.Zero(t, r.Rute.Jumlah)
	})
}

func TestLogistik_FilterKomoditas(t *testing.T) {
	s, id := logistikTerkendali(t, "dashboard_logistik_komoditas")

	r := s.logistikDari(t, emailBerhak, "?komoditas_kode=TMT")

	assert.Equal(t, gerakRingkasan{Masuk: gerakJenis{2, 20}, Keluar: gerakJenis{1, 5}, Pindah: gerakJenis{1, 8}}, r.Ringkasan)
	assert.Equal(t, id.urut(9, 1, 3, 4), r.idTerbaru())
	assert.Equal(t, []string{"RT-E"}, kodeRute(r.Rute), "rute memakai komoditas yang sama")
	assert.Equal(t, 1, r.Rute.Jumlah)

	for _, q := range []string{"?komoditas_kode=TIDAK-ADA", "?kabupaten_kode=9999"} {
		kosong := s.logistikDari(t, emailBerhak, q)
		assert.Equal(t, gerakRingkasan{}, kosong.Ringkasan, q)
		assert.Empty(t, kosong.Terbaru, q)
		assert.Zero(t, kosong.Rute.Jumlah, q)
	}
}

// Daftar dibatasi sepuluh baris, sedangkan jumlah dan total tetap atas semuanya.
func TestLogistik_BatasDaftarDanTotalAtasSemua(t *testing.T) {
	s := terpisah(t, "dashboard_logistik_batas", false)
	h := hariIni()
	lot := s.sisipLotPasti(t, "CS-01", "TMT", 1000, pukul(h.AddDate(0, 0, -30), 8, 0), pukul(h.AddDate(0, 0, 60), 8, 0), 0.1, "AKTIF")
	var terakhir int64
	for i := range 12 {
		terakhir = s.sisipTransaksiPasti(t, lot, "MASUK", 1.5, "", "CS-01", pukul(h, 5, i), "", "")
		s.sisipRutePasti(t, "RT-"+strconv.Itoa(100+i), "1106", "1171", "CBM-KRT", 10, int64(1000000+i*10), "USULAN")
	}

	r := s.logistikDari(t, emailBerhak, "")

	assert.Equal(t, gerakJenis{Jumlah: 12, VolumeTon: 18}, r.Ringkasan.Masuk, "ringkasan atas semua 12 transaksi")
	require.Len(t, r.Terbaru, 10, "daftar terbaru dibatasi sepuluh")
	assert.Equal(t, strconv.FormatInt(terakhir, 10), r.Terbaru[0].ID, "yang paling baru lebih dulu")
	assert.Equal(t, 12, r.Rute.Jumlah, "jumlah rute atas semua rute aktif")
	assert.Equal(t, 120.0, r.Rute.VolumeTon)
	assert.EqualValues(t, 12*1000000+10*(0+11)*12/2, r.Rute.Biaya)
	require.Len(t, r.Rute.Items, 10)
	assert.Equal(t, "RT-111", *r.Rute.Items[0].Kode, "biaya terbesar lebih dulu")
	assert.Equal(t, "RT-102", *r.Rute.Items[9].Kode)
}

// Rute yang dibuat optimasi tidak punya kode lama: kode null, ID tetap string desimal.
func TestLogistik_RuteTanpaKodeLama(t *testing.T) {
	s := terpisah(t, "dashboard_logistik_rute_optimasi", false)
	require.NoError(t, s.db.Exec(`
		INSERT INTO rute_distribusi (asal_kode, tujuan_kode, komoditas_kode, volume_ton, jarak_km, jam_tempuh, biaya, status, sumber)
		VALUES ('1106', '1171', 'CBM-KRT', 12.5, 268, 6, 5000000, 'USULAN', 'OPTIMASI')`).Error)

	r := s.logistikDari(t, emailBerhak, "")

	require.Len(t, r.Rute.Items, 1)
	assert.Nil(t, r.Rute.Items[0].Kode)
	assert.NotEmpty(t, r.Rute.Items[0].ID)
	assert.Equal(t, "OPTIMASI", r.Rute.Items[0].Sumber)
	assert.True(t, strings.Contains(string(s.mentah(t, awalan+"/logistik", emailBerhak)), `"kode":null`))
}

// Hasil cache satu cakupan tidak boleh tersaji ke cakupan lain.
func TestLogistik_HasilCacheTidakTersajiKeCakupanLain(t *testing.T) {
	s, _ := logistikTerkendali(t, "dashboard_logistik_cache")

	for range 3 {
		for _, email := range []string{emailBerhak, emailKoordinator, emailManajerData, emailAgregator} {
			provinsi := email == emailBerhak || email == emailManajerData
			r := s.logistikDari(t, email, "")
			assert.Equal(t, map[bool]int{true: 7, false: 0}[provinsi], len(r.Terbaru), email)
			assert.Equal(t, map[bool]int{true: 4, false: 2}[provinsi], r.Rute.Jumlah, email)
		}
	}
}

func TestLogistik_DataBaruTampilSetelahCacheDibuang(t *testing.T) {
	s, _ := logistikTerkendali(t, "dashboard_logistik_segar")
	sebelum := s.logistikDari(t, emailBerhak, "").Ringkasan.Masuk.Jumlah
	lot := s.sisipLotPasti(t, "CS-02", "KTG", 10, pukul(hariIni(), 6, 0), pukul(hariIni().AddDate(0, 0, 20), 8, 0), 0.1, "AKTIF")

	s.sisipTransaksiPasti(t, lot, "MASUK", 10, "", "CS-02", pukul(hariIni(), 6, 0), "", "")

	assert.Equal(t, sebelum, s.logistikDari(t, emailBerhak, "").Ringkasan.Masuk.Jumlah, "dalam TTL hasil tersimpan disajikan")
	s.segarkan(t)
	assert.Equal(t, sebelum+1, s.logistikDari(t, emailBerhak, "").Ringkasan.Masuk.Jumlah)
}

func TestLogistik_ParameterTidakValidDijawab400(t *testing.T) {
	s := sharedSuite(t)
	for _, q := range []string{"?kabupaten_kode=abc", "?kabupaten_kode=117", "?komoditas_kode=cabai%20merah", "?komoditas_kode=A"} {
		assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodGet, awalan+"/logistik"+q, emailBerhak, nil).Code, q)
	}
}

func TestLogistik_Otorisasi(t *testing.T) {
	s := sharedSuite(t)
	jalur := awalan + "/logistik"

	assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodGet, jalur, "", nil).Code)
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, jalur, emailTakBerhak, nil).Code)
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, jalur, "superadmin@gardainflasi.id", nil).Code)
	for _, email := range []string{emailBerhak, emailKoordinator, emailAgregator, emailManajerData} {
		assert.Equal(t, http.StatusOK, s.call(t, http.MethodGet, jalur, email, nil).Code, email)
	}
}

func TestLogistik_BasisDataKosongTidakGalat(t *testing.T) {
	s := terpisah(t, "dashboard_logistik_kosong", false)

	r := s.logistikDari(t, emailBerhak, "")

	assert.Equal(t, gerakRingkasan{}, r.Ringkasan)
	mentah := string(s.mentah(t, awalan+"/logistik", emailBerhak))
	assert.Contains(t, mentah, `"komoditas":[]`)
	assert.Contains(t, mentah, `"terbaru":[]`)
	assert.Contains(t, mentah, `"items":[]`)
	assert.Contains(t, mentah, `"masuk":{"jumlah":0,"volume_ton":0}`)
}
