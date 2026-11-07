package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/garda-inflasi/api/pkg/idx"
)

// RisikoTinggiAmbang adalah batas risiko food loss (pecahan 0–1). Lot dengan
// risiko di atas nilai ini dihitung "berisiko tinggi" pada ringkasan stok.
const RisikoTinggiAmbang = 0.6

// FasilitasColdStorage maps to the cold_storage table. ID adalah UUID; Kode adalah kode tampilan (CS-01) dan
// alias di URL, unik, dan boleh kosong untuk fasilitas yang dibuat tanpa kode.
type FasilitasColdStorage struct {
	ID            string   `gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()"`
	Kode          *string  `gorm:"column:kode"`
	Nama          string   `gorm:"column:nama"`
	KabupatenKode string   `gorm:"column:kabupaten_kode"`
	Lat           float64  `gorm:"column:lat"`
	Lng           float64  `gorm:"column:lng"`
	KapasitasTon  int      `gorm:"column:kapasitas_ton"`
	SuhuTargetC   float64  `gorm:"column:suhu_target_c"`
	SuhuAktualC   *float64 `gorm:"column:suhu_aktual_c"`
	Status        string   `gorm:"column:status"`

	// Batas alarm; nil berarti suhu_target_c ± ToleransiSuhuC.
	SuhuMinC  *float64 `gorm:"column:suhu_min_c"`
	SuhuMaksC *float64 `gorm:"column:suhu_maks_c"`
	// Turunan dari telemetri terbaru.
	KelembapanAktualPct *float64   `gorm:"column:kelembapan_aktual_pct"`
	TelemetriPada       *time.Time `gorm:"column:telemetri_pada"`
}

func (FasilitasColdStorage) TableName() string {
	return "cold_storage"
}

// StokColdStorage maps to the stok_cold_storage table. ID adalah BIGINT identity yang dibuat basis data; Kode
// adalah kode tampilan lot (STK-000042, atau ID lama pada data migrasi).
type StokColdStorage struct {
	ID              idx.ID    `gorm:"column:id;primaryKey;autoIncrement"`
	Kode            *string   `gorm:"column:kode"`
	ColdStorageID   string    `gorm:"column:cold_storage_id;type:uuid"`
	KomoditasKode   string    `gorm:"column:komoditas_kode"`
	VolumeTon       float64   `gorm:"column:volume_ton"`
	MasukPada       time.Time `gorm:"column:masuk_pada"`
	KedaluwarsaPada time.Time `gorm:"column:kedaluwarsa_pada"`
	Pemilik         string    `gorm:"column:pemilik"`
	RisikoFoodLoss  float64   `gorm:"column:risiko_food_loss"`
	// Status adalah salah satu StatusLot*; bawaan AKTIF.
	Status string `gorm:"column:status;default:AKTIF"`
}

func (StokColdStorage) TableName() string {
	return "stok_cold_storage"
}

// FasilitasView adalah fasilitas beserta turunan yang dihitung saat dibaca:
// nama wilayah (join kabupaten) dan volume terisi (jumlah stok fasilitas).
type FasilitasView struct {
	ID            string   `gorm:"column:id"`
	Kode          *string  `gorm:"column:kode"`
	Nama          string   `gorm:"column:nama"`
	KabupatenKode string   `gorm:"column:kabupaten_kode"`
	KabupatenNama string   `gorm:"column:kabupaten_nama"`
	Lat           float64  `gorm:"column:lat"`
	Lng           float64  `gorm:"column:lng"`
	KapasitasTon  int      `gorm:"column:kapasitas_ton"`
	TerisiTon     float64  `gorm:"column:terisi_ton"`
	SuhuTargetC   float64  `gorm:"column:suhu_target_c"`
	SuhuAktualC   *float64 `gorm:"column:suhu_aktual_c"`
	Status        string   `gorm:"column:status"`

	SuhuMinC            *float64   `gorm:"column:suhu_min_c"`
	SuhuMaksC           *float64   `gorm:"column:suhu_maks_c"`
	KelembapanAktualPct *float64   `gorm:"column:kelembapan_aktual_pct"`
	TelemetriPada       *time.Time `gorm:"column:telemetri_pada"`

	// Diisi usecase.
	UtilisasiPersen float64 `gorm:"-"`
	// BatasMinC/BatasMaksC adalah batas alarm efektif; Alarm benar bila suhu aktual di luarnya.
	BatasMinC  float64 `gorm:"-"`
	BatasMaksC float64 `gorm:"-"`
	Alarm      bool    `gorm:"-"`
}

// StokView adalah satu lot stok beserta nama fasilitas dan komoditasnya.
type StokView struct {
	ID              idx.ID    `gorm:"column:id"`
	Kode            *string   `gorm:"column:kode"`
	ColdStorageID   string    `gorm:"column:cold_storage_id"`
	ColdStorageNama string    `gorm:"column:cold_storage_nama"`
	KomoditasKode   string    `gorm:"column:komoditas_kode"`
	KomoditasNama   string    `gorm:"column:komoditas_nama"`
	VolumeTon       float64   `gorm:"column:volume_ton"`
	MasukPada       time.Time `gorm:"column:masuk_pada"`
	KedaluwarsaPada time.Time `gorm:"column:kedaluwarsa_pada"`
	RisikoFoodLoss  float64   `gorm:"column:risiko_food_loss"`
	Pemilik         string    `gorm:"column:pemilik"`
	Status          string    `gorm:"column:status"`

	// Diisi usecase.
	SisaMasaSimpanHari int `gorm:"-"`
}

// StokSort adalah kolom pengurutan yang diizinkan untuk daftar stok.
type StokSort string

const (
	SortSisaMasaSimpan StokSort = "sisa_masa_simpan"
	SortKomoditas      StokSort = "komoditas"
	SortFasilitas      StokSort = "fasilitas"
	SortVolume         StokSort = "volume"
	SortMasuk          StokSort = "masuk"
	SortRisiko         StokSort = "risiko"
	SortPemilik        StokSort = "pemilik"
)

// StokQuery adalah kriteria daftar stok: filter, urutan, dan halaman.
type StokQuery struct {
	// Fasilitas menyaring stok pada satu fasilitas (UUID atau kode); kosong berarti semua.
	Fasilitas idx.RujukanUUID
	Search    string
	Sort      StokSort
	Desc      bool
	Page      int
	Limit     int
}

// Offset menghitung posisi baris pertama halaman.
func (q StokQuery) Offset() int {
	return (q.Page - 1) * q.Limit
}

// StokPage adalah satu halaman stok beserta agregat seluruh hasil filter.
type StokPage struct {
	Items          []StokView
	Total          int64
	TotalVolumeTon float64
}

// RingkasanStok adalah agregat seluruh stok, tidak terpengaruh filter tabel.
type RingkasanStok struct {
	TotalLot        int64
	TotalVolumeTon  float64
	RisikoTinggiLot int64
	RisikoTinggiTon float64
}

// BulkAction represents a bulk action on cold storage.
type BulkAction string

const (
	BulkRencanakanDistribusi BulkAction = "rencanakan_distribusi"
	BulkRekomendasiOlahan    BulkAction = "rekomendasi_olahan"
	// Dua aksi berikut mengubah stok (13-B) dan ditangani modul operasi.
	BulkTandaiPrioritasJual BulkAction = "tandai_prioritas_jual"
	BulkBuang               BulkAction = "buang"
)

// BulkResult adalah hasil aksi massal: lot yang ditemukan dan volumenya.
type BulkResult struct {
	Berhasil  int64
	VolumeTon float64
	// IDDitemukan adalah id lot yang diproses; id lain pada permintaan dianggap gagal.
	IDDitemukan []idx.ID
}

// RujukanFasilitas membaca teks bebas (isi body atau query) sebagai UUID fasilitas atau, bila bukan UUID,
// kode fasilitas (alias CS-01). Teks kosong menghasilkan rujukan kosong.
func RujukanFasilitas(teks string) idx.RujukanUUID {
	teks = strings.TrimSpace(teks)
	if id, ok := idx.NormalUUID(teks); ok {
		return idx.RujukanUUID{ID: id}
	}
	return idx.RujukanUUID{Kode: teks}
}

// KosongRujukan melaporkan rujukan yang tidak menunjuk apa pun.
func KosongRujukan(r idx.RujukanUUID) bool { return r.ID == "" && r.Kode == "" }

// KodeLot adalah kode tampilan lot baru. Diturunkan dari ID identity yang unik, sehingga dua lot tidak pernah
// mendapat kode yang sama; kode lama (STK- dan 12 karakter heksadesimal, atau kode seed) berbeda bentuk.
func KodeLot(id idx.ID) string { return fmt.Sprintf("STK-%06d", int64(id)) }
