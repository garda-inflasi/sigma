package domain

import "context"

type Repository interface {
	// List mengembalikan seluruh brief, periode terbaru lebih dulu.
	List(ctx context.Context) ([]BriefView, error)
	// Get mengembalikan satu brief; gorm.ErrRecordNotFound bila tidak ada.
	Get(ctx context.Context, id string) (*BriefView, error)
	// Create menyimpan brief dan melaporkan apakah baris benar-benar dibuat
	// (false bila id sudah dipakai, tanpa mengubah baris yang ada).
	Create(ctx context.Context, pb *PolicyBrief) (bool, error)
}
