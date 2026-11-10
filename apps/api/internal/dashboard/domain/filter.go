package domain

import (
	"errors"
	"fmt"
	"regexp"
	"time"
)

// ErrValidasi dibungkus dengan penjelasan untuk pemanggil; cocokkan dengan errors.Is. Pesannya aman ditampilkan
// apa adanya (tidak memuat isian pemanggil).
var ErrValidasi = errors.New("parameter tidak valid")

// Batas dan bawaan periode KPI dan heatmap. Tanggal selalu tanggal kalender waktu Aceh (WIB).
const (
	// TanggalLayout adalah format dari dan sampai.
	TanggalLayout = "2006-01-02"

	// TrenHariBawaan adalah panjang periode bawaan (hari, tanggal acuan termasuk): 14 hari, sama dengan tren
	// sebelum filter periode ada.
	TrenHariBawaan = 14
	// RentangMaksHari adalah rentang terpanjang dari..sampai (inklusif).
	RentangMaksHari = 90
	// RiwayatHari: dari dan sampai harus jatuh dalam sekian hari terakhir (hari ini termasuk). Dashboard operasional
	// menampilkan keadaan terkini; periode yang lebih tua adalah urusan dashboard eksekutif dan tren harga.
	RiwayatHari = 90

	// BandingHari: kartu KPI membandingkan nilai pada tanggal acuan dengan nilai sampai 13 hari sebelumnya (delta
	// entri membandingkan dua pekan penuh; indikator lain hanya butuh 7 hari).
	BandingHari = 13
	// JendelaValidasiHari: tingkat validasi dihitung atas jendela sekian hari yang berakhir pada tanggal itu.
	JendelaValidasiHari = 7
	// RiwayatMVHari adalah panjang riwayat fakta mv_kpi_operasional (migrasi 000124): hari ini dan 119 hari
	// sebelumnya. Harus mencakup RiwayatHari ditambah BandingHari dan JendelaValidasiHari-1 hari di belakangnya
	// (dijaga TestRiwayatMVMencukupi).
	RiwayatMVHari = 120
)

var (
	kabupatenKodeRe = regexp.MustCompile(`^[0-9]{4}$`)
	// Pola yang sama dengan kode komoditas di modul master (master/domain/perubahan.go).
	komoditasKodeRe = regexp.MustCompile(`^[A-Z0-9][A-Z0-9-]{1,29}$`)
)

// PermintaanFilter adalah isian query filter KPI dan heatmap apa adanya dari pemanggil, belum diperiksa.
type PermintaanFilter struct {
	// KabupatenKode (kode BPS 4 digit) menyaring satu kabupaten/kota; kosong: semua yang dalam cakupan.
	KabupatenKode string
	// KomoditasKode menyaring satu komoditas; kosong: semua.
	KomoditasKode string
	// Dari dan Sampai (YYYY-MM-DD, waktu Aceh, inklusif) membatasi periode; kosong: bawaan.
	Dari   string
	Sampai string
}

// Filter adalah PermintaanFilter yang sudah diperiksa dan dinormalisasi. Tanggalnya bernilai DATE (tengah malam UTC).
type Filter struct {
	KabupatenKode string
	KomoditasKode string
	// Dari adalah tanggal pertama tren KPI. Pada heatmap hanya berarti sebagai batas awal tanggal harga terakhir
	// sebuah sel (lihat HeatmapQuery.Dari).
	Dari time.Time
	// Sampai adalah tanggal acuan: nilai kartu KPI dan sel heatmap dihitung per tanggal ini.
	Sampai time.Time
	// Periode benar bila pemanggil mengisi dari atau sampai. Tanpa itu, heatmap dibaca dari mv_heatmap_harga.
	Periode bool
}

func galat(format string, a ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrValidasi}, a...)...)
}

// periksaKode memeriksa bentuk kabupaten_kode dan komoditas_kode (kosong lolos). Kode yang bentuknya benar tetapi
// tidak ada di master tidak ditolak: hasilnya kosong. Pesan galat tidak memantulkan isian pemanggil.
func periksaKode(kabupaten, komoditas string) error {
	if kabupaten != "" && !kabupatenKodeRe.MatchString(kabupaten) {
		return galat("kabupaten_kode harus 4 digit kode BPS kabupaten/kota, mis. 1171")
	}
	if komoditas != "" && !komoditasKodeRe.MatchString(komoditas) {
		return galat("komoditas_kode harus berupa huruf besar, angka, dan tanda hubung (2-30 karakter), mis. CBM-KRT")
	}
	return nil
}

// NormalisasiFilter memeriksa permintaan dan mengisi bawaannya terhadap hariIni (tanggal Aceh hari ini):
//   - sampai bawaan hari ini; dari bawaan 13 hari sebelum sampai (periode 14 hari), dipangkas ke awal riwayat.
//   - sampai tidak boleh melewati hari ini, dari tidak boleh setelah sampai, rentang paling lama RentangMaksHari
//     hari, dan periode harus berada dalam RiwayatHari hari terakhir.
//   - kabupaten_kode 4 digit; komoditas_kode huruf besar, angka, dan tanda hubung. Kode yang tidak ada di master
//     tidak ditolak: hasilnya kosong.
//
// Semua galat dibungkus ErrValidasi dengan pesan yang bisa dikirim ke pemanggil.
func NormalisasiFilter(req PermintaanFilter, hariIni time.Time) (Filter, error) {
	hariIni = time.Date(hariIni.Year(), hariIni.Month(), hariIni.Day(), 0, 0, 0, 0, time.UTC)

	if err := periksaKode(req.KabupatenKode, req.KomoditasKode); err != nil {
		return Filter{}, err
	}

	sampai, dari := hariIni, time.Time{}
	if req.Sampai != "" {
		t, err := time.Parse(TanggalLayout, req.Sampai)
		if err != nil {
			return Filter{}, galat("sampai harus berformat YYYY-MM-DD (tanggal waktu Aceh), mis. %s", hariIni.Format(TanggalLayout))
		}
		sampai = t
	}
	if req.Dari != "" {
		t, err := time.Parse(TanggalLayout, req.Dari)
		if err != nil {
			return Filter{}, galat("dari harus berformat YYYY-MM-DD (tanggal waktu Aceh), mis. %s", hariIni.Format(TanggalLayout))
		}
		dari = t
	}

	awalRiwayat := hariIni.AddDate(0, 0, -(RiwayatHari - 1))
	switch {
	case sampai.After(hariIni):
		return Filter{}, galat("sampai tidak boleh setelah hari ini (%s, waktu Aceh)", hariIni.Format(TanggalLayout))
	case req.Dari != "" && dari.After(hariIni):
		return Filter{}, galat("dari tidak boleh setelah hari ini (%s, waktu Aceh)", hariIni.Format(TanggalLayout))
	case req.Dari != "" && dari.After(sampai):
		return Filter{}, galat("dari tidak boleh setelah sampai")
	case req.Dari != "" && HariAntara(dari, sampai) > RentangMaksHari:
		return Filter{}, galat("rentang dari..sampai paling lama %d hari", RentangMaksHari)
	case req.Dari != "" && dari.Before(awalRiwayat), req.Dari == "" && sampai.Before(awalRiwayat):
		return Filter{}, galat("periode di luar riwayat dashboard: hanya %d hari terakhir yang tersedia (sejak %s)",
			RiwayatHari, awalRiwayat.Format(TanggalLayout))
	}
	if req.Dari == "" {
		dari = sampai.AddDate(0, 0, -(TrenHariBawaan - 1))
		if dari.Before(awalRiwayat) {
			dari = awalRiwayat
		}
	}

	return Filter{
		KabupatenKode: req.KabupatenKode,
		KomoditasKode: req.KomoditasKode,
		Dari:          dari,
		Sampai:        sampai,
		Periode:       req.Dari != "" || req.Sampai != "",
	}, nil
}

// HariAntara menghitung jumlah hari kalender dari awal sampai akhir, keduanya termasuk. Kedua tanggal bernilai DATE
// (tengah malam UTC), jadi selisihnya selalu kelipatan 24 jam.
func HariAntara(awal, akhir time.Time) int {
	return int(akhir.Sub(awal)/(24*time.Hour)) + 1
}
