package domain

import (
	"context"
	"time"
)

// PenggunaFoto memetakan tabel pengguna_foto (migrasi 000124): foto profil pemilik akun, satu per akun. Isinya JPEG
// hasil olahan server (bukan unggahan asli) dan hanya terbaca oleh pemiliknya.
type PenggunaFoto struct {
	PenggunaID string `gorm:"column:pengguna_id;type:uuid;primaryKey"`
	// Tipe adalah tipe media hasil olahan; saat ini selalu FotoTipeJPEG.
	Tipe   string `gorm:"column:tipe"`
	Lebar  int    `gorm:"column:lebar"`
	Tinggi int    `gorm:"column:tinggi"`
	// Ukuran adalah jumlah byte Data (CHECK 1 sampai 524288 dan sama dengan octet_length(data)).
	Ukuran int `gorm:"column:ukuran"`
	// SHA256 adalah sidik jari konten (heksadesimal huruf kecil, 64 karakter): ETag foto dan asal FotoVersi.
	SHA256     string    `gorm:"column:sha256"`
	Data       []byte    `gorm:"column:data"`
	DiubahPada time.Time `gorm:"column:diubah_pada;default:now()"`
}

func (PenggunaFoto) TableName() string {
	return "pengguna_foto"
}

// FotoTipeJPEG adalah satu-satunya tipe media yang disimpan: semua unggahan dienkode ulang menjadi JPEG.
const FotoTipeJPEG = "image/jpeg"

// FotoVersiPanjang adalah jumlah karakter pertama SHA256 yang menjadi versi foto.
const FotoVersiPanjang = 12

// FotoVersi menurunkan versi foto (kunci cache ?v= di UI) dari sidik jari kontennya; kosong bila tidak ada foto.
func FotoVersi(sha256 string) string {
	if len(sha256) < FotoVersiPanjang {
		return ""
	}
	return sha256[:FotoVersiPanjang]
}

// FotoRepository menyimpan foto profil pemilik akun. Tidak ada metode yang membaca foto orang lain atas nama
// pemanggil: semua menerima id pemilik dari token, bukan dari masukan klien.
type FotoRepository interface {
	// GetFoto membaca foto pemilik beserta byte-nya; galat membungkus gorm.ErrRecordNotFound bila belum ada.
	GetFoto(ctx context.Context, userID string) (*PenggunaFoto, error)
	// GetFotoSHA256 hanya membaca sidik jari foto, tanpa memuat byte gambar ("" bila belum ada). Dipakai
	// GET /v1/auth/me untuk foto_versi.
	GetFotoSHA256(ctx context.Context, userID string) (string, error)
	// SimpanFoto menulis foto pemilik, menggantikan yang lama (upsert atas pengguna_id) dan memperbarui diubah_pada.
	// Galat membungkus gorm.ErrForeignKeyViolated bila akunnya sudah tidak ada.
	SimpanFoto(ctx context.Context, foto *PenggunaFoto) error
	// HapusFoto menghapus foto pemilik; true bila ada baris yang terhapus (menghapus yang tidak ada bukan galat).
	HapusFoto(ctx context.Context, userID string) (bool, error)
}
