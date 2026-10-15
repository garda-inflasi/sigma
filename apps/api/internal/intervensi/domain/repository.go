package domain

import "context"

type Repository interface {
	List(ctx context.Context) ([]KartuIntervensi, error)
	GetByID(ctx context.Context, id int64) (*KartuIntervensi, error)
	Create(ctx context.Context, kartu *KartuIntervensi) error
	UpdateTahap(ctx context.Context, id int64, tahap TahapEnum, urutan int) error
	BulkAction(ctx context.Context, ids []int64, action BulkAction) (int64, error)
}
