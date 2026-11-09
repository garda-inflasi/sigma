package integration

import (
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/pkg/seed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Tes kontrak seed EWS: reference/07_ews_aturan.sql dan demo/06_ews_alerts.sql. Seeder Go lama (internal/ews/seeder)
// punya tes bentuk data, idempotensi, dan ID stabil; tes itu ikut terhapus bersama paketnya, jadi kontraknya dijaga
// di sini terhadap PostgreSQL sungguhan. Yang diperiksa hanya isi seed; perilaku API ada di berkas tes lain.

func (s *suite) hitung(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, s.db.Raw(sql, args...).Scan(&n).Error)
	return n
}

// idPerKode memetakan kode (EWS-n / ATR-...) ke ID BIGINT baris yang dibuat basis data.
func (s *suite) idPerKode(t *testing.T, tabel string) map[string]int64 {
	t.Helper()
	var rows []struct {
		Kode string
		ID   int64
	}
	require.NoError(t, s.db.Raw(`SELECT kode, id FROM `+tabel+` WHERE kode IS NOT NULL`).Scan(&rows).Error)
	out := make(map[string]int64, len(rows))
	for _, r := range rows {
		out[r.Kode] = r.ID
	}
	return out
}

func TestSeed_DemoWorldIsTheSeventeenAlertsWithTheirHistory(t *testing.T) {
	s := newSuite(t, "ews_seed_dunia")

	assert.Equal(t, 9, s.hitung(t, `SELECT COUNT(*) FROM aturan_ews`), "tiga komoditas pilot x tiga jenis")
	assert.Equal(t, 17, s.hitung(t, `SELECT COUNT(*) FROM ews_alert`), "12 lonjakan harga, 2 surplus, 3 alert tetap; tanpa alert buatan mesin")
	assert.Equal(t, 31, s.hitung(t, `SELECT COUNT(*) FROM ews_riwayat`))

	t.Run("kode seed di bawah rentang sequence kode alert buatan mesin", func(t *testing.T) {
		// Bila seed memuat kode di atas awal sequence, alert pertama mesin bentrok (uq_ews_alert_kode) dan mesin galat.
		var r struct{ Tertinggi, Awal int64 }
		require.NoError(t, s.db.Raw(`SELECT (SELECT max(substring(kode from '^EWS-([0-9]+)$')::bigint) FROM ews_alert) AS tertinggi,
			(SELECT start_value FROM pg_sequences WHERE schemaname = 'public' AND sequencename = 'ews_alert_kode_seq') AS awal`).Scan(&r).Error)
		require.NotZero(t, r.Awal, "sequence kode alert tidak ditemukan")
		assert.Less(t, r.Tertinggi, r.Awal)
	})

	t.Run("tier demo lain tidak ikut: mesin tidak melihat harga, cuaca, atau stok demo", func(t *testing.T) {
		for _, tabel := range []string{"entri_harga", "cuaca_harian", "stok_cold_storage", "intervensi"} {
			assert.Zero(t, s.hitung(t, `SELECT COUNT(*) FROM `+tabel), tabel)
		}
		assert.Zero(t, s.hitung(t, `SELECT COUNT(*) FROM audit_log WHERE entitas IN ('ews_alert', 'aturan_ews')`))
	})

	t.Run("setiap severity dan status terwakili, severity mengikuti ambang aturan", func(t *testing.T) {
		var rows []struct {
			Status string
			N      int
		}
		require.NoError(t, s.db.Raw(`SELECT status::text AS status, COUNT(*) AS n FROM ews_alert GROUP BY 1`).Scan(&rows).Error)
		status := map[string]int{}
		for _, r := range rows {
			status[r.Status] = r.N
		}
		assert.Equal(t, map[string]int{"BARU": 7, "DIAKUI": 6, "DIESKALASI": 3, "SELESAI": 1}, status)

		var sev []struct{ Kode, Severity string }
		require.NoError(t, s.db.Raw(`SELECT kode, severity::text AS severity FROM ews_alert`).Scan(&sev).Error)
		bySeverity := map[string]string{}
		for _, r := range sev {
			bySeverity[r.Kode] = r.Severity
		}
		assert.Equal(t, "KRITIS", bySeverity["EWS-1000"], "cabai +21,4% >= kritis 18")
		assert.Equal(t, "WASPADA", bySeverity["EWS-1004"], "cabai +11,3% antara waspada 8 dan siaga 12")
		assert.Equal(t, "WASPADA", bySeverity["EWS-1008"], "beras +4,2% antara waspada 3 dan siaga 5")
		assert.Equal(t, "SIAGA", bySeverity["EWS-0901"], "hujan 148 mm antara siaga 140 dan kritis 200")
		assert.Equal(t, "WASPADA", bySeverity["EWS-0902"], "stok turun 32% antara waspada 25 dan siaga 40")
		assert.Equal(t, "INFO", bySeverity["EWS-1012"], "surplus selalu info")
	})

	t.Run("waktu relatif terhadap saat seed dijalankan", func(t *testing.T) {
		var r struct{ Terbaru, Tertua time.Time }
		require.NoError(t, s.db.Raw(`SELECT (SELECT dibuat_pada FROM ews_alert WHERE kode = 'EWS-1000') AS terbaru,
			(SELECT dibuat_pada FROM ews_alert WHERE kode = 'EWS-1013') AS tertua`).Scan(&r).Error)
		sekarang := jamDB(t, s.db)
		assert.WithinDuration(t, sekarang.Add(-4*time.Minute), r.Terbaru, 2*time.Minute)
		assert.WithinDuration(t, sekarang.Add(-311*time.Minute), r.Tertua, 2*time.Minute)
	})

	t.Run("riwayat mengikuti daur hidup tiap alert dengan kode RW-D-<kode alert>-<n>", func(t *testing.T) {
		var rows []struct{ Status, Jalur string }
		require.NoError(t, s.db.Raw(`SELECT a.status::text AS status, string_agg(r.ke::text, '>' ORDER BY r.waktu, r.id) AS jalur
			FROM ews_alert a JOIN ews_riwayat r ON r.alert_id = a.id GROUP BY a.id, a.status`).Scan(&rows).Error)
		require.Len(t, rows, 17, "setiap alert punya riwayat")
		want := map[string]string{
			"BARU": "BARU", "DIAKUI": "BARU>DIAKUI", "DIESKALASI": "BARU>DIAKUI>DIESKALASI", "SELESAI": "BARU>DIAKUI>SELESAI",
		}
		for _, r := range rows {
			assert.Equal(t, want[r.Status], r.Jalur, r.Status)
		}
		assert.Equal(t, 1, s.hitung(t, `SELECT COUNT(*) FROM ews_riwayat WHERE kode = 'RW-D-EWS-1000-0'`))
		assert.Equal(t, 3, s.hitung(t, `SELECT COUNT(*) FROM ews_riwayat r JOIN ews_alert a ON a.id = r.alert_id WHERE a.kode = 'EWS-1011'`), "EWS-1011 dieskalasi: BARU, DIAKUI, DIESKALASI")
		assert.Equal(t, 3, s.hitung(t, `SELECT COUNT(*) FROM ews_alert WHERE status = 'DIESKALASI' AND dieskalasi_oleh_id IS NOT NULL AND dieskalasi_pada > diakui_pada`),
			"alert dieskalasi membawa waktu dan pelaku eskalasi")
	})

	t.Run("alert merujuk aturan lewat ID basis data yang dicari dari kodenya", func(t *testing.T) {
		var kode string
		require.NoError(t, s.db.Raw(`SELECT t.kode FROM ews_alert a JOIN aturan_ews t ON t.id = a.aturan_id WHERE a.kode = 'EWS-1000'`).Scan(&kode).Error)
		assert.Equal(t, "ATR-LH-CBM-KRT", kode)
		assert.Zero(t, s.hitung(t, `SELECT COUNT(*) FROM ews_alert WHERE aturan_id IS NULL AND komoditas_kode IN ('CBM-KRT', 'BWM', 'BRS-MED') AND jenis <> 'OVER_SUPPLY'`),
			"alert komoditas pilot selalu punya aturan pemicu")
	})
}

func TestSeed_RerunRestoresDemoAlertsWithoutDuplicatesAndKeepsAdminChoices(t *testing.T) {
	s := newSuite(t, "ews_seed_ulang")
	alert1, aturan1 := s.idPerKode(t, "ews_alert"), s.idPerKode(t, "aturan_ews")

	// Pengguna dan mesin mengubah data: alert contoh ditutup, satu aturan dimatikan dan satu dikalibrasi ulang, mesin
	// membuat alert baru.
	require.NoError(t, s.db.Exec(`UPDATE ews_alert SET status = 'SELESAI', severity = 'INFO', selesai_pada = NOW() WHERE kode = 'EWS-1000'`).Error)
	require.NoError(t, s.db.Exec(`UPDATE aturan_ews SET aktif = false, kanal = ARRAY['EMAIL'], ambang_kritis = 77 WHERE kode = 'ATR-LH-BWM'`).Error)
	require.NoError(t, s.db.Exec(`UPDATE aturan_ews SET ambang_kritis = 99, versi = 2 WHERE kode = 'ATR-LH-CBM-KRT'`).Error)
	require.NoError(t, s.db.Exec(`INSERT INTO ews_alert (kode, severity, jenis, judul, pesan, kabupaten_kode, nilai, satuan_nilai, skor_dampak, sumber)
		VALUES ('EWS-2000', 'INFO', 'ANOMALI_DATA', 'Buatan mesin', 'p', '1171', 1, '%', 1, 'Rule engine EWS')`).Error)

	for range 2 {
		require.NoError(t, seed.SeedReference(t.Context(), s.db))
		require.NoError(t, seed.SeedFiles(t.Context(), s.db, "demo/06_ews_alerts.sql"))
	}

	assert.Equal(t, 9, s.hitung(t, `SELECT COUNT(*) FROM aturan_ews`), "aturan tidak boleh bertambah")
	assert.Equal(t, 18, s.hitung(t, `SELECT COUNT(*) FROM ews_alert`), "17 alert contoh tidak bertambah; alert buatan mesin dibiarkan")
	assert.Equal(t, 31, s.hitung(t, `SELECT COUNT(*) FROM ews_riwayat`), "riwayat tidak boleh bertambah")
	alert2, aturan2 := s.idPerKode(t, "ews_alert"), s.idPerKode(t, "aturan_ews")
	for kode, id := range alert1 {
		assert.Equal(t, id, alert2[kode], "ID alert %s tidak berubah saat seed diulang", kode)
	}
	assert.Equal(t, aturan1, aturan2, "ID aturan tidak berubah saat seed diulang")
	assert.Contains(t, alert2, "EWS-2000")

	t.Run("alert contoh kembali ke keadaan awal", func(t *testing.T) {
		var a struct {
			Status, Severity string
			SelesaiPada      *time.Time
		}
		require.NoError(t, s.db.Raw(`SELECT status::text AS status, severity::text AS severity, selesai_pada FROM ews_alert WHERE kode = 'EWS-1000'`).Scan(&a).Error)
		assert.Equal(t, "BARU", a.Status)
		assert.Equal(t, "KRITIS", a.Severity)
		assert.Nil(t, a.SelesaiPada)
	})

	t.Run("aturan versi 1 mengikuti seed tetapi status aktif dan kanal pilihan admin dibiarkan", func(t *testing.T) {
		var a struct {
			Aktif        bool
			Kanal        string
			AmbangKritis float64
		}
		require.NoError(t, s.db.Raw(`SELECT aktif, array_to_string(kanal, ',') AS kanal, ambang_kritis FROM aturan_ews WHERE kode = 'ATR-LH-BWM'`).Scan(&a).Error)
		assert.False(t, a.Aktif, "seed tidak mengaktifkan ulang aturan yang dimatikan admin")
		assert.Equal(t, "EMAIL", a.Kanal)
		assert.EqualValues(t, 18, a.AmbangKritis, "aturan yang belum dikalibrasi (versi 1) mengikuti nilai seed")
	})

	t.Run("aturan yang sudah dikalibrasi (versi > 1) tidak disentuh", func(t *testing.T) {
		var kritis float64
		require.NoError(t, s.db.Raw(`SELECT ambang_kritis FROM aturan_ews WHERE kode = 'ATR-LH-CBM-KRT'`).Scan(&kritis).Error)
		assert.EqualValues(t, 99, kritis)
	})
}

// Basis data hasil migrasi (atau hasil seed lama) sudah memuat baris dengan kode = ID lama tetapi ID dan isi berbeda:
// seed memperbaruinya menurut kode, tidak menggandakannya dan tidak mengubah ID-nya.
func TestSeed_DoesNotDuplicateRowsThatAlreadyHaveTheOldCode(t *testing.T) {
	s := newSuite(t, "ews_seed_migrasi")
	const idLama = 500
	require.NoError(t, s.db.Exec(`DELETE FROM ews_alert`).Error) // riwayat ikut terhapus (CASCADE)
	require.NoError(t, s.db.Exec(`INSERT INTO ews_alert (id, kode, severity, jenis, judul, pesan, kabupaten_kode, nilai, satuan_nilai, skor_dampak, sumber) OVERRIDING SYSTEM VALUE
		VALUES (?, 'EWS-1000', 'INFO', 'LONJAKAN_HARGA', 'Lama', 'p', '1171', 1, '%', 1, 's')`, idLama).Error)
	aturanLama := s.idPerKode(t, "aturan_ews")["ATR-LH-CBM-KRT"]
	require.NoError(t, s.db.Exec(`UPDATE aturan_ews SET ambang_waspada = 1, ambang_siaga = 2, ambang_kritis = 3 WHERE kode = 'ATR-LH-CBM-KRT'`).Error)

	require.NoError(t, seed.SeedReference(t.Context(), s.db))
	require.NoError(t, seed.SeedFiles(t.Context(), s.db, "demo/06_ews_alerts.sql"))

	assert.Equal(t, 9, s.hitung(t, `SELECT COUNT(*) FROM aturan_ews`))
	assert.Equal(t, 17, s.hitung(t, `SELECT COUNT(*) FROM ews_alert`))
	assert.Equal(t, 31, s.hitung(t, `SELECT COUNT(*) FROM ews_riwayat`))
	var sesudah struct {
		Aturan, Alert int64
		Kritis        float64
		Judul         string
	}
	require.NoError(t, s.db.Raw(`SELECT (SELECT id FROM aturan_ews WHERE kode = 'ATR-LH-CBM-KRT') AS aturan, (SELECT id FROM ews_alert WHERE kode = 'EWS-1000') AS alert,
		(SELECT ambang_kritis FROM aturan_ews WHERE kode = 'ATR-LH-CBM-KRT') AS kritis, (SELECT judul FROM ews_alert WHERE kode = 'EWS-1000') AS judul`).Scan(&sesudah).Error)
	assert.Equal(t, aturanLama, sesudah.Aturan)
	assert.EqualValues(t, idLama, sesudah.Alert, "ID baris lama dipertahankan")
	assert.EqualValues(t, 18, sesudah.Kritis, "aturan versi 1 mengikuti nilai seed")
	assert.Equal(t, "Lonjakan harga cabai merah keriting", sesudah.Judul, "baris lama diperbarui menurut seed, bukan digandakan")
}

func TestSeed_DemoAlertsFailClearlyWithoutRulesAndLeaveNothingHalfBuilt(t *testing.T) {
	db := pg.CreateDatabase(t, "ews_seed_tanpa_aturan")
	pgtest.MigrateUp(t, db)
	require.NoError(t, seed.SeedReference(t.Context(), db))
	require.NoError(t, seed.SeedFiles(t.Context(), db, "demo/01_auth_users.sql"))
	require.NoError(t, db.Exec(`DELETE FROM aturan_ews`).Error)

	err := seed.SeedFiles(t.Context(), db, "demo/06_ews_alerts.sql")

	require.Error(t, err)
	assert.ErrorContains(t, err, "aturan ews belum ada", "tanpa aturan, aturan_id alert tidak boleh diam-diam NULL")
	var n int64
	require.NoError(t, db.Table("ews_alert").Count(&n).Error)
	assert.Zero(t, n, "tidak ada alert setengah jadi")
}

// jamDB membaca jam PostgreSQL. Waktu seed dihitung dari now() basis data, jadi pembandingnya harus jam yang sama:
// jam container Docker bisa tertinggal puluhan detik sampai menit dari host (mis. setelah Mac tidur).
func jamDB(t *testing.T, db *gorm.DB) time.Time {
	t.Helper()
	var jam time.Time
	require.NoError(t, db.Raw(`SELECT clock_timestamp()`).Scan(&jam).Error)
	return jam
}
