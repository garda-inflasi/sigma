package smoketest

import (
	"net/http"
	"os"
	"sort"
	"testing"

	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestMain(m *testing.M) {
	code := pgtest.Main(m, &pg)
	removeSeedBinary()
	os.Exit(code)
}

// smokeCase adalah satu endpoint utama web yang harus mengembalikan data setelah
// seed reference + demo. Modul menambahkannya lewat registerSmoke di
// smoke_<modul>_test.go.
type smokeCase struct {
	// Name unik, mis. "coldstorage: daftar stok".
	Name           string
	Method, Path   string
	Email          string         // akun demo berperan berhak: harus 200
	MinItems       map[string]int // path larik di data -> jumlah minimal ("" = data itu sendiri)
	ForbiddenEmail string         // akun demo tak berhak: harus 403; kosong = tidak diuji
}

var smokeCases = map[string]smokeCase{}

func registerSmoke(cases ...smokeCase) {
	for _, c := range cases {
		if _, exists := smokeCases[c.Name]; exists {
			panic("smoketest: kasus " + c.Name + " terdaftar dua kali")
		}
		smokeCases[c.Name] = c
	}
}

func TestSmoke_EndpointsReturnSeededData(t *testing.T) {
	s := shared(t)
	require.NotEmpty(t, smokeCases, "tidak ada kasus asap terdaftar")

	names := make([]string, 0, len(smokeCases))
	for name := range smokeCases {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		c := smokeCases[name]
		t.Run(name, func(t *testing.T) {
			w := s.do(t, c.Method, c.Path, s.login(t, c.Email), nil)
			require.Equal(t, http.StatusOK, w.Code, "peran berhak (%s) harus 200: %s", c.Email, w.Body.String())

			data := readEnvelope(t, w).Data
			for path, min := range c.MinItems {
				assert.GreaterOrEqual(t, countItems(t, data, path), min, "larik %q di data kurang dari %d item", path, min)
			}

			if c.ForbiddenEmail != "" {
				denied := s.do(t, c.Method, c.Path, s.login(t, c.ForbiddenEmail), nil)
				assert.Equal(t, http.StatusForbidden, denied.Code, "peran tak berhak (%s) harus 403: %s", c.ForbiddenEmail, denied.Body.String())
			}

			assert.Equal(t, http.StatusUnauthorized, s.do(t, c.Method, c.Path, "", nil).Code, "tanpa token harus 401")
		})
	}
}

func TestSeed_IsIdempotent(t *testing.T) {
	db := pg.NewDatabase(t, "smoke_idempotent")
	pgtest.MigrateUp(t, db)

	seedAll(t, "smoke_idempotent")
	first := rowCounts(t, db)
	seedAll(t, "smoke_idempotent")
	second := rowCounts(t, db)

	assert.Equal(t, first, second, "menjalankan seed dua kali tidak boleh mengubah jumlah baris")
	assert.NotZero(t, first["cold_storage"], "seed demo harus mengisi data")
}

func TestSeed_DemoIsRefusedInProduction(t *testing.T) {
	db := pg.NewDatabase(t, "smoke_production")
	pgtest.MigrateUp(t, db)

	out, err := runSeed(t, "smoke_production", "production", "demo")

	require.Error(t, err, "seed demo di produksi harus gagal")
	assert.Contains(t, out, "APP_ENV=production")
	assert.Zero(t, rowCounts(t, db)["cold_storage"], "tidak ada data demo boleh masuk")

	out, err = runSeed(t, "smoke_production", "production", "reference")
	require.NoError(t, err, "seed reference harus boleh di produksi:\n%s", out)
	assert.NotZero(t, rowCounts(t, db)["cold_storage"])
}

// rowCounts menghitung baris tiap tabel aplikasi.
func rowCounts(t *testing.T, db *gorm.DB) map[string]int64 {
	t.Helper()
	var tables []string
	require.NoError(t, db.Raw(`
		SELECT tablename FROM pg_tables
		 WHERE schemaname = 'public' AND tablename <> 'goose_db_version'`).Scan(&tables).Error)

	counts := make(map[string]int64, len(tables))
	for _, table := range tables {
		var n int64
		require.NoError(t, db.Table(table).Count(&n).Error)
		counts[table] = n
	}
	return counts
}
