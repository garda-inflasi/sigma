package integration

import (
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const emailSetuju2 = "strategis.bps@gardainflasi.id" // STRATEGIS_BPS: intervensi.setujui

type langkahRow struct {
	Urutan         int     `json:"urutan"`
	Label          string  `json:"label"`
	IzinDibutuhkan string  `json:"izin_dibutuhkan"`
	Keputusan      *string `json:"keputusan"`
	OlehID         *string `json:"oleh_id"`
	Catatan        string  `json:"catatan"`
}

type persetujuanRow struct {
	ID              string       `json:"id"`
	IntervensiID    string       `json:"intervensi_id"`
	KartuJudul      string       `json:"kartu_judul"`
	Jenis           string       `json:"jenis"`
	TahapTujuan     *string      `json:"tahap_tujuan"`
	Status          string       `json:"status"`
	LangkahKini     int          `json:"langkah_kini"`
	IzinLangkahKini string       `json:"izin_langkah_kini"`
	DiajukanOlehID  *string      `json:"diajukan_oleh_id"`
	Langkah         []langkahRow `json:"langkah"`
}

func (s *suite) ajukan(t *testing.T, email, kartu, tahap string) (int, persetujuanRow, string) {
	t.Helper()
	w := s.call(t, http.MethodPost, "/v1/intervensi/"+rujukan(kartu)+"/persetujuan", email, map[string]any{"tahap_tujuan": tahap, "catatan": "Siap diajukan"})
	if w.Code != http.StatusCreated {
		return w.Code, persetujuanRow{}, w.Body.String()
	}
	return w.Code, data[persetujuanRow](t, w), w.Body.String()
}

func (s *suite) putus(t *testing.T, email, id, keputusan, catatan string) (int, persetujuanRow, string) {
	t.Helper()
	w := s.call(t, http.MethodPost, "/v1/intervensi/persetujuan/"+id+"/keputusan", email, map[string]any{"keputusan": keputusan, "catatan": catatan})
	if w.Code != http.StatusOK {
		return w.Code, persetujuanRow{}, w.Body.String()
	}
	return w.Code, data[persetujuanRow](t, w), w.Body.String()
}

func (s *suite) daftarPersetujuan(t *testing.T, query string) []persetujuanRow {
	t.Helper()
	w := s.call(t, http.MethodGet, "/v1/intervensi/persetujuan"+query, emailLihat, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	return data[[]persetujuanRow](t, w)
}

func TestAlur_AreSeededAsData(t *testing.T) {
	s := sharedSuite(t)
	w := s.call(t, http.MethodGet, "/v1/intervensi/alur", emailLihat, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	alur := data[[]struct {
		Kode        string  `json:"kode"`
		Jenis       string  `json:"jenis"`
		Siklus      *string `json:"siklus"`
		TahapTujuan *string `json:"tahap_tujuan"`
		Aktif       bool    `json:"aktif"`
		Langkah     []struct {
			Urutan int    `json:"urutan"`
			Izin   string `json:"izin_dibutuhkan"`
		} `json:"langkah"`
	}](t, w)

	byKode := map[string]int{}
	for i, a := range alur {
		byKode[a.Kode] = i
		assert.True(t, a.Aktif, a.Kode)
	}
	require.Len(t, alur, 5)
	fiskal := alur[byKode["pindah-tekanan-fiskal"]]
	require.Len(t, fiskal.Langkah, 2, "dukungan fiskal: Dewan Pengarah lalu Kanwil DJPb")
	assert.Equal(t, "intervensi.setujui", fiskal.Langkah[0].Izin)
	assert.Equal(t, "intervensi.setujui_fiskal", fiskal.Langkah[1].Izin)
	assert.Equal(t, "ESKALASI", alur[byKode["eskalasi"]].Jenis)
	assert.Nil(t, alur[byKode["eskalasi"]].TahapTujuan)

	assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodGet, "/v1/intervensi/alur", "", nil).Code)
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, "/v1/intervensi/alur", emailTanpaIzin, nil).Code)
}

func TestPersetujuan_SingleStepApprovalMovesTheCard(t *testing.T) {
	s := newSuite(t, "intervensi_setuju_satu")

	code, p, body := s.ajukan(t, emailKelola, "INT-301", "t-petakan")
	require.Equal(t, http.StatusCreated, code, body)
	assert.Equal(t, "MENUNGGU", p.Status)
	assert.Equal(t, "PINDAH_TAHAP", p.Jenis)
	require.Len(t, p.Langkah, 1)
	assert.Equal(t, "intervensi.setujui", p.IzinLangkahKini)
	require.NotNil(t, p.DiajukanOlehID)
	assert.Equal(t, uid("manajer_program"), *p.DiajukanOlehID)
	assert.Equal(t, "t-ews", card(t, s, "INT-301").Tahap, "kartu belum berpindah sebelum disetujui")

	t.Run("pemegang kelola tidak dapat memutuskan", func(t *testing.T) {
		code, _, _ := s.putus(t, emailKelola, p.ID, "SETUJU", "")
		assert.Equal(t, http.StatusForbidden, code)
	})

	t.Run("penyetuju menyetujui; kartu pindah ke akhir kolom tujuan", func(t *testing.T) {
		before := card(t, s, "INT-301").Versi
		code, done, body := s.putus(t, emailSetuju, p.ID, "SETUJU", "Disetujui")
		require.Equal(t, http.StatusOK, code, body)
		assert.Equal(t, "DISETUJUI", done.Status)
		require.NotNil(t, done.Langkah[0].Keputusan)
		assert.Equal(t, "SETUJU", *done.Langkah[0].Keputusan)
		assert.Equal(t, uid("strategis_bi"), *done.Langkah[0].OlehID)

		k := card(t, s, "INT-301")
		assert.Equal(t, "t-petakan", k.Tahap)
		assert.Equal(t, before+1, k.Versi)
		assert.Equal(t, []string{"INT-307", "INT-301"}, column(t, s, "t-petakan"), "diletakkan di akhir kolom")
	})

	t.Run("riwayat kartu mencatat pengajuan, persetujuan, dan pemindahan", func(t *testing.T) {
		var aksi []string
		for _, r := range riwayat(t, s, "INT-301") {
			aksi = append(aksi, r.Aksi)
		}
		assert.Equal(t, []string{"BUAT", "AJUKAN", "SETUJU", "PINDAH_TAHAP"}, aksi)
	})

	t.Run("sudah diputuskan: tidak dapat diputus lagi", func(t *testing.T) {
		code, _, _ := s.putus(t, emailSetuju2, p.ID, "SETUJU", "")
		assert.Equal(t, http.StatusConflict, code)
	})

	t.Run("audit mencatat pengajuan dan keputusan", func(t *testing.T) {
		var aksi []string
		require.NoError(t, s.db.Raw(`SELECT aksi FROM audit_log WHERE entitas = 'intervensi' AND aksi IN ('AJUKAN_PERSETUJUAN', 'PUTUS_PERSETUJUAN') ORDER BY waktu, hasil`).Scan(&aksi).Error)
		assert.Equal(t, []string{"AJUKAN_PERSETUJUAN", "PUTUS_PERSETUJUAN"}, aksi, "penolakan izin di route tidak sampai ke usecase; penyetujuan tercatat")
	})
}

func TestPersetujuan_MultiStepChainAndSeparationOfDuties(t *testing.T) {
	s := newSuite(t, "intervensi_setuju_berjenjang")

	// Seed demo: INT-304 (t-logistik) menunggu pindah ke t-fiskal oleh manajer_program (2 langkah).
	menunggu := s.daftarPersetujuan(t, "?status=MENUNGGU")
	require.Len(t, menunggu, 1)
	p := menunggu[0]
	assert.Equal(t, kid("INT-304"), p.IntervensiID, "permintaan menunjuk kartu lewat UUID")
	assert.Regexp(t, `^[1-9][0-9]*$`, p.ID, "ID permintaan adalah string desimal")
	require.Len(t, p.Langkah, 2)
	assert.Equal(t, 1, p.LangkahKini)

	t.Run("langkah 2 tidak dapat dilompati: langkah 1 harus diputus lebih dulu", func(t *testing.T) {
		code, _, body := s.putus(t, emailFiskal, p.ID, "SETUJU", "")
		require.Equal(t, http.StatusOK, code, body) // DJPb memegang setujui, jadi sah memutus langkah 1 sekarang
		// Tes pemisahan tugas ada di bawah; di sini kembalikan keputusan langkah 1 ke penyetuju lain.
		require.NoError(t, s.db.Exec(`UPDATE intervensi_persetujuan_langkah SET oleh = garda_uuid_stabil('pengguna', 'strategis_bi') WHERE persetujuan_id = ? AND urutan = 1`, p.ID).Error)
	})

	t.Run("langkah 2 butuh izin fiskal: penyetuju biasa ditolak dengan izin yang jelas", func(t *testing.T) {
		code, _, body := s.putus(t, emailSetuju, p.ID, "SETUJU", "")
		assert.Equal(t, http.StatusForbidden, code)
		assert.Contains(t, body, "intervensi.setujui_fiskal")
		assert.Equal(t, "t-logistik", card(t, s, "INT-304").Tahap, "belum pindah sebelum langkah terakhir")
	})

	t.Run("langkah terakhir disetujui pemegang izin fiskal: kartu pindah", func(t *testing.T) {
		code, done, body := s.putus(t, emailFiskal, p.ID, "SETUJU", "Instrumen belanja disetujui")
		require.Equal(t, http.StatusOK, code, body)
		assert.Equal(t, "DISETUJUI", done.Status)
		assert.Equal(t, "t-fiskal", card(t, s, "INT-304").Tahap)
	})
}

func TestPersetujuan_SamePersonCannotDecideTwoSteps(t *testing.T) {
	s := newSuite(t, "intervensi_setuju_pemisahan")
	// t-ews ke t-fiskal: [setujui, setujui, setujui_fiskal].
	_, p, _ := s.ajukan(t, emailKelola, "INT-301", "t-fiskal")

	code, afterOne, body := s.putus(t, emailFiskal, p.ID, "SETUJU", "")
	require.Equal(t, http.StatusOK, code, body)
	assert.Equal(t, 2, afterOne.LangkahKini)

	code, _, body = s.putus(t, emailFiskal, p.ID, "SETUJU", "")
	assert.Equal(t, http.StatusForbidden, code)
	assert.Contains(t, body, "Pemisahan tugas")

	code, afterTwo, _ := s.putus(t, emailSetuju, p.ID, "SETUJU", "")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, 3, afterTwo.LangkahKini)
	assert.Equal(t, "intervensi.setujui_fiskal", afterTwo.IzinLangkahKini)
}

func TestPersetujuan_RejectRequiresNoteAndLeavesCardUntouched(t *testing.T) {
	s := newSuite(t, "intervensi_setuju_tolak")
	code, p, body := s.ajukan(t, emailKelola, "INT-302", "t-petakan")
	require.Equal(t, http.StatusCreated, code, body)

	code, _, _ = s.putus(t, emailSetuju, p.ID, "TOLAK", "  ")
	assert.Equal(t, http.StatusBadRequest, code, "penolakan wajib bercatatan")

	code, done, body := s.putus(t, emailSetuju, p.ID, "TOLAK", "Suplai belum dipetakan")
	require.Equal(t, http.StatusOK, code, body)
	assert.Equal(t, "DITOLAK", done.Status)
	assert.Equal(t, "Suplai belum dipetakan", done.Langkah[0].Catatan)
	assert.Equal(t, "t-ews", card(t, s, "INT-302").Tahap)

	// Setelah ditolak, kartu boleh diajukan lagi.
	code, _, body = s.ajukan(t, emailKelola, "INT-302", "t-petakan")
	assert.Equal(t, http.StatusCreated, code, body)
}

func TestPersetujuan_ChainSpansEveryApprovalStageOnThePath(t *testing.T) {
	s := newSuite(t, "intervensi_setuju_rantai")

	// t-ews (1) ke t-fiskal (5) melewati t-petakan (satu langkah) dan t-fiskal (dua langkah).
	code, p, body := s.ajukan(t, emailKelola, "INT-301", "t-fiskal")
	require.Equal(t, http.StatusCreated, code, body)
	require.Len(t, p.Langkah, 3, "tahap persetujuan yang dilalui tidak boleh dilewati")
	assert.Equal(t, []string{"intervensi.setujui", "intervensi.setujui", "intervensi.setujui_fiskal"},
		[]string{p.Langkah[0].IzinDibutuhkan, p.Langkah[1].IzinDibutuhkan, p.Langkah[2].IzinDibutuhkan})
	assert.Equal(t, "Persetujuan Dewan Pengarah", p.Langkah[0].Label)
}

func TestPersetujuan_AjukanValidation(t *testing.T) {
	s := newSuite(t, "intervensi_setuju_validasi")

	t.Run("izin kelola dibutuhkan", func(t *testing.T) {
		body := map[string]any{"tahap_tujuan": "t-petakan"}
		assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodPost, "/v1/intervensi/INT-301/persetujuan", "", body).Code)
		assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPost, "/v1/intervensi/INT-301/persetujuan", emailSetuju, body).Code)
	})

	t.Run("perpindahan tanpa tahap persetujuan atau mundur tidak butuh persetujuan", func(t *testing.T) {
		code, _, body := s.ajukan(t, emailKelola, "INT-301", "t-verifikasi")
		assert.Equal(t, http.StatusBadRequest, code)
		assert.Contains(t, body, "pindahkan kartu langsung")
		code, _, _ = s.ajukan(t, emailKelola, "INT-307", "t-ews") // mundur
		assert.Equal(t, http.StatusBadRequest, code)
		code, _, _ = s.ajukan(t, emailKelola, "INT-301", "t-ews") // tahap sama
		assert.Equal(t, http.StatusBadRequest, code)
	})

	t.Run("tahap tidak valid, kartu tidak ada, masukan kosong", func(t *testing.T) {
		code, _, _ := s.ajukan(t, emailKelola, "INT-301", "n-pantau")
		assert.Equal(t, http.StatusBadRequest, code)
		code, _, _ = s.ajukan(t, emailKelola, "INT-9999", "t-petakan")
		assert.Equal(t, http.StatusNotFound, code)
		w404 := s.call(t, http.MethodPost, "/v1/intervensi/bukan-uuid/persetujuan", emailKelola, map[string]any{"tahap_tujuan": "t-petakan"})
		assert.Equal(t, http.StatusNotFound, w404.Code, "ID salah bentuk")
		assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodPost, "/v1/intervensi/INT-301/persetujuan", emailKelola, map[string]any{}).Code)
	})

	t.Run("kartu selesai tidak dapat diajukan", func(t *testing.T) {
		code, _, _ := s.ajukan(t, emailKelola, "INT-319", "s-selesai")
		assert.Equal(t, http.StatusConflict, code)
	})

	t.Run("satu permintaan terbuka per kartu", func(t *testing.T) {
		code, _, body := s.ajukan(t, emailKelola, "INT-301", "t-petakan")
		require.Equal(t, http.StatusCreated, code, body)
		code, _, _ = s.ajukan(t, emailKelola, "INT-301", "t-selesai")
		assert.Equal(t, http.StatusConflict, code)
	})
}

func TestPersetujuan_EskalasiOpensReviewAndRejectionClearsTheFlag(t *testing.T) {
	s := newSuite(t, "intervensi_setuju_eskalasi")
	w := s.call(t, http.MethodPost, "/v1/intervensi/bulk", emailKelola, map[string]any{"action": "eskalasi", "ids": []string{kid("INT-302"), kid("INT-303")}})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var eskalasi []persetujuanRow
	for _, p := range s.daftarPersetujuan(t, "?status=MENUNGGU") {
		if p.Jenis == "ESKALASI" {
			eskalasi = append(eskalasi, p)
		}
	}
	require.Len(t, eskalasi, 2, "tiap kartu yang dieskalasi masuk antrean telaah Dewan Pengarah")
	assert.Nil(t, eskalasi[0].TahapTujuan)
	assert.Equal(t, "intervensi.setujui", eskalasi[0].IzinLangkahKini)

	byKartu := map[string]persetujuanRow{}
	for _, p := range eskalasi {
		byKartu[p.IntervensiID] = p
	}
	require.Contains(t, byKartu, kid("INT-302"))
	require.Contains(t, byKartu, kid("INT-303"))

	code, accepted, body := s.putus(t, emailSetuju, byKartu[kid("INT-302")].ID, "SETUJU", "Eskalasi diterima")
	require.Equal(t, http.StatusOK, code, body)
	assert.Equal(t, "DISETUJUI", accepted.Status)
	assert.NotNil(t, card(t, s, "INT-302").DieskalasiPada, "eskalasi yang diterima tetap tercatat pada kartu")
	assert.Equal(t, "t-ews", card(t, s, "INT-302").Tahap, "persetujuan eskalasi tidak memindahkan kartu")

	versi := card(t, s, "INT-303").Versi
	code, _, body = s.putus(t, emailSetuju, byKartu[kid("INT-303")].ID, "TOLAK", "Belum perlu eskalasi")
	require.Equal(t, http.StatusOK, code, body)
	k := card(t, s, "INT-303")
	assert.Nil(t, k.DieskalasiPada, "eskalasi yang ditolak dicabut")
	assert.Equal(t, versi+1, k.Versi)
}

func TestPersetujuan_ConcurrentDecisionsOnlyOneWins(t *testing.T) {
	s := newSuite(t, "intervensi_setuju_serentak")
	_, p, _ := s.ajukan(t, emailKelola, "INT-301", "t-petakan")

	codes := make([]int, 2)
	var wg sync.WaitGroup
	for i, email := range []string{emailSetuju, emailSetuju2} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes[i], _, _ = s.putus(t, email, p.ID, "SETUJU", "")
		}()
	}
	wg.Wait()

	assert.ElementsMatch(t, []int{http.StatusOK, http.StatusConflict}, codes)
	assert.Equal(t, "t-petakan", card(t, s, "INT-301").Tahap)
	var moves int
	require.NoError(t, s.db.Raw(`SELECT COUNT(*) FROM intervensi_riwayat WHERE intervensi_id = ?::uuid AND aksi = 'PINDAH_TAHAP' AND ke = 't-petakan'`, kid("INT-301")).Scan(&moves).Error)
	assert.Equal(t, 1, moves, "kartu dipindahkan sekali saja")
}

func TestPersetujuan_RequesterCannotApproveOwnRequest(t *testing.T) {
	s := newSuite(t, "intervensi_setuju_sendiri")
	_, p, _ := s.ajukan(t, emailKelola, "INT-301", "t-petakan")
	require.NoError(t, s.db.Exec(`UPDATE intervensi_persetujuan SET diajukan_oleh = garda_uuid_stabil('pengguna', 'strategis_bi') WHERE id = ?`, p.ID).Error)

	code, _, body := s.putus(t, emailSetuju, p.ID, "SETUJU", "")

	assert.Equal(t, http.StatusForbidden, code)
	assert.Contains(t, body, "Pemisahan tugas")
	assert.Equal(t, "t-ews", card(t, s, "INT-301").Tahap)
}

func TestPersetujuan_ListAndDecisionAuthorization(t *testing.T) {
	s := sharedSuite(t)
	assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodGet, "/v1/intervensi/persetujuan", "", nil).Code)
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, "/v1/intervensi/persetujuan", emailTanpaIzin, nil).Code)
	assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodGet, "/v1/intervensi/persetujuan?status=ACAK", emailLihat, nil).Code)

	body := map[string]any{"keputusan": "SETUJU"}
	assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodPost, "/v1/intervensi/persetujuan/1/keputusan", "", body).Code)
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPost, "/v1/intervensi/persetujuan/1/keputusan", emailKelola, body).Code, "kelola saja tidak cukup")
	assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodPost, "/v1/intervensi/persetujuan/99999/keputusan", emailSetuju, body).Code)
	for _, bukanID := range []string{"abc", "0", "-1", "01", "1.5"} {
		assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodPost, "/v1/intervensi/persetujuan/"+bukanID+"/keputusan", emailSetuju, body).Code, "ID salah bentuk dijawab 404: "+bukanID)
	}
	assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodPost, "/v1/intervensi/persetujuan/1/keputusan", emailSetuju, map[string]any{"keputusan": "MUNGKIN"}).Code)
}
