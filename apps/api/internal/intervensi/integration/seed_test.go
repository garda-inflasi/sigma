package integration

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/pkg/seed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Tes kontrak seed intervensi: reference/08_intervensi_tahap.sql dan demo/07_intervensi_kartu.sql. Seeder Go lama
// (internal/intervensi/seeder) punya tes bentuk data, idempotensi, UUID stabil, dan kecocokan fixture kontrak web;
// tes itu ikut terhapus bersama paketnya, jadi kontraknya dijaga di sini terhadap PostgreSQL sungguhan. Yang diperiksa
// hanya isi seed; perilaku API ada di berkas tes lain.

// fixtureTahapPath adalah cermin seed yang dibaca tes web: SIKLUS_TAHAP di apps/web/src/lib/data/siklus.ts harus sama.
const fixtureTahapPath = "../../../../../packages/contracts/intervensi-tahap.json"

func (s *suite) hitung(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, s.db.Raw(sql, args...).Scan(&n).Error)
	return n
}

func TestSeed_TahapAndAlurMatchTheContractFixtureAndCoverEveryApprovalStage(t *testing.T) {
	s := sharedSuite(t)

	type tahap struct {
		Siklus         string `json:"siklus"`
		Kode           string `json:"kode"`
		Urutan         int    `json:"urutan"`
		Label          string `json:"label"`
		Petunjuk       string `json:"petunjuk"`
		IzinDibutuhkan string `json:"izin_dibutuhkan"`
	}
	raw, err := os.ReadFile(fixtureTahapPath)
	require.NoError(t, err)
	var want []tahap
	require.NoError(t, json.Unmarshal(raw, &want))
	var got []tahap
	require.NoError(t, s.db.Raw(`SELECT lower(siklus::text) AS siklus, kode_tahap AS kode, urutan, label, petunjuk, izin_dibutuhkan FROM tahap_intervensi_def`).Scan(&got).Error)
	assert.ElementsMatch(t, want, got, "packages/contracts/intervensi-tahap.json tidak sama dengan reference/08_intervensi_tahap.sql; perbarui keduanya bersama siklus.ts")

	t.Run("urutan berurutan dari 1 per siklus dan tahap penutup tanpa SLA", func(t *testing.T) {
		var rows []struct {
			Siklus         string
			Jumlah, Maks   int
			Terakhir       string
			SlaTerakhirNil bool
		}
		require.NoError(t, s.db.Raw(`SELECT d.siklus::text AS siklus, COUNT(*) AS jumlah, MAX(d.urutan) AS maks,
				(SELECT t.kode_tahap FROM tahap_intervensi_def t WHERE t.siklus = d.siklus ORDER BY t.urutan DESC LIMIT 1) AS terakhir,
				(SELECT t.sla_hari IS NULL FROM tahap_intervensi_def t WHERE t.siklus = d.siklus ORDER BY t.urutan DESC LIMIT 1) AS sla_terakhir_nil
			FROM tahap_intervensi_def d GROUP BY d.siklus`).Scan(&rows).Error)
		jumlah := map[string]int{}
		for _, r := range rows {
			jumlah[r.Siklus] = r.Jumlah
			assert.Equal(t, r.Jumlah, r.Maks, "%s: urutan tahap harus 1..n", r.Siklus)
			assert.Regexp(t, `-selesai$`, r.Terakhir, "tahap terakhir %s harus penutup", r.Siklus)
			assert.True(t, r.SlaTerakhirNil, "tahap penutup %s tidak punya SLA", r.Siklus)
		}
		assert.Equal(t, map[string]int{"NORMAL": 5, "TEKANAN": 6, "SURPLUS": 5}, jumlah)
	})

	t.Run("setiap tahap persetujuan punya alur dan sebaliknya; langkah akhir alur = izin tahap tujuan", func(t *testing.T) {
		assert.Equal(t, 5, s.hitung(t, `SELECT COUNT(*) FROM alur_persetujuan`))
		assert.Equal(t, 6, s.hitung(t, `SELECT COUNT(*) FROM alur_persetujuan_langkah`))
		assert.Equal(t, 1, s.hitung(t, `SELECT COUNT(*) FROM alur_persetujuan WHERE jenis = 'ESKALASI' AND siklus IS NULL AND tahap_tujuan IS NULL`))
		assert.Zero(t, s.hitung(t, `SELECT COUNT(*) FROM tahap_intervensi_def d WHERE d.izin_dibutuhkan <> 'intervensi.kelola'
			AND NOT EXISTS (SELECT 1 FROM alur_persetujuan a WHERE a.jenis = 'PINDAH_TAHAP' AND a.siklus = d.siklus AND a.tahap_tujuan = d.kode_tahap)`),
			"tahap persetujuan tanpa alur")
		assert.Zero(t, s.hitung(t, `SELECT COUNT(*) FROM alur_persetujuan a JOIN tahap_intervensi_def d ON d.siklus = a.siklus AND d.kode_tahap = a.tahap_tujuan
			WHERE a.jenis = 'PINDAH_TAHAP' AND d.izin_dibutuhkan = 'intervensi.kelola'`), "alur untuk tahap yang bukan tahap persetujuan")
		assert.Zero(t, s.hitung(t, `SELECT COUNT(*) FROM alur_persetujuan a JOIN tahap_intervensi_def d ON d.siklus = a.siklus AND d.kode_tahap = a.tahap_tujuan
			WHERE a.jenis = 'PINDAH_TAHAP' AND (SELECT l.izin_dibutuhkan FROM alur_persetujuan_langkah l WHERE l.alur_kode = a.kode ORDER BY l.urutan DESC LIMIT 1) <> d.izin_dibutuhkan`),
			"langkah akhir alur bertentangan dengan izin tahap tujuan")
	})
}

func TestSeed_DemoBoardIsTheNineteenCardsWithTheirHistory(t *testing.T) {
	s := sharedSuite(t)

	assert.Equal(t, 19, s.hitung(t, `SELECT COUNT(*) FROM intervensi`), "7 tekanan, 6 normal, 6 surplus; tanpa kartu buatan workflow otomatis")
	assert.Equal(t, 28, s.hitung(t, `SELECT COUNT(*) FROM intervensi_pj`))
	assert.Equal(t, 50, s.hitung(t, `SELECT COUNT(*) FROM intervensi_riwayat`), "19 pembuatan + 31 pemindahan tahap")
	assert.Equal(t, 1, s.hitung(t, `SELECT COUNT(*) FROM intervensi_persetujuan`))
	assert.Equal(t, 2, s.hitung(t, `SELECT COUNT(*) FROM intervensi_persetujuan_langkah`))

	t.Run("kode seed di bawah rentang sequence kode kartu baru", func(t *testing.T) {
		// Bila seed memuat kode di atas awal sequence, pembuatan kartu pertama bentrok (uq_intervensi_kode) dan gagal 500.
		var r struct{ Tertinggi, Awal int64 }
		require.NoError(t, s.db.Raw(`SELECT (SELECT max(substring(kode from '^INT-([0-9]+)$')::bigint) FROM intervensi) AS tertinggi,
			(SELECT start_value FROM pg_sequences WHERE schemaname = 'public' AND sequencename = 'intervensi_kode_seq') AS awal`).Scan(&r).Error)
		require.NotZero(t, r.Awal, "sequence kode kartu tidak ditemukan")
		assert.Less(t, r.Tertinggi, r.Awal)
	})

	t.Run("ID kartu = UUID stabil dari kode dan urutan unik per tahap", func(t *testing.T) {
		assert.Zero(t, s.hitung(t, `SELECT COUNT(*) FROM intervensi WHERE id <> garda_uuid_stabil('intervensi', kode)`))
		assert.Zero(t, s.hitung(t, `SELECT COUNT(*) FROM (SELECT tahap, urutan FROM intervensi GROUP BY 1, 2 HAVING COUNT(*) > 1) x`))
	})

	t.Run("sebaran siklus, kartu selesai, dan alert pemicu", func(t *testing.T) {
		var rows []struct {
			Siklus string
			N      int
		}
		require.NoError(t, s.db.Raw(`SELECT siklus::text AS siklus, COUNT(*) AS n FROM intervensi GROUP BY 1`).Scan(&rows).Error)
		siklus := map[string]int{}
		for _, r := range rows {
			siklus[r.Siklus] = r.N
		}
		assert.Equal(t, map[string]int{"TEKANAN": 7, "NORMAL": 6, "SURPLUS": 6}, siklus)

		var selesai []string
		require.NoError(t, s.db.Raw(`SELECT kode FROM intervensi WHERE selesai_pada IS NOT NULL AND tahap LIKE '%-selesai' AND checklist_selesai = checklist_total ORDER BY kode`).Scan(&selesai).Error)
		assert.Equal(t, []string{"INT-313", "INT-319"}, selesai, "kartu di tahap penutup selesai dengan checklist tuntas")
		assert.Equal(t, 2, s.hitung(t, `SELECT COUNT(*) FROM intervensi WHERE selesai_pada IS NOT NULL`), "hanya kartu di tahap penutup yang selesai")

		var alert []struct{ Kode, AlertKode string }
		require.NoError(t, s.db.Raw(`SELECT i.kode, a.kode AS alert_kode FROM intervensi i JOIN ews_alert a ON a.id = i.ews_alert_id ORDER BY i.kode`).Scan(&alert).Error)
		assert.Equal(t, []struct{ Kode, AlertKode string }{
			{"INT-301", "EWS-1000"}, {"INT-302", "EWS-1002"}, {"INT-303", "EWS-1003"}, {"INT-314", "EWS-1012"}, {"INT-315", "EWS-1013"},
		}, alert, "hanya 5 kartu punya alert pemicu; 14 lainnya tanpa alert")

		assert.Zero(t, s.hitung(t, `SELECT COUNT(*) FROM intervensi i WHERE (SELECT COUNT(*) FROM intervensi_pj p WHERE p.intervensi_id = i.id) NOT BETWEEN 1 AND 2`),
			"tiap kartu punya satu atau dua penanggung jawab")
	})

	t.Run("waktu relatif terhadap saat seed dijalankan; tahap_sejak mengikuti perjalanan ke tahap sekarang", func(t *testing.T) {
		var r struct {
			Dibuat, Tenggat, Diajukan time.Time
			SejakAwal, SejakFiskal    bool
		}
		require.NoError(t, s.db.Raw(`SELECT (SELECT dibuat_pada FROM intervensi WHERE kode = 'INT-301') AS dibuat,
				(SELECT tenggat FROM intervensi WHERE kode = 'INT-301') AS tenggat,
				(SELECT diajukan_pada FROM intervensi_persetujuan) AS diajukan,
				(SELECT tahap_sejak = dibuat_pada FROM intervensi WHERE kode = 'INT-301') AS sejak_awal,
				(SELECT tahap_sejak = dibuat_pada + interval '24 hours' FROM intervensi WHERE kode = 'INT-305') AS sejak_fiskal`).Scan(&r).Error)
		sekarang := jamDB(t, s.db)
		assert.WithinDuration(t, sekarang.Add(-240*time.Hour), r.Dibuat, 2*time.Minute)
		assert.WithinDuration(t, sekarang.Add(24*time.Hour), r.Tenggat, 2*time.Minute)
		assert.WithinDuration(t, sekarang.Add(-3*time.Hour), r.Diajukan, 2*time.Minute)
		assert.True(t, r.SejakAwal, "INT-301 di t-ews (tahap pertama): tahap_sejak = dibuat_pada")
		assert.True(t, r.SejakFiskal, "INT-305 di t-fiskal (tahap kelima): tahap_sejak = dibuat_pada + 4 x 6 jam")
	})

	t.Run("riwayat tiap kartu dimulai dari pembuatan dan berjalan tanpa putus sampai tahap sekarang", func(t *testing.T) {
		assert.Zero(t, s.hitung(t, `SELECT COUNT(*) FROM intervensi i WHERE (SELECT r.aksi FROM intervensi_riwayat r WHERE r.intervensi_id = i.id ORDER BY r.waktu, r.id LIMIT 1) <> 'BUAT'`))
		assert.Zero(t, s.hitung(t, `SELECT COUNT(*) FROM intervensi i WHERE (SELECT r.ke FROM intervensi_riwayat r WHERE r.intervensi_id = i.id ORDER BY r.waktu DESC, r.id DESC LIMIT 1) <> i.tahap`),
			"riwayat berakhir di tahap kartu sekarang")
		assert.Zero(t, s.hitung(t, `SELECT COUNT(*) FROM (SELECT aksi, dari, lag(ke) OVER (PARTITION BY intervensi_id ORDER BY waktu, id) AS sebelumnya FROM intervensi_riwayat) x
			WHERE aksi = 'PINDAH_TAHAP' AND dari IS DISTINCT FROM sebelumnya`), "rantai tahap tak putus")
		assert.Equal(t, 4, s.hitung(t, `SELECT COUNT(*) FROM intervensi_riwayat WHERE intervensi_id = garda_uuid_stabil('intervensi', 'INT-304')`), "INT-304 di tahap keempat: dibuat + 3 pemindahan")
	})

	t.Run("satu permintaan persetujuan terbuka: INT-304 ke t-fiskal oleh manajer program, rantai disalin dari alur", func(t *testing.T) {
		var p struct {
			IntervensiID, AlurKode, Status, TahapTujuan, DiajukanOleh string
			LangkahKini                                               int
		}
		require.NoError(t, s.db.Raw(`SELECT intervensi_id::text AS intervensi_id, alur_kode, status, tahap_tujuan, diajukan_oleh::text AS diajukan_oleh, langkah_kini FROM intervensi_persetujuan`).Scan(&p).Error)
		assert.Equal(t, kid("INT-304"), p.IntervensiID)
		assert.Equal(t, "pindah-tekanan-fiskal", p.AlurKode)
		assert.Equal(t, "MENUNGGU", p.Status)
		assert.Equal(t, "t-fiskal", p.TahapTujuan)
		assert.Equal(t, uid("manajer_program"), p.DiajukanOleh)
		assert.Equal(t, 1, p.LangkahKini)
		var izin []string
		require.NoError(t, s.db.Raw(`SELECT izin_dibutuhkan FROM intervensi_persetujuan_langkah ORDER BY urutan`).Scan(&izin).Error)
		assert.Equal(t, []string{"intervensi.setujui", "intervensi.setujui_fiskal"}, izin)
	})
}

func TestSeed_RerunRestoresDemoCardsWithoutDuplicatesAndKeepsEditedAlur(t *testing.T) {
	s := newSuite(t, "intervensi_seed_ulang")

	// Pengguna mengubah data: kartu contoh dipindah dan dieskalasi, ditambah penanggung jawab, permintaan diputuskan;
	// satu kartu baru dibuat; admin mengubah satu alur persetujuan.
	require.NoError(t, s.db.Exec(`UPDATE intervensi SET tahap = 't-verifikasi', versi = 7, dieskalasi_pada = NOW(), dieskalasi_oleh_id = garda_uuid_stabil('pengguna', 'manajer_data') WHERE kode = 'INT-301'`).Error)
	require.NoError(t, s.db.Exec(`INSERT INTO intervensi_pj (intervensi_id, pengguna_id) VALUES (garda_uuid_stabil('intervensi', 'INT-301'), garda_uuid_stabil('pengguna', 'strategis_bi'))`).Error)
	require.NoError(t, s.db.Exec(`INSERT INTO intervensi_riwayat (intervensi_id, aksi, dari, ke) VALUES (garda_uuid_stabil('intervensi', 'INT-301'), 'PINDAH_TAHAP', 't-ews', 't-verifikasi')`).Error)
	require.NoError(t, s.db.Exec(`UPDATE intervensi_persetujuan SET status = 'DISETUJUI', selesai_pada = NOW()`).Error)
	require.NoError(t, s.db.Exec(`INSERT INTO intervensi (kode, siklus, tahap, judul, komoditas_kode, kabupaten_kode, prioritas, tenggat)
		VALUES ('INT-1000', 'TEKANAN', 't-ews', 'Kartu baru', 'CBM-KRT', '1171', 'TINGGI', NOW() + interval '3 days')`).Error)
	require.NoError(t, s.db.Exec(`UPDATE alur_persetujuan_langkah SET label = 'Diubah admin' WHERE alur_kode = 'pindah-tekanan-fiskal' AND urutan = 2`).Error)
	require.NoError(t, s.db.Exec(`UPDATE alur_persetujuan SET aktif = false WHERE kode = 'pindah-surplus-selesai'`).Error)
	require.NoError(t, s.db.Exec(`UPDATE tahap_intervensi_def SET label = 'Diubah', sla_hari = 99 WHERE kode_tahap = 't-ews'`).Error)

	for range 2 {
		require.NoError(t, seed.SeedReference(t.Context(), s.db))
		require.NoError(t, seed.SeedFiles(t.Context(), s.db, "demo/07_intervensi_kartu.sql"))
	}

	assert.Equal(t, 16, s.hitung(t, `SELECT COUNT(*) FROM tahap_intervensi_def`))
	assert.Equal(t, 20, s.hitung(t, `SELECT COUNT(*) FROM intervensi`), "19 kartu contoh tidak bertambah; kartu baru dibiarkan")
	assert.Equal(t, 28, s.hitung(t, `SELECT COUNT(*) FROM intervensi_pj`), "penanggung jawab tidak boleh bertambah")
	assert.Equal(t, 50, s.hitung(t, `SELECT COUNT(*) FROM intervensi_riwayat`), "riwayat tidak boleh bertambah")
	assert.Equal(t, 1, s.hitung(t, `SELECT COUNT(*) FROM intervensi_persetujuan`), "permintaan dibangun ulang tanpa menggandakan")
	assert.Equal(t, 2, s.hitung(t, `SELECT COUNT(*) FROM intervensi_persetujuan_langkah`))
	assert.Zero(t, s.hitung(t, `SELECT COUNT(*) FROM intervensi WHERE kode LIKE 'INT-3%' AND id <> garda_uuid_stabil('intervensi', kode)`), "UUID kartu seed tetap stabil")

	t.Run("kartu contoh kembali ke keadaan awal", func(t *testing.T) {
		var k struct {
			Tahap          string
			Versi          int
			DieskalasiPada *time.Time
			AlertKode      string
		}
		require.NoError(t, s.db.Raw(`SELECT i.tahap, i.versi, i.dieskalasi_pada, a.kode AS alert_kode FROM intervensi i JOIN ews_alert a ON a.id = i.ews_alert_id WHERE i.kode = 'INT-301'`).Scan(&k).Error)
		assert.Equal(t, "t-ews", k.Tahap)
		assert.Equal(t, 1, k.Versi)
		assert.Nil(t, k.DieskalasiPada)
		assert.Equal(t, "EWS-1000", k.AlertKode)
		var pj []string
		require.NoError(t, s.db.Raw(`SELECT pengguna_id::text FROM intervensi_pj WHERE intervensi_id = garda_uuid_stabil('intervensi', 'INT-301')`).Scan(&pj).Error)
		assert.Equal(t, []string{uid("manajer_program")}, pj, "penanggung jawab dikembalikan ke keadaan awal")
		var status string
		require.NoError(t, s.db.Raw(`SELECT status FROM intervensi_persetujuan`).Scan(&status).Error)
		assert.Equal(t, "MENUNGGU", status)
	})

	t.Run("definisi tahap milik kode: mengikuti seed lagi", func(t *testing.T) {
		var d struct {
			Label  string
			SLA    int
			Jumlah int
		}
		require.NoError(t, s.db.Raw(`SELECT label, sla_hari AS sla, (SELECT COUNT(*) FROM tahap_intervensi_def) AS jumlah FROM tahap_intervensi_def WHERE kode_tahap = 't-ews'`).Scan(&d).Error)
		assert.Equal(t, "EWS terpicu", d.Label)
		assert.Equal(t, 1, d.SLA)
		assert.Equal(t, 16, d.Jumlah)
	})

	t.Run("alur persetujuan yang diubah admin tidak ditimpa dan tidak diaktifkan ulang", func(t *testing.T) {
		var label string
		require.NoError(t, s.db.Raw(`SELECT label FROM alur_persetujuan_langkah WHERE alur_kode = 'pindah-tekanan-fiskal' AND urutan = 2`).Scan(&label).Error)
		assert.Equal(t, "Diubah admin", label)
		var aktif bool
		require.NoError(t, s.db.Raw(`SELECT aktif FROM alur_persetujuan WHERE kode = 'pindah-surplus-selesai'`).Scan(&aktif).Error)
		assert.False(t, aktif)
	})
}

func TestSeed_DemoCardsFailClearlyWithoutAlertsOrTahapAndLeaveNothingHalfBuilt(t *testing.T) {
	siapkan := func(t *testing.T, name string, berkas ...string) *suite {
		t.Helper()
		db := pg.CreateDatabase(t, name)
		pgtest.MigrateUp(t, db)
		require.NoError(t, seed.SeedReference(t.Context(), db))
		require.NoError(t, seed.SeedFiles(t.Context(), db, berkas...))
		return &suite{db: db}
	}

	t.Run("tanpa alert EWS demo", func(t *testing.T) {
		s := siapkan(t, "intervensi_seed_tanpa_alert", "demo/01_auth_users.sql")

		err := seed.SeedFiles(t.Context(), s.db, "demo/07_intervensi_kartu.sql")

		require.Error(t, err)
		assert.ErrorContains(t, err, "alert ews demo belum ada", "tanpa alert, ews_alert_id kartu tidak boleh diam-diam NULL")
		assert.Zero(t, s.hitung(t, `SELECT COUNT(*) FROM intervensi`), "tidak ada kartu setengah jadi")
	})

	t.Run("tanpa definisi tahap", func(t *testing.T) {
		s := siapkan(t, "intervensi_seed_tanpa_tahap", "demo/01_auth_users.sql", "demo/06_ews_alerts.sql")
		require.NoError(t, s.db.Exec(`DELETE FROM alur_persetujuan`).Error) // alur menunjuk tahap (RESTRICT); langkahnya ikut terhapus
		require.NoError(t, s.db.Exec(`DELETE FROM tahap_intervensi_def`).Error)

		err := seed.SeedFiles(t.Context(), s.db, "demo/07_intervensi_kartu.sql")

		require.Error(t, err)
		assert.ErrorContains(t, err, "definisi tahap belum ada")
		assert.Zero(t, s.hitung(t, `SELECT COUNT(*) FROM intervensi`), "tidak ada kartu setengah jadi")
	})
}

// jamDB membaca jam PostgreSQL. Waktu seed dihitung dari now() basis data, jadi pembandingnya harus jam yang sama:
// jam container Docker bisa tertinggal puluhan detik sampai menit dari host (mis. setelah Mac tidur).
func jamDB(t *testing.T, db *gorm.DB) time.Time {
	t.Helper()
	var jam time.Time
	require.NoError(t, db.Raw(`SELECT clock_timestamp()`).Scan(&jam).Error)
	return jam
}
