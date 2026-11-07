package domain

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/garda-inflasi/api/pkg/idx"
)

// Hasil verifikasi rantai hash.
const (
	StatusValid = "VALID"
	StatusRusak = "RUSAK"
	// StatusGagal: verifikasinya sendiri gagal (mis. basis data tak terjangkau); keadaan rantai tidak diketahui.
	StatusGagal = "GAGAL"
)

// Pemicu verifikasi.
const (
	PemicuJadwal = "JADWAL"
	PemicuManual = "MANUAL"
)

// Verifikasi memetakan tabel audit_log_verifikasi (append-only): hasil satu verifikasi rantai.
type Verifikasi struct {
	ID            idx.ID    `gorm:"column:id;primaryKey;autoIncrement"`
	Mulai         time.Time `gorm:"column:mulai"`
	Selesai       time.Time `gorm:"column:selesai"`
	Pemicu        string    `gorm:"column:pemicu"`
	Status        string    `gorm:"column:status"`
	JumlahBaris   int64     `gorm:"column:jumlah_baris"`
	JumlahSegmen  int64     `gorm:"column:jumlah_segmen"`
	JumlahMasalah int64     `gorm:"column:jumlah_masalah"`
	IDAwal        *int64    `gorm:"column:id_awal"`
	IDAkhir       *int64    `gorm:"column:id_akhir"`
	HashAkhir     []byte    `gorm:"column:hash_akhir;type:bytea"`
	// Masalah adalah JSON daftar Masalah (paling banyak MaksMasalahDicatat).
	Masalah    *string `gorm:"column:masalah;type:jsonb"`
	Keterangan *string `gorm:"column:keterangan"`
}

func (Verifikasi) TableName() string { return "audit_log_verifikasi" }

// Arsip memetakan tabel audit_log_arsip (append-only): satu partisi yang sudah dilepas ke berkas arsip.
type Arsip struct {
	ID             idx.ID    `gorm:"column:id;primaryKey;autoIncrement"`
	Partisi        string    `gorm:"column:partisi"`
	Dari           time.Time `gorm:"column:dari"`
	Sampai         time.Time `gorm:"column:sampai"`
	JumlahBaris    int64     `gorm:"column:jumlah_baris"`
	IDPertama      *int64    `gorm:"column:id_pertama"`
	IDTerakhir     *int64    `gorm:"column:id_terakhir"`
	Ringkasan      []byte    `gorm:"column:ringkasan;type:bytea"`
	Berkas         string    `gorm:"column:berkas"`
	SHA256Berkas   string    `gorm:"column:sha256_berkas"`
	RetensiBulan   int       `gorm:"column:retensi_bulan"`
	DiarsipkanPada time.Time `gorm:"column:diarsipkan_pada"`
	DiarsipkanOleh string    `gorm:"column:diarsipkan_oleh"`
}

func (Arsip) TableName() string { return "audit_log_arsip" }

// SegmenArsipRow memetakan tabel audit_log_arsip_segmen.
type SegmenArsipRow struct {
	ID          idx.ID `gorm:"column:id;primaryKey;autoIncrement"`
	ArsipID     int64  `gorm:"column:arsip_id"`
	IDAwal      int64  `gorm:"column:id_awal"`
	IDAkhir     int64  `gorm:"column:id_akhir"`
	JumlahBaris int64  `gorm:"column:jumlah_baris"`
	HashMasuk   []byte `gorm:"column:hash_masuk;type:bytea"`
	HashKeluar  []byte `gorm:"column:hash_keluar;type:bytea"`
}

func (SegmenArsipRow) TableName() string { return "audit_log_arsip_segmen" }

// Partisi adalah satu partisi bulanan audit_log yang masih aktif.
type Partisi struct {
	Nama string
	// Dari (inklusif) dan Sampai (eksklusif) dibaca dari nama audit_log_pYYYYMM; nil bila nama tidak berpola itu.
	Dari, Sampai *time.Time
	// PerkiraanBaris adalah perkiraan planner (pg_class.reltuples); nil bila tabel belum pernah dianalisis.
	PerkiraanBaris *int64
}

var polaPartisi = regexp.MustCompile(`^audit_log_p(\d{4})(\d{2})$`)

// RentangPartisi membaca rentang bulan (UTC) dari nama partisi audit_log_pYYYYMM.
func RentangPartisi(nama string) (dari, sampai time.Time, ok bool) {
	m := polaPartisi.FindStringSubmatch(nama)
	if m == nil {
		return time.Time{}, time.Time{}, false
	}
	t, err := time.Parse("200601", m[1]+m[2])
	if err != nil {
		return time.Time{}, time.Time{}, false
	}
	return t, t.AddDate(0, 1, 0), true
}

// NamaPartisi adalah nama partisi bulanan yang memuat waktu t (bulan UTC).
func NamaPartisi(t time.Time) string {
	return fmt.Sprintf("audit_log_p%s", t.UTC().Format("200601"))
}

// LepasPartisi adalah bukti ekspor yang diserahkan ke audit_log_lepas_partisi.
type LepasPartisi struct {
	Partisi      string
	RetensiBulan int
	JumlahBaris  int64
	// Ringkasan adalah SHA-256 atas gabungan hash seluruh baris partisi, urut id.
	Ringkasan    []byte
	Berkas       string
	SHA256Berkas string
}

// RepositoryIntegritas membaca dan memelihara rantai hash, partisi, dan arsip audit_log.
type RepositoryIntegritas interface {
	// PindaiRantai membuka satu snapshot konsisten (REPEATABLE READ, hanya baca), menyerahkan seluruh segmen arsip
	// (urut id_awal) ke awal, lalu seluruh baris audit urut id naik ke periksa.
	PindaiRantai(ctx context.Context, awal func([]SegmenArsip) error, periksa func(AuditLog) error) error
	SimpanVerifikasi(ctx context.Context, v *Verifikasi) error
	// VerifikasiTerakhir mengembalikan nil bila belum ada verifikasi.
	VerifikasiTerakhir(ctx context.Context) (*Verifikasi, error)
	// DaftarPartisi mengembalikan partisi aktif, terbaru lebih dulu.
	DaftarPartisi(ctx context.Context) ([]Partisi, error)
	// DaftarArsip mengembalikan partisi yang sudah diarsipkan, terbaru lebih dulu.
	DaftarArsip(ctx context.Context) ([]Arsip, error)
	// SegmenArsip mengembalikan segmen satu arsip (arsipID > 0) atau semua segmen (arsipID 0), urut id_awal.
	SegmenArsip(ctx context.Context, arsipID int64) ([]SegmenArsip, error)
	// SiapkanPartisi membuat partisi bulanan yang belum ada mulai bulan dari, sebanyak jumlahBulan.
	SiapkanPartisi(ctx context.Context, dari time.Time, jumlahBulan int) ([]string, error)
	// BarisRentang menyerahkan baris dengan dari <= waktu < sampai urut id naik (satu partisi bulanan).
	BarisRentang(ctx context.Context, dari, sampai time.Time, fn func(AuditLog) error) error
	// LepasPartisi mencatat arsip dan melepas partisi (lihat audit_log_lepas_partisi); mengembalikan id arsip.
	LepasPartisi(ctx context.Context, in LepasPartisi) (int64, error)
	// DenganKunciPemeliharaan menjalankan fn sambil memegang kunci yang dipakai bersama verifikasi dan retensi,
	// sehingga keduanya tidak pernah berjalan bersamaan (juga antar-instance worker).
	DenganKunciPemeliharaan(ctx context.Context, fn func(context.Context) error) error
}
