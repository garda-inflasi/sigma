package integration

import (
	"context"
	"testing"
	"time"

	mlrepo "github.com/garda-inflasi/api/internal/ml/repository/mongo"
	"github.com/garda-inflasi/api/internal/ml/usecase"
	"github.com/garda-inflasi/api/pkg/platform/mongodb"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Siklus fixture. C09 adalah siklus final terbaru; C10 lebih baru tetapi BERJALAN (setengah tertulis, tidak boleh
// terbaca); C07 GAGAL; C08 BERHASIL lebih lama.
const (
	c07 = "C20261124T07Z"
	c08 = "C20261124T08Z"
	c09 = "C20261124T09Z"
	c10 = "C20261124T10Z"
)

var tsSiklus = map[string]time.Time{
	c07: time.Date(2026, 11, 24, 7, 0, 0, 0, time.UTC),
	c08: time.Date(2026, 11, 24, 8, 0, 0, 0, time.UTC),
	c09: time.Date(2026, 11, 24, 9, 0, 0, 0, time.UTC),
	c10: time.Date(2026, 11, 24, 10, 0, 0, 0, time.UTC),
}

// tgl adalah tanggal bisnis: tengah malam UTC dari tanggal WIB.
func tgl(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

// mongoUji adalah pasangan database dataunseen/datasafe milik satu tes.
type mongoUji struct {
	unseen, safe string
}

func (m mongoUji) cfg() mongodb.Config {
	return mongodb.Config{URI: mg.URI(), DBUnseen: m.unseen, DBSafe: m.safe, Timeout: 2 * time.Second}
}

// service membuat Service usecase yang membaca database tes ini.
func (m mongoUji) service(t *testing.T) *usecase.Service {
	t.Helper()
	k, err := mongodb.Buka(m.cfg())
	require.NoError(t, err)
	t.Cleanup(func() { _ = k.Tutup(context.Background()) })
	return usecase.New(mlrepo.New(k), "", nil)
}

func (m mongoUji) tulis(t *testing.T, db, koleksi string, docs ...bson.M) {
	t.Helper()
	if len(docs) == 0 {
		return
	}
	items := make([]any, len(docs))
	for i, d := range docs {
		items[i] = d
	}
	_, err := mg.Client().Database(db).Collection(koleksi).InsertMany(context.Background(), items)
	require.NoError(t, err)
}

func (m mongoUji) hapusSiklus(t *testing.T, koleksi string, filter bson.M) {
	t.Helper()
	_, err := mg.Client().Database(m.safe).Collection(koleksi).DeleteMany(context.Background(), filter)
	require.NoError(t, err)
}

func mongoBaru(t *testing.T) mongoUji {
	t.Helper()
	return mongoUji{unseen: mg.NamaDB(t, "unseen"), safe: mg.NamaDB(t, "safe")}
}

func metaSiklus(id string) bson.M { return bson.M{"cycle_id": id, "cycle_timestamp": tsSiklus[id]} }

func gabung(a, b bson.M) bson.M {
	out := bson.M{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

func dokSiklus(id, status string) bson.M {
	return gabung(metaSiklus(id), bson.M{
		"_id": id, "data_cutoff": tsSiklus[id], "duration_ms": 41234, "status": status, "pesan": "tidak ada data baru",
		"versi_kode": "pipeline-1.0", "jumlah": bson.M{"datasafe_forecasts": 2, "datasafe_ews_signals": 6},
		"source_freshness": bson.A{
			bson.M{"sumber": "PIHPS", "dataset": "harga_harian", "status": "FRESH", "last_available_date": tgl(2026, 11, 23),
				"crawled_at": tsSiklus[id].Add(-time.Hour), "umur_hari": 1, "pesan": ""},
			bson.M{"sumber": "BMKG", "dataset": "prakiraan", "status": "ERROR", "last_available_date": nil,
				"crawled_at": tsSiklus[id].Add(-time.Hour), "umur_hari": nil, "pesan": "DILEWATI: robots.txt melarang"},
		},
		"target_inflasi": "INDEKS_PANGAN",
		"model_metrics": bson.M{
			"klasifikasi": bson.M{"model": "lightgbm", "f1": 0.71, "roc_auc": 0.78, "brier": 0.19, "n_uji": 42, "kalibrasi": "sigmoid"},
			"regresi": bson.M{
				"pemenang":    bson.M{"lightgbm_quantile": 1, "sarimax": 0, "holt_winters": 0, "seasonal_naive": 1, "naive": 0},
				"mape_median": 3.2, "rmse_median": 812.5, "mape_baseline_median": 4.1, "tingkat": "penuh",
				"params": bson.M{"besar": "dibuang proyeksi"}, "model_global": "lightgbm_quantile", "n_seri": 2,
				"mengalahkan_baseline": 1, "n_dievaluasi_holdout": 2, "n_dilewati": 0, "n_gagal": 0,
			},
			"kegagalan": bson.A{}, "durasi_tahap_ms": bson.M{"fitur": 10},
		},
	})
}

func kurva(cutoff time.Time, harga float64) bson.A {
	out := bson.A{}
	for _, h := range []int{7, 14, 30} {
		f := 1 + float64(h)/300
		out = append(out, bson.M{"tanggal": cutoff.AddDate(0, 0, h), "horizon": h,
			"p10": harga * (f - 0.05), "p50": harga * f, "p90": harga * (f + 0.05)})
	}
	return out
}

func dokForecast(id, kom, kab string, harga float64) bson.M {
	cut := tgl(2026, 11, 24)
	return gabung(metaSiklus(id), bson.M{
		"_id": id + ":" + kom + ":" + kab, "komoditas_kode": kom, "kabupaten_kode": kab,
		"harga_terakhir": harga, "tanggal_terakhir": tgl(2026, 11, 23), "satuan": "Rp/kg",
		"model": bson.M{"nama": "lightgbm_quantile", "fallback": false, "alasan": "", "n_kandidat": 5},
		"metrik": bson.M{"rmse": 640.2, "mape": 2.9, "baseline": "seasonal_naive", "mape_baseline": 4.0,
			"mengalahkan_baseline": true, "evaluasi": "holdout_lipatan_terakhir", "n_lipatan": 4, "n_lipatan_seleksi": 3,
			"horizon": bson.A{7, 14, 30}},
		"kurva":   kurva(cut, harga),
		"riwayat": bson.A{bson.M{"tanggal": tgl(2026, 11, 22), "nilai": harga - 500}, bson.M{"tanggal": tgl(2026, 11, 23), "nilai": harga}},
	})
}

func dokInflasi(id, wilayah string) bson.M {
	return gabung(metaSiklus(id), bson.M{
		"_id": id + ":" + wilayah, "wilayah_kode": wilayah, "periode": tgl(2026, 12, 1), "kelas": "INFLASI",
		"probabilitas": 0.64, "skor_tekanan": 66.0, "siklus": "TEKANAN", "target": "INDEKS_PANGAN",
		"proyeksi":   bson.M{"yoy": nil, "mtm": 0.42, "ytd": nil},
		"aktual":     bson.M{"yoy": nil, "mtm": 0.31, "ytd": nil, "periode": tgl(2026, 10, 31)},
		"penyumbang": bson.A{bson.M{"komoditas_kode": "CBM-KRT", "nama": "Cabai merah keriting", "andil": 0.21}},
		"kalibrasi":  bson.M{"metode": "sigmoid", "brier": 0.19},
	})
}

func dokXai(id, target string, kom *string, wilayah string) bson.M {
	var k any
	kunci := "-"
	if kom != nil {
		k, kunci = *kom, *kom
	}
	return gabung(metaSiklus(id), bson.M{
		"_id": id + ":" + target + ":" + kunci + ":" + wilayah, "target": target, "komoditas_kode": k, "wilayah_kode": wilayah,
		"metode": "shap", "narasi": "Faktor terbesar: pasokan, tren harga lapangan.",
		"faktor": bson.A{bson.M{"kelompok": "PASOKAN", "label": "Pasokan", "kontribusi": 0.12, "arah": "NAIK",
			"fitur": bson.A{bson.M{"nama": "lag_7", "kontribusi": 0.08}}}},
	})
}

func sinyal(id, kunci, severity, jenis string, kom, kab *string, nilai *float64, aktif bool) bson.M {
	var k, w, n any
	if kom != nil {
		k = *kom
	}
	if kab != nil {
		w = *kab
	}
	if nilai != nil {
		n = *nilai
	}
	return gabung(metaSiklus(id), bson.M{
		"_id": id + ":" + kunci, "sinyal_kunci": kunci, "severity": severity, "jenis": jenis, "komoditas_kode": k,
		"kabupaten_kode": w, "nilai": n, "ambang": 10.0, "metode": "zscore_musiman", "judul": "Judul " + kunci,
		"pesan": "Pesan " + kunci + ".", "aktif": aktif,
	})
}

func s(v string) *string   { return &v }
func f(v float64) *float64 { return &v }

// Kunci sinyal fixture.
const (
	kAnomali   = "ANOMALI_HARGA:CBM-KRT:1171:NAIK"
	kProyeksi  = "PROYEKSI_LONJAKAN:BWM:1174"
	kTekanan   = "TEKANAN_INFLASI:1107"
	kDilewati  = "SUMBER_DATA:BMKG:prakiraan"
	kGagal     = "SUMBER_DATA:BPS:inflasi"
	kProvinsi  = "TEKANAN_INFLASI:11"
	kPulihLama = "ANOMALI_HARGA:BWM:1171:NAIK"
)

// sinyalSiklus adalah sinyal standar satu siklus.
func sinyalSiklus(id string) []bson.M {
	return []bson.M{
		sinyal(id, kAnomali, "SIAGA", "ANOMALI_HARGA", s("CBM-KRT"), s("1171"), f(12.5), true),
		sinyal(id, kProyeksi, "WASPADA", "PROYEKSI_LONJAKAN", s("BWM"), s("1174"), f(6.1), true),
		sinyal(id, kTekanan, "WASPADA", "TEKANAN_INFLASI", nil, s("1107"), f(66), true),
		sinyal(id, kDilewati, "INFO", "SUMBER_DATA", nil, nil, nil, true),
		sinyal(id, kGagal, "SIAGA", "SUMBER_DATA", nil, nil, f(40), true),
	}
}

// formatBaru mengubah dokumen fixture ke kontrak terbaru: klasifikasi tidak layak (kelas null, XAI inflasi tidak
// terbit) dan XAI forecast bersatuan kontribusi.
func formatBaruSiklus(d bson.M) bson.M {
	mm := d["model_metrics"].(bson.M)
	k := mm["klasifikasi"].(bson.M)
	k["brier_baseline"], k["layak"], k["alasan"] = 0.24, false, "ROC-AUC holdout 0,54 < 0,60"
	return d
}

func formatBaruInflasi(d bson.M) bson.M {
	d["kelas"], d["layak"], d["alasan"] = nil, false, "ROC-AUC holdout 0,54 < 0,60"
	return d
}

// isiStandar menulis fixture lengkap: empat siklus, hasil C08 dan C09, dan crawl_runs.
func isiStandar(t *testing.T, m mongoUji) {
	t.Helper()
	// C09 (terbaru) memakai format lama tanpa `layak`; C08 memakai format baru (klasifikasi tidak layak).
	m.tulis(t, m.safe, mlrepo.KoleksiSiklus, dokSiklus(c07, "GAGAL"), formatBaruSiklus(dokSiklus(c08, "BERHASIL")), dokSiklus(c09, "SEBAGIAN"), dokSiklus(c10, "BERJALAN"))
	for _, id := range []string{c08, c09, c10} {
		m.tulis(t, m.safe, mlrepo.KoleksiForecast, dokForecast(id, "CBM-KRT", "1171", 52000), dokForecast(id, "BWM", "1174", 41000))
		m.tulis(t, m.safe, mlrepo.KoleksiEws, sinyalSiklus(id)...)
		if id == c08 {
			m.tulis(t, m.safe, mlrepo.KoleksiInflasi, formatBaruInflasi(dokInflasi(id, "1171")), formatBaruInflasi(dokInflasi(id, "1174")), formatBaruInflasi(dokInflasi(id, "1107")))
			xf := dokXai(id, "forecast", s("CBM-KRT"), "1171")
			xf["satuan_kontribusi"] = "poin_persen_harga"
			m.tulis(t, m.safe, mlrepo.KoleksiXai, xf)
			continue
		}
		m.tulis(t, m.safe, mlrepo.KoleksiInflasi, dokInflasi(id, "1171"), dokInflasi(id, "1174"), dokInflasi(id, "1107"))
		m.tulis(t, m.safe, mlrepo.KoleksiXai, dokXai(id, "forecast", s("CBM-KRT"), "1171"), dokXai(id, "inflasi", nil, "1171"))
	}
	// C10 setengah tertulis membawa sinyal KRITIS yang tidak boleh terbaca siapa pun.
	m.tulis(t, m.safe, mlrepo.KoleksiEws, sinyal(c10, "ANOMALI_HARGA:GLA:1171:NAIK", "KRITIS", "ANOMALI_HARGA", s("GLA"), s("1171"), f(40), true))

	hari := func(d int, jam int) time.Time { return time.Date(2026, 11, d, jam, 0, 0, 0, time.UTC) }
	m.tulis(t, m.unseen, mlrepo.KoleksiCrawlRuns,
		// PIHPS: run terjadwal terakhir ERROR; backfill yang lebih baru (FRESH) tidak boleh menutupinya.
		bson.M{"_id": "run-pihps-1", "crawler": "pihps", "sumber": "PIHPS", "dataset": "harga_harian", "crawl_date": "2026-11-23",
			"mulai": hari(23, 1), "selesai": hari(23, 2), "status": "FRESH", "jumlah_baru": 120, "jumlah_diteruskan": 0,
			"last_available_date": tgl(2026, 11, 22), "pesan": "", "pemicu": "JADWAL"},
		bson.M{"_id": "run-pihps-2", "crawler": "pihps", "sumber": "PIHPS", "dataset": "harga_harian", "crawl_date": "2026-11-24",
			"mulai": hari(24, 1), "selesai": hari(24, 1), "status": "ERROR", "jumlah_baru": 0, "jumlah_diteruskan": 0,
			"last_available_date": tgl(2026, 11, 22), "pesan": "HTTP 503", "pemicu": "JADWAL"},
		bson.M{"_id": "run-pihps-bf", "crawler": "pihps", "sumber": "PIHPS", "dataset": "harga_harian", "crawl_date": "2026-11-24",
			"mulai": hari(24, 3), "selesai": hari(24, 4), "status": "FRESH", "jumlah_baru": 7000, "jumlah_diteruskan": 0,
			"last_available_date": tgl(2026, 11, 23), "pesan": "", "pemicu": "BACKFILL"},
		// BMKG sengaja dilewati.
		bson.M{"_id": "run-bmkg-1", "crawler": "bmkg", "sumber": "BMKG", "dataset": "prakiraan", "crawl_date": "2026-11-24",
			"mulai": hari(24, 1), "selesai": hari(24, 1), "status": "ERROR", "jumlah_baru": 0, "jumlah_diteruskan": 0,
			"last_available_date": nil, "pesan": "DILEWATI: robots.txt melarang", "pemicu": "JADWAL"},
		// BPS hanya pernah di-backfill.
		bson.M{"_id": "run-bps-bf", "crawler": "bps", "sumber": "BPS", "dataset": "inflasi", "crawl_date": "2026-11-20",
			"mulai": hari(20, 1), "selesai": hari(20, 1), "status": "ERROR", "jumlah_baru": 0, "jumlah_diteruskan": 0,
			"last_available_date": nil, "pesan": "butuh BPS_API_KEY", "pemicu": "BACKFILL"},
	)
}
