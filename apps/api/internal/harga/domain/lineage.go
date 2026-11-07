package domain

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/garda-inflasi/api/pkg/idx"
)

// AktorGUC adalah variabel sesi lokal-transaksi yang memberi tahu pemicu entri_harga_catat_versi siapa pelakunya
// (migrasi 000118). Repository mengisinya di awal transaksi yang mengubah entri.
const AktorGUC = "garda.aktor_id"

// KolomBerubah adalah kolom TEXT[] berisi nama kolom entri_harga yang berubah pada satu versi. Nama kolom hanya
// memuat huruf kecil dan garis bawah, jadi format array teks PostgreSQL cukup dibaca tanpa penanganan escape.
type KolomBerubah []string

// Scan membaca nilai array teks PostgreSQL, mis. {divalidasi_oleh,status}.
func (k *KolomBerubah) Scan(src any) error {
	var raw string
	switch v := src.(type) {
	case nil:
		*k = KolomBerubah{}
		return nil
	case string:
		raw = v
	case []byte:
		raw = string(v)
	default:
		return fmt.Errorf("KolomBerubah: tipe %T tidak didukung", src)
	}

	raw = strings.TrimSpace(raw)
	if len(raw) < 2 || raw[0] != '{' || raw[len(raw)-1] != '}' {
		return fmt.Errorf("KolomBerubah: bukan array PostgreSQL: %q", raw)
	}
	body := raw[1 : len(raw)-1]
	if body == "" {
		*k = KolomBerubah{}
		return nil
	}
	parts := strings.Split(body, ",")
	for i, p := range parts {
		parts[i] = strings.Trim(p, `"`)
	}
	*k = parts
	return nil
}

// Value menulis array teks PostgreSQL. Nama yang memuat karakter di luar huruf kecil, angka, dan garis bawah ditolak.
func (k KolomBerubah) Value() (driver.Value, error) {
	for _, nama := range k {
		if nama == "" || strings.ContainsAny(nama, `{},"\ `) {
			return nil, fmt.Errorf("KolomBerubah: nama kolom %q tidak valid", nama)
		}
	}
	return "{" + strings.Join(k, ",") + "}", nil
}

// VersiEntri memetakan tabel entri_harga_versi (migrasi 000118): satu versi isi entri. Baris ditulis oleh pemicu
// basis data dan tidak bisa diubah atau dihapus; model ini hanya untuk kontrak skema dan pembacaan.
type VersiEntri struct {
	ID        int64           `gorm:"column:id;primaryKey;autoIncrement"`
	EntriID   idx.ID          `gorm:"column:entri_id"`
	Versi     int             `gorm:"column:versi"`
	Aksi      string          `gorm:"column:aksi"`
	Status    string          `gorm:"column:status"`
	Harga     int             `gorm:"column:harga"`
	Oleh      *string         `gorm:"column:oleh;type:uuid"`
	Waktu     time.Time       `gorm:"column:waktu;default:now()"`
	Perubahan KolomBerubah    `gorm:"column:perubahan;type:_text"`
	Data      json.RawMessage `gorm:"column:data;type:jsonb"`
}

func (VersiEntri) TableName() string {
	return "entri_harga_versi"
}

// VersiView adalah satu versi entri beserta nama pelakunya.
type VersiView struct {
	Versi     int             `gorm:"column:versi"`
	Aksi      string          `gorm:"column:aksi"`
	Status    string          `gorm:"column:status"`
	Harga     int             `gorm:"column:harga"`
	Oleh      *string         `gorm:"column:oleh;type:uuid"`
	OlehNama  *string         `gorm:"column:oleh_nama"`
	Waktu     time.Time       `gorm:"column:waktu"`
	Perubahan KolomBerubah    `gorm:"column:perubahan"`
	Data      json.RawMessage `gorm:"column:data"`
}

// NamaAktor adalah nama pelaku tiap tahap jejak entri (nama akun saat ini; NULL bila pelakunya tidak tercatat).
type NamaAktor struct {
	Divalidasi  *string `gorm:"column:divalidasi_nama"`
	Dipublikasi *string `gorm:"column:dipublikasi_nama"`
	Ditolak     *string `gorm:"column:ditolak_nama"`
}

// TugasTertaut adalah tugas lapangan yang dipenuhi entri (tabel tugas_entri).
type TugasTertaut struct {
	ID            idx.ID    `gorm:"column:id"`
	Kode          *string   `gorm:"column:kode"`
	Judul         string    `gorm:"column:judul"`
	DitautkanPada time.Time `gorm:"column:ditautkan_pada"`
}

// RujukanAudit menunjuk satu baris audit_log yang menyebut entri (aksi input atau aksi massal yang memuatnya).
// Hanya kolom ringkas; rincian lengkap ada di halaman audit trail.
type RujukanAudit struct {
	ID            idx.ID    `gorm:"column:id"`
	Waktu         time.Time `gorm:"column:waktu"`
	Aksi          string    `gorm:"column:aksi"`
	AktorNama     *string   `gorm:"column:aktor_nama"`
	AktorPeran    *string   `gorm:"column:aktor_peran"`
	Hasil         string    `gorm:"column:hasil"`
	CorrelationID *string   `gorm:"column:correlation_id"`
}

// BatasRujukanAudit membatasi jumlah rujukan audit pada satu lineage.
const BatasRujukanAudit = 100

// Lineage adalah jejak asal sebuah entri harga: siapa mencatat dan dari mana, validasi otomatis, siapa memvalidasi,
// menerbitkan, atau menolak, seluruh versi isinya, dan rujukan ke audit trail (PRD §1.4).
type Lineage struct {
	Entri EntriView
	Nama  NamaAktor
	// Tugas adalah tugas lapangan yang dipenuhi entri; kosong bila entri tidak berasal dari tugas.
	Tugas []TugasTertaut
	// VersiTerkini adalah nomor versi terbesar; 0 bila entri belum punya versi.
	VersiTerkini int
	Versi        []VersiView
	Riwayat      []RiwayatView
	Audit        []RujukanAudit
}
