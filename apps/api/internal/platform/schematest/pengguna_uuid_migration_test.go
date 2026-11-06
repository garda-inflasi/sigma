package schematest

import (
	"testing"

	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Migrasi 000110 mengubah pengguna.id VARCHAR menjadi UUID. Tes dipatok ke versi 109 dan 110 (bukan "terbaru")
// supaya migrasi konversi berikutnya (000111+) tidak mengubah bentuk fixture ID lama di bawah.
const (
	versiSebelumPenggunaUUID = 109
	versiPenggunaUUID        = 110
)

// idPenggunaLama adalah ID lama (slug) yang dipakai fixture: slug seed, ID uji, dan format USR-.
var idPenggunaLama = []string{"superadmin", "mahasiswa_9", "user_nonaktif", "test-user-001", "USR-2024-0001"}

func uuidPengguna(slug string) string { return idx.UUIDStabil("pengguna", slug) }

func exec(t *testing.T, db *gorm.DB, sql string, args ...any) {
	t.Helper()
	require.NoError(t, db.Exec(sql, args...).Error, sql)
}

func scanString(t *testing.T, db *gorm.DB, sql string, args ...any) string {
	t.Helper()
	var out string
	require.NoError(t, db.Raw(sql, args...).Scan(&out).Error, sql)
	return out
}

func scanCount(t *testing.T, db *gorm.DB, sql string, args ...any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Raw(sql, args...).Scan(&n).Error, sql)
	return n
}

// isiDataPenggunaLama mengisi basis data versi 109 dengan baris berbentuk data lama: lima pengguna ber-ID slug
// dan baris anak di banyak tabel (CASCADE, SET NULL, RESTRICT/NO ACTION), termasuk audit_log entitas 'pengguna'.
func isiDataPenggunaLama(t *testing.T, db *gorm.DB) {
	t.Helper()
	sql := []string{
		`INSERT INTO provinsi (kode, nama) VALUES ('11', 'Aceh')`,
		`INSERT INTO kabupaten (kode, nama, jenis, ibukota, lat, lng, provinsi_kode) VALUES ('1174', 'Lhokseumawe', 'KOTA', 'Lhokseumawe', 5.18, 97.15, '11')`,
		`INSERT INTO pasar (id, nama, kabupaten_kode, lat, lng) VALUES ('PSR-1', 'Pasar Inti', '1174', 5.18, 97.15)`,
		`INSERT INTO komoditas (kode, nama, satuan, kategori, harga_acuan, shelf_life_hari) VALUES ('CBM-KRT', 'Cabai Merah', 'kg', 'cabai', 50000, 7)`,

		`INSERT INTO pengguna (id, nama, email, peran, lapis, lembaga, kabupaten_kode) VALUES
			('superadmin', 'Super Admin', 'superadmin@x.id', 'ADMIN_SISTEM', 'SISTEM', 'BPPSDMP', NULL),
			('mahasiswa_9', 'Mahasiswa 9', 'mahasiswa9@x.id', 'OPS_MAHASISWA', 'OPERASIONAL', 'FMIPA', '1174'),
			('user_nonaktif', 'Nonaktif', 'nonaktif@x.id', 'OPS_MAHASISWA', 'OPERASIONAL', 'FMIPA', NULL),
			('test-user-001', 'Uji 001', 'uji001@x.id', 'OPS_KOORDINATOR', 'OPERASIONAL', 'BPPSDMP', NULL),
			('USR-2024-0001', 'Undangan', 'usr@x.id', 'OPS_MAHASISWA', 'OPERASIONAL', 'FMIPA', NULL)`,

		// Milik akun (CASCADE) dan jejak (SET NULL).
		`INSERT INTO refresh_tokens (user_id, token_hash, family_id, expires_at) VALUES ('mahasiswa_9', 'rt-hash-1', gen_random_uuid(), now() + interval '1 day')`,
		`INSERT INTO login_attempts (user_id, success) VALUES ('superadmin', true), (NULL, false)`,
		`INSERT INTO mfa_kode_cadangan (user_id, kode_hash) VALUES ('test-user-001', 'hash-mfa-1')`,
		`INSERT INTO password_reset_tokens (user_id, token_hash, expires_at) VALUES ('mahasiswa_9', 'prt-hash-1', now() + interval '1 hour')`,
		`INSERT INTO undangan_pengguna (user_id, token_hash, email, diundang_oleh, expires_at) VALUES ('USR-2024-0001', 'und-hash-1', 'usr@x.id', 'superadmin', now() + interval '3 days')`,
		`INSERT INTO roles (kunci, label, lapis, urutan, diperbarui_oleh) VALUES ('ADMIN_SISTEM', 'Admin Sistem', 'SISTEM', 1, 'superadmin')`,
		`INSERT INTO permissions (nama, deskripsi, kelompok) VALUES ('harga.lihat', 'Lihat harga', 'harga')`,
		`INSERT INTO user_permissions (pengguna_id, permission_nama, alasan, diberikan_oleh) VALUES ('mahasiswa_9', 'harga.lihat', 'uji', 'superadmin')`,
		`INSERT INTO preferensi_notifikasi (pengguna_id, kanal) VALUES ('mahasiswa_9', 'EMAIL')`,
		`INSERT INTO langganan_push (pengguna_id, endpoint, p256dh, auth) VALUES ('mahasiswa_9', 'https://push.example/1', 'k', 'a')`,

		// Audit: rujukan keras (aktor_id) dan rujukan lunak (entitas_id).
		`INSERT INTO audit_log (id, aktor_id, aksi, entitas, entitas_id, hasil) VALUES
			('A1', 'superadmin', 'ASSIGN_ROLE', 'pengguna', 'mahasiswa_9', 'BERHASIL'),
			('A2', 'superadmin', 'EKSPOR_DATA', 'pengguna', 'ekspor', 'BERHASIL'),
			('A3', 'mahasiswa_9', 'HARGA_INPUT', 'entri_harga', 'mahasiswa_9', 'BERHASIL'),
			('A4', NULL, 'LOGIN', 'pengguna', 'sudah-dihapus', 'GAGAL'),
			('A5', 'test-user-001', 'LOGIN', 'pengguna', 'test-user-001', 'BERHASIL')`,

		// Bukti asal data (RESTRICT) dan riwayat.
		`INSERT INTO entri_harga (id, komoditas_kode, pasar_id, tipe, harga, satuan, dicatat_pada, petugas_id, divalidasi_oleh, dipublikasi_oleh, ditolak_oleh)
			VALUES ('EH-1', 'CBM-KRT', 'PSR-1', 'KONSUMEN', 52000, 'kg', now(), 'mahasiswa_9', 'superadmin', 'superadmin', 'superadmin')`,
		`INSERT INTO riwayat_status_entri (entri_id, ke, oleh) VALUES ('EH-1', 'DIVALIDASI', 'superadmin')`,
		`INSERT INTO tugas_lapangan (id, judul, pasar_id, petugas_id, komoditas, tipe, tenggat, target, dibuat_oleh_id)
			VALUES ('TGS-1', 'Survei', 'PSR-1', 'mahasiswa_9', ARRAY['CBM-KRT'], 'KONSUMEN', now() + interval '1 day', 3, 'superadmin')`,
		`INSERT INTO ews_alert (id, severity, jenis, judul, pesan, kabupaten_kode, nilai, satuan_nilai, skor_dampak, sumber, diakui_oleh_id, dieskalasi_oleh_id)
			VALUES ('EWS-1', 'WASPADA', 'LONJAKAN_HARGA', 'Lonjakan', 'pesan', '1174', 10, '%', 5, 'rule', 'superadmin', 'user_nonaktif')`,
		`INSERT INTO ews_riwayat (alert_id, ke, oleh) VALUES ('EWS-1', 'DIAKUI', 'superadmin')`,
		`INSERT INTO ews_notifikasi (alert_id, pengguna_id, kanal, tujuan) VALUES
			('EWS-1', 'mahasiswa_9', 'EMAIL', 'mahasiswa9@x.id'), ('EWS-1', 'mahasiswa_9', 'PUSH', 'mahasiswa_9')`,
		`INSERT INTO aturan_ews (id, jenis, ambang_waspada, ambang_siaga, ambang_kritis, diubah_oleh_id) VALUES ('AR-1', 'LONJAKAN_HARGA', 1, 2, 3, 'superadmin')`,
		`INSERT INTO tahap_intervensi_def (siklus, kode_tahap, urutan, label) VALUES ('NORMAL', 'IDENTIFIKASI', 1, 'Identifikasi')`,
		`INSERT INTO intervensi (id, siklus, tahap, judul, komoditas_kode, kabupaten_kode, prioritas, tenggat, dibuat_oleh_id)
			VALUES ('INT-1', 'NORMAL', 'IDENTIFIKASI', 'Operasi pasar', 'CBM-KRT', '1174', 'TINGGI', now() + interval '7 day', 'superadmin')`,
		`INSERT INTO intervensi_pj (intervensi_id, pengguna_id) VALUES ('INT-1', 'mahasiswa_9')`,
		`INSERT INTO intervensi_riwayat (intervensi_id, aksi, oleh) VALUES ('INT-1', 'BUAT', 'superadmin')`,
		`INSERT INTO policy_brief (id, periode, judul, ringkasan, rekomendasi, halaman, disusun_oleh_id)
			VALUES ('PB-1', '2026-10', 'Brief', 'ringkas', '[]'::jsonb, 1, 'superadmin')`,
		`INSERT INTO policy_brief_versi (brief_id, versi, konten, oleh) VALUES ('PB-1', 1, '{}'::jsonb, 'superadmin')`,
		`INSERT INTO model_registry (id, komoditas_kode, nama, versi, mape, dilatih_pada, dipromosikan_oleh)
			VALUES ('MDL-1', 'CBM-KRT', 'Model', 'v1', 0.1, now(), 'superadmin')`,
		`INSERT INTO model_promosi (id, model_id, komoditas_kode, aksi, status_lama, status_baru, oleh)
			VALUES ('PRM-1', 'MDL-1', 'CBM-KRT', 'PROMOSI', 'KANDIDAT', 'PRODUKSI', 'superadmin')`,
		`INSERT INTO skenario_simulasi (id, komoditas_kode, kabupaten_kode, horizon_hari, parameter, hasil, dibuat_oleh)
			VALUES ('SIM-1', 'CBM-KRT', '1174', 7, '{}'::jsonb, '{}'::jsonb, 'superadmin')`,
		`INSERT INTO sumber_data (id, nama, penyedia, jenis, cakupan, jadwal_cron, atribusi)
			VALUES ('SRC-1', 'Sumber', 'Penyedia', 'API', 'Aceh', '0 6 * * *', 'atribusi')`,
		`INSERT INTO eksekusi_sumber (sumber_id, pemicu, aktor_id, status, selesai) VALUES
			('SRC-1', 'MANUAL', 'superadmin', 'SUKSES', now()), ('SRC-1', 'JADWAL', NULL, 'SUKSES', now())`,
		`INSERT INTO perubahan_master (id, entitas, entitas_id, versi, perubahan, diajukan_oleh_id, disetujui_oleh_id)
			VALUES ('PM-1', 'komoditas', 'CBM-KRT', 1, '{}'::jsonb, 'mahasiswa_9', 'superadmin')`,
	}
	for _, s := range sql {
		exec(t, db, s)
	}
}

// kolomFKPengguna membaca setiap kolom FK ke pengguna(id): tabel, kolom, tipe, ON DELETE.
type kolomFKPengguna struct {
	Tabel, Kolom, Tipe, SaatHapus string
}

func bacaKolomFKPengguna(t *testing.T, db *gorm.DB) []kolomFKPengguna {
	t.Helper()
	var rows []kolomFKPengguna
	require.NoError(t, db.Raw(`
		SELECT con.conrelid::regclass::text AS tabel, a.attname AS kolom, format_type(a.atttypid, a.atttypmod) AS tipe,
		       CASE con.confdeltype WHEN 'a' THEN 'NO ACTION' WHEN 'r' THEN 'RESTRICT' WHEN 'c' THEN 'CASCADE'
		                            WHEN 'n' THEN 'SET NULL' ELSE 'SET DEFAULT' END AS saat_hapus
		  FROM pg_constraint con
		  JOIN pg_attribute a ON a.attrelid = con.conrelid AND a.attnum = con.conkey[1]
		 WHERE con.contype = 'f' AND con.confrelid = 'pengguna'::regclass
		 ORDER BY 1, 2`).Scan(&rows).Error)
	return rows
}

// TestMigrasiPenggunaUUID_DataLamaIkutDanBisaDibalik: migrasi 000110 memetakan ID slug ke UUID stabil di semua
// tabel (FK keras dan rujukan lunak audit_log), menyimpan ID lama untuk Down, dan Down memulihkannya persis.
func TestMigrasiPenggunaUUID_DataLamaIkutDanBisaDibalik(t *testing.T) {
	db := pg.NewDatabase(t, "migrate_pengguna_uuid")
	migrator := pgtest.Migrator(t, db)
	require.NoError(t, migrator.UpTo(t.Context(), versiSebelumPenggunaUUID))
	isiDataPenggunaLama(t, db)
	// Pengguna yang sudah ada sebelum Up tidak punya baris pemetaan sampai Up; akun baru sesudah Up tidak punya ID lama.
	require.Equal(t, "character varying(255)", scanString(t, db,
		`SELECT format_type(atttypid, atttypmod) FROM pg_attribute WHERE attrelid = 'pengguna'::regclass AND attname = 'id'`))

	require.NoError(t, migrator.UpTo(t.Context(), versiPenggunaUUID), "up 000110")

	t.Run("pengguna.id menjadi uuid stabil dengan bawaan acak", func(t *testing.T) {
		assert.Equal(t, "uuid", scanString(t, db,
			`SELECT format_type(atttypid, atttypmod) FROM pg_attribute WHERE attrelid = 'pengguna'::regclass AND attname = 'id'`))
		assert.Equal(t, "gen_random_uuid()", scanString(t, db,
			`SELECT pg_get_expr(adbin, adrelid) FROM pg_attrdef WHERE adrelid = 'pengguna'::regclass AND adnum =
			   (SELECT attnum FROM pg_attribute WHERE attrelid = 'pengguna'::regclass AND attname = 'id')`))
		for _, slug := range idPenggunaLama {
			email := map[string]string{
				"superadmin": "superadmin@x.id", "mahasiswa_9": "mahasiswa9@x.id", "user_nonaktif": "nonaktif@x.id",
				"test-user-001": "uji001@x.id", "USR-2024-0001": "usr@x.id",
			}[slug]
			assert.Equal(t, uuidPengguna(slug), scanString(t, db, `SELECT id::text FROM pengguna WHERE email = ?`, email), slug)
		}
	})

	t.Run("pemetaan ID lama tercatat satu banding satu", func(t *testing.T) {
		assert.EqualValues(t, len(idPenggunaLama), scanCount(t, db, `SELECT count(*) FROM pengguna_id_lama`))
		for _, slug := range idPenggunaLama {
			assert.Equal(t, slug, scanString(t, db, `SELECT id_lama FROM pengguna_id_lama WHERE id = ?`, uuidPengguna(slug)), slug)
		}
	})

	t.Run("baris anak ikut ke UUID yang sama", func(t *testing.T) {
		kasus := []struct{ nama, sql, slug string }{
			{"refresh_tokens.user_id", `SELECT user_id::text FROM refresh_tokens`, "mahasiswa_9"},
			{"login_attempts.user_id", `SELECT user_id::text FROM login_attempts WHERE user_id IS NOT NULL`, "superadmin"},
			{"mfa_kode_cadangan.user_id", `SELECT user_id::text FROM mfa_kode_cadangan`, "test-user-001"},
			{"password_reset_tokens.user_id", `SELECT user_id::text FROM password_reset_tokens`, "mahasiswa_9"},
			{"undangan_pengguna.user_id", `SELECT user_id::text FROM undangan_pengguna`, "USR-2024-0001"},
			{"undangan_pengguna.diundang_oleh", `SELECT diundang_oleh::text FROM undangan_pengguna`, "superadmin"},
			{"roles.diperbarui_oleh", `SELECT diperbarui_oleh::text FROM roles`, "superadmin"},
			{"user_permissions.pengguna_id", `SELECT pengguna_id::text FROM user_permissions`, "mahasiswa_9"},
			{"user_permissions.diberikan_oleh", `SELECT diberikan_oleh::text FROM user_permissions`, "superadmin"},
			{"preferensi_notifikasi.pengguna_id", `SELECT pengguna_id::text FROM preferensi_notifikasi`, "mahasiswa_9"},
			{"langganan_push.pengguna_id", `SELECT pengguna_id::text FROM langganan_push`, "mahasiswa_9"},
			{"audit_log.aktor_id", `SELECT aktor_id::text FROM audit_log WHERE id = 'A1'`, "superadmin"},
			{"entri_harga.petugas_id", `SELECT petugas_id::text FROM entri_harga`, "mahasiswa_9"},
			{"entri_harga.divalidasi_oleh", `SELECT divalidasi_oleh::text FROM entri_harga`, "superadmin"},
			{"entri_harga.dipublikasi_oleh", `SELECT dipublikasi_oleh::text FROM entri_harga`, "superadmin"},
			{"entri_harga.ditolak_oleh", `SELECT ditolak_oleh::text FROM entri_harga`, "superadmin"},
			{"riwayat_status_entri.oleh", `SELECT oleh::text FROM riwayat_status_entri`, "superadmin"},
			{"tugas_lapangan.petugas_id", `SELECT petugas_id::text FROM tugas_lapangan`, "mahasiswa_9"},
			{"tugas_lapangan.dibuat_oleh_id", `SELECT dibuat_oleh_id::text FROM tugas_lapangan`, "superadmin"},
			{"ews_alert.diakui_oleh_id", `SELECT diakui_oleh_id::text FROM ews_alert`, "superadmin"},
			{"ews_alert.dieskalasi_oleh_id", `SELECT dieskalasi_oleh_id::text FROM ews_alert`, "user_nonaktif"},
			{"ews_riwayat.oleh", `SELECT oleh::text FROM ews_riwayat`, "superadmin"},
			{"ews_notifikasi.pengguna_id", `SELECT pengguna_id::text FROM ews_notifikasi`, "mahasiswa_9"},
			{"aturan_ews.diubah_oleh_id", `SELECT diubah_oleh_id::text FROM aturan_ews`, "superadmin"},
			{"intervensi.dibuat_oleh_id", `SELECT dibuat_oleh_id::text FROM intervensi`, "superadmin"},
			{"intervensi_pj.pengguna_id", `SELECT pengguna_id::text FROM intervensi_pj`, "mahasiswa_9"},
			{"intervensi_riwayat.oleh", `SELECT oleh::text FROM intervensi_riwayat`, "superadmin"},
			{"policy_brief.disusun_oleh_id", `SELECT disusun_oleh_id::text FROM policy_brief`, "superadmin"},
			{"policy_brief_versi.oleh", `SELECT oleh::text FROM policy_brief_versi`, "superadmin"},
			{"model_registry.dipromosikan_oleh", `SELECT dipromosikan_oleh::text FROM model_registry`, "superadmin"},
			{"model_promosi.oleh", `SELECT oleh::text FROM model_promosi`, "superadmin"},
			{"skenario_simulasi.dibuat_oleh", `SELECT dibuat_oleh::text FROM skenario_simulasi`, "superadmin"},
			{"eksekusi_sumber.aktor_id", `SELECT aktor_id::text FROM eksekusi_sumber WHERE pemicu = 'MANUAL'`, "superadmin"},
			{"perubahan_master.diajukan_oleh_id", `SELECT diajukan_oleh_id::text FROM perubahan_master`, "mahasiswa_9"},
			{"perubahan_master.disetujui_oleh_id", `SELECT disetujui_oleh_id::text FROM perubahan_master`, "superadmin"},
			{"pengguna.kabupaten_kode tetap", `SELECT id::text FROM pengguna WHERE kabupaten_kode = '1174'`, "mahasiswa_9"},
		}
		for _, k := range kasus {
			assert.Equal(t, uuidPengguna(k.slug), scanString(t, db, k.sql), k.nama)
		}
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM login_attempts WHERE user_id IS NULL`), "percobaan anonim tetap NULL")
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM eksekusi_sumber WHERE aktor_id IS NULL`), "eksekusi terjadwal tetap NULL")
	})

	t.Run("rujukan lunak audit_log entitas pengguna dipetakan, sisanya tidak", func(t *testing.T) {
		assert.Equal(t, uuidPengguna("mahasiswa_9"), scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'A1'`))
		assert.Equal(t, "ekspor", scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'A2'`), "bukan ID pengguna")
		assert.Equal(t, "mahasiswa_9", scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'A3'`), "entitas lain tidak disentuh")
		assert.Equal(t, "sudah-dihapus", scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'A4'`), "akun yang tidak ada tidak dikarang")
		assert.Equal(t, uuidPengguna("test-user-001"), scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'A5'`))
	})

	t.Run("ews_notifikasi.tujuan: PUSH berisi id pengguna dan ikut dipetakan, EMAIL tidak disentuh", func(t *testing.T) {
		assert.Equal(t, uuidPengguna("mahasiswa_9"), scanString(t, db, `SELECT tujuan FROM ews_notifikasi WHERE kanal = 'PUSH'`))
		assert.Equal(t, "mahasiswa9@x.id", scanString(t, db, `SELECT tujuan FROM ews_notifikasi WHERE kanal = 'EMAIL'`))
	})

	t.Run("audit_log tetap append-only", func(t *testing.T) {
		assert.Equal(t, "O", scanString(t, db, `SELECT tgenabled::text FROM pg_trigger WHERE tgname = 'trg_audit_log_append_only'`))
		err := db.Exec(`UPDATE audit_log SET aksi = 'DIUBAH' WHERE id = 'A1'`).Error
		require.Error(t, err)
		assert.Contains(t, err.Error(), "append-only")
	})

	t.Run("semua FK ke pengguna bertipe uuid dengan ON DELETE eksplisit", func(t *testing.T) {
		fks := bacaKolomFKPengguna(t, db)
		assert.GreaterOrEqual(t, len(fks), 42)
		for _, fk := range fks {
			assert.Equal(t, "uuid", fk.Tipe, "%s.%s", fk.Tabel, fk.Kolom)
			assert.NotEqual(t, "NO ACTION", fk.SaatHapus, "%s.%s", fk.Tabel, fk.Kolom)
		}
	})

	t.Run("aturan hapus: bukti asal data menahan, milik akun ikut terhapus, jejak dilepas", func(t *testing.T) {
		// mahasiswa_9 adalah penginput entri_harga: RESTRICT.
		err := db.Exec(`DELETE FROM pengguna WHERE id = ?`, uuidPengguna("mahasiswa_9")).Error
		require.Error(t, err)
		assert.Contains(t, err.Error(), "23503")

		// test-user-001 hanya punya kode MFA (CASCADE) dan jejak audit (SET NULL, lewat pemicu yang mengizinkannya).
		require.NoError(t, db.Exec(`DELETE FROM pengguna WHERE id = ?`, uuidPengguna("test-user-001")).Error)
		assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM mfa_kode_cadangan`))
		assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM pengguna_id_lama WHERE id_lama = 'test-user-001'`))
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM audit_log WHERE id = 'A5' AND aktor_id IS NULL`))

		// user_nonaktif hanya jejak eskalasi alert (SET NULL).
		require.NoError(t, db.Exec(`DELETE FROM pengguna WHERE id = ?`, uuidPengguna("user_nonaktif")).Error)
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM ews_alert WHERE dieskalasi_oleh_id IS NULL`))
	})

	t.Run("akun baru memakai gen_random_uuid dan ID-nya dikembalikan sebagai teks saat down", func(t *testing.T) {
		exec(t, db, `INSERT INTO pengguna (nama, email, peran, lapis, lembaga) VALUES ('Baru', 'baru@x.id', 'OPS_MAHASISWA', 'OPERASIONAL', 'X')`)
		baru := scanString(t, db, `SELECT id::text FROM pengguna WHERE email = 'baru@x.id'`)
		assert.True(t, idx.ValidUUID(baru), baru)
		exec(t, db, `INSERT INTO login_attempts (user_id, success) VALUES (?, true)`, baru)

		require.NoError(t, migrator.DownTo(t.Context(), versiSebelumPenggunaUUID), "down 000110")

		assert.Equal(t, "character varying(255)", scanString(t, db,
			`SELECT format_type(atttypid, atttypmod) FROM pg_attribute WHERE attrelid = 'pengguna'::regclass AND attname = 'id'`))
		assert.Equal(t, baru, scanString(t, db, `SELECT id FROM pengguna WHERE email = 'baru@x.id'`), "akun baru: id::text")
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM login_attempts WHERE user_id = ?`, baru))
	})

	t.Run("down memulihkan ID lama, aturan semula, dan membuang pemetaan", func(t *testing.T) {
		// Dihapus sesudah Up: tidak kembali.
		for _, slug := range []string{"superadmin", "mahasiswa_9", "USR-2024-0001"} {
			assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM pengguna WHERE id = ?`, slug), slug)
		}
		assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM pengguna WHERE id IN ('test-user-001', 'user_nonaktif')`))

		assert.Equal(t, "mahasiswa_9", scanString(t, db, `SELECT user_id FROM refresh_tokens`))
		assert.Equal(t, "superadmin", scanString(t, db, `SELECT user_id FROM login_attempts WHERE user_id = 'superadmin'`))
		assert.Equal(t, "mahasiswa_9", scanString(t, db, `SELECT petugas_id FROM entri_harga`))
		assert.Equal(t, "superadmin", scanString(t, db, `SELECT divalidasi_oleh FROM entri_harga`))
		assert.Equal(t, "mahasiswa_9", scanString(t, db, `SELECT pengguna_id FROM intervensi_pj`))
		assert.Equal(t, "superadmin", scanString(t, db, `SELECT oleh FROM model_promosi`))
		assert.Equal(t, "superadmin", scanString(t, db, `SELECT diperbarui_oleh FROM roles`))
		assert.Equal(t, "mahasiswa_9", scanString(t, db, `SELECT pengguna_id FROM user_permissions`))
		assert.Equal(t, "superadmin", scanString(t, db, `SELECT aktor_id FROM audit_log WHERE id = 'A1'`))

		assert.Equal(t, "mahasiswa_9", scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'A1'`), "rujukan lunak kembali")
		assert.Equal(t, "mahasiswa_9", scanString(t, db, `SELECT tujuan FROM ews_notifikasi WHERE kanal = 'PUSH'`))
		assert.Equal(t, "mahasiswa9@x.id", scanString(t, db, `SELECT tujuan FROM ews_notifikasi WHERE kanal = 'EMAIL'`))
		assert.Equal(t, "ekspor", scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'A2'`))

		for _, fk := range bacaKolomFKPengguna(t, db) {
			assert.Equal(t, "character varying(255)", fk.Tipe, "%s.%s", fk.Tabel, fk.Kolom)
		}
		assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM pg_class WHERE relname = 'pengguna_id_lama'`), "tabel pemetaan dibuang")
		assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM pg_proc WHERE proname = 'garda_id_lama_pengguna'`), "fungsi bantu dibuang")
		assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM pg_class WHERE relname = 'idx_entri_harga_divalidasi_oleh'`), "indeks baru dibuang")
		assert.Equal(t, "NO ACTION", scanString(t, db, `
			SELECT CASE confdeltype WHEN 'a' THEN 'NO ACTION' ELSE confdeltype::text END FROM pg_constraint
			 WHERE conname = 'entri_harga_petugas_id_fkey'`))
		assert.Equal(t, "SET NULL", scanString(t, db, `
			SELECT CASE confdeltype WHEN 'n' THEN 'SET NULL' ELSE confdeltype::text END FROM pg_constraint
			 WHERE conname = 'ews_riwayat_oleh_fkey'`))
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM pg_constraint WHERE conname = 'chk_entri_harga_pemisahan_tugas'`))
		assert.Equal(t, "O", scanString(t, db, `SELECT tgenabled::text FROM pg_trigger WHERE tgname = 'trg_audit_log_append_only'`))
	})

	t.Run("up ulang atas data yang sudah dibalik menghasilkan UUID yang sama", func(t *testing.T) {
		require.NoError(t, migrator.UpTo(t.Context(), versiPenggunaUUID), "up kedua")
		for _, slug := range []string{"superadmin", "mahasiswa_9", "USR-2024-0001"} {
			assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM pengguna WHERE id = ?::uuid`, uuidPengguna(slug)), slug)
		}
		assert.Equal(t, uuidPengguna("mahasiswa_9"), scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'A1'`))
		assert.Equal(t, uuidPengguna("mahasiswa_9"), scanString(t, db, `SELECT petugas_id::text FROM entri_harga`))
	})
}

// TestMigrasiPenggunaUUID_PemisahanTugasTetapDitegakkan: CHECK pemisahan tugas dibuat ulang sesudah kolomnya
// menjadi uuid, NOT VALID seperti aslinya.
func TestMigrasiPenggunaUUID_PemisahanTugasTetapDitegakkan(t *testing.T) {
	db := pg.NewDatabase(t, "migrate_pengguna_uuid_check")
	migrator := pgtest.Migrator(t, db)
	require.NoError(t, migrator.UpTo(t.Context(), versiPenggunaUUID))

	exec(t, db, `INSERT INTO provinsi (kode, nama) VALUES ('11', 'Aceh')`)
	exec(t, db, `INSERT INTO kabupaten (kode, nama, jenis, ibukota, lat, lng, provinsi_kode) VALUES ('1174', 'L', 'KOTA', 'L', 5, 97, '11')`)
	exec(t, db, `INSERT INTO pasar (id, nama, kabupaten_kode, lat, lng) VALUES ('PSR-1', 'P', '1174', 5, 97)`)
	exec(t, db, `INSERT INTO komoditas (kode, nama, satuan, kategori, harga_acuan, shelf_life_hari) VALUES ('K', 'K', 'kg', 'x', 1, 1)`)
	exec(t, db, `INSERT INTO pengguna (nama, email, peran, lapis, lembaga) VALUES ('A', 'a@x.id', 'OPS_MAHASISWA', 'OPERASIONAL', 'X')`)

	assert.False(t, scanCount(t, db, `SELECT count(*) FROM pg_constraint WHERE conname = 'chk_entri_harga_pemisahan_tugas' AND convalidated`) > 0, "NOT VALID")
	err := db.Exec(`INSERT INTO entri_harga (id, komoditas_kode, pasar_id, tipe, harga, satuan, dicatat_pada, petugas_id, divalidasi_oleh)
		SELECT 'EH-X', 'K', 'PSR-1', 'KONSUMEN', 1, 'kg', now(), id, id FROM pengguna`).Error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chk_entri_harga_pemisahan_tugas")
}
