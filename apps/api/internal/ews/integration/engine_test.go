package integration

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	ewsseeder "github.com/garda-inflasi/api/internal/ews/seeder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MANAJER_DATA: ews.lihat + ews.atur_ambang.
const emailAtur = "manajer.data@gardainflasi.id"

type evaluasiRow struct {
	Aturan    int `json:"aturan"`
	Diukur    int `json:"diukur"`
	Dibuat    int `json:"dibuat"`
	Dinaikkan int `json:"dinaikkan"`
	Dilewati  int `json:"dilewati"`
	Diredam   int `json:"diredam"`
	Galat     int `json:"galat"`
}

func (s *suite) evaluasi(t *testing.T) evaluasiRow {
	t.Helper()
	w := s.call(t, http.MethodPost, "/v1/ews/evaluasi", emailAtur, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	return data[evaluasiRow](t, w)
}

type alertMesin struct {
	ID       string  `gorm:"column:id"`
	Severity string  `gorm:"column:severity"`
	Status   string  `gorm:"column:status"`
	Nilai    float64 `gorm:"column:nilai"`
	AturanID string  `gorm:"column:aturan_id"`
	DedupKey string  `gorm:"column:dedup_key"`
	Skor     int     `gorm:"column:skor_dampak"`
	Pesan    string  `gorm:"column:pesan"`
}

func (s *suite) alertMesin(t *testing.T, dedupKey string) []alertMesin {
	t.Helper()
	var rows []alertMesin
	require.NoError(t, s.db.Raw(`SELECT id, severity, status, nilai, aturan_id, dedup_key, skor_dampak, pesan
		FROM ews_alert WHERE dedup_key = ? ORDER BY dibuat_pada, id`, dedupKey).Scan(&rows).Error)
	return rows
}

func (s *suite) entriHarga(t *testing.T, id, komoditas, pasar string, harga int, status string, dicatat time.Time) {
	t.Helper()
	require.NoError(t, s.db.Exec(`INSERT INTO entri_harga (id, komoditas_kode, pasar_id, tipe, harga, satuan, status, dicatat_pada)
		VALUES (?, ?, ?, 'KONSUMEN', ?, 'kg', CAST(? AS status_entri), ?)`, id, komoditas, pasar, harga, status, dicatat).Error)
}

func TestEngine_PriceSpikeCreatesOneIdempotentAlert(t *testing.T) {
	s := newSuite(t, "ews_engine_harga")
	now := time.Now()

	// Dasar 3 hari lalu Rp50.000; kini Rp60.000 (+20%) di Banda Aceh.
	s.entriHarga(t, "H1", "CBM-KRT", "PSR-1171-01", 50000, "DIVALIDASI", now.Add(-72*time.Hour))
	s.entriHarga(t, "H2", "CBM-KRT", "PSR-1171-02", 50000, "DIPUBLIKASI", now.Add(-70*time.Hour))
	s.entriHarga(t, "H3", "CBM-KRT", "PSR-1171-01", 60000, "DIVALIDASI", now.Add(-2*time.Hour))
	// Entri belum tervalidasi tidak dihitung, betapapun ekstrem.
	s.entriHarga(t, "H4", "CBM-KRT", "PSR-1106-01", 40000, "DIVALIDASI", now.Add(-72*time.Hour))
	s.entriHarga(t, "H5", "CBM-KRT", "PSR-1106-01", 90000, "DIAJUKAN", now.Add(-2*time.Hour))

	first := s.evaluasi(t)
	assert.Equal(t, 9, first.Aturan)
	assert.Equal(t, 1, first.Dibuat)
	assert.Zero(t, first.Galat)

	alerts := s.alertMesin(t, "LONJAKAN_HARGA|CBM-KRT|1171")
	require.Len(t, alerts, 1)
	a := alerts[0]
	assert.Equal(t, "KRITIS", a.Severity)
	assert.Equal(t, "BARU", a.Status)
	assert.Equal(t, "ATR-LH-CBM-KRT", a.AturanID)
	assert.InDelta(t, 20, a.Nilai, 0.001)
	assert.Equal(t, 80+11, a.Skor)
	assert.Contains(t, a.Pesan, "naik 20,0%")
	assert.Contains(t, a.Pesan, "Kota Banda Aceh")
	assert.Empty(t, s.alertMesin(t, "LONJAKAN_HARGA|CBM-KRT|1106"), "entri DIAJUKAN tidak boleh memicu alert")

	var riwayat []struct {
		Dari        *string
		Ke, Catatan string
	}
	require.NoError(t, s.db.Raw(`SELECT dari, ke, catatan FROM ews_riwayat WHERE alert_id = ?`, a.ID).Scan(&riwayat).Error)
	require.Len(t, riwayat, 1)
	assert.Nil(t, riwayat[0].Dari)
	assert.Equal(t, "BARU", riwayat[0].Ke)

	second := s.evaluasi(t)
	assert.Zero(t, second.Dibuat, "kondisi yang sama tidak membuat alert ganda")
	assert.Equal(t, 1, second.Dilewati)
	assert.Len(t, s.alertMesin(t, "LONJAKAN_HARGA|CBM-KRT|1171"), 1)

	var audit []string
	require.NoError(t, s.db.Raw(`SELECT aksi FROM audit_log WHERE entitas = 'ews_alert' ORDER BY waktu`).Scan(&audit).Error)
	assert.Equal(t, []string{"BUAT_OTOMATIS", "EVALUASI", "EVALUASI"}, audit)
}

func TestEngine_RaisesSeverityOnWorseningConditionAndRecordsHistory(t *testing.T) {
	s := newSuite(t, "ews_engine_naik")
	now := time.Now()
	s.entriHarga(t, "H1", "BWM", "PSR-1107-01", 50000, "DIVALIDASI", now.Add(-72*time.Hour))
	s.entriHarga(t, "H2", "BWM", "PSR-1107-01", 54500, "DIVALIDASI", now.Add(-2*time.Hour)) // +9%

	require.Equal(t, 1, s.evaluasi(t).Dibuat)
	before := s.alertMesin(t, "LONJAKAN_HARGA|BWM|1107")
	require.Len(t, before, 1)
	assert.Equal(t, "WASPADA", before[0].Severity)

	require.NoError(t, s.db.Exec(`UPDATE entri_harga SET harga = 56500 WHERE id = 'H2'`).Error) // +13%
	res := s.evaluasi(t)
	assert.Equal(t, 1, res.Dinaikkan)
	assert.Zero(t, res.Dibuat)

	after := s.alertMesin(t, "LONJAKAN_HARGA|BWM|1107")
	require.Len(t, after, 1, "tetap satu alert")
	assert.Equal(t, before[0].ID, after[0].ID)
	assert.Equal(t, "SIAGA", after[0].Severity)

	// Membaik kembali tidak menurunkan tingkat alert yang masih aktif.
	require.NoError(t, s.db.Exec(`UPDATE entri_harga SET harga = 54500 WHERE id = 'H2'`).Error)
	assert.Equal(t, 1, s.evaluasi(t).Dilewati)
	assert.Equal(t, "SIAGA", s.alertMesin(t, "LONJAKAN_HARGA|BWM|1107")[0].Severity)

	w := s.call(t, http.MethodGet, "/v1/ews/"+after[0].ID+"/riwayat", emailLihat, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	riwayat := data[[]struct {
		Dari     *string `json:"dari"`
		Ke       string  `json:"ke"`
		OlehNama *string `json:"oleh_nama"`
		Catatan  string  `json:"catatan"`
	}](t, w)
	require.Len(t, riwayat, 2)
	assert.Equal(t, "Dibuat oleh rule engine", riwayat[0].Catatan)
	assert.Equal(t, "Tingkat naik dari WASPADA ke SIAGA", riwayat[1].Catatan)
	assert.Nil(t, riwayat[1].OlehNama, "perubahan oleh mesin tidak punya pelaku pengguna")
}

func TestEngine_CooldownSuppressesRecentlyClosedCondition(t *testing.T) {
	s := newSuite(t, "ews_engine_cooldown")
	now := time.Now()
	s.entriHarga(t, "H1", "BWM", "PSR-1107-01", 50000, "DIVALIDASI", now.Add(-72*time.Hour))
	s.entriHarga(t, "H2", "BWM", "PSR-1107-01", 60000, "DIVALIDASI", now.Add(-2*time.Hour))
	require.Equal(t, 1, s.evaluasi(t).Dibuat)
	id := s.alertMesin(t, "LONJAKAN_HARGA|BWM|1107")[0].ID

	w := s.call(t, http.MethodPost, "/v1/ews/bulk", emailPenuh, map[string]any{"action": "selesai", "ids": []string{id}})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	res := s.evaluasi(t)
	assert.Equal(t, 1, res.Diredam, "baru ditutup: masih dalam masa peredaman")
	assert.Zero(t, res.Dibuat)

	require.NoError(t, s.db.Exec(`UPDATE ews_alert SET selesai_pada = ? WHERE id = ?`, now.Add(-25*time.Hour), id).Error)
	res = s.evaluasi(t)
	assert.Equal(t, 1, res.Dibuat, "peredaman 24 jam lewat: kondisi yang masih berlangsung memicu alert baru")
	alerts := s.alertMesin(t, "LONJAKAN_HARGA|BWM|1107")
	require.Len(t, alerts, 2)
	assert.Equal(t, "SELESAI", alerts[0].Status)
	assert.Equal(t, "BARU", alerts[1].Status)
}

func TestEngine_StockDeclineAndRainfallRules(t *testing.T) {
	s := newSuite(t, "ews_engine_stok_cuaca")
	now := time.Now()

	require.NoError(t, s.db.Exec(`INSERT INTO cold_storage (id, nama, kabupaten_kode, lat, lng, kapasitas_ton, suhu_target_c, status)
		VALUES ('CS-T1', 'CS Uji', '1108', 5.5, 95.3, 100, 4, 'OPERASIONAL')`).Error)
	lot := func(id string, ton float64, masuk, kedaluwarsa time.Time) {
		require.NoError(t, s.db.Exec(`INSERT INTO stok_cold_storage (id, cold_storage_id, komoditas_kode, volume_ton, masuk_pada, pemilik, risiko_food_loss, kedaluwarsa_pada)
			VALUES (?, 'CS-T1', 'CBM-KRT', ?, ?, 'Koperasi', 0.1, ?)`, id, ton, masuk, kedaluwarsa).Error)
	}
	lot("L1", 10, now.Add(-10*24*time.Hour), now.Add(-24*time.Hour)) // ada di awal jendela, sudah kedaluwarsa
	lot("L2", 4, now.Add(-24*time.Hour), now.Add(10*24*time.Hour))   // stok baru yang layak
	// Stok 10 ton -> 4 ton dalam 5 hari = turun 60% (>= kritis 55).

	for d := range 5 {
		require.NoError(t, s.db.Exec(`INSERT INTO cuaca_harian (id, kabupaten_kode, tanggal, curah_hujan_mm, suhu_c, kelembapan, anomali, sumber)
			VALUES (?, '1106', ?, 30, 22, 90, false, 'BMKG')`, fmt.Sprintf("C%d", d), now.AddDate(0, 0, -d).Format(time.DateOnly)).Error)
	}
	// 5 x 30 mm = 150 mm dalam 10 hari: antara siaga 140 dan kritis 200.

	// Hujan 150 mm melampaui ambang cuaca tiga komoditas pilot (beras medium, cabai, bawang), stok hanya cabai.
	res := s.evaluasi(t)
	assert.Equal(t, 4, res.Dibuat)

	stok := s.alertMesin(t, "KELANGKAAN_STOK|CBM-KRT|1108")
	require.Len(t, stok, 1)
	assert.Equal(t, "KRITIS", stok[0].Severity)
	assert.InDelta(t, -60, stok[0].Nilai, 0.001, "penurunan stok bernilai negatif")

	cuaca := s.alertMesin(t, "ANOMALI_CUACA|CBM-KRT|1106")
	require.Len(t, cuaca, 1)
	assert.Equal(t, "SIAGA", cuaca[0].Severity)
	assert.InDelta(t, 150, cuaca[0].Nilai, 0.001)

	assert.Zero(t, s.evaluasi(t).Dibuat, "idempoten")
}

func TestEngine_InactiveRuleIsIgnoredAndThresholdsAreHonoured(t *testing.T) {
	s := newSuite(t, "ews_engine_aturan_nonaktif")
	now := time.Now()
	s.entriHarga(t, "H1", "CBM-KRT", "PSR-1171-01", 50000, "DIVALIDASI", now.Add(-72*time.Hour))
	s.entriHarga(t, "H2", "CBM-KRT", "PSR-1171-01", 60000, "DIVALIDASI", now.Add(-2*time.Hour))
	require.NoError(t, s.db.Exec(`UPDATE aturan_ews SET aktif = false WHERE id = 'ATR-LH-CBM-KRT'`).Error)

	res := s.evaluasi(t)
	assert.Equal(t, 8, res.Aturan)
	assert.Zero(t, res.Dibuat)

	require.NoError(t, s.db.Exec(`UPDATE aturan_ews SET aktif = true, ambang_waspada = 30, ambang_siaga = 40, ambang_kritis = 50 WHERE id = 'ATR-LH-CBM-KRT'`).Error)
	assert.Zero(t, s.evaluasi(t).Dibuat, "+20% di bawah ambang waspada 30%")

	require.NoError(t, s.db.Exec(`UPDATE aturan_ews SET ambang_waspada = 15, ambang_siaga = 30, ambang_kritis = 50 WHERE id = 'ATR-LH-CBM-KRT'`).Error)
	assert.Equal(t, 1, s.evaluasi(t).Dibuat)
	assert.Equal(t, "WASPADA", s.alertMesin(t, "LONJAKAN_HARGA|CBM-KRT|1171")[0].Severity)
}

func TestEngine_EvaluasiEndpointAuthorization(t *testing.T) {
	s := sharedSuite(t)
	assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodPost, "/v1/ews/evaluasi", "", nil).Code)
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPost, "/v1/ews/evaluasi", emailPenuh, nil).Code, "ews.eskalasi saja tidak cukup")
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPost, "/v1/ews/evaluasi", emailLihat, nil).Code)
	assert.Equal(t, http.StatusOK, s.call(t, http.MethodPost, "/v1/ews/evaluasi", emailAtur, nil).Code)
}

type aturanRow struct {
	ID            string   `json:"id"`
	AmbangWaspada float64  `json:"ambang_waspada"`
	AmbangSiaga   float64  `json:"ambang_siaga"`
	AmbangKritis  float64  `json:"ambang_kritis"`
	JendelaHari   int      `json:"jendela_hari"`
	CooldownJam   int      `json:"cooldown_jam"`
	Kanal         []string `json:"kanal"`
	Aktif         bool     `json:"aktif"`
	Keterangan    string   `json:"keterangan"`
	Versi         int      `json:"versi"`
	DiubahOlehID  *string  `json:"diubah_oleh_id"`
}

func bodyAturan(versi int, waspada, siaga, kritis float64) map[string]any {
	return map[string]any{
		"versi": versi, "ambang_waspada": waspada, "ambang_siaga": siaga, "ambang_kritis": kritis,
		"jendela_hari": 7, "cooldown_jam": 12, "kanal": []string{"DASHBOARD", "EMAIL"}, "aktif": true, "keterangan": "Dikalibrasi ulang",
	}
}

func TestAturan_UpdateThresholds(t *testing.T) {
	s := newSuite(t, "ews_aturan_update")
	const path = "/v1/ews/aturan/ATR-LH-BWM"

	t.Run("izin ews.atur_ambang dibutuhkan", func(t *testing.T) {
		assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodPut, path, "", bodyAturan(1, 5, 10, 20)).Code)
		assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPut, path, emailPenuh, bodyAturan(1, 5, 10, 20)).Code)
		assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPut, path, emailLihat, bodyAturan(1, 5, 10, 20)).Code)
	})

	t.Run("ambang harus menaik dan positif", func(t *testing.T) {
		for name, body := range map[string]map[string]any{
			"sama":    bodyAturan(1, 10, 10, 20),
			"turun":   bodyAturan(1, 20, 10, 5),
			"nol":     bodyAturan(1, 0, 10, 20),
			"tanpa v": {"ambang_waspada": 5, "ambang_siaga": 10, "ambang_kritis": 20},
		} {
			assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodPut, path, emailAtur, body).Code, name)
		}
		bad := bodyAturan(1, 5, 10, 20)
		bad["kanal"] = []string{"TELEGRAM"}
		assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodPut, path, emailAtur, bad).Code, "kanal di luar daftar")
		assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodPut, "/v1/ews/aturan/TIDAK-ADA", emailAtur, bodyAturan(1, 5, 10, 20)).Code)
	})

	t.Run("simpan menaikkan versi, mencatat riwayat dan audit", func(t *testing.T) {
		w := s.call(t, http.MethodPut, path, emailAtur, bodyAturan(1, 5, 10, 20))
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		got := data[aturanRow](t, w)
		assert.Equal(t, 2, got.Versi)
		assert.EqualValues(t, 10, got.AmbangSiaga)
		assert.Equal(t, 12, got.CooldownJam)
		assert.Equal(t, []string{"DASHBOARD", "EMAIL"}, got.Kanal)
		require.NotNil(t, got.DiubahOlehID)
		assert.Equal(t, uid("manajer_data"), *got.DiubahOlehID)

		var snapshot []struct {
			Versi        int
			AmbangKritis float64
		}
		require.NoError(t, s.db.Raw(`SELECT versi, ambang_kritis FROM aturan_ews_riwayat WHERE aturan_id = 'ATR-LH-BWM'`).Scan(&snapshot).Error)
		require.Len(t, snapshot, 1)
		assert.Equal(t, 2, snapshot[0].Versi)
		assert.EqualValues(t, 20, snapshot[0].AmbangKritis)

		var audit []struct{ Aksi, Sebelum, Sesudah string }
		require.NoError(t, s.db.Raw(`SELECT aksi, sebelum::text AS sebelum, sesudah::text AS sesudah FROM audit_log WHERE entitas = 'aturan_ews'`).Scan(&audit).Error)
		require.Len(t, audit, 1)
		assert.Equal(t, "UBAH_AMBANG", audit[0].Aksi)
		assert.Contains(t, audit[0].Sebelum, `"ambang_kritis": 18`)
		assert.Contains(t, audit[0].Sesudah, `"ambang_kritis": 20`)
	})

	t.Run("versi usang ditolak dengan 409 dan tidak mengubah data", func(t *testing.T) {
		w := s.call(t, http.MethodPut, path, emailAtur, bodyAturan(1, 6, 11, 21))
		assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())

		list := data[[]aturanRow](t, s.call(t, http.MethodGet, "/v1/ews/aturan", emailLihat, nil))
		for _, a := range list {
			if a.ID == "ATR-LH-BWM" {
				assert.EqualValues(t, 20, a.AmbangKritis)
				assert.Equal(t, 2, a.Versi)
			}
		}
	})

	t.Run("seed reference tidak menimpa aturan yang sudah diubah", func(t *testing.T) {
		_, err := ewsseeder.SeedReference(t.Context(), s.db)
		require.NoError(t, err)
		list := data[[]aturanRow](t, s.call(t, http.MethodGet, "/v1/ews/aturan", emailLihat, nil))
		for _, a := range list {
			switch a.ID {
			case "ATR-LH-BWM":
				assert.EqualValues(t, 20, a.AmbangKritis, "hasil kalibrasi dipertahankan")
				assert.Equal(t, "Dikalibrasi ulang", a.Keterangan)
			case "ATR-LH-CBM-KRT":
				assert.Equal(t, 1, a.Versi)
			}
		}
	})
}

func TestBulk_RecordsHistoryAndTimestamps(t *testing.T) {
	s := newSuite(t, "ews_bulk_riwayat")

	for _, step := range []struct{ email, action string }{{emailAkui, "akui"}, {emailPenuh, "eskalasi"}, {emailPenuh, "selesai"}} {
		w := s.call(t, http.MethodPost, "/v1/ews/bulk", step.email, map[string]any{"action": step.action, "ids": []string{"EWS-1000"}})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	}

	w := s.call(t, http.MethodGet, "/v1/ews/EWS-1000/riwayat", emailLihat, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	rows := data[[]struct {
		Dari     *string `json:"dari"`
		Ke       string  `json:"ke"`
		OlehID   *string `json:"oleh_id"`
		OlehNama *string `json:"oleh_nama"`
	}](t, w)
	require.Len(t, rows, 4, "dibuat (seed), akui, eskalasi, selesai")
	var path []string
	for _, r := range rows {
		path = append(path, r.Ke)
	}
	assert.Equal(t, []string{"BARU", "DIAKUI", "DIESKALASI", "SELESAI"}, path)
	require.NotNil(t, rows[2].OlehNama)
	assert.Equal(t, "Dian Permata, S.Si., M.M.", *rows[2].OlehNama)

	var stamp struct {
		DieskalasiPada, SelesaiPada *time.Time
		DieskalasiOlehID            *string
	}
	require.NoError(t, s.db.Raw(`SELECT dieskalasi_pada, selesai_pada, dieskalasi_oleh_id FROM ews_alert WHERE id = 'EWS-1000'`).Scan(&stamp).Error)
	assert.NotNil(t, stamp.DieskalasiPada)
	assert.NotNil(t, stamp.SelesaiPada)
	require.NotNil(t, stamp.DieskalasiOlehID)
	assert.Equal(t, uid("manajer_risiko"), *stamp.DieskalasiOlehID)

	assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodGet, "/v1/ews/TIDAK-ADA/riwayat", emailLihat, nil).Code)
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, "/v1/ews/EWS-1000/riwayat", emailTanpaIzin, nil).Code)
}
