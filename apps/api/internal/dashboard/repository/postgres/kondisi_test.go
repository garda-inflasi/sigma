package postgres

import (
	"testing"

	"github.com/garda-inflasi/api/internal/dashboard/domain"
	"github.com/stretchr/testify/assert"
)

func TestKondisiAsalAtauTujuan(t *testing.T) {
	tabel := []struct {
		nama     string
		cakupan  domain.Cakupan
		filter   string
		wantSQL  string
		wantArgs []any
	}{
		{"cakupan tertutup: tidak ada baris", domain.Cakupan{}, "", "FALSE", nil},
		{"cakupan tertutup dengan filter: tetap tidak ada baris", domain.Cakupan{}, "1171", "FALSE", nil},
		{"seluruh Aceh tanpa filter", domain.CakupanSemua(), "", "TRUE", nil},
		{"seluruh Aceh dengan filter kabupaten: asal atau tujuan", domain.CakupanSemua(), "1108", "(a = ? OR b = ?)", []any{"1108", "1108"}},
		{"kabupaten tanpa filter: asal atau tujuan", domain.CakupanKabupaten("1171"), "", "(a = ? OR b = ?)", []any{"1171", "1171"}},
		{"kabupaten dengan filter yang sama", domain.CakupanKabupaten("1171"), "1171", "(a = ? OR b = ?)", []any{"1171", "1171"}},
		{"kabupaten dengan filter kabupaten lain: di luar cakupan", domain.CakupanKabupaten("1171"), "1108", "FALSE", nil},
	}
	for _, c := range tabel {
		t.Run(c.nama, func(t *testing.T) {
			sql, args := kondisiAsalAtauTujuan("a", "b", c.cakupan, c.filter)

			assert.Equal(t, c.wantSQL, sql)
			assert.Equal(t, c.wantArgs, args)
		})
	}
}

// Nilai kabupaten selalu menjadi argumen terikat, bukan bagian teks SQL.
func TestKondisiAsalAtauTujuan_KabupatenTidakMasukTeksSQL(t *testing.T) {
	sql, args := kondisiAsalAtauTujuan("a", "b", domain.CakupanSemua(), "1108'; DROP TABLE x;--")

	assert.NotContains(t, sql, "DROP")
	assert.Equal(t, []any{"1108'; DROP TABLE x;--", "1108'; DROP TABLE x;--"}, args)
}
