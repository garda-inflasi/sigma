package domain

import (
	"context"
	"time"
)

// Hasil sebuah aksi yang dicatat.
const (
	HasilBerhasil = "BERHASIL"
	HasilGagal    = "GAGAL"
	HasilDitolak  = "DITOLAK"
)

// AuditLog memetakan tabel audit_log. Tabel bersifat append-only (trigger di
// basis data), jadi entity ini hanya dibuat dan dibaca, tidak pernah diubah.
type AuditLog struct {
	ID            string    `gorm:"column:id;primaryKey"`
	Waktu         time.Time `gorm:"column:waktu;default:now()"`
	AktorID       *string   `gorm:"column:aktor_id"`
	AktorNama     *string   `gorm:"column:aktor_nama"`
	AktorEmail    *string   `gorm:"column:aktor_email"`
	AktorPeran    *string   `gorm:"column:aktor_peran"`
	Aksi          string    `gorm:"column:aksi"`
	Entitas       string    `gorm:"column:entitas"`
	EntitasID     string    `gorm:"column:entitas_id"`
	Hasil         string    `gorm:"column:hasil"`
	IP            *string   `gorm:"column:ip"`
	UserAgent     *string   `gorm:"column:user_agent"`
	CorrelationID *string   `gorm:"column:correlation_id"`
	Sebelum       *string   `gorm:"column:sebelum;type:jsonb"`
	Sesudah       *string   `gorm:"column:sesudah;type:jsonb"`
	Metadata      *string   `gorm:"column:metadata;type:jsonb"`
}

func (AuditLog) TableName() string {
	return "audit_log"
}

// Entry adalah satu aksi yang hendak dicatat. Nama, email, dan peran aktor
// dicatat otomatis dari akunnya; IP, user agent, dan correlation id dari Origin
// pada context.
type Entry struct {
	// AktorID kosong untuk aksi tanpa pelaku terautentikasi (mis. login gagal
	// dengan email tak dikenal).
	AktorID   string
	Aksi      string
	Entitas   string
	EntitasID string
	Hasil     string
	// Sebelum dan Sesudah adalah keadaan objek (di-marshal ke JSON) bila aksi mengubah data.
	Sebelum, Sesudah any
	Metadata         map[string]any
	// Waktu kosong berarti sekarang.
	Waktu time.Time
}

// Writer mencatat satu baris jejak audit. Modul lain menerimanya lewat
// konstruktor; kegagalan mencatat dilaporkan ke pemanggil, yang memutuskan apakah
// aksinya ikut gagal.
type Writer interface {
	Write(ctx context.Context, entry Entry) error
}

// Actor adalah identitas pelaku yang disalin ke baris audit agar tetap terbaca
// walau akunnya kemudian dihapus.
type Actor struct {
	ID    string
	Nama  string
	Email string
	Peran string
}

// Filter menyaring daftar audit di sisi server.
type Filter struct {
	Dari   *time.Time // inklusif
	Sampai *time.Time // eksklusif
	// Aktor cocok dengan id akun persis, atau potongan nama/email.
	Aktor   string
	Aksi    string
	Entitas string
	Hasil   string
	// EntitasID mencocokkan id objek persis (jejak satu objek).
	EntitasID string
	// Peran mencocokkan peran aktor persis (snapshot saat aksi).
	Peran string
	// CorrelationID mencocokkan semua baris dari satu permintaan.
	CorrelationID string
	// Q adalah pencarian bebas (tak peka huruf besar-kecil) pada aksi, entitas,
	// id entitas, nama/email aktor, dan correlation id.
	Q string
}

// Cursor menandai posisi halaman berikutnya pada urutan (waktu, id) menurun.
type Cursor struct {
	Waktu time.Time
	ID    string
}

// Page adalah satu halaman daftar audit; NextCursor kosong berarti halaman terakhir.
type Page struct {
	Items      []AuditLog
	NextCursor string
}
