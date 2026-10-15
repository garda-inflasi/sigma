package domain

import "context"

type Repository interface {
	GetByKode(ctx context.Context, kode string) (*ProyeksiXai, error)
	List(ctx context.Context, page, limit int) ([]ProyeksiXai, int64, error)
}
