// Package dto adalah bentuk JSON hasil pipeline ML (docs/PIPELINE-DATA-ML.md §5). Waktu kejadian memakai RFC3339
// UTC; tanggal bisnis (periode, tanggal kurva, last_available_date) memakai YYYY-MM-DD.
package dto

import (
	"math"
	"sort"
	"time"

	"github.com/garda-inflasi/api/internal/ml/domain"
	"github.com/garda-inflasi/api/internal/ml/usecase"
	"github.com/garda-inflasi/api/pkg/idx"
)

// Penanda asal data.
const (
	// SumberDatasafe: blok berasal dari hasil training (MongoDB datasafe).
	SumberDatasafe = "DATASAFE"
	// SumberDataunseen: blok berasal dari status crawl (MongoDB dataunseen.crawl_runs).
	SumberDataunseen = "DATAUNSEEN"
	// DataUtamaPostgres: isi `data` di samping `ml` tetap dari PostgreSQL apa adanya (termasuk penanda DEMO miliknya).
	DataUtamaPostgres = "POSTGRES"
)

const tanggalISO = "2006-01-02"

func waktu(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func waktuP(t *time.Time) *string {
	if t == nil || t.IsZero() {
		return nil
	}
	s := waktu(*t)
	return &s
}

func tanggal(t time.Time) string { return t.UTC().Format(tanggalISO) }

func tanggalP(t *time.Time) *string {
	if t == nil || t.IsZero() {
		return nil
	}
	s := tanggal(*t)
	return &s
}

func intP(f *float64) *int {
	if f == nil || math.IsNaN(*f) {
		return nil
	}
	v := int(math.Round(*f))
	return &v
}

func strP(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Meta adalah penanda yang dibawa setiap blok: ketersediaan, alasan bila tidak tersedia, asal data, dan siklus.
type Meta struct {
	Tersedia       bool   `json:"tersedia"`
	Alasan         string `json:"alasan,omitempty"`
	SumberData     string `json:"sumber_data,omitempty"`
	CycleID        string `json:"cycle_id,omitempty"`
	CycleTimestamp string `json:"cycle_timestamp,omitempty"`
}

// MetaMati adalah Meta blok yang tidak tersedia.
func MetaMati(alasan string) Meta { return Meta{Alasan: alasan} }

// MetaSiklus adalah Meta blok datasafe yang tersedia untuk siklus s.
func MetaSiklus(s domain.Siklus) Meta {
	return Meta{Tersedia: true, SumberData: SumberDatasafe, CycleID: s.CycleID, CycleTimestamp: waktu(s.CycleTimestamp)}
}

// MetaSiklusMati adalah Meta blok datasafe yang tidak punya data walau siklusnya tersedia.
func MetaSiklusMati(s domain.Siklus, alasan string) Meta {
	m := MetaSiklus(s)
	m.Tersedia, m.Alasan = false, alasan
	return m
}

// SiklusRingkas mengenali satu siklus.
type SiklusRingkas struct {
	CycleID        string  `json:"cycle_id"`
	CycleTimestamp string  `json:"cycle_timestamp"`
	DataCutoff     *string `json:"data_cutoff"`
	Status         string  `json:"status"`
	TargetInflasi  *string `json:"target_inflasi"`
	LabelTarget    *string `json:"label_target"`
}

func ToSiklusRingkas(s domain.Siklus) *SiklusRingkas {
	return &SiklusRingkas{
		CycleID: s.CycleID, CycleTimestamp: waktu(s.CycleTimestamp), DataCutoff: waktuP(s.DataCutoff), Status: s.Status,
		TargetInflasi: s.TargetInflasi, LabelTarget: labelTargetP(s.TargetInflasi),
	}
}

func labelTargetP(t *string) *string {
	if t == nil {
		return nil
	}
	return strP(domain.LabelTarget(*t))
}

// Lampiran adalah isi kunci `ml` pada respons endpoint yang diperkaya. Tersedia/Alasan/Siklus menerangkan siklus
// yang dipakai; setiap blok tetap membawa ketersediaannya sendiri (izin, cakupan baris, data per wilayah).
type Lampiran struct {
	Tersedia        bool                 `json:"tersedia"`
	Alasan          string               `json:"alasan,omitempty"`
	DataUtama       string               `json:"data_utama"`
	Siklus          *SiklusRingkas       `json:"siklus,omitempty"`
	Inflasi         *BlokInflasi         `json:"inflasi,omitempty"`
	ForecastRingkas *BlokForecastRingkas `json:"forecast_ringkas,omitempty"`
	Forecast        *BlokForecast        `json:"forecast,omitempty"`
	Model           *BlokModel           `json:"model,omitempty"`
	Ews             *BlokEws             `json:"ews,omitempty"`
	Wilayah         *BlokWilayah         `json:"wilayah,omitempty"`
	Kesegaran       *BlokKesegaran       `json:"kesegaran,omitempty"`
	Makro           *BlokMakro           `json:"makro,omitempty"`
}

// ── Inflasi ──────────────────────────────────────────────────────────

type Nilai3Response struct {
	Yoy *float64 `json:"yoy"`
	Mtm *float64 `json:"mtm"`
	Ytd *float64 `json:"ytd"`
}

type AktualResponse struct {
	Yoy     *float64 `json:"yoy"`
	Mtm     *float64 `json:"mtm"`
	Ytd     *float64 `json:"ytd"`
	Periode *string  `json:"periode"`
}

type PenyumbangResponse struct {
	KomoditasKode string   `json:"komoditas_kode"`
	Nama          string   `json:"nama"`
	Andil         *float64 `json:"andil"`
}

type FiturResponse struct {
	Nama       string   `json:"nama"`
	Kontribusi *float64 `json:"kontribusi"`
}

type FaktorResponse struct {
	Kelompok   string          `json:"kelompok"`
	Label      string          `json:"label"`
	Kontribusi *float64        `json:"kontribusi"`
	Arah       string          `json:"arah"`
	Fitur      []FiturResponse `json:"fitur"`
}

// PenjelasanResponse adalah hasil XAI (SHAP atau dekomposisi) untuk satu forecast atau satu wilayah inflasi.
type PenjelasanResponse struct {
	Metode string `json:"metode"`
	// SatuanKontribusi: poin_persen_harga (forecast) atau poin_persen_probabilitas (inflasi); null pada dokumen lama.
	SatuanKontribusi *string          `json:"satuan_kontribusi"`
	Faktor           []FaktorResponse `json:"faktor"`
	Narasi           string           `json:"narasi"`
}

func ToPenjelasan(x *domain.Xai) *PenjelasanResponse {
	if x == nil {
		return nil
	}
	faktor := make([]FaktorResponse, len(x.Faktor))
	for i, f := range x.Faktor {
		fitur := make([]FiturResponse, len(f.Fitur))
		for j, ft := range f.Fitur {
			fitur[j] = FiturResponse{Nama: ft.Nama, Kontribusi: ft.Kontribusi}
		}
		faktor[i] = FaktorResponse{Kelompok: f.Kelompok, Label: f.Label, Kontribusi: f.Kontribusi, Arah: f.Arah, Fitur: fitur}
	}
	return &PenjelasanResponse{Metode: x.Metode, SatuanKontribusi: x.SatuanKontribusi, Faktor: faktor, Narasi: x.Narasi}
}

// InflasiWilayahResponse adalah sinyal inflasi satu wilayah. Target INDEKS_PANGAN berarti label dihitung dari indeks
// harga pangan PIHPS, bukan inflasi resmi BPS; yoy/ytd boleh null. Kelas hanya boleh ditampilkan sebagai prediksi bila
// kelayakan = LAYAK; TIDAK_LAYAK (kelas null) dan TIDAK_DIKETAHUI (dokumen lama tanpa `layak`) ditampilkan sebagai
// "belum dapat diandalkan".
type InflasiWilayahResponse struct {
	WilayahKode       string               `json:"wilayah_kode"`
	Tingkat           string               `json:"tingkat"`
	Periode           *string              `json:"periode"`
	Kelas             *string              `json:"kelas"`
	Layak             *bool                `json:"layak"`
	Kelayakan         string               `json:"kelayakan"`
	AlasanKelayakan   *string              `json:"alasan_kelayakan"`
	Probabilitas      *float64             `json:"probabilitas"`
	SkorTekanan       *float64             `json:"skor_tekanan"`
	Siklus            string               `json:"siklus"`
	Target            string               `json:"target"`
	LabelTarget       string               `json:"label_target"`
	BukanInflasiResmi bool                 `json:"bukan_inflasi_resmi"`
	Proyeksi          Nilai3Response       `json:"proyeksi"`
	Aktual            AktualResponse       `json:"aktual"`
	Penyumbang        []PenyumbangResponse `json:"penyumbang"`
	Kalibrasi         KalibrasiResponse    `json:"kalibrasi"`
	Penjelasan        *PenjelasanResponse  `json:"penjelasan"`
}

type KalibrasiResponse struct {
	Metode *string  `json:"metode"`
	Brier  *float64 `json:"brier"`
}

// BlokInflasi memuat sinyal inflasi per wilayah siklus. Wilayah tanpa sinyal tidak dicantumkan (tidak diisi nilai lain).
type BlokInflasi struct {
	Meta
	// Kelayakan klasifikasi siklus (LAYAK, TIDAK_LAYAK, TIDAK_DIKETAHUI) beserta alasannya.
	Kelayakan         string                   `json:"kelayakan"`
	AlasanKelayakan   *string                  `json:"alasan_kelayakan"`
	Target            *string                  `json:"target"`
	LabelTarget       *string                  `json:"label_target"`
	BukanInflasiResmi bool                     `json:"bukan_inflasi_resmi"`
	Wilayah           []InflasiWilayahResponse `json:"wilayah"`
}

// Tingkat wilayah menurut panjang kode BPS.
func Tingkat(kode string) string {
	if len(kode) <= 2 {
		return "PROVINSI"
	}
	return "KABUPATEN"
}

func ToInflasiWilayah(s domain.SinyalInflasi, xai *domain.Xai) InflasiWilayahResponse {
	penyumbang := make([]PenyumbangResponse, len(s.Penyumbang))
	for i, p := range s.Penyumbang {
		penyumbang[i] = PenyumbangResponse{KomoditasKode: p.KomoditasKode, Nama: p.Nama, Andil: p.Andil}
	}
	return InflasiWilayahResponse{
		WilayahKode: s.WilayahKode, Tingkat: Tingkat(s.WilayahKode), Periode: tanggalP(s.Periode), Kelas: s.Kelas,
		Layak: s.Layak, Kelayakan: domain.Kelayakan(s.Layak), AlasanKelayakan: s.Alasan,
		Probabilitas: s.Probabilitas, SkorTekanan: s.SkorTekanan, Siklus: s.Siklus, Target: s.Target,
		LabelTarget: domain.LabelTarget(s.Target), BukanInflasiResmi: s.Target != domain.TargetResmiBPS,
		Proyeksi:   Nilai3Response{Yoy: s.Proyeksi.Yoy, Mtm: s.Proyeksi.Mtm, Ytd: s.Proyeksi.Ytd},
		Aktual:     AktualResponse{Yoy: s.Aktual.Yoy, Mtm: s.Aktual.Mtm, Ytd: s.Aktual.Ytd, Periode: tanggalP(s.Aktual.Periode)},
		Penyumbang: penyumbang, Kalibrasi: KalibrasiResponse{Metode: s.Kalibrasi.Metode, Brier: s.Kalibrasi.Brier},
		Penjelasan: ToPenjelasan(xai),
	}
}

// ── Forecast ─────────────────────────────────────────────────────────

// TitikKurvaResponse adalah satu horizon kurva kuantil. PerubahanP50Pct relatif terhadap harga_terakhir.
type TitikKurvaResponse struct {
	Horizon         int      `json:"horizon"`
	Tanggal         string   `json:"tanggal"`
	P10             *float64 `json:"p10"`
	P50             *float64 `json:"p50"`
	P90             *float64 `json:"p90"`
	PerubahanP50Pct *float64 `json:"perubahan_p50_pct"`
}

// ModelForecastResponse adalah model yang dipublikasikan. Nama boleh naive atau seasonal_naive bila tidak ada kandidat
// yang mengalahkan baseline; AdalahBaseline menandai keadaan itu.
type ModelForecastResponse struct {
	Nama           string  `json:"nama"`
	AdalahBaseline bool    `json:"adalah_baseline"`
	Fallback       bool    `json:"fallback"`
	Alasan         *string `json:"alasan"`
	NKandidat      *int    `json:"n_kandidat"`
}

// MetrikForecastResponse membandingkan model pemenang dengan baseline terbaik (naive atau seasonal_naive).
type MetrikForecastResponse struct {
	MAPE                *float64 `json:"mape"`
	RMSE                *float64 `json:"rmse"`
	Baseline            *string  `json:"baseline"`
	MapeBaseline        *float64 `json:"mape_baseline"`
	MengalahkanBaseline *bool    `json:"mengalahkan_baseline"`
	Evaluasi            *string  `json:"evaluasi"`
	NLipatan            *int     `json:"n_lipatan"`
	NLipatanSeleksi     *int     `json:"n_lipatan_seleksi"`
	Horizon             []int    `json:"horizon"`
}

// ForecastResponse adalah forecast satu komoditas x kabupaten.
type ForecastResponse struct {
	KomoditasKode   string                 `json:"komoditas_kode"`
	KabupatenKode   string                 `json:"kabupaten_kode"`
	HargaTerakhir   *float64               `json:"harga_terakhir"`
	TanggalTerakhir *string                `json:"tanggal_terakhir"`
	Satuan          string                 `json:"satuan"`
	Kurva           []TitikKurvaResponse   `json:"kurva"`
	Model           ModelForecastResponse  `json:"model"`
	Metrik          MetrikForecastResponse `json:"metrik"`
}

func persen(nilai, dasar *float64) *float64 {
	if nilai == nil || dasar == nil || *dasar == 0 {
		return nil
	}
	v := math.Round((*nilai / *dasar - 1)*1000) / 10
	return &v
}

func ToForecast(f domain.Forecast) ForecastResponse {
	kurva := make([]TitikKurvaResponse, len(f.Kurva))
	for i, t := range f.Kurva {
		kurva[i] = TitikKurvaResponse{Horizon: t.Horizon, Tanggal: tanggal(t.Tanggal), P10: t.P10, P50: t.P50, P90: t.P90,
			PerubahanP50Pct: persen(t.P50, f.HargaTerakhir)}
	}
	sort.SliceStable(kurva, func(i, j int) bool { return kurva[i].Horizon < kurva[j].Horizon })
	horizon := make([]int, 0, len(f.Metrik.Horizon))
	for _, h := range f.Metrik.Horizon {
		horizon = append(horizon, int(h))
	}
	return ForecastResponse{
		KomoditasKode: f.KomoditasKode, KabupatenKode: f.KabupatenKode, HargaTerakhir: f.HargaTerakhir,
		TanggalTerakhir: tanggalP(f.TanggalTerakhir), Satuan: f.Satuan, Kurva: kurva,
		Model: ModelForecastResponse{Nama: f.Model.Nama, AdalahBaseline: f.Model.Nama == "naive" || f.Model.Nama == "seasonal_naive",
			Fallback: f.Model.Fallback, Alasan: f.Model.Alasan, NKandidat: intP(f.Model.NKandidat)},
		Metrik: MetrikForecastResponse{
			MAPE: f.Metrik.MAPE, RMSE: f.Metrik.RMSE, Baseline: f.Metrik.Baseline, MapeBaseline: f.Metrik.MapeBaseline,
			MengalahkanBaseline: f.Metrik.MengalahkanBaseline, Evaluasi: f.Metrik.Evaluasi, NLipatan: intP(f.Metrik.NLipatan),
			NLipatanSeleksi: intP(f.Metrik.NLipatanSeleksi),
			Horizon:         horizon,
		},
	}
}

// BlokForecastRingkas memuat forecast tiap komoditas pada satu kabupaten (atau semua kabupaten bila tanpa filter).
type BlokForecastRingkas struct {
	Meta
	KabupatenKode   *string            `json:"kabupaten_kode"`
	WilayahTersedia []string           `json:"wilayah_tersedia"`
	Items           []ForecastResponse `json:"items"`
}

type TitikRiwayatResponse struct {
	Tanggal string   `json:"tanggal"`
	Nilai   *float64 `json:"nilai"`
}

// ForecastDetailResponse menambahkan riwayat harga asli yang dipakai pelatihan dan penjelasan XAI.
type ForecastDetailResponse struct {
	ForecastResponse
	Riwayat    []TitikRiwayatResponse `json:"riwayat"`
	Penjelasan *PenjelasanResponse    `json:"penjelasan"`
}

// BlokForecast adalah forecast detail satu komoditas x kabupaten (GET /v1/prediksi/:kode).
type BlokForecast struct {
	Meta
	KomoditasKode   string                  `json:"komoditas_kode"`
	KabupatenKode   string                  `json:"kabupaten_kode"`
	WilayahTersedia []string                `json:"wilayah_tersedia"`
	Item            *ForecastDetailResponse `json:"item"`
}

func ToForecastDetail(f domain.Forecast, xai *domain.Xai) *ForecastDetailResponse {
	riwayat := make([]TitikRiwayatResponse, len(f.Riwayat))
	for i, r := range f.Riwayat {
		riwayat[i] = TitikRiwayatResponse{Tanggal: tanggal(r.Tanggal), Nilai: r.Nilai}
	}
	return &ForecastDetailResponse{ForecastResponse: ToForecast(f), Riwayat: riwayat, Penjelasan: ToPenjelasan(xai)}
}

// ── Model ────────────────────────────────────────────────────────────

// KlasifikasiResponse adalah metrik klasifikasi inflasi. Layak null (dokumen lama) berkelayakan TIDAK_DIKETAHUI.
type KlasifikasiResponse struct {
	Model         *string  `json:"model"`
	F1            *float64 `json:"f1"`
	RocAuc        *float64 `json:"roc_auc"`
	Brier         *float64 `json:"brier"`
	BrierBaseline *float64 `json:"brier_baseline"`
	NUji          *int     `json:"n_uji"`
	Kalibrasi     *string  `json:"kalibrasi"`
	Layak         *bool    `json:"layak"`
	Kelayakan     string   `json:"kelayakan"`
	Alasan        *string  `json:"alasan"`
}

type RegresiResponse struct {
	Pemenang            map[string]int `json:"pemenang"`
	MapeMedian          *float64       `json:"mape_median"`
	RmseMedian          *float64       `json:"rmse_median"`
	MapeBaselineMedian  *float64       `json:"mape_baseline_median"`
	NSeri               *int           `json:"n_seri"`
	MengalahkanBaseline *int           `json:"mengalahkan_baseline"`
	NDievaluasiHoldout  *int           `json:"n_dievaluasi_holdout"`
	NDilewati           *int           `json:"n_dilewati"`
	NGagal              *int           `json:"n_gagal"`
	ModelGlobal         *string        `json:"model_global"`
	Tingkat             *string        `json:"tingkat"`
}

// MetrikModelResponse adalah ml_training_cycles.model_metrics yang dipublikasikan.
type MetrikModelResponse struct {
	Klasifikasi *KlasifikasiResponse `json:"klasifikasi"`
	Regresi     *RegresiResponse     `json:"regresi"`
}

func ToMetrikModel(m *domain.MetrikModel) MetrikModelResponse {
	var out MetrikModelResponse
	if m == nil {
		return out
	}
	if k := m.Klasifikasi; k != nil {
		out.Klasifikasi = &KlasifikasiResponse{Model: k.Model, F1: k.F1, RocAuc: k.RocAuc, Brier: k.Brier, BrierBaseline: k.BrierBaseline,
			NUji: intP(k.NUji), Kalibrasi: k.Kalibrasi, Layak: k.Layak, Kelayakan: domain.Kelayakan(k.Layak), Alasan: k.Alasan}
	}
	if r := m.Regresi; r != nil {
		pemenang := make(map[string]int, len(r.Pemenang))
		for k, v := range r.Pemenang {
			pemenang[k] = int(v)
		}
		out.Regresi = &RegresiResponse{
			Pemenang: pemenang, MapeMedian: r.MapeMedian, RmseMedian: r.RmseMedian, MapeBaselineMedian: r.MapeBaselineMedian,
			NSeri: intP(r.NSeri), MengalahkanBaseline: intP(r.MengalahkanBaseline), NDievaluasiHoldout: intP(r.NDievaluasiHoldout),
			NDilewati: intP(r.NDilewati), NGagal: intP(r.NGagal), ModelGlobal: r.ModelGlobal, Tingkat: r.Tingkat,
		}
	}
	return out
}

// BlokModel adalah metrik model siklus.
type BlokModel struct {
	Meta
	Status        string  `json:"status"`
	TargetInflasi *string `json:"target_inflasi"`
	LabelTarget   *string `json:"label_target"`
	MetrikModelResponse
}

// ── EWS ──────────────────────────────────────────────────────────────

// AlertTautanResponse adalah alert EWS PostgreSQL hasil jembatan untuk satu sinyal.
type AlertTautanResponse struct {
	ID     idx.ID  `json:"id"`
	Kode   *string `json:"kode"`
	Status string  `json:"status"`
}

// SinyalEwsResponse adalah satu sinyal datasafe_ews_signals. MenjadiAlert benar bila sinyal aktif bertingkat
// WASPADA ke atas (INFO, termasuk sumber yang sengaja dilewati, tidak pernah menjadi alert).
type SinyalEwsResponse struct {
	SinyalKunci   string               `json:"sinyal_kunci"`
	Severity      string               `json:"severity"`
	Jenis         string               `json:"jenis"`
	KomoditasKode *string              `json:"komoditas_kode"`
	KabupatenKode *string              `json:"kabupaten_kode"`
	Nilai         *float64             `json:"nilai"`
	Ambang        *float64             `json:"ambang"`
	Metode        *string              `json:"metode"`
	Judul         string               `json:"judul"`
	Pesan         string               `json:"pesan"`
	Aktif         bool                 `json:"aktif"`
	MenjadiAlert  bool                 `json:"menjadi_alert"`
	Alert         *AlertTautanResponse `json:"alert"`
}

func ToSinyalEws(s domain.SinyalEws, tautan map[string]domain.TautanAlert) SinyalEwsResponse {
	out := SinyalEwsResponse{
		SinyalKunci: s.SinyalKunci, Severity: s.Severity, Jenis: s.Jenis, KomoditasKode: s.KomoditasKode,
		KabupatenKode: s.KabupatenKode, Nilai: s.Nilai, Ambang: s.Ambang, Metode: s.Metode, Judul: s.Judul, Pesan: s.Pesan,
		Aktif: s.Aktif, MenjadiAlert: s.MenjadiAlert(),
	}
	if t, ok := tautan[s.SinyalKunci]; ok {
		out.Alert = &AlertTautanResponse{ID: t.AlertID, Kode: t.AlertKode, Status: t.Status}
	}
	return out
}

type RingkasanEwsResponse struct {
	Aktif   int `json:"aktif"`
	Kritis  int `json:"kritis"`
	Siaga   int `json:"siaga"`
	Waspada int `json:"waspada"`
	Info    int `json:"info"`
}

// BlokEws adalah sinyal EWS aktif siklus dalam cakupan pemanggil, paling parah lebih dulu.
type BlokEws struct {
	Meta
	Ringkasan RingkasanEwsResponse `json:"ringkasan"`
	Sinyal    []SinyalEwsResponse  `json:"sinyal"`
	Terpotong bool                 `json:"terpotong"`
}

// ── Wilayah (spasial) ────────────────────────────────────────────────

type InflasiRingkasResponse struct {
	Kelas        *string  `json:"kelas"`
	Kelayakan    string   `json:"kelayakan"`
	Probabilitas *float64 `json:"probabilitas"`
	SkorTekanan  *float64 `json:"skor_tekanan"`
	Siklus       string   `json:"siklus"`
	Target       string   `json:"target"`
	LabelTarget  string   `json:"label_target"`
}

// WilayahMLResponse merangkum cakupan hasil ML satu wilayah.
type WilayahMLResponse struct {
	WilayahKode       string                  `json:"wilayah_kode"`
	Tingkat           string                  `json:"tingkat"`
	KomoditasForecast []string                `json:"komoditas_forecast"`
	Inflasi           *InflasiRingkasResponse `json:"inflasi"`
	SinyalAktif       RingkasanEwsResponse    `json:"sinyal_aktif"`
}

// BlokWilayah memuat hanya wilayah yang punya hasil ML; wilayah lain berarti tidak ada forecast/sinyal.
type BlokWilayah struct {
	Meta
	Items []WilayahMLResponse `json:"items"`
}

// ── Kesegaran ────────────────────────────────────────────────────────

type RunResponse struct {
	RunID             string  `json:"run_id"`
	Crawler           string  `json:"crawler"`
	CrawlDate         string  `json:"crawl_date"`
	Mulai             *string `json:"mulai"`
	Selesai           *string `json:"selesai"`
	Status            string  `json:"status"`
	JumlahBaru        *int    `json:"jumlah_baru"`
	JumlahDiteruskan  *int    `json:"jumlah_diteruskan"`
	LastAvailableDate *string `json:"last_available_date"`
	Pesan             string  `json:"pesan"`
	Pemicu            string  `json:"pemicu"`
	Dilewati          bool    `json:"dilewati"`
}

func toRun(r *domain.CrawlRun) *RunResponse {
	if r == nil {
		return nil
	}
	return &RunResponse{
		RunID: r.RunID, Crawler: r.Crawler, CrawlDate: r.CrawlDate, Mulai: waktuP(r.Mulai), Selesai: waktuP(r.Selesai),
		Status: r.Status, JumlahBaru: intP(r.JumlahBaru), JumlahDiteruskan: intP(r.JumlahDiteruskan),
		LastAvailableDate: tanggalP(r.LastAvailableDate), Pesan: r.Pesan, Pemicu: r.Pemicu, Dilewati: domain.Dilewati(r.Pesan),
	}
}

// KesegaranSiklusResponse adalah kesegaran sumber menurut siklus training terbaru (source_freshness).
type KesegaranSiklusResponse struct {
	Status            *string `json:"status"`
	LastAvailableDate *string `json:"last_available_date"`
	CrawledAt         *string `json:"crawled_at"`
	UmurHari          *int    `json:"umur_hari"`
	Pesan             string  `json:"pesan"`
}

// SumberKesegaranResponse adalah kesegaran satu (sumber, dataset). Kategori dinilai dari run terakhir non-BACKFILL:
// SEGAR, BASI, GAGAL, DILEWATI (pesan DILEWATI:, sengaja tidak dijalankan), BELUM_ADA_RUN_TERJADWAL.
type SumberKesegaranResponse struct {
	Sumber            string                   `json:"sumber"`
	Dataset           string                   `json:"dataset"`
	Kategori          string                   `json:"kategori"`
	Status            *string                  `json:"status"`
	Pesan             string                   `json:"pesan"`
	LastAvailableDate *string                  `json:"last_available_date"`
	UmurHari          *int                     `json:"umur_hari"`
	RunTerakhir       *RunResponse             `json:"run_terakhir"`
	BackfillTerakhir  *RunResponse             `json:"backfill_terakhir"`
	MenurutSiklus     *KesegaranSiklusResponse `json:"menurut_siklus"`
}

type RingkasanKesegaranResponse struct {
	Segar                int `json:"segar"`
	Basi                 int `json:"basi"`
	Gagal                int `json:"gagal"`
	Dilewati             int `json:"dilewati"`
	BelumAdaRunTerjadwal int `json:"belum_ada_run_terjadwal"`
}

// BlokKesegaran adalah status crawl tiap sumber ditambah sinyal SUMBER_DATA siklus terbaru (termasuk INFO).
type BlokKesegaran struct {
	Meta
	DihitungPada string                     `json:"dihitung_pada"`
	Siklus       *SiklusRingkas             `json:"siklus"`
	AlasanSiklus string                     `json:"alasan_siklus,omitempty"`
	Ringkasan    RingkasanKesegaranResponse `json:"ringkasan"`
	Sumber       []SumberKesegaranResponse  `json:"sumber"`
	SinyalSumber []SinyalEwsResponse        `json:"sinyal_sumber"`
}

func ToKesegaran(h *usecase.HasilKesegaran, tautan map[string]domain.TautanAlert) *BlokKesegaran {
	out := &BlokKesegaran{
		Meta:         Meta{Tersedia: true, SumberData: SumberDataunseen},
		DihitungPada: waktu(h.DihitungPada), Sumber: []SumberKesegaranResponse{}, SinyalSumber: []SinyalEwsResponse{},
	}
	if h.Siklus != nil {
		out.Siklus = ToSiklusRingkas(*h.Siklus)
		out.CycleID, out.CycleTimestamp = h.Siklus.CycleID, waktu(h.Siklus.CycleTimestamp)
	} else if !h.KeadaanSiklus.Tersedia {
		out.AlasanSiklus = h.KeadaanSiklus.Alasan
	}
	for _, e := range h.Entri {
		item := SumberKesegaranResponse{
			Sumber: e.Sumber, Dataset: e.Dataset, Kategori: e.Kategori, Status: e.Status, Pesan: e.Pesan,
			LastAvailableDate: tanggalP(e.LastAvailableDate), UmurHari: e.UmurHari,
			RunTerakhir: toRun(e.Terjadwal), BackfillTerakhir: toRun(e.Backfill),
		}
		if f := e.MenurutSiklus; f != nil {
			item.MenurutSiklus = &KesegaranSiklusResponse{Status: f.Status, LastAvailableDate: tanggalP(f.LastAvailableDate),
				CrawledAt: waktuP(f.CrawledAt), UmurHari: intP(f.UmurHari), Pesan: f.Pesan}
		}
		switch e.Kategori {
		case usecase.KategoriSegar:
			out.Ringkasan.Segar++
		case usecase.KategoriBasi:
			out.Ringkasan.Basi++
		case usecase.KategoriGagal:
			out.Ringkasan.Gagal++
		case usecase.KategoriDilewati:
			out.Ringkasan.Dilewati++
		case usecase.KategoriBelumAda:
			out.Ringkasan.BelumAdaRunTerjadwal++
		}
		out.Sumber = append(out.Sumber, item)
	}
	for _, s := range h.Sinyal {
		out.SinyalSumber = append(out.SinyalSumber, ToSinyalEws(s, tautan))
	}
	return out
}

// ── Makro ────────────────────────────────────────────────────────────

type MakroResponse struct {
	Domain            string   `json:"domain"`
	Indikator         string   `json:"indikator"`
	WilayahKode       *string  `json:"wilayah_kode"`
	Periode           *string  `json:"periode"`
	Nilai             *float64 `json:"nilai"`
	Satuan            string   `json:"satuan"`
	Perubahan         *float64 `json:"perubahan"`
	Sumber            string   `json:"sumber"`
	IsCarriedForward  bool     `json:"is_carried_forward"`
	LastAvailableDate *string  `json:"last_available_date"`
}

// BlokMakro memuat indikator makro siklus (kosong selama sumber makro belum tersedia, mis. tanpa BPS_API_KEY).
type BlokMakro struct {
	Meta
	Items []MakroResponse `json:"items"`
}

func ToMakro(m domain.Makro) MakroResponse {
	return MakroResponse{
		Domain: m.Domain, Indikator: m.Indikator, WilayahKode: m.WilayahKode, Periode: tanggalP(m.Periode), Nilai: m.Nilai,
		Satuan: m.Satuan, Perubahan: m.Perubahan, Sumber: m.Sumber, IsCarriedForward: m.IsCarriedForward,
		LastAvailableDate: tanggalP(m.LastAvailableDate),
	}
}

// ── Siklus (endpoint /v1/ml/cycles) ──────────────────────────────────

// SiklusResponse adalah satu siklus training. SourceFreshness hanya dikirim kepada pemegang sumber.lihat.
type SiklusResponse struct {
	CycleID         string                  `json:"cycle_id"`
	CycleTimestamp  string                  `json:"cycle_timestamp"`
	DataCutoff      *string                 `json:"data_cutoff"`
	Status          string                  `json:"status"`
	Final           bool                    `json:"final"`
	DurationMS      *int                    `json:"duration_ms"`
	Pesan           string                  `json:"pesan"`
	VersiKode       string                  `json:"versi_kode"`
	Jumlah          map[string]int          `json:"jumlah"`
	TargetInflasi   *string                 `json:"target_inflasi"`
	LabelTarget     *string                 `json:"label_target"`
	ModelMetrics    MetrikModelResponse     `json:"model_metrics"`
	SourceFreshness []KesegaranSiklusSumber `json:"source_freshness,omitempty"`
}

// KesegaranSiklusSumber adalah satu entri source_freshness siklus.
type KesegaranSiklusSumber struct {
	Sumber  string `json:"sumber"`
	Dataset string `json:"dataset"`
	KesegaranSiklusResponse
	Dilewati bool `json:"dilewati"`
}

func ToSiklus(s domain.Siklus, denganKesegaran bool) SiklusResponse {
	jumlah := make(map[string]int, len(s.Jumlah))
	for k, v := range s.Jumlah {
		jumlah[k] = int(v)
	}
	out := SiklusResponse{
		CycleID: s.CycleID, CycleTimestamp: waktu(s.CycleTimestamp), DataCutoff: waktuP(s.DataCutoff), Status: s.Status,
		Final: domain.Final(s.Status), DurationMS: intP(s.DurationMS), Pesan: s.Pesan, VersiKode: s.VersiKode, Jumlah: jumlah,
		TargetInflasi: s.TargetInflasi, LabelTarget: labelTargetP(s.TargetInflasi), ModelMetrics: ToMetrikModel(s.ModelMetrics),
	}
	if denganKesegaran {
		out.SourceFreshness = make([]KesegaranSiklusSumber, len(s.SourceFreshness))
		for i, f := range s.SourceFreshness {
			out.SourceFreshness[i] = KesegaranSiklusSumber{
				Sumber: f.Sumber, Dataset: f.Dataset, Dilewati: domain.Dilewati(f.Pesan),
				KesegaranSiklusResponse: KesegaranSiklusResponse{Status: f.Status, LastAvailableDate: tanggalP(f.LastAvailableDate),
					CrawledAt: waktuP(f.CrawledAt), UmurHari: intP(f.UmurHari), Pesan: f.Pesan},
			}
		}
	}
	return out
}

// DaftarSiklusResponse adalah satu halaman GET /v1/ml/cycles.
type DaftarSiklusResponse struct {
	Tersedia bool             `json:"tersedia"`
	Alasan   string           `json:"alasan,omitempty"`
	Items    []SiklusResponse `json:"items"`
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	Limit    int              `json:"limit"`
}
