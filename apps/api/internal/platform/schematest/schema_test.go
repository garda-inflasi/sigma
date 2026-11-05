package schematest

import (
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var pg *pgtest.Server

func TestMain(m *testing.M) {
	os.Exit(pgtest.Main(m, &pg))
}

// fileSuffix adalah bagian nama berkas untuk modul: tanda hubung menjadi garis bawah.
func fileSuffix(module string) string { return strings.ReplaceAll(module, "-", "_") }

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// TestEntitiesMatchMigrations adalah tes kontrak: modul di luar daftar pending
// harus cocok dengan skema; modul pending harus masih terbukti tidak cocok.
func TestEntitiesMatchMigrations(t *testing.T) {
	db := pg.NewDatabase(t, "schema_contract")
	pgtest.MigrateUp(t, db)
	actual := loadSchema(t, db)

	for module := range pendingModules {
		_, registered := registeredModels[module]
		require.True(t, registered, "pending_%s_test.go menandai modul yang tidak punya models_%s_test.go", fileSuffix(module), fileSuffix(module))
	}

	for _, module := range sortedKeys(registeredModels) {
		t.Run(module, func(t *testing.T) {
			problems := compareEntities(registeredModels[module], actual)
			reason, pending := pendingModules[module]

			switch {
			case !pending:
				assert.Empty(t, problems, "entity modul %s tidak cocok dengan migrasi", module)
			case len(problems) == 0:
				assert.Fail(t, "daftar pending basi",
					"modul %s sudah cocok dengan migrasi; hapus pending_%s_test.go", module, fileSuffix(module))
			default:
				t.Skipf("BELUM SELARAS (%s): %d ketidakcocokan, mis. %s", reason, len(problems), problems[0])
			}
		})
	}
}

func TestMigrationsApplyUpDownUp(t *testing.T) {
	db := pg.NewDatabase(t, "migrate_updownup")
	migrator := pgtest.Migrator(t, db)

	require.NoError(t, migrator.Up(t.Context()), "up pertama")
	latest, err := migrator.Version(t.Context())
	require.NoError(t, err)
	require.Positive(t, latest)

	require.NoError(t, migrator.DownTo(t.Context(), 0), "down sampai kosong")
	var tables []string
	require.NoError(t, db.Raw(`
		SELECT table_name FROM information_schema.tables
		 WHERE table_schema = 'public' AND table_name <> 'goose_db_version'`).Scan(&tables).Error)
	assert.Empty(t, tables, "down penuh harus menghapus semua tabel aplikasi: %s", strings.Join(tables, ", "))

	require.NoError(t, migrator.Up(t.Context()), "up kedua")
	again, err := migrator.Version(t.Context())
	require.NoError(t, err)
	assert.Equal(t, latest, again)
}
