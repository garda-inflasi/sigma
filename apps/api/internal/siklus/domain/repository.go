package domain

import (
	"context"
	"errors"
)

// ErrNotFound dikembalikan bila wilayah tidak punya siklus aktif.
var ErrNotFound = errors.New("siklus aktif tidak ditemukan")

// Repository defines the data access interface for siklus.
type Repository interface {
	GetActiveByWilayah(ctx context.Context, wilayahKode string) (*SiklusEkonomi, error)
}
