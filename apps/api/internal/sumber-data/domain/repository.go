package domain

import "context"

type Repository interface {
	List(ctx context.Context) ([]SumberData, error)
	GetByID(ctx context.Context, id int64) (*SumberData, error)
	BulkAction(ctx context.Context, ids []int64, action BulkAction) (int64, error)
}
