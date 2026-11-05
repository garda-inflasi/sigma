package integration

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListAudit_AdvancedFilters(t *testing.T) {
	s := newSuite(t, "audit_list_adv_filters")
	s.seedLog(t, 12)
	token := s.login(t, adminEmail)
	require.NoError(t, s.writer.Write(t.Context(), audit.Entry{
		AktorID: "target-1", Aksi: "AKUI_EWS", Entitas: "ews_alert", EntitasID: "EWS-77", Hasil: audit.HasilBerhasil,
	}))

	t.Run("entitas_id persis", func(t *testing.T) {
		_, p := s.listAudit(t, token, "?entitas_id=ENT-03&limit=200")
		require.Len(t, p.Items, 1)
		assert.Equal(t, "ENT-03", p.Items[0].EntitasID)

		_, none := s.listAudit(t, token, "?entitas_id=ENT-0&limit=200")
		assert.Empty(t, none.Items, "entitas_id harus persis, bukan awalan")
	})

	t.Run("peran aktor", func(t *testing.T) {
		_, p := s.listAudit(t, token, "?peran=OPS_MAHASISWA&limit=200")
		require.Len(t, p.Items, 1)
		assert.Equal(t, "EWS-77", p.Items[0].EntitasID)
	})

	t.Run("pencarian bebas pada id entitas, aksi, dan aktor", func(t *testing.T) {
		for query, want := range map[string]int{
			"?q=ews-77&limit=200":       1,
			"?q=akui_ews&limit=200":     1,
			"?q=target@garda&limit=200": 1,
			"?q=tidak-ada-sama-sekali":  0,
		} {
			_, p := s.listAudit(t, token, query)
			assert.Len(t, p.Items, want, query)
		}
	})

	t.Run("pencarian bebas menetralkan wildcard LIKE", func(t *testing.T) {
		_, p := s.listAudit(t, token, "?q=%25&limit=200")
		assert.Empty(t, p.Items, "% harus dicari secara harfiah")
	})

	t.Run("correlation_id mengelompokkan satu permintaan", func(t *testing.T) {
		var cid string
		require.NoError(t, s.db.Raw(`SELECT correlation_id FROM audit_log WHERE aksi = 'LOGIN' LIMIT 1`).Scan(&cid).Error)
		require.NotEmpty(t, cid)

		_, p := s.listAudit(t, token, "?correlation_id="+cid+"&limit=200")
		require.NotEmpty(t, p.Items)
		for _, item := range p.Items {
			assert.Equal(t, "LOGIN", item.Aksi)
		}
	})
}

type detailBody struct {
	Data struct {
		ID        string          `json:"id"`
		Aksi      string          `json:"aksi"`
		Sebelum   json.RawMessage `json:"sebelum"`
		Sesudah   json.RawMessage `json:"sesudah"`
		Perubahan []struct {
			Path    string          `json:"path"`
			Jenis   string          `json:"jenis"`
			Sebelum json.RawMessage `json:"sebelum"`
			Sesudah json.RawMessage `json:"sesudah"`
		} `json:"perubahan"`
	} `json:"data"`
}

func TestGetAudit_DetailWithDiff(t *testing.T) {
	s := newSuite(t, "audit_detail")
	adminToken := s.login(t, adminEmail)
	w := s.call(t, http.MethodPut, "/v1/users/target-1/roles", adminToken, map[string]string{"peran": "OPS_KOORDINATOR"})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var id string
	require.NoError(t, s.db.Raw(`SELECT id FROM audit_log WHERE aksi = 'ASSIGN_ROLE'`).Scan(&id).Error)

	t.Run("peran dengan izin mendapat diff", func(t *testing.T) {
		w := s.call(t, http.MethodGet, "/v1/audit/"+id, adminToken, nil)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())

		var body detailBody
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Equal(t, "ASSIGN_ROLE", body.Data.Aksi)
		require.Len(t, body.Data.Perubahan, 1)
		assert.Equal(t, "peran", body.Data.Perubahan[0].Path)
		assert.Equal(t, "DIUBAH", body.Data.Perubahan[0].Jenis)
		assert.JSONEq(t, `"OPS_MAHASISWA"`, string(body.Data.Perubahan[0].Sebelum))
		assert.JSONEq(t, `"OPS_KOORDINATOR"`, string(body.Data.Perubahan[0].Sesudah))
	})

	t.Run("entri tanpa sebelum/sesudah memberi perubahan kosong, bukan null", func(t *testing.T) {
		var loginID string
		require.NoError(t, s.db.Raw(`SELECT id FROM audit_log WHERE aksi = 'LOGIN' LIMIT 1`).Scan(&loginID).Error)

		w := s.call(t, http.MethodGet, "/v1/audit/"+loginID, adminToken, nil)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"perubahan":[]`)
	})

	t.Run("id tidak ada 404", func(t *testing.T) {
		w := s.call(t, http.MethodGet, "/v1/audit/tidak-ada", adminToken, nil)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("tanpa izin 403, tanpa token 401", func(t *testing.T) {
		w := s.call(t, http.MethodGet, "/v1/audit/"+id, s.login(t, viewerEmail), nil)
		assert.Equal(t, http.StatusForbidden, w.Code)

		w = s.call(t, http.MethodGet, "/v1/audit/"+id, "", nil)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func parseCSV(t *testing.T, raw string) [][]string {
	t.Helper()
	bom := string(rune(0xFEFF))
	require.True(t, strings.HasPrefix(raw, bom), "ekspor harus diawali BOM UTF-8")
	r := csv.NewReader(strings.NewReader(strings.TrimPrefix(raw, bom)))
	r.Comma = ';'
	records, err := r.ReadAll()
	require.NoError(t, err)
	return records
}

func TestExportAudit_FollowsFilterAndIsAudited(t *testing.T) {
	s := newSuite(t, "audit_export")
	s.seedLog(t, 20)
	token := s.login(t, adminEmail)

	w := s.call(t, http.MethodGet, "/v1/audit/export?aksi=PUBLIKASI_HARGA&hasil=BERHASIL", token, nil)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Header().Get("Content-Type"), "text/csv")
	assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment")
	records := parseCSV(t, w.Body.String())
	header, rows := records[0], records[1:]
	assert.Equal(t, "waktu", header[0])
	var want int64
	require.NoError(t, s.db.Raw(`SELECT count(*) FROM audit_log WHERE aksi = 'PUBLIKASI_HARGA' AND hasil = 'BERHASIL'`).Scan(&want).Error)
	require.NotZero(t, want)
	assert.EqualValues(t, want, len(rows))
	for _, row := range rows {
		assert.Equal(t, "PUBLIKASI_HARGA", row[4])
		assert.Equal(t, "BERHASIL", row[7])
	}

	exports := s.rows(t, "EKSPOR_DATA")
	require.Len(t, exports, 1, "ekspor tercatat tepat sekali")
	assert.Equal(t, "admin-1", *exports[0].AktorID)
	assert.Equal(t, "audit_log", exports[0].Entitas)
}

func TestExportAudit_NeutralizesSpreadsheetFormulas(t *testing.T) {
	s := newSuite(t, "audit_export_formula")
	token := s.login(t, adminEmail)
	require.NoError(t, s.writer.Write(t.Context(), audit.Entry{
		AktorID: "admin-1", Aksi: "UJI_RUMUS", Entitas: "uji", EntitasID: `=HYPERLINK("http://jahat")`, Hasil: audit.HasilBerhasil,
	}))

	w := s.call(t, http.MethodGet, "/v1/audit/export?aksi=UJI_RUMUS", token, nil)

	require.Equal(t, http.StatusOK, w.Code)
	records := parseCSV(t, w.Body.String())
	require.Len(t, records, 2)
	assert.Equal(t, `'=HYPERLINK("http://jahat")`, records[1][6])
}

func TestExportAudit_RejectsInvalidFilterAndUnauthorized(t *testing.T) {
	s := newSuite(t, "audit_export_rbac")

	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, "/v1/audit/export", s.login(t, viewerEmail), nil).Code)
	assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodGet, "/v1/audit/export", "", nil).Code)
	assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodGet, "/v1/audit/export?dari=kemarin", s.login(t, adminEmail), nil).Code)
	assert.Empty(t, s.rows(t, "EKSPOR_DATA"), "ekspor yang ditolak tidak dicatat sebagai ekspor")
}
