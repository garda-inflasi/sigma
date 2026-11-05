package seeder

import (
	"os"
	"testing"
	"time"

	authseeder "github.com/garda-inflasi/api/internal/auth/seeder"
	"github.com/garda-inflasi/api/internal/ews/domain"
	masterseeder "github.com/garda-inflasi/api/internal/master/seeder"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var (
	pg      *pgtest.Server
	seedNow = time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
)

func TestMain(m *testing.M) {
	os.Exit(pgtest.Main(m, &pg))
}

func TestBuildAturan_Shape(t *testing.T) {
	aturan := BuildAturan()

	require.Len(t, aturan, 9, "tiga komoditas pilot x tiga jenis")
	ids := map[string]bool{}
	for _, a := range aturan {
		assert.False(t, ids[a.ID], "id ganda %s", a.ID)
		ids[a.ID] = true
		assert.Less(t, a.AmbangWaspada, a.AmbangSiaga, a.ID)
		assert.Less(t, a.AmbangSiaga, a.AmbangKritis, a.ID)
		assert.Positive(t, a.JendelaHari, a.ID)
		assert.True(t, a.Aktif, a.ID)
		assert.Contains(t, a.Keterangan, "dikalibrasi", "%s harus ditandai sebagai ambang awal", a.ID)
		require.NotNil(t, a.KomoditasKode)
	}
	assert.Contains(t, ids, "ATR-LH-CBM-KRT")
}

func TestBuildAlerts_ShapeAndRelativeTime(t *testing.T) {
	alerts := BuildAlerts(seedNow)

	require.Len(t, alerts, 17)
	ids, aturan := map[string]bool{}, map[string]bool{}
	for _, a := range BuildAturan() {
		aturan[a.ID] = true
	}
	for _, a := range alerts {
		assert.False(t, ids[a.ID], "id ganda %s", a.ID)
		ids[a.ID] = true
		assert.Contains(t, []string{"INFO", "WASPADA", "SIAGA", "KRITIS"}, a.Severity)
		assert.Contains(t, []string{"%", "mm"}, a.SatuanNilai)
		assert.True(t, a.DibuatPada.Before(seedNow), "%s harus di masa lalu", a.ID)
		assert.True(t, a.AturanID == nil || aturan[*a.AturanID], "%s merujuk aturan yang tidak ada", a.ID)
		if a.Status == domain.StatusBaru {
			assert.Nil(t, a.DiakuiOlehID, a.ID)
		} else {
			require.NotNil(t, a.DiakuiOlehID, a.ID)
			require.NotNil(t, a.DiakuiPada, a.ID)
			assert.True(t, a.DiakuiPada.After(a.DibuatPada), a.ID)
		}
	}
	assert.Equal(t, seedNow.Add(-4*time.Minute), alerts[0].DibuatPada)
}

func TestBuildAlerts_CoverEverySeverityAndStatus(t *testing.T) {
	severities, statuses := map[string]int{}, map[string]int{}
	for _, a := range BuildAlerts(seedNow) {
		severities[a.Severity]++
		statuses[a.Status]++
	}
	for _, s := range []string{"INFO", "WASPADA", "SIAGA", "KRITIS"} {
		assert.NotZero(t, severities[s], "severity %s", s)
	}
	for _, s := range []string{"BARU", "DIAKUI", "DIESKALASI", "SELESAI"} {
		assert.NotZero(t, statuses[s], "status %s", s)
	}
}

func TestBuildAlerts_SeverityFollowsRuleThresholds(t *testing.T) {
	bySeverity := map[string]string{}
	for _, a := range BuildAlerts(seedNow) {
		bySeverity[a.ID] = a.Severity
	}
	assert.Equal(t, "KRITIS", bySeverity["EWS-1000"], "cabai +21,4% >= kritis 18")
	assert.Equal(t, "WASPADA", bySeverity["EWS-1004"], "cabai +11,3% antara waspada 8 dan siaga 12")
	assert.Equal(t, "WASPADA", bySeverity["EWS-1008"], "beras +4,2% antara waspada 3 dan siaga 5")
	assert.Equal(t, "SIAGA", bySeverity["EWS-0901"], "hujan 148 mm antara siaga 140 dan kritis 200")
	assert.Equal(t, "WASPADA", bySeverity["EWS-0902"], "stok turun 32% antara waspada 25 dan siaga 40")
	assert.Equal(t, "INFO", bySeverity["EWS-1012"], "surplus selalu info")
}

func seededDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db := pg.NewDatabase(t, name)
	pgtest.MigrateUp(t, db)
	_, err := masterseeder.SeedReference(t.Context(), db)
	require.NoError(t, err)
	_, err = authseeder.SeedDemo(t.Context(), db)
	require.NoError(t, err)
	return db
}

func count(t *testing.T, db *gorm.DB, table string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Table(table).Count(&n).Error)
	return n
}

func TestSeed_IsIdempotentAndRoundTripsKanal(t *testing.T) {
	db := seededDB(t, "ews_seed_idempotent")
	ctx := t.Context()

	for range 2 {
		ref, err := SeedReference(ctx, db)
		require.NoError(t, err)
		assert.Equal(t, 9, ref.Aturan)
		demo, err := SeedDemo(ctx, db, seedNow)
		require.NoError(t, err)
		assert.Equal(t, 17, demo.Alert)
	}

	assert.EqualValues(t, 9, count(t, db, "aturan_ews"), "aturan tidak boleh bertambah")
	assert.EqualValues(t, 17, count(t, db, "ews_alert"), "alert tidak boleh bertambah")

	var rule domain.AturanEws
	require.NoError(t, db.First(&rule, "id = ?", "ATR-LH-CBM-KRT").Error)
	assert.Equal(t, domain.Kanal{"DASHBOARD", "PUSH"}, rule.Kanal)
	assert.EqualValues(t, 18, rule.AmbangKritis)
}

func TestSeedReference_KeepsAdminChoices(t *testing.T) {
	db := seededDB(t, "ews_seed_keeps_admin")
	ctx := t.Context()
	_, err := SeedReference(ctx, db)
	require.NoError(t, err)
	require.NoError(t, db.Exec(`UPDATE aturan_ews SET aktif = false, kanal = ARRAY['EMAIL'] WHERE id = 'ATR-LH-BWM'`).Error)

	_, err = SeedReference(ctx, db)
	require.NoError(t, err)

	var rule domain.AturanEws
	require.NoError(t, db.First(&rule, "id = ?", "ATR-LH-BWM").Error)
	assert.False(t, rule.Aktif, "seed reference tidak boleh mengaktifkan ulang aturan yang dimatikan admin")
	assert.Equal(t, domain.Kanal{"EMAIL"}, rule.Kanal)
}

func TestMigration_RejectsMisorderedThresholds(t *testing.T) {
	db := seededDB(t, "ews_check_ambang")

	for name, vals := range map[string][3]float64{
		"waspada = siaga": {5, 5, 9},
		"siaga > kritis":  {5, 9, 8},
		"waspada > siaga": {9, 5, 12},
	} {
		err := db.Exec(`INSERT INTO aturan_ews (id, jenis, ambang_waspada, ambang_siaga, ambang_kritis) VALUES ('X', 'LONJAKAN_HARGA', ?, ?, ?)`,
			vals[0], vals[1], vals[2]).Error
		assert.ErrorContains(t, err, "chk_aturan_ews_ambang", name)
	}
}
