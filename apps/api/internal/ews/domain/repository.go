package domain

import (
	"context"

	"github.com/garda-inflasi/api/pkg/idx"
)

// Repository adalah akses data alert dan aturan EWS. Rujukan alert dan aturan boleh berupa ID BIGINT atau
// kode tampilan lama (EWS-..., ATR-...), sehingga tautan lama di notifikasi dan email tetap berlaku.
type Repository interface {
	List(ctx context.Context, q ListQuery) (items []Alert, total int64, err error)
	// GetByID mencari alert menurut ID atau kode (gorm.ErrRecordNotFound bila tidak ada).
	GetByID(ctx context.Context, ref idx.RujukanID) (*Alert, error)
	// ListRiwayat mengembalikan perubahan status alert, terlama dahulu.
	ListRiwayat(ctx context.Context, alertID idx.ID) ([]Riwayat, error)
	ListAturan(ctx context.Context) ([]Aturan, error)
	// GetAturan mencari aturan menurut ID atau kode (ErrAturanNotFound bila tidak ada).
	GetAturan(ctx context.Context, ref idx.RujukanID) (*Aturan, error)
	// UpdateAturan menyimpan ambang baru dan versinya bila versi masih cocok
	// (ErrVersiUsang bila tidak, ErrAturanNotFound bila aturan tak ada), serta
	// mencatat foto versi baru di riwayat. Mengembalikan aturan terbaru.
	UpdateAturan(ctx context.Context, id idx.ID, upd AturanUpdate, userID string) (*Aturan, error)
	// BulkAction menerapkan aksi pada alert yang statusnya boleh bertransisi dan
	// mengembalikan id yang benar-benar berubah.
	BulkAction(ctx context.Context, ids []idx.ID, action BulkAction, userID string) ([]idx.ID, error)
}
