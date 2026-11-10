package domain

import (
	"errors"

	harga "github.com/garda-inflasi/api/internal/harga/domain"
)

// ErrPenggunaTidakAda dikembalikan bila akun pemanggil tidak ada di basis data.
var ErrPenggunaTidakAda = errors.New("pengguna tidak ditemukan")

// PenggunaCakupan adalah peran dan kabupaten tugas pengguna yang menentukan Cakupan-nya.
type PenggunaCakupan struct {
	Peran string
	// KabupatenKode kosong untuk pengguna tingkat provinsi.
	KabupatenKode string
}

// Cakupan menentukan agregat dashboard yang boleh dilihat pemanggil (row scope 02-B): seluruh Aceh atau satu
// kabupaten/kota. Dashboard hanya memuat agregat, yang tidak punya "pemilik", jadi tidak ada cakupan "milik sendiri"
// seperti di modul harga: Koordinator dan Agregator melihat kabupaten tugasnya saja, peran tingkat provinsi ke atas
// melihat seluruh Aceh. Semua endpoint /v1/dashboard/operasional/* memakai cakupan yang sama.
//
// Nilai nol tertutup (tidak ada data yang lolos), jadi cakupan yang lupa diisi tidak membuka seluruh Aceh; membuka
// semua data harus eksplisit lewat Semua. Repository menerjemahkan nilai nol menjadi kondisi FALSE.
type Cakupan struct {
	// Semua membuka data seluruh Aceh.
	Semua bool
	// KabupatenKode (kode BPS) membatasi data pada satu kabupaten/kota. Diabaikan bila Semua.
	KabupatenKode string
}

// CakupanSemua membuka data seluruh Aceh.
func CakupanSemua() Cakupan { return Cakupan{Semua: true} }

// CakupanKabupaten membatasi data pada satu kabupaten/kota; kode kosong menghasilkan cakupan tertutup.
func CakupanKabupaten(kode string) Cakupan { return Cakupan{KabupatenKode: kode} }

// CakupanPeran menurunkan cakupan dashboard dari peran dan kabupaten tugas pengguna. Klasifikasi peran SENGAJA
// tidak diulang di sini: sumber tunggalnya harga/domain.CakupanPeran (02-B), supaya aturan peran tidak bercabang
// antarmodul. Dari cakupan harga hanya batas wilayahnya yang dipakai (Semua dan kabupaten); kepemilikan entri
// diabaikan karena agregat tidak berpemilik. Akibatnya:
//   - Koordinator dan Agregator: kabupaten tugasnya; tanpa kabupaten tugas tertutup, bukan seluruh Aceh.
//   - Peran tingkat provinsi ke atas (strategis, manajerial, admin sistem): seluruh Aceh.
//   - Mahasiswa dan peran yang belum dikenal: tertutup.
func CakupanPeran(peran, kabupatenKode, penggunaID string) Cakupan {
	c := harga.CakupanPeran(peran, kabupatenKode, penggunaID)
	return Cakupan{Semua: c.Semua, KabupatenKode: c.KabupatenKode}
}

// Tertutup melaporkan apakah cakupan tidak membuka data apa pun.
func (c Cakupan) Tertutup() bool { return !c.Semua && c.KabupatenKode == "" }

// Irisi menggabungkan cakupan dengan filter kabupaten dari permintaan (kosong: tanpa filter) dan mengembalikan
// kabupaten efektif: kosong berarti seluruh Aceh (hanya mungkin bila Semua dan tanpa filter). ada=false berarti
// irisannya kosong: cakupan tertutup, atau filter menunjuk kabupaten di luar cakupan. Hasilnya harus kosong, bukan
// galat dan bukan seluruh Aceh (konsisten dengan modul harga, yang menyembunyikan baris di luar cakupan).
func (c Cakupan) Irisi(filterKabupaten string) (kabupaten string, ada bool) {
	switch {
	case c.Semua:
		return filterKabupaten, true
	case c.KabupatenKode == "":
		return "", false
	case filterKabupaten == "" || filterKabupaten == c.KabupatenKode:
		return c.KabupatenKode, true
	}
	return "", false
}
