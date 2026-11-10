package usecase

import (
	"math"
	"strings"
	"time"
)

// saatIni membaca jam SEKALI dan mengembalikan saat itu beserta tanggal Aceh-nya (DATE, tengah malam UTC). Batas hari,
// kunci cache, dan status efektif tugas harus berasal dari satu pembacaan jam, juga di sekitar tengah malam.
func (uc *UseCase) saatIni() (sekarang, hari time.Time) {
	sekarang = uc.now()
	t := sekarang.In(wib)
	return sekarang, time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// awalHariWIB mengubah tanggal Aceh (DATE) menjadi tengah malam waktu Aceh pada tanggal itu.
func awalHariWIB(hari time.Time) time.Time {
	return time.Date(hari.Year(), hari.Month(), hari.Day(), 0, 0, 0, 0, wib)
}

// awalJendela adalah tanggal pertama jendela JendelaOperasionalHari hari yang berakhir pada hari.
func awalJendela(hari time.Time, panjang int) time.Time { return hari.AddDate(0, 0, -(panjang - 1)) }

// bulatkanRupiah membulatkan ke rupiah penuh; harga entri adalah bilangan bulat, jadi rata-ratanya pun tampil bulat.
func bulatkanRupiah(v float64) float64 { return math.Round(v) }

// bulatkanKg membulatkan tonase ke kilogram (tiga desimal), ketelitian penyimpanan stok. Dipakai untuk satu
// transaksi: pembulatan satu desimal membuat transaksi kecil tampil 0,0 ton.
func bulatkanKg(v float64) float64 {
	if r := math.Round(v*1000) / 1000; r != 0 {
		return r
	}
	return 0
}

// kurang membandingkan dua teks tanpa membedakan huruf besar-kecil, dengan teks asli sebagai pemutus seri; urutan
// nama yang sama pada semua mesin, tidak bergantung pada collation basis data.
func kurang(a, b string) bool {
	la, lb := strings.ToLower(a), strings.ToLower(b)
	if la != lb {
		return la < lb
	}
	return a < b
}
