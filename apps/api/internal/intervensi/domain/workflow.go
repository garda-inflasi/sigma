package domain

import (
	"context"
	"errors"
	"time"

	"github.com/garda-inflasi/api/pkg/idx"
)

// Jenis permintaan persetujuan (CHECK intervensi_persetujuan.jenis).
const (
	JenisPindahTahap = "PINDAH_TAHAP"
	JenisEskalasi    = "ESKALASI"
)

// Status permintaan persetujuan.
const (
	PersetujuanMenunggu  = "MENUNGGU"
	PersetujuanDisetujui = "DISETUJUI"
	PersetujuanDitolak   = "DITOLAK"
)

// Keputusan pada satu langkah.
const (
	KeputusanSetuju = "SETUJU"
	KeputusanTolak  = "TOLAK"
)

var (
	// ErrTahapTidakValid: tahap tidak terdefinisi untuk siklus kartu.
	ErrTahapTidakValid = errors.New("tahap tidak valid untuk siklus ini")
	// ErrKartuSelesai: kartu sudah ditutup, tidak ada yang perlu diajukan.
	ErrKartuSelesai = errors.New("kartu sudah selesai")
	// ErrTanpaPersetujuan: perpindahan tidak melewati tahap persetujuan (pindahkan langsung).
	ErrTanpaPersetujuan = errors.New("perpindahan ini tidak memerlukan persetujuan")
	// ErrTanpaAlur: tidak ada alur persetujuan aktif untuk sasaran ini.
	ErrTanpaAlur = errors.New("tidak ada alur persetujuan aktif")
	// ErrPersetujuanTerbuka: kartu sudah punya permintaan persetujuan yang menunggu.
	ErrPersetujuanTerbuka = errors.New("kartu masih memiliki permintaan persetujuan yang menunggu")
	// ErrPersetujuanNotFound: permintaan persetujuan tidak ada.
	ErrPersetujuanNotFound = errors.New("permintaan persetujuan tidak ditemukan")
	// ErrPersetujuanSelesai: permintaan sudah diputuskan, atau langkahnya sudah berpindah.
	ErrPersetujuanSelesai = errors.New("permintaan persetujuan sudah diputuskan atau langkahnya berubah")
)

// AlurPersetujuan memetakan tabel alur_persetujuan: definisi alur persetujuan sebagai data.
type AlurPersetujuan struct {
	Kode        string  `gorm:"column:kode;primaryKey"`
	Nama        string  `gorm:"column:nama"`
	Jenis       string  `gorm:"column:jenis"`
	Siklus      *string `gorm:"column:siklus"`
	TahapTujuan *string `gorm:"column:tahap_tujuan"`
	Aktif       bool    `gorm:"column:aktif"`
}

func (AlurPersetujuan) TableName() string { return "alur_persetujuan" }

// AlurLangkah memetakan tabel alur_persetujuan_langkah.
type AlurLangkah struct {
	AlurKode       string `gorm:"column:alur_kode;primaryKey"`
	Urutan         int    `gorm:"column:urutan;primaryKey"`
	Label          string `gorm:"column:label"`
	IzinDibutuhkan string `gorm:"column:izin_dibutuhkan"`
}

func (AlurLangkah) TableName() string { return "alur_persetujuan_langkah" }

// Alur adalah definisi alur beserta langkah-langkahnya.
type Alur struct {
	AlurPersetujuan
	Langkah []AlurLangkah
}

// IntervensiPersetujuan memetakan tabel intervensi_persetujuan.
type IntervensiPersetujuan struct {
	ID           idx.ID     `gorm:"column:id;primaryKey;autoIncrement"`
	IntervensiID string     `gorm:"column:intervensi_id;type:uuid"`
	AlurKode     *string    `gorm:"column:alur_kode"`
	Jenis        string     `gorm:"column:jenis"`
	TahapTujuan  *string    `gorm:"column:tahap_tujuan"`
	Status       string     `gorm:"column:status;default:MENUNGGU"`
	LangkahKini  int        `gorm:"column:langkah_kini;default:1"`
	DiajukanOleh *string    `gorm:"column:diajukan_oleh;type:uuid"`
	DiajukanPada time.Time  `gorm:"column:diajukan_pada;default:now()"`
	SelesaiPada  *time.Time `gorm:"column:selesai_pada"`
	Catatan      string     `gorm:"column:catatan"`
}

func (IntervensiPersetujuan) TableName() string { return "intervensi_persetujuan" }

// PersetujuanLangkah memetakan tabel intervensi_persetujuan_langkah: satu langkah rantai
// persetujuan sebuah permintaan beserta keputusannya (kosong sampai diputuskan).
type PersetujuanLangkah struct {
	PersetujuanID  idx.ID     `gorm:"column:persetujuan_id;primaryKey"`
	Urutan         int        `gorm:"column:urutan;primaryKey"`
	Label          string     `gorm:"column:label"`
	IzinDibutuhkan string     `gorm:"column:izin_dibutuhkan"`
	Keputusan      *string    `gorm:"column:keputusan"`
	Oleh           *string    `gorm:"column:oleh;type:uuid"`
	Waktu          *time.Time `gorm:"column:waktu"`
	Catatan        string     `gorm:"column:catatan"`
}

func (PersetujuanLangkah) TableName() string { return "intervensi_persetujuan_langkah" }

// LangkahPersetujuan adalah langkah beserta nama pemutusnya.
type LangkahPersetujuan struct {
	PersetujuanLangkah
	OlehNama *string `gorm:"column:oleh_nama;->"`
}

// Persetujuan adalah permintaan persetujuan beserta rantai langkah dan ringkasan kartunya.
type Persetujuan struct {
	IntervensiPersetujuan
	KartuJudul     string               `gorm:"column:kartu_judul;->"`
	KartuTahap     string               `gorm:"column:kartu_tahap;->"`
	DiajukanOlehNm *string              `gorm:"column:diajukan_oleh_nama;->"`
	Langkah        []LangkahPersetujuan `gorm:"-"`
}

// IzinLangkahKini adalah izin yang dibutuhkan untuk memutuskan langkah yang sedang menunggu.
func (p Persetujuan) IzinLangkahKini() string {
	for _, l := range p.Langkah {
		if l.Urutan == p.LangkahKini {
			return l.IzinDibutuhkan
		}
	}
	return ""
}

// AjukanInput adalah masukan permintaan persetujuan. Oleh kosong berarti sistem.
type AjukanInput struct {
	// KartuID adalah UUID kartu.
	KartuID     string
	Jenis       string
	TahapTujuan string // wajib untuk JenisPindahTahap
	Oleh        string
	Catatan     string
}

// PutusInput adalah keputusan atas langkah LangkahKini sebuah permintaan.
type PutusInput struct {
	PersetujuanID idx.ID
	// LangkahKini adalah langkah yang dilihat pemutus; ditolak bila permintaan sudah bergerak.
	LangkahKini int
	Keputusan   string
	Oleh        string
	Catatan     string
}

// KartuMelewatiSLA adalah kartu aktif yang lebih lama di tahapnya daripada SLA tahap itu.
type KartuMelewatiSLA struct {
	ID        string `gorm:"column:id"` // UUID kartu
	TahapKode string `gorm:"column:tahap_kode"`
	Label     string `gorm:"column:label"`
	SLAHari   int    `gorm:"column:sla_hari"`
}

// KartuOtomatis adalah kartu yang dibuat workflow dari alert EWS. Tahap awal dan nomor
// ditentukan repository; pembuatnya sistem.
type KartuOtomatis struct {
	AlertID idx.ID
	// AlertKode adalah kode tampilan alert (EWS-n), dipakai pada teks riwayat kartu; boleh kosong.
	AlertKode                                                         string
	Siklus, Judul, Deskripsi, KomoditasKode, KabupatenKode, Prioritas string
	// HariTenggatCadangan dipakai bila tahap awal tidak punya SLA.
	HariTenggatCadangan int
	// PeranPenanggungJawab: pengguna aktif berperan ini ditugasi otomatis.
	PeranPenanggungJawab []string
	Sekarang             time.Time
}

// WorkflowRepository adalah akses data workflow persetujuan berjenjang dan otomasi.
type WorkflowRepository interface {
	ListAlur(ctx context.Context) ([]Alur, error)
	// Ajukan membuat permintaan persetujuan. Untuk JenisPindahTahap rantainya adalah gabungan
	// langkah alur tiap tahap persetujuan dari tahap sekarang (tidak termasuk) sampai tahap tujuan
	// (termasuk); ErrTanpaPersetujuan bila tak ada, ErrKartuSelesai, ErrPersetujuanTerbuka.
	Ajukan(ctx context.Context, in AjukanInput) (*Persetujuan, error)
	GetPersetujuan(ctx context.Context, id idx.ID) (*Persetujuan, error)
	// ListPersetujuan mengembalikan permintaan (terbaru dahulu); status kosong = semua.
	ListPersetujuan(ctx context.Context, status string) ([]Persetujuan, error)
	// Putus menyimpan keputusan. Setuju pada langkah terakhir menyelesaikan permintaan dan, untuk
	// pemindahan, memindahkan kartu ke tahap tujuan; tolak menutup permintaan. ErrPersetujuanSelesai
	// bila permintaan sudah diputuskan atau langkahnya bukan in.LangkahKini.
	Putus(ctx context.Context, in PutusInput) (*Persetujuan, error)

	// KartuMelewatiSLA mengembalikan kartu belum selesai dan belum dieskalasi yang melewati SLA tahap pada waktu now.
	KartuMelewatiSLA(ctx context.Context, now time.Time) ([]KartuMelewatiSLA, error)
	// EskalasiSLA mengeskalasi satu kartu atas nama sistem; false bila kartu tak lagi memenuhi syarat.
	EskalasiSLA(ctx context.Context, id, alasan string) (bool, error)
	// BuatKartuOtomatis membuat kartu dari alert; nil bila alert sudah punya kartu.
	BuatKartuOtomatis(ctx context.Context, in KartuOtomatis) (*Intervensi, error)
}
