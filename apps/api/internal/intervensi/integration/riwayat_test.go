package integration

import (
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (s *suite) pindah(t *testing.T, email, id, tahap string, urutan, versi int) *struct {
	Code int
	Body string
} {
	t.Helper()
	w := s.call(t, http.MethodPatch, "/v1/intervensi/"+id+"/tahap", email, map[string]any{"tahap": tahap, "urutan": urutan, "versi": versi})
	return &struct {
		Code int
		Body string
	}{w.Code, w.Body.String()}
}

type riwayatRow struct {
	Aksi     string  `json:"aksi"`
	Dari     *string `json:"dari"`
	Ke       *string `json:"ke"`
	OlehID   *string `json:"oleh_id"`
	OlehNama *string `json:"oleh_nama"`
}

func riwayat(t *testing.T, s *suite, id string) []riwayatRow {
	t.Helper()
	w := s.call(t, http.MethodGet, "/v1/intervensi/"+id+"/riwayat", emailLihat, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	return data[[]riwayatRow](t, w)
}

func TestMove_OptimisticLockingRejectsStaleVersion(t *testing.T) {
	s := newSuite(t, "intervensi_versi")
	before := card(t, s, "INT-301")
	require.Equal(t, 1, before.Versi)

	t.Run("versi wajib", func(t *testing.T) {
		w := s.call(t, http.MethodPatch, "/v1/intervensi/INT-301/tahap", emailKelola, map[string]any{"tahap": "t-verifikasi", "urutan": 0})
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("pemindahan sah mengembalikan dan menyimpan versi baru", func(t *testing.T) {
		w := s.call(t, http.MethodPatch, "/v1/intervensi/INT-301/tahap", emailKelola, map[string]any{"tahap": "t-verifikasi", "urutan": 0, "versi": 1})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, 2, data[struct {
			Versi int `json:"versi"`
		}](t, w).Versi)
		assert.Equal(t, 2, card(t, s, "INT-301").Versi)
	})

	t.Run("pengguna lain yang masih memegang versi lama ditolak 409 dan kartu tidak berubah", func(t *testing.T) {
		res := s.pindah(t, emailKelola, "INT-301", "t-ews", 0, 1)
		assert.Equal(t, http.StatusConflict, res.Code, res.Body)
		after := card(t, s, "INT-301")
		assert.Equal(t, "t-verifikasi", after.Tahap)
		assert.Equal(t, 2, after.Versi)
	})

	t.Run("dua pemindahan bersamaan dari versi yang sama: tepat satu menang", func(t *testing.T) {
		codes := make([]int, 2)
		var wg sync.WaitGroup
		for i, tahap := range []string{"t-ews", "t-verifikasi"} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				codes[i] = s.pindah(t, emailKelola, "INT-301", tahap, 0, 2).Code
			}()
		}
		wg.Wait()
		assert.ElementsMatch(t, []int{http.StatusOK, http.StatusConflict}, codes)
		assert.Equal(t, 3, card(t, s, "INT-301").Versi, "hanya satu kenaikan versi")
	})

	t.Run("penolakan izin tidak menaikkan versi", func(t *testing.T) {
		v := card(t, s, "INT-301").Versi
		assert.Equal(t, http.StatusForbidden, s.pindah(t, emailKelola, "INT-301", "t-petakan", 0, v).Code)
		assert.Equal(t, v, card(t, s, "INT-301").Versi)
	})
}

func TestMove_CannotSkipApprovalStageWithoutItsPermission(t *testing.T) {
	s := newSuite(t, "intervensi_lompat")

	// INT-301 di t-ews (1) ke t-logistik (4) melompati t-petakan (3, butuh setujui): peran kelola ditolak dengan alasan jelas.
	res := s.pindah(t, emailKelola, "INT-301", "t-logistik", 0, 1)
	assert.Equal(t, http.StatusForbidden, res.Code)
	assert.Contains(t, res.Body, "melewati tahap")
	assert.Contains(t, res.Body, "Petakan suplai sentra surplus")
	assert.Contains(t, res.Body, "intervensi.setujui")
	assert.Equal(t, "t-ews", card(t, s, "INT-301").Tahap)
	assert.Equal(t, 1, card(t, s, "INT-301").Versi)

	// Penyetuju memegang izin tahap yang dilompati, tetapi tahap tujuan butuh kelola: ditolak oleh izin tujuan.
	assert.Equal(t, http.StatusForbidden, s.pindah(t, emailSetuju, "INT-301", "t-logistik", 0, 1).Code)

	// Tahap yang bersebelahan tetap boleh, dan mundur melewati tahap persetujuan juga boleh.
	assert.Equal(t, http.StatusOK, s.pindah(t, emailKelola, "INT-301", "t-verifikasi", 0, 1).Code)
	assert.Equal(t, http.StatusOK, s.pindah(t, emailKelola, "INT-304", "t-verifikasi", 0, 1).Code, "t-logistik ke t-verifikasi mundur melewati t-petakan")
}

func TestMove_RecordsHistoryOnlyWhenTahapChanges(t *testing.T) {
	s := newSuite(t, "intervensi_riwayat_pindah")
	seeded := riwayat(t, s, "INT-303") // kartu demo di t-verifikasi
	require.Len(t, seeded, 2, "dibuat di t-ews lalu dipindah ke t-verifikasi")
	assert.Equal(t, "BUAT", seeded[0].Aksi)
	assert.Equal(t, "PINDAH_TAHAP", seeded[1].Aksi)

	// Mengurutkan ulang dalam kolom yang sama menaikkan versi tetapi tidak menambah riwayat.
	require.Equal(t, http.StatusOK, s.pindah(t, emailKelola, "INT-303", "t-verifikasi", 1, 1).Code)
	assert.Equal(t, 2, card(t, s, "INT-303").Versi)
	assert.Len(t, riwayat(t, s, "INT-303"), 2)

	require.Equal(t, http.StatusOK, s.pindah(t, emailKelola, "INT-303", "t-ews", 0, 2).Code)
	rows := riwayat(t, s, "INT-303")
	require.Len(t, rows, 3)
	assert.Equal(t, "t-verifikasi", *rows[2].Dari)
	assert.Equal(t, "t-ews", *rows[2].Ke)
}

type hasilBulk struct {
	Berhasil int `json:"berhasil"`
	Gagal    int `json:"gagal"`
	Hasil    []struct {
		ID       string `json:"id"`
		Berhasil bool   `json:"berhasil"`
		Alasan   string `json:"alasan"`
	} `json:"hasil"`
}

func TestHistory_CreateAssignEscalateAndMove(t *testing.T) {
	s := newSuite(t, "intervensi_riwayat")
	w := s.call(t, http.MethodPost, "/v1/intervensi", emailKelola, map[string]any{
		"judul": "Operasi pasar", "siklus": "TEKANAN", "prioritas": "RENDAH", "komoditas_kode": "BRS-MED", "kabupaten_kode": "1171",
		"tenggat": "2030-01-01T00:00:00Z",
	})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	id := data[struct {
		ID string `json:"id"`
	}](t, w).ID

	bulk := func(action string, ids ...string) (int, hasilBulk) {
		w := s.call(t, http.MethodPost, "/v1/intervensi/bulk", emailKelola, map[string]any{"action": action, "ids": ids})
		if w.Code != http.StatusOK {
			return w.Code, hasilBulk{}
		}
		return w.Code, data[hasilBulk](t, w)
	}

	code, res := bulk("tugaskan", id, "INT-319", "INT-9999", id)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, 1, res.Berhasil)
	assert.Equal(t, 2, res.Gagal)
	require.Len(t, res.Hasil, 3, "id ganda dibuang; hasil per kartu sesuai urutan")
	assert.Equal(t, id, res.Hasil[0].ID)
	assert.True(t, res.Hasil[0].Berhasil)
	assert.Equal(t, "INT-319", res.Hasil[1].ID)
	assert.Equal(t, "Kartu sudah selesai", res.Hasil[1].Alasan)
	assert.Equal(t, "Kartu tidak ditemukan", res.Hasil[2].Alasan)

	k := card(t, s, id)
	require.Len(t, k.PenanggungJawab, 1)
	assert.Equal(t, "manajer_program", k.PenanggungJawab[0].ID)
	assert.Equal(t, 2, k.Versi, "penugasan menaikkan versi")

	_, again := bulk("tugaskan", id)
	assert.Equal(t, "Anda sudah menjadi penanggung jawab", again.Hasil[0].Alasan)
	assert.Equal(t, 2, card(t, s, id).Versi, "kartu yang gagal tidak berubah")

	_, esc := bulk("eskalasi", id)
	assert.Equal(t, 1, esc.Berhasil)
	k = card(t, s, id)
	assert.Equal(t, "TINGGI", k.Prioritas, "kartu yang dieskalasi dinaikkan ke prioritas tinggi")
	assert.NotNil(t, k.DieskalasiPada)
	assert.Equal(t, 3, k.Versi)
	_, escAgain := bulk("eskalasi", id)
	assert.Equal(t, "Kartu sudah dieskalasi", escAgain.Hasil[0].Alasan)

	require.Equal(t, http.StatusOK, s.pindah(t, emailKelola, id, "t-verifikasi", 0, 3).Code)

	rows := riwayat(t, s, id)
	var aksi []string
	for _, r := range rows {
		aksi = append(aksi, r.Aksi)
	}
	assert.Equal(t, []string{"BUAT", "TUGASKAN", "ESKALASI", "AJUKAN", "PINDAH_TAHAP"}, aksi, "eskalasi membuka permintaan telaah Dewan Pengarah (AJUKAN)")
	last := rows[len(rows)-1]
	assert.Equal(t, "t-ews", *last.Dari)
	assert.Equal(t, "t-verifikasi", *last.Ke)
	require.NotNil(t, last.OlehNama)
	assert.Equal(t, "Rina Safitri, S.T., M.T.", *last.OlehNama)

	var audited []string
	require.NoError(t, s.db.Raw(`SELECT aksi FROM audit_log WHERE entitas = 'intervensi' AND aksi LIKE 'BULK_%' ORDER BY waktu`).Scan(&audited).Error)
	assert.Equal(t, []string{"BULK_TUGASKAN", "BULK_ESKALASI"}, audited, "hanya aksi yang mengubah data")
}

func TestBulkAndHistoryAuthorization(t *testing.T) {
	s := sharedSuite(t)
	body := map[string]any{"action": "tugaskan", "ids": []string{"INT-301"}}

	assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodPost, "/v1/intervensi/bulk", "", body).Code)
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPost, "/v1/intervensi/bulk", emailLihat, body).Code)
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPost, "/v1/intervensi/bulk", emailSetuju, body).Code, "penyetuju tanpa kelola tidak menugaskan/mengeskalasi")

	assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodPost, "/v1/intervensi/bulk", emailKelola, map[string]any{"action": "hapus", "ids": []string{"INT-301"}}).Code)
	assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodPost, "/v1/intervensi/bulk", emailKelola, map[string]any{"action": "tugaskan", "ids": []string{}}).Code)

	assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodGet, "/v1/intervensi/INT-301/riwayat", "", nil).Code)
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, "/v1/intervensi/INT-301/riwayat", emailTanpaIzin, nil).Code)
	assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodGet, "/v1/intervensi/INT-9999/riwayat", emailLihat, nil).Code)
}
