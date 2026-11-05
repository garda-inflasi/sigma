package integration

import (
	"encoding/csv"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func bacaCSV(t *testing.T, raw string) [][]string {
	t.Helper()
	bom := string(rune(0xFEFF))
	require.True(t, strings.HasPrefix(raw, bom), "laporan harus diawali BOM UTF-8")
	r := csv.NewReader(strings.NewReader(strings.TrimPrefix(raw, bom)))
	r.Comma = ';'
	records, err := r.ReadAll()
	require.NoError(t, err)
	return records
}

func TestKPIExport_Authorization(t *testing.T) {
	s := sharedSuite(t)

	assert.Equal(t, http.StatusUnauthorized, s.get(t, "/v1/kpi/export", "").Code)
	assert.Equal(t, http.StatusForbidden, s.get(t, "/v1/kpi/export", emailTanpaIzn).Code)
	assert.Equal(t, http.StatusOK, s.get(t, "/v1/kpi/export", emailLihat).Code)
}

func TestKPIExport_ReportMatchesListAndIsAudited(t *testing.T) {
	s := sharedSuite(t)
	var sebelum int64
	require.NoError(t, s.db.Raw(`SELECT COUNT(*) FROM audit_log WHERE aksi = 'EKSPOR_DATA' AND entitas = 'kpi_nilai'`).Scan(&sebelum).Error)

	w := s.get(t, "/v1/kpi/export", emailLihat)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Header().Get("Content-Type"), "text/csv")
	assert.Contains(t, w.Header().Get("Content-Disposition"), "laporan-kpi-")
	records := bacaCSV(t, w.Body.String())
	header, rows := records[0], records[1:]
	assert.Equal(t, []string{"kelompok", "kode", "nama", "satuan", "arah_baik", "otomatis", "periode_nilai", "nilai", "target", "ambang_waspada", "baseline", "status"}, header)

	daftarKPI := daftar(t, s.get(t, "/v1/kpi", emailLihat))
	require.Len(t, rows, len(daftarKPI), "satu baris per KPI")
	by := map[string][]string{}
	for _, row := range rows {
		by[row[1]] = row
	}
	util := by["LOG-UTILISASI-CS"]
	assert.Equal(t, "LOGISTIK", util[0])
	assert.Equal(t, "true", util[5])
	assert.NotEmpty(t, util[6], "periode nilai")
	assert.NotEmpty(t, util[7], "nilai terhitung dari buku besar cold storage demo")
	assert.Equal(t, "75", util[8], "target PRD")
	assert.Empty(t, util[10], "baseline menunggu keputusan pemangku kepentingan")
	assert.Equal(t, "TERCAPAI", util[11])

	menunggu := by["DATA-F1-EWS"]
	assert.Equal(t, "false", menunggu[5])
	assert.Empty(t, menunggu[6], "KPI tanpa nilai: kolom nilai kosong, bukan 0")
	assert.Empty(t, menunggu[7])
	assert.Equal(t, "MENUNGGU_DATA", menunggu[11])

	var sesudah int64
	require.NoError(t, s.db.Raw(`SELECT COUNT(*) FROM audit_log WHERE aksi = 'EKSPOR_DATA' AND entitas = 'kpi_nilai'`).Scan(&sesudah).Error)
	assert.Equal(t, sebelum+1, sesudah, "ekspor tercatat tepat sekali")
}

func TestKPIExport_PeriodAndRegionFilters(t *testing.T) {
	s := sharedSuite(t)

	lampau := bacaCSV(t, s.get(t, "/v1/kpi/export?periode=2001-01", emailLihat).Body.String())
	for _, row := range lampau[1:] {
		assert.Empty(t, row[7], "%s: tak ada nilai pada 2001", row[1])
	}

	wilayah := bacaCSV(t, s.get(t, "/v1/kpi/export?wilayah=1171", emailLihat).Body.String())
	for _, row := range wilayah[1:] {
		assert.Empty(t, row[7], "%s: nilai provinsi tidak bocor ke tampilan wilayah", row[1])
	}

	assert.Equal(t, http.StatusBadRequest, s.get(t, "/v1/kpi/export?periode=bulan-ini", emailLihat).Code)
	assert.Equal(t, http.StatusBadRequest, s.get(t, "/v1/kpi/export?wilayah=abc", emailLihat).Code)
}
