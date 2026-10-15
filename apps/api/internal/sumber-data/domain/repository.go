package domain

import "context"

type Repository interface {
	List(ctx context.Context) ([]SumberData, error)
}
