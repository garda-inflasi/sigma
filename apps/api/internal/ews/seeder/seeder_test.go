package seeder

import (
	"os"
	"testing"
	"time"

	authseeder "github.com/garda-inflasi/api/internal/auth/seeder"
	"github.com/garda-inflasi/api/internal/ews/domain"
	masterseeder "github.com/garda-inflasi/api/internal/master/seeder"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/pkg/idx"
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
	kodes := map[string]bool{}
	for _, a := range aturan {
		require.NotNil(t, a.Kode)
		kode := *a.Kode
		assert.Zero(t, a.ID, "ID dibuat basis data: %s", kode)
		assert.False(t, kodes[kode], "kode ganda %s", kode)
		kodes[kode] = true
		assert.Less(t, a.AmbangWaspada, a.AmbangSiaga, kode)
		assert.Less(t, a.AmbangSiaga, a.AmbangKritis, kode)
		assert.Positive(t, a.JendelaHari, kode)
		assert.True(t, a.Aktif, kode)
		assert.Contains(t, a.Keterangan, "dikalibrasi", "%s harus ditandai sebagai ambang awal", kode)
		require.NotNil(t, a.KomoditasKode)
	}
	assert.Contains(t, kodes, "ATR-LH-CBM-KRT")
}

func TestBuildAlerts_ShapeAndRelativeTime(t *testing.T) {
	alerts := BuildAlerts(seedNow)

	require.Len(t, alerts, 17)
	kodes, aturan := map[string]bool{}, map[string]bool{}
	for _, a := range BuildAturan() {
		aturan[*a.Kode] = true
	}
	for _, a := range alerts {
		require.NotNil(t, a.Kode)
		kode := *a.Kode
		assert.Zero(t, a.ID, "ID dibuat basis data: %s", kode)
		assert.False(t, kodes[kode], "kode ganda %s", kode)
		kodes[kode] = true
		assert.Contains(t, []string{"INFO", "WASPADA", "SIAGA", "KRITIS"}, a.Severity)
		assert.Contains(t, []string{"%", "mm"}, a.SatuanNilai)
		assert.True(t, a.DibuatPada.Before(seedNow), "%s harus di masa lalu", kode)
		if ak, ada := AturanKodeAlert(a); ada {
			assert.True(t, aturan[ak], "%s merujuk aturan %s yang tidak ada", kode, ak)
		}
		if a.Status == domain.StatusBaru {
			assert.Nil(t, a.DiakuiOlehID, kode)
		} else {
			require.NotNil(t, a.DiakuiOlehID, kode)
			require.NotNil(t, a.DiakuiPada, kode)
			assert.True(t, a.DiakuiPada.After(a.DibuatPada), kode)
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
		bySeverity[*a.Kode] = a.Severity
	}
	assert.Equal(t, "KRITIS", bySeverity["EWS-1000"], "cabai +21,4% >= kritis 18")
	assert.Equal(t, "WASPADA", bySeverity["EWS-1004"], "cabai +11,3% antara waspada 8 dan siaga 12")
	assert.Equal(t, "WASPADA", bySeverity["EWS-1008"], "beras +4,2% antara waspada 3 dan siaga 5")
	assert.Equal(t, "SIAGA", bySeverity["EWS-0901"], "hujan 148 mm antara siaga 140 dan kritis 200")
	assert.Equal(t, "WASPADA", bySeverity["EWS-0902"], "stok turun 32% antara waspada 25 dan siaga 40")
	assert.Equal(t, "INFO", bySeverity["EWS-1012"], "surplus selalu info")
}

func TestBuildRiwayat_FollowsEachAlertLifecycle(t *testing.T) {
	alerts := BuildAlerts(seedNow)
	for i := range alerts {
		alerts[i].ID = idx.ID(i + 1) // BuildRiwayat dipanggil sesudah alert di-insert (ID dari basis data)
	}
	riwayat := BuildRiwayat(alerts)

	byAlert := map[idx.ID][]string{}
	kodes := map[string]bool{}
	for _, r := range riwayat {
		require.NotNil(t, r.Kode)
		assert.False(t, kodes[*r.Kode], "kode ganda %s", *r.Kode)
		kodes[*r.Kode] = true
		byAlert[r.AlertID] = append(byAlert[r.AlertID], r.Ke)
	}
	assert.Contains(t, kodes, "RW-D-EWS-1000-0", "kode riwayat demo = RW-D-<kode alert>-<n>, sama dengan ID lama")
	for _, a := range alerts {
		path := byAlert[a.ID]
		require.NotEmpty(t, path, *a.Kode)
		assert.Equal(t, domain.StatusBaru, path[0], *a.Kode)
		switch a.Status {
		case domain.StatusBaru:
			assert.Len(t, path, 1, *a.Kode)
		case domain.StatusDiakui:
			assert.Equal(t, []string{"BARU", "DIAKUI"}, path, *a.Kode)
		case domain.StatusDieskalasi:
			assert.Equal(t, []string{"BARU", "DIAKUI", "DIESKALASI"}, path, *a.Kode)
			require.NotNil(t, a.DieskalasiPada, *a.Kode)
			assert.True(t, a.DieskalasiPada.After(*a.DiakuiPada), *a.Kode)
		case domain.StatusSelesai:
			assert.Equal(t, []string{"BARU", "DIAKUI", "SELESAI"}, path, *a.Kode)
			require.NotNil(t, a.SelesaiPada, *a.Kode)
		}
	}
}

func TestBuildAturan_StartAtVersionOneWithCooldown(t *testing.T) {
	for _, a := range BuildAturan() {
		assert.Equal(t, 1, a.Versi, *a.Kode)
		assert.Equal(t, 24, a.CooldownJam, *a.Kode)
		assert.Nil(t, a.KabupatenKode, "aturan seed berlaku untuk semua kabupaten")
	}
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
		assert.Equal(t, 31, demo.Riwayat)
	}

	assert.EqualValues(t, 9, count(t, db, "aturan_ews"), "aturan tidak boleh bertambah")
	assert.EqualValues(t, 17, count(t, db, "ews_alert"), "alert tidak boleh bertambah")
	assert.EqualValues(t, 31, count(t, db, "ews_riwayat"), "riwayat tidak boleh bertambah")

	var rule domain.AturanEws
	require.NoError(t, db.First(&rule, "kode = ?", "ATR-LH-CBM-KRT").Error)
	assert.Equal(t, domain.Kanal{"DASHBOARD", "PUSH"}, rule.Kanal)
	assert.EqualValues(t, 18, rule.AmbangKritis)
	assert.Positive(t, rule.ID, "ID dibuat basis data (identity)")
}

// Seed memakai kode sebagai kunci idempotensi: ID baris tidak berubah antar-seed, dan alert, aturan, serta
// riwayat tetap saling menunjuk.
func TestSeed_KeepsRowIDsAndLinksStable(t *testing.T) {
	db := seededDB(t, "ews_seed_stable_ids")
	ctx := t.Context()
	_, err := SeedReference(ctx, db)
	require.NoError(t, err)
	_, err = SeedDemo(ctx, db, seedNow)
	require.NoError(t, err)
	ambil := func() (map[string]int64, map[string]int64) {
		var alert, aturan []struct {
			Kode string
			ID   int64
		}
		require.NoError(t, db.Raw(`SELECT kode, id FROM ews_alert`).Scan(&alert).Error)
		require.NoError(t, db.Raw(`SELECT kode, id FROM aturan_ews`).Scan(&aturan).Error)
		a, r := map[string]int64{}, map[string]int64{}
		for _, x := range alert {
			a[x.Kode] = x.ID
		}
		for _, x := range aturan {
			r[x.Kode] = x.ID
		}
		return a, r
	}
	alert1, aturan1 := ambil()

	_, err = SeedReference(ctx, db)
	require.NoError(t, err)
	_, err = SeedDemo(ctx, db, seedNow)
	require.NoError(t, err)
	alert2, aturan2 := ambil()

	assert.Equal(t, alert1, alert2, "ID alert tidak berubah saat seed diulang")
	assert.Equal(t, aturan1, aturan2, "ID aturan tidak berubah saat seed diulang")

	var rujukan struct {
		AturanKode string
		Riwayat    int64
		Yatim      int64
	}
	require.NoError(t, db.Raw(`
		SELECT (SELECT t.kode FROM ews_alert a JOIN aturan_ews t ON t.id = a.aturan_id WHERE a.kode = 'EWS-1000') AS aturan_kode,
		       (SELECT count(*) FROM ews_riwayat r JOIN ews_alert a ON a.id = r.alert_id WHERE a.kode = 'EWS-1011') AS riwayat,
		       (SELECT count(*) FROM ews_alert WHERE aturan_id IS NULL AND komoditas_kode IN ('CBM-KRT', 'BWM', 'BRS-MED') AND jenis <> 'OVER_SUPPLY') AS yatim`).
		Scan(&rujukan).Error)
	assert.Equal(t, "ATR-LH-CBM-KRT", rujukan.AturanKode, "alert demo merujuk aturan lewat ID basis data")
	assert.EqualValues(t, 3, rujukan.Riwayat, "EWS-1011 dieskalasi: BARU, DIAKUI, DIESKALASI, tidak ganda sesudah seed ulang")
	assert.Zero(t, rujukan.Yatim)
}

// Basis data hasil migrasi sudah memuat baris dengan kode = ID lama; seed tidak menggandakannya dan tidak
// mengubah ID-nya.
func TestSeed_DoesNotDuplicateRowsThatAlreadyHaveTheOldCode(t *testing.T) {
	db := seededDB(t, "ews_seed_migrated")
	ctx := t.Context()
	require.NoError(t, db.Exec(`INSERT INTO aturan_ews (kode, komoditas_kode, jenis, ambang_waspada, ambang_siaga, ambang_kritis)
		VALUES ('ATR-LH-CBM-KRT', 'CBM-KRT', 'LONJAKAN_HARGA', 1, 2, 3)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO ews_alert (kode, severity, jenis, judul, pesan, kabupaten_kode, nilai, satuan_nilai, skor_dampak, sumber)
		VALUES ('EWS-1000', 'INFO', 'LONJAKAN_HARGA', 'Lama', 'p', '1171', 1, '%', 1, 's')`).Error)
	var idAturan, idAlert int64
	require.NoError(t, db.Raw(`SELECT id FROM aturan_ews WHERE kode = 'ATR-LH-CBM-KRT'`).Scan(&idAturan).Error)
	require.NoError(t, db.Raw(`SELECT id FROM ews_alert WHERE kode = 'EWS-1000'`).Scan(&idAlert).Error)

	_, err := SeedReference(ctx, db)
	require.NoError(t, err)
	_, err = SeedDemo(ctx, db, seedNow)
	require.NoError(t, err)

	assert.EqualValues(t, 9, count(t, db, "aturan_ews"))
	assert.EqualValues(t, 17, count(t, db, "ews_alert"))
	var sesudah struct{ Aturan, Alert int64 }
	require.NoError(t, db.Raw(`SELECT (SELECT id FROM aturan_ews WHERE kode = 'ATR-LH-CBM-KRT') AS aturan, (SELECT id FROM ews_alert WHERE kode = 'EWS-1000') AS alert`).Scan(&sesudah).Error)
	assert.Equal(t, idAturan, sesudah.Aturan)
	assert.Equal(t, idAlert, sesudah.Alert)
	var kritis float64
	require.NoError(t, db.Raw(`SELECT ambang_kritis FROM aturan_ews WHERE kode = 'ATR-LH-CBM-KRT'`).Scan(&kritis).Error)
	assert.EqualValues(t, 18, kritis, "aturan versi 1 mengikuti nilai seed")
}

func TestSeedDemo_FailsClearlyWithoutRules(t *testing.T) {
	db := seededDB(t, "ews_seed_tanpa_aturan")

	_, err := SeedDemo(t.Context(), db, seedNow)

	require.Error(t, err)
	assert.ErrorContains(t, err, "ews.reference")
	assert.Zero(t, count(t, db, "ews_alert"), "tidak ada alert setengah jadi")
}

func TestSeedReference_KeepsAdminChoices(t *testing.T) {
	db := seededDB(t, "ews_seed_keeps_admin")
	ctx := t.Context()
	_, err := SeedReference(ctx, db)
	require.NoError(t, err)
	require.NoError(t, db.Exec(`UPDATE aturan_ews SET aktif = false, kanal = ARRAY['EMAIL'] WHERE kode = 'ATR-LH-BWM'`).Error)

	_, err = SeedReference(ctx, db)
	require.NoError(t, err)

	var rule domain.AturanEws
	require.NoError(t, db.First(&rule, "kode = ?", "ATR-LH-BWM").Error)
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
		err := db.Exec(`INSERT INTO aturan_ews (kode, jenis, ambang_waspada, ambang_siaga, ambang_kritis) VALUES ('X', 'LONJAKAN_HARGA', ?, ?, ?)`,
			vals[0], vals[1], vals[2]).Error
		assert.ErrorContains(t, err, "chk_aturan_ews_ambang", name)
	}
}
