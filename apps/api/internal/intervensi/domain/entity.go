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

var (
	// ErrNotFound: kartu tidak ada.
	ErrNotFound = errors.New("kartu intervensi tidak ditemukan")
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
}

func (Intervensi) TableName() string { return "intervensi" }

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
