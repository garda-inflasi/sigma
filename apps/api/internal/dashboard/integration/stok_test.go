package integration

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Stok real-time (08-B): lot yang berlaku per komoditas, kapasitas, dan utilisasi. Definisinya meminjam KPI cs
// (migrasi 000124) dan modul cold-storage; tes ini membuktikan keduanya tidak bertentangan.

// Data demo (reference/09_cold_storage.sql dan demo/11_cold_storage_ops.sql): 23 lot di empat fasilitas. CS-01 347 t,
// CS-02 240 t, CS-03 193 t (operasional, kapasitas 400/300/250) dan CS-04 33 t (perawatan). Hanya yang operasional
// dihitung (780 t terhadap 950 t, 82,1%); lot CS-04 dilaporkan terpisah sebagai di luar operasional.
func TestStok_DataDemo(t *testing.T) {
	s := sharedSuite(t)

	r := s.stokDari(t, emailBerhak, "")

	assert.Equal(t, tanggal(hariIni()), r.Tanggal)
	assert.Equal(t, 7, r.AmbangKedaluwarsaHari)
	assert.Equal(t, 0.6, r.AmbangRisiko)
	g := r.Ringkasan
	assert.Equal(t, 780.0, g.TotalTon)
	assert.Equal(t, 21, g.JumlahLot, "23 lot dikurangi dua lot CS-04")
	assert.Equal(t, 3, g.JumlahFasilitas)
	assert.Equal(t, 950, g.KapasitasTon)
	assert.Equal(t, 3, g.FasilitasOperasional)
	assert.Equal(t, 82.1, g.UtilisasiPct)
	assert.Equal(t, 114.4, g.MendekatiKedaluwarsaTon, "34,9 (CS01-01) + 31,0 + 15,4 + 33,1 (CS03-01..03): sisa masa simpan <= 7 hari")
	assert.Equal(t, 172.7, g.RisikoTinggiTon, "empat lot berisiko > 0,6, sama dengan halaman cold storage")
	assert.Zero(t, g.KedaluwarsaLot)
	assert.Zero(t, g.KedaluwarsaTon)
	assert.Equal(t, 33.0, g.DiLuarOperasionalTon, "12,9 t telur dan 20,1 t bawang di CS-04 (perawatan)")

	// Terbesar lebih dulu: ikan tongkol 240, bawang merah 162,1, cabai merah 151,3, kentang 99,1, tomat 79,5, rawit 48.
	assert.Equal(t, []string{"IKN-TGK", "BWM", "CBM-KRT", "KTG", "TMT", "CBR-RWT"}, r.kodeKomoditas())
	cabai, _ := r.cari("CBM-KRT")
	assert.Equal(t, stokKomoditas{KomoditasKode: "CBM-KRT", KomoditasNama: "Cabai merah keriting", TotalTon: 151.3, JumlahLot: 5, JumlahFasilitas: 2,
		MendekatiKedaluwarsaTon: 34.9, RisikoTinggiTon: 34.9}, cabai, "dua fasilitas: CS-01 (4 lot) dan CS-03 (1 lot)")
	tomat, _ := r.cari("TMT")
	assert.Equal(t, 79.5, tomat.TotalTon)
	assert.Equal(t, 79.5, tomat.MendekatiKedaluwarsaTon, "semua lot tomat sisa masa simpannya 6-7 hari")
	assert.Equal(t, 31.0, tomat.RisikoTinggiTon)
	_, ada := r.cari("TLR")
	assert.False(t, ada, "telur hanya ada di fasilitas perawatan")

	// Ringkasan dan baris per komoditas berasal dari fakta yang sama.
	var jumlahTon float64
	var jumlahLot int
	for _, k := range r.Komoditas {
		jumlahTon += k.TotalTon
		jumlahLot += k.JumlahLot
	}
	assert.InDelta(t, g.TotalTon, jumlahTon, 0.35, "selisih hanya dari pembulatan satu desimal per komoditas")
	assert.Equal(t, g.JumlahLot, jumlahLot)
}

// Rekonsiliasi dengan modul cold-storage: total volume lot yang belum habis (RingkasanStok halaman cold storage) =
// stok berlaku + stok kedaluwarsa + stok di fasilitas non-operasional (+ lot yang belum masuk, nol pada data demo).
func TestStok_DapatDirekonsiliasiDenganHalamanColdStorage(t *testing.T) {
	s := sharedSuite(t)

	r := s.stokDari(t, emailBerhak, "").Ringkasan

	var volumeColdStorage float64
	require.NoError(t, s.db.Raw(`SELECT COALESCE(SUM(volume_ton), 0) FROM stok_cold_storage WHERE status <> 'HABIS'`).Scan(&volumeColdStorage).Error)
	assert.InDelta(t, volumeColdStorage, r.TotalTon+r.KedaluwarsaTon+r.DiLuarOperasionalTon, 0.15)
	assert.InDelta(t, 813.0, volumeColdStorage, 0.001, "347 + 240 + 193 + 33 t pada seed demo")
}

// Utilisasi sama dengan kartu KPI cs pada tampilan yang sama: satu definisi stok berlaku dan kapasitas.
func TestStok_UtilisasiSamaDenganKartuKpiCs(t *testing.T) {
	s := sharedSuite(t)
	s.segarkan(t)

	cs := func(kpis []kpi) float64 { return kartuByID(t, kpis, "cs").Nilai }

	tabel := []struct {
		nama, query string
	}{
		{"seluruh Aceh", ""},
		{"kabupaten dengan cold storage", "?kabupaten_kode=" + kabSelatan},
		{"kabupaten dengan fasilitas perawatan saja", "?kabupaten_kode=1107"},
		{"kabupaten tanpa fasilitas", "?kabupaten_kode=" + kabBanda},
		{"komoditas: pembilang hanya komoditas itu, kapasitas tetap", "?komoditas_kode=CBM-KRT"},
		{"kabupaten dan komoditas", "?kabupaten_kode=1106&komoditas_kode=TMT"},
		{"komoditas yang tidak punya stok", "?komoditas_kode=MYK"},
	}
	for _, c := range tabel {
		t.Run(c.nama, func(t *testing.T) {
			stok := s.stokDari(t, emailBerhak, c.query)
			kartu := cs(s.kpiDari(t, emailBerhak, c.query))

			assert.Equal(t, kartu, stok.Ringkasan.UtilisasiPct, "utilisasi stok dan kartu cs")
		})
	}
}

// Hitungan tangan pada data terkendali. Hari ini = H (tanggal Aceh); lot berlaku bila tanggal masuk <= H dan tanggal
// kedaluwarsa > H; mendekati kedaluwarsa bila tanggal kedaluwarsa <= H+7; risiko tinggi bila risiko > 0,6.
//
//	lot  komoditas  fasilitas  ton   masuk      kedaluwarsa     risiko status   -> kelas
//	A    TMT        CS-01      10    H-3        H+10            0,30           berlaku
//	B    TMT        CS-01      20    H-2        H+7 23.59       0,60           berlaku, mendekati (H+7 termasuk), BUKAN risiko tinggi (tepat 0,6)
//	C    TMT        CS-03       5    H-2        H+8 00.00       0,61           berlaku, bukan mendekati (H+8), risiko tinggi
//	D    BWM        CS-01      30    H-1        H+1 00.00       0,90           berlaku (besok), mendekati, risiko tinggi
//	E    BWM        CS-01       7    H-5        H 23.59         0,20           KEDALUWARSA (tanggalnya hari ini)
//	F    BWM        CS-01       8    H-9        H-1 12.00       0,95           KEDALUWARSA
//	G    CBM-KRT    CS-01       4    H+1 08.00  H+20            0,10           belum masuk (tidak dihitung)
//	H    CBM-KRT    CS-01       6    H 23.59    H+15            0,10           berlaku (tanggal masuk hari ini)
//	I    IKN-TGK    CS-02      12    H-10       H+30            0,80           berlaku, risiko tinggi
//	J    TMT        CS-01      50    H-3        H+10            0,10  HABIS    tidak dihitung di mana pun
//	K    BWM        CS-03       3    H-1        H+3             0,50  PRIORITAS_JUAL berlaku, mendekati
//	L    TLR        CS-04       9    H-2        H+9             0,10           di luar operasional (perawatan)
//	M    BWM        CS-04       4    H-4        H-2             0,10           di luar operasional (walau kedaluwarsa: kelas fasilitas lebih dulu)
//
// Seluruh Aceh:
//
//	TMT      A + B + C            = 35,0 t, 3 lot, 2 fasilitas (CS-01, CS-03), mendekati 20,0 (B), risiko 5,0 (C)
//	BWM      D + K                = 33,0 t, 2 lot, 2 fasilitas, mendekati 33,0 (D + K), risiko 30,0 (D)
//	IKN-TGK  I                    = 12,0 t, 1 lot, 1 fasilitas, risiko 12,0
//	CBM-KRT  H                    =  6,0 t, 1 lot, 1 fasilitas
//	total 86,0 t; 7 lot; 3 fasilitas; kapasitas 400 + 300 + 250 = 950; utilisasi 86 / 950 = 9,1%
//	mendekati 20 + 33 = 53,0; risiko 5 + 30 + 12 = 47,0; kedaluwarsa (E + F) 15,0 t di 2 lot; di luar operasional 9 + 4 = 13,0
func stokTerkendali(t *testing.T, nama string) (*suite, time.Time) {
	t.Helper()
	s := terpisah(t, nama, false)
	h := hariIni()
	hari := func(n int, jam, menit int) time.Time { return pukul(h.AddDate(0, 0, n), jam, menit) }

	s.sisipLotPasti(t, "CS-01", "TMT", 10, hari(-3, 8, 0), hari(10, 8, 0), 0.30, "AKTIF")
	s.sisipLotPasti(t, "CS-01", "TMT", 20, hari(-2, 8, 0), hari(7, 23, 59), 0.60, "AKTIF")
	s.sisipLotPasti(t, "CS-03", "TMT", 5, hari(-2, 8, 0), hari(8, 0, 0), 0.61, "AKTIF")
	s.sisipLotPasti(t, "CS-01", "BWM", 30, hari(-1, 8, 0), hari(1, 0, 0), 0.90, "AKTIF")
	s.sisipLotPasti(t, "CS-01", "BWM", 7, hari(-5, 8, 0), hari(0, 23, 59), 0.20, "AKTIF")
	s.sisipLotPasti(t, "CS-01", "BWM", 8, hari(-9, 8, 0), hari(-1, 12, 0), 0.95, "AKTIF")
	s.sisipLotPasti(t, "CS-01", "CBM-KRT", 4, hari(1, 8, 0), hari(20, 8, 0), 0.10, "AKTIF")
	s.sisipLotPasti(t, "CS-01", "CBM-KRT", 6, hari(0, 23, 59), hari(15, 8, 0), 0.10, "AKTIF")
	s.sisipLotPasti(t, "CS-02", "IKN-TGK", 12, hari(-10, 8, 0), hari(30, 8, 0), 0.80, "AKTIF")
	s.sisipLotPasti(t, "CS-01", "TMT", 50, hari(-3, 8, 0), hari(10, 8, 0), 0.10, "HABIS")
	s.sisipLotPasti(t, "CS-03", "BWM", 3, hari(-1, 8, 0), hari(3, 8, 0), 0.50, "PRIORITAS_JUAL")
	s.sisipLotPasti(t, "CS-04", "TLR", 9, hari(-2, 8, 0), hari(9, 8, 0), 0.10, "AKTIF")
	s.sisipLotPasti(t, "CS-04", "BWM", 4, hari(-4, 8, 0), hari(-2, 8, 0), 0.10, "AKTIF")
	return s, h
}

func TestStok_AngkaHasilHitungTangan(t *testing.T) {
	s, _ := stokTerkendali(t, "dashboard_stok_terkendali")

	r := s.stokDari(t, emailBerhak, "")

	g := r.Ringkasan
	assert.Equal(t, 86.0, g.TotalTon)
	assert.Equal(t, 7, g.JumlahLot)
	assert.Equal(t, 3, g.JumlahFasilitas)
	assert.Equal(t, 950, g.KapasitasTon)
	assert.Equal(t, 3, g.FasilitasOperasional)
	assert.Equal(t, 9.1, g.UtilisasiPct)
	assert.Equal(t, 53.0, g.MendekatiKedaluwarsaTon)
	assert.Equal(t, 47.0, g.RisikoTinggiTon, "lot B bertepatan 0,6 tidak termasuk (di ATAS ambang)")
	assert.Equal(t, 2, g.KedaluwarsaLot)
	assert.Equal(t, 15.0, g.KedaluwarsaTon)
	assert.Equal(t, 13.0, g.DiLuarOperasionalTon)

	assert.Equal(t, []string{"TMT", "BWM", "IKN-TGK", "CBM-KRT"}, r.kodeKomoditas())
	tomat, _ := r.cari("TMT")
	assert.Equal(t, stokKomoditas{KomoditasKode: "TMT", KomoditasNama: "Tomat", TotalTon: 35, JumlahLot: 3, JumlahFasilitas: 2,
		MendekatiKedaluwarsaTon: 20, RisikoTinggiTon: 5}, tomat)
	bawang, _ := r.cari("BWM")
	assert.Equal(t, stokKomoditas{KomoditasKode: "BWM", KomoditasNama: "Bawang merah", TotalTon: 33, JumlahLot: 2, JumlahFasilitas: 2,
		MendekatiKedaluwarsaTon: 33, RisikoTinggiTon: 30}, bawang, "lot PRIORITAS_JUAL ikut; yang kedaluwarsa dan di perawatan tidak")
	ikan, _ := r.cari("IKN-TGK")
	assert.Equal(t, stokKomoditas{KomoditasKode: "IKN-TGK", KomoditasNama: "Ikan tongkol", TotalTon: 12, JumlahLot: 1, JumlahFasilitas: 1, RisikoTinggiTon: 12}, ikan)
	rawit, _ := r.cari("CBM-KRT")
	assert.Equal(t, stokKomoditas{KomoditasKode: "CBM-KRT", KomoditasNama: "Cabai merah keriting", TotalTon: 6, JumlahLot: 1, JumlahFasilitas: 1}, rawit,
		"lot yang masuk besok tidak dihitung; lot yang masuk hari ini (tanggal) dihitung")

	// Rekonsiliasi: semua lot yang belum habis = berlaku + kedaluwarsa + di luar operasional + belum masuk (G, 4 t).
	var belumHabis float64
	require.NoError(t, s.db.Raw(`SELECT SUM(volume_ton) FROM stok_cold_storage WHERE status <> 'HABIS'`).Scan(&belumHabis).Error)
	assert.InDelta(t, 118.0, belumHabis, 0.001)
	assert.InDelta(t, belumHabis, g.TotalTon+g.KedaluwarsaTon+g.DiLuarOperasionalTon+4.0, 0.001)
}

func TestStok_FilterKomoditasDanKabupatenHasilHitungTangan(t *testing.T) {
	s, _ := stokTerkendali(t, "dashboard_stok_filter")

	t.Run("komoditas TMT: kapasitas tetap seluruh fasilitas, pembilang hanya tomat", func(t *testing.T) {
		r := s.stokDari(t, emailBerhak, "?komoditas_kode=TMT")
		assert.Equal(t, 35.0, r.Ringkasan.TotalTon)
		assert.Equal(t, 3, r.Ringkasan.JumlahLot)
		assert.Equal(t, 2, r.Ringkasan.JumlahFasilitas)
		assert.Equal(t, 950, r.Ringkasan.KapasitasTon)
		assert.Equal(t, 3.7, r.Ringkasan.UtilisasiPct, "35 / 950")
		assert.Equal(t, 20.0, r.Ringkasan.MendekatiKedaluwarsaTon)
		assert.Equal(t, 5.0, r.Ringkasan.RisikoTinggiTon)
		assert.Zero(t, r.Ringkasan.KedaluwarsaTon, "lot HABIS tomat tidak dihitung di mana pun")
		assert.Zero(t, r.Ringkasan.DiLuarOperasionalTon)
		assert.Equal(t, []string{"TMT"}, r.kodeKomoditas())
	})
	t.Run("komoditas BWM: kedaluwarsa dan di perawatan ikut difilter", func(t *testing.T) {
		r := s.stokDari(t, emailBerhak, "?komoditas_kode=BWM")
		assert.Equal(t, 33.0, r.Ringkasan.TotalTon)
		assert.Equal(t, 15.0, r.Ringkasan.KedaluwarsaTon)
		assert.Equal(t, 4.0, r.Ringkasan.DiLuarOperasionalTon, "lot M saja")
	})
	t.Run("kabupaten 1108 (CS-01)", func(t *testing.T) {
		r := s.stokDari(t, emailBerhak, "?kabupaten_kode="+kabSelatan)
		assert.Equal(t, 66.0, r.Ringkasan.TotalTon, "A 10 + B 20 + D 30 + H 6")
		assert.Equal(t, 4, r.Ringkasan.JumlahLot)
		assert.Equal(t, 1, r.Ringkasan.JumlahFasilitas)
		assert.Equal(t, 400, r.Ringkasan.KapasitasTon)
		assert.Equal(t, 16.5, r.Ringkasan.UtilisasiPct)
		assert.Equal(t, 50.0, r.Ringkasan.MendekatiKedaluwarsaTon, "B 20 + D 30")
		assert.Equal(t, 30.0, r.Ringkasan.RisikoTinggiTon, "D")
		assert.Equal(t, 15.0, r.Ringkasan.KedaluwarsaTon)
		assert.Zero(t, r.Ringkasan.DiLuarOperasionalTon)
	})
	t.Run("kabupaten 1107: hanya fasilitas perawatan", func(t *testing.T) {
		r := s.stokDari(t, emailBerhak, "?kabupaten_kode=1107")
		assert.Zero(t, r.Ringkasan.TotalTon)
		assert.Zero(t, r.Ringkasan.KapasitasTon, "fasilitas perawatan bukan kapasitas operasional")
		assert.Zero(t, r.Ringkasan.UtilisasiPct)
		assert.Equal(t, 13.0, r.Ringkasan.DiLuarOperasionalTon)
		assert.Empty(t, r.Komoditas)
	})
	t.Run("kabupaten tanpa fasilitas dan kode yang tidak ada", func(t *testing.T) {
		for _, kab := range []string{kabBanda, "9999"} {
			r := s.stokDari(t, emailBerhak, "?kabupaten_kode="+kab)
			assert.Zero(t, r.Ringkasan.TotalTon+float64(r.Ringkasan.KapasitasTon), kab)
			assert.NotNil(t, r.Komoditas, kab)
			assert.Empty(t, r.Komoditas, kab)
		}
	})
	t.Run("komoditas yang tidak ada: kosong, kapasitas tetap", func(t *testing.T) {
		r := s.stokDari(t, emailBerhak, "?komoditas_kode=TIDAK-ADA")
		assert.Zero(t, r.Ringkasan.TotalTon)
		assert.Equal(t, 950, r.Ringkasan.KapasitasTon)
		assert.Empty(t, r.Komoditas)
	})
}

// Row scope (02-B): Koordinator dan Agregator hanya melihat fasilitas di kabupaten tugasnya.
func TestStok_CakupanKoordinatorDanAgregator(t *testing.T) {
	s, _ := stokTerkendali(t, "dashboard_stok_cakupan")

	t.Run("Banda Aceh tidak punya fasilitas: nol, bukan stok seluruh Aceh", func(t *testing.T) {
		for _, email := range []string{emailKoordinator, emailAgregator} {
			r := s.stokDari(t, email, "")
			assert.Zero(t, r.Ringkasan.TotalTon, email)
			assert.Zero(t, r.Ringkasan.KapasitasTon, email)
			assert.NotNil(t, r.Komoditas)
			assert.Empty(t, r.Komoditas)
		}
	})
	t.Run("dipindah ke Aceh Besar: hanya CS-01, sama dengan filter kabupaten", func(t *testing.T) {
		s.pindahKabupaten(t, emailKoordinator, kabSelatan)
		s.pindahKabupaten(t, emailAgregator, kabSelatan)
		filter := s.stokDari(t, emailBerhak, "?kabupaten_kode="+kabSelatan)

		for _, email := range []string{emailKoordinator, emailAgregator} {
			r := s.stokDari(t, email, "")
			assert.Equal(t, filter, r, email)
			assert.Equal(t, 66.0, r.Ringkasan.TotalTon)

			// Filter di luar cakupan: kosong, bukan 403 dan bukan data kabupaten lain.
			lain := s.stokDari(t, email, "?kabupaten_kode=1106")
			assert.Zero(t, lain.Ringkasan.TotalTon+float64(lain.Ringkasan.KapasitasTon), email)
			assert.Empty(t, lain.Komoditas)
			// Filter komoditas tetap bekerja di dalam cakupan.
			assert.Equal(t, 30.0, s.stokDari(t, email, "?komoditas_kode=BWM").Ringkasan.TotalTon, email)
		}
	})
	t.Run("tanpa kabupaten tugas: tertutup", func(t *testing.T) {
		s.pindahKabupaten(t, emailKoordinator, "")
		r := s.stokDari(t, emailKoordinator, "")
		assert.Zero(t, r.Ringkasan.TotalTon+float64(r.Ringkasan.KapasitasTon))
		assert.Empty(t, r.Komoditas)
	})
}

// Keterkinian: stok dibaca langsung dari tabel. Transaksi yang baru masuk tampil begitu cache (TTL 20 detik) tidak
// lagi menyimpan hasil lama; penyegaran agregat (epoch) membuangnya seketika.
func TestStok_DataBaruTampilSetelahCacheDibuang(t *testing.T) {
	s, h := stokTerkendali(t, "dashboard_stok_segar")
	sebelum := s.stokDari(t, emailBerhak, "").Ringkasan.TotalTon

	s.sisipLotPasti(t, "CS-02", "KTG", 25, pukul(h, 6, 0), pukul(h.AddDate(0, 0, 20), 8, 0), 0.1, "AKTIF")

	assert.Equal(t, sebelum, s.stokDari(t, emailBerhak, "").Ringkasan.TotalTon, "dalam TTL hasil tersimpan disajikan")
	s.segarkan(t)
	assert.Equal(t, sebelum+25, s.stokDari(t, emailBerhak, "").Ringkasan.TotalTon)
}

func TestStok_ParameterTidakValidDijawab400(t *testing.T) {
	s := sharedSuite(t)
	for _, q := range []string{"?kabupaten_kode=abc", "?kabupaten_kode=117", "?komoditas_kode=cabai%20merah", "?komoditas_kode=A", "?komoditas_kode=-ABC"} {
		w := s.call(t, http.MethodGet, awalan+"/stok"+q, emailBerhak, nil)
		assert.Equal(t, http.StatusBadRequest, w.Code, q)
	}
}

func TestStok_Otorisasi(t *testing.T) {
	s := sharedSuite(t)
	jalur := awalan + "/stok"

	assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodGet, jalur, "", nil).Code)
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, jalur, emailTakBerhak, nil).Code)
	for _, email := range []string{emailBerhak, emailKoordinator, emailAgregator, emailManajerData, "superadmin@gardainflasi.id"} {
		assert.Equal(t, http.StatusOK, s.call(t, http.MethodGet, jalur, email, nil).Code, email)
	}
}

func TestStok_BasisDataKosongNolBukanGalat(t *testing.T) {
	s := terpisah(t, "dashboard_stok_kosong", false)

	r := s.stokDari(t, emailBerhak, "")

	assert.Zero(t, r.Ringkasan.TotalTon)
	assert.Zero(t, r.Ringkasan.UtilisasiPct)
	assert.Equal(t, 950, r.Ringkasan.KapasitasTon, "kapasitas fasilitas ada walau belum ada lot")
	assert.NotNil(t, r.Komoditas)
	assert.Empty(t, r.Komoditas)
	assert.Contains(t, string(s.mentah(t, awalan+"/stok", emailBerhak)), `"komoditas":[]`)
}
