package integration

import (
	"bytes"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	authRoutes "github.com/garda-inflasi/api/internal/auth/routes"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	spasialRoutes "github.com/garda-inflasi/api/internal/spasial/routes"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/seed"
	"github.com/garda-inflasi/api/pkg/token"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	// STRATEGIS_BI membawa spasial.lihat tetapi tidak logistik.kelola.
	emailHanyaLihat = "strategis.bi@gardainflasi.id"
	// MANAJER_PROGRAM membawa spasial.lihat dan logistik.kelola.
	emailLogistik = "manajer.program@gardainflasi.id"
)

type optimasiResp struct {
	KomoditasKode  string  `json:"komoditas_kode"`
	KomoditasNama  string  `json:"komoditas_nama"`
	Periode        string  `json:"periode"`
	TotalVolumeTon float64 `json:"total_volume_ton"`
	TotalBiaya     int64   `json:"total_biaya"`
	OptimasiID     *string `json:"optimasi_id"`
	OptimasiKode   *string `json:"optimasi_kode"`
	// Pembanding: rencana serakah untuk volume yang sama.
	BiayaPembanding   *int64   `json:"biaya_pembanding"`
	PenghematanPersen *float64 `json:"penghematan_persen"`
	Rute              []struct {
		AsalKode   string  `json:"asal_kode"`
		TujuanKode string  `json:"tujuan_kode"`
		VolumeTon  float64 `json:"volume_ton"`
		JarakKm    float64 `json:"jarak_km"`
		JamTempuh  float64 `json:"jam_tempuh"`
		Biaya      int64   `json:"biaya"`
	} `json:"rute"`
	BelumTerpenuhi []struct {
		Kode string  `json:"kode"`
		Ton  float64 `json:"ton"`
	} `json:"belum_terpenuhi"`
	SurplusSisa []struct {
		Kode string  `json:"kode"`
		Ton  float64 `json:"ton"`
	} `json:"surplus_sisa"`
}

// writableSuite membuat basis data tersendiri (suite bersama bersifat baca-saja).
func writableSuite(t *testing.T, name string) (*suite, *gorm.DB) {
	t.Helper()
	db := pg.NewDatabase(t, name)
	pgtest.MigrateUp(t, db)
	require.NoError(t, seed.SeedReference(t.Context(), db))
	require.NoError(t, seed.SeedFiles(t.Context(), db, berkasSeedDemo...))

	tokens, log := token.NewManager(jwtSecret), logger.NewDefault()
	router := gin.New()
	v1 := router.Group("/v1")
	authRoutes.Register(v1, &authRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
	spasialRoutes.Register(v1, &spasialRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
	return &suite{router: router, tokens: map[string]string{}, periode: periodeSeed(t, db)}, db
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

const pathOptimasi = "/v1/spasial/optimasi-rute"

func TestOptimasi_Authorization(t *testing.T) {
	s, _ := writableSuite(t, "spasial_optimasi_authz")
	body := map[string]any{"komoditas_kode": "CBM-KRT"}

	assert.Equal(t, http.StatusUnauthorized, s.post(t, pathOptimasi, "", body).Code)
	assert.Equal(t, http.StatusForbidden, s.post(t, pathOptimasi, emailTanpaIzin, body).Code)
	assert.Equal(t, http.StatusOK, s.post(t, pathOptimasi, emailHanyaLihat, body).Code, "simulasi cukup spasial.lihat")
	assert.Equal(t, http.StatusForbidden, s.post(t, pathOptimasi, emailHanyaLihat, map[string]any{"komoditas_kode": "CBM-KRT", "simpan": true}).Code,
		"menyimpan butuh logistik.kelola")
	assert.Equal(t, http.StatusOK, s.post(t, pathOptimasi, emailLogistik, map[string]any{"komoditas_kode": "CBM-KRT", "simpan": true}).Code)

	assert.Equal(t, http.StatusUnauthorized, s.call(t, pathOptimasi, "").Code)
	assert.Equal(t, http.StatusForbidden, s.call(t, pathOptimasi, emailTanpaIzin).Code)
	assert.Equal(t, http.StatusOK, s.call(t, pathOptimasi, emailHanyaLihat).Code)
}

func TestOptimasi_PlanMatchesDemoBalanceAndRespectsCapacities(t *testing.T) {
	s := sharedSuite(t)

	type neracaRow struct {
		KabupatenKode     string  `json:"kabupaten_kode"`
		SurplusDefisitTon float64 `json:"surplus_defisit_ton"`
	}
	neraca := data[[]neracaRow](t, s.call(t, "/v1/spasial/neraca?komoditas=CBM-KRT", emailHanyaLihat))
	posisi := map[string]float64{}
	var surplus, defisit float64
	for _, n := range neraca {
		posisi[n.KabupatenKode] = n.SurplusDefisitTon
		if n.SurplusDefisitTon > 0 {
			surplus += n.SurplusDefisitTon
		} else {
			defisit -= n.SurplusDefisitTon
		}
	}
	require.Greater(t, defisit, 300.0, "tiga titik panas cabai (Banda Aceh, Aceh Besar, Lhokseumawe)")
	require.Greater(t, surplus, 0.0)

	w := s.post(t, pathOptimasi, emailHanyaLihat, map[string]any{"komoditas_kode": "CBM-KRT"})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	plan := data[optimasiResp](t, w)
	assert.Equal(t, "CBM-KRT", plan.KomoditasKode)
	assert.NotEmpty(t, plan.KomoditasNama)
	assert.Nil(t, plan.OptimasiID, "simulasi tidak disimpan")
	assert.InDelta(t, math.Min(surplus, defisit), plan.TotalVolumeTon, 0.05, "semua wilayah terhubung: volume tersalur = min(surplus, defisit)")

	keluar, masuk := map[string]float64{}, map[string]float64{}
	var biaya float64
	for _, r := range plan.Rute {
		assert.Positive(t, r.VolumeTon)
		assert.Greater(t, posisi[r.AsalKode], 0.0, "asal %s harus surplus", r.AsalKode)
		assert.Less(t, posisi[r.TujuanKode], 0.0, "tujuan %s harus defisit", r.TujuanKode)
		assert.Greater(t, r.JarakKm, 0.0)
		keluar[r.AsalKode] += r.VolumeTon
		masuk[r.TujuanKode] += r.VolumeTon
		biaya += r.VolumeTon * r.JarakKm * 4200
		assert.InDelta(t, r.VolumeTon*r.JarakKm*4200, float64(r.Biaya), 1, "biaya = ton x km x tarif")
	}
	for kode, ton := range keluar {
		assert.LessOrEqual(t, ton, posisi[kode]+1e-9, "pemasok %s melampaui surplus", kode)
	}
	for kode, ton := range masuk {
		assert.LessOrEqual(t, ton, -posisi[kode]+1e-9, "peminta %s melampaui defisit", kode)
	}
	assert.InDelta(t, biaya, float64(plan.TotalBiaya), float64(len(plan.Rute)), "total biaya = jumlah biaya rute")

	var belum float64
	for _, b := range plan.BelumTerpenuhi {
		belum += b.Ton
	}
	assert.InDelta(t, defisit, plan.TotalVolumeTon+belum, 0.05, "tersalur + belum terpenuhi = total defisit")
}

func TestOptimasi_ScenarioAndConstraints(t *testing.T) {
	s, _ := writableSuite(t, "spasial_optimasi_skenario")
	run := func(body map[string]any) optimasiResp {
		w := s.post(t, pathOptimasi, emailHanyaLihat, body)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		return data[optimasiResp](t, w)
	}

	dasar := run(map[string]any{"komoditas_kode": "CBM-KRT"})

	t.Run("batas waktu tempuh sangat ketat melarang semua rute", func(t *testing.T) {
		plan := run(map[string]any{"komoditas_kode": "CBM-KRT", "maks_jam_tempuh": 0.1})

		assert.Empty(t, plan.Rute)
		assert.Zero(t, plan.TotalVolumeTon)
		assert.NotEmpty(t, plan.BelumTerpenuhi)
		assert.NotEmpty(t, plan.SurplusSisa)
	})

	t.Run("tarif mengubah biaya tetapi tidak volume", func(t *testing.T) {
		plan := run(map[string]any{"komoditas_kode": "CBM-KRT", "tarif_per_ton_km": 8400})

		assert.Equal(t, dasar.TotalVolumeTon, plan.TotalVolumeTon)
		assert.InDelta(t, 2*float64(dasar.TotalBiaya), float64(plan.TotalBiaya), float64(len(plan.Rute)))
	})

	t.Run("penyesuaian pasokan: panen gagal di sentra mengurangi volume tersalur", func(t *testing.T) {
		require.NotEmpty(t, dasar.Rute)
		sentra := dasar.Rute[0].AsalKode
		var dikirim float64
		for _, r := range dasar.Rute {
			if r.AsalKode == sentra {
				dikirim += r.VolumeTon
			}
		}

		plan := run(map[string]any{
			"komoditas_kode": "CBM-KRT",
			"penyesuaian":    []map[string]any{{"kabupaten_kode": sentra, "ton": -dikirim}},
		})

		assert.Less(t, plan.TotalVolumeTon, dasar.TotalVolumeTon)
		for _, r := range plan.Rute {
			assert.NotEqual(t, sentra, r.AsalKode, "surplus sentra sudah habis")
		}
	})

	t.Run("simulasi tidak menulis apa pun", func(t *testing.T) {
		w := s.call(t, "/v1/spasial/rute", emailHanyaLihat)
		assert.Len(t, data[[]map[string]any](t, w), 5, "hanya 5 rute demo")
		assert.Empty(t, data[[]map[string]any](t, s.call(t, pathOptimasi, emailHanyaLihat)))
	})
}

func TestOptimasi_RejectsInvalidRequests(t *testing.T) {
	s, _ := writableSuite(t, "spasial_optimasi_invalid")

	cases := map[string]struct {
		body map[string]any
		want int
	}{
		"tanpa komoditas":               {map[string]any{}, http.StatusBadRequest},
		"periode bukan tanggal":         {map[string]any{"komoditas_kode": "CBM-KRT", "periode": "besok"}, http.StatusBadRequest},
		"faktor jalan di bawah 1":       {map[string]any{"komoditas_kode": "CBM-KRT", "faktor_jalan": 0.2}, http.StatusBadRequest},
		"tarif negatif":                 {map[string]any{"komoditas_kode": "CBM-KRT", "tarif_per_ton_km": -5}, http.StatusBadRequest},
		"batas waktu terlalu besar":     {map[string]any{"komoditas_kode": "CBM-KRT", "maks_jam_tempuh": 999}, http.StatusBadRequest},
		"penyesuaian wilayah asing":     {map[string]any{"komoditas_kode": "CBM-KRT", "penyesuaian": []map[string]any{{"kabupaten_kode": "9999", "ton": 1}}}, http.StatusBadRequest},
		"penyesuaian tanpa kode":        {map[string]any{"komoditas_kode": "CBM-KRT", "penyesuaian": []map[string]any{{"ton": 1}}}, http.StatusBadRequest},
		"skenario tidak boleh disimpan": {map[string]any{"komoditas_kode": "CBM-KRT", "simpan": true, "penyesuaian": []map[string]any{{"kabupaten_kode": "1171", "ton": 1}}}, http.StatusBadRequest},
		"komoditas tanpa neraca":        {map[string]any{"komoditas_kode": "TIDAK-ADA"}, http.StatusNotFound},
		"periode tanpa neraca":          {map[string]any{"komoditas_kode": "CBM-KRT", "periode": "2001-01-01"}, http.StatusNotFound},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			w := s.post(t, pathOptimasi, emailLogistik, tc.body)
			assert.Equal(t, tc.want, w.Code, w.Body.String())
		})
	}
}

func TestOptimasi_SavedPlanBecomesProposedRoutesAndSupersedesTheOldOne(t *testing.T) {
	s, db := writableSuite(t, "spasial_optimasi_simpan")
	save := func(body map[string]any) optimasiResp {
		body["simpan"] = true
		w := s.post(t, pathOptimasi, emailLogistik, body)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		return data[optimasiResp](t, w)
	}

	first := save(map[string]any{"komoditas_kode": "CBM-KRT"})

	require.NotNil(t, first.OptimasiID)
	require.NotEmpty(t, first.Rute)
	t.Run("ID dan kode rencana dari basis data, bukan buatan aplikasi", func(t *testing.T) {
		assert.Regexp(t, `^[1-9][0-9]*$`, *first.OptimasiID, "ID BIGINT sebagai string desimal")
		require.NotNil(t, first.OptimasiKode)
		assert.Equal(t, "OPT-"+strings.Repeat("0", 6-len(*first.OptimasiID))+*first.OptimasiID, *first.OptimasiKode, "kode turunan ID identity")
		var kode string
		require.NoError(t, db.Raw(`SELECT kode FROM optimasi_rute WHERE id = ?`, *first.OptimasiID).Scan(&kode).Error)
		assert.Equal(t, *first.OptimasiKode, kode)
	})

	t.Run("rute usulan tertaut ke rencana dan tampil di daftar rute", func(t *testing.T) {
		var rows []struct {
			ID, Status, Sumber, OptimasiID string
			Kode                           *string
		}
		require.NoError(t, db.Raw(`SELECT id, kode, status, sumber, optimasi_id FROM rute_distribusi WHERE optimasi_id = ?`, *first.OptimasiID).Scan(&rows).Error)
		require.Len(t, rows, len(first.Rute))
		for _, r := range rows {
			assert.Equal(t, "USULAN", r.Status)
			assert.Equal(t, "OPTIMASI", r.Sumber)
			assert.Equal(t, *first.OptimasiID, r.OptimasiID, "FK menunjuk ID hasil RETURNING")
			assert.Nil(t, r.Kode, "rute baru tidak diberi kode buatan aplikasi")
			assert.Regexp(t, `^[1-9][0-9]*$`, r.ID)
		}

		list := data[[]map[string]any](t, s.call(t, "/v1/spasial/rute", emailHanyaLihat))
		assert.Len(t, list, 5+len(first.Rute))
	})

	t.Run("cache jarak terisi dari estimasi", func(t *testing.T) {
		var n int64
		require.NoError(t, db.Raw(`SELECT COUNT(*) FROM jarak_antar_wilayah WHERE sumber = 'ESTIMASI'`).Scan(&n).Error)
		assert.EqualValues(t, len(first.Rute), n)
	})

	t.Run("rencana dan aksi tercatat", func(t *testing.T) {
		var rencana struct {
			Parameter         string
			TotalVolumeTon    float64
			TotalBiaya        int64
			DibuatOleh        string
			BelumTerpenuhiTon float64
			BiayaPembanding   *int64
		}
		require.NoError(t, db.Raw(`SELECT parameter::text, total_volume_ton, total_biaya, dibuat_oleh, belum_terpenuhi_ton, biaya_pembanding FROM optimasi_rute WHERE id = ?`, *first.OptimasiID).Scan(&rencana).Error)
		require.NotNil(t, rencana.BiayaPembanding, "semua wilayah terhubung: rencana serakah menyalurkan volume yang sama")
		assert.GreaterOrEqual(t, *rencana.BiayaPembanding+int64(len(first.Rute)), rencana.TotalBiaya, "optimum tidak lebih mahal dari serakah")
		require.NotNil(t, first.PenghematanPersen)
		assert.GreaterOrEqual(t, *first.PenghematanPersen, 0.0)
		assert.Equal(t, rencana.BiayaPembanding, first.BiayaPembanding)
		assert.JSONEq(t, `{"tarif_ton_km":4200,"faktor_jalan":1.35,"kecepatan_km":45,"maks_jam_tempuh":0}`, rencana.Parameter)
		assert.Equal(t, first.TotalVolumeTon, rencana.TotalVolumeTon)
		assert.Equal(t, first.TotalBiaya, rencana.TotalBiaya)
		assert.Equal(t, uid("manajer_program"), rencana.DibuatOleh)

		var aksi int64
		require.NoError(t, db.Raw(`SELECT COUNT(*) FROM audit_log WHERE aksi = 'OPTIMASI_RUTE' AND entitas_id = ? AND aktor_id = garda_uuid_stabil('pengguna', 'manajer_program')`, *first.OptimasiID).Scan(&aksi).Error)
		assert.EqualValues(t, 1, aksi, "audit mencatat ID BIGINT rencana")
	})

	second := save(map[string]any{"komoditas_kode": "CBM-KRT", "tarif_per_ton_km": 5000})

	t.Run("menyimpan ulang untuk komoditas dan periode yang sama menggantikan rencana lama", func(t *testing.T) {
		require.NotEqual(t, *first.OptimasiID, *second.OptimasiID)
		var lama []string
		require.NoError(t, db.Raw(`SELECT DISTINCT status FROM rute_distribusi WHERE optimasi_id = ?`, *first.OptimasiID).Scan(&lama).Error)
		assert.Equal(t, []string{"DIGANTIKAN"}, lama)

		list := data[[]map[string]any](t, s.call(t, "/v1/spasial/rute", emailHanyaLihat))
		assert.Len(t, list, 5+len(second.Rute), "rute yang digantikan tidak tampil lagi")
	})

	t.Run("riwayat memuat kedua rencana, terbaru dulu", func(t *testing.T) {
		type rencana struct {
			ID            string  `json:"id"`
			Kode          *string `json:"kode"`
			KomoditasNama string  `json:"komoditas_nama"`
			TotalBiaya    int64   `json:"total_biaya"`
			DibuatOleh    *string `json:"dibuat_oleh"`
		}
		list := data[[]rencana](t, s.call(t, pathOptimasi, emailHanyaLihat))

		require.Len(t, list, 2)
		assert.Equal(t, *second.OptimasiID, list[0].ID)
		assert.Equal(t, *first.OptimasiID, list[1].ID)
		require.NotNil(t, list[0].Kode)
		assert.Equal(t, *second.OptimasiKode, *list[0].Kode)
		assert.NotEmpty(t, list[0].KomoditasNama)
		require.NotNil(t, list[0].DibuatOleh)
	})

	t.Run("komoditas lain tidak menggantikan", func(t *testing.T) {
		save(map[string]any{"komoditas_kode": "BWM"})

		var n int64
		require.NoError(t, db.Raw(`SELECT COUNT(*) FROM rute_distribusi WHERE optimasi_id = ? AND status = 'USULAN'`, *second.OptimasiID).Scan(&n).Error)
		assert.EqualValues(t, len(second.Rute), n)
	})
}

// Aturan ON DELETE 000113: menghapus rencana menghapus rutenya (CASCADE); kabupaten yang dirujuk rute ditahan
// (RESTRICT); matriks jarak dan geometri ikut terhapus bersama kabupatennya (CASCADE).
func TestOnDeleteRules_RoutesFollowPlanAndRegionsAreProtected(t *testing.T) {
	s, db := writableSuite(t, "spasial_on_delete")
	w := s.post(t, pathOptimasi, emailLogistik, map[string]any{"komoditas_kode": "CBM-KRT", "simpan": true})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	plan := data[optimasiResp](t, w)
	require.NotNil(t, plan.OptimasiID)
	require.NotEmpty(t, plan.Rute)

	// Wilayah yang dirujuk rute demo (1106 asal, 1171 tujuan) tidak dapat dihapus.
	err := db.Exec(`DELETE FROM kabupaten WHERE kode = '1171'`).Error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "23503")

	// Komoditas yang dipakai rencana dan neraca tidak dapat dihapus.
	err = db.Exec(`DELETE FROM komoditas WHERE kode = 'CBM-KRT'`).Error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "23503")

	var sebelum int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM rute_distribusi`).Scan(&sebelum).Error)
	require.NoError(t, db.Exec(`DELETE FROM optimasi_rute WHERE id = ?`, *plan.OptimasiID).Error)
	var sesudah, sisa int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM rute_distribusi`).Scan(&sesudah).Error)
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM rute_distribusi WHERE optimasi_id = ?`, *plan.OptimasiID).Scan(&sisa).Error)
	assert.Zero(t, sisa, "rute milik rencana ikut terhapus")
	assert.Equal(t, sebelum-int64(len(plan.Rute)), sesudah, "rute demo tanpa rencana tidak tersentuh")

	// Matriks jarak dan geometri adalah turunan wilayah: dihapus bersama kabupaten yang tidak lagi dirujuk tabel lain.
	require.NoError(t, db.Exec(`DELETE FROM jarak_antar_wilayah`).Error)
	require.NoError(t, db.Exec(`INSERT INTO kabupaten (kode, nama, jenis, ibukota, lat, lng, provinsi_kode)
		SELECT '9999', 'KABUPATEN UJI', 'KABUPATEN', 'Uji', 1, 1, provinsi_kode FROM kabupaten LIMIT 1`).Error)
	require.NoError(t, db.Exec(`INSERT INTO jarak_antar_wilayah (asal_kode, tujuan_kode, jarak_km, jam_tempuh) VALUES ('9999', '1171', 10, 1), ('1171', '9999', 10, 1)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO wilayah_geometri (kabupaten_kode, geojson, min_lat, min_lng, max_lat, max_lng)
		VALUES ('9999', '{"type":"Polygon","coordinates":[[[1,1],[2,1],[2,2],[1,1]]]}', 1, 1, 2, 2)`).Error)
	require.NoError(t, db.Exec(`DELETE FROM kabupaten WHERE kode = '9999'`).Error)
	var n int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM jarak_antar_wilayah`).Scan(&n).Error)
	assert.Zero(t, n)
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM wilayah_geometri WHERE kabupaten_kode = '9999'`).Scan(&n).Error)
	assert.Zero(t, n)
}

func TestMigration000102_RejectsInvalidDistancesAndDownKeepsRoutes(t *testing.T) {
	_, db := writableSuite(t, "spasial_optimasi_migrasi")

	assert.ErrorContains(t, db.Exec(`INSERT INTO jarak_antar_wilayah (asal_kode, tujuan_kode, jarak_km, jam_tempuh) VALUES ('1171', '1171', 1, 1)`).Error, "chk_jarak_antar_wilayah_beda")
	assert.ErrorContains(t, db.Exec(`INSERT INTO jarak_antar_wilayah (asal_kode, tujuan_kode, jarak_km, jam_tempuh) VALUES ('1171', '1172', -1, 1)`).Error, "chk_jarak_antar_wilayah_nilai")
	assert.ErrorContains(t, db.Exec(`INSERT INTO jarak_antar_wilayah (asal_kode, tujuan_kode, jarak_km, jam_tempuh, sumber) VALUES ('1171', '1172', 1, 1, 'GOOGLE')`).Error, "chk_jarak_antar_wilayah_sumber")
	assert.NoError(t, db.Exec(`INSERT INTO jarak_antar_wilayah (asal_kode, tujuan_kode, jarak_km, jam_tempuh, sumber) VALUES ('1171', '1172', 25.5, 0.6, 'OSRM')`).Error)

	migrator := pgtest.Migrator(t, db)
	require.NoError(t, migrator.DownTo(t.Context(), 101))

	var n int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM rute_distribusi`).Scan(&n).Error)
	assert.EqualValues(t, 5, n, "rute demo tidak tersentuh oleh Down")
	var tabel []string
	require.NoError(t, db.Raw(`SELECT table_name FROM information_schema.tables WHERE table_name IN ('optimasi_rute', 'jarak_antar_wilayah')`).Scan(&tabel).Error)
	assert.Empty(t, tabel)

	require.NoError(t, migrator.UpTo(t.Context(), 102))
}
