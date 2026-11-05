package domain

import (
	"context"
	"time"
)

// HasilDraf adalah draf yang selesai disusun, siap disimpan ke brief yang sedang DIPROSES.
type HasilDraf struct {
	BriefID string
	JobID   string
	// OlehID adalah penyusun yang meminta draf; dicatat pada versi brief.
	OlehID      string
	Judul       string
	Ringkasan   string
	Rekomendasi string // dokumen JSON larik teks
	Halaman     int
	Konten      string // dokumen JSON Konten
	Snapshot    string // dokumen JSON fakta yang dipakai
	Penyusun    string
	ModelLLM    *string
	PromptVersi *string
	PromptHash  *string
	// Catatan mencatat alasan LLM digantikan templat; nil bila tidak ada.
	Catatan *string
	Waktu   time.Time
}

type Repository interface {
	// List mengembalikan seluruh brief, periode terbaru lebih dulu.
	List(ctx context.Context) ([]BriefView, error)
	// Get mengembalikan satu brief; gorm.ErrRecordNotFound bila tidak ada.
	Get(ctx context.Context, id string) (*BriefView, error)
	// GetDetail mengembalikan satu brief lengkap dengan isi; gorm.ErrRecordNotFound bila tidak ada.
	GetDetail(ctx context.Context, id string) (*DetailView, error)
	// Create menyimpan brief dan melaporkan apakah baris benar-benar dibuat
	// (false bila id sudah dipakai, tanpa mengubah baris yang ada).
	Create(ctx context.Context, pb *PolicyBrief) (bool, error)
	// MulaiUlang mengembalikan brief GAGAL, atau DIPROSES yang tidak berubah sejak
	// `basiSebelum` (proses yang mati), ke DIPROSES. False bila brief tidak ada atau statusnya lain.
	MulaiUlang(ctx context.Context, id string, basiSebelum time.Time) (bool, error)
	// Transisi mengubah status dari `dari` ke `ke` secara atomik dan melaporkan apakah
	// baris berubah (false bila tidak ada atau statusnya bukan `dari`).
	Transisi(ctx context.Context, id, dari, ke string) (bool, error)

	// MulaiJob mencatat percobaan penyusunan.
	MulaiJob(ctx context.Context, job *BriefJob) error
	// SelesaikanDraf menyimpan draf ke brief yang masih DIPROSES (status menjadi DRAF), menambah
	// versi brief, dan menutup job, dalam satu transaksi. False bila brief tidak lagi DIPROSES.
	SelesaikanDraf(ctx context.Context, d HasilDraf) (bool, error)
	// GagalkanDraf menandai brief DIPROSES dan job-nya GAGAL dengan catatan galat.
	GagalkanDraf(ctx context.Context, briefID, jobID, galat string, waktu time.Time) error

	// Kumpulkan membaca data nyata periode (awal bulan) untuk konteks brief.
	Kumpulkan(ctx context.Context, periode time.Time) (*DataPeriode, error)
}
