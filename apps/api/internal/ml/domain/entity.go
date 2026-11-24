// Package domain berisi model baca hasil pipeline data dan ML (MongoDB `dataunseen` dan `datasafe`) serta keadaan
// jembatan EWS di PostgreSQL. Bentuk dokumen mengikuti kontrak docs/PIPELINE-DATA-ML.md §2-3; field yang boleh null
// di kontrak bertipe pointer supaya "tidak ada" tidak pernah terbaca sebagai nol.
package domain

import (
	"regexp"
	"strings"
	"time"
)

// Status siklus training (ml_training_cycles.status).
const (
	StatusBerjalan = "BERJALAN"
	StatusBerhasil = "BERHASIL"
	StatusSebagian = "SEBAGIAN"
	StatusGagal    = "GAGAL"
)

// StatusFinal adalah status siklus yang boleh dibaca sebagai hasil: siklus baru menjadi "terbaru" hanya setelah
// berstatus BERHASIL atau SEBAGIAN, jadi pembaca tidak pernah melihat siklus yang setengah tertulis.
var StatusFinal = []string{StatusBerhasil, StatusSebagian}

// Final melaporkan apakah status termasuk StatusFinal.
func Final(status string) bool { return status == StatusBerhasil || status == StatusSebagian }

// Target label inflasi.
const (
	TargetResmiBPS     = "RESMI_BPS"
	TargetIndeksPangan = "INDEKS_PANGAN"
)

// LabelTarget adalah sebutan yang wajib dipakai UI untuk target: indeks pangan tidak boleh disebut inflasi resmi.
func LabelTarget(target string) string {
	switch target {
	case TargetResmiBPS:
		return "Inflasi resmi BPS"
	case TargetIndeksPangan:
		return "Indeks harga pangan PIHPS (bukan inflasi resmi)"
	}
	return ""
}

// Pemicu crawl dan awalan pesan adapter yang sengaja dilewati.
const (
	PemicuBackfill = "BACKFILL"
	AwalanDilewati = "DILEWATI:"
)

// Status sumber (crawl_runs.status, raw_*.source_status).
const (
	SumberFresh = "FRESH"
	SumberStale = "STALE_CARRIED_FORWARD"
	SumberError = "ERROR"
)

// Jenis sinyal EWS pipeline (datasafe_ews_signals.jenis).
const (
	SinyalAnomaliHarga     = "ANOMALI_HARGA"
	SinyalProyeksiLonjakan = "PROYEKSI_LONJAKAN"
	SinyalTekananInflasi   = "TEKANAN_INFLASI"
	SinyalSumberData       = "SUMBER_DATA"
)

var polaCycleID = regexp.MustCompile(`^C\d{8}T\d{2}Z$`)

// CycleIDValid melaporkan apakah id berformat C{YYYYMMDD}T{HH}Z.
func CycleIDValid(id string) bool { return polaCycleID.MatchString(id) }

// Dilewati melaporkan apakah pesan menandai adapter yang sengaja tidak dijalankan.
func Dilewati(pesan string) bool { return strings.HasPrefix(strings.TrimSpace(pesan), AwalanDilewati) }

// Kesegaran adalah satu entri ml_training_cycles.source_freshness[].
type Kesegaran struct {
	Sumber            string     `bson:"sumber"`
	Dataset           string     `bson:"dataset"`
	Status            *string    `bson:"status"`
	LastAvailableDate *time.Time `bson:"last_available_date"`
	CrawledAt         *time.Time `bson:"crawled_at"`
	UmurHari          *float64   `bson:"umur_hari"`
	Pesan             string     `bson:"pesan"`
}

// Kelayakan klasifikasi inflasi (docs/PIPELINE-DATA-ML.md, "Tambahan yang disahkan"). Dokumen lama tanpa field
// `layak` berkelayakan TIDAK_DIKETAHUI: tidak pernah dianggap layak.
const (
	KelayakanLayak          = "LAYAK"
	KelayakanTidakLayak     = "TIDAK_LAYAK"
	KelayakanTidakDiketahui = "TIDAK_DIKETAHUI"
)

// Kelayakan memetakan field `layak` (boleh tidak ada) ke status kelayakan.
func Kelayakan(layak *bool) string {
	switch {
	case layak == nil:
		return KelayakanTidakDiketahui
	case *layak:
		return KelayakanLayak
	}
	return KelayakanTidakLayak
}

// MetrikKlasifikasi adalah model_metrics.klasifikasi. Layak nil berarti dokumen format lama (tidak diketahui).
type MetrikKlasifikasi struct {
	Model         *string  `bson:"model"`
	F1            *float64 `bson:"f1"`
	RocAuc        *float64 `bson:"roc_auc"`
	Brier         *float64 `bson:"brier"`
	BrierBaseline *float64 `bson:"brier_baseline"`
	NUji          *float64 `bson:"n_uji"`
	Kalibrasi     *string  `bson:"kalibrasi"`
	Layak         *bool    `bson:"layak"`
	Alasan        *string  `bson:"alasan"`
}

// MetrikRegresi adalah model_metrics.regresi.
type MetrikRegresi struct {
	Pemenang            map[string]float64 `bson:"pemenang"`
	MapeMedian          *float64           `bson:"mape_median"`
	RmseMedian          *float64           `bson:"rmse_median"`
	MapeBaselineMedian  *float64           `bson:"mape_baseline_median"`
	Tingkat             *string            `bson:"tingkat"`
	ModelGlobal         *string            `bson:"model_global"`
	SeleksiCycleID      *string            `bson:"seleksi_cycle_id"`
	NSeri               *float64           `bson:"n_seri"`
	MengalahkanBaseline *float64           `bson:"mengalahkan_baseline"`
	NDievaluasiHoldout  *float64           `bson:"n_dievaluasi_holdout"`
	NDilewati           *float64           `bson:"n_dilewati"`
	NGagal              *float64           `bson:"n_gagal"`
}

// MetrikModel adalah ml_training_cycles.model_metrics.
type MetrikModel struct {
	Klasifikasi    *MetrikKlasifikasi `bson:"klasifikasi"`
	Regresi        *MetrikRegresi     `bson:"regresi"`
	JumlahDataBaru *float64           `bson:"jumlah_data_baru"`
}

// Siklus adalah satu dokumen ml_training_cycles.
type Siklus struct {
	CycleID         string             `bson:"cycle_id"`
	CycleTimestamp  time.Time          `bson:"cycle_timestamp"`
	DataCutoff      *time.Time         `bson:"data_cutoff"`
	DurationMS      *float64           `bson:"duration_ms"`
	Status          string             `bson:"status"`
	Pesan           string             `bson:"pesan"`
	VersiKode       string             `bson:"versi_kode"`
	Jumlah          map[string]float64 `bson:"jumlah"`
	SourceFreshness []Kesegaran        `bson:"source_freshness"`
	TargetInflasi   *string            `bson:"target_inflasi"`
	ModelMetrics    *MetrikModel       `bson:"model_metrics"`
}

// Nilai3 adalah tiga ukuran inflasi (yoy, mtm, ytd); masing-masing boleh null.
type Nilai3 struct {
	Yoy     *float64   `bson:"yoy"`
	Mtm     *float64   `bson:"mtm"`
	Ytd     *float64   `bson:"ytd"`
	Periode *time.Time `bson:"periode"`
}

// Penyumbang adalah satu entri penyumbang[] sinyal inflasi.
type Penyumbang struct {
	KomoditasKode string   `bson:"komoditas_kode"`
	Nama          string   `bson:"nama"`
	Andil         *float64 `bson:"andil"`
}

// Kalibrasi adalah metode kalibrasi probabilitas.
type Kalibrasi struct {
	Metode *string  `bson:"metode"`
	Brier  *float64 `bson:"brier"`
}

// SinyalInflasi adalah satu dokumen datasafe_inflasi_signals (siklus x wilayah). Kelas null bila klasifikasi tidak
// layak; Layak nil berarti dokumen format lama (kelayakan tidak diketahui).
type SinyalInflasi struct {
	CycleID        string       `bson:"cycle_id"`
	CycleTimestamp time.Time    `bson:"cycle_timestamp"`
	WilayahKode    string       `bson:"wilayah_kode"`
	Periode        *time.Time   `bson:"periode"`
	Kelas          *string      `bson:"kelas"`
	Layak          *bool        `bson:"layak"`
	Alasan         *string      `bson:"alasan"`
	Probabilitas   *float64     `bson:"probabilitas"`
	SkorTekanan    *float64     `bson:"skor_tekanan"`
	Siklus         string       `bson:"siklus"`
	Target         string       `bson:"target"`
	Proyeksi       Nilai3       `bson:"proyeksi"`
	Aktual         Nilai3       `bson:"aktual"`
	Penyumbang     []Penyumbang `bson:"penyumbang"`
	Kalibrasi      Kalibrasi    `bson:"kalibrasi"`
}

// ModelForecast adalah datasafe_forecasts.model.
type ModelForecast struct {
	Nama      string   `bson:"nama"`
	Fallback  bool     `bson:"fallback"`
	Alasan    *string  `bson:"alasan"`
	NKandidat *float64 `bson:"n_kandidat"`
}

// MetrikForecast adalah datasafe_forecasts.metrik. Baseline adalah nama baseline terbaik (naive atau seasonal_naive).
type MetrikForecast struct {
	RMSE                *float64  `bson:"rmse"`
	MAPE                *float64  `bson:"mape"`
	Baseline            *string   `bson:"baseline"`
	MapeBaseline        *float64  `bson:"mape_baseline"`
	MengalahkanBaseline *bool     `bson:"mengalahkan_baseline"`
	Evaluasi            *string   `bson:"evaluasi"`
	NLipatan            *float64  `bson:"n_lipatan"`
	NLipatanSeleksi     *float64  `bson:"n_lipatan_seleksi"`
	Horizon             []float64 `bson:"horizon"`
}

// TitikKurva adalah satu titik kurva kuantil forecast.
type TitikKurva struct {
	Tanggal time.Time `bson:"tanggal"`
	Horizon int       `bson:"horizon"`
	P10     *float64  `bson:"p10"`
	P50     *float64  `bson:"p50"`
	P90     *float64  `bson:"p90"`
}

// TitikRiwayat adalah satu titik harga asli yang dipakai pelatihan.
type TitikRiwayat struct {
	Tanggal time.Time `bson:"tanggal"`
	Nilai   *float64  `bson:"nilai"`
}

// Forecast adalah satu dokumen datasafe_forecasts (siklus x komoditas x kabupaten). Riwayat hanya terisi pada
// bacaan detail (Repository.Forecast); bacaan snapshot membuangnya.
type Forecast struct {
	CycleID         string         `bson:"cycle_id"`
	CycleTimestamp  time.Time      `bson:"cycle_timestamp"`
	KomoditasKode   string         `bson:"komoditas_kode"`
	KabupatenKode   string         `bson:"kabupaten_kode"`
	HargaTerakhir   *float64       `bson:"harga_terakhir"`
	TanggalTerakhir *time.Time     `bson:"tanggal_terakhir"`
	Satuan          string         `bson:"satuan"`
	Model           ModelForecast  `bson:"model"`
	Metrik          MetrikForecast `bson:"metrik"`
	Kurva           []TitikKurva   `bson:"kurva"`
	Riwayat         []TitikRiwayat `bson:"riwayat"`
}

// FiturXai adalah satu fitur penyusun faktor.
type FiturXai struct {
	Nama       string   `bson:"nama"`
	Kontribusi *float64 `bson:"kontribusi"`
}

// FaktorXai adalah satu kelompok faktor kebijakan (CUACA, PASOKAN, ...).
type FaktorXai struct {
	Kelompok   string     `bson:"kelompok"`
	Label      string     `bson:"label"`
	Kontribusi *float64   `bson:"kontribusi"`
	Arah       string     `bson:"arah"`
	Fitur      []FiturXai `bson:"fitur"`
}

// Target penjelasan XAI.
const (
	XaiForecast = "forecast"
	XaiInflasi  = "inflasi"
)

// Xai adalah satu dokumen datasafe_xai_explanations. SatuanKontribusi: poin_persen_harga (forecast) atau
// poin_persen_probabilitas (inflasi); nil pada dokumen format lama.
type Xai struct {
	CycleID          string      `bson:"cycle_id"`
	CycleTimestamp   time.Time   `bson:"cycle_timestamp"`
	Target           string      `bson:"target"`
	KomoditasKode    *string     `bson:"komoditas_kode"`
	WilayahKode      string      `bson:"wilayah_kode"`
	Metode           string      `bson:"metode"`
	SatuanKontribusi *string     `bson:"satuan_kontribusi"`
	Faktor           []FaktorXai `bson:"faktor"`
	Narasi           string      `bson:"narasi"`
}

// SinyalEws adalah satu dokumen datasafe_ews_signals.
type SinyalEws struct {
	CycleID        string    `bson:"cycle_id"`
	CycleTimestamp time.Time `bson:"cycle_timestamp"`
	SinyalKunci    string    `bson:"sinyal_kunci"`
	Severity       string    `bson:"severity"`
	Jenis          string    `bson:"jenis"`
	KomoditasKode  *string   `bson:"komoditas_kode"`
	KabupatenKode  *string   `bson:"kabupaten_kode"`
	Nilai          *float64  `bson:"nilai"`
	Ambang         *float64  `bson:"ambang"`
	Metode         *string   `bson:"metode"`
	Judul          string    `bson:"judul"`
	Pesan          string    `bson:"pesan"`
	Aktif          bool      `bson:"aktif"`
}

// MenjadiAlert melaporkan apakah jembatan EWS menjadikan sinyal ini alert: hanya sinyal aktif bertingkat WASPADA,
// SIAGA, atau KRITIS. INFO (termasuk SUMBER_DATA untuk sumber yang sengaja dilewati) dan sinyal pulih tidak pernah.
func (s SinyalEws) MenjadiAlert() bool {
	return s.Aktif && (s.Severity == "WASPADA" || s.Severity == "SIAGA" || s.Severity == "KRITIS")
}

// Makro adalah satu dokumen datasafe_macro_indicators.
type Makro struct {
	CycleID           string     `bson:"cycle_id"`
	Domain            string     `bson:"domain"`
	Indikator         string     `bson:"indikator"`
	WilayahKode       *string    `bson:"wilayah_kode"`
	Periode           *time.Time `bson:"periode"`
	Nilai             *float64   `bson:"nilai"`
	Satuan            string     `bson:"satuan"`
	Perubahan         *float64   `bson:"perubahan"`
	Sumber            string     `bson:"sumber"`
	IsCarriedForward  bool       `bson:"is_carried_forward"`
	LastAvailableDate *time.Time `bson:"last_available_date"`
}

// KlasterWilayah adalah satu dokumen datasafe_region_clusters (atau baris PostgreSQL region_clusters).
type KlasterWilayah struct {
	CycleID              string   `bson:"cycle_id"`
	RegencyCode          string   `bson:"regency_code"`
	ClusterID            int      `bson:"cluster_id"`
	ClusterLabel         string   `bson:"cluster_label"`
	CentroidDistance     float64  `bson:"centroid_distance"`
	VolatilityScore      float64  `bson:"volatility_score"`
	ClimateExposureScore float64  `bson:"climate_exposure_score"`
	SilhouetteScore      *float64 `bson:"silhouette_score,omitempty"`
}

// CrawlRun adalah satu dokumen crawl_runs.
type CrawlRun struct {
	RunID             string     `bson:"_id"`
	Crawler           string     `bson:"crawler"`
	Sumber            string     `bson:"sumber"`
	Dataset           string     `bson:"dataset"`
	CrawlDate         string     `bson:"crawl_date"`
	Mulai             *time.Time `bson:"mulai"`
	Selesai           *time.Time `bson:"selesai"`
	Status            string     `bson:"status"`
	JumlahBaru        *float64   `bson:"jumlah_baru"`
	JumlahDiteruskan  *float64   `bson:"jumlah_diteruskan"`
	LastAvailableDate *time.Time `bson:"last_available_date"`
	Pesan             string     `bson:"pesan"`
	Pemicu            string     `bson:"pemicu"`
}

// RunSumber merangkum run terakhir satu (sumber, dataset): Terjadwal adalah run terakhir non-BACKFILL (dasar
// penilaian kesegaran), Backfill adalah run BACKFILL terakhir (hanya informasi).
type RunSumber struct {
	Sumber    string
	Dataset   string
	Terjadwal *CrawlRun
	Backfill  *CrawlRun
}

// Snapshot adalah seluruh hasil satu siklus final yang dibutuhkan pembaca API, tanpa riwayat harga forecast.
// Isi siklus tidak pernah berubah (append-only), jadi snapshot aman di-cache per cycle_id.
type Snapshot struct {
	Siklus   Siklus
	Inflasi  []SinyalInflasi
	Forecast []Forecast
	Xai      []Xai
	Ews      []SinyalEws
	Makro    []Makro
	Klaster  []KlasterWilayah
}
