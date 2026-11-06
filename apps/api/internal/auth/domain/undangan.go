package domain

import (
	"context"
	"errors"
	"time"
)

// StatusMenunggu adalah status akun yang sudah dibuat admin tetapi belum diaktivasi pemiliknya: tanpa kata
// sandi, tidak bisa masuk, dan hanya berubah lewat aktivasi (tautan undangan) atau dihapus.
const StatusMenunggu = "MENUNGGU"

// ErrBukanMenunggu: aksi undangan hanya berlaku untuk akun berstatus MENUNGGU.
var ErrBukanMenunggu = errors.New("akun tidak berstatus menunggu aktivasi")

// Undangan memetakan tabel undangan_pengguna: satu tautan aktivasi sekali pakai. Token mentah tidak pernah
// disimpan, hanya hash SHA-256-nya.
type Undangan struct {
	ID        int64  `gorm:"column:id;primaryKey;autoIncrement"`
	UserID    string `gorm:"column:user_id"`
	TokenHash string `gorm:"column:token_hash"`
	// Email adalah alamat tujuan saat undangan dikirim; aktivasi hanya sah selama email akun masih sama.
	Email        string     `gorm:"column:email"`
	DiundangOleh *string    `gorm:"column:diundang_oleh"`
	ExpiresAt    time.Time  `gorm:"column:expires_at"`
	UsedAt       *time.Time `gorm:"column:used_at"`
	DicabutPada  *time.Time `gorm:"column:dicabut_pada"`
	DibuatPada   time.Time  `gorm:"column:dibuat_pada;default:now()"`
}

func (Undangan) TableName() string { return "undangan_pengguna" }

// UndanganBaru adalah satu akun MENUNGGU beserta undangan pertamanya, ditulis bersama.
type UndanganBaru struct {
	User     *Pengguna
	Undangan *Undangan
}

// UndanganRepository menyimpan undangan pengguna dan menegakkan aktivasi atomiknya.
type UndanganRepository interface {
	// BuatUndangan menulis semua akun MENUNGGU dan undangannya dalam SATU transaksi: semua tersimpan atau tidak
	// satu pun. Galat membungkus gorm.ErrDuplicatedKey bila ada email (tak peka huruf besar-kecil) atau id
	// yang sudah dipakai.
	BuatUndangan(ctx context.Context, baru []UndanganBaru) error
	// GantiUndangan mencabut semua undangan hidup akun lalu menyimpan u sebagai satu-satunya yang berlaku,
	// dalam satu transaksi dengan kunci baris akun. Mengembalikan akun itu. Galat membungkus
	// gorm.ErrRecordNotFound bila akun tidak ada dan ErrBukanMenunggu bila statusnya bukan MENUNGGU.
	GantiUndangan(ctx context.Context, userID string, u *Undangan, now time.Time) (*Pengguna, error)
	// GetUndangan mencari undangan menurut hash token; galat membungkus gorm.ErrRecordNotFound bila tidak ada.
	GetUndangan(ctx context.Context, tokenHash string) (*Undangan, error)
	// Aktifkan memakai undangan dan mengaktifkan akunnya dalam satu transaksi: token ditandai terpakai, kata
	// sandi diisi, status AKTIF, penanda wajib ganti dimatikan. Hanya berhasil bila undangan belum dipakai,
	// belum dicabut, belum kedaluwarsa pada now, akunnya masih MENUNGGU, dan email akun masih sama dengan
	// alamat tujuan undangan. Mengembalikan akun yang diaktifkan; (nil, nil) bila syarat itu tidak terpenuhi
	// (tidak ada yang berubah).
	Aktifkan(ctx context.Context, tokenHash, passwordHash string, now time.Time) (*Pengguna, error)
	// EmailTerdaftar mengembalikan subset email (huruf kecil) yang sudah dipakai akun mana pun.
	EmailTerdaftar(ctx context.Context, emailKecil []string) (map[string]bool, error)
	// KabupatenAda mengembalikan subset kode kabupaten yang ada di master wilayah.
	KabupatenAda(ctx context.Context, kode []string) (map[string]bool, error)
}

// EksporRepository membaca pengguna untuk ekspor CSV.
type EksporRepository interface {
	// ListUsersUntukEkspor mengembalikan paling banyak batas pengguna yang cocok dengan filter daftar yang sama
	// (FilterPengguna, sudah dinormalkan), terbaru dulu. sekarang adalah waktu acuan filter Terkunci.
	// terpotong true bila masih ada yang cocok di luar batas.
	ListUsersUntukEkspor(ctx context.Context, filter FilterPengguna, sekarang time.Time, batas int) (users []Pengguna, terpotong bool, err error)
}
