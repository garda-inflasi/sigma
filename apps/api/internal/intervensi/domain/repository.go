package domain

import (
	"context"
	"errors"

	"github.com/garda-inflasi/api/pkg/idx"
)

// Repository adalah akses data kartu intervensi dan definisi tahapnya. ID kartu adalah UUID kanonik huruf
// kecil; rujukan dari rute boleh berupa UUID atau kode tampilan lama (INT-n) lewat idx.RujukanUUID.
type Repository interface {
	List(ctx context.Context, q ListQuery) ([]Kartu, error)
	// ListTahap mengembalikan definisi tahap terurut menurut siklus lalu urutan.
	ListTahap(ctx context.Context) ([]TahapDef, error)
	// GetByID mencari kartu menurut ID atau kode (gorm.ErrRecordNotFound bila tidak ada).
	GetByID(ctx context.Context, ref idx.RujukanUUID) (*Intervensi, error)
	// Create menyimpan kartu pada tahap `tahap` di urutan paling bawah dan
	// mengembalikan kartu yang tersimpan (dengan id baru).
	Create(ctx context.Context, in NewIntervensi, tahap string) (*Intervensi, error)
	// Move memindahkan kartu ke tahap `tahap` pada posisi index (dijepit ke rentang
	// kolom), menomori ulang kolom tujuan, mengisi/mengosongkan selesai_pada, dan
	// mencatat riwayat bila tahap berganti. Ditolak dengan ErrVersiUsang bila versi
	// kartu bukan `versi`. Mengembalikan versi baru kartu.
	Move(ctx context.Context, id, tahap string, index int, selesai bool, versi int, oleh string) (versiBaru int, err error)
	// BulkAksi menerapkan aksi pada tiap kartu dan mengembalikan hasil per kartu
	// (sesuai urutan ids). Kartu yang gagal tidak menggagalkan yang lain.
	BulkAksi(ctx context.Context, aksi BulkAksi, ids []string, oleh string) ([]HasilKartu, error)
	// ListRiwayat mengembalikan peristiwa kartu, terlama dahulu.
	ListRiwayat(ctx context.Context, id string) ([]Riwayat, error)
}

// PenggunaCakupan adalah peran dan kabupaten tugas pengguna (pengguna.peran, pengguna.kabupaten_kode); dasar
// cakupan wilayah (02-B). KabupatenKode kosong berarti tidak punya kabupaten tugas.
type PenggunaCakupan struct {
	Peran         string
	KabupatenKode string
}

// ErrPenggunaTidakAda: pengguna tidak ditemukan saat membaca cakupannya.
var ErrPenggunaTidakAda = errors.New("pengguna tidak ditemukan")

// PembacaCakupan membaca cakupan pengguna segar dari basis data per permintaan, bukan dari klaim token, supaya
// perubahan peran atau kabupaten berlaku seketika.
type PembacaCakupan interface {
	PenggunaCakupan(ctx context.Context, penggunaID string) (PenggunaCakupan, error)
}
