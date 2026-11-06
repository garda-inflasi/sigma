package domain

import (
	"context"
	"errors"
	"time"
)

// IzinPengelolaan adalah izin yang menjaga sistem tetap bisa diurus: selalu harus ada pengguna aktif yang
// memegangnya lewat perannya (lihat ErrPemegangTerakhir).
const IzinPengelolaan = "pengguna.kelola"

var (
	// ErrPemegangTerakhir: perubahan akan membuat sistem kehilangan pemegang terakhir izin pengelolaan
	// (matriks peran, ganti peran, penonaktifan, penghapusan). Tidak ada yang ditulis.
	ErrPemegangTerakhir = errors.New("tidak boleh menghapus pemegang terakhir izin pengguna.kelola")
	// ErrVersiPeranBerubah: matriks peran sudah diubah pihak lain sejak dibaca; tidak ada yang ditimpa.
	ErrVersiPeranBerubah = errors.New("matriks peran sudah diubah pihak lain, muat ulang")
	// ErrIzinTidakDikenal: nama izin tidak ada di katalog.
	ErrIzinTidakDikenal = errors.New("izin tidak dikenal")
	// ErrIzinDariPeran: pengguna sudah memiliki izin itu lewat perannya.
	ErrIzinDariPeran = errors.New("pengguna sudah memiliki izin ini lewat perannya")
	// ErrIzinLangsungAda: pengguna sudah punya izin langsung yang masih berlaku untuk izin itu.
	ErrIzinLangsungAda = errors.New("izin langsung ini sudah diberikan dan masih berlaku")
	// ErrIzinLangsungTidakAda: tidak ada izin langsung untuk dicabut.
	ErrIzinLangsungTidakAda = errors.New("izin langsung tidak ditemukan")
)

// Peran memetakan tabel roles: satu dari 14 nilai enum peran. Versi naik setiap matriksnya diubah.
type Peran struct {
	Kunci          string    `gorm:"column:kunci;primaryKey"`
	Label          string    `gorm:"column:label"`
	Lapis          string    `gorm:"column:lapis"`
	Urutan         int       `gorm:"column:urutan"`
	Versi          int64     `gorm:"column:versi;default:1"`
	DiperbaruiPada time.Time `gorm:"column:diperbarui_pada;default:now()"`
	DiperbaruiOleh *string   `gorm:"column:diperbarui_oleh;type:uuid"`
}

func (Peran) TableName() string { return "roles" }

// Izin memetakan tabel permissions: katalog izin domain.aksi.
type Izin struct {
	Nama      string `gorm:"column:nama;primaryKey"`
	Deskripsi string `gorm:"column:deskripsi"`
	Kelompok  string `gorm:"column:kelompok"`
}

func (Izin) TableName() string { return "permissions" }

// PeranIzin memetakan tabel role_permissions: satu izin milik satu peran.
type PeranIzin struct {
	RoleKunci      string `gorm:"column:role_kunci;primaryKey"`
	PermissionNama string `gorm:"column:permission_nama;primaryKey"`
}

func (PeranIzin) TableName() string { return "role_permissions" }

// IzinLangsung memetakan tabel user_permissions: izin yang diberikan kepada satu pengguna di luar perannya.
type IzinLangsung struct {
	PenggunaID     string     `gorm:"column:pengguna_id;type:uuid;primaryKey"`
	PermissionNama string     `gorm:"column:permission_nama;primaryKey"`
	BerlakuSampai  *time.Time `gorm:"column:berlaku_sampai"`
	Alasan         string     `gorm:"column:alasan"`
	DiberikanOleh  *string    `gorm:"column:diberikan_oleh;type:uuid"`
	DiberikanPada  time.Time  `gorm:"column:diberikan_pada;default:now()"`
}

func (IzinLangsung) TableName() string { return "user_permissions" }

// Aktif true bila izin langsung masih berlaku pada saat sekarang.
func (g IzinLangsung) Aktif(sekarang time.Time) bool {
	return g.BerlakuSampai == nil || g.BerlakuSampai.After(sekarang)
}

// IzinEfektif adalah izin satu pengguna aktif pada saat dibaca: izin peran digabung izin langsung yang masih
// berlaku. KedaluwarsaDalam adalah sisa waktu (dihitung oleh jam basis data, bukan jam aplikasi) sampai izin
// langsung berlaku terdekat habis; nol bila tidak ada yang akan habis. Hasil tidak boleh dipakai lewat saat itu.
type IzinEfektif struct {
	Peran            string
	Izin             []string
	KedaluwarsaDalam time.Duration
}

// PeranDetail adalah satu peran beserta izin yang dimilikinya saat ini.
type PeranDetail struct {
	Peran
	Izin []string
}

// SetIzinPeran adalah permintaan mengganti seluruh izin satu peran dengan kunci optimistis Versi.
type SetIzinPeran struct {
	Peran string
	// Versi adalah versi peran yang dibaca klien; berbeda dengan versi saat ini berarti ErrVersiPeranBerubah.
	Versi int64
	Izin  []string
	Oleh  string
}

// HasilSetIzinPeran melaporkan hasil penggantian: izin sebelum/sesudah dan versi baru. Berubah false bila
// himpunannya sama (tidak ada yang ditulis, versi tidak naik).
type HasilSetIzinPeran struct {
	Peran          Peran
	Sebelum        []string
	Sesudah        []string
	Berubah        bool
	DiperbaruiPada time.Time
}

// BerikanIzin adalah permintaan memberi izin langsung. BerlakuSampai nil berarti tanpa batas.
type BerikanIzin struct {
	PenggunaID    string
	Izin          string
	BerlakuSampai *time.Time
	Alasan        string
	Oleh          string
}

// IzinLangsungDetail adalah satu izin langsung beserta metadata katalog dan nama pemberi.
type IzinLangsungDetail struct {
	IzinLangsung
	Deskripsi         string
	Kelompok          string
	DiberikanOlehNama *string
}

// IzinRepository adalah penyimpanan RBAC berbasis basis data (Opsi B). Repositori produksi
// (repository/postgres) mengimplementasikannya selain Repository; UseCase memakainya bila tersedia.
type IzinRepository interface {
	// IzinEfektifPengguna membaca izin efektif satu pengguna AKTIF. gorm.ErrRecordNotFound bila pengguna tidak
	// ada atau tidak aktif.
	IzinEfektifPengguna(ctx context.Context, userID string) (*IzinEfektif, error)
	// ListPeran mengembalikan semua peran beserta izinnya, berurutan menurut Urutan.
	ListPeran(ctx context.Context) ([]PeranDetail, error)
	// ListIzin mengembalikan katalog izin (kelompok, lalu nama).
	ListIzin(ctx context.Context) ([]Izin, error)
	// SetIzinPeran mengganti seluruh izin satu peran dalam satu transaksi: kunci versi optimistis
	// (ErrVersiPeranBerubah), nama izin harus ada di katalog (ErrIzinTidakDikenal), dan sistem tidak boleh
	// kehilangan pemegang terakhir IzinPengelolaan (ErrPemegangTerakhir). gorm.ErrRecordNotFound bila peran tidak ada.
	SetIzinPeran(ctx context.Context, p SetIzinPeran) (*HasilSetIzinPeran, error)
	// ListIzinLangsung mengembalikan semua izin langsung satu pengguna (aktif dan kedaluwarsa), terbaru dulu, dan
	// izin yang dimiliki lewat perannya. gorm.ErrRecordNotFound bila pengguna tidak ada.
	ListIzinLangsung(ctx context.Context, userID string) (*IzinPengguna, error)
	// BerikanIzinLangsung menyimpan izin langsung. Menolak izin yang sudah dimiliki lewat peran (ErrIzinDariPeran)
	// atau yang masih berlaku sebagai izin langsung (ErrIzinLangsungAda); baris kedaluwarsa diganti dan
	// dikembalikan sebagai sebelumnya. gorm.ErrRecordNotFound bila pengguna tidak ada.
	BerikanIzinLangsung(ctx context.Context, g BerikanIzin) (sesudah *IzinLangsungDetail, sebelumnya *IzinLangsung, err error)
	// CabutIzinLangsung menghapus izin langsung (aktif maupun kedaluwarsa) dan mengembalikan barisnya.
	// ErrIzinLangsungTidakAda bila tidak ada.
	CabutIzinLangsung(ctx context.Context, userID, izin string) (*IzinLangsung, error)
	// HitungPemegangPengelolaan menghitung pengguna AKTIF yang memegang IzinPengelolaan lewat perannya.
	HitungPemegangPengelolaan(ctx context.Context) (int64, error)
}

// IzinPengguna adalah ringkasan izin seorang pengguna untuk layar administrasi.
type IzinPengguna struct {
	Peran     string
	IzinPeran []string
	Langsung  []IzinLangsungDetail
}
