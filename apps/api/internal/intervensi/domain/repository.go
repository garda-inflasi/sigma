package domain

import "context"

type Repository interface {
	List(ctx context.Context) ([]KartuIntervensi, error)
	GetByID(ctx context.Context, id int64) (*KartuIntervensi, error)
	UpdateTahap(ctx context.Context, id int64, tahap TahapEnum, urutan int) error
}
