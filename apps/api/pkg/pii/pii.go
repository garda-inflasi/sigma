// Package pii memuat pembantu untuk data pribadi (UU PDP): penyamaran nilai yang tampil di respons untuk pihak
// lain dan di jejak audit.
package pii

import "strings"

// TeleponTersamar menyamarkan nomor telepon: tiga karakter pertama (kode negara "+62") dan tiga karakter terakhir
// dipertahankan, sisanya menjadi "*". Contoh: "+6281234567890" menjadi "+62********890".
//
// Nomor kosong tetap kosong. Nilai yang terlalu pendek untuk disamarkan sebagian (6 karakter atau kurang)
// disamarkan seluruhnya, supaya penyamaran tidak pernah membuka lebih banyak dari nomornya sendiri.
func TeleponTersamar(telepon string) string {
	huruf := []rune(telepon)
	n := len(huruf)
	if n == 0 {
		return ""
	}
	const awal, akhir = 3, 3
	if n <= awal+akhir {
		return strings.Repeat("*", n)
	}
	return string(huruf[:awal]) + strings.Repeat("*", n-awal-akhir) + string(huruf[n-akhir:])
}
