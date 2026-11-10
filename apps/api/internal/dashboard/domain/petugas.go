package domain

import "time"

// AktivitasPetugasQuery adalah permintaan aktivitas per petugas. Semua batas waktu dihitung usecase dari hari ini
// menurut waktu Aceh.
type AktivitasPetugasQuery struct {
	// Cakupan membatasi petugas menurut kabupaten tugasnya (02-B); nilai nol tertutup: tidak ada baris.
	Cakupan Cakupan
	// KabupatenKode adalah filter permintaan, digabung (AND) dengan Cakupan. Kosong: tanpa filter.
	KabupatenKode string

	// AwalHari dan AkhirHari membatasi "hari ini" waktu Aceh: [AwalHari, AkhirHari).
	AwalHari, AkhirHari time.Time
	// AwalJendela adalah awal jendela JendelaOperasionalHari hari yang berakhir hari ini (tengah malam waktu Aceh).
	AwalJendela time.Time
	// Sekarang adalah saat perhitungan: tugas yang tenggatnya sebelum ini dan belum selesai berstatus terlambat.
	Sekarang time.Time
}

// AktivitasPetugas adalah ringkasan aktivitas satu petugas lapangan (mahasiswa agen aktif). Hanya nama tampilan dan
// ID publik (UUID pengguna) yang dibawa; email, telepon, dan data pribadi lain tidak.
//
//   - EntriHariIni: entri harga KONSUMEN (selain DRAF) yang dicatat petugas hari ini waktu Aceh, tanpa memandang
//     pasarnya. TargetHarian sama dengan aktivitas-lapangan: tiap komoditas aktif sekali sehari.
//   - MenungguValidasi: entri petugas yang saat ini berstatus DIAJUKAN (antrean validasi, seusia apa pun).
//   - DitolakJendela: entri petugas yang dicatat dalam jendela JendelaOperasionalHari hari dan berstatus DITOLAK.
//     DITOLAK final (tidak ada transisi keluar), jadi hitungan seumur hidup hanya menumpuk; jendela menjaganya
//     bermakna.
//   - EntriTerakhirPada: waktu entri terakhir (selain DRAF, semua tipe dan umur); nil bila belum pernah mencatat.
//   - Tugas*: tugas lapangan dengan tenggat dalam jendela atau di masa depan, dikelompokkan menurut status efektif
//     (lihat lapangan/domain.TugasView.StatusEfektif): aktif = BELUM atau BERJALAN, terlambat = belum selesai dan
//     lewat tenggat, selesai = SELESAI.
type AktivitasPetugas struct {
	ID            string `gorm:"column:id"`
	Nama          string `gorm:"column:nama"`
	KabupatenKode string `gorm:"column:kabupaten_kode"`
	KabupatenNama string `gorm:"column:kabupaten_nama"`

	EntriHariIni      int        `gorm:"column:entri_hari_ini"`
	TargetHarian      int        `gorm:"column:target_harian"`
	MenungguValidasi  int        `gorm:"column:menunggu_validasi"`
	DitolakJendela    int        `gorm:"column:ditolak_jendela"`
	EntriTerakhirPada *time.Time `gorm:"column:entri_terakhir_pada"`

	TugasAktif     int `gorm:"column:tugas_aktif"`
	TugasSelesai   int `gorm:"column:tugas_selesai"`
	TugasTerlambat int `gorm:"column:tugas_terlambat"`
}

// AktivitasPetugasHasil adalah hasil GET /aktivitas-petugas.
type AktivitasPetugasHasil struct {
	// Tanggal adalah hari ini menurut waktu Aceh (DATE).
	Tanggal time.Time
	// Petugas terurut menurut nama kabupaten, nama petugas, lalu ID; tidak pernah nil.
	Petugas []AktivitasPetugas
}
