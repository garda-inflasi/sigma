package integration

// Alur usulan perubahan master data (04-B): ajukan → setujui/tolak/batalkan, pemisahan tugas, usulan kedaluwarsa,
// riwayat versi komoditas, pasar dengan kecamatan, dan aksi massal yang terhubung ke antrean.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type pelaku struct {
	ID   string  `json:"id"`
	Nama *string `json:"nama"`
}

type usulan struct {
	ID              string         `json:"id"`
	Entitas         string         `json:"entitas"`
	EntitasID       string         `json:"entitas_id"`
	EntitasNama     *string        `json:"entitas_nama"`
	Jenis           string         `json:"jenis"`
	Versi           int            `json:"versi"`
	Status          string         `json:"status"`
	Alasan          *string        `json:"alasan"`
	AlasanPenolakan *string        `json:"alasan_penolakan"`
	Perubahan       map[string]any `json:"perubahan"`
	Sebelum         map[string]any `json:"sebelum"`
	Diff            []struct {
		Bidang  string          `json:"bidang"`
		Sebelum json.RawMessage `json:"sebelum"`
		Sesudah json.RawMessage `json:"sesudah"`
	} `json:"diff"`
	DiajukanOleh  *pelaku `json:"diajukan_oleh"`
	DisetujuiOleh *pelaku `json:"disetujui_oleh"`
	DitolakOleh   *pelaku `json:"ditolak_oleh"`
}

func satu[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var res struct {
		Code string `json:"code"`
		Data T      `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res), w.Body.String())
	return res.Data
}

func kodeGalat(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var res struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res), w.Body.String())
	return res.Code
}

func (s *suite) idPengguna(t *testing.T, email string) string {
	t.Helper()
	var id string
	require.NoError(t, s.db.Raw(`SELECT id::text FROM pengguna WHERE email = ?`, email).Scan(&id).Error)
	require.NotEmpty(t, id, email)
	return id
}

func (s *suite) ajukan(t *testing.T, email string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	return s.call(t, http.MethodPost, "/v1/master/perubahan", email, body)
}

func komoditasBaru(kode string) map[string]any {
	return map[string]any{
		"entitas": "komoditas", "jenis": "TAMBAH", "entitas_id": kode, "alasan": "Komoditas baru untuk uji",
		"perubahan": map[string]any{
			"nama": "Uji " + kode, "kategori": "Hortikultura", "satuan": "kg", "harga_acuan": 10000,
			"shelf_life_hari": 30, "pilot": false,
		},
	}
}

func TestPerubahan_TambahUbahTolakSetujuiBatalkan(t *testing.T) {
	s := sharedSuite(t)
	const kode = "UJI-PRB-1"

	// TAMBAH: pengaju master.ajukan, penyetuju master.setujui.
	w := s.ajukan(t, emailManajer, komoditasBaru(kode))
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	u := satu[usulan](t, w)
	assert.Equal(t, "MENUNGGU", u.Status)
	assert.Equal(t, "TAMBAH", u.Jenis)
	assert.Equal(t, 1, u.Versi)
	require.NotNil(t, u.EntitasNama)
	assert.Equal(t, "Uji "+kode, *u.EntitasNama, "nama dari usulan karena data belum ada")
	assert.Len(t, u.Diff, 6)
	assert.Equal(t, "nama", u.Diff[0].Bidang, "diff urut menurut urutan bidang entitas")
	assert.JSONEq(t, "null", string(u.Diff[0].Sebelum))

	antrean := s.call(t, http.MethodGet, "/v1/master/perubahan?status=MENUNGGU&entitas=komoditas", emailLihat, nil)
	require.Equal(t, http.StatusOK, antrean.Code)
	total, err := strconv.Atoi(antrean.Header().Get("X-Total-Count"))
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, 1)

	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPost, "/v1/master/perubahan/"+u.ID+"/setujui", emailManajer, nil).Code,
		"pengaju tanpa master.setujui")
	w = s.call(t, http.MethodPost, "/v1/master/perubahan/"+u.ID+"/setujui", emailSetujui, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	u = satu[usulan](t, w)
	assert.Equal(t, "DISETUJUI", u.Status)
	require.NotNil(t, u.DisetujuiOleh)
	assert.Equal(t, s.idPengguna(t, emailSetujui), u.DisetujuiOleh.ID)

	var k struct {
		Versi      int
		Status     string
		HargaAcuan int
	}
	require.NoError(t, s.db.Raw(`SELECT versi, status::text AS status, harga_acuan FROM komoditas WHERE kode = ?`, kode).Scan(&k).Error)
	assert.Equal(t, 1, k.Versi)
	assert.Equal(t, "AKTIF", k.Status)

	// UBAH: bidang yang tidak berubah dibuang; komoditas ditandai menunggu.
	w = s.ajukan(t, emailManajer, map[string]any{
		"entitas": "komoditas", "jenis": "UBAH", "entitas_id": kode, "alasan": "Harga acuan naik",
		"perubahan": map[string]any{"harga_acuan": 12500, "nama": "Uji " + kode},
	})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	u = satu[usulan](t, w)
	assert.Equal(t, 2, u.Versi)
	require.Len(t, u.Diff, 1)
	assert.Equal(t, "harga_acuan", u.Diff[0].Bidang)
	assert.JSONEq(t, "10000", string(u.Diff[0].Sebelum))
	assert.JSONEq(t, "12500", string(u.Diff[0].Sesudah))
	require.NoError(t, s.db.Raw(`SELECT versi, status::text AS status, harga_acuan FROM komoditas WHERE kode = ?`, kode).Scan(&k).Error)
	assert.Equal(t, "MENUNGGU_PERSETUJUAN", k.Status)

	// Detail komoditas memuat usulan yang menunggu.
	w = s.call(t, http.MethodGet, "/v1/master/komoditas/"+kode, emailLihat, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	detail := satu[struct {
		PerubahanMenunggu *usulan `json:"perubahan_menunggu"`
	}](t, w)
	require.NotNil(t, detail.PerubahanMenunggu)
	assert.Equal(t, u.ID, detail.PerubahanMenunggu.ID)

	w = s.ajukan(t, emailAjukan, map[string]any{
		"entitas": "komoditas", "jenis": "UBAH", "entitas_id": kode, "alasan": "Usulan kedua",
		"perubahan": map[string]any{"shelf_life_hari": 40},
	})
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Equal(t, "USULAN_MENUNGGU", kodeGalat(t, w))

	// Tolak wajib beralasan; komoditas kembali aktif tanpa perubahan isi.
	assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodPost, "/v1/master/perubahan/"+u.ID+"/tolak", emailSetujui, map[string]any{}).Code)
	w = s.call(t, http.MethodPost, "/v1/master/perubahan/"+u.ID+"/tolak", emailSetujui, map[string]any{"alasan": "Belum ada dasar data"})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	u = satu[usulan](t, w)
	assert.Equal(t, "DITOLAK", u.Status)
	require.NotNil(t, u.AlasanPenolakan)
	assert.Equal(t, "Belum ada dasar data", *u.AlasanPenolakan)
	require.NoError(t, s.db.Raw(`SELECT versi, status::text AS status, harga_acuan FROM komoditas WHERE kode = ?`, kode).Scan(&k).Error)
	assert.Equal(t, "AKTIF", k.Status)
	assert.Equal(t, 10000, k.HargaAcuan)
	w = s.call(t, http.MethodGet, "/v1/master/komoditas/"+kode, emailLihat, nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"perubahan_menunggu":null`, "usulan yang sudah diputuskan tidak lagi tampil")
	assert.Equal(t, http.StatusConflict, s.call(t, http.MethodPost, "/v1/master/perubahan/"+u.ID+"/setujui", emailSetujui, nil).Code,
		"usulan yang sudah diputuskan")

	// UBAH lalu setujui: isi diterapkan, versi naik, riwayat versi menautkan usulannya.
	w = s.ajukan(t, emailManajer, map[string]any{
		"entitas": "komoditas", "jenis": "UBAH", "entitas_id": kode, "alasan": "Harga acuan naik (data BPS)",
		"perubahan": map[string]any{"harga_acuan": 12500},
	})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	u = satu[usulan](t, w)
	w = s.call(t, http.MethodPost, "/v1/master/perubahan/"+u.ID+"/setujui", emailFMIPA, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoError(t, s.db.Raw(`SELECT versi, status::text AS status, harga_acuan FROM komoditas WHERE kode = ?`, kode).Scan(&k).Error)
	assert.Equal(t, 2, k.Versi)
	assert.Equal(t, 12500, k.HargaAcuan)
	assert.Equal(t, "AKTIF", k.Status)
	var versiTertaut int64
	require.NoError(t, s.db.Raw(`SELECT count(*) FROM komoditas_versi WHERE komoditas_kode = ? AND versi = 2 AND perubahan_id = ?`, kode, u.ID).
		Scan(&versiTertaut).Error)
	assert.EqualValues(t, 1, versiTertaut)
	var audit int64
	require.NoError(t, s.db.Raw(`SELECT count(*) FROM audit_log WHERE entitas = 'komoditas' AND entitas_id = ?
		AND aksi IN ('AJUKAN_PERUBAHAN_MASTER', 'SETUJUI_PERUBAHAN_MASTER', 'TOLAK_PERUBAHAN_MASTER')`, kode).Scan(&audit).Error)
	assert.EqualValues(t, 6, audit, "tiga pengajuan, dua persetujuan, satu penolakan")

	// Usulan yang tidak mengubah apa pun ditolak; batalkan hanya oleh pengaju.
	w = s.ajukan(t, emailManajer, map[string]any{
		"entitas": "komoditas", "jenis": "UBAH", "entitas_id": kode, "alasan": "Sama saja",
		"perubahan": map[string]any{"harga_acuan": 12500},
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "TANPA_PERUBAHAN", kodeGalat(t, w))
	w = s.ajukan(t, emailManajer, map[string]any{
		"entitas": "komoditas", "jenis": "UBAH", "entitas_id": kode, "alasan": "Umur simpan",
		"perubahan": map[string]any{"shelf_life_hari": 45},
	})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	u = satu[usulan](t, w)
	w = s.call(t, http.MethodPost, "/v1/master/perubahan/"+u.ID+"/batalkan", emailAjukan, nil)
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Equal(t, "BUKAN_PENGAJU", kodeGalat(t, w))
	w = s.call(t, http.MethodPost, "/v1/master/perubahan/"+u.ID+"/batalkan", emailManajer, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	u = satu[usulan](t, w)
	assert.Equal(t, "DITOLAK", u.Status)
	require.NotNil(t, u.AlasanPenolakan)
	assert.Equal(t, "Dibatalkan pengaju", *u.AlasanPenolakan)

	// Detail dan galat rute.
	w = s.call(t, http.MethodGet, "/v1/master/perubahan/"+u.ID, emailLihat, nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, u.ID, satu[usulan](t, w).ID)
	for _, id := range []string{"abc", "0", "999999999"} {
		assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodGet, "/v1/master/perubahan/"+id, emailLihat, nil).Code, id)
	}
}

func TestPerubahan_ValidasiDanIzin(t *testing.T) {
	s := sharedSuite(t)
	kasus := []struct {
		nama  string
		body  map[string]any
		kode  int
		galat string
	}{
		{"bidang tak dikenal", map[string]any{"entitas": "komoditas", "jenis": "UBAH", "entitas_id": "BWM", "alasan": "uji", "perubahan": map[string]any{"status": "AKTIF"}}, http.StatusBadRequest, "400"},
		{"tipe salah", map[string]any{"entitas": "komoditas", "jenis": "UBAH", "entitas_id": "BWM", "alasan": "uji", "perubahan": map[string]any{"harga_acuan": "mahal"}}, http.StatusBadRequest, "400"},
		{"pecahan untuk bilangan bulat", map[string]any{"entitas": "komoditas", "jenis": "UBAH", "entitas_id": "BWM", "alasan": "uji", "perubahan": map[string]any{"harga_acuan": 10.5}}, http.StatusBadRequest, "400"},
		{"satuan tak terdaftar", map[string]any{"entitas": "komoditas", "jenis": "UBAH", "entitas_id": "BWM", "alasan": "uji", "perubahan": map[string]any{"satuan": "karung"}}, http.StatusBadRequest, "400"},
		{"tanpa alasan", map[string]any{"entitas": "komoditas", "jenis": "UBAH", "entitas_id": "BWM", "perubahan": map[string]any{"harga_acuan": 1}}, http.StatusBadRequest, "400"},
		{"entitas tak didukung", map[string]any{"entitas": "kabupaten", "jenis": "UBAH", "entitas_id": "1171", "alasan": "uji", "perubahan": map[string]any{"nama": "x"}}, http.StatusBadRequest, "400"},
		{"data tidak ada", map[string]any{"entitas": "komoditas", "jenis": "UBAH", "entitas_id": "TIDAK-ADA", "alasan": "uji", "perubahan": map[string]any{"harga_acuan": 1}}, http.StatusNotFound, "404"},
		{"kode sudah ada", komoditasBaru("BWM"), http.StatusConflict, "KODE_SUDAH_ADA"},
		{"kode baru salah bentuk", komoditasBaru("uji kecil"), http.StatusBadRequest, "400"},
	}
	for _, k := range kasus {
		t.Run(k.nama, func(t *testing.T) {
			w := s.ajukan(t, emailManajer, k.body)
			assert.Equal(t, k.kode, w.Code, w.Body.String())
			assert.Equal(t, k.galat, kodeGalat(t, w))
		})
	}

	assert.Equal(t, http.StatusForbidden, s.ajukan(t, emailLihat, komoditasBaru("UJI-IZIN")).Code, "peran lihat-saja")
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, "/v1/master/perubahan", emailTanpaIzn, nil).Code)
	assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodGet, "/v1/master/perubahan", "", nil).Code)
	assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodGet, "/v1/master/perubahan?status=APA", emailLihat, nil).Code)
}

func TestPerubahan_PemisahanTugasDanKedaluwarsa(t *testing.T) {
	s := sharedSuite(t)
	const kode = "UJI-PRB-2"
	w := s.ajukan(t, emailManajer, komoditasBaru(kode))
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	require.Equal(t, http.StatusOK, s.call(t, http.MethodPost, "/v1/master/perubahan/"+satu[usulan](t, w).ID+"/setujui", emailSetujui, nil).Code)

	// Usulan milik penyetuju sendiri (mis. peran diberi dua izin): tidak boleh diputuskan sendiri.
	var id string
	require.NoError(t, s.db.Raw(`
		INSERT INTO perubahan_master (entitas, entitas_id, jenis, versi, perubahan, sebelum, diajukan_oleh_id, alasan)
		VALUES ('komoditas', ?, 'UBAH', 2, '{"harga_acuan": 11000}', '{"harga_acuan": 10000}', ?, 'usulan sendiri')
		RETURNING id::text`, kode, s.idPengguna(t, emailSetujui)).Scan(&id).Error)
	for _, aksi := range []string{"setujui", "tolak"} {
		w = s.call(t, http.MethodPost, "/v1/master/perubahan/"+id+"/"+aksi, emailSetujui, map[string]any{"alasan": "uji pemisahan"})
		assert.Equal(t, http.StatusForbidden, w.Code, aksi)
		assert.Equal(t, "PEMISAHAN_TUGAS", kodeGalat(t, w), aksi)
	}

	// Data berubah sejak diajukan (versi maju di luar usulan): persetujuan ditolak sebagai kedaluwarsa.
	require.NoError(t, s.db.Exec(`UPDATE komoditas SET versi = versi + 1 WHERE kode = ?`, kode).Error)
	w = s.call(t, http.MethodPost, "/v1/master/perubahan/"+id+"/setujui", emailFMIPA, nil)
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Equal(t, "USULAN_KEDALUWARSA", kodeGalat(t, w))
	var harga int
	require.NoError(t, s.db.Raw(`SELECT harga_acuan FROM komoditas WHERE kode = ?`, kode).Scan(&harga).Error)
	assert.Equal(t, 10000, harga, "usulan kedaluwarsa tidak diterapkan")
	require.Equal(t, http.StatusOK, s.call(t, http.MethodPost, "/v1/master/perubahan/"+id+"/tolak", emailFMIPA, map[string]any{"alasan": "kedaluwarsa"}).Code)
}

func TestPerubahan_PasarDenganKecamatan(t *testing.T) {
	s := sharedSuite(t)
	require.NoError(t, s.db.Exec(`INSERT INTO kecamatan (kode, nama, kabupaten_kode) VALUES
		('1171010', 'Meuraxa', '1171'), ('1171020', 'Baiturrahman', '1171'), ('1172010', 'Sukajaya', '1172')
		ON CONFLICT (kode) DO NOTHING`).Error)
	const pasar = "PSR-1171-01"

	w := s.ajukan(t, emailManajer, map[string]any{
		"entitas": "pasar", "jenis": "UBAH", "entitas_id": pasar, "alasan": "Kecamatan salah",
		"perubahan": map[string]any{"kecamatan_kode": "1172010"},
	})
	assert.Equal(t, http.StatusBadRequest, w.Code, "kecamatan di kabupaten lain")
	w = s.ajukan(t, emailManajer, map[string]any{
		"entitas": "pasar", "jenis": "UBAH", "entitas_id": pasar, "alasan": "Lengkapi kecamatan dan radius",
		"perubahan": map[string]any{"kecamatan_kode": "1171020", "radius_meter": 550, "kabupaten_kode": "1172"},
	})
	assert.Equal(t, http.StatusBadRequest, w.Code, "kabupaten tidak bisa diubah lewat UBAH")

	w = s.ajukan(t, emailManajer, map[string]any{
		"entitas": "pasar", "jenis": "UBAH", "entitas_id": pasar, "alasan": "Lengkapi kecamatan dan radius",
		"perubahan": map[string]any{"kecamatan_kode": "1171020", "radius_meter": 550},
	})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	u := satu[usulan](t, w)
	require.Equal(t, http.StatusOK, s.call(t, http.MethodPost, "/v1/master/perubahan/"+u.ID+"/setujui", emailSetujui, nil).Code)
	var p struct {
		KecamatanKode *string
		RadiusMeter   int
		Versi         int
	}
	require.NoError(t, s.db.Raw(`SELECT kecamatan_kode, radius_meter, versi FROM pasar WHERE id = ?`, pasar).Scan(&p).Error)
	require.NotNil(t, p.KecamatanKode)
	assert.Equal(t, "1171020", *p.KecamatanKode)
	assert.Equal(t, 550, p.RadiusMeter)
	assert.Equal(t, 2, p.Versi)

	// TAMBAH pasar: radius dan status aktif bawaan.
	w = s.ajukan(t, emailManajer, map[string]any{
		"entitas": "pasar", "jenis": "TAMBAH", "entitas_id": "PSR-1171-99", "alasan": "Pasar baru dipantau",
		"perubahan": map[string]any{"nama": "Pasar Uji", "kabupaten_kode": "1171", "lat": 5.55, "lng": 95.32, "kecamatan_kode": "1171010"},
	})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	u = satu[usulan](t, w)
	require.Equal(t, http.StatusOK, s.call(t, http.MethodPost, "/v1/master/perubahan/"+u.ID+"/setujui", emailSetujui, nil).Code)
	var baru struct {
		Nama        string
		RadiusMeter int
		Aktif       bool
		Lat         float64
	}
	require.NoError(t, s.db.Raw(`SELECT nama, radius_meter, aktif, lat FROM pasar WHERE id = 'PSR-1171-99'`).Scan(&baru).Error)
	assert.Equal(t, "Pasar Uji", baru.Nama)
	assert.Equal(t, 400, baru.RadiusMeter)
	assert.True(t, baru.Aktif)
	assert.InDelta(t, 5.55, baru.Lat, 1e-9)
}

func TestBulk_TerhubungAntreanUsulan(t *testing.T) {
	s := sharedSuite(t)
	const kode = "UJI-PRB-3"
	w := s.ajukan(t, emailManajer, komoditasBaru(kode))
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	require.Equal(t, http.StatusOK, s.call(t, http.MethodPost, "/v1/master/perubahan/"+satu[usulan](t, w).ID+"/setujui", emailSetujui, nil).Code)

	bulk := func(email, aksi string) []struct {
		Kode     string `json:"kode"`
		Berhasil bool   `json:"berhasil"`
		Pesan    string `json:"pesan"`
	} {
		t.Helper()
		w := s.call(t, http.MethodPost, "/v1/master/komoditas/bulk", email, map[string]any{"action": aksi, "kode": []string{kode}})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var res struct {
			Data struct {
				Rincian []struct {
					Kode     string `json:"kode"`
					Berhasil bool   `json:"berhasil"`
					Pesan    string `json:"pesan"`
				} `json:"rincian"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
		return res.Data.Rincian
	}
	statusUsulan := func() []string {
		var status []string
		require.NoError(t, s.db.Raw(`SELECT status::text FROM perubahan_master WHERE entitas = 'komoditas' AND entitas_id = ? AND jenis = 'TINJAU' ORDER BY id`, kode).
			Scan(&status).Error)
		return status
	}

	// Ajukan massal → usulan TINJAU; setujui massal → usulan disetujui, versi naik.
	assert.True(t, bulk(emailManajer, "ajukan_perubahan")[0].Berhasil)
	assert.Equal(t, []string{"MENUNGGU"}, statusUsulan())
	assert.True(t, bulk(emailSetujui, "setujui")[0].Berhasil)
	assert.Equal(t, []string{"DISETUJUI"}, statusUsulan())
	var versi int
	require.NoError(t, s.db.Raw(`SELECT versi FROM komoditas WHERE kode = ?`, kode).Scan(&versi).Error)
	assert.Equal(t, 2, versi)

	// Usulan milik penyetuju sendiri: item gagal dengan pesan pemisahan tugas.
	require.NoError(t, s.db.Exec(`
		INSERT INTO perubahan_master (entitas, entitas_id, jenis, versi, perubahan, sebelum, diajukan_oleh_id, alasan)
		VALUES ('komoditas', ?, 'TINJAU', 3, '{}', '{}', ?, 'usulan sendiri')`, kode, s.idPengguna(t, emailSetujui)).Error)
	require.NoError(t, s.db.Exec(`UPDATE komoditas SET status = 'MENUNGGU_PERSETUJUAN' WHERE kode = ?`, kode).Error)
	rincian := bulk(emailSetujui, "setujui")
	assert.False(t, rincian[0].Berhasil)
	assert.Contains(t, rincian[0].Pesan, "penyetuju lain")

	// Arsipkan menutup usulan yang menunggu.
	assert.True(t, bulk(emailManajer, "arsipkan")[0].Berhasil)
	assert.Equal(t, []string{"DISETUJUI", "DITOLAK"}, statusUsulan())
	var alasan string
	require.NoError(t, s.db.Raw(`SELECT alasan_penolakan FROM perubahan_master WHERE entitas_id = ? AND status = 'DITOLAK'`, kode).Scan(&alasan).Error)
	assert.Equal(t, "Komoditas diarsipkan", alasan)
	w = s.ajukan(t, emailManajer, map[string]any{
		"entitas": "komoditas", "jenis": "UBAH", "entitas_id": kode, "alasan": "Setelah arsip",
		"perubahan": map[string]any{"harga_acuan": 1},
	})
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Equal(t, "STATUS_TIDAK_SESUAI", kodeGalat(t, w), fmt.Sprintf("komoditas arsip: %s", w.Body.String()))
}
