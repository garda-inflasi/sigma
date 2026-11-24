package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	authRoutes "github.com/garda-inflasi/api/internal/auth/routes"
	dashboardRoutes "github.com/garda-inflasi/api/internal/dashboard/routes"
	eksekutifRoutes "github.com/garda-inflasi/api/internal/eksekutif/routes"
	ewsRoutes "github.com/garda-inflasi/api/internal/ews/routes"
	kpiRoutes "github.com/garda-inflasi/api/internal/kpi/routes"
	mlrepo "github.com/garda-inflasi/api/internal/ml/repository/mongo"
	mlRoutes "github.com/garda-inflasi/api/internal/ml/routes"
	"github.com/garda-inflasi/api/internal/ml/usecase"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	policyBriefRoutes "github.com/garda-inflasi/api/internal/policy-brief/routes"
	prediksiRoutes "github.com/garda-inflasi/api/internal/prediksi/routes"
	spasialRoutes "github.com/garda-inflasi/api/internal/spasial/routes"
	sumberDataRoutes "github.com/garda-inflasi/api/internal/sumber-data/routes"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/platform/mongodb"
	"github.com/garda-inflasi/api/pkg/seed"
	"github.com/garda-inflasi/api/pkg/token"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const jwtSecret = "ml-integration-secret-key-32chars!!!"

// Akun demo per peran (db/seeds/demo/01_auth_users.sql).
const (
	emailAdmin       = "superadmin@gardainflasi.id"           // ADMIN_SISTEM: seluruh izin
	emailBI          = "strategis.bi@gardainflasi.id"         // prediksi.lihat + sumber.lihat + ews.lihat
	emailData        = "manajer.data@gardainflasi.id"         // prediksi.lihat + sumber.lihat
	emailKebijakan   = "manajer.kebijakan@gardainflasi.id"    // prediksi.lihat tanpa sumber.lihat
	emailPembiayaan  = "manajer.pembiayaan@gardainflasi.id"   // dashboard.eksekutif tanpa prediksi.lihat/sumber.lihat
	emailKoordinator = "koordinator.lapangan@gardainflasi.id" // OPS_KOORDINATOR kabupaten 1171: ews.lihat saja
	emailMahasiswa   = "mahasiswa.1@gardainflasi.id"          // tanpa izin ML
)

// pgBersama adalah basis data demo lengkap yang dipakai tes API (hanya dibaca).
var (
	pgOnce sync.Once
	pgDemo *gorm.DB
)

func dbDemo(t *testing.T) *gorm.DB {
	t.Helper()
	butuhDocker(t)
	pgOnce.Do(func() {
		db := pg.CreateDatabase(t, "ml_api_demo")
		pgtest.MigrateUp(t, db)
		require.NoError(t, seed.SeedReference(context.Background(), db))
		require.NoError(t, seed.SeedDemo(context.Background(), db))
		pgDemo = db
	})
	require.NotNil(t, pgDemo, "basis data demo gagal dibuat")
	return pgDemo
}

type apiUji struct {
	router *gin.Engine
	tokens map[string]string
}

// tokenPerDB menyimpan token per basis data: sesi hanya sah di basis data tempat login dilakukan.
var tokenPerDB = map[*gorm.DB]map[string]string{}

// routerUji merakit router seperti cmd/api: auth, lalu ML (pengaya) sebelum modul lain.
func routerUji(t *testing.T, db *gorm.DB, svc *usecase.Service) *apiUji {
	t.Helper()
	tokens, log := token.NewManager(jwtSecret), logger.New(logger.ERROR, io.Discard)
	router := gin.New()
	v1 := router.Group("/v1")
	authRoutes.Register(v1, &authRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
	mlRoutes.Register(v1, &mlRoutes.Dependencies{DB: db, Token: tokens, Logger: log, Service: svc})
	eksekutifRoutes.Register(v1, &eksekutifRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
	dashboardRoutes.Register(v1, &dashboardRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
	ewsRoutes.Register(v1, &ewsRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
	prediksiRoutes.Register(v1, &prediksiRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
	sumberDataRoutes.Register(v1, &sumberDataRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
	spasialRoutes.Register(v1, &spasialRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
	kpiRoutes.Register(v1, &kpiRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
	policyBriefRoutes.Register(v1, &policyBriefRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
	if tokenPerDB[db] == nil {
		tokenPerDB[db] = map[string]string{}
	}
	return &apiUji{router: router, tokens: tokenPerDB[db]}
}

func (a *apiUji) login(t *testing.T, email string) string {
	t.Helper()
	if tok, ok := a.tokens[email]; ok {
		return tok
	}
	body, _ := json.Marshal(map[string]string{"email": email, "password": seed.DemoPassword})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	a.router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "login %s: %s", email, w.Body.String())
	var res struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	a.tokens[email] = res.Data.AccessToken
	return res.Data.AccessToken
}

func (a *apiUji) get(t *testing.T, path, email string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if email != "" {
		req.Header.Set("Authorization", "Bearer "+a.login(t, email))
	}
	w := httptest.NewRecorder()
	a.router.ServeHTTP(w, req)
	return w
}

// amplop adalah respons standar dengan lampiran ml mentah.
type amplop struct {
	Code string          `json:"code"`
	Data json.RawMessage `json:"data"`
	ML   json.RawMessage `json:"ml"`
}

func baca(t *testing.T, w *httptest.ResponseRecorder) amplop {
	t.Helper()
	var a amplop
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &a), w.Body.String())
	return a
}

func urai[T any](t *testing.T, raw json.RawMessage) T {
	t.Helper()
	var v T
	require.NoError(t, json.Unmarshal(raw, &v), string(raw))
	return v
}

// Bentuk lampiran yang diperiksa tes (subset kontrak §5).
type meta struct {
	Tersedia       bool   `json:"tersedia"`
	Alasan         string `json:"alasan"`
	SumberData     string `json:"sumber_data"`
	CycleID        string `json:"cycle_id"`
	CycleTimestamp string `json:"cycle_timestamp"`
}

type titik struct {
	Horizon int      `json:"horizon"`
	Tanggal string   `json:"tanggal"`
	P10     *float64 `json:"p10"`
	P50     *float64 `json:"p50"`
	P90     *float64 `json:"p90"`
}

type forecastItem struct {
	KomoditasKode string  `json:"komoditas_kode"`
	KabupatenKode string  `json:"kabupaten_kode"`
	Kurva         []titik `json:"kurva"`
	Model         struct {
		Nama           string `json:"nama"`
		AdalahBaseline bool   `json:"adalah_baseline"`
		Fallback       bool   `json:"fallback"`
	} `json:"model"`
	Metrik struct {
		MAPE                *float64 `json:"mape"`
		RMSE                *float64 `json:"rmse"`
		Baseline            *string  `json:"baseline"`
		MapeBaseline        *float64 `json:"mape_baseline"`
		MengalahkanBaseline *bool    `json:"mengalahkan_baseline"`
		NLipatanSeleksi     *int     `json:"n_lipatan_seleksi"`
	} `json:"metrik"`
	Riwayat    []map[string]any `json:"riwayat"`
	Penjelasan *struct {
		Metode           string           `json:"metode"`
		SatuanKontribusi *string          `json:"satuan_kontribusi"`
		Faktor           []map[string]any `json:"faktor"`
	} `json:"penjelasan"`
}

type sinyalItem struct {
	SinyalKunci   string  `json:"sinyal_kunci"`
	Severity      string  `json:"severity"`
	KabupatenKode *string `json:"kabupaten_kode"`
	MenjadiAlert  bool    `json:"menjadi_alert"`
	Alert         *struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	} `json:"alert"`
}

type lampiran struct {
	Tersedia  bool   `json:"tersedia"`
	Alasan    string `json:"alasan"`
	DataUtama string `json:"data_utama"`
	Siklus    *struct {
		CycleID       string  `json:"cycle_id"`
		Status        string  `json:"status"`
		TargetInflasi *string `json:"target_inflasi"`
		LabelTarget   *string `json:"label_target"`
	} `json:"siklus"`
	Inflasi *struct {
		meta
		Target            *string `json:"target"`
		LabelTarget       *string `json:"label_target"`
		BukanInflasiResmi bool    `json:"bukan_inflasi_resmi"`
		Kelayakan         string  `json:"kelayakan"`
		AlasanKelayakan   *string `json:"alasan_kelayakan"`
		Wilayah           []struct {
			WilayahKode     string  `json:"wilayah_kode"`
			Target          string  `json:"target"`
			Kelas           *string `json:"kelas"`
			Layak           *bool   `json:"layak"`
			Kelayakan       string  `json:"kelayakan"`
			AlasanKelayakan *string `json:"alasan_kelayakan"`
			Proyeksi        struct {
				Yoy *float64 `json:"yoy"`
				Mtm *float64 `json:"mtm"`
			} `json:"proyeksi"`
			Penjelasan *map[string]any `json:"penjelasan"`
		} `json:"wilayah"`
	} `json:"inflasi"`
	ForecastRingkas *struct {
		meta
		KabupatenKode   *string        `json:"kabupaten_kode"`
		WilayahTersedia []string       `json:"wilayah_tersedia"`
		Items           []forecastItem `json:"items"`
	} `json:"forecast_ringkas"`
	Forecast *struct {
		meta
		WilayahTersedia []string      `json:"wilayah_tersedia"`
		Item            *forecastItem `json:"item"`
	} `json:"forecast"`
	Model *struct {
		meta
		Regresi *struct {
			MapeMedian         *float64       `json:"mape_median"`
			MapeBaselineMedian *float64       `json:"mape_baseline_median"`
			Pemenang           map[string]int `json:"pemenang"`
		} `json:"regresi"`
	} `json:"model"`
	Ews *struct {
		meta
		Ringkasan map[string]int `json:"ringkasan"`
		Sinyal    []sinyalItem   `json:"sinyal"`
	} `json:"ews"`
	Wilayah *struct {
		meta
		Items []struct {
			WilayahKode string `json:"wilayah_kode"`
		} `json:"items"`
	} `json:"wilayah"`
	Kesegaran *struct {
		meta
		Ringkasan map[string]int `json:"ringkasan"`
		Sumber    []struct {
			Sumber      string `json:"sumber"`
			Dataset     string `json:"dataset"`
			Kategori    string `json:"kategori"`
			RunTerakhir *struct {
				RunID  string `json:"run_id"`
				Pemicu string `json:"pemicu"`
			} `json:"run_terakhir"`
			BackfillTerakhir *struct {
				RunID string `json:"run_id"`
			} `json:"backfill_terakhir"`
		} `json:"sumber"`
		SinyalSumber []sinyalItem `json:"sinyal_sumber"`
	} `json:"kesegaran"`
	Makro *struct {
		meta
		Items []map[string]any `json:"items"`
	} `json:"makro"`
}

func ml(t *testing.T, w *httptest.ResponseRecorder) lampiran {
	t.Helper()
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	a := baca(t, w)
	require.NotEmpty(t, a.ML, "respons sukses endpoint yang diperkaya wajib membawa ml: %s", w.Body.String())
	return urai[lampiran](t, a.ML)
}

// apiStandar adalah router dengan Mongo berfixture standar dan router pembanding tanpa Mongo.
func apiStandar(t *testing.T) (dengan, tanpa *apiUji) {
	t.Helper()
	db := dbDemo(t)
	m := mongoBaru(t)
	isiStandar(t, m)
	return routerUji(t, db, m.service(t)), routerUji(t, db, usecase.New(nil, "", nil))
}

func TestAPI_RBACEndpointBaru(t *testing.T) {
	dengan, _ := apiStandar(t)
	kasus := []struct {
		path  string
		email string
		kode  int
	}{
		{"/v1/ml/cycles", "", http.StatusUnauthorized},
		{"/v1/ml/cycles", emailAdmin, http.StatusOK},
		{"/v1/ml/cycles", emailBI, http.StatusOK},
		{"/v1/ml/cycles", emailKebijakan, http.StatusOK},
		{"/v1/ml/cycles", emailPembiayaan, http.StatusForbidden},
		{"/v1/ml/cycles", emailKoordinator, http.StatusForbidden},
		{"/v1/ml/cycles", emailMahasiswa, http.StatusForbidden},
		{"/v1/ml/cycles/" + c09, "", http.StatusUnauthorized},
		{"/v1/ml/cycles/" + c09, emailAdmin, http.StatusOK},
		{"/v1/ml/cycles/" + c09, emailKebijakan, http.StatusOK},
		{"/v1/ml/cycles/" + c09, emailPembiayaan, http.StatusForbidden},
		{"/v1/ml/freshness", "", http.StatusUnauthorized},
		{"/v1/ml/freshness", emailAdmin, http.StatusOK},
		{"/v1/ml/freshness", emailData, http.StatusOK},
		{"/v1/ml/freshness", emailBI, http.StatusOK},
		{"/v1/ml/freshness", emailKebijakan, http.StatusForbidden},
		{"/v1/ml/freshness", emailKoordinator, http.StatusForbidden},
	}
	for _, k := range kasus {
		w := dengan.get(t, k.path, k.email)
		assert.Equalf(t, k.kode, w.Code, "%s sebagai %q: %s", k.path, k.email, w.Body.String())
	}
}

func TestAPI_DaftarDanDetailSiklus(t *testing.T) {
	dengan, _ := apiStandar(t)

	type daftar struct {
		Tersedia bool `json:"tersedia"`
		Total    int  `json:"total"`
		Items    []struct {
			CycleID string `json:"cycle_id"`
			Status  string `json:"status"`
			Final   bool   `json:"final"`
		} `json:"items"`
	}
	w := dengan.get(t, "/v1/ml/cycles?limit=2", emailBI)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	d := urai[daftar](t, baca(t, w).Data)
	assert.True(t, d.Tersedia)
	assert.Equal(t, 4, d.Total)
	assert.Equal(t, "4", w.Header().Get("X-Total-Count"))
	require.Len(t, d.Items, 2)
	assert.Equal(t, c10, d.Items[0].CycleID)
	assert.False(t, d.Items[0].Final, "BERJALAN bukan final")

	d = urai[daftar](t, baca(t, dengan.get(t, "/v1/ml/cycles?status=FINAL", emailBI)).Data)
	assert.Equal(t, 2, d.Total)
	d = urai[daftar](t, baca(t, dengan.get(t, "/v1/ml/cycles?status=gagal&dari=2026-11-24&sampai=2026-11-24", emailBI)).Data)
	assert.Equal(t, 1, d.Total, "tanggal WIB 2026-11-24 = 23 Nov 17:00Z s.d. 24 Nov 17:00Z")
	for _, salah := range []string{"?status=SELESAI", "?limit=1000", "?page=0", "?dari=24-11-2026", "?dari=2026-11-25&sampai=2026-11-24"} {
		assert.Equalf(t, http.StatusBadRequest, dengan.get(t, "/v1/ml/cycles"+salah, emailBI).Code, salah)
	}

	type detail struct {
		CycleID         string           `json:"cycle_id"`
		Status          string           `json:"status"`
		LabelTarget     *string          `json:"label_target"`
		SourceFreshness []map[string]any `json:"source_freshness"`
		ModelMetrics    struct {
			Klasifikasi map[string]any `json:"klasifikasi"`
			Regresi     map[string]any `json:"regresi"`
		} `json:"model_metrics"`
	}
	x := urai[detail](t, baca(t, dengan.get(t, "/v1/ml/cycles/"+c09, emailAdmin)).Data)
	assert.Equal(t, "SEBAGIAN", x.Status)
	require.NotNil(t, x.LabelTarget)
	assert.Contains(t, *x.LabelTarget, "bukan inflasi resmi")
	assert.Len(t, x.SourceFreshness, 2, "pemegang sumber.lihat melihat source_freshness")
	assert.NotContains(t, x.ModelMetrics.Regresi, "params", "parameter model tidak dipublikasikan")
	assert.Equal(t, 4.1, x.ModelMetrics.Regresi["mape_baseline_median"])

	assert.Equal(t, "TIDAK_DIKETAHUI", x.ModelMetrics.Klasifikasi["kelayakan"], "format lama tanpa layak")
	assert.Nil(t, x.ModelMetrics.Klasifikasi["layak"])
	baru := urai[detail](t, baca(t, dengan.get(t, "/v1/ml/cycles/"+c08, emailAdmin)).Data)
	assert.Equal(t, "TIDAK_LAYAK", baru.ModelMetrics.Klasifikasi["kelayakan"])
	assert.Equal(t, false, baru.ModelMetrics.Klasifikasi["layak"])
	assert.Equal(t, 0.24, baru.ModelMetrics.Klasifikasi["brier_baseline"])
	assert.NotEmpty(t, baru.ModelMetrics.Klasifikasi["alasan"])

	x = urai[detail](t, baca(t, dengan.get(t, "/v1/ml/cycles/"+c09, emailKebijakan)).Data)
	assert.Empty(t, x.SourceFreshness, "tanpa sumber.lihat source_freshness tidak dikirim")

	assert.Equal(t, http.StatusNotFound, dengan.get(t, "/v1/ml/cycles/C20200101T00Z", emailBI).Code)
	assert.Equal(t, http.StatusBadRequest, dengan.get(t, "/v1/ml/cycles/terbaru", emailBI).Code)
}

type kesegaranUji struct {
	meta
	Ringkasan map[string]int `json:"ringkasan"`
	Siklus    *struct {
		CycleID string `json:"cycle_id"`
	} `json:"siklus"`
	Sumber []struct {
		Sumber      string `json:"sumber"`
		Kategori    string `json:"kategori"`
		RunTerakhir *struct {
			RunID string `json:"run_id"`
		} `json:"run_terakhir"`
		BackfillTerakhir *struct {
			RunID string `json:"run_id"`
		} `json:"backfill_terakhir"`
	} `json:"sumber"`
	SinyalSumber []sinyalItem `json:"sinyal_sumber"`
}

func TestAPI_Freshness(t *testing.T) {
	dengan, _ := apiStandar(t)
	w := dengan.get(t, "/v1/ml/freshness", emailData)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	k := urai[kesegaranUji](t, baca(t, w).Data)
	assert.True(t, k.Tersedia)
	assert.Equal(t, "DATAUNSEEN", k.SumberData)
	require.NotNil(t, k.Siklus)
	assert.Equal(t, c09, k.Siklus.CycleID)
	per := map[string]string{}
	for _, s := range k.Sumber {
		per[s.Sumber] = s.Kategori
		if s.Sumber == "PIHPS" {
			require.NotNil(t, s.RunTerakhir)
			assert.Equal(t, "run-pihps-2", s.RunTerakhir.RunID, "BACKFILL tidak dinilai untuk kesegaran")
			require.NotNil(t, s.BackfillTerakhir)
			assert.Equal(t, "run-pihps-bf", s.BackfillTerakhir.RunID)
		}
	}
	assert.Equal(t, map[string]string{"PIHPS": "GAGAL", "BMKG": "DILEWATI", "BPS": "BELUM_ADA_RUN_TERJADWAL"}, per)
	assert.Equal(t, map[string]int{"segar": 0, "basi": 0, "gagal": 1, "dilewati": 1, "belum_ada_run_terjadwal": 1}, k.Ringkasan)
	kunci := map[string]bool{}
	for _, s := range k.SinyalSumber {
		kunci[s.SinyalKunci] = s.MenjadiAlert
	}
	assert.Equal(t, map[string]bool{kDilewati: false, kGagal: true}, kunci, "INFO (dilewati) tetap muncul tetapi tidak menjadi alert")
}

func TestAPI_PengayaanPrediksi(t *testing.T) {
	dengan, tanpa := apiStandar(t)

	w := dengan.get(t, "/v1/prediksi", emailBI)
	l := ml(t, w)
	assert.True(t, l.Tersedia)
	assert.Equal(t, "POSTGRES", l.DataUtama)
	require.NotNil(t, l.Siklus)
	assert.Equal(t, c09, l.Siklus.CycleID)
	require.NotNil(t, l.ForecastRingkas)
	fr := l.ForecastRingkas
	assert.True(t, fr.Tersedia)
	assert.Equal(t, "DATASAFE", fr.SumberData)
	assert.Equal(t, c09, fr.CycleID)
	assert.NotEmpty(t, fr.CycleTimestamp)
	assert.Equal(t, []string{"1171", "1174"}, fr.WilayahTersedia)
	require.Len(t, fr.Items, 1, "bawaan kabupaten 1171 seperti data")
	assert.Equal(t, "CBM-KRT", fr.Items[0].KomoditasKode)
	require.NotNil(t, l.Model)
	require.NotNil(t, l.Model.Regresi)
	assert.Equal(t, 4.1, *l.Model.Regresi.MapeBaselineMedian)

	// data tidak diubah: identik dengan respons tanpa Mongo, dan penanda DEMO miliknya tetap.
	wt := tanpa.get(t, "/v1/prediksi", emailBI)
	require.Equal(t, http.StatusOK, wt.Code)
	assert.JSONEq(t, string(baca(t, wt).Data), string(baca(t, w).Data))
	data := urai[[]struct {
		Sumber string `json:"sumber"`
	}](t, baca(t, w).Data)
	require.NotEmpty(t, data)
	assert.Equal(t, "DEMO", data[0].Sumber, "angka DEMO tetap berlabel DEMO")

	// Wilayah tanpa forecast: dinyatakan tidak tersedia, bukan diisi angka lain.
	l = ml(t, dengan.get(t, "/v1/prediksi?kabupaten=1101", emailBI))
	assert.False(t, l.ForecastRingkas.Tersedia)
	assert.Equal(t, "WILAYAH_TANPA_DATA", l.ForecastRingkas.Alasan)
	assert.Empty(t, l.ForecastRingkas.Items)
	assert.Equal(t, []string{"1171", "1174"}, l.ForecastRingkas.WilayahTersedia)

	// Detail: kurva P10/P50/P90 horizon 7/14/30, metrik vs baseline, model, XAI, riwayat.
	l = ml(t, dengan.get(t, "/v1/prediksi/CBM-KRT?kabupaten=1171", emailBI))
	require.NotNil(t, l.Forecast)
	require.True(t, l.Forecast.Tersedia, l.Forecast.Alasan)
	it := l.Forecast.Item
	require.NotNil(t, it)
	require.Len(t, it.Kurva, 3)
	for i, h := range []int{7, 14, 30} {
		assert.Equal(t, h, it.Kurva[i].Horizon)
		assert.Less(t, *it.Kurva[i].P10, *it.Kurva[i].P50)
		assert.Less(t, *it.Kurva[i].P50, *it.Kurva[i].P90)
	}
	assert.Equal(t, "2026-12-01", it.Kurva[0].Tanggal)
	assert.Equal(t, "lightgbm_quantile", it.Model.Nama)
	require.NotNil(t, it.Metrik.Baseline)
	assert.Equal(t, "seasonal_naive", *it.Metrik.Baseline)
	assert.Equal(t, 4.0, *it.Metrik.MapeBaseline)
	assert.Equal(t, 2.9, *it.Metrik.MAPE)
	assert.True(t, *it.Metrik.MengalahkanBaseline)
	assert.Len(t, it.Riwayat, 2)
	require.NotNil(t, it.Penjelasan)
	assert.Equal(t, "shap", it.Penjelasan.Metode)
	assert.NotEmpty(t, it.Penjelasan.Faktor)
	assert.Nil(t, it.Penjelasan.SatuanKontribusi, "dokumen lama tanpa satuan_kontribusi")
	assert.False(t, it.Model.AdalahBaseline)
	assert.Equal(t, 3, *it.Metrik.NLipatanSeleksi)

	l = ml(t, dengan.get(t, "/v1/prediksi/CBM-KRT?kabupaten=1171&cycle_id="+c08, emailBI))
	require.NotNil(t, l.Forecast.Item)
	require.NotNil(t, l.Forecast.Item.Penjelasan.SatuanKontribusi)
	assert.Equal(t, "poin_persen_harga", *l.Forecast.Item.Penjelasan.SatuanKontribusi)

	l = ml(t, dengan.get(t, "/v1/prediksi/BRS-MED?kabupaten=1171", emailBI))
	assert.False(t, l.Forecast.Tersedia)
	assert.Equal(t, "WILAYAH_TANPA_DATA", l.Forecast.Alasan)
	assert.Empty(t, l.Forecast.WilayahTersedia, "BRS-MED belum punya forecast di wilayah mana pun")

	// ?cycle_id=: siklus lama, siklus belum final, format salah, tidak ada. Data tetap 200.
	l = ml(t, dengan.get(t, "/v1/prediksi?cycle_id="+c08, emailBI))
	assert.Equal(t, c08, l.Siklus.CycleID)
	assert.Equal(t, c08, l.ForecastRingkas.CycleID)
	for cycle, alasan := range map[string]string{c10: "SIKLUS_BELUM_FINAL", c07: "SIKLUS_BELUM_FINAL", "kemarin": "CYCLE_ID_TIDAK_VALID", "C20200101T00Z": "SIKLUS_TIDAK_DITEMUKAN"} {
		l = ml(t, dengan.get(t, "/v1/prediksi?cycle_id="+cycle, emailBI))
		assert.False(t, l.Tersedia, cycle)
		assert.Equal(t, alasan, l.Alasan, cycle)
		assert.False(t, l.ForecastRingkas.Tersedia, cycle)
	}

	// Ekspor CSV dan respons galat tidak membawa ml.
	w = dengan.get(t, "/v1/prediksi/TIDAK-ADA?kabupaten=1171", emailBI)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Empty(t, baca(t, w).ML)
}

func TestAPI_PengayaanEksekutifIzinDanLabel(t *testing.T) {
	dengan, _ := apiStandar(t)
	l := ml(t, dengan.get(t, "/v1/dashboard/eksekutif", emailBI))
	require.NotNil(t, l.Inflasi)
	assert.True(t, l.Inflasi.Tersedia)
	require.NotNil(t, l.Inflasi.Target)
	assert.Equal(t, "INDEKS_PANGAN", *l.Inflasi.Target)
	assert.True(t, l.Inflasi.BukanInflasiResmi, "indeks pangan bukan inflasi resmi")
	require.NotNil(t, l.Inflasi.LabelTarget)
	assert.Contains(t, *l.Inflasi.LabelTarget, "bukan inflasi resmi")
	var kode []string
	for _, w := range l.Inflasi.Wilayah {
		kode = append(kode, w.WilayahKode)
		assert.Nil(t, w.Proyeksi.Yoy, "yoy null tanpa BPS, tidak diisi")
		assert.NotNil(t, w.Proyeksi.Mtm)
	}
	assert.Equal(t, []string{"1107", "1171", "1174"}, kode, "provinsi 11 tidak punya sinyal dan tidak diisi")
	// C09 format lama (tanpa `layak`): kelayakan tidak diketahui, bukan layak; kelas diteruskan apa adanya.
	assert.Equal(t, "TIDAK_DIKETAHUI", l.Inflasi.Kelayakan)
	for _, w := range l.Inflasi.Wilayah {
		assert.Nil(t, w.Layak)
		assert.Equal(t, "TIDAK_DIKETAHUI", w.Kelayakan)
		require.NotNil(t, w.Kelas)
	}

	// C08 format baru: klasifikasi tidak layak, kelas null beserta alasan, XAI inflasi tidak terbit.
	l = ml(t, dengan.get(t, "/v1/dashboard/eksekutif?cycle_id="+c08, emailBI))
	require.True(t, l.Inflasi.Tersedia)
	assert.Equal(t, "TIDAK_LAYAK", l.Inflasi.Kelayakan)
	require.NotNil(t, l.Inflasi.AlasanKelayakan)
	for _, w := range l.Inflasi.Wilayah {
		assert.Nil(t, w.Kelas, "tidak layak: kelas null, bukan prediksi")
		require.NotNil(t, w.Layak)
		assert.False(t, *w.Layak)
		assert.Equal(t, "TIDAK_LAYAK", w.Kelayakan)
		assert.NotNil(t, w.AlasanKelayakan)
		assert.Nil(t, w.Penjelasan, "XAI inflasi tidak ada bila klasifikasi tidak layak")
	}
	assert.True(t, l.Model.Tersedia)
	assert.True(t, l.Ews.Tersedia)
	assert.True(t, l.Kesegaran.Tersedia)
	assert.False(t, l.Makro.Tersedia)
	assert.Equal(t, "TIDAK_ADA_DATA", l.Makro.Alasan)

	// dashboard.eksekutif tanpa prediksi.lihat dan sumber.lihat: blok dinyatakan tanpa izin, bukan dibuka.
	l = ml(t, dengan.get(t, "/v1/dashboard/eksekutif", emailPembiayaan))
	assert.False(t, l.Inflasi.Tersedia)
	assert.Equal(t, "TANPA_IZIN", l.Inflasi.Alasan)
	assert.Empty(t, l.Inflasi.Wilayah)
	assert.Equal(t, "TANPA_IZIN", l.Model.Alasan)
	assert.Equal(t, "TANPA_IZIN", l.Kesegaran.Alasan)
	assert.Equal(t, "TANPA_IZIN", l.Ews.Alasan)
}

func TestAPI_PengayaanEwsCakupanBaris(t *testing.T) {
	dengan, _ := apiStandar(t)
	l := ml(t, dengan.get(t, "/v1/ews", emailAdmin))
	require.NotNil(t, l.Ews)
	kunci := map[string]bool{}
	for _, s := range l.Ews.Sinyal {
		kunci[s.SinyalKunci] = s.MenjadiAlert
	}
	assert.Equal(t, map[string]bool{kAnomali: true, kProyeksi: true, kTekanan: true, kDilewati: false, kGagal: true}, kunci)
	assert.Equal(t, "SIAGA", l.Ews.Sinyal[0].Severity, "paling parah lebih dulu")
	assert.Equal(t, 5, l.Ews.Ringkasan["aktif"])

	// OPS_KOORDINATOR kabupaten 1171 hanya melihat sinyal kabupatennya; sinyal tingkat sistem tidak.
	l = ml(t, dengan.get(t, "/v1/ews", emailKoordinator))
	require.Len(t, l.Ews.Sinyal, 1)
	assert.Equal(t, kAnomali, l.Ews.Sinyal[0].SinyalKunci)

	// OPS_KOORDINATOR di dashboard operasional: blok forecast tanpa prediksi.lihat.
	l = ml(t, dengan.get(t, "/v1/dashboard/operasional/kpi", emailKoordinator))
	assert.Equal(t, "TANPA_IZIN", l.ForecastRingkas.Alasan)
	require.Len(t, l.Ews.Sinyal, 1)
}

func TestAPI_PengayaanModulLain(t *testing.T) {
	dengan, _ := apiStandar(t)
	l := ml(t, dengan.get(t, "/v1/sumber-data", emailAdmin))
	require.NotNil(t, l.Kesegaran)
	assert.True(t, l.Kesegaran.Tersedia)
	assert.Equal(t, 1, l.Kesegaran.Ringkasan["gagal"])

	l = ml(t, dengan.get(t, "/v1/spasial/wilayah", emailAdmin))
	require.NotNil(t, l.Wilayah)
	var kode []string
	for _, w := range l.Wilayah.Items {
		kode = append(kode, w.WilayahKode)
	}
	assert.Equal(t, []string{"1107", "1171", "1174"}, kode)

	l = ml(t, dengan.get(t, "/v1/kpi", emailAdmin))
	require.NotNil(t, l.Model)
	assert.True(t, l.Model.Tersedia)

	l = ml(t, dengan.get(t, "/v1/policy-brief", emailAdmin))
	require.NotNil(t, l.Inflasi)
	assert.True(t, l.Inflasi.Tersedia)
	require.NotNil(t, l.Makro)

	l = ml(t, dengan.get(t, "/v1/dashboard/operasional/heatmap?kabupaten_kode=1174", emailAdmin))
	require.NotNil(t, l.ForecastRingkas)
	require.Len(t, l.ForecastRingkas.Items, 1)
	assert.Equal(t, "BWM", l.ForecastRingkas.Items[0].KomoditasKode)
}

// Tanpa konfigurasi Mongo: endpoint lama tetap 200 dengan data yang sama, lampiran menyatakan tidak dikonfigurasi.
func TestAPI_TanpaMongo(t *testing.T) {
	dengan, tanpa := apiStandar(t)
	for _, path := range []string{"/v1/prediksi", "/v1/ews", "/v1/dashboard/eksekutif", "/v1/sumber-data", "/v1/kpi", "/v1/spasial/wilayah", "/v1/policy-brief"} {
		w := tanpa.get(t, path, emailAdmin)
		l := ml(t, w)
		assert.False(t, l.Tersedia, path)
		assert.Equal(t, "TIDAK_DIKONFIGURASI", l.Alasan, path)
		assert.Equal(t, "POSTGRES", l.DataUtama, path)
	}
	assert.JSONEq(t, string(baca(t, dengan.get(t, "/v1/ews", emailAdmin)).Data), string(baca(t, tanpa.get(t, "/v1/ews", emailAdmin)).Data))

	w := tanpa.get(t, "/v1/ml/cycles", emailAdmin)
	require.Equal(t, http.StatusOK, w.Code)
	d := urai[map[string]any](t, baca(t, w).Data)
	assert.Equal(t, false, d["tersedia"])
	assert.Equal(t, "TIDAK_DIKONFIGURASI", d["alasan"])
	assert.Equal(t, []any{}, d["items"])
	assert.Equal(t, http.StatusServiceUnavailable, tanpa.get(t, "/v1/ml/cycles/"+c09, emailAdmin).Code)
	w = tanpa.get(t, "/v1/ml/freshness", emailAdmin)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, false, urai[map[string]any](t, baca(t, w).Data)["tersedia"])
}

// Mongo mati: endpoint lama tidak jatuh, tidak menunggu lama, dan data utuh.
func TestAPI_MongoMatiTidakMenjatuhkanEndpointLama(t *testing.T) {
	db := dbDemo(t)
	k, err := mongodb.Buka(mongodb.Config{URI: "mongodb://127.0.0.1:1/?directConnection=true", DBUnseen: "u", DBSafe: "s", Timeout: 500 * time.Millisecond})
	require.NoError(t, err)
	t.Cleanup(func() { _ = k.Tutup(context.Background()) })
	mati := routerUji(t, db, usecase.New(mlrepo.New(k), "", nil))
	tanpa := routerUji(t, db, usecase.New(nil, "", nil))

	for _, path := range []string{"/v1/prediksi", "/v1/prediksi/CBM-KRT", "/v1/ews", "/v1/dashboard/eksekutif", "/v1/sumber-data", "/v1/kpi", "/v1/policy-brief"} {
		mulai := time.Now()
		w := mati.get(t, path, emailAdmin)
		durasi := time.Since(mulai)
		l := ml(t, w)
		assert.False(t, l.Tersedia, path)
		assert.Contains(t, []string{"MONGO_TIDAK_TERSEDIA", "MONGO_LAMBAT"}, l.Alasan, path)
		assert.Less(t, durasi, 4*time.Second, path)
		if path == "/v1/prediksi" || path == "/v1/ews" {
			assert.JSONEq(t, string(baca(t, tanpa.get(t, path, emailAdmin)).Data), string(baca(t, w).Data), path)
		}
	}
	w := mati.get(t, "/v1/ml/cycles/"+c09, emailAdmin)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}
