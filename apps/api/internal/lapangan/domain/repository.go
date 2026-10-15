package domain

import "context"

type Repository interface {
	ListSaya(ctx context.Context, pic string) ([]TugasLapangan, error)
}
