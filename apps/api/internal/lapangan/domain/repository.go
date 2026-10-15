package domain

import "context"

type Repository interface {
	ListSaya(ctx context.Context, pic string) ([]TugasLapangan, error)
	GetByID(ctx context.Context, id int64) (*TugasLapangan, error)
	BulkAction(ctx context.Context, ids []int64, action BulkAction) (int64, error)
}
