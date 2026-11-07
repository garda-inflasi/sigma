package schematest

import (
	"errors"
	"testing"

	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Migrasi 000112 mengubah kunci EWS dan intervensi (aturan_ews, ews_alert, riwayatnya: VARCHAR -> BIGINT;
// intervensi: VARCHAR -> UUID). Tes dipatok ke versi 111 dan 112 (bukan "terbaru") supaya migrasi konversi
// lain tidak mengubah bentuk fixture ID lama di bawah. Fixture hanya mengisi tabel milik grup ini ditambah
// tabel referensi dan pengguna.
const (
	versiSebelumEwsIntervensi = 111
	versiEwsIntervensi        = 112
)

func g2UUIDIntervensi(kode string) string { return idx.UUIDStabil("intervensi", kode) }

// g2Daftar membaca satu kolom teks dari banyak baris.
func g2Daftar(t *testing.T, db *gorm.DB, sql string, args ...any) []string {
	t.Helper()
	var out []string
	require.NoError(t, db.Raw(sql, args...).Scan(&out).Error, sql)
	return out
}

// g2Rollback menjalankan fn dalam transaksi yang selalu dibatalkan, untuk memeriksa aturan hapus tanpa
// merusak fixture yang masih dipakai langkah berikutnya.
func g2Rollback(t *testing.T, db *gorm.DB, fn func(tx *gorm.DB)) {
	t.Helper()
	sentinel := errors.New("rollback")
	err := db.Transaction(func(tx *gorm.DB) error {
		fn(tx)
		return sentinel
	})
	require.ErrorIs(t, err, sentinel)
}

// g2IsiDataLama mengisi basis data versi 111 dengan baris berbentuk data lama.
func g2IsiDataLama(t *testing.T, db *gorm.DB) {
	t.Helper()
	admin, manajer := uuidPengguna("superadmin"), uuidPengguna("manajer_risiko")
	sql := []string{
		`INSERT INTO provinsi (kode, nama) VALUES ('11', 'Aceh')`,
		`INSERT INTO kabupaten (kode, nama, jenis, ibukota, lat, lng, provinsi_kode) VALUES
			('1171', 'Banda Aceh', 'KOTA', 'Banda Aceh', 5.55, 95.32, '11'),
			('1174', 'Lhokseumawe', 'KOTA', 'Lhokseumawe', 5.18, 97.15, '11')`,
		`INSERT INTO komoditas (kode, nama, satuan, kategori, harga_acuan, shelf_life_hari) VALUES
			('CBM-KRT', 'Cabai Merah', 'kg', 'cabai', 50000, 7), ('BWM', 'Bawang Merah', 'kg', 'bawang', 40000, 30)`,
	}
	for _, s := range sql {
		exec(t, db, s)
	}
	exec(t, db, `INSERT INTO pengguna (id, nama, email, peran, lapis, lembaga) VALUES
		(?, 'Super Admin', 'superadmin@x.id', 'ADMIN_SISTEM', 'SISTEM', 'BPPSDMP'),
		(?, 'Manajer Risiko', 'risiko@x.id', 'OPS_MAHASISWA', 'OPERASIONAL', 'FMIPA')`, admin, manajer)

	// aturan_ews: urutan ID baru menurut diubah_pada, NULL terakhir.
	exec(t, db, `INSERT INTO aturan_ews (id, komoditas_kode, kabupaten_kode, jenis, ambang_waspada, ambang_siaga, ambang_kritis, jendela_hari, diubah_pada, diubah_oleh_id) VALUES
		('ATR-LH-BWM', 'BWM', NULL, 'LONJAKAN_HARGA', 8, 12, 18, 7, '2026-01-02T00:00:00Z', ?),
		('ATR-LH-CBM-KRT', 'CBM-KRT', '1171', 'LONJAKAN_HARGA', 8, 12, 18, 7, '2026-01-01T00:00:00Z', ?),
		('ATR-KS-CBM-KRT', 'CBM-KRT', NULL, 'KELANGKAAN_STOK', 25, 40, 55, 5, NULL, NULL)`, admin, admin)
	// aturan_ews_riwayat memakai default 'AR-' || nextval: AR-1 (waktu paling akhir kedua), AR-2 (paling awal), AR-3.
	exec(t, db, `INSERT INTO aturan_ews_riwayat (aturan_id, versi, ambang_waspada, ambang_siaga, ambang_kritis, jendela_hari, cooldown_jam, kanal, aktif, diubah_oleh_id, waktu) VALUES
		('ATR-LH-BWM', 2, 8, 12, 18, 7, 24, ARRAY['DASHBOARD','PUSH'], true, ?, '2026-03-02T00:00:00Z'),
		('ATR-LH-BWM', 1, 8, 12, 18, 7, 24, ARRAY['DASHBOARD','PUSH'], true, NULL, '2026-03-01T00:00:00Z'),
		('ATR-LH-CBM-KRT', 1, 8, 12, 18, 7, 24, ARRAY['DASHBOARD'], true, NULL, '2026-03-03T00:00:00Z')`, admin)

	// ews_alert: EWS-9 dan EWS-10 punya dibuat_pada yang sama; pemutus seri harus urutan alami (9 < 10).
	exec(t, db, `INSERT INTO ews_alert (id, severity, jenis, judul, pesan, komoditas_kode, kabupaten_kode, nilai, satuan_nilai, skor_dampak, status, sumber, dibuat_pada, aturan_id, dedup_key, diakui_oleh_id) VALUES
		('EWS-0901', 'WASPADA', 'ANOMALI_CUACA', 'Hujan', 'p', 'CBM-KRT', '1171', 148, 'mm', 81, 'DIAKUI', 's', '2026-04-01T00:00:00Z', NULL, NULL, ?),
		('EWS-1000', 'KRITIS', 'LONJAKAN_HARGA', 'Lonjakan', 'p', 'CBM-KRT', '1171', 21, '%', 90, 'BARU', 's', '2026-04-02T00:00:00Z', 'ATR-LH-CBM-KRT', 'LONJAKAN_HARGA|CBM-KRT|1171', NULL),
		('EWS-1001', 'SIAGA', 'LONJAKAN_HARGA', 'Selesai', 'p', 'BWM', '1174', 12, '%', 70, 'SELESAI', 's', '2026-04-03T00:00:00Z', 'ATR-LH-BWM', NULL, NULL),
		('EWS-2000', 'SIAGA', 'LONJAKAN_HARGA', 'Sistem', 'p', 'BWM', '1171', 13, '%', 70, 'BARU', 'Rule engine EWS', '2026-04-04T00:00:00Z', 'ATR-LH-BWM', 'LONJAKAN_HARGA|BWM|1171', NULL),
		('EWS-9', 'INFO', 'OVER_SUPPLY', 'Seri 9', 'p', NULL, '1174', -8, '%', 30, 'BARU', 's', '2026-04-05T00:00:00Z', NULL, NULL, NULL),
		('EWS-10', 'INFO', 'OVER_SUPPLY', 'Seri 10', 'p', NULL, '1174', -9, '%', 30, 'BARU', 's', '2026-04-05T00:00:00Z', NULL, NULL, NULL)`, manajer)
	exec(t, db, `INSERT INTO ews_event (alert_id) VALUES ('EWS-1000'), ('EWS-2000'), ('EWS-1000')`)
	exec(t, db, `INSERT INTO ews_notifikasi (alert_id, pengguna_id, kanal, tujuan) VALUES
		('EWS-1000', ?, 'EMAIL', 'risiko@x.id'), ('EWS-1000', ?, 'PUSH', ?), ('EWS-0901', ?, 'EMAIL', 'risiko@x.id')`, manajer, manajer, manajer, manajer)
	// ews_riwayat: RW-1 dan RW-2 dari default; RW-9 dan RW-10 punya waktu sama; id gaya seeder.
	exec(t, db, `INSERT INTO ews_riwayat (alert_id, dari, ke, oleh, waktu, catatan) VALUES
		('EWS-1000', NULL, 'BARU', NULL, '2026-04-02T00:00:00Z', 'dibuat'),
		('EWS-1000', 'BARU', 'DIAKUI', ?, '2026-04-02T01:00:00Z', 'diakui')`, admin)
	exec(t, db, `INSERT INTO ews_riwayat (id, alert_id, dari, ke, oleh, waktu) VALUES
		('RW-10', 'EWS-2000', NULL, 'BARU', NULL, '2026-04-04T00:00:00Z'),
		('RW-9', 'EWS-2000', 'BARU', 'DIAKUI', NULL, '2026-04-04T00:00:00Z'),
		('RW-D-EWS-1000-0', 'EWS-0901', NULL, 'BARU', NULL, '2026-04-01T00:00:00Z')`)

	// Intervensi dan anak-anaknya.
	exec(t, db, `INSERT INTO tahap_intervensi_def (siklus, kode_tahap, urutan, label) VALUES
		('NORMAL', 'n-pantau', 1, 'Pantau'), ('TEKANAN', 't-ews', 1, 'EWS'), ('TEKANAN', 't-petakan', 2, 'Petakan')`)
	exec(t, db, `INSERT INTO intervensi (id, siklus, tahap, judul, komoditas_kode, kabupaten_kode, prioritas, tenggat, ews_alert_id, dibuat_oleh_id) VALUES
		('INT-301', 'NORMAL', 'n-pantau', 'Rekap', 'BWM', '1171', 'RENDAH', now() + interval '3 day', NULL, ?),
		('INT-1000', 'TEKANAN', 't-ews', 'Dari alert', 'CBM-KRT', '1171', 'TINGGI', now() + interval '3 day', 'EWS-1000', NULL),
		('INT-302', 'TEKANAN', 't-petakan', 'Petakan', 'CBM-KRT', '1174', 'SEDANG', now() + interval '3 day', NULL, ?)`, admin, admin)
	exec(t, db, `INSERT INTO intervensi_pj (intervensi_id, pengguna_id) VALUES ('INT-301', ?), ('INT-1000', ?)`, manajer, manajer)
	exec(t, db, `INSERT INTO intervensi_riwayat (intervensi_id, aksi, ke, oleh) VALUES ('INT-301', 'BUAT', 'n-pantau', ?), ('INT-1000', 'BUAT', 't-ews', NULL)`, admin)
	exec(t, db, `INSERT INTO alur_persetujuan (kode, nama, jenis, siklus, tahap_tujuan) VALUES
		('pindah-tekanan-petakan', 'Petakan', 'PINDAH_TAHAP', 'TEKANAN', 't-petakan'), ('eskalasi', 'Eskalasi', 'ESKALASI', NULL, NULL)`)
	exec(t, db, `INSERT INTO alur_persetujuan_langkah (alur_kode, urutan, label, izin_dibutuhkan) VALUES ('pindah-tekanan-petakan', 1, 'Dewan', 'intervensi.setujui')`)
	exec(t, db, `INSERT INTO intervensi_persetujuan (intervensi_id, alur_kode, jenis, tahap_tujuan, status, diajukan_oleh) VALUES
		('INT-302', 'pindah-tekanan-petakan', 'PINDAH_TAHAP', 't-petakan', 'MENUNGGU', ?),
		('INT-301', 'eskalasi', 'ESKALASI', NULL, 'DISETUJUI', ?)`, admin, admin)
	exec(t, db, `INSERT INTO intervensi_persetujuan_langkah (persetujuan_id, urutan, label, izin_dibutuhkan, keputusan, oleh) VALUES
		(1, 1, 'Dewan', 'intervensi.setujui', NULL, NULL), (2, 1, 'Dewan', 'intervensi.setujui', 'SETUJU', ?)`, manajer)

	// Rujukan lunak audit_log.
	exec(t, db, `INSERT INTO audit_log (id, aktor_id, aksi, entitas, entitas_id, hasil) VALUES
		('A1', ?, 'BUAT_OTOMATIS', 'ews_alert', 'EWS-1000', 'BERHASIL'),
		('A2', ?, 'EVALUASI', 'ews_alert', 'rule engine', 'BERHASIL'),
		('A3', ?, 'BULK_AKUI', 'ews_alert', '3 alert', 'BERHASIL'),
		('A4', ?, 'UBAH_AMBANG', 'aturan_ews', 'ATR-LH-BWM', 'BERHASIL'),
		('A5', ?, 'PINDAH_TAHAP', 'intervensi', 'INT-301', 'BERHASIL'),
		('A6', ?, 'PINDAH_TAHAP', 'intervensi', 'INT-DIHAPUS', 'BERHASIL'),
		('A7', ?, 'LOGIN', 'pengguna', 'EWS-1000', 'BERHASIL'),
		('A8', ?, 'AKUI_EWS', 'ews_alert', 'EWS-77', 'BERHASIL')`, admin, admin, admin, admin, admin, admin, admin, admin)
}

// g2Kolom membaca tipe kolom (format_type) satu tabel.
func g2Tipe(t *testing.T, db *gorm.DB, tabel, kolom string) string {
	t.Helper()
	return scanString(t, db, `SELECT format_type(atttypid, atttypmod) FROM pg_attribute WHERE attrelid = ?::regclass AND attname = ?`, tabel, kolom)
}

func g2Identity(t *testing.T, db *gorm.DB, tabel string) string {
	t.Helper()
	return scanString(t, db, `SELECT attidentity::text FROM pg_attribute WHERE attrelid = ?::regclass AND attname = 'id'`, tabel)
}

func g2AdaIndeks(t *testing.T, db *gorm.DB, nama string) bool {
	t.Helper()
	return scanCount(t, db, `SELECT count(*) FROM pg_indexes WHERE schemaname = 'public' AND indexname = ?`, nama) == 1
}

// g2AturanHapus membaca ON DELETE sebuah FK menurut nama constraint.
func g2AturanHapus(t *testing.T, db *gorm.DB, nama string) string {
	t.Helper()
	return scanString(t, db, `SELECT CASE confdeltype WHEN 'a' THEN 'NO ACTION' WHEN 'r' THEN 'RESTRICT' WHEN 'c' THEN 'CASCADE'
		WHEN 'n' THEN 'SET NULL' ELSE 'SET DEFAULT' END FROM pg_constraint WHERE conname = ? AND contype = 'f'`, nama)
}

func g2KodeKe(t *testing.T, db *gorm.DB, tabel, kode string) string {
	t.Helper()
	return scanString(t, db, `SELECT id::text FROM `+tabel+` WHERE kode = ?`, kode)
}

// TestMigrasiEwsIntervensi_DataLamaIkutDanBisaDibalik: migrasi 000112 memetakan ID lama ke BIGINT (EWS) dan UUID
// stabil (intervensi) di semua tabel anak, menyimpan ID lama di kode, menegakkan ON DELETE eksplisit, dan Down
// memulihkan bentuk lama.
func TestMigrasiEwsIntervensi_DataLamaIkutDanBisaDibalik(t *testing.T) {
	db := pg.NewDatabase(t, "migrate_ews_intervensi")
	migrator := pgtest.Migrator(t, db)
	require.NoError(t, migrator.UpTo(t.Context(), versiSebelumEwsIntervensi))
	g2IsiDataLama(t, db)
	for tabel, kolom := range map[string]string{
		"aturan_ews": "id", "ews_alert": "id", "ews_riwayat": "id", "aturan_ews_riwayat": "id", "intervensi": "id",
		"ews_event": "alert_id", "intervensi_pj": "intervensi_id",
	} {
		assert.Contains(t, g2Tipe(t, db, tabel, kolom), "character varying", "%s.%s sebelum Up", tabel, kolom)
	}

	require.NoError(t, migrator.UpTo(t.Context(), versiEwsIntervensi), "up 000112")

	t.Run("kunci induk: BIGINT identity untuk EWS, UUID untuk intervensi", func(t *testing.T) {
		for _, tabel := range []string{"aturan_ews", "aturan_ews_riwayat", "ews_alert", "ews_riwayat", "ews_event", "ews_notifikasi", "intervensi_persetujuan"} {
			assert.Equal(t, "bigint", g2Tipe(t, db, tabel, "id"), tabel)
			assert.Equal(t, "a", g2Identity(t, db, tabel), tabel+" GENERATED ALWAYS")
		}
		assert.Equal(t, "uuid", g2Tipe(t, db, "intervensi", "id"))
		assert.Equal(t, "gen_random_uuid()", scanString(t, db, `SELECT pg_get_expr(adbin, adrelid) FROM pg_attrdef WHERE adrelid = 'intervensi'::regclass
			AND adnum = (SELECT attnum FROM pg_attribute WHERE attrelid = 'intervensi'::regclass AND attname = 'id')`))
		assert.Equal(t, "character varying(50)", g2Tipe(t, db, "alur_persetujuan", "kode"), "alur_persetujuan tetap KODE")
		for _, tabel := range []string{"aturan_ews", "aturan_ews_riwayat", "ews_alert", "ews_riwayat", "intervensi"} {
			assert.Equal(t, "character varying(64)", g2Tipe(t, db, tabel, "kode"), tabel)
			assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM pg_constraint WHERE conname = ? AND contype = 'u'`, "uq_"+tabel+"_kode"), tabel)
		}
		// Bawaan 'AR-'/'RW-' || nextval dan sequence lamanya dibuang; sequence kode berganti nama.
		for _, tabel := range []string{"aturan_ews_riwayat", "ews_riwayat"} {
			assert.Empty(t, scanString(t, db, `SELECT COALESCE(pg_get_expr(adbin, adrelid), '') FROM pg_attribute a LEFT JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
				WHERE a.attrelid = ?::regclass AND a.attname = 'id'`, tabel), tabel)
		}
		assert.Equal(t, "public.ews_alert_id_seq", scanString(t, db, `SELECT pg_get_serial_sequence('ews_alert', 'id')`), "identity memakai nama baku")
		assert.Equal(t, "public.aturan_ews_riwayat_id_seq", scanString(t, db, `SELECT pg_get_serial_sequence('aturan_ews_riwayat', 'id')`), "sequence lama diganti sequence identity")
		assert.Equal(t, "public.ews_riwayat_id_seq", scanString(t, db, `SELECT pg_get_serial_sequence('ews_riwayat', 'id')`))
		assert.EqualValues(t, 0, scanCount(t, db, `SELECT count(*) FROM pg_class WHERE relname = 'intervensi_id_seq' AND relkind = 'S'`), "intervensi.id UUID tidak punya sequence")
		assert.EqualValues(t, 2, scanCount(t, db, `SELECT count(*) FROM pg_class WHERE relname IN ('ews_alert_kode_seq', 'intervensi_kode_seq') AND relkind = 'S'`))
	})

	t.Run("ID baru berurutan menurut waktu, pemutus seri urutan alami, kode = ID lama", func(t *testing.T) {
		urut := func(tabel string) []string {
			return g2Daftar(t, db, `SELECT kode FROM `+tabel+` ORDER BY id`)
		}
		assert.Equal(t, []string{"ATR-LH-CBM-KRT", "ATR-LH-BWM", "ATR-KS-CBM-KRT"}, urut("aturan_ews"), "diubah_pada, NULL terakhir")
		assert.Equal(t, []string{"AR-2", "AR-1", "AR-3"}, urut("aturan_ews_riwayat"))
		assert.Equal(t, []string{"EWS-0901", "EWS-1000", "EWS-1001", "EWS-2000", "EWS-9", "EWS-10"}, urut("ews_alert"), "EWS-9 sebelum EWS-10")
		assert.Equal(t, []string{"RW-D-EWS-1000-0", "RW-1", "RW-2", "RW-9", "RW-10"}, urut("ews_riwayat"), "RW-9 sebelum RW-10")
		assert.Equal(t, "1", g2KodeKe(t, db, "aturan_ews", "ATR-LH-CBM-KRT"))
		assert.Equal(t, "4", g2KodeKe(t, db, "ews_alert", "EWS-2000"))

		// ews_event, ews_notifikasi, dan identity tabel yang sudah BIGINT tidak berubah nilainya.
		assert.Equal(t, []string{"1", "2", "3"}, g2Daftar(t, db, `SELECT id::text FROM ews_event ORDER BY id`))
		assert.Equal(t, []string{"1", "2"}, g2Daftar(t, db, `SELECT id::text FROM intervensi_persetujuan ORDER BY id`))
	})

	t.Run("intervensi.id = garda_uuid_stabil dari ID lama", func(t *testing.T) {
		for _, kode := range []string{"INT-301", "INT-1000", "INT-302"} {
			assert.Equal(t, g2UUIDIntervensi(kode), g2KodeKe(t, db, "intervensi", kode), kode)
		}
	})

	t.Run("baris anak menunjuk ID baru yang benar", func(t *testing.T) {
		assert.Equal(t, []string{"EWS-1000", "EWS-2000", "EWS-1000"}, g2Daftar(t, db, `SELECT a.kode FROM ews_event e JOIN ews_alert a ON a.id = e.alert_id ORDER BY e.id`))
		assert.Equal(t, []string{"EWS-0901", "EWS-1000", "EWS-1000"}, g2Daftar(t, db, `SELECT a.kode FROM ews_notifikasi n JOIN ews_alert a ON a.id = n.alert_id ORDER BY a.id, n.id`))
		assert.Equal(t, []string{"EWS-0901", "EWS-1000", "EWS-1000", "EWS-2000", "EWS-2000"}, g2Daftar(t, db, `SELECT a.kode FROM ews_riwayat r JOIN ews_alert a ON a.id = r.alert_id ORDER BY r.id`))
		assert.Equal(t, []string{"ATR-LH-BWM", "ATR-LH-BWM", "ATR-LH-CBM-KRT"}, g2Daftar(t, db, `SELECT t.kode FROM aturan_ews_riwayat r JOIN aturan_ews t ON t.id = r.aturan_id ORDER BY t.kode, r.id`))
		assert.Equal(t, []string{"ATR-LH-CBM-KRT", "ATR-LH-BWM", "ATR-LH-BWM"}, g2Daftar(t, db, `SELECT t.kode FROM ews_alert a JOIN aturan_ews t ON t.id = a.aturan_id ORDER BY a.id`))
		assert.EqualValues(t, 3, scanCount(t, db, `SELECT count(*) FROM ews_alert WHERE aturan_id IS NULL`), "alert tanpa aturan tetap NULL")
		assert.Equal(t, "EWS-1000", scanString(t, db, `SELECT a.kode FROM intervensi i JOIN ews_alert a ON a.id = i.ews_alert_id WHERE i.kode = 'INT-1000'`))
		assert.EqualValues(t, 2, scanCount(t, db, `SELECT count(*) FROM intervensi WHERE ews_alert_id IS NULL`), "kartu tanpa alert tetap NULL")
		assert.Equal(t, g2UUIDIntervensi("INT-301"), scanString(t, db, `SELECT intervensi_id::text FROM intervensi_pj WHERE pengguna_id = ? AND intervensi_id = ?::uuid`, uuidPengguna("manajer_risiko"), g2UUIDIntervensi("INT-301")))
		assert.Equal(t, []string{g2UUIDIntervensi("INT-301"), g2UUIDIntervensi("INT-1000")}, g2Daftar(t, db, `SELECT intervensi_id::text FROM intervensi_riwayat ORDER BY id`))
		assert.Equal(t, []string{g2UUIDIntervensi("INT-302"), g2UUIDIntervensi("INT-301")}, g2Daftar(t, db, `SELECT intervensi_id::text FROM intervensi_persetujuan ORDER BY id`))
		assert.Equal(t, "pindah-tekanan-petakan", scanString(t, db, `SELECT alur_kode FROM intervensi_persetujuan WHERE id = 1`))
		assert.EqualValues(t, 2, scanCount(t, db, `SELECT count(*) FROM intervensi_persetujuan_langkah`))
	})

	t.Run("rujukan lunak audit_log dipetakan, sisanya tidak, dan tetap append-only", func(t *testing.T) {
		assert.Equal(t, g2KodeKe(t, db, "ews_alert", "EWS-1000"), scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'A1'`))
		assert.Equal(t, "rule engine", scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'A2'`))
		assert.Equal(t, "3 alert", scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'A3'`))
		assert.Equal(t, g2KodeKe(t, db, "aturan_ews", "ATR-LH-BWM"), scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'A4'`))
		assert.Equal(t, g2UUIDIntervensi("INT-301"), scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'A5'`))
		assert.Equal(t, "INT-DIHAPUS", scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'A6'`), "kartu yang tidak ada tidak dikarang")
		assert.Equal(t, "EWS-1000", scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'A7'`), "entitas lain tidak disentuh")
		assert.Equal(t, "EWS-77", scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'A8'`), "alert yang tidak ada tidak dikarang")
		assert.Equal(t, "O", scanString(t, db, `SELECT tgenabled::text FROM pg_trigger WHERE tgname = 'trg_audit_log_append_only'`))
		err := db.Exec(`UPDATE audit_log SET aksi = 'DIUBAH' WHERE id = 'A1'`).Error
		require.Error(t, err)
		assert.Contains(t, err.Error(), "append-only")
	})

	t.Run("FK bertipe sama dengan induk dan punya ON DELETE eksplisit", func(t *testing.T) {
		aturan := map[string]string{
			"aturan_ews_riwayat_aturan_id_fkey":                  "CASCADE",
			"ews_alert_aturan_id_fkey":                           "SET NULL",
			"ews_event_alert_id_fkey":                            "CASCADE",
			"ews_notifikasi_alert_id_fkey":                       "CASCADE",
			"ews_riwayat_alert_id_fkey":                          "CASCADE",
			"intervensi_ews_alert_id_fkey":                       "RESTRICT",
			"intervensi_persetujuan_intervensi_id_fkey":          "CASCADE",
			"intervensi_pj_intervensi_id_fkey":                   "CASCADE",
			"intervensi_riwayat_intervensi_id_fkey":              "CASCADE",
			"intervensi_persetujuan_alur_kode_fkey":              "SET NULL",
			"aturan_ews_komoditas_kode_fkey":                     "RESTRICT",
			"aturan_ews_kabupaten_kode_fkey":                     "RESTRICT",
			"ews_alert_komoditas_kode_fkey":                      "RESTRICT",
			"ews_alert_kabupaten_kode_fkey":                      "RESTRICT",
			"intervensi_komoditas_kode_fkey":                     "RESTRICT",
			"intervensi_kabupaten_kode_fkey":                     "RESTRICT",
			"fk_intervensi_tahap":                                "RESTRICT",
			"fk_alur_persetujuan_tahap":                          "RESTRICT",
			"intervensi_persetujuan_langkah_persetujuan_id_fkey": "CASCADE",
		}
		for nama, harapan := range aturan {
			assert.Equal(t, harapan, g2AturanHapus(t, db, nama), nama)
		}
		for _, fk := range bacaFK(t, db) {
			switch fk.Anak {
			case "aturan_ews", "aturan_ews_riwayat", "ews_alert", "ews_riwayat", "ews_event", "ews_notifikasi",
				"intervensi", "intervensi_pj", "intervensi_riwayat", "intervensi_persetujuan", "alur_persetujuan":
				assert.Empty(t, pelanggaranFK(fk), fk.Nama)
			}
		}
	})

	t.Run("indeks: FK terindeks, komposit dan parsial baru ada, yang redundan dibuang", func(t *testing.T) {
		for _, nama := range []string{
			"idx_aturan_ews_komoditas_kode", "idx_aturan_ews_kabupaten_kode", "idx_ews_event_alert",
			"idx_ews_alert_status_dibuat", "idx_ews_alert_kabupaten_dibuat", "idx_ews_alert_aktif",
			"idx_intervensi_komoditas_kode", "idx_intervensi_kabupaten_kode", "idx_intervensi_aktif_tahap_sejak",
			"idx_intervensi_persetujuan_intervensi_id", "idx_intervensi_persetujuan_alur_kode",
			"idx_intervensi_persetujuan_status_diajukan", "idx_intervensi_persetujuan_menunggu", "idx_alur_persetujuan_tahap",
			// tetap ada (dibangun ulang oleh ALTER COLUMN TYPE)
			"idx_ews_alert_dibuat_pada", "idx_ews_alert_aturan", "idx_ews_alert_status_severity_waktu", "idx_ews_alert_komoditas_kabupaten_waktu",
			"uq_ews_alert_dedup_aktif", "idx_ews_riwayat_alert", "idx_intervensi_ews_alert", "uq_intervensi_otomatis_alert",
			"uq_intervensi_persetujuan_terbuka", "idx_intervensi_riwayat_kartu", "intervensi_pj_pkey", "uq_ews_notifikasi",
			"aturan_ews_riwayat_aturan_id_versi_key",
		} {
			assert.True(t, g2AdaIndeks(t, db, nama), nama)
		}
		assert.False(t, g2AdaIndeks(t, db, "idx_ews_status_severity"), "awalan idx_ews_alert_status_severity_waktu")
		assert.False(t, g2AdaIndeks(t, db, "idx_intervensi_persetujuan_status"), "diganti versi berurutan menurun")
		assert.Contains(t, scanString(t, db, `SELECT indexdef FROM pg_indexes WHERE indexname = 'idx_ews_alert_dibuat_pada'`), "(dibuat_pada DESC, id)")
		assert.Contains(t, scanString(t, db, `SELECT indexdef FROM pg_indexes WHERE indexname = 'idx_ews_alert_aktif'`), "WHERE (status <> 'SELESAI'")
		assert.Contains(t, scanString(t, db, `SELECT indexdef FROM pg_indexes WHERE indexname = 'idx_intervensi_persetujuan_menunggu'`), "'MENUNGGU'")
	})

	t.Run("aturan hapus: RESTRICT menahan, CASCADE ikut terhapus, SET NULL mengosongkan", func(t *testing.T) {
		// Alert pemicu kartu menahan penghapusan (bukti asal kartu).
		g2Rollback(t, db, func(tx *gorm.DB) {
			err := tx.Exec(`DELETE FROM ews_alert WHERE kode = 'EWS-1000'`).Error
			require.Error(t, err)
			assert.Contains(t, err.Error(), "23503")
		})
		// Rujukan master dan definisi tahap menahan.
		for _, sql := range []string{
			`DELETE FROM komoditas WHERE kode = 'BWM'`,
			`DELETE FROM kabupaten WHERE kode = '1171'`,
			`DELETE FROM tahap_intervensi_def WHERE kode_tahap = 't-petakan'`,
		} {
			g2Rollback(t, db, func(tx *gorm.DB) {
				err := tx.Exec(sql).Error
				require.Error(t, err, sql)
				assert.Contains(t, err.Error(), "23503", sql)
			})
		}
		// Alert tanpa kartu: event, notifikasi, dan riwayatnya ikut terhapus.
		g2Rollback(t, db, func(tx *gorm.DB) {
			require.NoError(t, tx.Exec(`DELETE FROM ews_alert WHERE kode = 'EWS-0901'`).Error)
			assert.Zero(t, scanCount(t, tx, `SELECT count(*) FROM ews_riwayat WHERE kode = 'RW-D-EWS-1000-0'`))
			assert.EqualValues(t, 2, scanCount(t, tx, `SELECT count(*) FROM ews_notifikasi`))
		})
		g2Rollback(t, db, func(tx *gorm.DB) {
			require.NoError(t, tx.Exec(`DELETE FROM intervensi WHERE kode = 'INT-301'`).Error)
			assert.Zero(t, scanCount(t, tx, `SELECT count(*) FROM intervensi_pj WHERE intervensi_id = ?::uuid`, g2UUIDIntervensi("INT-301")))
			assert.Zero(t, scanCount(t, tx, `SELECT count(*) FROM intervensi_riwayat WHERE intervensi_id = ?::uuid`, g2UUIDIntervensi("INT-301")))
			assert.EqualValues(t, 1, scanCount(t, tx, `SELECT count(*) FROM intervensi_persetujuan`), "permintaan kartu itu ikut terhapus")
			assert.EqualValues(t, 1, scanCount(t, tx, `SELECT count(*) FROM intervensi_persetujuan_langkah`), "langkahnya ikut terhapus")
		})
		// Aturan dihapus: riwayatnya ikut terhapus, alert kehilangan rujukan aturan.
		g2Rollback(t, db, func(tx *gorm.DB) {
			require.NoError(t, tx.Exec(`DELETE FROM aturan_ews WHERE kode = 'ATR-LH-BWM'`).Error)
			assert.EqualValues(t, 1, scanCount(t, tx, `SELECT count(*) FROM aturan_ews_riwayat`))
			assert.EqualValues(t, 5, scanCount(t, tx, `SELECT count(*) FROM ews_alert WHERE aturan_id IS NULL`))
		})
		// Alur dihapus: permintaan tetap ada tanpa rujukan alur.
		g2Rollback(t, db, func(tx *gorm.DB) {
			require.NoError(t, tx.Exec(`DELETE FROM alur_persetujuan WHERE kode = 'pindah-tekanan-petakan'`).Error)
			assert.EqualValues(t, 1, scanCount(t, tx, `SELECT count(*) FROM intervensi_persetujuan WHERE alur_kode IS NULL`))
		})
	})

	t.Run("identity berlanjut dari max+1, id eksplisit ditolak, kode baru dari sequence", func(t *testing.T) {
		exec(t, db, `INSERT INTO ews_alert (severity, jenis, judul, pesan, kabupaten_kode, nilai, satuan_nilai, skor_dampak, sumber, kode)
			VALUES ('INFO', 'OVER_SUPPLY', 'Baru', 'p', '1174', 1, '%', 1, 's', 'EWS-' || nextval('ews_alert_kode_seq'))`)
		assert.Equal(t, "7", g2KodeKe(t, db, "ews_alert", "EWS-2001"), "id berlanjut, kode sesudah EWS-2000")
		exec(t, db, `INSERT INTO aturan_ews_riwayat (aturan_id, versi, ambang_waspada, ambang_siaga, ambang_kritis, jendela_hari, cooldown_jam, kanal, aktif)
			VALUES (3, 1, 1, 2, 3, 7, 24, ARRAY['EMAIL'], true)`)
		assert.EqualValues(t, 4, scanCount(t, db, `SELECT max(id) FROM aturan_ews_riwayat`))
		exec(t, db, `INSERT INTO ews_riwayat (alert_id, ke) VALUES (7, 'BARU')`)
		assert.EqualValues(t, 6, scanCount(t, db, `SELECT max(id) FROM ews_riwayat`))
		exec(t, db, `INSERT INTO intervensi (siklus, tahap, judul, komoditas_kode, kabupaten_kode, prioritas, tenggat, kode)
			VALUES ('NORMAL', 'n-pantau', 'Baru', 'BWM', '1174', 'RENDAH', now(), 'INT-' || nextval('intervensi_kode_seq'))`)
		baru := scanString(t, db, `SELECT id::text FROM intervensi WHERE kode = 'INT-1001'`)
		assert.True(t, idx.ValidUUID(baru), baru)
		assert.NotEqual(t, g2UUIDIntervensi("INT-1001"), baru, "baris baru memakai gen_random_uuid")

		for _, sql := range []string{
			`INSERT INTO ews_alert (id, severity, jenis, judul, pesan, kabupaten_kode, nilai, satuan_nilai, skor_dampak, sumber) VALUES (999, 'INFO', 'OVER_SUPPLY', 'x', 'p', '1174', 1, '%', 1, 's')`,
			`INSERT INTO aturan_ews (id, jenis, ambang_waspada, ambang_siaga, ambang_kritis) VALUES (999, 'LONJAKAN_HARGA', 1, 2, 3)`,
			`INSERT INTO ews_riwayat (id, alert_id, ke) VALUES (999, 1, 'BARU')`,
		} {
			err := db.Exec(sql).Error
			require.Error(t, err, sql)
			assert.Contains(t, err.Error(), "non-DEFAULT", sql)
		}
		// Kode unik.
		err := db.Exec(`UPDATE ews_alert SET kode = 'EWS-1000' WHERE kode = 'EWS-2001'`).Error
		require.Error(t, err)
		assert.Contains(t, err.Error(), "uq_ews_alert_kode")
	})

	t.Run("down memulihkan ID lama, default, sequence, dan aturan semula", func(t *testing.T) {
		require.NoError(t, migrator.DownTo(t.Context(), versiSebelumEwsIntervensi), "down 000112")

		for _, k := range [][2]string{
			{"aturan_ews", "id"}, {"ews_alert", "id"}, {"ews_riwayat", "id"}, {"aturan_ews_riwayat", "id"}, {"intervensi", "id"},
			{"ews_event", "alert_id"}, {"ews_notifikasi", "alert_id"}, {"ews_riwayat", "alert_id"}, {"intervensi_pj", "intervensi_id"},
			{"intervensi_riwayat", "intervensi_id"}, {"intervensi_persetujuan", "intervensi_id"}, {"aturan_ews_riwayat", "aturan_id"},
			{"ews_alert", "aturan_id"}, {"intervensi", "ews_alert_id"},
		} {
			assert.Equal(t, "character varying(255)", g2Tipe(t, db, k[0], k[1]), k[0]+"."+k[1])
		}
		for _, tabel := range []string{"aturan_ews", "aturan_ews_riwayat", "ews_alert", "ews_riwayat", "intervensi"} {
			assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM pg_attribute WHERE attrelid = ?::regclass AND attname = 'kode' AND NOT attisdropped`, tabel), tabel+": kolom kode dibuang")
		}

		assert.ElementsMatch(t, []string{"ATR-KS-CBM-KRT", "ATR-LH-BWM", "ATR-LH-CBM-KRT"}, g2Daftar(t, db, `SELECT id FROM aturan_ews`))
		assert.ElementsMatch(t, []string{"EWS-0901", "EWS-10", "EWS-1000", "EWS-1001", "EWS-2000", "EWS-2001", "EWS-9"}, g2Daftar(t, db, `SELECT id FROM ews_alert`),
			"alert yang dibuat sesudah Up membawa kodenya")
		assert.ElementsMatch(t, []string{"INT-1000", "INT-1001", "INT-301", "INT-302"}, g2Daftar(t, db, `SELECT id FROM intervensi`))
		assert.Contains(t, g2Daftar(t, db, `SELECT id FROM ews_riwayat`), "RW-D-EWS-1000-0")
		assert.Contains(t, g2Daftar(t, db, `SELECT id FROM ews_riwayat`), "RW-10")
		assert.Contains(t, g2Daftar(t, db, `SELECT id FROM aturan_ews_riwayat`), "AR-2")

		// Baris tanpa kode memakai id::text.
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM ews_riwayat WHERE id = '6'`))
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM aturan_ews_riwayat WHERE id = '4'`))

		assert.Equal(t, []string{"EWS-1000", "EWS-2000", "EWS-1000"}, g2Daftar(t, db, `SELECT alert_id FROM ews_event ORDER BY id`), "event menunjuk ID lama")
		assert.Equal(t, "EWS-1000", scanString(t, db, `SELECT ews_alert_id FROM intervensi WHERE id = 'INT-1000'`))
		assert.Equal(t, "ATR-LH-CBM-KRT", scanString(t, db, `SELECT aturan_id FROM ews_alert WHERE id = 'EWS-1000'`))
		assert.Equal(t, "INT-301", scanString(t, db, `SELECT intervensi_id FROM intervensi_pj WHERE intervensi_id = 'INT-301'`))
		assert.Equal(t, "INT-302", scanString(t, db, `SELECT intervensi_id FROM intervensi_persetujuan WHERE id = 1`))
		assert.Equal(t, "EWS-1000", scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'A1'`), "rujukan lunak kembali")
		assert.Equal(t, "ATR-LH-BWM", scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'A4'`))
		assert.Equal(t, "INT-301", scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'A5'`))
		assert.Equal(t, "O", scanString(t, db, `SELECT tgenabled::text FROM pg_trigger WHERE tgname = 'trg_audit_log_append_only'`))

		// Default dan sequence lama kembali, dimajukan melewati nomor yang terpakai.
		assert.Equal(t, "('RW-'::text || (nextval('ews_riwayat_id_seq'::regclass))::text)", scanString(t, db,
			`SELECT pg_get_expr(adbin, adrelid) FROM pg_attrdef WHERE adrelid = 'ews_riwayat'::regclass`))
		exec(t, db, `INSERT INTO ews_riwayat (alert_id, ke) VALUES ('EWS-1000', 'BARU')`)
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM ews_riwayat WHERE id = 'RW-11'`), "sequence RW melewati RW-10")
		exec(t, db, `INSERT INTO aturan_ews_riwayat (aturan_id, versi, ambang_waspada, ambang_siaga, ambang_kritis, jendela_hari, cooldown_jam, kanal, aktif)
			VALUES ('ATR-KS-CBM-KRT', 2, 1, 2, 3, 7, 24, ARRAY['EMAIL'], true)`)
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM aturan_ews_riwayat WHERE id = 'AR-4'`), "sequence AR melewati AR-3")
		assert.EqualValues(t, 2001, scanCount(t, db, `SELECT last_value FROM pg_sequences WHERE sequencename = 'ews_alert_id_seq'`), "sequence kode kembali bernama ews_alert_id_seq")
		assert.EqualValues(t, 0, scanCount(t, db, `SELECT count(*) FROM pg_class WHERE relname IN ('ews_alert_kode_seq', 'intervensi_kode_seq') AND relkind = 'S'`))
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM pg_class WHERE relname = 'intervensi_id_seq' AND relkind = 'S'`))

		// ON DELETE semula (NO ACTION = bawaan) dan indeks baru dibuang.
		for _, nama := range []string{
			"intervensi_ews_alert_id_fkey", "aturan_ews_komoditas_kode_fkey", "aturan_ews_kabupaten_kode_fkey", "ews_alert_komoditas_kode_fkey",
			"ews_alert_kabupaten_kode_fkey", "intervensi_komoditas_kode_fkey", "intervensi_kabupaten_kode_fkey", "fk_intervensi_tahap", "fk_alur_persetujuan_tahap",
		} {
			assert.Equal(t, "NO ACTION", g2AturanHapus(t, db, nama), nama)
		}
		assert.Equal(t, "CASCADE", g2AturanHapus(t, db, "ews_event_alert_id_fkey"))
		assert.Equal(t, "SET NULL", g2AturanHapus(t, db, "ews_alert_aturan_id_fkey"))
		for _, nama := range []string{"idx_ews_alert_aktif", "idx_ews_event_alert", "idx_intervensi_persetujuan_menunggu", "idx_aturan_ews_komoditas_kode", "idx_alur_persetujuan_tahap"} {
			assert.False(t, g2AdaIndeks(t, db, nama), nama)
		}
		assert.True(t, g2AdaIndeks(t, db, "idx_ews_status_severity"))
		assert.True(t, g2AdaIndeks(t, db, "idx_intervensi_persetujuan_status"))
		assert.True(t, g2AdaIndeks(t, db, "idx_ews_alert_dibuat_pada"))
		assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM pg_proc WHERE proname LIKE 'g2\_%'`), "fungsi bantu sementara tidak tersisa")
	})

	t.Run("up ulang atas data yang sudah dibalik menghasilkan UUID yang sama dan urutan yang sama untuk baris lama", func(t *testing.T) {
		require.NoError(t, migrator.UpTo(t.Context(), versiEwsIntervensi), "up kedua")
		assert.Equal(t, g2UUIDIntervensi("INT-301"), g2KodeKe(t, db, "intervensi", "INT-301"))
		assert.Equal(t, g2UUIDIntervensi("INT-1000"), g2KodeKe(t, db, "intervensi", "INT-1000"))
		assert.Equal(t, "1", g2KodeKe(t, db, "ews_alert", "EWS-0901"))
		assert.Equal(t, "1", g2KodeKe(t, db, "aturan_ews", "ATR-LH-CBM-KRT"))
		assert.Equal(t, "EWS-1000", scanString(t, db, `SELECT a.kode FROM intervensi i JOIN ews_alert a ON a.id = i.ews_alert_id WHERE i.kode = 'INT-1000'`))
		assert.Equal(t, g2KodeKe(t, db, "ews_alert", "EWS-1000"), scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'A1'`))
		// Kode baru tidak bentrok: sequence dimajukan melewati EWS-2001 dan INT-1001.
		assert.EqualValues(t, 2002, scanCount(t, db, `SELECT nextval('ews_alert_kode_seq')`))
		assert.EqualValues(t, 1002, scanCount(t, db, `SELECT nextval('intervensi_kode_seq')`))
	})
}

// TestMigrasiEwsIntervensi_TabelKosongDanSequenceKodeTidakMundur: migrasi pada basis data tanpa baris (seperti
// instalasi baru) memberi identity mulai dari 1 dan tidak memundurkan sequence kode di atas angka seed.
func TestMigrasiEwsIntervensi_TabelKosongDanSequenceKodeTidakMundur(t *testing.T) {
	db := pg.NewDatabase(t, "migrate_ews_intervensi_kosong")
	migrator := pgtest.Migrator(t, db)
	require.NoError(t, migrator.UpTo(t.Context(), versiEwsIntervensi))

	assert.EqualValues(t, 2000, scanCount(t, db, `SELECT nextval('ews_alert_kode_seq')`), "tanpa kode lama, sequence tetap mulai di 2000")
	assert.EqualValues(t, 1000, scanCount(t, db, `SELECT nextval('intervensi_kode_seq')`))
	assert.EqualValues(t, 1, scanCount(t, db, `SELECT seqstart FROM pg_sequence WHERE seqrelid = pg_get_serial_sequence('ews_riwayat', 'id')::regclass`))
	assert.EqualValues(t, 1, scanCount(t, db, `SELECT seqstart FROM pg_sequence WHERE seqrelid = pg_get_serial_sequence('aturan_ews', 'id')::regclass`))
}
