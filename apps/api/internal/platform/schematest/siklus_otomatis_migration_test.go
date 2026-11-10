package schematest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/garda-inflasi/api/internal/platform/migrate"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Migrasi 000125 (slice 08-B) menambah tabel aturan_siklus dan tiga kolom jejak keputusan pada siklus_ekonomi
// (otomatis, alasan, pulih_sejak). Tes dipatok ke versi 123 -> 125 (migrasi 000124 milik slice lain tidak menyentuh
// siklus_ekonomi), supaya migrasi yang datang sesudahnya tidak mengubah hasilnya.
const (
	versiSebelumSiklusOtomatis = 123
	versiSiklusOtomatis        = 125
)

func TestMigrations_SiklusOtomatisTabelKolomCheckHakDownUp(t *testing.T) {
	const namaDB = "migrate_siklus_otomatis"
	db := pg.NewDatabase(t, namaDB)
	migrator := pgtest.Migrator(t, db)
	require.NoError(t, migrator.UpTo(t.Context(), versiSebelumSiklusOtomatis))
	exec(t, db, `INSERT INTO provinsi (kode, nama) VALUES ('11', 'ACEH')`)
	// Baris lama (seed/penetapan manual) sebelum migrasi: satu aktif.
	exec(t, db, `INSERT INTO siklus_ekonomi (kode, wilayah_kode, siklus, mulai_pada) VALUES ('SE-11-LAMA', '11', 'TEKANAN', now() - interval '6 days')`)
	ada := func(tabel, kolom string) int64 {
		return scanCount(t, db, `SELECT count(*) FROM information_schema.columns WHERE table_name = ? AND column_name = ?`, tabel, kolom)
	}
	assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM information_schema.tables WHERE table_name = 'aturan_siklus'`), "kontrol: tabel belum ada")
	assert.Zero(t, ada("siklus_ekonomi", "otomatis"), "kontrol: kolom belum ada")

	t.Run("up: tabel aturan dan tiga kolom; baris lama tidak otomatis dan tanpa alasan", func(t *testing.T) {
		require.NoError(t, migrator.Apply(t.Context(), versiSiklusOtomatis, true))

		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM information_schema.tables WHERE table_name = 'aturan_siklus'`))
		assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM aturan_siklus`), "migrasi hanya membuat skema; isi awal ada di seed reference")
		assert.Equal(t, "boolean", scanString(t, db, `SELECT data_type FROM information_schema.columns WHERE table_name = 'siklus_ekonomi' AND column_name = 'otomatis'`))
		assert.Equal(t, "jsonb", scanString(t, db, `SELECT udt_name FROM information_schema.columns WHERE table_name = 'siklus_ekonomi' AND column_name = 'alasan'`))
		assert.Equal(t, "timestamp with time zone", scanString(t, db, `SELECT data_type FROM information_schema.columns WHERE table_name = 'siklus_ekonomi' AND column_name = 'pulih_sejak'`))
		assert.Equal(t, "NO", scanString(t, db, `SELECT is_nullable FROM information_schema.columns WHERE table_name = 'siklus_ekonomi' AND column_name = 'otomatis'`))
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM siklus_ekonomi WHERE kode = 'SE-11-LAMA' AND NOT otomatis AND alasan IS NULL AND pulih_sejak IS NULL`),
			"baris lama utuh: otomatis false, alasan dan pulih_sejak kosong")
		assert.NotEmpty(t, scanString(t, db, `SELECT coalesce(obj_description('aturan_siklus'::regclass, 'pg_class'), '')`), "tabel berkomentar")
		assert.NotEmpty(t, scanString(t, db, `SELECT coalesce(col_description('siklus_ekonomi'::regclass, attnum), '') FROM pg_attribute WHERE attrelid = 'siklus_ekonomi'::regclass AND attname = 'pulih_sejak'`))
		assert.Equal(t, "a", scanString(t, db, `SELECT attidentity FROM pg_attribute WHERE attrelid = 'aturan_siklus'::regclass AND attname = 'id'`), "PK identity (GENERATED ALWAYS)")
	})

	t.Run("Up aman diulang (IF NOT EXISTS) dan tidak menyentuh baris", func(t *testing.T) {
		berkas, err := os.ReadFile(filepath.Join(pgtest.MigrationsDir(), "000125_08b_siklus_otomatis.sql"))
		require.NoError(t, err)
		up, _, _ := strings.Cut(string(berkas), "-- +goose Down")
		exec(t, db, `INSERT INTO aturan_siklus (kode, siklus, indikator, ambang, prioritas) VALUES ('SKL-ULANG', 'SURPLUS', 'UTILISASI_STOK', 90, 20)`)

		sqlDB, err := db.DB()
		require.NoError(t, err)
		_, err = sqlDB.ExecContext(t.Context(), up)
		require.NoError(t, err)

		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM aturan_siklus WHERE kode = 'SKL-ULANG'`), "aturan tidak terhapus")
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM siklus_ekonomi`))
		exec(t, db, `DELETE FROM aturan_siklus WHERE kode = 'SKL-ULANG'`)
	})

	t.Run("aturan_siklus: nilai sah diterima, termasuk batas", func(t *testing.T) {
		for _, sah := range []string{
			`('A1', 'TEKANAN', 'ALERT_EWS', 'LONJAKAN_HARGA', 'SIAGA', 1, NULL, 10, 48)`,
			`('A2', 'SURPLUS', 'ALERT_EWS', 'OVER_SUPPLY', 'INFO', 5, NULL, 1, 0)`,
			`('A3', 'SURPLUS', 'UTILISASI_STOK', NULL, NULL, NULL, 100, 20, 8760)`,
			`('A4', 'SURPLUS', 'UTILISASI_STOK', NULL, NULL, NULL, 0.5, 20, 72)`,
		} {
			exec(t, db, `INSERT INTO aturan_siklus (kode, siklus, indikator, jenis_ews, severity_min, jumlah_min, ambang, prioritas, pulih_jam) VALUES `+sah)
		}
		exec(t, db, `DELETE FROM aturan_siklus`)
	})

	t.Run("aturan_siklus: CHECK menolak bentuk yang tidak sah", func(t *testing.T) {
		const kolom = `INSERT INTO aturan_siklus (kode, siklus, indikator, jenis_ews, severity_min, jumlah_min, ambang, prioritas, pulih_jam) VALUES `
		for nama, c := range map[string]struct{ nilai, constraint string }{
			"siklus NORMAL bukan pemicu":               {`('X', 'NORMAL', 'ALERT_EWS', 'LONJAKAN_HARGA', 'SIAGA', 1, NULL, 10, 48)`, "chk_aturan_siklus_siklus"},
			"indikator tak dikenal":                    {`('X', 'TEKANAN', 'LAIN', 'LONJAKAN_HARGA', 'SIAGA', 1, NULL, 10, 48)`, "chk_aturan_siklus_indikator"},
			"alert tanpa jenis":                        {`('X', 'TEKANAN', 'ALERT_EWS', NULL, 'SIAGA', 1, NULL, 10, 48)`, "chk_aturan_siklus_parameter"},
			"alert tanpa severity minimum":             {`('X', 'TEKANAN', 'ALERT_EWS', 'LONJAKAN_HARGA', NULL, 1, NULL, 10, 48)`, "chk_aturan_siklus_parameter"},
			"alert tanpa jumlah minimum":               {`('X', 'TEKANAN', 'ALERT_EWS', 'LONJAKAN_HARGA', 'SIAGA', NULL, NULL, 10, 48)`, "chk_aturan_siklus_parameter"},
			"alert dengan jumlah minimum nol":          {`('X', 'TEKANAN', 'ALERT_EWS', 'LONJAKAN_HARGA', 'SIAGA', 0, NULL, 10, 48)`, "chk_aturan_siklus_parameter"},
			"alert dengan ambang (milik stok)":         {`('X', 'TEKANAN', 'ALERT_EWS', 'LONJAKAN_HARGA', 'SIAGA', 1, 90, 10, 48)`, "chk_aturan_siklus_parameter"},
			"stok tanpa ambang":                        {`('X', 'SURPLUS', 'UTILISASI_STOK', NULL, NULL, NULL, NULL, 20, 72)`, "chk_aturan_siklus_parameter"},
			"stok dengan ambang nol":                   {`('X', 'SURPLUS', 'UTILISASI_STOK', NULL, NULL, NULL, 0, 20, 72)`, "chk_aturan_siklus_parameter"},
			"stok dengan ambang di atas 100 persen":    {`('X', 'SURPLUS', 'UTILISASI_STOK', NULL, NULL, NULL, 100.5, 20, 72)`, "chk_aturan_siklus_parameter"},
			"stok dengan jenis alert (milik alert)":    {`('X', 'SURPLUS', 'UTILISASI_STOK', 'OVER_SUPPLY', NULL, NULL, 90, 20, 72)`, "chk_aturan_siklus_parameter"},
			"stok dengan jumlah minimum (milik alert)": {`('X', 'SURPLUS', 'UTILISASI_STOK', NULL, NULL, 1, 90, 20, 72)`, "chk_aturan_siklus_parameter"},
			"prioritas nol":                            {`('X', 'TEKANAN', 'ALERT_EWS', 'LONJAKAN_HARGA', 'SIAGA', 1, NULL, 0, 48)`, "chk_aturan_siklus_prioritas"},
			"masa pulih negatif":                       {`('X', 'TEKANAN', 'ALERT_EWS', 'LONJAKAN_HARGA', 'SIAGA', 1, NULL, 10, -1)`, "chk_aturan_siklus_pulih"},
			"masa pulih lebih dari setahun":            {`('X', 'TEKANAN', 'ALERT_EWS', 'LONJAKAN_HARGA', 'SIAGA', 1, NULL, 10, 8761)`, "chk_aturan_siklus_pulih"},
		} {
			err := db.Exec(kolom + c.nilai).Error
			require.Error(t, err, nama)
			assert.Contains(t, err.Error(), c.constraint, nama)
		}
		exec(t, db, kolom+`('DUP', 'TEKANAN', 'ALERT_EWS', 'LONJAKAN_HARGA', 'SIAGA', 1, NULL, 10, 48)`)
		err := db.Exec(kolom + `('DUP', 'SURPLUS', 'ALERT_EWS', 'OVER_SUPPLY', 'SIAGA', 1, NULL, 20, 72)`).Error
		require.Error(t, err)
		assert.Contains(t, err.Error(), "uq_aturan_siklus_kode", "kode UNIQUE: kunci idempoten seed")
		exec(t, db, `DELETE FROM aturan_siklus`)
	})

	t.Run("siklus_ekonomi: CHECK jejak keputusan dan indeks siklus aktif tetap berlaku", func(t *testing.T) {
		exec(t, db, `UPDATE siklus_ekonomi SET akhir_pada = now() WHERE kode = 'SE-11-LAMA'`)
		const sisip = `INSERT INTO siklus_ekonomi (wilayah_kode, siklus, mulai_pada, akhir_pada, otomatis, alasan, pulih_sejak) VALUES `

		for nama, c := range map[string]struct{ nilai, constraint string }{
			"otomatis tanpa alasan":                  {`('11', 'NORMAL', now(), NULL, true, NULL, NULL)`, "chk_siklus_ekonomi_otomatis"},
			"otomatis dengan alasan bukan objek":     {`('11', 'NORMAL', now(), NULL, true, '[1]'::jsonb, NULL)`, "chk_siklus_ekonomi_otomatis"},
			"penanda pulih pada baris yang tertutup": {`('11', 'NORMAL', now() - interval '2 days', now() - interval '1 day', false, NULL, now() - interval '1 day')`, "chk_siklus_ekonomi_pulih"},
		} {
			err := db.Exec(sisip + c.nilai).Error
			require.Error(t, err, nama)
			assert.Contains(t, err.Error(), c.constraint, nama)
		}
		// Sah: baris aktif otomatis dengan alasan objek dan penanda pulih; baris manual tanpa alasan.
		exec(t, db, sisip+`('11', 'TEKANAN', now(), NULL, true, '{"ringkasan": "x"}'::jsonb, now())`)
		err := db.Exec(sisip + `('11', 'NORMAL', now(), NULL, false, NULL, NULL)`).Error
		require.Error(t, err)
		assert.Contains(t, err.Error(), "uq_siklus_ekonomi_aktif", "satu siklus aktif per wilayah tetap dijaga indeks unik parsial")
		exec(t, db, `UPDATE siklus_ekonomi SET akhir_pada = now() + interval '1 second', pulih_sejak = NULL WHERE otomatis`)
	})

	t.Run("hak garda_aplikasi: tabel baru dan kolom baru terjangkau tanpa GRANT tambahan, DDL tetap ditolak", func(t *testing.T) {
		const nama, sandi = "garda_app_siklus", "sandi-uji-cukup-panjang-8"
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, migrate.SiapkanPenggunaAplikasi(t.Context(), sqlDB, nama, sandi))
		app := pg.BukaSebagai(t, namaDB, nama, sandi)

		require.NoError(t, app.Exec(`INSERT INTO aturan_siklus (kode, siklus, indikator, ambang, prioritas) VALUES ('SKL-APP', 'SURPLUS', 'UTILISASI_STOK', 90, 20)`).Error)
		require.NoError(t, app.Exec(`UPDATE aturan_siklus SET ambang = 95 WHERE kode = 'SKL-APP'`).Error)
		assert.EqualValues(t, 1, scanCount(t, app, `SELECT count(*) FROM aturan_siklus WHERE kode = 'SKL-APP' AND ambang = 95`))
		require.NoError(t, app.Exec(`INSERT INTO siklus_ekonomi (wilayah_kode, siklus, mulai_pada, otomatis, alasan)
			VALUES ('11', 'NORMAL', now(), true, '{"ringkasan": "x"}'::jsonb)`).Error, "api/worker menulis siklus dengan kolom baru")
		require.NoError(t, app.Exec(`UPDATE siklus_ekonomi SET pulih_sejak = now() WHERE otomatis AND akhir_pada IS NULL`).Error)
		require.NoError(t, app.Exec(`UPDATE siklus_ekonomi SET akhir_pada = now() + interval '5 seconds', pulih_sejak = NULL WHERE otomatis AND akhir_pada IS NULL`).Error)
		require.NoError(t, app.Exec(`DELETE FROM aturan_siklus WHERE kode = 'SKL-APP'`).Error)
		err = app.Exec(`DROP TABLE aturan_siklus`).Error
		require.Error(t, err)
		assert.Contains(t, err.Error(), "42501", "peran aplikasi tidak boleh DDL")
	})

	t.Run("down membuang tabel dan kolom tanpa menyentuh baris siklus; up memasang lagi", func(t *testing.T) {
		exec(t, db, `INSERT INTO aturan_siklus (kode, siklus, indikator, ambang, prioritas) VALUES ('SKL-DOWN', 'SURPLUS', 'UTILISASI_STOK', 90, 20)`)
		jumlahSiklus := scanCount(t, db, `SELECT count(*) FROM siklus_ekonomi`)
		require.Greater(t, jumlahSiklus, int64(1), "kontrol: ada baris buatan mesin selain baris lama")

		require.NoError(t, migrator.Apply(t.Context(), versiSiklusOtomatis, false))

		assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM information_schema.tables WHERE table_name = 'aturan_siklus'`))
		for _, kolom := range []string{"otomatis", "alasan", "pulih_sejak"} {
			assert.Zero(t, ada("siklus_ekonomi", kolom), kolom)
		}
		assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM pg_constraint WHERE conname IN ('chk_siklus_ekonomi_otomatis', 'chk_siklus_ekonomi_pulih')`), "CHECK ikut terbuang")
		assert.Equal(t, jumlahSiklus, scanCount(t, db, `SELECT count(*) FROM siklus_ekonomi`), "baris siklus utuh")
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM pg_indexes WHERE indexname = 'uq_siklus_ekonomi_aktif'`), "indeks siklus aktif utuh")

		require.NoError(t, migrator.Apply(t.Context(), versiSiklusOtomatis, true))
		assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM aturan_siklus`), "aturan yang terbuang tidak kembali")
		assert.EqualValues(t, 1, ada("siklus_ekonomi", "otomatis"))
		assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM siklus_ekonomi WHERE otomatis`), "kolom dipasang ulang dengan nilai bawaan false")
	})
}
