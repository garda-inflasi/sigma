package domain

import (
	"context"
	"errors"
	"time"

	"github.com/garda-inflasi/api/pkg/idx"
)

// Galat domain data referensi. Lapisan delivery memetakannya ke 404 dan 409.
var (
	// ErrTidakDitemukan: kode tidak dikenal (atau bentuknya tidak mungkin ada).
	ErrTidakDitemukan = errors.New("master: tidak ditemukan")
	// ErrAdaUsulan: entitas masih punya usulan perubahan berstatus MENUNGGU.
	ErrAdaUsulan = errors.New("master: ada usulan perubahan yang menunggu")
	// ErrDirujuk: entitas masih dirujuk tabel lain (FK RESTRICT, SQLSTATE 23503).
	ErrDirujuk = errors.New("master: masih dirujuk data lain")
)

// Nama entitas (EntitasKomoditas, EntitasPasar) didefinisikan di perubahan.go.

// AksiHapusMaster adalah aksi audit penghapusan master.
const AksiHapusMaster = "HAPUS_MASTER"

// KecamatanRef adalah kecamatan beserta nama kabupaten/kota induknya.
type KecamatanRef struct {
	ID            idx.ID  `gorm:"column:id"`
	Kode          string  `gorm:"column:kode"`
	Nama          string  `gorm:"column:nama"`
	KabupatenKode string  `gorm:"column:kabupaten_kode"`
	KabupatenNama string  `gorm:"column:kabupaten_nama"`
	KodeDagri     *string `gorm:"column:kode_dagri"`
}

// SatuanRef adalah satuan beserta jumlah komoditas yang memakainya (semua status).
type SatuanRef struct {
	ID              idx.ID `gorm:"column:id"`
	Kode            string `gorm:"column:kode"`
	Nama            string `gorm:"column:nama"`
	JumlahKomoditas int64  `gorm:"column:jumlah_komoditas"`
}

// PedagangFilter menyaring daftar pedagang. Aktif nil berarti semua.
type PedagangFilter struct {
	PasarID string
	Aktif   *bool
}

// PedagangRef adalah pedagang beserta nama pasarnya dan jumlah entri harga yang memuat kodenya.
type PedagangRef struct {
	ID          idx.ID  `gorm:"column:id"`
	PasarID     string  `gorm:"column:pasar_id"`
	PasarNama   string  `gorm:"column:pasar_nama"`
	Kode        string  `gorm:"column:kode"`
	Nama        *string `gorm:"column:nama"`
	Aktif       bool    `gorm:"column:aktif"`
	JumlahEntri int64   `gorm:"column:jumlah_entri"`
}

// KomoditasVersiRef adalah satu versi definisi komoditas beserta akhir masa berlakunya.
// BerlakuSampai nil untuk versi yang kini berlaku; selain itu berlaku_dari versi berikutnya.
type KomoditasVersiRef struct {
	ID            idx.ID     `gorm:"column:id"`
	Versi         int        `gorm:"column:versi"`
	Nama          string     `gorm:"column:nama"`
	Kategori      string     `gorm:"column:kategori"`
	Satuan        string     `gorm:"column:satuan"`
	HargaAcuan    int        `gorm:"column:harga_acuan"`
	ShelfLifeHari int        `gorm:"column:shelf_life_hari"`
	Pilot         bool       `gorm:"column:pilot"`
	BerlakuDari   time.Time  `gorm:"column:berlaku_dari"`
	BerlakuSampai *time.Time `gorm:"column:berlaku_sampai"`
	PerubahanID   *idx.ID    `gorm:"column:perubahan_id"`
}

// WilayahDetail adalah kabupaten/kota beserta cacah kecamatan dan pasarnya.
type WilayahDetail struct {
	Kabupaten       Kabupaten
	JumlahKecamatan int64
	JumlahPasar     int64
}

// PasarDetail adalah pasar beserta nama kabupaten, nama kecamatan (bila ada), dan cacah pedagang terdaftar.
// Pasar.KecamatanNama sudah terisi.
type PasarDetail struct {
	Pasar          Pasar
	KabupatenNama  string
	JumlahPedagang int64
}

// KomoditasDetail adalah komoditas beserta riwayat versinya (terbaru lebih dulu).
type KomoditasDetail struct {
	Komoditas    Komoditas
	RiwayatVersi []KomoditasVersiRef
}

// RepositoryReferensi adalah akses data referensi master: kecamatan, satuan, pedagang, detail, riwayat versi,
// dan penghapusan. Terpisah dari Repository agar daftar utama tetap kecil.
type RepositoryReferensi interface {
	// ListKecamatan terurut menurut kode; kabupatenKode kosong berarti semua.
	ListKecamatan(ctx context.Context, kabupatenKode string, q ListQuery) ([]KecamatanRef, int64, error)
	// GetKecamatan mengembalikan ErrTidakDitemukan bila kode tidak ada.
	GetKecamatan(ctx context.Context, kode string) (KecamatanRef, error)
	ListSatuan(ctx context.Context, q ListQuery) ([]SatuanRef, int64, error)
	// ListPedagang terurut menurut pasar_id, kode.
	ListPedagang(ctx context.Context, f PedagangFilter, q ListQuery) ([]PedagangRef, int64, error)

	// GetKomoditas mengembalikan ErrTidakDitemukan bila kode tidak ada.
	GetKomoditas(ctx context.Context, kode string) (Komoditas, error)
	// RiwayatVersiKomoditas mengurutkan terbaru dulu (berlaku_dari, id menurun).
	RiwayatVersiKomoditas(ctx context.Context, kode string) ([]KomoditasVersiRef, error)
	GetWilayah(ctx context.Context, kode string) (WilayahDetail, error)
	GetPasar(ctx context.Context, kode string) (PasarDetail, error)

	// HapusKomoditas dan HapusPasar menghapus satu baris dan mengembalikan salinan yang dihapus. Galat:
	// ErrTidakDitemukan, ErrAdaUsulan (perubahan_master MENUNGGU untuk entitas itu), ErrDirujuk (FK 23503).
	HapusKomoditas(ctx context.Context, kode string) (Komoditas, error)
	HapusPasar(ctx context.Context, kode string) (Pasar, error)
}

// PilihVersiPada memilih versi yang berlaku pada waktu pada dari riwayat terbaru-dulu: berlaku_dari terakhir yang
// <= pada, atau versi paling awal bila pada lebih tua dari seluruh riwayat. ok false hanya bila riwayat kosong.
func PilihVersiPada(riwayat []KomoditasVersiRef, pada time.Time) (KomoditasVersiRef, bool) {
	if len(riwayat) == 0 {
		return KomoditasVersiRef{}, false
	}
	for _, v := range riwayat {
		if !v.BerlakuDari.After(pada) {
			return v, true
		}
	}
	return riwayat[len(riwayat)-1], true
}
