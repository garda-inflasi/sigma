package integration

import (
	"strconv"
	"testing"

	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/stretchr/testify/require"
)

// uid memberi UUID akun uji dari slug lamanya (rumus yang sama dengan seed dan migrasi 000110), supaya ID pengguna
// di tes berbentuk UUID seperti di basis data. Slug tetap terbaca di tempat pemakaian.
func uid(slug string) string { return idx.UUIDStabil("pengguna", slug) }

// fasID memberi UUID fasilitas seed dari kodenya (CS-01): rumus yang sama dengan seeder dan migrasi 000113.
func fasID(kode string) string { return idx.UUIDStabil("cold_storage", kode) }

// lotTakAda adalah ID lot yang pasti tidak ada (sah bentuknya, tetapi tidak pernah dibuat).
const lotTakAda = "999999999"

// lot mengembalikan ID BIGINT (string desimal) lot seed menurut kodenya (STK-CS01-05), karena ID lot sekarang
// diberi basis data sedangkan kode seed tetap.
func (s *suite) lot(t *testing.T, kode string) string {
	t.Helper()
	var id int64
	require.NoError(t, s.db.Raw(`SELECT id FROM stok_cold_storage WHERE kode = ?`, kode).Scan(&id).Error)
	require.NotZero(t, id, "lot seed %s tidak ada", kode)
	return strconv.FormatInt(id, 10)
}

// lots seperti lot untuk beberapa kode sekaligus.
func (s *suite) lots(t *testing.T, kode ...string) []string {
	t.Helper()
	out := make([]string, len(kode))
	for i, k := range kode {
		out[i] = s.lot(t, k)
	}
	return out
}

// kodes mengambil kode tampilan tiap baris respons.
func kodes(rows []map[string]any) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i], _ = r["kode"].(string)
	}
	return out
}

// idNum membaca id BIGINT respons (string desimal) sebagai angka, supaya urutan "9" < "10" diperiksa benar.
func idNum(t *testing.T, row map[string]any) int64 {
	t.Helper()
	teks, ok := row["id"].(string)
	require.True(t, ok, "id harus string desimal: %v", row["id"])
	n, err := strconv.ParseInt(teks, 10, 64)
	require.NoError(t, err)
	return n
}
