package domain

import (
	"context"
	"errors"

	"github.com/garda-inflasi/api/pkg/idx"
)

// ErrNotFound dikembalikan Get bila sumber tidak ada.
var ErrNotFound = errors.New("sumber data tidak ditemukan")

// Ringkas adalah identitas sebuah sumber: ID (UUID) beserta kodenya.
type Ringkas struct {
	ID   string `gorm:"column:id"`
	Kode string `gorm:"column:kode"`
}

type Repository interface {
	// List mengembalikan katalog, urut menurut kode.
	List(ctx context.Context) ([]SumberData, error)
	// Get mencari sumber menurut UUID atau kode (alias); ErrNotFound bila tidak ada.
	Get(ctx context.Context, ref idx.RujukanUUID) (*SumberData, error)
	// Cari mengembalikan identitas sumber yang id-nya ada di uuids atau kodenya ada di kodes.
	Cari(ctx context.Context, uuids, kodes []string) ([]Ringkas, error)
	// BulkAction menerapkan aksi pada sumber ber-id itu yang memenuhi syaratnya (jeda: yang
	// belum dijeda; lanjutkan: yang sedang dijeda) dan mengembalikan id yang berubah.
	BulkAction(ctx context.Context, ids []string, action BulkAction) ([]string, error)
}
