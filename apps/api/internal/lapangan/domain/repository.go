package domain

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound dikembalikan bila tugas tidak ada atau bukan milik pemanggil.
var ErrNotFound = errors.New("tugas tidak ditemukan")

type Repository interface {
	// Acuan membaca data acuan unduhan offline; median dihitung dari entri resmi sejak waktu itu.
	Acuan(ctx context.Context, sejak time.Time, minSampel int) (DataAcuan, error)
	// ListSaya mengembalikan tugas milik petugas: yang belum selesai lebih dulu, menurut tenggat.
	ListSaya(ctx context.Context, petugasID string) ([]TugasView, error)
	// GetByID mengembalikan satu tugas; semua true membuka tugas milik petugas lain.
	GetByID(ctx context.Context, id, petugasID string, semua bool) (*TugasView, error)
	// Aksesibel menyaring ids ke tugas yang boleh disentuh pemanggil.
	Aksesibel(ctx context.Context, ids []string, petugasID string, semua bool) ([]string, error)
	// TandaiSelesai menutup tugas yang targetnya sudah terpenuhi dan mengembalikan id yang berubah.
	TandaiSelesai(ctx context.Context, ids []string, petugasID string, semua bool, waktu time.Time) ([]string, error)
}
