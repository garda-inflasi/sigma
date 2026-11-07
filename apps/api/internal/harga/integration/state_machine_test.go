package integration

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type rincianBulk struct {
	ID    string `json:"id"`
	Sebab string `json:"sebab"`
}

type riwayat struct {
	Dari     *string `json:"dari"`
	Ke       string  `json:"ke"`
	Oleh     *string `json:"oleh"`
	OlehNama *string `json:"oleh_nama"`
	Waktu    string  `json:"waktu"`
	Alasan   *string `json:"alasan"`
}

func (s *suite) bulk(t *testing.T, email string, body map[string]any) *hasilBulkRincian {
	t.Helper()
	w := s.call(t, http.MethodPost, "/v1/harga/bulk", email, body)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	h := data[hasilBulkRincian](t, w)
	return &h
}

type hasilBulkRincian struct {
	Berhasil int           `json:"berhasil"`
	Gagal    int           `json:"gagal"`
	Rincian  []rincianBulk `json:"rincian"`
}

func TestBulk_ReportsReasonPerSkippedEntry(t *testing.T) {
	s := sharedSuite(t)
	validated, own := s.buat(t, emailMahasiswa), s.buat(t, emailKoordinator)
	require.Equal(t, 1, s.bulk(t, emailKoordinator, map[string]any{"action": "validasi", "ids": []string{validated}}).Berhasil)

	got := s.bulk(t, emailKoordinator, map[string]any{"action": "validasi", "ids": []string{validated, own, "999999999"}})

	assert.Equal(t, hasilBulkRincian{Gagal: 3, Rincian: []rincianBulk{
		{ID: validated, Sebab: "status_tidak_sesuai"},
		{ID: own, Sebab: "pemisahan_tugas"},
		{ID: "999999999", Sebab: "tidak_ditemukan"},
	}}, *got)
}

func TestBulk_TolakRequiresReason(t *testing.T) {
	s := sharedSuite(t)
	id := s.buat(t, emailMahasiswa)

	for _, alasan := range []string{"", "  ", "x"} {
		w := s.call(t, http.MethodPost, "/v1/harga/bulk", emailKoordinator, map[string]any{"action": "tolak", "ids": []string{id}, "alasan": alasan})
		assert.Equal(t, http.StatusBadRequest, w.Code, "alasan %q", alasan)
	}
	assert.Equal(t, "DIAJUKAN", s.status(t, id))
}

func TestBulk_TolakSetsReasonVisibleToTheSubmitter(t *testing.T) {
	s := sharedSuite(t)
	id := s.buat(t, emailMahasiswa)
	require.Equal(t, 1, s.bulk(t, emailKoordinator, map[string]any{"action": "tolak", "ids": []string{id}, "alasan": "satuan salah, survei ulang"}).Berhasil)

	rows := data[[]struct {
		ID              string  `json:"id"`
		AlasanPenolakan *string `json:"alasan_penolakan"`
	}](t, s.call(t, http.MethodGet, "/v1/harga/saya?status=DITOLAK&limit=1000", emailMahasiswa, nil))

	var ditemukan bool
	for _, e := range rows {
		require.NotNil(t, e.AlasanPenolakan, "setiap entri DITOLAK menyertakan alasan: %s", e.ID)
		if e.ID == id {
			ditemukan = true
			assert.Equal(t, "satuan salah, survei ulang", *e.AlasanPenolakan)
		}
	}
	assert.True(t, ditemukan)

	detail := data[struct {
		DitolakOleh *string `json:"ditolak_oleh"`
		DitolakPada *string `json:"ditolak_pada"`
	}](t, s.call(t, http.MethodGet, "/v1/harga/"+id, emailKoordinator, nil))
	require.NotNil(t, detail.DitolakOleh)
	assert.Equal(t, uid("koordinator_lapangan"), *detail.DitolakOleh)
	assert.NotNil(t, detail.DitolakPada)
}

func TestBulk_AjukanDraftOnlyByOwner(t *testing.T) {
	s := sharedSuite(t)
	id := s.buat(t, emailMahasiswa)
	require.Equal(t, 1, s.bulk(t, emailKoordinator, map[string]any{"action": "kembalikan", "ids": []string{id}}).Berhasil)
	require.Equal(t, "DRAF", s.status(t, id))

	// Mahasiswa lain memegang harga.input tetapi bukan pencatatnya.
	lain := s.bulk(t, "mahasiswa.2@gardainflasi.id", map[string]any{"action": "ajukan", "ids": []string{id}})
	assert.Equal(t, hasilBulkRincian{Gagal: 1, Rincian: []rincianBulk{{ID: id, Sebab: "bukan_pemilik"}}}, *lain)

	pemilik := s.bulk(t, emailMahasiswa, map[string]any{"action": "ajukan", "ids": []string{id}})
	assert.Equal(t, hasilBulkRincian{Berhasil: 1, Rincian: []rincianBulk{}}, *pemilik)
	assert.Equal(t, "DIAJUKAN", s.status(t, id))

	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPost, "/v1/harga/bulk", emailMahasiswa, map[string]any{"action": "validasi", "ids": []string{id}}).Code, "mahasiswa tidak boleh memvalidasi")
}

func TestBulk_PublikasiRespectsSeparationOfDuties(t *testing.T) {
	s := sharedSuite(t)
	id := s.buat(t, emailMahasiswa)
	require.Equal(t, 1, s.bulk(t, emailKoordinator, map[string]any{"action": "validasi", "ids": []string{id}}).Berhasil)
	// Entri milik manajer data tidak boleh dipublikasikan olehnya sendiri.
	manajerOwn := s.buat(t, emailMahasiswa)
	require.NoError(t, s.db.Exec("UPDATE entri_harga SET petugas_id = garda_uuid_stabil('pengguna', 'manajer_data'), status = 'DIVALIDASI', divalidasi_oleh = garda_uuid_stabil('pengguna', 'koordinator_lapangan') WHERE id = ?", manajerOwn).Error)

	got := s.bulk(t, emailManajerData, map[string]any{"action": "publikasi", "ids": []string{manajerOwn, id}})

	assert.Equal(t, hasilBulkRincian{Berhasil: 1, Gagal: 1, Rincian: []rincianBulk{{ID: manajerOwn, Sebab: "pemisahan_tugas"}}}, *got)
}

func TestRiwayat_RecordsEveryTransition(t *testing.T) {
	s := sharedSuite(t)
	id := s.buat(t, emailMahasiswa)
	require.Equal(t, 1, s.bulk(t, emailKoordinator, map[string]any{"action": "validasi", "ids": []string{id}}).Berhasil)
	require.Equal(t, 1, s.bulk(t, emailManajerData, map[string]any{"action": "publikasi", "ids": []string{id}}).Berhasil)

	w := s.call(t, http.MethodGet, "/v1/harga/"+id+"/riwayat", emailKoordinator, nil)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	got := data[[]riwayat](t, w)
	require.Len(t, got, 3)
	assert.Nil(t, got[0].Dari)
	assert.Equal(t, []string{"DIAJUKAN", "DIVALIDASI", "DIPUBLIKASI"}, []string{got[0].Ke, got[1].Ke, got[2].Ke})
	assert.Equal(t, "DIAJUKAN", *got[1].Dari)
	assert.Equal(t, uid("mahasiswa_1"), *got[0].Oleh)
	assert.Equal(t, uid("koordinator_lapangan"), *got[1].Oleh)
	assert.Equal(t, uid("manajer_data"), *got[2].Oleh)
	require.NotNil(t, got[1].OlehNama)
	assert.NotEmpty(t, *got[1].OlehNama)

	t.Run("petugas melihat riwayat entrinya sendiri saja", func(t *testing.T) {
		assert.Equal(t, http.StatusOK, s.call(t, http.MethodGet, "/v1/harga/"+id+"/riwayat", emailMahasiswa, nil).Code)
		assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodGet, "/v1/harga/"+id+"/riwayat", "mahasiswa.2@gardainflasi.id", nil).Code)
	})
	t.Run("izin dan entri tidak ada", func(t *testing.T) {
		assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodGet, "/v1/harga/tidak-ada/riwayat", emailKoordinator, nil).Code)
		assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, "/v1/harga/"+id+"/riwayat", "superadmin@gardainflasi.id", nil).Code, "tanpa harga.lihat dan harga.input")
		assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodGet, "/v1/harga/"+id+"/riwayat", "", nil).Code)
	})
}

func TestRiwayat_RejectionCarriesReason(t *testing.T) {
	s := sharedSuite(t)
	id := s.buat(t, emailMahasiswa)
	require.Equal(t, 1, s.bulk(t, emailKoordinator, map[string]any{"action": "tolak", "ids": []string{id}, "alasan": "bukti kurang"}).Berhasil)

	got := data[[]riwayat](t, s.call(t, http.MethodGet, "/v1/harga/"+id+"/riwayat", emailKoordinator, nil))

	require.Len(t, got, 2)
	assert.Equal(t, "DITOLAK", got[1].Ke)
	require.NotNil(t, got[1].Alasan)
	assert.Equal(t, "bukti kurang", *got[1].Alasan)
}

func TestSeededHistoryHasRiwayatAndReasons(t *testing.T) {
	s := sharedSuite(t)

	var tanpaRiwayat int64
	require.NoError(t, s.db.Raw(`
		SELECT count(*) FROM entri_harga e
		 WHERE e.kode LIKE 'HRG-D%' AND NOT EXISTS (SELECT 1 FROM riwayat_status_entri h WHERE h.entri_id = e.id)`).Scan(&tanpaRiwayat).Error)
	assert.Zero(t, tanpaRiwayat)

	var dipublikasi, riwayatPublikasi int64
	require.NoError(t, s.db.Raw("SELECT count(*) FROM entri_harga WHERE kode LIKE 'HRG-D%' AND status = 'DIPUBLIKASI'").Scan(&dipublikasi).Error)
	require.NoError(t, s.db.Raw(`
		SELECT count(*) FROM riwayat_status_entri h JOIN entri_harga e ON e.id = h.entri_id
		 WHERE e.kode LIKE 'HRG-D%' AND h.ke = 'DIPUBLIKASI'`).Scan(&riwayatPublikasi).Error)
	assert.Positive(t, dipublikasi)
	assert.Equal(t, dipublikasi, riwayatPublikasi)

	var ditolakTanpaAlasan int64
	require.NoError(t, s.db.Raw("SELECT count(*) FROM entri_harga WHERE status = 'DITOLAK' AND alasan_penolakan IS NULL").Scan(&ditolakTanpaAlasan).Error)
	assert.Zero(t, ditolakTanpaAlasan)
}

func TestDatabaseEnforcesSeparationOfDutiesAndRejectionReason(t *testing.T) {
	s := sharedSuite(t)
	id := s.buat(t, emailMahasiswa)

	err := s.db.Exec("UPDATE entri_harga SET status = 'DIVALIDASI', divalidasi_oleh = petugas_id WHERE id = ?", id).Error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chk_entri_harga_pemisahan_tugas")

	err = s.db.Exec("UPDATE entri_harga SET status = 'DITOLAK' WHERE id = ?", id).Error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chk_entri_harga_alasan_penolakan")
}
