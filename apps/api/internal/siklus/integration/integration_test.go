// Package integration menguji penentuan siklus otomatis (08-B) terhadap PostgreSQL sungguhan: mesin (alert EWS dan stok
// cold storage menjadi TEKANAN/SURPLUS/NORMAL, masa pulih, prioritas, idempoten, serentak, audit), API
// (GET /v1/siklus/aktif, GET /v1/siklus/aturan, POST /v1/siklus/evaluasi), seed aturan, dan interaksinya dengan seed demo.
//
// Basis data tiap tes dibuat dari migrasi sungguhan (goose) dan hanya diisi seed yang dibutuhkan, supaya data demo
// modul lain (harga, cuaca, cold storage) tidak ikut terbaca mesin: reference (master, RBAC, aturan EWS, fasilitas cold
// storage, aturan siklus) ditambah demo/01_auth_users.sql untuk login. Waktu yang dimanipulasi tes memakai jam basis data
// (now() di SQL), sama dengan jam mesin, sehingga selisih jam antara host dan kontainer Docker tidak memengaruhi hasil.
package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	authRoutes "github.com/garda-inflasi/api/internal/auth/routes"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/internal/siklus/domain"
	"github.com/garda-inflasi/api/internal/siklus/engine"
	siklusjobs "github.com/garda-inflasi/api/internal/siklus/jobs"
	siklusRoutes "github.com/garda-inflasi/api/internal/siklus/routes"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/seed"
	"github.com/garda-inflasi/api/pkg/seedkit"
	"github.com/garda-inflasi/api/pkg/token"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	jwtSecret = "siklus-integration-secret-key-32chars!"

	// MANAJER_DATA: dashboard.operasional, ews.lihat, dan ews.atur_ambang.
	emailAtur = "manajer.data@gardainflasi.id"
	// STRATEGIS_BI: dashboard.operasional dan ews.lihat, tanpa ews.atur_ambang.
	emailLihat = "strategis.bi@gardainflasi.id"
	// MANAJER_RISIKO: dashboard.operasional, ews.lihat, ews.akui, ews.eskalasi; tanpa ews.atur_ambang.
	emailRisiko = "manajer.risiko@gardainflasi.id"
	// OPS_MAHASISWA: tanpa dashboard.operasional dan tanpa izin EWS.
	emailTanpaIzin = "mahasiswa.1@gardainflasi.id"
)

var pg *pgtest.Server

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(pgtest.Main(m, &pg))
}

type suite struct {
	db     *gorm.DB
	router *gin.Engine
	engine *engine.Engine
	tokens map[string]string
}

// bareDB adalah basis data hasil migrasi tanpa seed apa pun (basis data kosong).
func bareDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db := pg.NewDatabase(t, name)
	pgtest.MigrateUp(t, db)
	return db
}

// newSuite membuat basis data bermigrasi dengan seed reference dan akun demo, lengkap dengan router.
func newSuite(t *testing.T, name string) *suite {
	t.Helper()
	db := bareDB(t, name)
	require.NoError(t, seed.SeedReference(t.Context(), db))
	require.NoError(t, seed.SeedFiles(t.Context(), db, "demo/01_auth_users.sql"))

	tokens, log := token.NewManager(jwtSecret), logger.NewDefault()
	router := gin.New()
	v1 := router.Group("/v1")
	authRoutes.Register(v1, &authRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
	siklusRoutes.Register(v1, &siklusRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
	return &suite{db: db, router: router, engine: siklusjobs.Shared(db, log).Engine, tokens: map[string]string{}}
}

func (s *suite) call(t *testing.T, method, path, email string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var payload bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&payload).Encode(body))
	}
	req := httptest.NewRequest(method, path, &payload)
	req.Header.Set("Content-Type", "application/json")
	if email != "" {
		req.Header.Set("Authorization", "Bearer "+s.login(t, email))
	}
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	return w
}

func (s *suite) login(t *testing.T, email string) string {
	t.Helper()
	if tok, ok := s.tokens[email]; ok {
		return tok
	}
	body, _ := json.Marshal(map[string]string{"email": email, "password": seed.DemoPassword})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var res struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	s.tokens[email] = res.Data.AccessToken
	return res.Data.AccessToken
}

func data[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var res struct {
		Data T `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res), w.Body.String())
	return res.Data
}

// jsonField membaca satu kunci tingkat atas dokumen JSON (mis. kolom audit sebelum/sesudah) sebagai teks.
func jsonField(t *testing.T, dokumen, kunci string) string {
	t.Helper()
	var objek map[string]any
	require.NoError(t, json.Unmarshal([]byte(dokumen), &objek), dokumen)
	nilai, ada := objek[kunci]
	require.True(t, ada, "kunci %q tidak ada di %s", kunci, dokumen)
	teks, ok := nilai.(string)
	require.True(t, ok, "kunci %q bukan teks di %s", kunci, dokumen)
	return teks
}

// bacaAlasan mengurai kolom siklus_ekonomi.alasan (jsonb dibaca sebagai teks; urutan kunci ditentukan PostgreSQL).
func bacaAlasan(t *testing.T, dokumen string) domain.Alasan {
	t.Helper()
	var alasan domain.Alasan
	require.NoError(t, json.Unmarshal([]byte(dokumen), &alasan), dokumen)
	return alasan
}

// uid memberi UUID akun uji dari slug lamanya (rumus yang sama dengan seed dan migrasi 000110).
func uid(slug string) string { return seedkit.IDPengguna(slug) }

var opsiJadwal = domain.OpsiEvaluasi{Pemicu: domain.PemicuJadwal}

// evaluasi menjalankan satu putaran mesin untuk Aceh dan menggagalkan tes bila galat.
func (s *suite) evaluasi(t *testing.T) domain.HasilEvaluasi {
	t.Helper()
	hasil, err := s.engine.Evaluasi(t.Context(), domain.WilayahBawaan, opsiJadwal)
	require.NoError(t, err)
	return hasil
}

// baris siklus ekonomi untuk pembacaan tes.
type barisSiklus struct {
	ID         int64
	Kode       *string
	Siklus     string
	MulaiPada  time.Time
	AkhirPada  *time.Time
	Otomatis   bool
	Alasan     *string
	PulihSejak *time.Time
}

func (s *suite) semuaSiklus(t *testing.T) []barisSiklus {
	t.Helper()
	return semuaSiklus(t, s.db)
}

func semuaSiklus(t *testing.T, db *gorm.DB) []barisSiklus {
	t.Helper()
	var rows []barisSiklus
	require.NoError(t, db.Raw(`SELECT id, kode, siklus::text AS siklus, mulai_pada, akhir_pada, otomatis, alasan::text AS alasan, pulih_sejak
		FROM siklus_ekonomi WHERE wilayah_kode = '11' ORDER BY mulai_pada, id`).Scan(&rows).Error)
	return rows
}

// aktif mengembalikan satu-satunya siklus aktif Aceh; gagal bila tidak tepat satu.
func (s *suite) aktif(t *testing.T) barisSiklus {
	t.Helper()
	var aktifRows []barisSiklus
	for _, r := range s.semuaSiklus(t) {
		if r.AkhirPada == nil {
			aktifRows = append(aktifRows, r)
		}
	}
	require.Len(t, aktifRows, 1, "harus tepat satu siklus aktif untuk Aceh")
	return aktifRows[0]
}

// urutanSiklus menulis siklus semua baris berurutan menurut waktu mulai, mis. [NORMAL TEKANAN SURPLUS].
func (s *suite) urutanSiklus(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, r := range s.semuaSiklus(t) {
		out = append(out, r.Siklus)
	}
	return out
}

// alert menyisipkan satu alert EWS langsung (tanpa rule engine EWS) di kabupaten Aceh.
func (s *suite) alert(t *testing.T, kode, jenis, severity, status, kabupaten string) {
	t.Helper()
	require.NoError(t, s.db.Exec(`INSERT INTO ews_alert (kode, severity, jenis, judul, pesan, kabupaten_kode, nilai, satuan_nilai, skor_dampak, status, sumber, selesai_pada)
		VALUES (?, CAST(? AS severity), CAST(? AS jenis_ews), 'Uji', 'Alert uji siklus', ?, 10, '%', 70, CAST(? AS status_ews), 'Uji',
		        CASE WHEN ? = 'SELESAI' THEN now() END)`, kode, severity, jenis, kabupaten, status, status).Error)
}

// selesaikanAlert menutup alert seperti aksi pengguna (SELESAI + selesai_pada).
func (s *suite) selesaikanAlert(t *testing.T, kode string) {
	t.Helper()
	require.NoError(t, s.db.Exec(`UPDATE ews_alert SET status = 'SELESAI', selesai_pada = now() WHERE kode = ?`, kode).Error)
}

// mundurkanPulih memundurkan penanda masa pulih siklus aktif menjadi sekian jam yang lalu (jam basis data).
func (s *suite) mundurkanPulih(t *testing.T, jam float64) {
	t.Helper()
	res := s.db.Exec(`UPDATE siklus_ekonomi SET pulih_sejak = now() - make_interval(secs => ?) WHERE wilayah_kode = '11' AND akhir_pada IS NULL`, jam*3600)
	require.NoError(t, res.Error)
	require.EqualValues(t, 1, res.RowsAffected)
}

// lot menyisipkan satu lot stok layak simpan (masuk kemarin, kedaluwarsa sebulan lagi) di fasilitas CS-01.
func (s *suite) lot(t *testing.T, kode string, ton float64) {
	t.Helper()
	require.NoError(t, s.db.Exec(`INSERT INTO stok_cold_storage (kode, cold_storage_id, komoditas_kode, volume_ton, masuk_pada, pemilik, risiko_food_loss, kedaluwarsa_pada)
		VALUES (?, (SELECT id FROM cold_storage WHERE kode = 'CS-01'), 'CBM-KRT', ?, now() - interval '1 day', 'Koperasi', 0.1, now() + interval '30 days')`, kode, ton).Error)
}

type barisAudit struct {
	Aksi      string
	EntitasID string
	AktorID   *string
	Sebelum   *string
	Sesudah   *string
	Metadata  *string
}

func (s *suite) audit(t *testing.T, aksi string) []barisAudit {
	t.Helper()
	var rows []barisAudit
	require.NoError(t, s.db.Raw(`SELECT aksi, entitas_id, aktor_id::text AS aktor_id, sebelum::text AS sebelum, sesudah::text AS sesudah, metadata::text AS metadata
		FROM audit_log WHERE entitas = 'siklus_ekonomi' AND aksi = ? ORDER BY id`, aksi).Scan(&rows).Error)
	return rows
}

// --- API -----------------------------------------------------------------------------------------------------------

type responAktif struct {
	Siklus   string `json:"siklus"`
	Sejak    string `json:"sejak"`
	Otomatis bool   `json:"otomatis"`
	Alasan   *struct {
		Ringkasan string `json:"ringkasan"`
		Pemicu    []struct {
			Aturan    string   `json:"aturan"`
			Siklus    string   `json:"siklus"`
			Indikator string   `json:"indikator"`
			Nilai     float64  `json:"nilai"`
			Ambang    float64  `json:"ambang"`
			Satuan    string   `json:"satuan"`
			Prioritas int      `json:"prioritas"`
			Rujukan   []string `json:"rujukan"`
		} `json:"pemicu"`
		Ditekan []struct {
			Aturan string `json:"aturan"`
			Siklus string `json:"siklus"`
		} `json:"ditekan"`
	} `json:"alasan"`
}

func TestAktif_OtorisasiDanBentukLama(t *testing.T) {
	s := newSuite(t, "siklus_it_aktif")
	require.NoError(t, s.db.Exec(`INSERT INTO siklus_ekonomi (kode, wilayah_kode, siklus, mulai_pada) VALUES ('SE-11-UJI', '11', 'TEKANAN', '2026-09-26 20:00:00+00')`).Error)

	assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodGet, "/v1/siklus/aktif", "", nil).Code)
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, "/v1/siklus/aktif", emailTanpaIzin, nil).Code)

	w := s.call(t, http.MethodGet, "/v1/siklus/aktif", emailLihat, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	got := data[responAktif](t, w)
	// Bentuk lama tidak berubah: siklus huruf kecil dan tanggal waktu Aceh (26 September 20.00 UTC = 27 September WIB).
	assert.Equal(t, "tekanan", got.Siklus)
	assert.Equal(t, "2026-09-27", got.Sejak)
	// Tambahan aditif: baris dari seed atau penetapan manual tidak otomatis dan tanpa alasan (null, bukan hilang).
	assert.False(t, got.Otomatis)
	assert.Nil(t, got.Alasan)
	assert.Contains(t, w.Body.String(), `"alasan":null`)

	assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodGet, "/v1/siklus/aktif?wilayah_kode=99", emailLihat, nil).Code)
}

func TestAktif_BasisDataKosongTetap404SampaiEvaluasiPertama(t *testing.T) {
	s := newSuite(t, "siklus_it_aktif_kosong")
	assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodGet, "/v1/siklus/aktif", emailLihat, nil).Code)

	s.evaluasi(t)

	w := s.call(t, http.MethodGet, "/v1/siklus/aktif", emailLihat, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	got := data[responAktif](t, w)
	assert.Equal(t, "normal", got.Siklus)
	assert.True(t, got.Otomatis)
	require.NotNil(t, got.Alasan)
	assert.Equal(t, "Siklus awal NORMAL: tidak ada aturan yang memicu.", got.Alasan.Ringkasan)
	assert.Empty(t, got.Alasan.Pemicu)
	assert.Empty(t, got.Alasan.Ditekan)
	assert.Contains(t, w.Body.String(), `"pemicu":[]`, "daftar kosong, bukan null")
}

func TestAktif_MenampilkanAlasanKeputusanMesin(t *testing.T) {
	s := newSuite(t, "siklus_it_aktif_alasan")
	s.alert(t, "EWS-T1", "LONJAKAN_HARGA", "KRITIS", "BARU", "1171")
	s.alert(t, "EWS-T2", "LONJAKAN_HARGA", "SIAGA", "DIAKUI", "1108")
	s.alert(t, "EWS-T3", "OVER_SUPPLY", "SIAGA", "BARU", "1106")
	s.evaluasi(t)

	got := data[responAktif](t, s.call(t, http.MethodGet, "/v1/siklus/aktif", emailLihat, nil))

	assert.Equal(t, "tekanan", got.Siklus)
	assert.True(t, got.Otomatis)
	require.NotNil(t, got.Alasan)
	require.Len(t, got.Alasan.Pemicu, 1)
	pemicu := got.Alasan.Pemicu[0]
	assert.Equal(t, "SKL-TKN-LONJAKAN-HARGA", pemicu.Aturan)
	assert.Equal(t, "tekanan", pemicu.Siklus)
	assert.Equal(t, "ALERT_EWS", pemicu.Indikator)
	assert.EqualValues(t, 2, pemicu.Nilai)
	assert.EqualValues(t, 1, pemicu.Ambang)
	assert.Equal(t, "alert", pemicu.Satuan)
	assert.Equal(t, 10, pemicu.Prioritas)
	assert.Equal(t, []string{"EWS-T1", "EWS-T2"}, pemicu.Rujukan, "alert contoh: paling parah dulu")
	require.Len(t, got.Alasan.Ditekan, 1)
	assert.Equal(t, "SKL-SRP-OVER-SUPPLY", got.Alasan.Ditekan[0].Aturan)
	assert.Equal(t, "surplus", got.Alasan.Ditekan[0].Siklus)
	assert.Contains(t, got.Alasan.Ringkasan, "Siklus awal TEKANAN: 2 alert LONJAKAN_HARGA ≥ SIAGA.")
	assert.Contains(t, got.Alasan.Ringkasan, "Aturan SURPLUS ikut memicu")
}

type responAturan struct {
	ID          string   `json:"id"`
	Kode        string   `json:"kode"`
	Siklus      string   `json:"siklus"`
	Indikator   string   `json:"indikator"`
	JenisEws    *string  `json:"jenis_ews"`
	SeverityMin *string  `json:"severity_min"`
	JumlahMin   *int     `json:"jumlah_min"`
	Ambang      *float64 `json:"ambang"`
	Prioritas   int      `json:"prioritas"`
	PulihJam    int      `json:"pulih_jam"`
	Aktif       bool     `json:"aktif"`
	Keterangan  string   `json:"keterangan"`
}

func TestAturan_BacaOtorisasiDanIsiBawaan(t *testing.T) {
	s := newSuite(t, "siklus_it_aturan")

	assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodGet, "/v1/siklus/aturan", "", nil).Code)
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, "/v1/siklus/aturan", emailTanpaIzin, nil).Code)
	w := s.call(t, http.MethodGet, "/v1/siklus/aturan", emailLihat, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	aturan := data[[]responAturan](t, w)
	require.Len(t, aturan, 5)
	var kode []string
	for _, a := range aturan {
		kode = append(kode, a.Kode)
		assert.Regexp(t, `^[1-9][0-9]*$`, a.ID, "ID BIGINT sebagai string desimal")
		assert.True(t, a.Aktif)
		assert.NotEmpty(t, a.Keterangan)
	}
	// Urutan seperti mesin: prioritas, TEKANAN sebelum SURPLUS, lalu ID.
	assert.Equal(t, []string{
		"SKL-TKN-LONJAKAN-HARGA", "SKL-TKN-KELANGKAAN-STOK", "SKL-TKN-ANOMALI-CUACA", "SKL-SRP-OVER-SUPPLY", "SKL-SRP-UTILISASI-STOK",
	}, kode)

	lonjakan := aturan[0]
	assert.Equal(t, "tekanan", lonjakan.Siklus)
	assert.Equal(t, "ALERT_EWS", lonjakan.Indikator)
	require.NotNil(t, lonjakan.JenisEws)
	assert.Equal(t, "LONJAKAN_HARGA", *lonjakan.JenisEws)
	require.NotNil(t, lonjakan.SeverityMin)
	assert.Equal(t, "SIAGA", *lonjakan.SeverityMin)
	require.NotNil(t, lonjakan.JumlahMin)
	assert.Equal(t, 1, *lonjakan.JumlahMin)
	assert.Nil(t, lonjakan.Ambang)
	assert.Equal(t, 10, lonjakan.Prioritas)
	assert.Equal(t, 48, lonjakan.PulihJam)

	utilisasi := aturan[4]
	assert.Equal(t, "surplus", utilisasi.Siklus)
	assert.Equal(t, "UTILISASI_STOK", utilisasi.Indikator)
	assert.Nil(t, utilisasi.JenisEws)
	require.NotNil(t, utilisasi.Ambang)
	assert.EqualValues(t, 90, *utilisasi.Ambang)
	assert.Equal(t, 20, utilisasi.Prioritas)
	assert.Equal(t, 72, utilisasi.PulihJam)

	// Aturan nonaktif tetap terlihat (aktif false) supaya operator melihat seluruh konfigurasi.
	require.NoError(t, s.db.Exec(`UPDATE aturan_siklus SET aktif = false WHERE kode = 'SKL-SRP-OVER-SUPPLY'`).Error)
	for _, a := range data[[]responAturan](t, s.call(t, http.MethodGet, "/v1/siklus/aturan", emailLihat, nil)) {
		assert.Equal(t, a.Kode != "SKL-SRP-OVER-SUPPLY", a.Aktif, a.Kode)
	}
}

type responEvaluasi struct {
	WilayahKode   string  `json:"wilayah_kode"`
	Dilewati      bool    `json:"dilewati"`
	SebabDilewati *string `json:"sebab_dilewati"`
	Berubah       bool    `json:"berubah"`
	Sebelumnya    *string `json:"sebelumnya"`
	Siklus        *string `json:"siklus"`
	Sejak         *string `json:"sejak"`
	Otomatis      bool    `json:"otomatis"`
	Ringkasan     string  `json:"ringkasan"`
	Aturan        []struct {
		Kode      string   `json:"kode"`
		Siklus    string   `json:"siklus"`
		Indikator string   `json:"indikator"`
		Terukur   bool     `json:"terukur"`
		Nilai     float64  `json:"nilai"`
		Ambang    float64  `json:"ambang"`
		Satuan    string   `json:"satuan"`
		Memicu    bool     `json:"memicu"`
		Prioritas int      `json:"prioritas"`
		Rujukan   []string `json:"rujukan"`
		Rincian   string   `json:"rincian"`
	} `json:"aturan"`
}

func TestEvaluasiManual_OtorisasiHasilDanAudit(t *testing.T) {
	s := newSuite(t, "siklus_it_manual")

	// 401 tanpa token; 403 untuk yang tidak punya ews.atur_ambang walau punya izin dasbor/EWS lain.
	assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodPost, "/v1/siklus/evaluasi", "", nil).Code)
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPost, "/v1/siklus/evaluasi", emailTanpaIzin, nil).Code)
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPost, "/v1/siklus/evaluasi", emailLihat, nil).Code, "ews.lihat saja tidak cukup")
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPost, "/v1/siklus/evaluasi", emailRisiko, nil).Code, "ews.eskalasi saja tidak cukup")
	assert.Empty(t, s.semuaSiklus(t), "permintaan yang ditolak tidak menjalankan mesin")
	assert.Empty(t, s.audit(t, "EVALUASI"))

	// Putaran pertama pada basis data tanpa siklus: NORMAL awal.
	w := s.call(t, http.MethodPost, "/v1/siklus/evaluasi", emailAtur, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	pertama := data[responEvaluasi](t, w)
	assert.Equal(t, "11", pertama.WilayahKode)
	assert.False(t, pertama.Dilewati)
	assert.Nil(t, pertama.SebabDilewati)
	assert.True(t, pertama.Berubah)
	assert.Nil(t, pertama.Sebelumnya)
	require.NotNil(t, pertama.Siklus)
	assert.Equal(t, "normal", *pertama.Siklus)
	assert.True(t, pertama.Otomatis)
	assert.Len(t, pertama.Aturan, 5, "hasil ukur tiap aturan aktif")

	// Alert SIAGA muncul; evaluasi manual berikutnya menggantinya ke TEKANAN atas nama pemintanya.
	s.alert(t, "EWS-M1", "LONJAKAN_HARGA", "SIAGA", "BARU", "1171")
	w = s.call(t, http.MethodPost, "/v1/siklus/evaluasi", emailAtur, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	kedua := data[responEvaluasi](t, w)
	assert.True(t, kedua.Berubah)
	require.NotNil(t, kedua.Sebelumnya)
	assert.Equal(t, "normal", *kedua.Sebelumnya)
	require.NotNil(t, kedua.Siklus)
	assert.Equal(t, "tekanan", *kedua.Siklus)
	assert.Contains(t, kedua.Ringkasan, "Berpindah dari NORMAL ke TEKANAN")
	var lonjakan *int
	for i, a := range kedua.Aturan {
		if a.Kode == "SKL-TKN-LONJAKAN-HARGA" {
			lonjakan = &i
		}
	}
	require.NotNil(t, lonjakan)
	hasil := kedua.Aturan[*lonjakan]
	assert.True(t, hasil.Memicu)
	assert.True(t, hasil.Terukur)
	assert.EqualValues(t, 1, hasil.Nilai)
	assert.EqualValues(t, 1, hasil.Ambang)
	assert.Equal(t, "alert", hasil.Satuan)
	assert.Equal(t, []string{"EWS-M1"}, hasil.Rujukan)

	// Putaran ketiga tidak mengubah apa pun, tetapi tetap tercatat sebagai permintaan.
	ketiga := data[responEvaluasi](t, s.call(t, http.MethodPost, "/v1/siklus/evaluasi", emailAtur, nil))
	assert.False(t, ketiga.Berubah)
	assert.Contains(t, ketiga.Ringkasan, "Siklus tetap TEKANAN")

	// Audit: tiga permintaan EVALUASI oleh pengguna, dan dua pergantian siklus atas nama pengguna yang sama.
	evaluasi := s.audit(t, "EVALUASI")
	require.Len(t, evaluasi, 3)
	for _, a := range evaluasi {
		require.NotNil(t, a.AktorID)
		assert.Equal(t, uid("manajer_data"), *a.AktorID)
		assert.Equal(t, "mesin siklus", a.EntitasID)
	}
	ganti := s.audit(t, "GANTI_SIKLUS")
	require.Len(t, ganti, 2)
	for _, a := range ganti {
		require.NotNil(t, a.AktorID)
		assert.Equal(t, uid("manajer_data"), *a.AktorID, "pergantian karena evaluasi manual dicatat atas nama pemintanya")
		require.NotNil(t, a.Metadata)
		assert.Contains(t, *a.Metadata, `"pemicu": "manual"`)
	}
}

func TestEvaluasiManual_TanpaAturanAktifDilewatiDanSiklusTidakBerubah(t *testing.T) {
	s := newSuite(t, "siklus_it_manual_tanpa_aturan")
	s.alert(t, "EWS-N1", "LONJAKAN_HARGA", "KRITIS", "BARU", "1171")
	s.evaluasi(t)
	awal := s.aktif(t)
	require.Equal(t, "TEKANAN", awal.Siklus)

	require.NoError(t, s.db.Exec(`UPDATE aturan_siklus SET aktif = false`).Error)
	s.selesaikanAlert(t, "EWS-N1")
	s.mundurkanPulih(t, 100) // masa pulih sudah lewat jauh: dengan aturan aktif, siklus ini akan dilepas
	w := s.call(t, http.MethodPost, "/v1/siklus/evaluasi", emailAtur, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	got := data[responEvaluasi](t, w)

	assert.True(t, got.Dilewati)
	require.NotNil(t, got.SebabDilewati)
	assert.Equal(t, "tanpa_aturan", *got.SebabDilewati)
	assert.False(t, got.Berubah)
	require.NotNil(t, got.Siklus)
	assert.Equal(t, "tekanan", *got.Siklus, "siklus yang ada dilaporkan apa adanya")
	assert.Equal(t, awal.ID, s.aktif(t).ID, "tanpa aturan aktif mesin tidak boleh menyimpulkan 'normal' dan melepas siklus")
	assert.Len(t, s.audit(t, "GANTI_SIKLUS"), 1, "hanya pergantian awal")
}
