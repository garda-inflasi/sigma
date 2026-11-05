package domain

import "context"

// Repository adalah akses data alert dan aturan EWS.
type Repository interface {
	List(ctx context.Context, q ListQuery) (items []Alert, total int64, err error)
	GetByID(ctx context.Context, id string) (*Alert, error)
	ListAturan(ctx context.Context) ([]Aturan, error)
	// BulkAction menerapkan aksi pada alert yang statusnya boleh bertransisi dan
	// mengembalikan id yang benar-benar berubah.
	BulkAction(ctx context.Context, ids []string, action BulkAction, userID string) ([]string, error)
}
