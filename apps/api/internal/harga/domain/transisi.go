package domain

import (
	"slices"
	"time"
)

// Sebab sebuah entri dilewati pada aksi massal.
const (
	SebabTidakDitemukan = "tidak_ditemukan"
	SebabStatus         = "status_tidak_sesuai"
	SebabPemisahanTugas = "pemisahan_tugas"
	SebabBukanPemilik   = "bukan_pemilik"
)

// aturan adalah satu transisi sah: status asal, status tujuan, izin yang diperlukan,
// dan syarat pelaku terhadap petugas pencatat.
type aturan struct {
	dari []string
	ke   string
	izin string
	// pisahTugas melarang pelaku sama dengan petugas pencatat (validator/penerbit bukan pencatat).
	pisahTugas bool
	// hanyaPemilik mensyaratkan pelaku adalah petugas pencatat.
	hanyaPemilik bool
}

var aturanTransisi = map[BulkAction]aturan{
	BulkAjukan:     {dari: []string{StatusDraf}, ke: StatusDiajukan, izin: "harga.input", hanyaPemilik: true},
	BulkValidasi:   {dari: []string{StatusDiajukan}, ke: StatusDivalidasi, izin: "harga.validasi", pisahTugas: true},
	BulkPublikasi:  {dari: []string{StatusDivalidasi}, ke: StatusDipublikasi, izin: "harga.publikasi", pisahTugas: true},
	BulkTolak:      {dari: []string{StatusDiajukan}, ke: StatusDitolak, izin: "harga.validasi"},
	BulkKembalikan: {dari: []string{StatusDiajukan, StatusDivalidasi}, ke: StatusDraf, izin: "harga.validasi"},
}

// Izin mengembalikan izin yang diperlukan aksi, atau string kosong bila aksi tidak dikenal.
func (a BulkAction) Izin() string {
	return aturanTransisi[a].izin
}

// StatusTujuan mengembalikan status hasil aksi, atau string kosong bila aksi tidak dikenal.
func (a BulkAction) StatusTujuan() string {
	return aturanTransisi[a].ke
}

// AlasanWajib melaporkan apakah aksi mensyaratkan alasan tertulis.
func (a BulkAction) AlasanWajib() bool {
	return a == BulkTolak
}

// Evaluasi memeriksa apakah aktor boleh menjalankan aksi pada entri berstatus status
// yang dicatat petugasID. Mengembalikan string kosong bila boleh, selain itu salah satu Sebab*.
// Izin aktor diperiksa terpisah (per aksi) oleh lapisan delivery.
func Evaluasi(aksi BulkAction, status string, petugasID *string, aktorID string) string {
	a, ada := aturanTransisi[aksi]
	if !ada || !slices.Contains(a.dari, status) {
		return SebabStatus
	}
	pencatat := petugasID != nil && *petugasID == aktorID
	switch {
	case a.hanyaPemilik && !pencatat:
		return SebabBukanPemilik
	case a.pisahTugas && pencatat:
		return SebabPemisahanTugas
	}
	return ""
}

// RiwayatStatusEntri memetakan tabel riwayat_status_entri: satu perpindahan status.
type RiwayatStatusEntri struct {
	ID      int64     `gorm:"column:id;primaryKey;autoIncrement"`
	EntriID string    `gorm:"column:entri_id"`
	Dari    *string   `gorm:"column:dari"`
	Ke      string    `gorm:"column:ke"`
	Oleh    *string   `gorm:"column:oleh"`
	Waktu   time.Time `gorm:"column:waktu;default:now()"`
	Alasan  *string   `gorm:"column:alasan"`
}

func (RiwayatStatusEntri) TableName() string {
	return "riwayat_status_entri"
}

// RiwayatView adalah satu perpindahan status beserta nama pelakunya.
type RiwayatView struct {
	Dari     *string   `gorm:"column:dari"`
	Ke       string    `gorm:"column:ke"`
	Oleh     *string   `gorm:"column:oleh"`
	OlehNama *string   `gorm:"column:oleh_nama"`
	Waktu    time.Time `gorm:"column:waktu"`
	Alasan   *string   `gorm:"column:alasan"`
}

// HasilTransisi adalah nasib satu id pada aksi massal; Sebab terisi bila dilewati.
type HasilTransisi struct {
	ID    string
	Sebab string
}

// Diterapkan melaporkan apakah entri berpindah status.
func (h HasilTransisi) Diterapkan() bool {
	return h.Sebab == ""
}
