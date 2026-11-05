package domain

import (
	"context"
	"errors"
)

// ErrActorNotFound dikembalikan FindActor bila akun tidak ada.
var ErrActorNotFound = errors.New("aktor tidak ditemukan")

// ErrNotFound dikembalikan Get bila entri audit tidak ada.
var ErrNotFound = errors.New("entri audit tidak ditemukan")

type Repository interface {
	Create(ctx context.Context, log *AuditLog) error
	// FindActor mengambil identitas akun untuk snapshot aktor.
	FindActor(ctx context.Context, id string) (*Actor, error)
	// Get mengambil satu entri; ErrNotFound bila tidak ada.
	Get(ctx context.Context, id string) (*AuditLog, error)
	// List mengembalikan sampai limit baris terbaru lebih dulu, mulai setelah
	// after (nil = dari yang terbaru).
	List(ctx context.Context, filter Filter, after *Cursor, limit int) ([]AuditLog, error)
}
