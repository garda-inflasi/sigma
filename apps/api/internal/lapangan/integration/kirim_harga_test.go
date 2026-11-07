package integration

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lampau adalah waktu pencatatan setahun lalu: jauh dari histori demo (90 hari),
// sehingga kiriman uji tidak bentrok dengan entri seed. Tiap tes memakai
// komoditas sendiri agar kiriman antar tes tidak saling bentrok.
func lampau() time.Time { return time.Now().AddDate(-1, 0, 0).Truncate(time.Minute) }

type hasilKirim struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type bentrokBody struct {
	VersiServer struct {
		ID          string `json:"id"`
		Harga       int    `json:"harga"`
		DicatatPada string `json:"dicatat_pada"`
		Petugas     string `json:"petugas"`
	} `json:"versi_server"`
}

func (s *suite) kirim(t *testing.T, email, kunci string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	headers := map[string]string{}
	if kunci != "" {
		headers["Idempotency-Key"] = kunci
	}
	return s.callWith(t, http.MethodPost, "/v1/lapangan/harga", email, headers, body)
}

func entriBody(komoditas string, harga int, dicatat time.Time) map[string]any {
	return map[string]any{
		"komoditas_kode": komoditas, "pasar_id": "PSR-1171-01", "tipe": "KONSUMEN", "harga": harga,
		"dicatat_pada": dicatat.Format(time.RFC3339),
	}
}

func (s *suite) hitung(t *testing.T, query string, args ...any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, s.db.Raw(query, args...).Scan(&n).Error)
	return n
}

func TestKirimHarga_ReplayReturnsSameEntry(t *testing.T) {
	s := sharedSuite(t)
	body := entriBody("MYK", 21000, lampau())

	first := s.kirim(t, emailMahasiswa1, "kunci-replay", body)
	require.Equal(t, http.StatusCreated, first.Code, first.Body.String())
	created := data[hasilKirim](t, first)
	assert.Equal(t, "DIAJUKAN", created.Status)
	assert.Empty(t, first.Header().Get("Idempotent-Replayed"))

	again := s.kirim(t, emailMahasiswa1, "kunci-replay", body)
	require.Equal(t, http.StatusOK, again.Code, again.Body.String())
	assert.Equal(t, created, data[hasilKirim](t, again), "kiriman ulang mengembalikan hasil yang sama")
	assert.Equal(t, "true", again.Header().Get("Idempotent-Replayed"))

	assert.EqualValues(t, 1, s.hitung(t, "SELECT count(*) FROM entri_harga WHERE idempotency_key = 'kunci-replay'"))
	assert.EqualValues(t, 1, s.hitung(t, "SELECT count(*) FROM audit_log WHERE aksi = 'HARGA_INPUT' AND entitas_id = ?", created.ID), "audit tercatat sekali")
}

func TestKirimHarga_ReplayIgnoresLaterConflicts(t *testing.T) {
	s := sharedSuite(t)
	waktu := lampau().Add(-24 * time.Hour)
	first := s.kirim(t, emailMahasiswa1, "kunci-lama", entriBody("DAG-AYM", 38000, waktu))
	require.Equal(t, http.StatusCreated, first.Code, first.Body.String())

	// Petugas lain kemudian mempertahankan harga berbeda pada pasar dan komoditas yang sama.
	other := entriBody("DAG-AYM", 45000, waktu.Add(30*time.Minute))
	other["pertahankan"] = true
	require.Equal(t, http.StatusCreated, s.kirim(t, emailMahasiswa2, "kunci-lain", other).Code)

	again := s.kirim(t, emailMahasiswa1, "kunci-lama", entriBody("DAG-AYM", 38000, waktu))
	require.Equal(t, http.StatusOK, again.Code, "kiriman ulang tidak dinilai ulang terhadap entri baru: %s", again.Body.String())
	assert.Equal(t, data[hasilKirim](t, first), data[hasilKirim](t, again))
}

func TestKirimHarga_SameKeyFromDifferentPetugasAreIndependent(t *testing.T) {
	s := sharedSuite(t)
	waktu := lampau().Add(-48 * time.Hour)
	// Harga sama tidak bentrok, jadi keduanya tersimpan walau kuncinya sama.
	a := s.kirim(t, emailMahasiswa1, "kunci-bersama", entriBody("GLA", 17000, waktu))
	b := s.kirim(t, emailMahasiswa2, "kunci-bersama", entriBody("GLA", 17000, waktu))

	require.Equal(t, http.StatusCreated, a.Code, a.Body.String())
	require.Equal(t, http.StatusCreated, b.Code, b.Body.String())
	assert.NotEqual(t, data[hasilKirim](t, a).ID, data[hasilKirim](t, b).ID)
	assert.EqualValues(t, 2, s.hitung(t, "SELECT count(*) FROM entri_harga WHERE idempotency_key = 'kunci-bersama'"))
}

func TestKirimHarga_ConcurrentSameKeyStoresOnce(t *testing.T) {
	s := sharedSuite(t)
	body := entriBody("BRS-MED", 14500, lampau().Add(-72*time.Hour))

	const jumlah = 8
	s.login(t, emailMahasiswa1) // token di-cache dulu: peta token bukan untuk akses serentak
	hasil := make([]*httptest.ResponseRecorder, jumlah)
	var wg sync.WaitGroup
	for i := range jumlah {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hasil[i] = s.kirim(t, emailMahasiswa1, "kunci-serentak", body)
		}()
	}
	wg.Wait()

	var dibuat int
	for _, w := range hasil {
		assert.Contains(t, []int{http.StatusCreated, http.StatusOK}, w.Code, w.Body.String())
		assert.Equal(t, data[hasilKirim](t, hasil[0]).ID, data[hasilKirim](t, w).ID, "semua kiriman menerima entri yang sama")
		if w.Code == http.StatusCreated {
			dibuat++
		}
	}
	assert.Equal(t, 1, dibuat)
	assert.EqualValues(t, 1, s.hitung(t, "SELECT count(*) FROM entri_harga WHERE idempotency_key = 'kunci-serentak'"))
}

func TestKirimHarga_ConflictWithOtherPetugas(t *testing.T) {
	s := sharedSuite(t)
	waktu := lampau().Add(-96 * time.Hour)
	first := s.kirim(t, emailMahasiswa1, "konflik-1", entriBody("CBR-RWT", 60000, waktu))
	require.Equal(t, http.StatusCreated, first.Code, first.Body.String())
	pertama := data[hasilKirim](t, first)

	t.Run("harga berbeda di dalam jendela", func(t *testing.T) {
		w := s.kirim(t, emailMahasiswa2, "konflik-2", entriBody("CBR-RWT", 66000, waktu.Add(10*time.Minute)))

		require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
		versi := data[bentrokBody](t, w).VersiServer
		assert.Equal(t, pertama.ID, versi.ID)
		assert.Equal(t, 60000, versi.Harga)
		assert.Equal(t, "Aulia Rahman", versi.Petugas)
		assert.NotEmpty(t, versi.DicatatPada)
		assert.Zero(t, s.hitung(t, "SELECT count(*) FROM entri_harga WHERE idempotency_key = 'konflik-2'"), "kiriman yang bentrok tidak disimpan")
	})
	t.Run("harga sama tidak bentrok", func(t *testing.T) {
		assert.Equal(t, http.StatusCreated, s.kirim(t, emailMahasiswa2, "konflik-3", entriBody("CBR-RWT", 60000, waktu.Add(10*time.Minute))).Code)
	})
	t.Run("di luar jendela tidak bentrok", func(t *testing.T) {
		assert.Equal(t, http.StatusCreated, s.kirim(t, emailMahasiswa2, "konflik-4", entriBody("CBR-RWT", 66000, waktu.Add(4*time.Hour))).Code)
	})
	t.Run("petugas yang sama tidak bentrok dengan dirinya", func(t *testing.T) {
		sendiri := lampau().Add(-120 * time.Hour)
		require.Equal(t, http.StatusCreated, s.kirim(t, emailMahasiswa1, "konflik-5a", entriBody("TLR", 30000, sendiri)).Code)
		assert.Equal(t, http.StatusCreated, s.kirim(t, emailMahasiswa1, "konflik-5b", entriBody("TLR", 33000, sendiri.Add(10*time.Minute))).Code)
	})
	t.Run("pertahankan menyimpan versi sendiri", func(t *testing.T) {
		body := entriBody("CBR-RWT", 70000, waktu.Add(15*time.Minute))
		body["pertahankan"] = true
		assert.Equal(t, http.StatusCreated, s.kirim(t, emailMahasiswa2, "konflik-6", body).Code)
	})
	t.Run("entri ditolak tidak dianggap bentrok", func(t *testing.T) {
		pasarLain := entriBody("CBR-RWT", 52000, waktu.Add(-48*time.Hour))
		ditolak := s.kirim(t, emailMahasiswa1, "konflik-7", pasarLain)
		require.Equal(t, http.StatusCreated, ditolak.Code)
		require.NoError(t, s.db.Exec("UPDATE entri_harga SET status = 'DITOLAK', alasan_penolakan = 'uji' WHERE id = ?", data[hasilKirim](t, ditolak).ID).Error)

		assert.Equal(t, http.StatusCreated, s.kirim(t, emailMahasiswa2, "konflik-8", entriBody("CBR-RWT", 58000, waktu.Add(-48*time.Hour+time.Minute))).Code)
	})
}

func TestKirimHarga_Validation(t *testing.T) {
	s := sharedSuite(t)
	kasus := map[string]map[string]any{
		"komoditas tidak ada": {"komoditas_kode": "XXX", "pasar_id": "PSR-1171-01", "tipe": "KONSUMEN", "harga": 1000},
		"pasar tidak ada":     {"komoditas_kode": "MYK", "pasar_id": "PSR-XXX", "tipe": "KONSUMEN", "harga": 1000},
		"harga nol":           {"komoditas_kode": "MYK", "pasar_id": "PSR-1171-01", "tipe": "KONSUMEN", "harga": 0},
		"tipe salah":          {"komoditas_kode": "MYK", "pasar_id": "PSR-1171-01", "tipe": "ECERAN", "harga": 1000},
		"waktu masa depan":    {"komoditas_kode": "MYK", "pasar_id": "PSR-1171-01", "tipe": "KONSUMEN", "harga": 1000, "dicatat_pada": time.Now().Add(2 * time.Hour).Format(time.RFC3339)},
		"lat di luar rentang": {"komoditas_kode": "MYK", "pasar_id": "PSR-1171-01", "tipe": "KONSUMEN", "harga": 1000, "lat": 91.0, "lng": 95.0},
		"lat tanpa lng":       {"komoditas_kode": "MYK", "pasar_id": "PSR-1171-01", "tipe": "KONSUMEN", "harga": 1000, "lat": 5.5},
	}
	for nama, body := range kasus {
		assert.Equal(t, http.StatusBadRequest, s.kirim(t, emailMahasiswa1, "validasi-"+nama, body).Code, nama)
	}
	assert.Zero(t, s.hitung(t, "SELECT count(*) FROM entri_harga WHERE idempotency_key LIKE 'validasi-%'"))
}

func TestKirimHarga_RequiresKeyAndPermission(t *testing.T) {
	s := sharedSuite(t)
	body := entriBody("MYK", 21000, lampau())

	assert.Equal(t, http.StatusBadRequest, s.kirim(t, emailMahasiswa1, "", body).Code, "tanpa Idempotency-Key")
	assert.Equal(t, http.StatusForbidden, s.kirim(t, emailTanpaIzin, "k", body).Code, "tanpa harga.input")
	assert.Equal(t, http.StatusUnauthorized, s.kirim(t, "", "k", body).Code)
}

func TestKirimHarga_AdvancesTaskProgressOncePerCommodity(t *testing.T) {
	s := sharedSuite(t)
	waktu := time.Now().Add(-time.Minute)
	require.NoError(t, s.db.Exec(`
		INSERT INTO tugas_lapangan (kode, judul, pasar_id, petugas_id, komoditas, tipe, tenggat, status, target, dibuat_pada)
		VALUES ('TGS-uji-progres', 'Uji progres', 'PSR-1174-01', garda_uuid_stabil('pengguna', 'mahasiswa_9'), '{TMT,KTG}', 'KONSUMEN', ?, 'BELUM', 2, ?)`,
		time.Now().AddDate(-10, 0, 0), time.Now().AddDate(-11, 0, 0)).Error)
	kirim := func(kunci, komoditas string) *httptest.ResponseRecorder {
		body := entriBody(komoditas, 15000, waktu)
		body["pasar_id"] = "PSR-1174-01"
		return s.kirim(t, "mahasiswa.9@gardainflasi.id", kunci, body)
	}
	terisi := func() int {
		for _, item := range data[[]tugas](t, s.call(t, http.MethodGet, "/v1/lapangan/tugas/saya", "mahasiswa.9@gardainflasi.id", nil)) {
			if item.Kode != nil && *item.Kode == "TGS-uji-progres" {
				return item.Terisi
			}
		}
		require.Fail(t, "tugas uji tidak ditemukan")
		return -1
	}

	rincian := func() map[string]bool {
		w := s.call(t, http.MethodGet, "/v1/lapangan/tugas/TGS-uji-progres", "mahasiswa.9@gardainflasi.id", nil)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		hasil := map[string]bool{}
		for _, r := range data[tugasDetail](t, w).Rincian {
			assert.NotEmpty(t, r.Nama, r.Kode)
			hasil[r.Kode] = r.Terisi
		}
		return hasil
	}
	assert.Equal(t, map[string]bool{"TMT": false, "KTG": false}, rincian(), "belum ada entri tertaut")

	first := kirim("progres-1", "TMT")
	require.Equal(t, http.StatusCreated, first.Code, first.Body.String())
	assert.Equal(t, 1, terisi())
	assert.Equal(t, map[string]bool{"TMT": true, "KTG": false}, rincian(), "rincian mengikuti entri tertaut")
	assert.EqualValues(t, 1, s.hitung(t, "SELECT count(*) FROM tugas_entri WHERE tugas_id = (SELECT id FROM tugas_lapangan WHERE kode = 'TGS-uji-progres') AND entri_id = ?", data[hasilKirim](t, first).ID))

	require.Equal(t, http.StatusCreated, kirim("progres-2", "TMT").Code)
	assert.Equal(t, 1, terisi(), "komoditas yang sama tidak menambah progres lagi")

	require.Equal(t, http.StatusCreated, kirim("progres-3", "KTG").Code)
	assert.Equal(t, 2, terisi())

	require.Equal(t, http.StatusCreated, kirim("progres-4", "TMT").Code)
	assert.Equal(t, 2, terisi(), "progres tidak melebihi target")

	// Entri seed petugas ini hari ini berharga lain, jadi petugas lain harus mempertahankan versinya.
	other := entriBody("TMT", 15000, waktu)
	other["pasar_id"], other["pertahankan"] = "PSR-1174-01", true
	assert.Equal(t, http.StatusCreated, s.kirim(t, emailMahasiswa1, "progres-5", other).Code)
	assert.Equal(t, 2, terisi(), "entri petugas lain tidak mengubah tugas ini")
}

func TestAcuan(t *testing.T) {
	s := sharedSuite(t)

	w := s.call(t, http.MethodGet, "/v1/lapangan/acuan", emailMahasiswa1, nil)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	acuan := data[struct {
		Versi     string  `json:"versi"`
		Toleransi float64 `json:"toleransi"`
		Komoditas []struct {
			Kode       string `json:"kode"`
			HargaAcuan int    `json:"harga_acuan"`
		} `json:"komoditas"`
		Pasar []struct {
			ID          string `json:"id"`
			RadiusMeter int    `json:"radius_meter"`
		} `json:"pasar"`
		HargaAcuan []struct {
			KomoditasKode string `json:"komoditas_kode"`
			KabupatenKode string `json:"kabupaten_kode"`
			Tipe          string `json:"tipe"`
			Median        int    `json:"median"`
			Bawah         int    `json:"bawah"`
			Atas          int    `json:"atas"`
			Sampel        int    `json:"sampel"`
		} `json:"harga_acuan"`
	}](t, w)
	assert.NotEmpty(t, acuan.Versi)
	assert.InDelta(t, 0.25, acuan.Toleransi, 0.0001)
	assert.GreaterOrEqual(t, len(acuan.Komoditas), 10)
	assert.GreaterOrEqual(t, len(acuan.Pasar), 20)
	require.NotEmpty(t, acuan.HargaAcuan)
	for _, h := range acuan.HargaAcuan {
		assert.Less(t, h.Bawah, h.Median, "%s/%s/%s", h.KomoditasKode, h.KabupatenKode, h.Tipe)
		assert.Greater(t, h.Atas, h.Median)
		assert.GreaterOrEqual(t, h.Sampel, 3)
	}
	assert.Equal(t, `"`+acuan.Versi+`"`, w.Header().Get("ETag"))

	t.Run("versi sama dijawab 304", func(t *testing.T) {
		again := s.callWith(t, http.MethodGet, "/v1/lapangan/acuan", emailMahasiswa1, map[string]string{"If-None-Match": w.Header().Get("ETag")}, nil)
		assert.Equal(t, http.StatusNotModified, again.Code)
		assert.Empty(t, again.Body.String())
	})
	t.Run("versi lain dijawab penuh", func(t *testing.T) {
		again := s.callWith(t, http.MethodGet, "/v1/lapangan/acuan", emailMahasiswa1, map[string]string{"If-None-Match": `"usang"`}, nil)
		assert.Equal(t, http.StatusOK, again.Code)
	})
	t.Run("izin", func(t *testing.T) {
		assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, "/v1/lapangan/acuan", emailTanpaIzin, nil).Code)
		assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodGet, "/v1/lapangan/acuan", "", nil).Code)
	})
}
