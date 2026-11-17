package domain

import (
	"context"
	"errors"
	"time"

	hargadomain "github.com/garda-inflasi/api/internal/harga/domain"
	"github.com/garda-inflasi/api/pkg/idx"
)

// ErrNotFound dikembalikan bila tugas tidak ada atau bukan milik pemanggil.
var ErrNotFound = errors.New("tugas tidak ditemukan")

// TugasRef menyebut satu baris tugas: ID BIGINT dan kode lamanya (bila ada).
type TugasRef struct {
	ID   idx.ID  `gorm:"column:id"`
	Kode *string `gorm:"column:kode"`
}

// Cocok melaporkan apakah rujukan ref (ID atau kode lama) menunjuk baris ini.
func (t TugasRef) Cocok(ref idx.RujukanID) bool {
	if ref.ID != 0 {
		return ref.ID == t.ID
	}
	return ref.Kode != "" && t.Kode != nil && *t.Kode == ref.Kode
}

type Repository interface {
	// PenggunaCakupan membaca peran dan kabupaten tugas pengguna langsung dari DB per request (02-B).
	PenggunaCakupan(ctx context.Context, userID string) (peran, kabupatenKode string, ditemukan bool, err error)

	// Acuan membaca data acuan unduhan offline; median dihitung dari entri resmi sejak waktu itu.
	Acuan(ctx context.Context, sejak time.Time, minSampel int) (DataAcuan, error)
	// ListSaya mengembalikan tugas sesuai cakupan baris pemanggil: yang belum selesai lebih dulu, menurut tenggat.
	ListSaya(ctx context.Context, cakupan hargadomain.Cakupan) ([]TugasView, error)
	// GetByID mengembalikan satu tugas menurut ID BIGINT atau kode lama (alias tautan dan antrean lama)
	// sesuai cakupan baris pemanggil.
	GetByID(ctx context.Context, ref idx.RujukanID, cakupan hargadomain.Cakupan) (*TugasView, error)
	// Rincian mengembalikan nama dan status terisi tiap kode komoditas tugas (urutan tidak dijamin).
	Rincian(ctx context.Context, tugasID idx.ID, kode []string) ([]RincianKomoditas, error)
	// Aksesibel menyaring refs (ID atau kode lama) ke baris tugas yang boleh disentuh pemanggil.
	Aksesibel(ctx context.Context, refs []idx.RujukanID, cakupan hargadomain.Cakupan) ([]TugasRef, error)
	// TandaiSelesai menutup tugas yang targetnya sudah terpenuhi dan mengembalikan baris yang berubah.
	TandaiSelesai(ctx context.Context, refs []idx.RujukanID, cakupan hargadomain.Cakupan, waktu time.Time) ([]TugasRef, error)

	// EntriMilikPetugas memeriksa keberadaan entri_harga dan mengembalikan petugas_id serta kabupaten_kode pasarnya.
	EntriMilikPetugas(ctx context.Context, entriID idx.ID) (petugasID, kabupatenKode string, ditemukan bool, err error)
	// SimpanLampiranEntri mencatat metadata lampiran foto dan memperbarui entri_harga.foto_url dalam satu transaksi.
	SimpanLampiranEntri(ctx context.Context, lampiran *LampiranEntri, fotoURL string) error
	// LampiranTerbaru mengambil lampiran foto terbaru untuk sebuah entri_harga.
	LampiranTerbaru(ctx context.Context, entriID idx.ID) (*LampiranEntri, error)

	// KodeEtikAktif mengembalikan versi kode etik aktif beserta status persetujuan pengguna.
	KodeEtikAktif(ctx context.Context, userID string) (*KodeEtikStatus, error)
	// SetujuiKodeEtik mencatat persetujuan pengguna terhadap versi kode etik tertentu (idempoten).
	SetujuiKodeEtik(ctx context.Context, userID, versi string) (*PersetujuanKodeEtik, error)

	// ListKinerja membaca view kinerja_petugas sesuai cakupan baris pemanggil.
	ListKinerja(ctx context.Context, cakupan hargadomain.Cakupan) ([]KinerjaPetugas, error)
}
