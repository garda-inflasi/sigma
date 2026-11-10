package domain

// PermintaanSaringan adalah isian query ringkasan operasional (aktivitas petugas, stok, produsen, logistik) apa
// adanya dari pemanggil, belum diperiksa. Berbeda dengan PermintaanFilter, ringkasan ini tidak punya periode
// pilihan: jendela waktunya tetap relatif terhadap hari ini (JendelaOperasionalHari).
type PermintaanSaringan struct {
	// KabupatenKode (kode BPS 4 digit) menyaring satu kabupaten/kota; kosong: semua yang dalam cakupan.
	KabupatenKode string
	// KomoditasKode menyaring satu komoditas; kosong: semua. Diabaikan oleh aktivitas per petugas.
	KomoditasKode string
}

// Saringan adalah PermintaanSaringan yang sudah diperiksa.
type Saringan struct {
	KabupatenKode string
	KomoditasKode string
}

// NormalisasiSaringan memeriksa bentuk kabupaten_kode (4 digit) dan komoditas_kode (huruf besar, angka, tanda hubung).
// Kode yang tidak ada di master tidak ditolak: hasilnya kosong. Galat dibungkus ErrValidasi dengan pesan yang aman
// dikirim ke pemanggil (tidak memantulkan isiannya).
func NormalisasiSaringan(req PermintaanSaringan) (Saringan, error) {
	if err := periksaKode(req.KabupatenKode, req.KomoditasKode); err != nil {
		return Saringan{}, err
	}
	return Saringan(req), nil
}
