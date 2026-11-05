package domain

import "context"

// Repository adalah akses data kartu intervensi dan definisi tahapnya.
type Repository interface {
	List(ctx context.Context, q ListQuery) ([]Kartu, error)
	// ListTahap mengembalikan definisi tahap terurut menurut siklus lalu urutan.
	ListTahap(ctx context.Context) ([]TahapDef, error)
	GetByID(ctx context.Context, id string) (*Intervensi, error)
	// Create menyimpan kartu pada tahap `tahap` di urutan paling bawah dan
	// mengembalikan kartu yang tersimpan (dengan id baru).
	Create(ctx context.Context, in NewIntervensi, tahap string) (*Intervensi, error)
	// Move memindahkan kartu ke tahap `tahap` pada posisi index (dijepit ke rentang
	// kolom), menomori ulang kolom tujuan, dan mengisi/mengosongkan selesai_pada.
	Move(ctx context.Context, id, tahap string, index int, selesai bool) error
}
