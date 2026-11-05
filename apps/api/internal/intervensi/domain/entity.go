// Package domain berisi model dan kontrak repository modul intervensi.
package domain

import (
	"errors"
	"time"
)

// Siklus ekonomi (enum siklus). Kartu intervensi selalu milik satu siklus.
const (
	SiklusNormal  = "NORMAL"
	SiklusTekanan = "TEKANAN"
	SiklusSurplus = "SURPLUS"
)

// IzinBawaan dibutuhkan untuk memindahkan kartu ke tahap yang tak menentukan izin lain.
const IzinBawaan = "intervensi.kelola"

// Aksi pada riwayat kartu (CHECK intervensi_riwayat.aksi).
const (
	AksiBuat        = "BUAT"
	AksiPindahTahap = "PINDAH_TAHAP"
	AksiTugaskan    = "TUGASKAN"
	AksiEskalasi    = "ESKALASI"
)

var (
	// ErrNotFound: kartu tidak ada.
	ErrNotFound = errors.New("kartu intervensi tidak ditemukan")
	// ErrVersiUsang: kartu sudah diubah pengguna lain sejak dibaca klien.
	ErrVersiUsang = errors.New("kartu telah diubah pengguna lain")
	// ErrInvalidReference: komoditas, kabupaten, alert, atau penanggung jawab tidak ada.
	ErrInvalidReference = errors.New("rujukan tidak ditemukan")
)

// Intervensi memetakan tabel intervensi (satu kartu kanban).
type Intervensi struct {
	ID               string     `gorm:"column:id;primaryKey"`
	Siklus           string     `gorm:"column:siklus"`
	Tahap            string     `gorm:"column:tahap"`
	Urutan           int        `gorm:"column:urutan"`
	Judul            string     `gorm:"column:judul"`
	Deskripsi        string     `gorm:"column:deskripsi"`
	KomoditasKode    string     `gorm:"column:komoditas_kode"`
	KabupatenKode    string     `gorm:"column:kabupaten_kode"`
	Prioritas        string     `gorm:"column:prioritas"`
	Tenggat          time.Time  `gorm:"column:tenggat"`
	RekomendasiAI    bool       `gorm:"column:rekomendasi_ai"`
	ChecklistSelesai int        `gorm:"column:checklist_selesai"`
	ChecklistTotal   int        `gorm:"column:checklist_total"`
	EwsAlertID       *string    `gorm:"column:ews_alert_id"`
	DibuatOlehID     string     `gorm:"column:dibuat_oleh_id"`
	DibuatPada       time.Time  `gorm:"column:dibuat_pada;default:now()"`
	SelesaiPada      *time.Time `gorm:"column:selesai_pada"`
	// Versi naik tiap perubahan kartu; dipakai untuk optimistic locking.
	Versi            int        `gorm:"column:versi;default:1"`
	DieskalasiPada   *time.Time `gorm:"column:dieskalasi_pada"`
	DieskalasiOlehID *string    `gorm:"column:dieskalasi_oleh_id"`
}

func (Intervensi) TableName() string { return "intervensi" }

// IntervensiRiwayat memetakan tabel intervensi_riwayat: satu peristiwa pada kartu.
// Oleh kosong berarti sistem.
type IntervensiRiwayat struct {
	ID           int64     `gorm:"column:id;primaryKey;->"`
	IntervensiID string    `gorm:"column:intervensi_id"`
	Aksi         string    `gorm:"column:aksi"`
	Dari         *string   `gorm:"column:dari"`
	Ke           *string   `gorm:"column:ke"`
	Oleh         *string   `gorm:"column:oleh"`
	Waktu        time.Time `gorm:"column:waktu;default:now()"`
	Alasan       string    `gorm:"column:alasan"`
}

func (IntervensiRiwayat) TableName() string { return "intervensi_riwayat" }

// Riwayat adalah satu peristiwa kartu beserta nama pelakunya.
type Riwayat struct {
	IntervensiRiwayat
	OlehNama *string `gorm:"column:oleh_nama;->"`
}

// BulkAksi adalah aksi massal pada kartu.
type BulkAksi string

const (
	BulkTugaskan BulkAksi = "tugaskan"
	BulkEskalasi BulkAksi = "eskalasi"
)

// HasilKartu adalah hasil satu aksi massal pada satu kartu.
type HasilKartu struct {
	ID       string
	Berhasil bool
	// Alasan menjelaskan kegagalan; kosong bila berhasil.
	Alasan string
}

// IntervensiPJ memetakan tabel intervensi_pj (penanggung jawab, relasi M:N).
type IntervensiPJ struct {
	IntervensiID string `gorm:"column:intervensi_id;primaryKey"`
	PenggunaID   string `gorm:"column:pengguna_id;primaryKey"`
}

func (IntervensiPJ) TableName() string { return "intervensi_pj" }

// TahapDef memetakan tabel tahap_intervensi_def: satu tahap pada satu siklus.
type TahapDef struct {
	Siklus         string `gorm:"column:siklus;primaryKey"`
	KodeTahap      string `gorm:"column:kode_tahap;primaryKey"`
	Urutan         int    `gorm:"column:urutan"`
	Label          string `gorm:"column:label"`
	Petunjuk       string `gorm:"column:petunjuk"`
	IzinDibutuhkan string `gorm:"column:izin_dibutuhkan"`
	SLAHari        *int   `gorm:"column:sla_hari"`
}

func (TahapDef) TableName() string { return "tahap_intervensi_def" }

// PenanggungJawab adalah pengguna yang ditugasi pada sebuah kartu.
type PenanggungJawab struct {
	ID   string
	Nama string
}

// Kartu adalah kartu beserta penanggung jawabnya.
type Kartu struct {
	Intervensi
	PenanggungJawab []PenanggungJawab
}

// ListQuery menyaring daftar kartu; filter kosong berarti semua.
type ListQuery struct {
	Siklus, Tahap, Prioritas string
}

// NewIntervensi adalah masukan pembuatan kartu. Tahap awal dan urutan ditentukan sistem.
type NewIntervensi struct {
	Siklus, Judul, Deskripsi, KomoditasKode, KabupatenKode, Prioritas string
	Tenggat                                                           time.Time
	EwsAlertID                                                        *string
	PenanggungJawabIDs                                                []string
	DibuatOlehID                                                      string
}
