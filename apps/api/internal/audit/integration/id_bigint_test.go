package integration

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// audit_log.id adalah BIGINT identity (migrasi 000115): JSON menulisnya sebagai string desimal, urutan
// (waktu, id) numerik, dan rute detail menerima ID atau kode (ID lama) sebagai alias.

// seedSerentak menulis n baris berwaktu sama, sehingga urutan hanya ditentukan ID: ID melewati batas 9 -> 10.
func (s *suite) seedSerentak(t *testing.T, n int) {
	t.Helper()
	waktu := time.Date(2026, 4, 1, 8, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		require.NoError(t, s.writer.Write(t.Context(), audit.Entry{
			AktorID: uid("admin-1"), Aksi: "UJI_SERENTAK", Entitas: "uji", EntitasID: "serentak",
			Hasil: audit.HasilBerhasil, Waktu: waktu,
		}))
	}
}

func TestAuditID_JSONStringDesimalDanUrutanNumerik(t *testing.T) {
	s := newSuite(t, "audit_id_numerik")
	s.seedSerentak(t, 12)
	token := s.login(t, adminEmail)

	w := s.call(t, http.MethodGet, "/v1/audit?aksi=UJI_SERENTAK&limit=200", token, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var mentah struct {
		Data struct {
			Items []map[string]any `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &mentah))
	require.Len(t, mentah.Data.Items, 12)

	var ids []int
	for _, item := range mentah.Data.Items {
		teks, ok := item["id"].(string)
		require.True(t, ok, "id ditulis sebagai string JSON, bukan angka: %v", item["id"])
		n, err := strconv.Atoi(teks)
		require.NoError(t, err, "id berupa angka desimal: %s", teks)
		ids = append(ids, n)
		assert.NotContains(t, item, "kode", "baris baru tidak punya kode (ID lama)")
	}
	for i := 1; i < len(ids); i++ {
		assert.Greater(t, ids[i-1], ids[i], "waktu sama: ID menurun secara numerik (10 mendahului 9), bukan leksikografis")
	}
}

func TestAuditID_PaginasiCursorMelewatiBatasDigit(t *testing.T) {
	s := newSuite(t, "audit_id_cursor")
	s.seedSerentak(t, 12)
	token := s.login(t, adminEmail)

	var seen []string
	cursor := ""
	for halaman := 0; ; halaman++ {
		require.Less(t, halaman, 20, "paginasi tidak berhenti")
		query := "?aksi=UJI_SERENTAK&limit=3"
		if cursor != "" {
			query += "&cursor=" + cursor
		}
		code, p := s.listAudit(t, token, query)
		require.Equal(t, http.StatusOK, code)
		for _, item := range p.Items {
			seen = append(seen, item.ID)
		}
		if p.NextCursor == "" {
			break
		}
		cursor = p.NextCursor
	}

	require.Len(t, seen, 12, "tiap baris tepat sekali: tidak ada yang terlewat atau dobel di batas ID 9 dan 10")
	assert.Len(t, uniq(seen), 12)
}

func uniq(v []string) map[string]bool {
	m := map[string]bool{}
	for _, x := range v {
		m[x] = true
	}
	return m
}

func TestAuditID_DetailByIDDanKode(t *testing.T) {
	s := newSuite(t, "audit_id_detail")
	token := s.login(t, adminEmail)
	idLama := "0a1b2c3d-1111-4222-8333-444455556666"
	require.NoError(t, s.db.Exec(`
		INSERT INTO audit_log (kode, aksi, entitas, entitas_id, hasil, sebelum, sesudah)
		VALUES (?, 'UJI_ALIAS', 'uji', '1', 'BERHASIL', '{"a":1}', '{"a":2}')`, idLama).Error)
	var id string
	require.NoError(t, s.db.Raw(`SELECT id::text FROM audit_log WHERE kode = ?`, idLama).Scan(&id).Error)
	require.NotEmpty(t, id)

	t.Run("lewat ID BIGINT", func(t *testing.T) {
		w := s.call(t, http.MethodGet, "/v1/audit/"+id, token, nil)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var body struct {
			Data struct {
				ID   string  `json:"id"`
				Kode *string `json:"kode"`
				Aksi string  `json:"aksi"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Equal(t, id, body.Data.ID)
		require.NotNil(t, body.Data.Kode)
		assert.Equal(t, idLama, *body.Data.Kode)
		assert.Equal(t, "UJI_ALIAS", body.Data.Aksi)
	})

	t.Run("lewat kode (ID lama) sebagai alias", func(t *testing.T) {
		w := s.call(t, http.MethodGet, "/v1/audit/"+idLama, token, nil)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), `"id":"`+id+`"`)
	})

	t.Run("ID salah bentuk atau tidak ada dijawab 404", func(t *testing.T) {
		for _, ref := range []string{
			"0", "-5", "007", "1.5", "9999999999999999999999", "bukan-id", "tidak-ada",
			strings.Repeat("a", 65), // lebih panjang dari kolom kode
		} {
			w := s.call(t, http.MethodGet, "/v1/audit/"+ref, token, nil)
			assert.Equal(t, http.StatusNotFound, w.Code, ref)
		}
		// ID yang sah tetapi tidak ada.
		w := s.call(t, http.MethodGet, "/v1/audit/987654321", token, nil)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("angka murni selalu dibaca sebagai ID, bukan kode", func(t *testing.T) {
		require.NoError(t, s.db.Exec(`INSERT INTO audit_log (kode, aksi, entitas, entitas_id, hasil) VALUES ('555', 'UJI_ANGKA', 'uji', '1', 'BERHASIL')`).Error)
		w := s.call(t, http.MethodGet, "/v1/audit/555", token, nil)
		assert.Equal(t, http.StatusNotFound, w.Code, "ID 555 tidak ada; kode '555' tidak pernah dicari lewat rute")
	})
}

func TestAuditID_TetapAppendOnly(t *testing.T) {
	s := newSuite(t, "audit_id_append_only")
	s.seedSerentak(t, 2)

	err := s.db.Exec(`UPDATE audit_log SET aksi = 'DIUBAH' WHERE aksi = 'UJI_SERENTAK'`).Error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "append-only")
	err = s.db.Exec(`DELETE FROM audit_log WHERE aksi = 'UJI_SERENTAK'`).Error
	require.Error(t, err)
	err = s.db.Exec(`TRUNCATE audit_log`).Error
	require.Error(t, err)

	err = s.db.Exec(`INSERT INTO audit_log (id, aksi, entitas, entitas_id, hasil) VALUES (999, 'X', 'x', '1', 'BERHASIL')`).Error
	require.Error(t, err, "id GENERATED ALWAYS: basis data satu-satunya pemberi ID")
}

// Ekspor menelusuri halaman (waktu, id) per ExportBatch baris. Semua baris berwaktu sama: urutan hanya ditentukan ID
// numerik, jadi tiap baris harus muncul tepat sekali walau melewati batas halaman dan batas digit ID (999 -> 1000).
func TestAuditID_EksporMelewatiBatasHalamanTanpaMenggandakan(t *testing.T) {
	s := newSuite(t, "audit_id_ekspor")
	token := s.login(t, adminEmail)
	require.NoError(t, s.db.Exec(`
		INSERT INTO audit_log (waktu, aksi, entitas, entitas_id, hasil)
		SELECT '2026-04-01 08:00+00', 'UJI_EKSPOR', 'uji', 'e-' || n, 'BERHASIL' FROM generate_series(1, 1100) n`).Error)

	w := s.call(t, http.MethodGet, "/v1/audit/export?aksi=UJI_EKSPOR", token, nil)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "1100", w.Header().Get("X-Export-Rows"))
	baris := parseCSV(t, w.Body.String())
	require.Len(t, baris, 1101, "kepala + 1100 baris")
	kolomEntitasID := -1
	for i, k := range baris[0] {
		if k == "entitas_id" {
			kolomEntitasID = i
		}
	}
	require.GreaterOrEqual(t, kolomEntitasID, 0)
	ada := map[string]bool{}
	for _, b := range baris[1:] {
		assert.False(t, ada[b[kolomEntitasID]], "baris dobel: %s", b[kolomEntitasID])
		ada[b[kolomEntitasID]] = true
	}
	assert.Len(t, ada, 1100)
}
