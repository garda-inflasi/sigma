package integration

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type eksekusiRow struct {
	ID                 int64          `json:"id"`
	SumberID           string         `json:"sumber_id"`
	Pemicu             string         `json:"pemicu"`
	Status             string         `json:"status"`
	Selesai            *string        `json:"selesai"`
	Dokumen            int            `json:"dokumen"`
	BarisDiambil       int            `json:"baris_diambil"`
	BarisTakTerpetakan int            `json:"baris_tak_terpetakan"`
	BarisTidakValid    int            `json:"baris_tidak_valid"`
	BarisDitolak       int            `json:"baris_ditolak"`
	Galat              *string        `json:"galat"`
	Ringkasan          map[string]any `json:"ringkasan"`
}

// jalankanBersih menghapus riwayat run agar tes mulai dari keadaan kosong.
func (s *suite) jalankanBersih(t *testing.T) {
	t.Helper()
	s.reset(t)
	for _, q := range []string{
		`DELETE FROM harga_eksternal`,
		`DELETE FROM data_mentah`,
		`DELETE FROM eksekusi_sumber`,
	} {
		require.NoError(t, s.db.Exec(q).Error)
	}
	s.fetcher.Tahan = nil
}

// tunggu menunggu run terakhir sumber selesai (status bukan BERJALAN) lewat API riwayat.
func (s *suite) tunggu(t *testing.T, id string) eksekusiRow {
	t.Helper()
	var terakhir eksekusiRow
	require.Eventually(t, func() bool {
		rows := decode[[]eksekusiRow](t, s.call(t, http.MethodGet, "/v1/sumber-data/"+id+"/eksekusi", emailLihat, nil))
		if len(rows) == 0 || rows[0].Status == "BERJALAN" {
			return false
		}
		terakhir = rows[0]
		return true
	}, 5*time.Second, 20*time.Millisecond, "run tidak selesai")
	return terakhir
}

func TestJalankan_Otorisasi(t *testing.T) {
	s := sharedSuite(t)
	s.jalankanBersih(t)

	assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodPost, "/v1/sumber-data/SRC-BI/jalankan", "", nil).Code)
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPost, "/v1/sumber-data/SRC-BI/jalankan", emailLihat, nil).Code, "sumber.lihat saja tidak boleh menjalankan")
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPost, "/v1/sumber-data/SRC-BI/jalankan", emailTanpaIzn, nil).Code)
	assert.Empty(t, s.fetcher.Dipanggil(), "penolakan tidak memicu pengambilan data")

	for _, path := range []string{"/v1/sumber-data/SRC-BI/eksekusi", "/v1/sumber-data/SRC-BI/sampel"} {
		assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodGet, path, "", nil).Code, path)
		assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, path, emailTanpaIzn, nil).Code, path)
		assert.Equal(t, http.StatusOK, s.call(t, http.MethodGet, path, emailLihat, nil).Code, path)
	}
}

func TestJalankan_MemicuRunNyataDanTercatat(t *testing.T) {
	s := sharedSuite(t)
	s.jalankanBersih(t)

	w := s.call(t, http.MethodPost, "/v1/sumber-data/SRC-BI/jalankan", emailKelola, nil)
	require.Equal(t, http.StatusAccepted, w.Code, w.Body.String())
	mulai := decode[eksekusiRow](t, w)
	assert.Equal(t, "BERJALAN", mulai.Status, "respons tidak menunggu run selesai")
	assert.Equal(t, "MANUAL", mulai.Pemicu)

	hasil := s.tunggu(t, "SRC-BI")
	assert.Equal(t, mulai.ID, hasil.ID)
	assert.Equal(t, "SUKSES", hasil.Status)
	assert.Equal(t, 1, hasil.Dokumen)
	assert.Equal(t, 14, hasil.BarisDiambil)
	assert.Equal(t, 3, hasil.BarisTakTerpetakan)
	assert.Equal(t, 4, hasil.BarisTidakValid)
	assert.Equal(t, 7, hasil.BarisDitolak)
	assert.NotNil(t, hasil.Selesai)
	assert.Nil(t, hasil.Galat)
	assert.Contains(t, hasil.Ringkasan, "tak_terpetakan", "baris yang tak terpetakan dilaporkan")

	// Aksi tercatat di audit trail dengan aktornya.
	var n int64
	require.NoError(t, s.db.Raw(`SELECT COUNT(*) FROM audit_log WHERE aksi = 'JALANKAN_SUMBER' AND entitas_id = 'SRC-BI' AND hasil = 'BERHASIL' AND aktor_email = ?`, emailKelola).Scan(&n).Error)
	assert.EqualValues(t, 1, n)

	// Katalog memuat statistik dari run nyata, bukan angka contoh.
	row := decode[sumberRow](t, s.call(t, http.MethodGet, "/v1/sumber-data/SRC-BI", emailLihat, nil))
	assert.Equal(t, "SEHAT", row.Status)
	require.NotNil(t, row.Keberhasilan7h)
	assert.InDelta(t, 100, *row.Keberhasilan7h, 0.001)

	// Sampel bronze tersedia.
	sampel := decode[[]struct {
		KunciPermintaan string `json:"kunci_permintaan"`
		Ukuran          int    `json:"ukuran"`
		Cuplikan        string `json:"cuplikan"`
	}](t, s.call(t, http.MethodGet, "/v1/sumber-data/SRC-BI/sampel", emailLihat, nil))
	require.Len(t, sampel, 1)
	assert.Equal(t, "wilayah=101", sampel[0].KunciPermintaan)
	assert.NotEmpty(t, sampel[0].Cuplikan)
}

func TestJalankan_Penolakan(t *testing.T) {
	s := sharedSuite(t)
	s.jalankanBersih(t)

	t.Run("sumber tak ada", func(t *testing.T) {
		assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodPost, "/v1/sumber-data/TIDAK-ADA/jalankan", emailKelola, nil).Code)
		assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodGet, "/v1/sumber-data/TIDAK-ADA/eksekusi", emailLihat, nil).Code)
		assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodGet, "/v1/sumber-data/TIDAK-ADA/sampel", emailLihat, nil).Code)
	})

	t.Run("tanpa konektor", func(t *testing.T) {
		assert.Equal(t, http.StatusUnprocessableEntity, s.call(t, http.MethodPost, "/v1/sumber-data/SRC-DJPB/jalankan", emailKelola, nil).Code)
	})

	t.Run("dijeda", func(t *testing.T) {
		require.NoError(t, s.db.Exec(`UPDATE sumber_data SET status = 'DIJEDA' WHERE id = 'SRC-BI'`).Error)
		assert.Equal(t, http.StatusConflict, s.call(t, http.MethodPost, "/v1/sumber-data/SRC-BI/jalankan", emailKelola, nil).Code)
		s.reset(t)
	})

	t.Run("sudah berjalan", func(t *testing.T) {
		s.fetcher.Tahan = make(chan struct{})
		first := s.call(t, http.MethodPost, "/v1/sumber-data/SRC-BI/jalankan", emailKelola, nil)
		require.Equal(t, http.StatusAccepted, first.Code, first.Body.String())

		assert.Equal(t, http.StatusConflict, s.call(t, http.MethodPost, "/v1/sumber-data/SRC-BI/jalankan", emailKelola, nil).Code)

		close(s.fetcher.Tahan)
		s.tunggu(t, "SRC-BI")
		s.fetcher.Tahan = nil
	})

	var ditolak int64
	require.NoError(t, s.db.Raw(`SELECT COUNT(*) FROM audit_log WHERE aksi = 'JALANKAN_SUMBER' AND hasil = 'DITOLAK'`).Scan(&ditolak).Error)
	assert.GreaterOrEqual(t, ditolak, int64(2), "penolakan (dijeda, sudah berjalan) tercatat")
}

func TestEksekusi_BatasDanUrutan(t *testing.T) {
	s := sharedSuite(t)
	s.jalankanBersih(t)
	for range 3 {
		require.Equal(t, http.StatusAccepted, s.call(t, http.MethodPost, "/v1/sumber-data/SRC-BI/jalankan", emailKelola, nil).Code)
		s.tunggu(t, "SRC-BI")
	}

	semua := decode[[]eksekusiRow](t, s.call(t, http.MethodGet, "/v1/sumber-data/SRC-BI/eksekusi", emailLihat, nil))
	require.Len(t, semua, 3)
	assert.Greater(t, semua[0].ID, semua[1].ID, "terbaru lebih dulu")

	dua := decode[[]eksekusiRow](t, s.call(t, http.MethodGet, "/v1/sumber-data/SRC-BI/eksekusi?limit=2", emailLihat, nil))
	assert.Len(t, dua, 2)

	for _, q := range []string{"limit=0", "limit=101", "limit=abc"} {
		assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodGet, "/v1/sumber-data/SRC-BI/eksekusi?"+q, emailLihat, nil).Code, q)
	}
	kosong := decode[[]eksekusiRow](t, s.call(t, http.MethodGet, "/v1/sumber-data/SRC-BPS/eksekusi", emailLihat, nil))
	assert.NotNil(t, kosong, "sumber tanpa run mengembalikan larik kosong, bukan null")
	assert.Empty(t, kosong)
}

func TestKatalog_MenandaiSumberYangBisaDijalankan(t *testing.T) {
	s := sharedSuite(t)
	s.reset(t)

	rows := decode[[]struct {
		ID             string `json:"id"`
		BisaDijalankan bool   `json:"bisa_dijalankan"`
	}](t, s.call(t, http.MethodGet, "/v1/sumber-data", emailLihat, nil))

	by := map[string]bool{}
	for _, r := range rows {
		by[r.ID] = r.BisaDijalankan
	}
	assert.True(t, by["SRC-BI"])
	assert.False(t, by["SRC-DJPB"])
}
