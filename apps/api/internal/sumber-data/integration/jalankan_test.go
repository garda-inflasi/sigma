package integration

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/dashboard/refresher"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type eksekusiRow struct {
	ID                 string         `json:"id"` // BIGINT ditulis sebagai string desimal
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
	require.NoError(t, refresher.Shared(s.db, logger.NewDefault()).Refresh(t.Context()))
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
	assert.Equal(t, sid("SRC-BI"), mulai.SumberID, "eksekusi menunjuk UUID sumber")
	assert.Regexp(t, `^[1-9][0-9]*$`, mulai.ID, "id eksekusi BIGINT sebagai string desimal")

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
	require.NoError(t, s.db.Raw(`SELECT COUNT(*) FROM audit_log WHERE aksi = 'JALANKAN_SUMBER' AND entitas = 'sumber_data' AND entitas_id = ? AND hasil = 'BERHASIL' AND aktor_email = ?`, sid("SRC-BI"), emailKelola).Scan(&n).Error)
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
		require.NoError(t, s.db.Exec(`UPDATE sumber_data SET status = 'DIJEDA' WHERE kode = 'SRC-BI'`).Error)
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
	id0, err0 := strconv.Atoi(semua[0].ID)
	id1, err1 := strconv.Atoi(semua[1].ID)
	require.NoError(t, err0)
	require.NoError(t, err1)
	assert.Greater(t, id0, id1, "terbaru lebih dulu")

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
		Kode           string `json:"kode"`
		BisaDijalankan bool   `json:"bisa_dijalankan"`
	}](t, s.call(t, http.MethodGet, "/v1/sumber-data", emailLihat, nil))

	by := map[string]bool{}
	for _, r := range rows {
		by[r.Kode] = r.BisaDijalankan
	}
	assert.True(t, by["SRC-BI"])
	assert.False(t, by["SRC-DJPB"])
}

type pipelineRow struct {
	Bronze int64 `json:"bronze"`
	Silver int64 `json:"silver"`
	Gold   int64 `json:"gold"`
	Demo   bool  `json:"demo"`
	Mutu   struct {
		Kelengkapan    *float64 `json:"kelengkapan"`
		Akurasi        *float64 `json:"akurasi"`
		KetepatanWaktu *float64 `json:"ketepatan_waktu"`
		Konsistensi    *float64 `json:"konsistensi"`
	} `json:"mutu"`
}

func TestPipeline_Otorisasi(t *testing.T) {
	s := sharedSuite(t)

	assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodGet, "/v1/sumber-data/pipeline", "", nil).Code)
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, "/v1/sumber-data/pipeline", emailTanpaIzn, nil).Code)
	assert.Equal(t, http.StatusOK, s.call(t, http.MethodGet, "/v1/sumber-data/pipeline", emailLihat, nil).Code)
}

func TestPipeline_KosongLaluTerisiDariRunNyata(t *testing.T) {
	s := sharedSuite(t)
	s.jalankanBersih(t)

	kosong := decode[pipelineRow](t, s.call(t, http.MethodGet, "/v1/sumber-data/pipeline", emailLihat, nil))
	assert.Zero(t, kosong.Bronze+kosong.Silver+kosong.Gold)
	assert.False(t, kosong.Demo)
	assert.Nil(t, kosong.Mutu.Kelengkapan, "tanpa run tidak ada angka mutu, bukan 100%")
	assert.Nil(t, kosong.Mutu.Konsistensi)
	assert.Nil(t, kosong.Mutu.Akurasi)
	require.NotNil(t, kosong.Mutu.KetepatanWaktu)
	assert.Zero(t, *kosong.Mutu.KetepatanWaktu, "SRC-BI punya konektor tetapi belum pernah sukses")

	require.Equal(t, http.StatusAccepted, s.call(t, http.MethodPost, "/v1/sumber-data/SRC-BI/jalankan", emailKelola, nil).Code)
	s.tunggu(t, "SRC-BI")
	require.NoError(t, refresher.Shared(s.db, logger.NewDefault()).Refresh(t.Context()))

	p := decode[pipelineRow](t, s.call(t, http.MethodGet, "/v1/sumber-data/pipeline", emailLihat, nil))
	assert.EqualValues(t, 1, p.Bronze)
	assert.EqualValues(t, 14, p.Silver)
	assert.EqualValues(t, 14, p.Gold)
	assert.False(t, p.Demo)
	require.NotNil(t, p.Mutu.Kelengkapan)
	assert.InDelta(t, 81.0, *p.Mutu.Kelengkapan, 0.001, "(21 dibaca - 4 tidak valid) / 21")
	require.NotNil(t, p.Mutu.Konsistensi)
	assert.InDelta(t, 85.7, *p.Mutu.Konsistensi, 0.001, "(21 dibaca - 3 tak terpetakan) / 21")
	assert.Nil(t, p.Mutu.Akurasi, "belum ada pasangan lapangan dan eksternal")
	require.NotNil(t, p.Mutu.KetepatanWaktu)
	assert.InDelta(t, 100, *p.Mutu.KetepatanWaktu, 0.001)
}

func TestPipeline_AkurasiDariSelisihLapanganVsEksternal(t *testing.T) {
	s := sharedSuite(t)
	s.jalankanBersih(t)
	// Hari ini di Banda Aceh: beras sepakat (selisih 2,9%), cabai tidak (50%).
	require.NoError(t, s.db.Exec(`
		INSERT INTO entri_harga (id, komoditas_kode, pasar_id, tipe, harga, satuan, status, dicatat_pada) VALUES
		('EH-PIPE-1', 'BRS-MED', 'PSR-1171-01', 'KONSUMEN', 14400, 'kg', 'DIVALIDASI', now()),
		('EH-PIPE-2', 'CBM-KRT', 'PSR-1171-01', 'KONSUMEN', 90000, 'kg', 'DIVALIDASI', now())`).Error)
	require.NoError(t, s.db.Exec(`
		INSERT INTO harga_eksternal (sumber_id, komoditas_kode, kabupaten_kode, tanggal, tipe, harga, satuan) VALUES
		(?, 'BRS-MED', '1171', (now() AT TIME ZONE 'Asia/Jakarta')::date, 'KONSUMEN', 14000, 'kg'),
		(?, 'CBM-KRT', '1171', (now() AT TIME ZONE 'Asia/Jakarta')::date, 'KONSUMEN', 60000, 'kg')`, sid("SRC-BI"), sid("SRC-BI")).Error)
	require.NoError(t, refresher.Shared(s.db, logger.NewDefault()).Refresh(t.Context()))

	p := decode[pipelineRow](t, s.call(t, http.MethodGet, "/v1/sumber-data/pipeline", emailLihat, nil))

	require.NotNil(t, p.Mutu.Akurasi)
	assert.InDelta(t, 50.0, *p.Mutu.Akurasi, 0.001)
	assert.EqualValues(t, 2, p.Gold)

	require.NoError(t, s.db.Exec(`DELETE FROM entri_harga WHERE id IN ('EH-PIPE-1', 'EH-PIPE-2')`).Error)
	require.NoError(t, refresher.Shared(s.db, logger.NewDefault()).Refresh(t.Context()))
}
