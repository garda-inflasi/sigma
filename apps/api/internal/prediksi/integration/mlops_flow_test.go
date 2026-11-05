package integration

import (
	"net/http"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/prediksi/mlclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (f *fakeML) client() mlclient.Client { return mlclient.New(f.srv.URL, nil) }

func (s *suite) hitung(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, s.db.Raw(query, args...).Scan(&n).Error)
	return n
}

type statusML struct {
	Dikonfigurasi bool   `json:"dikonfigurasi"`
	Tersedia      bool   `json:"tersedia"`
	Versi         string `json:"versi"`
}

func TestMLOps_Authorization(t *testing.T) {
	s := newMLSuite(t, "prediksi_ml_authz", newFakeML(t).client())

	for _, path := range []string{"/v1/prediksi/latih", "/v1/prediksi/segarkan", "/v1/prediksi/evaluasi"} {
		t.Run(path, func(t *testing.T) {
			body := map[string]string{"komoditas_kode": "CBM-KRT"}
			assert.Equal(t, http.StatusUnauthorized, s.post(t, path, "", body).Code)
			assert.Equal(t, http.StatusForbidden, s.post(t, path, emailBerhak, body).Code, "prediksi.lihat saja tidak cukup")
			assert.Equal(t, http.StatusForbidden, s.post(t, path, emailTanpaIzin, body).Code)
		})
	}
	assert.Equal(t, 0, s.hitung(t, `SELECT COUNT(*) FROM model_registry WHERE sumber = 'MODEL'`), "penolakan tidak mengubah data")
}

func TestMLOps_LatihMenulisKandidatDanProyeksiBersumberModel(t *testing.T) {
	ml := newFakeML(t)
	s := newMLSuite(t, "prediksi_ml_latih", ml.client())

	w := s.post(t, "/v1/prediksi/latih", emailKelola, map[string]string{"komoditas_kode": "CBM-KRT"})

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	m := decode[modelResp](t, w)
	assert.Equal(t, "CBM-KRT", m.KomoditasKode)
	assert.Equal(t, "KANDIDAT", m.Status)
	assert.Equal(t, "MODEL", m.Sumber)
	assert.Equal(t, 2.5, m.Mape)
	require.NotNil(t, m.BaselineMape)
	assert.Equal(t, 3.0, *m.BaselineMape)
	assert.Contains(t, m.Catatan, "backtest")

	t.Run("dilatih dari entri tervalidasi konsumen saja", func(t *testing.T) {
		require.Len(t, ml.lastTrain.Seri, 2)
		for _, seri := range ml.lastTrain.Seri {
			assert.Len(t, seri.Titik, 90, "harga ajuan dan produsen tidak ikut: %s", seri.ID)
			assert.Less(t, seri.Titik[len(seri.Titik)-1].Nilai, 100000)
		}
		assert.Equal(t, []int{7, 14, 30}, ml.lastTrain.Horizon)
	})

	t.Run("proyeksi tersimpan sebagai MODEL dengan interval berurutan", func(t *testing.T) {
		assert.Equal(t, 6, s.hitung(t, `SELECT COUNT(*) FROM prediksi_harga WHERE sumber = 'MODEL' AND model_versi = ?`, m.Versi), "2 kabupaten × 3 horizon")
		assert.Equal(t, 0, s.hitung(t, `SELECT COUNT(*) FROM prediksi_harga WHERE sumber = 'MODEL' AND NOT (bawah <= nilai AND nilai <= atas)`))
		assert.Equal(t, 1, s.hitung(t, `SELECT COUNT(*) FROM model_registry WHERE id = ? AND artefak_uri = 'ml://garda-ml/test' AND metrics->>'baseline_mape' = '3' AND jsonb_typeof(metrics->'seri'->'1171'->'params') = 'object'`, m.ID))
	})

	t.Run("tercatat di audit", func(t *testing.T) {
		assert.Equal(t, 1, s.hitung(t, `SELECT COUNT(*) FROM audit_log WHERE aksi = 'LATIH_MODEL' AND entitas_id = ? AND aktor_id = 'manajer_data'`, m.ID))
	})

	t.Run("kandidat belum dipakai: proyeksi yang tampil tetap demo", func(t *testing.T) {
		got := decode[detailResp](t, s.call(t, "/v1/prediksi/CBM-KRT", emailBerhak))
		assert.Equal(t, "DEMO", got.Sumber)
		assert.Equal(t, "v2.3.1", got.Model.Versi)
	})

	t.Run("muncul di registri berlabel MODEL", func(t *testing.T) {
		registri := decode[[]modelResp](t, s.call(t, "/v1/prediksi/registri", emailBerhak))
		var ditemukan *modelResp
		for i := range registri {
			if registri[i].ID == m.ID {
				ditemukan = &registri[i]
			}
		}
		require.NotNil(t, ditemukan)
		assert.Equal(t, "MODEL", ditemukan.Sumber)
	})
}

func TestMLOps_LatihKomoditasTanpaDataDan400(t *testing.T) {
	s := newMLSuite(t, "prediksi_ml_data", newFakeML(t).client())

	assert.Equal(t, http.StatusUnprocessableEntity, s.post(t, "/v1/prediksi/latih", emailKelola, map[string]string{"komoditas_kode": "BRS-MED"}).Code)
	assert.Equal(t, http.StatusBadRequest, s.post(t, "/v1/prediksi/latih", emailKelola, map[string]string{}).Code)
	assert.Equal(t, 0, s.hitung(t, `SELECT COUNT(*) FROM model_registry WHERE sumber = 'MODEL'`))
}

func TestMLOps_TanpaLayananMLAPITetapJalan(t *testing.T) {
	t.Setenv(mlclient.EnvURL, "")
	s := newMLSuite(t, "prediksi_ml_mati", nil)

	w := s.post(t, "/v1/prediksi/latih", emailKelola, map[string]string{"komoditas_kode": "CBM-KRT"})
	assert.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "Layanan ML tidak tersedia")
	assert.Equal(t, http.StatusServiceUnavailable, s.post(t, "/v1/prediksi/segarkan", emailKelola, map[string]string{"komoditas_kode": "CBM-KRT"}).Code)

	status := decode[statusML](t, s.call(t, "/v1/prediksi/ml", emailBerhak))
	assert.False(t, status.Dikonfigurasi)
	assert.False(t, status.Tersedia)

	// Data prediksi demo tetap tampil, berlabel DEMO.
	got := decode[[]ringkasanRow](t, s.call(t, "/v1/prediksi", emailBerhak))
	require.NotEmpty(t, got)
	assert.Equal(t, "DEMO", got[0].Sumber)
}

func TestMLOps_LayananMatiDiTengahJalan(t *testing.T) {
	ml := newFakeML(t)
	s := newMLSuite(t, "prediksi_ml_putus", ml.client())
	ml.srv.Close()

	assert.Equal(t, http.StatusServiceUnavailable, s.post(t, "/v1/prediksi/latih", emailKelola, map[string]string{"komoditas_kode": "CBM-KRT"}).Code)
	status := decode[statusML](t, s.call(t, "/v1/prediksi/ml", emailBerhak))
	assert.True(t, status.Dikonfigurasi)
	assert.False(t, status.Tersedia)
	assert.Equal(t, http.StatusOK, s.call(t, "/v1/prediksi", emailBerhak).Code)
}

func TestMLOps_StatusLayananTersedia(t *testing.T) {
	s := newMLSuite(t, "prediksi_ml_status", newFakeML(t).client())

	got := decode[statusML](t, s.call(t, "/v1/prediksi/ml", emailBerhak))

	assert.Equal(t, "test", got.Versi)
	assert.True(t, got.Tersedia)
}

// jadikanProduksi menukar model produksi demo dengan model hasil pelatihan
// (promosi lewat API ada di mlops_promosi_test.go; di sini cukup SQL).
func jadikanProduksi(t *testing.T, s *suite, m modelResp) {
	t.Helper()
	require.NoError(t, s.db.Exec(`UPDATE model_registry SET status = 'ARSIP' WHERE komoditas_kode = ? AND status = 'PRODUKSI'`, m.KomoditasKode).Error)
	require.NoError(t, s.db.Exec(`UPDATE model_registry SET status = 'PRODUKSI' WHERE id = ?`, m.ID).Error)
}

func TestMLOps_SegarkanIdempotenDanMenampilkanProyeksiModel(t *testing.T) {
	ml := newFakeML(t)
	s := newMLSuite(t, "prediksi_ml_segarkan", ml.client())
	body := map[string]string{"komoditas_kode": "CBM-KRT"}

	t.Run("model produksi demo ditolak", func(t *testing.T) {
		assert.Equal(t, http.StatusConflict, s.post(t, "/v1/prediksi/segarkan", emailKelola, body).Code)
	})
	t.Run("tanpa model produksi", func(t *testing.T) {
		assert.Equal(t, http.StatusNotFound, s.post(t, "/v1/prediksi/segarkan", emailKelola, map[string]string{"komoditas_kode": "XXX"}).Code)
	})

	m := decode[modelResp](t, s.post(t, "/v1/prediksi/latih", emailKelola, body))
	jadikanProduksi(t, s, m)
	sebelum := s.hitung(t, `SELECT COUNT(*) FROM prediksi_harga WHERE sumber = 'MODEL'`)

	for i := 0; i < 2; i++ {
		w := s.post(t, "/v1/prediksi/segarkan", emailKelola, body)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, 6, decode[struct {
			Proyeksi int `json:"proyeksi"`
		}](t, w).Proyeksi)
	}
	assert.Equal(t, sebelum, s.hitung(t, `SELECT COUNT(*) FROM prediksi_harga WHERE sumber = 'MODEL'`), "menyegarkan data yang sama tidak menambah baris")
	require.Len(t, ml.lastPredict.Seri, 2)
	assert.Equal(t, "holt", ml.lastPredict.Seri[0].Algoritma, "parameter terlatih dibawa dari registri")
	assert.Equal(t, 450.0, ml.lastPredict.Seri[0].GalatRMSE["14"])

	got := decode[detailResp](t, s.call(t, "/v1/prediksi/CBM-KRT", emailBerhak))
	assert.Equal(t, "MODEL", got.Sumber, "setelah promosi, proyeksi dari model terlatih yang tampil")
	assert.Equal(t, m.Versi, got.Model.Versi)
	assert.Equal(t, 1.5, got.Faktor[0].Kontribusi)
	assert.Equal(t, 2, s.hitung(t, `SELECT COUNT(*) FROM audit_log WHERE aksi = 'SEGARKAN_PROYEKSI'`))
}

func TestMLOps_EvaluasiMenghitungGalatNyata(t *testing.T) {
	s := newMLSuite(t, "prediksi_ml_evaluasi", newFakeML(t).client())
	m := decode[modelResp](t, s.post(t, "/v1/prediksi/latih", emailKelola, map[string]string{"komoditas_kode": "CBM-KRT"}))

	// Proyeksi yang jatuh tempo kemarin; realisasi kemarin (i=1) = 50.010. Proyeksi 51.010 => galat +2,00%.
	kemarin := hariIni().AddDate(0, 0, -1)
	require.NoError(t, s.db.Exec(`
		INSERT INTO prediksi_harga (id, komoditas_kode, kabupaten_kode, tanggal_target, horizon_hari, nilai, bawah, atas, harga_dasar, model_versi, faktor, sumber)
		VALUES ('PRM-UJI', 'CBM-KRT', '1171', ?, 7, 51010, 50000, 52000, 50000, ?, '[]', 'MODEL')`, kemarin.Format(time.DateOnly), m.Versi).Error)
	// Proyeksi demo yang jatuh tempo tidak dievaluasi.
	require.NoError(t, s.db.Exec(`UPDATE prediksi_harga SET tanggal_target = ? WHERE id = 'PRD-CBM-KRT-1171-14'`, kemarin.Format(time.DateOnly)).Error)

	for i, want := range []int{1, 0} {
		w := s.post(t, "/v1/prediksi/evaluasi", emailKelola, map[string]string{})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, want, decode[struct {
			Dievaluasi int `json:"dievaluasi"`
		}](t, w).Dievaluasi, "panggilan ke-%d", i+1)
	}

	var galat float64
	require.NoError(t, s.db.Raw(`SELECT galat_pct FROM prediksi_evaluasi WHERE prediksi_id = 'PRM-UJI'`).Scan(&galat).Error)
	assert.InDelta(t, 2.0, galat, 0.001)
	assert.Equal(t, 1, s.hitung(t, `SELECT COUNT(*) FROM prediksi_evaluasi`))

	registri := decode[[]modelResp](t, s.call(t, "/v1/prediksi/registri", emailBerhak))
	for _, r := range registri {
		if r.ID == m.ID {
			require.NotNil(t, r.MapeRealisasi)
			assert.InDelta(t, 2.0, *r.MapeRealisasi, 0.001)
			assert.Equal(t, 1, r.NEvaluasi)
			return
		}
	}
	t.Fatal("model tidak ada di registri")
}
