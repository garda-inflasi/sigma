package domain

import (
	"context"
	"errors"
)

// ErrNotFound dikembalikan GetByID bila sumber tidak ada.
var ErrNotFound = errors.New("sumber data tidak ditemukan")

type Repository interface {
	List(ctx context.Context) ([]SumberData, error)
	GetByID(ctx context.Context, id string) (*SumberData, error)
	// BulkAction menerapkan aksi pada sumber yang memenuhi syaratnya (jeda: yang
	// belum dijeda; lanjutkan: yang sedang dijeda) dan mengembalikan id yang berubah.
	BulkAction(ctx context.Context, ids []string, action BulkAction) ([]string, error)
}
