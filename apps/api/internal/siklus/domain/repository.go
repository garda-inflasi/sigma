package domain

import "context"

// Repository defines the data access interface for siklus.
type Repository interface {
	GetActiveByWilayah(ctx context.Context, wilayahKode string) (*SiklusEkonomi, error)
}
