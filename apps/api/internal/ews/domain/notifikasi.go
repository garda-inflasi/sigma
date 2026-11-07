package domain

import (
	"context"
	"errors"
	"time"

	"github.com/garda-inflasi/api/pkg/idx"
)

// Kanal notifikasi. DASHBOARD dilayani aliran SSE, bukan outbox.
const (
	KanalDashboard = "DASHBOARD"
	KanalEmail     = "EMAIL"
	KanalPush      = "PUSH"
	KanalWhatsApp  = "WHATSAPP"
)

// Status pengiriman notifikasi (outbox ews_notifikasi).
const (
	NotifMenunggu = "MENUNGGU"
	NotifTerkirim = "TERKIRIM"
	NotifGagal    = "GAGAL"
	NotifDicatat  = "DICATAT"
)

// ErrTanpaTujuan: notifikasi tidak dapat dikirim sama sekali (mis. pengguna belum punya
// langganan push). Tidak dicoba ulang; baris outbox ditandai DICATAT.
var ErrTanpaTujuan = errors.New("tidak ada tujuan pengiriman")

// KanalOutbox adalah kanal yang masuk antrean pengiriman beserta nilai bawaan penerimaan.
var KanalOutbox = []string{KanalEmail, KanalPush}

// SeverityMinBawaan adalah tingkat minimum bila pengguna belum mengatur preferensi.
const SeverityMinBawaan = SeveritySiaga

// EwsNotifikasi memetakan tabel ews_notifikasi.
type EwsNotifikasi struct {
	ID            idx.ID     `gorm:"column:id;primaryKey;autoIncrement"`
	AlertID       idx.ID     `gorm:"column:alert_id"`
	PenggunaID    string     `gorm:"column:pengguna_id;type:uuid"`
	Kanal         string     `gorm:"column:kanal"`
	Tujuan        string     `gorm:"column:tujuan"`
	Status        string     `gorm:"column:status;default:MENUNGGU"`
	Percobaan     int        `gorm:"column:percobaan"`
	JadwalBerikut time.Time  `gorm:"column:jadwal_berikut;default:now()"`
	DikirimPada   *time.Time `gorm:"column:dikirim_pada"`
	Galat         *string    `gorm:"column:galat"`
	DibuatPada    time.Time  `gorm:"column:dibuat_pada;default:now()"`
}

func (EwsNotifikasi) TableName() string { return "ews_notifikasi" }

// PreferensiNotifikasi memetakan tabel preferensi_notifikasi.
type PreferensiNotifikasi struct {
	PenggunaID  string    `gorm:"column:pengguna_id;type:uuid;primaryKey"`
	Kanal       string    `gorm:"column:kanal;primaryKey"`
	SeverityMin string    `gorm:"column:severity_min"`
	Aktif       bool      `gorm:"column:aktif"`
	DiubahPada  time.Time `gorm:"column:diubah_pada;default:now()"`
}

func (PreferensiNotifikasi) TableName() string { return "preferensi_notifikasi" }

// LanggananPush memetakan tabel langganan_push (satu perangkat).
type LanggananPush struct {
	ID         int64     `gorm:"column:id;primaryKey;->"`
	PenggunaID string    `gorm:"column:pengguna_id;type:uuid"`
	Endpoint   string    `gorm:"column:endpoint"`
	P256dh     string    `gorm:"column:p256dh"`
	Auth       string    `gorm:"column:auth"`
	DibuatPada time.Time `gorm:"column:dibuat_pada;default:now()"`
}

func (LanggananPush) TableName() string { return "langganan_push" }

// Pesan adalah satu notifikasi siap kirim ke satu penerima.
type Pesan struct {
	NotifikasiID int64
	Kanal        string
	// Percobaan adalah jumlah percobaan sebelumnya.
	Percobaan  int
	PenggunaID string
	// Tujuan: alamat email untuk EMAIL; id pengguna untuk PUSH (langganan dicari adaptor).
	Tujuan   string
	AlertID  idx.ID
	Severity string
	Judul    string
	Isi      string
	// TautanPath adalah jalur halaman alert di aplikasi, mis. /ews?id=12.
	TautanPath string
}

// Pengirim adalah adaptor satu kanal. Kirim mengembalikan ErrTanpaTujuan bila tak ada
// tujuan sama sekali; galat lain dianggap sementara dan dicoba ulang.
type Pengirim interface {
	Kanal() string
	Kirim(ctx context.Context, p Pesan) error
}

// NotifikasiRepository adalah akses data antrean, preferensi, dan langganan push.
type NotifikasiRepository interface {
	// Antrekan memasukkan notifikasi alert untuk pengguna aktif dengan salah satu peran
	// yang memenuhi preferensi (kanal diizinkan aturan alert, tingkat >= minimum, aktif).
	// Idempoten per (alert, pengguna, kanal); mengembalikan jumlah baris baru.
	Antrekan(ctx context.Context, alertID idx.ID, peran []string, now time.Time) (int, error)
	// Klaim mengambil paling banyak limit notifikasi jatuh tempo dan menunda jadwalnya
	// sebesar lease agar tidak diambil pekerja lain selagi dikirim.
	Klaim(ctx context.Context, now time.Time, limit int, lease time.Duration) ([]Pesan, error)
	// Tandai menyimpan hasil satu pengiriman. jadwalBerikut hanya dipakai untuk status MENUNGGU.
	Tandai(ctx context.Context, id int64, status string, galat string, jadwalBerikut time.Time, now time.Time) error

	ListPreferensi(ctx context.Context, penggunaID string) ([]PreferensiNotifikasi, error)
	SimpanPreferensi(ctx context.Context, penggunaID string, prefs []PreferensiNotifikasi) error

	SimpanLangganan(ctx context.Context, l LanggananPush) error
	HapusLangganan(ctx context.Context, penggunaID, endpoint string) (bool, error)
	ListLangganan(ctx context.Context, penggunaID string) ([]LanggananPush, error)
	// HapusLanggananKadaluwarsa membuang langganan yang ditolak layanan push (404/410).
	HapusLanggananKadaluwarsa(ctx context.Context, endpoint string) error

	// EskalasiOtomatis mengeskalasi alert BARU berseverity >= minimum yang berumur lebih dari
	// batas dan mengembalikan id-nya (riwayat dicatat oleh sistem).
	EskalasiOtomatis(ctx context.Context, now time.Time, batas time.Duration, minimum string) ([]idx.ID, error)
}
