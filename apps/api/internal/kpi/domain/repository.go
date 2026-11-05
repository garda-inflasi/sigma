package domain

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound dikembalikan bila KPI tidak ada.
var ErrNotFound = errors.New("kpi tidak ditemukan")

// ErrQueryTakDikenal dikembalikan Hitung bila sumber_query tidak punya penghitung.
var ErrQueryTakDikenal = errors.New("penghitung kpi tidak dikenal")

// HasilSimpan menjelaskan apa yang terjadi saat nilai disimpan.
type HasilSimpan int

const (
	// Baru: belum ada nilai untuk periode itu, versi 1 ditulis.
	Baru HasilSimpan = iota
	// Direvisi: nilai berbeda dari versi berlaku, versi baru ditulis.
	Direvisi
	// Tetap: nilai sama dengan versi berlaku, tidak ada yang ditulis.
	Tetap
)

type Repository interface {
	Definisi(ctx context.Context) ([]Definisi, error)
	DefinisiByKode(ctx context.Context, kode string) (*Definisi, error)
	// TargetBerlaku memilih per KPI target dengan periode_mulai terbaru yang tidak
	// melewati periode; target khusus wilayah didahulukan atas target umum.
	TargetBerlaku(ctx context.Context, periode time.Time, wilayah *string) ([]Target, error)
	// NilaiBerlaku mengembalikan versi terbesar tiap (kpi, periode) pada [dari, sampai], urut naik.
	NilaiBerlaku(ctx context.Context, dari, sampai time.Time, wilayah *string) ([]Nilai, error)
	// RiwayatKPI mengembalikan paling banyak n periode terakhir satu KPI (versi berlaku), urut naik.
	RiwayatKPI(ctx context.Context, kode string, wilayah *string, n int) ([]Nilai, error)
	// Hitung menjalankan penghitung bernama pada jendela. nil = tidak ada dasar data.
	Hitung(ctx context.Context, sumberQuery string, w Jendela) (*float64, error)
	// SimpanNilai menulis nilai (seluruh provinsi) secara idempoten dan berversi.
	SimpanNilai(ctx context.Context, kode string, periode time.Time, nilai float64, dihitungPada time.Time) (HasilSimpan, error)
}
