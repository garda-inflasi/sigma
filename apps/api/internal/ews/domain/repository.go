package domain

import "context"

// Repository adalah akses data alert dan aturan EWS.
type Repository interface {
	List(ctx context.Context, q ListQuery) (items []Alert, total int64, err error)
	GetByID(ctx context.Context, id string) (*Alert, error)
	// ListRiwayat mengembalikan perubahan status alert, terlama dahulu.
	ListRiwayat(ctx context.Context, alertID string) ([]Riwayat, error)
	ListAturan(ctx context.Context) ([]Aturan, error)
	GetAturan(ctx context.Context, id string) (*Aturan, error)
	// UpdateAturan menyimpan ambang baru dan versinya bila versi masih cocok
	// (ErrVersiUsang bila tidak, ErrAturanNotFound bila aturan tak ada), serta
	// mencatat foto versi baru di riwayat. Mengembalikan aturan terbaru.
	UpdateAturan(ctx context.Context, id string, upd AturanUpdate, userID string) (*Aturan, error)
	// BulkAction menerapkan aksi pada alert yang statusnya boleh bertransisi dan
	// mengembalikan id yang benar-benar berubah.
	BulkAction(ctx context.Context, ids []string, action BulkAction, userID string) ([]string, error)
}
