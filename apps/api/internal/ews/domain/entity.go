// Package domain berisi model dan kontrak repository modul EWS.
package domain

import (
	"database/sql/driver"
	"fmt"
	"strings"
	"time"
)

// Status siklus hidup alert (enum status_ews).
const (
	StatusBaru       = "BARU"
	StatusDiakui     = "DIAKUI"
	StatusDieskalasi = "DIESKALASI"
	StatusSelesai    = "SELESAI"
)

// EwsAlert memetakan tabel ews_alert.
type EwsAlert struct {
	ID            string     `gorm:"column:id;primaryKey"`
	Severity      string     `gorm:"column:severity"`
	Jenis         string     `gorm:"column:jenis"`
	Judul         string     `gorm:"column:judul"`
	Pesan         string     `gorm:"column:pesan"`
	KomoditasKode *string    `gorm:"column:komoditas_kode"`
	KabupatenKode string     `gorm:"column:kabupaten_kode"`
	Nilai         float64    `gorm:"column:nilai"`
	SatuanNilai   string     `gorm:"column:satuan_nilai"`
	SkorDampak    int        `gorm:"column:skor_dampak"`
	Status        string     `gorm:"column:status;default:BARU"`
	Sumber        string     `gorm:"column:sumber"`
	DibuatPada    time.Time  `gorm:"column:dibuat_pada;default:now()"`
	DiakuiOlehID  *string    `gorm:"column:diakui_oleh_id"`
	DiakuiPada    *time.Time `gorm:"column:diakui_pada"`
	AturanID      *string    `gorm:"column:aturan_id"`
	// DedupKey mengidentifikasi kondisi yang dipantau rule engine; hanya satu alert
	// aktif (bukan SELESAI) per kunci. Kosong untuk alert yang tidak berasal dari mesin.
	DedupKey         *string    `gorm:"column:dedup_key"`
	DieskalasiPada   *time.Time `gorm:"column:dieskalasi_pada"`
	DieskalasiOlehID *string    `gorm:"column:dieskalasi_oleh_id"`
	SelesaiPada      *time.Time `gorm:"column:selesai_pada"`
}

func (EwsAlert) TableName() string { return "ews_alert" }

// EwsRiwayat memetakan tabel ews_riwayat: satu perubahan status alert. Oleh kosong
// berarti sistem (rule engine atau eskalasi otomatis).
type EwsRiwayat struct {
	ID      string    `gorm:"column:id;primaryKey"`
	AlertID string    `gorm:"column:alert_id"`
	Dari    *string   `gorm:"column:dari"`
	Ke      string    `gorm:"column:ke"`
	Oleh    *string   `gorm:"column:oleh"`
	Waktu   time.Time `gorm:"column:waktu;default:now()"`
	Catatan string    `gorm:"column:catatan"`
}

func (EwsRiwayat) TableName() string { return "ews_riwayat" }

// Alert adalah alert beserta nama komoditas dan kabupaten untuk tampilan.
type Alert struct {
	EwsAlert      `gorm:"embedded"`
	KomoditasNama *string `gorm:"column:komoditas_nama;->"`
	KabupatenNama string  `gorm:"column:kabupaten_nama;->"`
}

// EwsEvent memetakan tabel ews_event: peristiwa alert untuk aliran SSE. Id berurut
// dipakai sebagai Last-Event-ID.
type EwsEvent struct {
	ID         int64     `gorm:"column:id;primaryKey;->"`
	AlertID    string    `gorm:"column:alert_id"`
	Tipe       string    `gorm:"column:tipe;default:alert"`
	DibuatPada time.Time `gorm:"column:dibuat_pada;default:now()"`
}

func (EwsEvent) TableName() string { return "ews_event" }

// Kanal adalah daftar kanal notifikasi (kolom TEXT[] aturan_ews.kanal).
type Kanal []string

// Scan membaca literal array PostgreSQL, mis. "{DASHBOARD,PUSH}".
func (k *Kanal) Scan(src any) error {
	var literal string
	switch v := src.(type) {
	case nil:
		*k = nil
		return nil
	case string:
		literal = v
	case []byte:
		literal = string(v)
	default:
		return fmt.Errorf("kanal: tipe %T tidak didukung", src)
	}
	literal = strings.TrimSuffix(strings.TrimPrefix(literal, "{"), "}")
	if literal == "" {
		*k = Kanal{}
		return nil
	}
	*k = strings.Split(literal, ",")
	return nil
}

// Value menulis daftar sebagai literal array PostgreSQL. Nilai kanal berasal
// dari kode (huruf besar dan garis bawah), bukan masukan pengguna.
func (k Kanal) Value() (driver.Value, error) {
	for _, item := range k {
		if item == "" || strings.ContainsAny(item, "{},\"\\ ") {
			return nil, fmt.Errorf("kanal %q tidak valid", item)
		}
	}
	return "{" + strings.Join(k, ",") + "}", nil
}

// AturanEws memetakan tabel aturan_ews: ambang per jenis (dan komoditas) beserta
// kanal notifikasi.
type AturanEws struct {
	ID            string  `gorm:"column:id;primaryKey"`
	KomoditasKode *string `gorm:"column:komoditas_kode"`
	// KabupatenKode membatasi aturan ke satu kabupaten; kosong berlaku untuk semua.
	KabupatenKode *string `gorm:"column:kabupaten_kode"`
	Jenis         string  `gorm:"column:jenis"`
	AmbangWaspada float64 `gorm:"column:ambang_waspada"`
	AmbangSiaga   float64 `gorm:"column:ambang_siaga"`
	AmbangKritis  float64 `gorm:"column:ambang_kritis"`
	JendelaHari   int     `gorm:"column:jendela_hari"`
	// CooldownJam adalah jeda setelah alert SELESAI sebelum kondisi yang sama boleh memicu alert baru.
	CooldownJam int    `gorm:"column:cooldown_jam"`
	Kanal       Kanal  `gorm:"column:kanal;type:_text"`
	Aktif       bool   `gorm:"column:aktif"`
	Keterangan  string `gorm:"column:keterangan"`
	// Versi naik tiap perubahan ambang; dipakai untuk optimistic locking.
	Versi        int        `gorm:"column:versi"`
	DiubahOlehID *string    `gorm:"column:diubah_oleh_id"`
	DiubahPada   *time.Time `gorm:"column:diubah_pada"`
}

func (AturanEws) TableName() string { return "aturan_ews" }

// AturanEwsRiwayat memetakan tabel aturan_ews_riwayat: foto satu versi ambang.
type AturanEwsRiwayat struct {
	ID            string    `gorm:"column:id;primaryKey"`
	AturanID      string    `gorm:"column:aturan_id"`
	Versi         int       `gorm:"column:versi"`
	AmbangWaspada float64   `gorm:"column:ambang_waspada"`
	AmbangSiaga   float64   `gorm:"column:ambang_siaga"`
	AmbangKritis  float64   `gorm:"column:ambang_kritis"`
	JendelaHari   int       `gorm:"column:jendela_hari"`
	CooldownJam   int       `gorm:"column:cooldown_jam"`
	Kanal         Kanal     `gorm:"column:kanal;type:_text"`
	Aktif         bool      `gorm:"column:aktif"`
	Keterangan    string    `gorm:"column:keterangan"`
	DiubahOlehID  *string   `gorm:"column:diubah_oleh_id"`
	Waktu         time.Time `gorm:"column:waktu;default:now()"`
}

func (AturanEwsRiwayat) TableName() string { return "aturan_ews_riwayat" }

// Aturan adalah aturan beserta nama komoditasnya.
type Aturan struct {
	AturanEws     `gorm:"embedded"`
	KomoditasNama *string `gorm:"column:komoditas_nama;->"`
}

// BulkAction adalah aksi massal pada alert.
type BulkAction string

const (
	BulkAkui     BulkAction = "akui"
	BulkEskalasi BulkAction = "eskalasi"
	BulkSelesai  BulkAction = "selesai"
)

// ListQuery menyaring dan memaginasi daftar alert. Filter kosong berarti semua.
type ListQuery struct {
	Status, Severity, Jenis, KabupatenKode string
	Page, Limit                            int
}
