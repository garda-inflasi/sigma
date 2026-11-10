package domain

import (
	coldstorage "github.com/garda-inflasi/api/internal/cold-storage/domain"
	eksekutif "github.com/garda-inflasi/api/internal/eksekutif/domain"
	harga "github.com/garda-inflasi/api/internal/harga/domain"
	lapangan "github.com/garda-inflasi/api/internal/lapangan/domain"
	spasial "github.com/garda-inflasi/api/internal/spasial/domain"
)

// Ringkasan operasional (08-B): aktivitas per petugas, stok, produsen, dan logistik. Definisi tiap angka SENGAJA
// meminjam konstanta modul pemiliknya (cold-storage, lapangan, spasial, harga, eksekutif), bukan menyalinnya: dua
// definisi "lot", "tugas terlambat", atau "rute aktif" yang bergeser satu sama lain membuat angka dashboard
// bertentangan dengan halaman modul asalnya.
const (
	// JendelaOperasionalHari adalah panjang jendela "terakhir": sekian hari kalender waktu Aceh yang berakhir hari ini
	// (hari ini termasuk). Dipakai untuk penolakan entri dan tugas per petugas, harga produsen, dan pergerakan logistik.
	// Sama dengan jendela tingkat validasi KPI (JendelaValidasiHari) supaya "pekan terakhir" berarti hal yang sama di
	// seluruh dashboard.
	JendelaOperasionalHari = 7

	// AmbangMendekatiKedaluwarsaHari: lot yang berlaku dan tanggal kedaluwarsanya (waktu Aceh) jatuh paling lambat
	// sekian hari setelah hari ini dihitung "mendekati kedaluwarsa". Lot hortikultura bermasa simpan 14-21 hari, jadi
	// sepekan cukup untuk menjualnya atau memindahkannya.
	AmbangMendekatiKedaluwarsaHari = 7

	// AmbangRisikoTinggi adalah batas risiko food loss (pecahan 0-1): lot dengan risiko di ATAS nilai ini berisiko
	// tinggi. Sama dengan ringkasan halaman cold storage.
	AmbangRisikoTinggi = coldstorage.RisikoTinggiAmbang

	// MaksPergerakanTerbaru dan MaksRuteDitampilkan membatasi daftar pergerakan stok terbaru dan rute distribusi
	// aktif pada /logistik. Jumlah dan total rute tetap dihitung atas semua rute aktif dalam cakupan.
	MaksPergerakanTerbaru = 10
	MaksRuteDitampilkan   = 10

	// WilayahProvinsi adalah kode wilayah neraca pangan tingkat provinsi (dipakai untuk tampilan seluruh Aceh).
	WilayahProvinsi = eksekutif.WilayahProvinsi
)

// Peran petugas lapangan yang punya target pencatatan harian. Koordinator dan Agregator tidak dihitung: target
// aktivitas-lapangan (jumlah petugas x komoditas aktif) juga hanya menghitung mahasiswa, jadi daftar per petugas
// dan angka per kabupaten berasal dari himpunan petugas yang sama.
const PeranPetugas = harga.PeranMahasiswa

// Status yang dipinjam dari modul pemiliknya (satu sumber kebenaran).
const (
	// StatusLotHabis: lot habis tidak ada di gudang; semua lot lain (AKTIF, PRIORITAS_JUAL) ada.
	StatusLotHabis = coldstorage.StatusLotHabis
	// StatusFasilitasOperasional: hanya fasilitas ini yang kapasitas dan stoknya dihitung (sama dengan KPI cs).
	StatusFasilitasOperasional = coldstorage.StatusFasilitasOperasional
	// StatusRuteDigantikan: rute aktif adalah semua rute selain yang digantikan rencana optimasi baru.
	StatusRuteDigantikan = spasial.StatusRuteDigantikan
	// SumberNeracaDemo menandai neraca pangan contoh; angka resmi (BPS, BI, DJPB, BAPANAS) lebih diutamakan.
	SumberNeracaDemo = eksekutif.SumberDemo
)

// Jenis transaksi stok.
const (
	JenisMasuk  = coldstorage.TransaksiMasuk
	JenisKeluar = coldstorage.TransaksiKeluar
	JenisPindah = coldstorage.TransaksiPindah
)

// Tipe harga entri.
const (
	TipeProdusen = harga.TipeProdusen
	TipeKonsumen = harga.TipeKonsumen
)

// Status entri harga.
const (
	EntriDraf     = harga.StatusDraf
	EntriDiajukan = harga.StatusDiajukan
	EntriDitolak  = harga.StatusDitolak
)

// Status tugas lapangan (status efektif; lihat lapangan/domain.TugasView.StatusEfektif).
const (
	TugasBelum     = lapangan.StatusBelum
	TugasBerjalan  = lapangan.StatusBerjalan
	TugasSelesai   = lapangan.StatusSelesai
	TugasTerlambat = lapangan.StatusTerlambat
)
