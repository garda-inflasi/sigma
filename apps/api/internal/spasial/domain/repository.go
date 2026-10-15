package domain

import "context"

type Repository interface {
	ListWilayah(ctx context.Context) ([]Wilayah, error)
}
