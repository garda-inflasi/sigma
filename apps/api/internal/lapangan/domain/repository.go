package domain

import (
	"context"
	"errors"
	"time"

	"github.com/garda-inflasi/api/pkg/idx"
)

// ErrNotFound dikembalikan bila tugas tidak ada atau bukan milik pemanggil.
var ErrNotFound = errors.New("tugas tidak ditemukan")

// TugasRef menyebut satu baris tugas: ID BIGINT dan kode lamanya (bila ada).
type TugasRef struct {
	ID   idx.ID  `gorm:"column:id"`
	Kode *string `gorm:"column:kode"`
}

// Cocok melaporkan apakah rujukan ref (ID atau kode lama) menunjuk baris ini.
func (t TugasRef) Cocok(ref idx.RujukanID) bool {
	if ref.ID != 0 {
		return ref.ID == t.ID
	}
	return ref.Kode != "" && t.Kode != nil && *t.Kode == ref.Kode
}

type Repository interface {
	// Acuan membaca data acuan unduhan offline; median dihitung dari entri resmi sejak waktu itu.
	Acuan(ctx context.Context, sejak time.Time, minSampel int) (DataAcuan, error)
	// ListSaya mengembalikan tugas milik petugas: yang belum selesai lebih dulu, menurut tenggat.
	ListSaya(ctx context.Context, petugasID string) ([]TugasView, error)
	// GetByID mengembalikan satu tugas menurut ID BIGINT atau kode lama (alias tautan dan antrean lama);
	// semua true membuka tugas milik petugas lain.
	GetByID(ctx context.Context, ref idx.RujukanID, petugasID string, semua bool) (*TugasView, error)
	// Rincian mengembalikan nama dan status terisi tiap kode komoditas tugas (urutan tidak dijamin).
	Rincian(ctx context.Context, tugasID idx.ID, kode []string) ([]RincianKomoditas, error)
	// Aksesibel menyaring refs (ID atau kode lama) ke baris tugas yang boleh disentuh pemanggil.
	Aksesibel(ctx context.Context, refs []idx.RujukanID, petugasID string, semua bool) ([]TugasRef, error)
	// TandaiSelesai menutup tugas yang targetnya sudah terpenuhi dan mengembalikan baris yang berubah.
	TandaiSelesai(ctx context.Context, refs []idx.RujukanID, petugasID string, semua bool, waktu time.Time) ([]TugasRef, error)
}
