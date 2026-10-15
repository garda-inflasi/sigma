package domain

import "context"

type Repository interface {
	ListKomoditas(ctx context.Context) ([]Komoditas, error)
}
