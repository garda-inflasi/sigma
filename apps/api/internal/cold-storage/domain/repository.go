package domain

import "context"

type Repository interface {
	GetAllFasilitas(ctx context.Context) ([]FasilitasColdStorage, error)
	GetAllStok(ctx context.Context) ([]StokColdStorage, error)
}
