package domain

import "context"

type Repository interface {
	List(ctx context.Context) ([]PolicyBrief, error)
	Create(ctx context.Context, pb *PolicyBrief) error
}
