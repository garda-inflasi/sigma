package integration

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type bulkModelResp struct {
	Berhasil int `json:"berhasil"`
	Gagal    int `json:"gagal"`
	Detail   []struct {
		ID     string `json:"id"`
		Alasan string `json:"alasan"`
	} `json:"detail"`
}

func (s *suite) bulkModel(t *testing.T, email, action string, ids ...string) (*bulkModelResp, int) {
	t.Helper()
	w := s.post(t, "/v1/prediksi/registri/bulk", email, map[string]any{"action": action, "ids": ids})
	if w.Code != http.StatusOK {
		return nil, w.Code
	}
	r := decode[bulkModelResp](t, w)
	return &r, w.Code
}

func (s *suite) statusModel(t *testing.T, id string) string {
	t.Helper()
	var status string
	require.NoError(t, s.db.Raw(`SELECT status::text FROM model_registry WHERE id = ?`, id).Scan(&status).Error)
	return status
}

func TestPromosi_Authorization(t *testing.T) {
	s := newMLSuite(t, "prediksi_promosi_authz", newFakeML(t).client())
	bulk := map[string]any{"action": "promosi", "ids": []string{"MDL-CBM-KRT-v2.4.0-rc1"}}

	assert.Equal(t, http.StatusUnauthorized, s.post(t, "/v1/prediksi/registri/bulk", "", bulk).Code)
	assert.Equal(t, http.StatusForbidden, s.post(t, "/v1/prediksi/registri/bulk", emailBerhak, bulk).Code, "prediksi.lihat saja tidak boleh mempromosikan")
	assert.Equal(t, http.StatusForbidden, s.post(t, "/v1/prediksi/registri/bulk", emailTanpaIzin, bulk).Code)
	assert.Equal(t, "KANDIDAT", s.statusModel(t, "MDL-CBM-KRT-v2.4.0-rc1"), "penolakan tidak mengubah status")
	assert.Equal(t, 0, s.hitung(t, `SELECT COUNT(*) FROM model_promosi`))

	sim := map[string]any{"komoditas_kode": "CBM-KRT"}
	assert.Equal(t, http.StatusUnauthorized, s.post(t, "/v1/prediksi/simulasi", "", sim).Code)
	assert.Equal(t, http.StatusForbidden, s.post(t, "/v1/prediksi/simulasi", emailTanpaIzin, sim).Code)
	assert.Equal(t, 0, s.hitung(t, `SELECT COUNT(*) FROM skenario_simulasi`))
}

func TestPromosi_AlurLatihPromosiDanRollback(t *testing.T) {
	s := newMLSuite(t, "prediksi_promosi_alur", newFakeML(t).client())
	m := decode[modelResp](t, s.post(t, "/v1/prediksi/latih", emailKelola, map[string]string{"komoditas_kode": "CBM-KRT"}))
	const demoProduksi = "MDL-CBM-KRT-v2.3.1"

	hasil, kode := s.bulkModel(t, emailKelola, "promosi", m.ID)
	require.Equal(t, http.StatusOK, kode)
	assert.Equal(t, 1, hasil.Berhasil)
	assert.Zero(t, hasil.Gagal)

	t.Run("model baru produksi, model lama diarsipkan, tetap satu produksi per komoditas", func(t *testing.T) {
		assert.Equal(t, "PRODUKSI", s.statusModel(t, m.ID))
		assert.Equal(t, "ARSIP", s.statusModel(t, demoProduksi))
		assert.Equal(t, 1, s.hitung(t, `SELECT COUNT(*) FROM model_registry WHERE komoditas_kode = 'CBM-KRT' AND status = 'PRODUKSI'`))
		assert.Equal(t, 1, s.hitung(t, `SELECT COUNT(*) FROM model_registry WHERE id = ? AND dipromosikan_oleh = 'manajer_data' AND dipromosikan_pada IS NOT NULL AND deployed_at IS NOT NULL`, m.ID))
	})

	t.Run("riwayat dan audit tercatat", func(t *testing.T) {
		assert.Equal(t, 1, s.hitung(t, `SELECT COUNT(*) FROM model_promosi WHERE model_id = ? AND aksi = 'PROMOSI' AND status_lama = 'KANDIDAT' AND status_baru = 'PRODUKSI' AND model_digantikan_id = ? AND oleh = 'manajer_data'`, m.ID, demoProduksi))
		assert.Equal(t, 1, s.hitung(t, `SELECT COUNT(*) FROM audit_log WHERE aksi = 'PROMOSI_MODEL' AND entitas_id = ? AND aktor_id = 'manajer_data'`, m.ID))
	})

	t.Run("halaman prediksi memakai proyeksi model terlatih", func(t *testing.T) {
		got := decode[detailResp](t, s.call(t, "/v1/prediksi/CBM-KRT", emailBerhak))
		assert.Equal(t, "MODEL", got.Sumber)
		assert.Equal(t, m.Versi, got.Model.Versi)
	})

	t.Run("mempromosikan ulang model produksi ditolak", func(t *testing.T) {
		hasil, _ := s.bulkModel(t, emailKelola, "promosi", m.ID)
		assert.Zero(t, hasil.Berhasil)
		require.Len(t, hasil.Detail, 1)
		assert.Contains(t, hasil.Detail[0].Alasan, "status model tidak memungkinkan")
	})

	t.Run("rollback: model lama dikembalikan ke produksi", func(t *testing.T) {
		hasil, _ := s.bulkModel(t, emailKelola, "promosi", demoProduksi)
		assert.Equal(t, 1, hasil.Berhasil)
		assert.Equal(t, "PRODUKSI", s.statusModel(t, demoProduksi))
		assert.Equal(t, "ARSIP", s.statusModel(t, m.ID))
		assert.Equal(t, 2, s.hitung(t, `SELECT COUNT(*) FROM model_promosi WHERE komoditas_kode = 'CBM-KRT'`))
		got := decode[detailResp](t, s.call(t, "/v1/prediksi/CBM-KRT", emailBerhak))
		assert.Equal(t, "DEMO", got.Sumber)
	})
}

func TestPromosi_DitolakBilaLebihBurukDariBaseline(t *testing.T) {
	ml := newFakeML(t)
	ml.mape, ml.baseline = 4.0, 3.0
	s := newMLSuite(t, "prediksi_promosi_baseline", ml.client())
	m := decode[modelResp](t, s.post(t, "/v1/prediksi/latih", emailKelola, map[string]string{"komoditas_kode": "CBM-KRT"}))
	require.Equal(t, "KANDIDAT", m.Status, "pelatihan tetap menyimpan kandidat, apa pun metriknya")

	hasil, kode := s.bulkModel(t, emailKelola, "promosi", m.ID)

	require.Equal(t, http.StatusOK, kode)
	assert.Zero(t, hasil.Berhasil)
	require.Len(t, hasil.Detail, 1)
	assert.Contains(t, hasil.Detail[0].Alasan, "lebih buruk dari baseline")
	assert.Equal(t, "KANDIDAT", s.statusModel(t, m.ID))
	assert.Equal(t, "PRODUKSI", s.statusModel(t, "MDL-CBM-KRT-v2.3.1"))
	assert.Equal(t, 0, s.hitung(t, `SELECT COUNT(*) FROM model_promosi`))
}

func TestPromosi_ModelTanpaProyeksiDitolak(t *testing.T) {
	s := newMLSuite(t, "prediksi_promosi_noproj", newFakeML(t).client())

	// Kandidat demo v2.4.0-rc1 tidak punya proyeksi: promosinya membuat komoditas kehilangan proyeksi.
	hasil, _ := s.bulkModel(t, emailKelola, "promosi", "MDL-CBM-KRT-v2.4.0-rc1")

	assert.Zero(t, hasil.Berhasil)
	require.Len(t, hasil.Detail, 1)
	assert.Contains(t, hasil.Detail[0].Alasan, "belum punya proyeksi")
	assert.Equal(t, "PRODUKSI", s.statusModel(t, "MDL-CBM-KRT-v2.3.1"))
	assert.Equal(t, http.StatusOK, s.call(t, "/v1/prediksi/CBM-KRT", emailBerhak).Code)
}

func TestPromosi_ArsipHanyaUntukKandidat(t *testing.T) {
	s := newMLSuite(t, "prediksi_promosi_arsip", newFakeML(t).client())

	hasil, _ := s.bulkModel(t, emailKelola, "arsip", "MDL-CBM-KRT-v2.4.0-rc1", "MDL-CBM-KRT-v2.3.1", "MDL-TIDAK-ADA")

	assert.Equal(t, 1, hasil.Berhasil)
	assert.Equal(t, 2, hasil.Gagal)
	assert.Equal(t, "ARSIP", s.statusModel(t, "MDL-CBM-KRT-v2.4.0-rc1"))
	assert.Equal(t, "PRODUKSI", s.statusModel(t, "MDL-CBM-KRT-v2.3.1"), "model produksi tidak dapat diarsipkan langsung")
	alasan := map[string]string{}
	for _, d := range hasil.Detail {
		alasan[d.ID] = d.Alasan
	}
	assert.Contains(t, alasan["MDL-CBM-KRT-v2.3.1"], "status model")
	assert.Equal(t, "model tidak ditemukan", alasan["MDL-TIDAK-ADA"])
	assert.Equal(t, 1, s.hitung(t, `SELECT COUNT(*) FROM model_promosi WHERE aksi = 'ARSIP' AND model_id = 'MDL-CBM-KRT-v2.4.0-rc1' AND oleh = 'manajer_data'`))
	assert.Equal(t, 1, s.hitung(t, `SELECT COUNT(*) FROM audit_log WHERE aksi = 'ARSIP_MODEL' AND entitas_id = 'MDL-CBM-KRT-v2.4.0-rc1'`))
}

func TestPromosi_PermintaanTidakValid(t *testing.T) {
	s := newMLSuite(t, "prediksi_promosi_400", newFakeML(t).client())

	for nama, body := range map[string]map[string]any{
		"aksi tidak dikenal": {"action": "hapus", "ids": []string{"x"}},
		"tanpa aksi":         {"ids": []string{"x"}},
		"id kosong":          {"action": "arsip", "ids": []string{}},
		"field lama":         {"aksi": "arsip", "ids": []string{"x"}},
	} {
		assert.Equal(t, http.StatusBadRequest, s.post(t, "/v1/prediksi/registri/bulk", emailKelola, body).Code, nama)
	}
}

type simulasiResp struct {
	ID            string  `json:"id"`
	KomoditasKode string  `json:"komoditas_kode"`
	HorizonHari   int     `json:"horizon_hari"`
	HargaKini     int     `json:"harga_kini"`
	Proyeksi      int     `json:"proyeksi"`
	DasarPct      float64 `json:"dasar_pct"`
	HasilPct      float64 `json:"hasil_pct"`
	BawahPct      float64 `json:"bawah_pct"`
	AtasPct       float64 `json:"atas_pct"`
	Efek          []struct {
		Faktor    string  `json:"faktor"`
		Koefisien float64 `json:"koefisien"`
		EfekPct   float64 `json:"efek_pct"`
		Sumber    string  `json:"sumber"`
	} `json:"efek"`
	SumberModel string `json:"sumber_model"`
	Terbatas    bool   `json:"terbatas"`
	Catatan     string `json:"catatan"`
}

func TestSimulasi_DihitungServerDanTersimpan(t *testing.T) {
	s := newMLSuite(t, "prediksi_simulasi", newFakeML(t).client())

	w := s.post(t, "/v1/prediksi/simulasi", emailBerhak, map[string]any{"komoditas_kode": "CBM-KRT", "pasokan_pct": 10, "subsidi_pct": 10})

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	got := decode[simulasiResp](t, w)
	assert.Equal(t, 14.2, got.DasarPct, "proyeksi model produksi (demo) 14 hari")
	assert.Equal(t, 8.9, got.HasilPct, "14,2 - 3,5 - 1,8")
	assert.Equal(t, 14, got.HorizonHari)
	assert.Equal(t, "DEMO", got.SumberModel)
	assert.True(t, got.Terbatas)
	assert.Contains(t, got.Catatan, "belum diestimasi dari data")
	require.Len(t, got.Efek, 3)
	assert.Equal(t, -3.5, got.Efek[1].EfekPct)
	assert.Equal(t, "ASUMSI", got.Efek[1].Sumber)
	assert.Less(t, got.BawahPct, got.HasilPct)
	assert.Less(t, got.HasilPct, got.AtasPct)
	assert.Positive(t, got.Proyeksi)

	assert.Equal(t, 1, s.hitung(t, `SELECT COUNT(*) FROM skenario_simulasi WHERE id = ? AND komoditas_kode = 'CBM-KRT' AND horizon_hari = 14
		AND dibuat_oleh = (SELECT id FROM pengguna WHERE email = ?)
		AND parameter->>'pasokan_pct' = '10' AND (hasil->>'hasil_pct')::numeric = 8.9`, got.ID, emailBerhak))
	assert.Equal(t, 1, s.hitung(t, `SELECT COUNT(*) FROM audit_log WHERE aksi = 'SIMULASI_SKENARIO' AND entitas_id = ?`, got.ID))
}

func TestSimulasi_NetralSamaDenganProyeksiModel(t *testing.T) {
	s := newMLSuite(t, "prediksi_simulasi_netral", newFakeML(t).client())

	got := decode[simulasiResp](t, s.post(t, "/v1/prediksi/simulasi", emailBerhak, map[string]any{"komoditas_kode": "CBM-KRT", "horizon_hari": 7, "kabupaten_kode": "1106"}))

	assert.Equal(t, got.DasarPct, got.HasilPct)
	assert.Equal(t, 7, got.HorizonHari)
	for _, e := range got.Efek {
		assert.Zero(t, e.EfekPct)
	}
}

func TestSimulasi_MemakaiModelTerlatihBilaProduksi(t *testing.T) {
	s := newMLSuite(t, "prediksi_simulasi_model", newFakeML(t).client())
	m := decode[modelResp](t, s.post(t, "/v1/prediksi/latih", emailKelola, map[string]string{"komoditas_kode": "CBM-KRT"}))
	_, kode := s.bulkModel(t, emailKelola, "promosi", m.ID)
	require.Equal(t, http.StatusOK, kode)

	got := decode[simulasiResp](t, s.post(t, "/v1/prediksi/simulasi", emailBerhak, map[string]any{"komoditas_kode": "CBM-KRT"}))

	assert.Equal(t, "MODEL", got.SumberModel)
}

func TestSimulasi_PermintaanTidakValid(t *testing.T) {
	s := newMLSuite(t, "prediksi_simulasi_400", newFakeML(t).client())

	for nama, body := range map[string]map[string]any{
		"tanpa komoditas":       {},
		"pasokan terlalu besar": {"komoditas_kode": "CBM-KRT", "pasokan_pct": 500},
		"hujan negatif":         {"komoditas_kode": "CBM-KRT", "hujan_pct": -1},
		"subsidi berlebih":      {"komoditas_kode": "CBM-KRT", "subsidi_pct": 101},
		"kabupaten salah":       {"komoditas_kode": "CBM-KRT", "kabupaten_kode": "abcd"},
		"horizon nol":           {"komoditas_kode": "CBM-KRT", "horizon_hari": 0},
	} {
		assert.Equal(t, http.StatusBadRequest, s.post(t, "/v1/prediksi/simulasi", emailBerhak, body).Code, nama)
	}
	assert.Equal(t, http.StatusNotFound, s.post(t, "/v1/prediksi/simulasi", emailBerhak, map[string]any{"komoditas_kode": "XXX"}).Code)
	assert.Equal(t, 0, s.hitung(t, `SELECT COUNT(*) FROM skenario_simulasi`), "permintaan gagal tidak menyimpan skenario")
}
