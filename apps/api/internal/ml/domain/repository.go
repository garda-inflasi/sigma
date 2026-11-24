package domain

import (
	"context"
	"errors"
	"time"

	"github.com/garda-inflasi/api/pkg/idx"
)

// ErrTidakDitemukan: dokumen yang diminta tidak ada.
var ErrTidakDitemukan = errors.New("data pipeline ML tidak ditemukan")

// ListSiklusQuery menyaring riwayat siklus. Status kosong berarti semua status; Dari/Sampai nil berarti tanpa batas
// (rentang memakai cycle_timestamp, Sampai eksklusif).
type ListSiklusQuery struct {
	Status      []string
	Dari        *time.Time
	Sampai      *time.Time
	Page, Limit int
}

// Repository adalah akses BACA ke MongoDB pipeline. Tidak ada metode tulis: API hanya membaca dataunseen dan
// datasafe (docs/PIPELINE-DATA-ML.md §1).
type Repository interface {
	// SiklusTerbaru mengembalikan siklus final (BERHASIL/SEBAGIAN) dengan cycle_timestamp terbaru;
	// ErrTidakDitemukan bila belum ada.
	SiklusTerbaru(ctx context.Context) (*Siklus, error)
	// Siklus mengembalikan satu siklus apa pun statusnya; ErrTidakDitemukan bila tidak ada.
	Siklus(ctx context.Context, cycleID string) (*Siklus, error)
	// ListSiklus mengembalikan satu halaman siklus (terbaru dahulu) dan jumlah seluruh hasil.
	ListSiklus(ctx context.Context, q ListSiklusQuery) ([]Siklus, int64, error)
	// Snapshot membaca seluruh hasil satu siklus (forecast tanpa riwayat). Pemanggil wajib memastikan siklus final.
	Snapshot(ctx context.Context, s Siklus) (*Snapshot, error)
	// Forecast membaca satu forecast lengkap dengan riwayat; ErrTidakDitemukan bila tidak ada.
	Forecast(ctx context.Context, cycleID, komoditasKode, kabupatenKode string) (*Forecast, error)
	// RunSumber merangkum run terakhir tiap (sumber, dataset) di crawl_runs, terjadwal dan backfill terpisah.
	RunSumber(ctx context.Context) ([]RunSumber, error)
	// Ping memeriksa Mongo.
	Ping(ctx context.Context) error
}

// EwsSinyalML memetakan tabel ews_sinyal_ml: keadaan jembatan EWS per sinyal_kunci (000131).
type EwsSinyalML struct {
	SinyalKunci    string    `gorm:"column:sinyal_kunci;primaryKey"`
	JenisML        string    `gorm:"column:jenis_ml"`
	Severity       string    `gorm:"column:severity"`
	Aktif          bool      `gorm:"column:aktif"`
	CycleID        string    `gorm:"column:cycle_id"`
	CycleTimestamp time.Time `gorm:"column:cycle_timestamp"`
	AlertID        *idx.ID   `gorm:"column:alert_id"`
	DiperbaruiPada time.Time `gorm:"column:diperbarui_pada;default:now()"`
}

func (EwsSinyalML) TableName() string { return "ews_sinyal_ml" }

// TautanAlert adalah alert yang terhubung ke satu sinyal_kunci beserta status terkininya.
type TautanAlert struct {
	SinyalKunci string  `gorm:"column:sinyal_kunci"`
	AlertID     idx.ID  `gorm:"column:alert_id"`
	AlertKode   *string `gorm:"column:alert_kode"`
	Status      string  `gorm:"column:status"`
}

// PostgresRepository adalah akses PostgreSQL modul ML: cakupan baris pemanggil, tautan sinyal ke alert, dan keadaan
// jembatan EWS.
type PostgresRepository interface {
	// PenggunaCakupan membaca peran dan kabupaten tugas pengguna (02-B).
	PenggunaCakupan(ctx context.Context, userID string) (peran, kabupatenKode string, ditemukan bool, err error)
	// TautanAlert mengembalikan alert per sinyal_kunci yang sudah dijembatani.
	TautanAlert(ctx context.Context, kunci []string) (map[string]TautanAlert, error)
}
