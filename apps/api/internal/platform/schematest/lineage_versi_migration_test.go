package schematest

import (
	"testing"

	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Migrasi 000118 menambah entri_harga_versi (lineage per record harga): backfill satu versi MIGRASI untuk entri yang
// sudah ada, pemicu versi pada entri_harga, dan penjaga append-only. Tes dipatok ke versi 115 (skema entri_harga
// sudah BIGINT) dan menjalankan 000118 sendirian lewat Apply, supaya migrasi lain yang datang sesudahnya tidak
// mengubah hasilnya.
const (
	versiSebelumLineage = 115
	versiLineage        = 118
)

func TestMigrations_LineageVersiBackfillDownUpDanCascade(t *testing.T) {
	db := pg.NewDatabase(t, "migrate_lineage_versi")
	migrator := pgtest.Migrator(t, db)
	require.NoError(t, migrator.UpTo(t.Context(), versiSebelumLineage))

	for _, s := range []string{
		`INSERT INTO provinsi (kode, nama) VALUES ('11', 'Aceh')`,
		`INSERT INTO kabupaten (kode, nama, jenis, ibukota, lat, lng, provinsi_kode) VALUES ('1171', 'Banda Aceh', 'KOTA', 'Banda Aceh', 5.55, 95.32, '11')`,
		`INSERT INTO komoditas (kode, nama, satuan, kategori, harga_acuan, shelf_life_hari) VALUES ('BWM', 'Bawang Merah', 'kg', 'bawang', 40000, 14)`,
		`INSERT INTO pasar (id, nama, kabupaten_kode, lat, lng) VALUES ('PSR-1', 'Pasar Inti', '1171', 5.55, 95.32)`,
		`INSERT INTO pengguna (id, nama, email, peran, lapis, lembaga) VALUES
			(garda_uuid_stabil('pengguna', 'petugas'), 'Petugas', 'petugas@x.id', 'OPS_MAHASISWA', 'OPERASIONAL', 'FMIPA'),
			(garda_uuid_stabil('pengguna', 'validator'), 'Validator', 'validator@x.id', 'OPS_KOORDINATOR', 'OPERASIONAL', 'BPPSDMP')`,
		`INSERT INTO entri_harga (kode, komoditas_kode, pasar_id, tipe, harga, satuan, status, dicatat_pada, dibuat_pada, diupdate_pada, petugas_id) VALUES
			('HRG-LAMA-1', 'BWM', 'PSR-1', 'KONSUMEN', 40000, 'kg', 'DIAJUKAN', '2026-10-01 01:00+00', '2026-10-01 01:00+00', '2026-10-01 01:00+00', garda_uuid_stabil('pengguna', 'petugas')),
			('HRG-LAMA-2', 'BWM', 'PSR-1', 'PRODUSEN', 30000, 'kg', 'DIAJUKAN', '2026-10-02 01:00+00', '2026-10-02 01:00+00', '2026-10-02 03:00+00', garda_uuid_stabil('pengguna', 'petugas'))`,
	} {
		exec(t, db, s)
	}

	t.Run("up: satu versi MIGRASI per entri yang sudah ada", func(t *testing.T) {
		require.NoError(t, migrator.Apply(t.Context(), versiLineage, true))

		assert.EqualValues(t, 2, scanCount(t, db, `SELECT count(*) FROM entri_harga_versi`))
		assert.EqualValues(t, 2, scanCount(t, db, `
			SELECT count(*) FROM entri_harga_versi
			 WHERE versi = 1 AND aksi = 'MIGRASI' AND oleh IS NULL AND perubahan = '{}' AND status = 'DIAJUKAN'`))
		assert.EqualValues(t, 1, scanCount(t, db, `
			SELECT count(*) FROM entri_harga_versi v JOIN entri_harga e ON e.id = v.entri_id
			 WHERE e.kode = 'HRG-LAMA-2' AND v.waktu = e.diupdate_pada AND v.harga = 30000 AND (v.data->>'id')::bigint = e.id`),
			"waktu versi = diupdate_pada, data = salinan baris")
	})

	t.Run("pemicu: perubahan isi menambah versi, pergeseran waktu tidak", func(t *testing.T) {
		exec(t, db, `UPDATE entri_harga SET dicatat_pada = dicatat_pada + interval '1 hour', diupdate_pada = now() WHERE kode = 'HRG-LAMA-1'`)
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM entri_harga_versi v JOIN entri_harga e ON e.id = v.entri_id WHERE e.kode = 'HRG-LAMA-1'`))

		exec(t, db, `UPDATE entri_harga SET status = 'DIVALIDASI', divalidasi_oleh = garda_uuid_stabil('pengguna', 'validator'), divalidasi_pada = now() WHERE kode = 'HRG-LAMA-1'`)
		assert.EqualValues(t, 1, scanCount(t, db, `
			SELECT count(*) FROM entri_harga_versi v JOIN entri_harga e ON e.id = v.entri_id
			 WHERE e.kode = 'HRG-LAMA-1' AND v.versi = 2 AND v.aksi = 'VALIDASI'
			   AND v.oleh = garda_uuid_stabil('pengguna', 'validator') AND v.perubahan = ARRAY['divalidasi_oleh', 'status']`))
	})

	t.Run("down membuang tabel dan pemicu tanpa menyentuh entri, up memasang lagi", func(t *testing.T) {
		require.NoError(t, migrator.Apply(t.Context(), versiLineage, false))
		assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM pg_class WHERE relname = 'entri_harga_versi'`))
		assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM pg_proc WHERE proname IN ('entri_harga_catat_versi', 'entri_harga_versi_tolak_perubahan')`))
		assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM pg_trigger WHERE tgname = 'trg_entri_harga_versi'`))
		assert.EqualValues(t, 2, scanCount(t, db, `SELECT count(*) FROM entri_harga`))

		require.NoError(t, migrator.Apply(t.Context(), versiLineage, true))
		assert.EqualValues(t, 2, scanCount(t, db, `SELECT count(*) FROM entri_harga_versi WHERE versi = 1 AND aksi = 'MIGRASI'`),
			"backfill dari keadaan entri saat ini")
	})

	t.Run("menghapus entri ikut menghapus versinya, menghapus versi langsung ditolak", func(t *testing.T) {
		assert.Error(t, db.Exec(`DELETE FROM entri_harga_versi`).Error)
		assert.Error(t, db.Exec(`UPDATE entri_harga_versi SET harga = 1`).Error)
		assert.Error(t, db.Exec(`TRUNCATE entri_harga_versi`).Error)

		exec(t, db, `DELETE FROM entri_harga WHERE kode = 'HRG-LAMA-2'`)
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM entri_harga_versi`))
	})
}
