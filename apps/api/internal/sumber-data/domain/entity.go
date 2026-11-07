package domain

import (
	"time"

	"github.com/garda-inflasi/api/pkg/idx"
)

// Jenis sumber.
const (
	JenisAPI     = "API"
	JenisScraper = "SCRAPER"
)

// Status sumber (enum status_sumber). Sumber yang baru dikatalogkan berstatus
// StatusBelumAktif sampai pernah dijalankan.
const (
	StatusBelumAktif = "BELUM_AKTIF"
	StatusSehat      = "SEHAT"
	StatusTertunda   = "TERTUNDA"
	StatusGagal      = "GAGAL"
	StatusDijeda     = "DIJEDA"
)

// MaksKode adalah panjang kode sumber (kolom kode varchar(64)).
const MaksKode = 64

// SumberData memetakan tabel sumber_data: metadata sebuah sumber data eksternal
// (API resmi atau scraper). TerakhirJalan, Keberhasilan7h, dan LatensiMs kosong
// untuk sumber yang belum pernah dijalankan; sesudah itu dihitung dari eksekusi_sumber.
//
// ID adalah UUID (identitas di URL dan kunci asing). Kode (UNIQUE, NOT NULL) adalah kode
// tampilan manusia dan kunci konfigurasi: konektor ingestion, pemetaan seeder, dan CLI
// merujuk sumber lewat kode, mis. SRC-BI. Kode juga alias rute /v1/sumber-data/:id.
type SumberData struct {
	ID       string `gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()"`
	Kode     string `gorm:"column:kode"`
	Nama     string `gorm:"column:nama"`
	Penyedia string `gorm:"column:penyedia"`
	Jenis    string `gorm:"column:jenis"`
	Cakupan  string `gorm:"column:cakupan"`
	// JadwalCron dibaca dalam zona waktu WIB (Asia/Jakarta).
	JadwalCron    string     `gorm:"column:jadwal_cron"`
	Status        string     `gorm:"column:status"`
	TerakhirJalan *time.Time `gorm:"column:terakhir_jalan"`
	// Keberhasilan7h dalam persen 0–100; nil bila belum ada eksekusi.
	Keberhasilan7h *float64 `gorm:"column:keberhasilan_7h"`
	LatensiMs      int      `gorm:"column:latensi_ms"`
	RobotsTxtPatuh bool     `gorm:"column:robots_txt_patuh"`
	Atribusi       string   `gorm:"column:atribusi"`
	URLDasar       *string  `gorm:"column:url_dasar"`
	// Konfigurasi adalah parameter permintaan konektor (JSON, bukan rahasia);
	// kunci API dibaca dari variabel lingkungan, tidak disimpan di sini.
	Konfigurasi string `gorm:"column:konfigurasi;type:jsonb;default:{}"`
}

func (SumberData) TableName() string {
	return "sumber_data"
}

// BulkAction adalah aksi massal pada katalog sumber. Baru mengubah metadata
// status; efek nyata pada penjadwal ada di 05-B.
type BulkAction string

const (
	BulkJeda      BulkAction = "jeda"
	BulkLanjutkan BulkAction = "lanjutkan"
)

// Rujukan membaca teks (UUID atau kode) menjadi rujukan sumber: UUID kanonik dicari pada id, selain itu pada kode.
// Dipakai CLI dan engine ingestion yang menerima sumber lewat kode konfigurasi.
func Rujukan(teks string) idx.RujukanUUID {
	if id, ok := idx.NormalUUID(teks); ok {
		return idx.RujukanUUID{ID: id}
	}
	return idx.RujukanUUID{Kode: teks}
}
