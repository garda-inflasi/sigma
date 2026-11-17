package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Riwayat aktivitas dan sesi akun sendiri (20): selalu milik pemanggil, tanpa snapshot, tanpa identitas orang lain.

type aktivitasItem struct {
	ID        string  `json:"id"`
	Aksi      string  `json:"aksi"`
	Entitas   string  `json:"entitas"`
	EntitasID string  `json:"entitas_id"`
	Hasil     string  `json:"hasil"`
	Sebagai   string  `json:"sebagai"`
	Perangkat string  `json:"perangkat"`
	IP        *string `json:"ip"`
	OlehPeran *string `json:"oleh_peran"`
}

type aktivitasHalaman struct {
	Items      []aktivitasItem `json:"items"`
	NextCursor string          `json:"next_cursor"`
}

func (s *suite) aktivitas(t *testing.T, bearer, query string) (int, aktivitasHalaman, string) {
	t.Helper()
	w := s.call(t, http.MethodGet, "/v1/auth/me/aktivitas"+query, bearer, nil)
	var body struct {
		Data aktivitasHalaman `json:"data"`
	}
	if w.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	}
	return w.Code, body.Data, w.Body.String()
}

func (s *suite) tulis(t *testing.T, e audit.Entry) {
	t.Helper()
	require.NoError(t, s.writer.Write(t.Context(), e))
}

func TestAktivitasSaya_HanyaMilikPemanggilTanpaSnapshotDanIdentitasOrangLain(t *testing.T) {
	s := newSuite(t, "audit_aktivitas_saya")
	viewer := s.login(t, viewerEmail)
	s.login(t, adminEmail)
	// Pelaku: aksi viewer sendiri dengan snapshot dan metadata yang tidak boleh bocor ke respons.
	s.tulis(t, audit.Entry{AktorID: uid("viewer-1"), Aksi: "UPDATE_PROFIL", Entitas: "pengguna", EntitasID: uid("viewer-1"), Hasil: audit.HasilBerhasil,
		Sebelum: map[string]any{"telepon": "+6281200000001"}, Sesudah: map[string]any{"telepon": "+6281200000002"}, Metadata: map[string]any{"rahasia": "x"}})
	// Sasaran: admin mengatur ulang MFA viewer. Aksi admin lain pada akun lain tidak boleh ikut.
	s.tulis(t, audit.Entry{AktorID: uid("admin-1"), Aksi: "RESET_MFA", Entitas: "pengguna", EntitasID: uid("viewer-1"), Hasil: audit.HasilBerhasil})
	s.tulis(t, audit.Entry{AktorID: uid("admin-1"), Aksi: "RESET_MFA", Entitas: "pengguna", EntitasID: uid("target-1"), Hasil: audit.HasilBerhasil})

	code, halaman, body := s.aktivitas(t, viewer, "")

	require.Equal(t, http.StatusOK, code, body)
	require.NotEmpty(t, halaman.Items)
	var login, profil, reset int
	for _, it := range halaman.Items {
		switch {
		case it.Aksi == "LOGIN" && it.Sebagai == "pelaku":
			login++
			assert.NotNil(t, it.IP, "perangkat sendiri tampil")
			assert.NotEmpty(t, it.Perangkat)
		case it.Aksi == "UPDATE_PROFIL":
			profil++
		case it.Aksi == "RESET_MFA":
			reset++
			assert.Equal(t, "sasaran", it.Sebagai)
			assert.Equal(t, uid("viewer-1"), it.EntitasID)
			require.NotNil(t, it.OlehPeran)
			assert.Equal(t, "ADMIN_SISTEM", *it.OlehPeran)
			assert.Nil(t, it.IP, "IP admin tidak ditampilkan")
			assert.Empty(t, it.Perangkat)
		}
	}
	assert.Equal(t, 1, login, "LOGIN viewer sendiri; LOGIN admin tidak ikut")
	assert.Equal(t, 1, profil)
	assert.Equal(t, 1, reset, "hanya reset terhadap akun ini")
	for _, bocor := range []string{"sebelum", "sesudah", "metadata", "rahasia", "+62812000000", "correlation_id", "aktor_nama", "aktor_email", adminEmail, "Nama admin-1"} {
		assert.NotContains(t, body, bocor, "respons tidak boleh memuat %q", bocor)
	}
}

func TestAktivitasSaya_ParameterPemilihAkunDiabaikan(t *testing.T) {
	s := newSuite(t, "audit_aktivitas_spoof")
	viewer := s.login(t, viewerEmail)
	s.login(t, adminEmail)
	_, polos, _ := s.aktivitas(t, viewer, "")

	for _, q := range []string{
		"?aktor=" + uid("admin-1"), "?pengguna_id=" + uid("admin-1"), "?user_id=" + uid("admin-1"),
		"?aktor=" + url.QueryEscape(adminEmail), "?q=admin", "?peran=ADMIN_SISTEM", "?correlation_id=x",
	} {
		code, got, body := s.aktivitas(t, viewer, q)
		require.Equal(t, http.StatusOK, code, q+body)
		assert.Equal(t, polos, got, "%s tidak boleh mengubah cakupan", q)
	}
}

func TestAktivitasSaya_FilterDanValidasi(t *testing.T) {
	s := newSuite(t, "audit_aktivitas_filter")
	viewer := s.login(t, viewerEmail)
	s.tulis(t, audit.Entry{AktorID: uid("viewer-1"), Aksi: "HARGA_VALIDASI", Entitas: "entri_harga", EntitasID: "1 entri", Hasil: audit.HasilDitolak})
	s.tulis(t, audit.Entry{AktorID: uid("viewer-1"), Aksi: "HARGA_VALIDASI", Entitas: "entri_harga", EntitasID: "2 entri", Hasil: audit.HasilBerhasil})
	s.tulis(t, audit.Entry{AktorID: uid("admin-1"), Aksi: "RESET_MFA", Entitas: "pengguna", EntitasID: uid("viewer-1"), Hasil: audit.HasilBerhasil})

	t.Run("aksi dan hasil", func(t *testing.T) {
		_, got, _ := s.aktivitas(t, viewer, "?aksi=HARGA_VALIDASI")
		assert.Len(t, got.Items, 2)
		_, got, _ = s.aktivitas(t, viewer, "?aksi=HARGA_VALIDASI&hasil=DITOLAK")
		require.Len(t, got.Items, 1)
		assert.Equal(t, "1 entri", got.Items[0].EntitasID)
	})
	t.Run("sebagai", func(t *testing.T) {
		_, pelaku, _ := s.aktivitas(t, viewer, "?sebagai=pelaku")
		for _, it := range pelaku.Items {
			assert.Equal(t, "pelaku", it.Sebagai)
		}
		_, sasaran, _ := s.aktivitas(t, viewer, "?sebagai=sasaran")
		require.Len(t, sasaran.Items, 1)
		assert.Equal(t, "RESET_MFA", sasaran.Items[0].Aksi)
	})
	t.Run("rentang tanggal", func(t *testing.T) {
		_, got, _ := s.aktivitas(t, viewer, "?dari=2999-01-01")
		assert.Empty(t, got.Items)
		_, got, _ = s.aktivitas(t, viewer, "?dari=2000-01-01&sampai=2999-12-31")
		assert.NotEmpty(t, got.Items)
	})
	t.Run("parameter salah dijawab 400", func(t *testing.T) {
		for _, q := range []string{"?sebagai=admin", "?hasil=SUKSES", "?dari=kemarin", "?sampai=besok", "?cursor=bukan-cursor", "?limit=abc", "?limit=-1"} {
			code, _, body := s.aktivitas(t, viewer, q)
			assert.Equal(t, http.StatusBadRequest, code, q+" "+body)
		}
	})
	t.Run("tanpa token 401", func(t *testing.T) {
		code, _, _ := s.aktivitas(t, "", "")
		assert.Equal(t, http.StatusUnauthorized, code)
	})
}

func TestAktivitasSaya_PaginasiCursorMelewatiSemuaBarisSekali(t *testing.T) {
	s := newSuite(t, "audit_aktivitas_paginasi")
	viewer := s.login(t, viewerEmail)
	for i := range 25 {
		s.tulis(t, audit.Entry{AktorID: uid("viewer-1"), Aksi: "HARGA_AJUKAN", Entitas: "entri_harga", EntitasID: fmt.Sprintf("%d entri", i), Hasil: audit.HasilBerhasil})
	}
	var total int
	require.NoError(t, s.db.Raw(`SELECT count(*) FROM audit_log WHERE aktor_id = ?::uuid OR (entitas = 'pengguna' AND entitas_id = ?)`, uid("viewer-1"), uid("viewer-1")).Scan(&total).Error)

	seen, cursor, halaman := map[string]bool{}, "", 0
	for {
		q := "?limit=10"
		if cursor != "" {
			q += "&cursor=" + url.QueryEscape(cursor)
		}
		code, got, body := s.aktivitas(t, viewer, q)
		require.Equal(t, http.StatusOK, code, body)
		halaman++
		require.LessOrEqual(t, len(got.Items), 10)
		for _, it := range got.Items {
			require.False(t, seen[it.ID], "baris %s muncul dua kali", it.ID)
			seen[it.ID] = true
		}
		if got.NextCursor == "" {
			break
		}
		cursor = got.NextCursor
		require.Less(t, halaman, 20, "paginasi tidak berhenti")
	}
	assert.Len(t, seen, total)
	assert.GreaterOrEqual(t, halaman, 3)
}

type sesiItem struct {
	ID        string  `json:"id"`
	Perangkat string  `json:"perangkat"`
	Status    string  `json:"status"`
	SaatIni   bool    `json:"saat_ini"`
	Berakhir  *string `json:"berakhir_pada"`
	IP        string  `json:"ip"`
}

type sesiHalaman struct {
	Items      []sesiItem `json:"items"`
	NextCursor string     `json:"next_cursor"`
}

func (s *suite) riwayatSesi(t *testing.T, bearer, query string) (int, sesiHalaman) {
	t.Helper()
	w := s.call(t, http.MethodGet, "/v1/auth/me/riwayat-sesi"+query, bearer, nil)
	var body struct {
		Data sesiHalaman `json:"data"`
	}
	if w.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	}
	return w.Code, body.Data
}

func TestRiwayatSesi_AktifDanBerakhirHanyaMilikSendiri(t *testing.T) {
	s := newSuite(t, "audit_riwayat_sesi")
	s.login(t, viewerEmail)
	kedua := s.login(t, viewerEmail)
	s.login(t, adminEmail)

	code, got := s.riwayatSesi(t, kedua, "")
	require.Equal(t, http.StatusOK, code)
	require.Len(t, got.Items, 2, "dua sesi viewer; sesi admin tidak ikut")
	assert.True(t, got.Items[0].SaatIni, "sesi terbaru adalah sesi ini")
	assert.False(t, got.Items[1].SaatIni)
	for _, it := range got.Items {
		assert.Equal(t, "AKTIF", it.Status)
		assert.Nil(t, it.Berakhir)
		assert.NotEmpty(t, it.Perangkat)
	}

	// Cabut sesi lain: sesi pertama berakhir sebagai DICABUT.
	require.Equal(t, http.StatusOK, s.call(t, http.MethodPost, "/v1/auth/sessions/revoke-others", kedua, nil).Code)

	_, semua := s.riwayatSesi(t, kedua, "")
	require.Len(t, semua.Items, 2)
	assert.Equal(t, "AKTIF", semua.Items[0].Status)
	assert.Equal(t, "DICABUT", semua.Items[1].Status)
	assert.NotNil(t, semua.Items[1].Berakhir)

	_, aktif := s.riwayatSesi(t, kedua, "?status=aktif")
	require.Len(t, aktif.Items, 1)
	assert.True(t, aktif.Items[0].SaatIni)
	_, berakhir := s.riwayatSesi(t, kedua, "?status=berakhir")
	require.Len(t, berakhir.Items, 1)
	assert.Equal(t, "DICABUT", berakhir.Items[0].Status)
}

func TestRiwayatSesi_KedaluwarsaPaginasiDanValidasi(t *testing.T) {
	s := newSuite(t, "audit_riwayat_sesi_halaman")
	s.login(t, viewerEmail)
	s.login(t, viewerEmail)
	token := s.login(t, viewerEmail)
	// Sesi tertua kedaluwarsa tanpa dicabut.
	require.NoError(t, s.db.Exec(`UPDATE refresh_tokens SET expires_at = now() - interval '1 hour'
		WHERE family_id = (SELECT family_id FROM refresh_tokens WHERE user_id = ?::uuid ORDER BY session_started_at LIMIT 1)`, uid("viewer-1")).Error)

	var urutan []sesiItem
	cursor := ""
	for range 5 {
		q := "?limit=1"
		if cursor != "" {
			q += "&cursor=" + url.QueryEscape(cursor)
		}
		code, got := s.riwayatSesi(t, token, q)
		require.Equal(t, http.StatusOK, code)
		urutan = append(urutan, got.Items...)
		if got.NextCursor == "" {
			break
		}
		cursor = got.NextCursor
	}
	require.Len(t, urutan, 3, "tiga sesi, satu per halaman, tanpa duplikat")
	assert.Equal(t, "AKTIF", urutan[0].Status)
	assert.Equal(t, "KEDALUWARSA", urutan[2].Status, "sesi tertua lewat masa berlaku")

	for _, q := range []string{"?status=semua2", "?cursor=rusak", "?limit=-5"} {
		code, _ := s.riwayatSesi(t, token, q)
		assert.Equal(t, http.StatusBadRequest, code, q)
	}
	code, _ := s.riwayatSesi(t, "", "")
	assert.Equal(t, http.StatusUnauthorized, code)
}
