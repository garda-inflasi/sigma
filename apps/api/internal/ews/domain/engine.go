package domain

import (
	"context"
	"errors"
	"time"

	"github.com/garda-inflasi/api/pkg/idx"
)

// Jenis alert (enum jenis_ews).
const (
	JenisLonjakanHarga  = "LONJAKAN_HARGA"
	JenisKelangkaanStok = "KELANGKAAN_STOK"
	JenisAnomaliCuaca   = "ANOMALI_CUACA"
	JenisOverSupply     = "OVER_SUPPLY"
)

// Tingkat keparahan (enum severity), dari rendah ke tinggi.
const (
	SeverityInfo    = "INFO"
	SeverityWaspada = "WASPADA"
	SeveritySiaga   = "SIAGA"
	SeverityKritis  = "KRITIS"
)

var (
	// ErrAturanNotFound: aturan ambang tidak ada.
	ErrAturanNotFound = errors.New("aturan ambang tidak ditemukan")
	// ErrVersiUsang: aturan sudah diubah pengguna lain sejak dibaca.
	ErrVersiUsang = errors.New("aturan telah diubah pengguna lain")
)

// SeverityRank mengurutkan tingkat keparahan; 0 untuk nilai yang tak dikenal.
func SeverityRank(severity string) int {
	switch severity {
	case SeverityInfo:
		return 1
	case SeverityWaspada:
		return 2
	case SeveritySiaga:
		return 3
	case SeverityKritis:
		return 4
	}
	return 0
}

// Pengukuran adalah hasil mengukur satu kondisi (komoditas x kabupaten) terhadap
// satu aturan. Nilai bertanda: penurunan stok bernilai negatif, seperti pada seed.
type Pengukuran struct {
	KomoditasKode *string
	KomoditasNama string
	KabupatenKode string
	KabupatenNama string
	Nilai         float64
	// Terbaru adalah angka terkini yang diukur: harga rata-rata (Rp), stok (ton),
	// atau curah hujan kumulatif (mm). SatuanKomoditas adalah satuan jual komoditas.
	Terbaru         float64
	SatuanKomoditas string
}

// NewAlert adalah alert yang hendak ditulis rule engine.
type NewAlert struct {
	AturanID      idx.ID
	Jenis         string
	Severity      string
	Judul         string
	Pesan         string
	KomoditasKode *string
	KabupatenKode string
	Nilai         float64
	SatuanNilai   string
	SkorDampak    int
	Sumber        string
	DedupKey      string
	CooldownJam   int
}

// HasilTulis menjelaskan apa yang terjadi saat NewAlert diterapkan.
type HasilTulis string

const (
	// HasilDibuat: alert baru tersimpan.
	HasilDibuat HasilTulis = "dibuat"
	// HasilDinaikkan: alert aktif untuk kondisi yang sama naik tingkat.
	HasilDinaikkan HasilTulis = "dinaikkan"
	// HasilDilewati: alert aktif sudah ada dengan tingkat yang sama atau lebih tinggi.
	HasilDilewati HasilTulis = "dilewati"
	// HasilDiredam: kondisi yang sama baru ditutup dan masih dalam masa peredaman.
	HasilDiredam HasilTulis = "diredam"
)

// RingkasanEvaluasi melaporkan satu putaran rule engine.
type RingkasanEvaluasi struct {
	Aturan    int `json:"aturan"`
	Diukur    int `json:"diukur"`
	Dibuat    int `json:"dibuat"`
	Dinaikkan int `json:"dinaikkan"`
	Dilewati  int `json:"dilewati"`
	Diredam   int `json:"diredam"`
	// Galat menghitung aturan atau kondisi yang gagal diproses (rinciannya ada di log).
	Galat int `json:"galat"`
}

// Evaluator menjalankan satu putaran rule engine.
type Evaluator interface {
	Evaluasi(ctx context.Context) (RingkasanEvaluasi, error)
}

// Observer diberi tahu setelah rule engine membuat alert baru atau menaikkan
// tingkat alert. Kegagalannya tidak membatalkan alert yang sudah tersimpan.
type Observer interface {
	AlertDitulis(ctx context.Context, alert Alert, hasil HasilTulis)
}

// EngineRepository adalah akses data yang dibutuhkan rule engine.
type EngineRepository interface {
	// AturanAktif mengembalikan aturan yang aktif, yang paling spesifik lebih dulu.
	AturanAktif(ctx context.Context) ([]AturanEws, error)
	// Ukur mengukur kondisi yang dicakup aturan pada waktu now dari data nyata
	// (entri harga tervalidasi, stok cold storage, cuaca harian).
	Ukur(ctx context.Context, aturan AturanEws, now time.Time) ([]Pengukuran, error)
	// Terapkan menulis alert secara idempoten menurut DedupKey dan peredaman.
	// Alert yang dikembalikan terisi hanya untuk HasilDibuat dan HasilDinaikkan.
	Terapkan(ctx context.Context, in NewAlert, now time.Time) (HasilTulis, *Alert, error)
}

// AturanUpdate adalah perubahan ambang. Versi adalah versi yang dibaca pengguna.
type AturanUpdate struct {
	Versi         int
	AmbangWaspada float64
	AmbangSiaga   float64
	AmbangKritis  float64
	JendelaHari   int
	CooldownJam   int
	Kanal         Kanal
	Aktif         bool
	Keterangan    string
}

// Riwayat adalah satu perubahan status alert beserta nama pelakunya.
type Riwayat struct {
	EwsRiwayat
	OlehNama *string `gorm:"column:oleh_nama;->"`
}
