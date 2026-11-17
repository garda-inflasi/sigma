package domain

import (
	"context"
	"errors"
	"time"
)

// Impersonasi peran dan akun (docs/impersonasi.md): superadmin, atau akun yang diizinkan kebijakan, sementara bekerja
// sebagai akun lain untuk memeriksa tampilan dan hak suatu peran. Seluruh aksi tetap dipertanggungjawabkan kepada pelaku
// asli lewat audit.

// PeranSuperadmin adalah peran superadmin (accounts.md: "Superadmin (Administrator Sistem)"). Selalu berhak memulai
// impersonasi, tidak pernah boleh menjadi sasaran, dan satu-satunya pemegang IzinImpersonasi.
const PeranSuperadmin = "ADMIN_SISTEM"

// IzinImpersonasi mengatur kebijakan dan riwayat impersonasi. Eksklusif milik PeranSuperadmin: ditegakkan di rute (izin
// dan peran), resolver izin efektif, editor matriks peran, dan pemberian izin langsung.
const IzinImpersonasi = "impersonasi.kelola"

// Jenis pihak pada kebijakan: satu peran, atau satu akun.
const (
	TipePeran    = "PERAN"
	TipePengguna = "PENGGUNA"
)

// Sebab sebuah impersonasi berakhir (impersonasi_sesi.alasan_berakhir).
const (
	AkhirStop        = "STOP"
	AkhirBerganti    = "BERGANTI"
	AkhirKedaluwarsa = "KEDALUWARSA"
	AkhirKebijakan   = "KEBIJAKAN"
	AkhirDiakhiri    = "DIAKHIRI"
	AkhirLogout      = "LOGOUT"
	AkhirDicabut     = "DICABUT"
)

// Kebijakan bawaan bila impersonasi_kebijakan belum pernah disimpan (migrasi tidak menyisipkan data).
const (
	BawaanImpersonasiAktif       = true
	BawaanImpersonasiWajibAlasan = true
	BawaanDurasiImpersonasiMenit = 60
)

// Batas masukan kebijakan.
const (
	MinDurasiImpersonasiMenit = 5
	MaxDurasiImpersonasiMenit = 1440
	MaxPelakuKebijakan        = 100
	MaxTargetPerPelaku        = 200
	MaxAlasanImpersonasi      = 500
)

var (
	// ErrVersiKebijakanBerubah: kebijakan sudah diubah pihak lain sejak dibaca; tidak ada yang ditimpa.
	ErrVersiKebijakanBerubah = errors.New("kebijakan impersonasi sudah diubah pihak lain, muat ulang")
	// ErrImpersonasiTidakAktif: catatan impersonasi tidak ada atau sudah berakhir.
	ErrImpersonasiTidakAktif = errors.New("sesi impersonasi tidak ada atau sudah berakhir")
)

// ── Tabel ─────────────────────────────────────────────────────────────

// ImpersonasiKebijakan memetakan impersonasi_kebijakan: satu baris (id = 1).
type ImpersonasiKebijakan struct {
	ID              int16     `gorm:"column:id;primaryKey"`
	Aktif           bool      `gorm:"column:aktif"`
	WajibAlasan     bool      `gorm:"column:wajib_alasan"`
	DurasiMaksMenit *int      `gorm:"column:durasi_maks_menit"`
	Versi           int64     `gorm:"column:versi;default:1"`
	DiperbaruiPada  time.Time `gorm:"column:diperbarui_pada;default:now()"`
	DiperbaruiOleh  *string   `gorm:"column:diperbarui_oleh;type:uuid"`
}

func (ImpersonasiKebijakan) TableName() string { return "impersonasi_kebijakan" }

// ImpersonasiPelaku memetakan impersonasi_pelaku: satu peran atau satu akun (selain superadmin) yang boleh memulai.
type ImpersonasiPelaku struct {
	ID         string    `gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()"`
	Tipe       string    `gorm:"column:tipe"`
	Peran      *string   `gorm:"column:peran"`
	PenggunaID *string   `gorm:"column:pengguna_id;type:uuid"`
	Aktif      bool      `gorm:"column:aktif"`
	DibuatPada time.Time `gorm:"column:dibuat_pada;default:now()"`
}

func (ImpersonasiPelaku) TableName() string { return "impersonasi_pelaku" }

// ImpersonasiTarget memetakan impersonasi_target: satu peran atau satu akun yang boleh menjadi sasaran satu pelaku.
type ImpersonasiTarget struct {
	ID         string  `gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()"`
	PelakuID   string  `gorm:"column:pelaku_id;type:uuid"`
	Tipe       string  `gorm:"column:tipe"`
	Peran      *string `gorm:"column:peran"`
	PenggunaID *string `gorm:"column:pengguna_id;type:uuid"`
}

func (ImpersonasiTarget) TableName() string { return "impersonasi_target" }

// ImpersonasiSesi memetakan impersonasi_sesi: satu impersonasi (riwayat sekaligus sumber kebenaran sisi server).
// Nama, email, dan peran pelaku dan sasaran adalah salinan saat dimulai.
type ImpersonasiSesi struct {
	ID              string     `gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()"`
	PelakuID        *string    `gorm:"column:pelaku_id;type:uuid"`
	PelakuNama      string     `gorm:"column:pelaku_nama"`
	PelakuEmail     string     `gorm:"column:pelaku_email"`
	PelakuPeran     string     `gorm:"column:pelaku_peran"`
	SasaranID       *string    `gorm:"column:sasaran_id;type:uuid"`
	SasaranNama     string     `gorm:"column:sasaran_nama"`
	SasaranEmail    string     `gorm:"column:sasaran_email"`
	SasaranPeran    string     `gorm:"column:sasaran_peran"`
	Alasan          *string    `gorm:"column:alasan"`
	FamilyID        string     `gorm:"column:family_id;type:uuid"`
	SesiAsalMulai   time.Time  `gorm:"column:sesi_asal_mulai"`
	MulaiPada       time.Time  `gorm:"column:mulai_pada;default:now()"`
	KedaluwarsaPada *time.Time `gorm:"column:kedaluwarsa_pada"`
	BerakhirPada    *time.Time `gorm:"column:berakhir_pada"`
	AlasanBerakhir  *string    `gorm:"column:alasan_berakhir"`
	IP              *string    `gorm:"column:ip"`
	UserAgent       *string    `gorm:"column:user_agent"`
}

func (ImpersonasiSesi) TableName() string { return "impersonasi_sesi" }

// ── Kebijakan ─────────────────────────────────────────────────────────

// Pengaturan adalah pengaturan global kebijakan impersonasi; nilai bawaan (Versi 0) bila belum pernah disimpan.
type Pengaturan struct {
	Aktif       bool
	WajibAlasan bool
	// DurasiMaksMenit nil berarti tanpa batas.
	DurasiMaksMenit    *int
	Versi              int64
	DiperbaruiPada     *time.Time
	DiperbaruiOleh     *string
	DiperbaruiOlehNama *string
}

// BawaanPengaturan adalah kebijakan yang berlaku sebelum superadmin pernah menyimpannya.
func BawaanPengaturan() Pengaturan {
	durasi := BawaanDurasiImpersonasiMenit
	return Pengaturan{Aktif: BawaanImpersonasiAktif, WajibAlasan: BawaanImpersonasiWajibAlasan, DurasiMaksMenit: &durasi}
}

// PihakKebijakan adalah satu pihak pada kebijakan: sebuah peran (Peran) atau sebuah akun (PenggunaID). Kolom Pengguna*
// hanya terisi saat dibaca (nama, email, dan peran akun terkini).
type PihakKebijakan struct {
	Tipe          string
	Peran         string
	PenggunaID    string
	PenggunaNama  string
	PenggunaEmail string
	PenggunaPeran string
}

// PelakuKebijakan adalah satu pelaku (peran atau akun) beserta sasaran yang diizinkan baginya.
type PelakuKebijakan struct {
	ID string
	PihakKebijakan
	Aktif  bool
	Target []PihakKebijakan
}

// SimpanKebijakan mengganti seluruh kebijakan (pengaturan, pelaku, dan sasaran) dalam satu transaksi. Versi adalah
// versi yang dibaca klien (0 bila belum pernah disimpan); berbeda dengan versi saat ini berarti ErrVersiKebijakanBerubah.
type SimpanKebijakan struct {
	Versi           int64
	Aktif           bool
	WajibAlasan     bool
	DurasiMaksMenit *int
	Pelaku          []PelakuKebijakan
	Oleh            string
}

// FilterAkunSasaran menyaring akun yang dapat menjadi sasaran. ADMIN_SISTEM dan KecualiID tidak pernah termasuk, dan
// hanya akun berstatus AKTIF. Semua true berarti seluruh akun lolos; selain itu akun harus berperan di Peran atau id-nya
// di PenggunaID.
type FilterAkunSasaran struct {
	Semua       bool
	Peran       []string
	PenggunaID  []string
	KecualiID   string
	HanyaPeran  string // bila terisi, hanya akun berperan ini (menyelesaikan target_peran)
	BatasJumlah int
}

// AkunSasaran adalah satu akun yang dapat diimpersonasi.
type AkunSasaran struct {
	ID            string  `gorm:"column:id"`
	Nama          string  `gorm:"column:nama"`
	Email         string  `gorm:"column:email"`
	Peran         string  `gorm:"column:peran"`
	Lembaga       string  `gorm:"column:lembaga"`
	KabupatenKode *string `gorm:"column:kabupaten_kode"`
}

// PenggunaRingkas adalah identitas singkat akun untuk validasi kebijakan.
type PenggunaRingkas struct {
	ID    string `gorm:"column:id"`
	Nama  string `gorm:"column:nama"`
	Email string `gorm:"column:email"`
	Peran string `gorm:"column:peran"`
}

// ── Sesi ──────────────────────────────────────────────────────────────

// MulaiImpersonasi menyimpan impersonasi baru dalam satu transaksi: catatan, refresh token pertamanya, dan pencabutan
// sesi yang digantikannya (sesi biasa pelaku, atau impersonasi sebelumnya bila mengganti sasaran).
type MulaiImpersonasi struct {
	Sesi  *ImpersonasiSesi
	Token *RefreshToken
	// Ganti terisi bila pelaku sedang impersonasi: catatan dan sesi itu diakhiri BERGANTI.
	Ganti *ImpersonasiSesi
	// CabutSesiAsal adalah family sesi biasa pelaku yang dicabut (kosong bila Ganti terisi).
	CabutSesiAsal string
	// PelakuID pemilik CabutSesiAsal.
	PelakuID string
	Waktu    time.Time
}

// AkhiriImpersonasi mengakhiri impersonasi dan mencabut sesinya. Pengganti (opsional) adalah refresh token sesi biasa
// pelaku yang disimpan dalam transaksi yang sama (berhenti dan mengembalikan pelaku ke akunnya).
type AkhiriImpersonasi struct {
	ID        string
	Alasan    string
	Waktu     time.Time
	Pengganti *RefreshToken
}

// FilterRiwayatImpersonasi adalah kriteria satu halaman riwayat. HanyaAktif menyaring impersonasi yang masih hidup.
type FilterRiwayatImpersonasi struct {
	HanyaAktif bool
	Limit      int
	After      *RiwayatCursor
}

// RiwayatImpersonasi adalah satu baris riwayat dengan keadaan efektif: Aktif dan Berakhir dihitung dari catatan dan sesi
// (family) refresh token, jadi impersonasi yang sesinya dicabut di luar alurnya (paksa keluar, ganti kata sandi sasaran)
// tampil berakhir DICABUT walau catatannya belum ditutup.
type RiwayatImpersonasi struct {
	ImpersonasiSesi
	Aktif              bool
	BerakhirEfektif    *time.Time
	AlasanAkhirEfektif *string
}

// ImpersonasiRepository adalah penyimpanan impersonasi. Repositori produksi (repository/postgres) mengimplementasikannya
// selain Repository; UseCase memakainya bila tersedia (nil pada tes unit lama).
type ImpersonasiRepository interface {
	// GetKebijakan membaca pengaturan; gorm.ErrRecordNotFound bila belum pernah disimpan (pakai BawaanPengaturan).
	GetKebijakan(ctx context.Context) (*Pengaturan, error)
	// ListPelaku mengembalikan seluruh pelaku beserta sasarannya dan identitas akun terkini (layar admin).
	ListPelaku(ctx context.Context) ([]PelakuKebijakan, error)
	// AturanPelaku mengembalikan pelaku AKTIF yang berlaku bagi akun: yang menunjuk akunnya, atau perannya.
	AturanPelaku(ctx context.Context, penggunaID, peran string) ([]PelakuKebijakan, error)
	// SimpanKebijakan mengganti seluruh kebijakan secara atomik dengan kunci versi optimistis.
	SimpanKebijakan(ctx context.Context, in SimpanKebijakan) (*Pengaturan, error)
	// RingkasPengguna membaca identitas singkat akun menurut id (yang tidak ada tidak dikembalikan).
	RingkasPengguna(ctx context.Context, ids []string) ([]PenggunaRingkas, error)
	// ListAkunSasaran mengembalikan akun AKTIF yang lolos filter, terurut peran lalu nama.
	ListAkunSasaran(ctx context.Context, f FilterAkunSasaran) ([]AkunSasaran, error)

	// MulaiImpersonasi menyimpan impersonasi baru secara atomik (lihat MulaiImpersonasi).
	MulaiImpersonasi(ctx context.Context, in MulaiImpersonasi) error
	// GetImpersonasi membaca satu catatan menurut id; gorm.ErrRecordNotFound bila tidak ada.
	GetImpersonasi(ctx context.Context, id string) (*ImpersonasiSesi, error)
	// AkhiriImpersonasi menutup catatan yang masih hidup dan mencabut family-nya. Mengembalikan catatan yang ditutup;
	// ErrImpersonasiTidakAktif bila tidak ada atau sudah berakhir (tidak ada yang ditulis).
	AkhiriImpersonasi(ctx context.Context, in AkhiriImpersonasi) (*ImpersonasiSesi, error)
	// ListRiwayatImpersonasi mengembalikan riwayat, terbaru dulu.
	ListRiwayatImpersonasi(ctx context.Context, f FilterRiwayatImpersonasi, now time.Time) ([]RiwayatImpersonasi, error)
}
