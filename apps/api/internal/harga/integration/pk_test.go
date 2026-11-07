package integration

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Kunci primer entri_harga: BIGINT identity yang diberi basis data, ID lama di kode (alias tautan lama).

type entriBerkode struct {
	ID   string  `json:"id"`
	Kode *string `json:"kode"`
}

func TestPK_IDBigintDalamStringDanKodeLamaPadaDataSeed(t *testing.T) {
	s := sharedSuite(t)

	w := s.call(t, http.MethodGet, "/v1/harga?limit=200", emailKoordinator, nil)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"id":"`, "ID BIGINT ditulis sebagai string JSON, bukan angka")
	rows := data[[]entriBerkode](t, w)
	require.NotEmpty(t, rows)
	for _, e := range rows {
		n, err := strconv.ParseInt(e.ID, 10, 64)
		require.NoError(t, err, "id harus string angka: %q", e.ID)
		assert.Positive(t, n)
	}
}

func TestPK_EntriBaruDiberiIDOlehBasisDataTanpaKode(t *testing.T) {
	s := sharedSuite(t)

	pertama, kedua := s.buat(t, emailMahasiswa), s.buat(t, emailMahasiswa)

	a, err := strconv.ParseInt(pertama, 10, 64)
	require.NoError(t, err, "id respons POST harus string angka: %q", pertama)
	b, err := strconv.ParseInt(kedua, 10, 64)
	require.NoError(t, err)
	assert.Greater(t, b, a, "identity menaik")

	detail := data[entriBerkode](t, s.call(t, http.MethodGet, "/v1/harga/"+pertama, emailKoordinator, nil))
	assert.Equal(t, pertama, detail.ID)
	assert.Nil(t, detail.Kode, "entri baru tidak punya kode tampilan")

	var kode *string
	require.NoError(t, s.db.Raw("SELECT kode FROM entri_harga WHERE id = ?", a).Scan(&kode).Error)
	assert.Nil(t, kode)
}

func TestPK_TautanLamaBerkodeTetapBerlaku(t *testing.T) {
	s := sharedSuite(t)
	const kode = "HRG-D05-00-02-K"
	var id string
	require.NoError(t, s.db.Raw("SELECT id::text FROM entri_harga WHERE kode = ?", kode).Scan(&id).Error)
	require.NotEmpty(t, id, "baris seed %s ada", kode)

	t.Run("detail", func(t *testing.T) {
		w := s.call(t, http.MethodGet, "/v1/harga/"+kode, emailKoordinator, nil)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		got := data[entriBerkode](t, w)
		assert.Equal(t, id, got.ID, "kode lama menunjuk baris yang sama dengan ID baru")
		require.NotNil(t, got.Kode)
		assert.Equal(t, kode, *got.Kode)
	})
	t.Run("riwayat", func(t *testing.T) {
		lewatKode := s.call(t, http.MethodGet, "/v1/harga/"+kode+"/riwayat", emailKoordinator, nil)
		require.Equal(t, http.StatusOK, lewatKode.Code, lewatKode.Body.String())
		lewatID := s.call(t, http.MethodGet, "/v1/harga/"+id+"/riwayat", emailKoordinator, nil)
		require.Equal(t, http.StatusOK, lewatID.Code)
		assert.JSONEq(t, lewatID.Body.String(), lewatKode.Body.String())
		assert.NotEmpty(t, data[[]riwayat](t, lewatKode))
	})
	t.Run("kode berisi UUID dari aplikasi lama", func(t *testing.T) {
		const uuidLama = "3f2b8c1e-9d4a-4e6f-8a21-5c7d9e0b1a34"
		require.NoError(t, s.db.Exec("UPDATE entri_harga SET kode = ? WHERE id = ?", uuidLama, id).Error)
		t.Cleanup(func() { s.db.Exec("UPDATE entri_harga SET kode = ? WHERE id = ?", kode, id) })
		w := s.call(t, http.MethodGet, "/v1/harga/"+uuidLama, emailKoordinator, nil)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, id, data[entriBerkode](t, w).ID)
	})
	t.Run("izin tetap berlaku lewat alias", func(t *testing.T) {
		assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, "/v1/harga/"+kode, emailMahasiswa, nil).Code)
		assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodGet, "/v1/harga/"+kode, "", nil).Code)
	})
	t.Run("petugas hanya membuka riwayat entrinya sendiri", func(t *testing.T) {
		var milikLain string
		require.NoError(t, s.db.Raw(`
			SELECT kode FROM entri_harga WHERE kode LIKE 'HRG-D%' AND petugas_id <> garda_uuid_stabil('pengguna', 'mahasiswa_1') LIMIT 1`).Scan(&milikLain).Error)
		require.NotEmpty(t, milikLain)
		assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodGet, "/v1/harga/"+milikLain+"/riwayat", emailMahasiswa, nil).Code)
	})
}

func TestPK_IDSalahBentukAtauTidakAdaDijawab404(t *testing.T) {
	s := sharedSuite(t)

	salah := map[string]string{
		"kode tidak dikenal":   "HRG-TIDAK-ADA",
		"nol":                  "0",
		"negatif":              "-1",
		"berawalan nol":        "007",
		"pecahan":              "1.5",
		"notasi ilmiah":        "1e3",
		"di luar rentang int8": "9223372036854775808",
		"spasi":                "%20",
		"terlalu panjang":      strings.Repeat("A", 65),
		"bentuk sah, tak ada":  "999999999",
	}
	for nama, id := range salah {
		for _, path := range []string{"/v1/harga/" + id, "/v1/harga/" + id + "/riwayat"} {
			w := s.call(t, http.MethodGet, path, emailKoordinator, nil)
			assert.Equal(t, http.StatusNotFound, w.Code, "%s (%s): %s", nama, path, w.Body.String())
		}
	}
	// Panjang tepat 64 dibaca sebagai kode (tidak ada) tanpa galat server.
	assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodGet, "/v1/harga/"+strings.Repeat("A", 64), emailKoordinator, nil).Code)
}

func TestPK_BulkMenolakIDSalahBentukDenganSeluruhPermintaan(t *testing.T) {
	s := sharedSuite(t)
	id := s.buat(t, emailMahasiswa)

	// Satu ID salah menolak seluruh daftar: tidak ada entri yang berubah.
	w := s.call(t, http.MethodPost, "/v1/harga/bulk", emailKoordinator, map[string]any{"action": "validasi", "ids": []string{id, "HRG-D00-00-01-K"}})
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "HRG-D00-00-01-K", "pesan menyebut ID yang salah")
	assert.Equal(t, "DIAJUKAN", s.status(t, id))

	// ID benar, dengan spasi dan duplikat, diterima: dirapikan oleh ParseDaftar.
	w = s.call(t, http.MethodPost, "/v1/harga/bulk", emailKoordinator, map[string]any{"action": "validasi", "ids": []string{" " + id, id}})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, hasilBulk{Berhasil: 1}, data[hasilBulk](t, w))
	assert.Equal(t, "DIVALIDASI", s.status(t, id))
}

func TestPK_Respons409MemuatIDBigintVersiServer(t *testing.T) {
	s := sharedSuite(t)
	waktu := "2025-03-05T03:00:00Z"
	kirim := func(email string, harga int) (code int, body string) {
		w := s.call(t, http.MethodPost, "/v1/harga", email, map[string]any{
			"komoditas_kode": "MYK", "pasar_id": "PSR-1171-01", "tipe": "KONSUMEN", "harga": harga, "dicatat_pada": waktu,
		})
		return w.Code, w.Body.String()
	}
	code, body := kirim(emailMahasiswa, 17000)
	require.Equal(t, http.StatusCreated, code, body)
	code, body = kirim("mahasiswa.2@gardainflasi.id", 20000)
	require.Equal(t, http.StatusConflict, code, body)

	var res struct {
		Data struct {
			VersiServer struct {
				ID string `json:"id"`
			} `json:"versi_server"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &res))
	_, err := strconv.ParseInt(res.Data.VersiServer.ID, 10, 64)
	require.NoError(t, err, "versi_server.id string angka: %q", res.Data.VersiServer.ID)
	assert.Equal(t, http.StatusOK, s.call(t, http.MethodGet, "/v1/harga/"+url.PathEscape(res.Data.VersiServer.ID), emailKoordinator, nil).Code)
}

func TestPK_AuditMencatatIDBigint(t *testing.T) {
	s := sharedSuite(t)
	id := s.buat(t, emailMahasiswa)

	var n int64
	require.NoError(t, s.db.Raw("SELECT count(*) FROM audit_log WHERE aksi = 'HARGA_INPUT' AND entitas = 'entri_harga' AND entitas_id = ?", id).Scan(&n).Error)
	assert.EqualValues(t, 1, n)
}

func TestPK_SetiapEntriSeedPunyaKode(t *testing.T) {
	s := sharedSuite(t)

	var tanpaKode, seed int64
	require.NoError(t, s.db.Raw("SELECT count(*) FROM entri_harga WHERE kode IS NULL AND kode_pedagang LIKE 'PDG-%'").Scan(&tanpaKode).Error)
	require.NoError(t, s.db.Raw("SELECT count(*) FROM entri_harga WHERE kode LIKE 'HRG-D%'").Scan(&seed).Error)
	assert.Zero(t, tanpaKode, "setiap entri seed (berkode pedagang PDG-…) punya kode")
	assert.Greater(t, seed, int64(10_000))
}
