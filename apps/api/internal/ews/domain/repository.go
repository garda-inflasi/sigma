package domain

import "context"

type Repository interface {
	List(ctx context.Context, limit int) ([]EwsAlert, error)
	GetByID(ctx context.Context, id int64) (*EwsAlert, error)
	BulkAction(ctx context.Context, ids []int64, action BulkAction, userID string) (int64, error)
}
