package domain

import (
	"context"
	"time"

	"github.com/garda-inflasi/api/pkg/idx"
)

// HasilDraf adalah draf yang selesai disusun, siap disimpan ke brief yang sedang DIPROSES.
type HasilDraf struct {
	// BriefID adalah UUID brief.
	BriefID string
	JobID   idx.ID
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
	// Get mengembalikan satu brief menurut UUID; gorm.ErrRecordNotFound bila tidak ada.
	Get(ctx context.Context, id string) (*BriefView, error)
	// GetDetail mengembalikan satu brief lengkap dengan isi menurut UUID atau kode (PB-YYYY-MM), tanpa
	// RiwayatVersi; gorm.ErrRecordNotFound bila tidak ada.
	GetDetail(ctx context.Context, ref idx.RujukanUUID) (*DetailView, error)
	// ListVersi mengembalikan versi isi brief (UUID), versi terbaru lebih dulu; kosong bila belum ada.
	ListVersi(ctx context.Context, briefID string) ([]VersiView, error)
	// Create menyimpan brief (ID UUID diberikan basis data dan diisi ke pb.ID) dan melaporkan apakah baris
	// benar-benar dibuat. Satu kode (periode) hanya satu brief: bila kodenya sudah dipakai, baris yang ada tidak
	// diubah, hasilnya false, dan pb.ID diisi dengan UUID brief yang ada.
	Create(ctx context.Context, pb *PolicyBrief) (bool, error)
	// MulaiUlang mengembalikan brief GAGAL, atau DIPROSES yang tidak berubah sejak
	// `basiSebelum` (proses yang mati), ke DIPROSES. False bila brief tidak ada atau statusnya lain.
	MulaiUlang(ctx context.Context, id string, basiSebelum time.Time) (bool, error)
	// Transisi mengubah status brief (UUID atau kode) dari `dari` ke `ke` secara atomik dan melaporkan UUID
	// brief serta apakah baris berubah (false bila tidak ada atau statusnya bukan `dari`).
	Transisi(ctx context.Context, ref idx.RujukanUUID, dari, ke string) (id string, berubah bool, err error)

	// MulaiJob mencatat percobaan penyusunan; ID BIGINT diberikan basis data dan diisi ke job.ID.
	MulaiJob(ctx context.Context, job *BriefJob) error
	// SelesaikanDraf menyimpan draf ke brief yang masih DIPROSES (status menjadi DRAF), menambah
	// versi brief, dan menutup job, dalam satu transaksi. False bila brief tidak lagi DIPROSES.
	SelesaikanDraf(ctx context.Context, d HasilDraf) (bool, error)
	// GagalkanDraf menandai brief DIPROSES dan job-nya GAGAL dengan catatan galat.
	GagalkanDraf(ctx context.Context, briefID string, jobID idx.ID, galat string, waktu time.Time) error

	// Kumpulkan membaca data nyata periode (awal bulan) untuk konteks brief.
	Kumpulkan(ctx context.Context, periode time.Time) (*DataPeriode, error)
}
