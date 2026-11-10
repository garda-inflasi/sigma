package integration

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/garda-inflasi/api/pkg/seed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type flagValidasi struct {
	DiLuarRentang bool `json:"di_luar_rentang"`
	Outlier       bool `json:"outlier"`
	Duplikat      bool `json:"duplikat"`
	LuarGeofence  bool `json:"luar_geofence"`
	TanpaLokasi   bool `json:"tanpa_lokasi"`
	Bentrok       bool `json:"bentrok"`
	Median        int  `json:"median"`
	JarakMeter    int  `json:"jarak_meter"`
}

type hasilCek struct {
	Status        string       `json:"status"`
	Harga         int          `json:"harga"`
	DeviasiPct    float64      `json:"deviasi_pct"`
	SkorAnomali   float64      `json:"skor_anomali"`
	DalamGeofence bool         `json:"dalam_geofence"`
	Flag          flagValidasi `json:"flag_validasi"`
}

// Pasar Peunayong (PSR-1171-01) berada di 5.5603, 95.3237 dengan radius 400 m.
const (
	latPasar, lngPasar = 5.5603, 95.3237
	latLuar            = 5.60 // sekitar 4,4 km dari pasar
)

// kirimCek mencatat satu entri bawang putih oleh mahasiswa_1 dan mengembalikan hasil cek otomatisnya.
func (s *suite) kirimCek(t *testing.T, body map[string]any) hasilCek {
	t.Helper()
	dasar := map[string]any{"komoditas_kode": "BWP", "pasar_id": "PSR-1171-01", "tipe": "KONSUMEN", "harga": 40000}
	for k, v := range body {
		dasar[k] = v
	}
	w := s.call(t, http.MethodPost, "/v1/harga", emailMahasiswa, dasar)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	id := data[struct {
		ID string `json:"id"`
	}](t, w).ID
	return data[hasilCek](t, s.call(t, http.MethodGet, "/v1/harga/"+id, emailKoordinator, nil))
}

func TestValidasiOtomatis_NormalEntryRaisesNoRangeOrOutlierFlag(t *testing.T) {
	s := sharedSuite(t)
	ringkasan := data[[]struct {
		Harga int `json:"harga"`
	}](t, s.call(t, http.MethodGet, "/v1/harga?komoditas=BWP&wilayah=1171&status=DIPUBLIKASI&limit=1", emailKoordinator, nil))
	require.NotEmpty(t, ringkasan)

	cek := s.kirimCek(t, map[string]any{"harga": ringkasan[0].Harga, "lat": latPasar, "lng": lngPasar})

	assert.False(t, cek.Flag.DiLuarRentang)
	assert.False(t, cek.Flag.Outlier)
	assert.False(t, cek.Flag.LuarGeofence)
	assert.False(t, cek.Flag.TanpaLokasi)
	assert.True(t, cek.DalamGeofence)
	assert.Positive(t, cek.Flag.Median, "median histori resmi tercatat")
	assert.Less(t, cek.SkorAnomali, 0.3)
}

func TestValidasiOtomatis_OutlierWithinRange(t *testing.T) {
	s := sharedSuite(t)

	cek := s.kirimCek(t, map[string]any{"harga": 70000, "lat": latPasar, "lng": lngPasar})

	assert.True(t, cek.Flag.Outlier)
	assert.False(t, cek.Flag.DiLuarRentang, "70.000 masih di dalam rentang wajar bawang putih")
	assert.Greater(t, cek.DeviasiPct, 40.0)
	assert.Greater(t, cek.SkorAnomali, 0.5)
}

func TestValidasiOtomatis_OutOfRangeWrongUnit(t *testing.T) {
	s := sharedSuite(t)

	cek := s.kirimCek(t, map[string]any{"harga": 400000, "lat": latPasar, "lng": lngPasar})

	assert.True(t, cek.Flag.DiLuarRentang)
	assert.True(t, cek.Flag.Outlier)
	assert.Equal(t, 1.0, cek.SkorAnomali)
}

func TestValidasiOtomatis_GeofenceAndMissingLocation(t *testing.T) {
	s := sharedSuite(t)

	luar := s.kirimCek(t, map[string]any{"lat": latLuar, "lng": lngPasar})
	assert.True(t, luar.Flag.LuarGeofence)
	assert.False(t, luar.DalamGeofence)
	assert.InDelta(t, 4400, luar.Flag.JarakMeter, 200)

	tanpa := s.kirimCek(t, nil)
	assert.True(t, tanpa.Flag.TanpaLokasi)
	assert.False(t, tanpa.Flag.LuarGeofence)
	assert.True(t, tanpa.DalamGeofence)
}

func TestValidasiOtomatis_DuplicateByMarketCommodityVendorAndTime(t *testing.T) {
	s := sharedSuite(t)
	pedagang := map[string]any{"kode_pedagang": "UJI-DUP-1", "harga": 41000}

	pertama := s.kirimCek(t, pedagang)
	kedua := s.kirimCek(t, pedagang)
	lain := s.kirimCek(t, map[string]any{"kode_pedagang": "UJI-DUP-2", "harga": 41000})

	assert.False(t, pertama.Flag.Duplikat)
	assert.True(t, kedua.Flag.Duplikat)
	assert.False(t, lain.Flag.Duplikat, "pedagang berbeda bukan duplikat")
}

func TestValidasiOtomatis_PertahankanMarksConflictFlag(t *testing.T) {
	s := sharedSuite(t)
	waktu := time.Now().AddDate(-1, 0, 0).Format(time.RFC3339)
	kirim := func(email string, harga int, pertahankan bool) *httptest.ResponseRecorder {
		return s.call(t, http.MethodPost, "/v1/harga", email, map[string]any{
			"komoditas_kode": "GLA", "pasar_id": "PSR-1171-01", "tipe": "KONSUMEN", "harga": harga,
			"dicatat_pada": waktu, "pertahankan": pertahankan,
		})
	}
	require.Equal(t, http.StatusCreated, kirim(emailMahasiswa, 18000, false).Code)

	ditolak := kirim("mahasiswa.2@gardainflasi.id", 21000, false)
	require.Equal(t, http.StatusConflict, ditolak.Code, ditolak.Body.String())
	versi := data[struct {
		VersiServer struct {
			Harga int `json:"harga"`
		} `json:"versi_server"`
	}](t, ditolak).VersiServer
	assert.Equal(t, 18000, versi.Harga)

	dipertahankan := kirim("mahasiswa.2@gardainflasi.id", 21000, true)
	require.Equal(t, http.StatusCreated, dipertahankan.Code, dipertahankan.Body.String())
	id := data[struct {
		ID string `json:"id"`
	}](t, dipertahankan).ID
	cek := data[hasilCek](t, s.call(t, http.MethodGet, "/v1/harga/"+id, emailKoordinator, nil))
	assert.True(t, cek.Flag.Bentrok, "entri yang dipertahankan walau bentrok ditandai untuk validator")
}

func TestValidasiOtomatis_QueueCanBeOrderedByScore(t *testing.T) {
	s := sharedSuite(t)

	w := s.call(t, http.MethodGet, "/v1/harga?status=DIAJUKAN&urut=skor&limit=200", emailKoordinator, nil)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	rows := data[[]struct {
		SkorAnomali float64 `json:"skor_anomali"`
	}](t, w)
	require.Greater(t, len(rows), 20)
	for i := 1; i < len(rows); i++ {
		assert.GreaterOrEqual(t, rows[i-1].SkorAnomali, rows[i].SkorAnomali, "skor menurun")
	}
	assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodGet, "/v1/harga?urut=acak", emailKoordinator, nil).Code)
}

func TestValidasiOtomatis_SeededDetailExposesFlags(t *testing.T) {
	s := sharedSuite(t)
	var id string
	require.NoError(t, s.db.Raw("SELECT id FROM entri_harga WHERE kode LIKE 'HRG-D%' AND (flag_validasi->>'outlier')::boolean LIMIT 1").Scan(&id).Error)
	require.NotEmpty(t, id, "histori demo memuat outlier bertanda")

	// Outlier seed ini bisa di kabupaten mana pun: dibuka peran seluruh Aceh.
	cek := data[hasilCek](t, s.call(t, http.MethodGet, "/v1/harga/"+id, emailLihatSaja, nil))

	assert.True(t, cek.Flag.Outlier)
}

func TestBatasHarga_ReferenceSeedCoversEveryActiveCommodity(t *testing.T) {
	s := sharedSuite(t)

	var tanpaBatas int64
	require.NoError(t, s.db.Raw(`
		SELECT count(*) FROM komoditas k
		 WHERE k.status = 'AKTIF' AND NOT EXISTS (SELECT 1 FROM batas_harga b WHERE b.komoditas_kode = k.kode AND b.kabupaten_kode IS NULL)`).Scan(&tanpaBatas).Error)
	assert.Zero(t, tanpaBatas)

	var min, maks, acuan int
	row := s.db.Raw(`SELECT b.batas_min, b.batas_maks, k.harga_acuan FROM batas_harga b JOIN komoditas k ON k.kode = b.komoditas_kode WHERE k.kode = 'CBM-KRT'`).Row()
	require.NoError(t, row.Scan(&min, &maks, &acuan))
	assert.Less(t, min, acuan)
	assert.Greater(t, maks, acuan)
}

func TestBatasHarga_ReseedKeepsAdminEdits(t *testing.T) {
	s := sharedSuite(t)
	require.NoError(t, s.db.Exec("UPDATE batas_harga SET batas_maks = 99999 WHERE kode = 'BH-TLR'").Error)

	require.NoError(t, seed.SeedFiles(t.Context(), s.db, "reference/06_harga_pasar.sql"))

	var maks int
	require.NoError(t, s.db.Raw("SELECT batas_maks FROM batas_harga WHERE kode = 'BH-TLR'").Scan(&maks).Error)
	assert.Equal(t, 99999, maks)
}
