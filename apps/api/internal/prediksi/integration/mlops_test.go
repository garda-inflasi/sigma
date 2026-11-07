package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	authRoutes "github.com/garda-inflasi/api/internal/auth/routes"
	authseeder "github.com/garda-inflasi/api/internal/auth/seeder"
	masterseeder "github.com/garda-inflasi/api/internal/master/seeder"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/internal/prediksi/mlclient"
	prediksiRoutes "github.com/garda-inflasi/api/internal/prediksi/routes"
	prediksiseeder "github.com/garda-inflasi/api/internal/prediksi/seeder"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/token"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// emailKelola adalah MANAJER_DATA: prediksi.lihat dan model.kelola.
const emailKelola = "manajer.data@gardainflasi.id"

// kabupatenRiwayat adalah kabupaten yang diberi riwayat harga cabai 90 hari.
var kabupatenRiwayat = map[string]string{"1171": "PSR-1171-01", "1108": "PSR-1108-01"}

// newMLSuite membuat basis data terpisah (registri tidak boleh bercampur dengan
// suite bersama) lengkap dengan riwayat harga 90 hari, dan router yang memakai klien ML `ml`.
func newMLSuite(t *testing.T, name string, ml mlclient.Client) *suite {
	t.Helper()
	db := pg.CreateDatabase(t, name)
	pgtest.MigrateUp(t, db)
	_, err := masterseeder.SeedReference(t.Context(), db)
	require.NoError(t, err)
	_, err = authseeder.SeedDemo(t.Context(), db)
	require.NoError(t, err)
	_, err = prediksiseeder.SeedDemo(t.Context(), db, fixedNow)
	require.NoError(t, err)
	seedRiwayat90(t, db)

	tokens, log := token.NewManager(jwtSecret), logger.NewDefault()
	router := gin.New()
	v1 := router.Group("/v1")
	authRoutes.Register(v1, &authRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
	prediksiRoutes.Register(v1, &prediksiRoutes.Dependencies{DB: db, Token: tokens, Logger: log, ML: ml})
	return &suite{db: db, router: router, tokens: map[string]string{}}
}

// hariIni adalah awal hari UTC saat tes berjalan; riwayat dan evaluasi memakai jam nyata.
func hariIni() time.Time { return time.Now().UTC().Truncate(24 * time.Hour) }

// seedRiwayat90 menanam harga konsumen tervalidasi cabai harian selama 90 hari terakhir
// (kemarin dan sebelumnya) di dua kabupaten, ditambah entri yang harus diabaikan.
func seedRiwayat90(t *testing.T, db *gorm.DB) {
	t.Helper()
	for kab, pasar := range kabupatenRiwayat {
		for i := 1; i <= 90; i++ {
			require.NoError(t, db.Exec(`
				INSERT INTO entri_harga (kode, komoditas_kode, pasar_id, tipe, harga, satuan, status, dicatat_pada)
				VALUES (?, 'CBM-KRT', ?, 'KONSUMEN', ?, 'kg', 'DIVALIDASI', ?)`,
				fmt.Sprintf("H-%s-%d", kab, i), pasar, 50000+i*10, hariIni().AddDate(0, 0, -i).Add(10*time.Hour)).Error)
		}
	}
	// Belum divalidasi dan harga produsen tidak boleh ikut dilatih.
	require.NoError(t, db.Exec(`
		INSERT INTO entri_harga (kode, komoditas_kode, pasar_id, tipe, harga, satuan, status, dicatat_pada) VALUES
		('X1', 'CBM-KRT', 'PSR-1171-01', 'KONSUMEN', 999999, 'kg', 'DIAJUKAN', ?),
		('X2', 'CBM-KRT', 'PSR-1171-01', 'PRODUSEN', 1000, 'kg', 'DIVALIDASI', ?)`,
		hariIni().Add(10*time.Hour), hariIni().Add(10*time.Hour)).Error)
}

// fakeML adalah layanan ML palsu di atas httptest yang mematuhi kontrak /train dan /predict:
// meramal harga terakhir naik Rp100 per hari horizon, dan mencatat permintaan terakhir.
type fakeML struct {
	srv         *httptest.Server
	lastTrain   mlclient.TrainRequest
	lastPredict mlclient.PredictRequest
	// mape dan baseline adalah metrik ringkasan yang dilaporkan /train.
	mape, baseline float64
}

func newFakeML(t *testing.T) *fakeML {
	t.Helper()
	f := &fakeML{mape: 2.5, baseline: 3.0}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok","versi":"test"}`))
	})
	mux.HandleFunc("/train", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&f.lastTrain))
		var seri []mlclient.HasilSeri
		for _, s := range f.lastTrain.Seri {
			seri = append(seri, hasilFake(s, f.lastTrain.Horizon))
		}
		jawab(t, w, mlclient.TrainResponse{
			VersiLayanan: "test", KomoditasKode: f.lastTrain.KomoditasKode, Seri: seri,
			Ringkasan: mlclient.Ringkasan{NSeriOK: len(seri), Mape: f.mape, Rmse: 1200, BaselineMape: f.baseline, BaselineRmse: 1500, Algoritma: map[string]int{"holt": len(seri)}},
		})
	})
	mux.HandleFunc("/predict", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&f.lastPredict))
		var seri []mlclient.HasilSeri
		for _, s := range f.lastPredict.Seri {
			seri = append(seri, hasilFake(s.Seri, f.lastPredict.Horizon))
		}
		jawab(t, w, mlclient.PredictResponse{VersiLayanan: "test", KomoditasKode: f.lastPredict.KomoditasKode, Seri: seri})
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func jawab(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(v))
}

func hasilFake(s mlclient.Seri, horizon []int) mlclient.HasilSeri {
	akhir := s.Titik[len(s.Titik)-1]
	tanggal, _ := time.Parse(time.DateOnly, akhir.Tanggal)
	h := mlclient.HasilSeri{
		ID: s.ID, Status: "OK", Algoritma: "holt",
		Params:     map[string]float64{"alpha": 0.5, "beta": 0.2, "gamma": 0, "phi": 0.95},
		GalatRMSE:  map[string]float64{"7": 300, "14": 450, "30": 700},
		Metrik:     mlclient.Metrik{Mape: 2.5, Rmse: 1200, BaselineMape: 3.0, BaselineRmse: 1500},
		HargaDasar: akhir.Nilai, TanggalDasar: akhir.Tanggal, NHari: len(s.Titik),
	}
	for _, hz := range horizon {
		nilai := akhir.Nilai + 100*hz
		h.Ramalan = append(h.Ramalan, mlclient.Ramalan{
			HorizonHari: hz, TanggalTarget: tanggal.AddDate(0, 0, hz).Format(time.DateOnly),
			Nilai: nilai, Bawah: nilai - 500, Atas: nilai + 500,
			Faktor: []mlclient.Faktor{{Nama: "Tren harga", Kontribusi: 1.5, Detail: "tren"}},
		})
	}
	return h
}

func (s *suite) post(t *testing.T, path, email string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if email != "" {
		req.Header.Set("Authorization", "Bearer "+s.login(t, email))
	}
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	return w
}

type modelResp struct {
	ID            string   `json:"id"`
	Kode          *string  `json:"kode"`
	KomoditasKode string   `json:"komoditas_kode"`
	Versi         string   `json:"versi"`
	Mape          float64  `json:"mape"`
	Status        string   `json:"status"`
	Sumber        string   `json:"sumber"`
	Catatan       string   `json:"catatan"`
	BaselineMape  *float64 `json:"baseline_mape"`
	MapeRealisasi *float64 `json:"mape_realisasi"`
	NEvaluasi     int      `json:"n_evaluasi"`
}
