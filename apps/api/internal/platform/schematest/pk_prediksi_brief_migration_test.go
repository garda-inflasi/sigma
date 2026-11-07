package schematest

import (
	"strings"
	"testing"

	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Migrasi 000114 mengubah kunci primer prediksi dan policy brief. Tes dipatok ke versi 113 dan 114 (bukan
// "terbaru") supaya migrasi konversi berikutnya tidak mengubah bentuk fixture ID lama di bawah.
const (
	versiSebelumPrediksiBrief = 113
	versiPrediksiBrief        = 114
)

// ID lama (varchar buatan aplikasi) yang dipakai fixture, dan UUID/ID baru yang diharapkan.
const (
	g4Model1 = "MDL-BWM-v2.3.1"                              // dilatih 2026-09-15 -> id 1
	g4Model2 = "MDL-CBM-KRT-v2.3.1"                          // dilatih 2026-10-01 -> id 2
	g4Model3 = "MDL-CBM-KRT-v2.4.0-rc1"                      // dilatih 2026-10-20 -> id 3
	g4Model4 = "MDL-BWM-ml-20261105-140312"                  // dilatih 2026-11-05 -> id 4
	g4Pred1  = "PRD-CBM-KRT-1171-7"                          // dibuat 2026-11-05 00:00 -> id 2 (kalah dari '...-14' pada seri waktu)
	g4Pred2  = "PRD-CBM-KRT-1171-14"                         // dibuat 2026-11-05 00:00 -> id 1
	g4Pred3  = "PRM-BWM-1174-14-ml-20261105-140312-20261119" // dibuat 2026-11-05 14:03 -> id 3
	g4Prm1   = "b2f1c4d0-0000-4000-8000-000000000001"        // waktu 2026-10-01 -> id 1
	g4Prm2   = "b2f1c4d0-0000-4000-8000-000000000002"        // waktu 2026-10-25 -> id 2
	g4SimA   = "5f0c7a52-0a57-4b9a-8f0e-0d1f6f3c9a01"        // dibuat 2026-11-02 -> id 1
	g4SimB   = "5f0c7a52-0a57-4b9a-8f0e-0d1f6f3c9a02"        // dibuat 2026-11-03 -> id 2
	g4JobA   = "9a1b2c3d-0000-4000-8000-00000000000a"        // mulai 2026-09-20 -> id 1
	g4JobB   = "9a1b2c3d-0000-4000-8000-00000000000b"        // mulai 2026-10-20 -> id 2
)

func g4UUIDBrief(kode string) string { return idx.UUIDStabil("policy_brief", kode) }

// g4IsiDataLama mengisi basis data versi 113 dengan baris berbentuk data lama pada tabel milik 000114 beserta
// tabel referensi dan pengguna yang dirujuknya. Baris sengaja disisipkan tidak berurutan waktu.
func g4IsiDataLama(t *testing.T, db *gorm.DB) {
	t.Helper()
	sql := []string{
		`INSERT INTO provinsi (kode, nama) VALUES ('11', 'Aceh')`,
		`INSERT INTO kabupaten (kode, nama, jenis, ibukota, lat, lng, provinsi_kode) VALUES
			('1171', 'Banda Aceh', 'KOTA', 'Banda Aceh', 5.55, 95.32, '11'),
			('1174', 'Lhokseumawe', 'KOTA', 'Lhokseumawe', 5.18, 97.15, '11')`,
		`INSERT INTO komoditas (kode, nama, satuan, kategori, harga_acuan, shelf_life_hari) VALUES
			('CBM-KRT', 'Cabai Merah', 'kg', 'cabai', 50000, 7), ('BWM', 'Bawang Merah', 'kg', 'bawang', 40000, 30)`,
		`INSERT INTO pengguna (id, nama, email, peran, lapis, lembaga) VALUES
			('` + uuidPengguna("superadmin") + `', 'Super Admin', 'superadmin@x.id', 'ADMIN_SISTEM', 'SISTEM', 'BPPSDMP'),
			('` + uuidPengguna("manajer_kebijakan") + `', 'Manajer Kebijakan', 'mk@x.id', 'MANAJER_KEBIJAKAN', 'MANAJERIAL', 'BPPSDMP')`,

		// model_registry: urutan sisip berbeda dari urutan dilatih_pada.
		`INSERT INTO model_registry (id, komoditas_kode, nama, versi, mape, status, dilatih_pada, sumber) VALUES
			('` + g4Model3 + `', 'CBM-KRT', 'Model', 'v2.4.0-rc1', 2.0, 'KANDIDAT', '2026-10-20 00:00+00', 'DEMO'),
			('` + g4Model4 + `', 'BWM', 'Model', 'ml-20261105-140312', 3.0, 'KANDIDAT', '2026-11-05 14:03:12+00', 'MODEL')`,
		`INSERT INTO model_registry (id, komoditas_kode, nama, versi, mape, status, dilatih_pada, sumber, dipromosikan_oleh, dipromosikan_pada) VALUES
			('` + g4Model2 + `', 'CBM-KRT', 'Model', 'v2.3.1', 2.2, 'PRODUKSI', '2026-10-01 00:00+00', 'DEMO', '` + uuidPengguna("superadmin") + `', '2026-10-01 01:00+00'),
			('` + g4Model1 + `', 'BWM', 'Model', 'v2.3.1', 2.6, 'PRODUKSI', '2026-09-15 00:00+00', 'DEMO', NULL, NULL)`,

		// prediksi_harga: P1 dan P2 punya dibuat_pada sama, jadi ID lama jadi pemutus seri.
		`INSERT INTO prediksi_harga (id, komoditas_kode, kabupaten_kode, tanggal_target, horizon_hari, nilai, bawah, atas, harga_dasar, model_versi, faktor, sumber, dibuat_pada) VALUES
			('` + g4Pred1 + `', 'CBM-KRT', '1171', '2026-11-12', 7, 52000, 51000, 53000, 50000, 'v2.3.1', '[]', 'DEMO', '2026-11-05 00:00+00'),
			('` + g4Pred3 + `', 'BWM', '1174', '2026-11-19', 14, 41000, 40000, 42000, 40000, 'ml-20261105-140312', '[]', 'MODEL', '2026-11-05 14:03:12+00'),
			('` + g4Pred2 + `', 'CBM-KRT', '1171', '2026-11-19', 14, 53000, 51500, 54500, 50000, 'v2.3.1', '[]', 'DEMO', '2026-11-05 00:00+00')`,
		`INSERT INTO prediksi_evaluasi (prediksi_id, realisasi, galat_pct, dievaluasi_pada) VALUES
			('` + g4Pred1 + `', 51000, 1.96, '2026-11-13 00:00+00'), ('` + g4Pred3 + `', 40500, 1.23, '2026-11-10 00:00+00')`,

		`INSERT INTO model_promosi (id, model_id, komoditas_kode, aksi, status_lama, status_baru, model_digantikan_id, oleh, waktu) VALUES
			('` + g4Prm2 + `', '` + g4Model3 + `', 'CBM-KRT', 'PROMOSI', 'KANDIDAT', 'PRODUKSI', '` + g4Model2 + `', '` + uuidPengguna("superadmin") + `', '2026-10-25 00:00+00'),
			('` + g4Prm1 + `', '` + g4Model2 + `', 'CBM-KRT', 'PROMOSI', 'KANDIDAT', 'PRODUKSI', NULL, '` + uuidPengguna("superadmin") + `', '2026-10-01 00:00+00')`,

		`INSERT INTO skenario_simulasi (id, komoditas_kode, kabupaten_kode, horizon_hari, parameter, hasil, dibuat_oleh, dibuat_pada) VALUES
			('` + g4SimB + `', 'CBM-KRT', '1171', 14, '{}'::jsonb, '{"id": "` + g4SimB + `"}'::jsonb, '` + uuidPengguna("superadmin") + `', '2026-11-03 00:00+00'),
			('` + g4SimA + `', 'BWM', '1174', 7, '{}'::jsonb, '{}'::jsonb, '` + uuidPengguna("superadmin") + `', '2026-11-02 00:00+00')`,

		`INSERT INTO policy_brief (id, periode, judul, ringkasan, rekomendasi, halaman, disusun_oleh_id, status) VALUES
			('PB-2026-10', 'Oktober 2026', 'Brief Oktober', 'ringkas', '[]'::jsonb, 1, '` + uuidPengguna("manajer_kebijakan") + `', 'DIPROSES'),
			('PB-2026-09', 'September 2026', 'Brief September', 'ringkas', '[]'::jsonb, 1, '` + uuidPengguna("manajer_kebijakan") + `', 'DRAF'),
			('PB-2026-08', 'Agustus 2026', 'Brief Agustus', 'ringkas', '[]'::jsonb, 1, '` + uuidPengguna("manajer_kebijakan") + `', 'DISETUJUI')`,
		`INSERT INTO policy_brief_versi (brief_id, versi, konten, oleh, waktu) VALUES
			('PB-2026-10', 1, '{}'::jsonb, '` + uuidPengguna("manajer_kebijakan") + `', '2026-10-20 00:00+00'),
			('PB-2026-09', 2, '{"v": 2}'::jsonb, '` + uuidPengguna("manajer_kebijakan") + `', '2026-09-25 00:00+00'),
			('PB-2026-09', 1, '{"v": 1}'::jsonb, '` + uuidPengguna("manajer_kebijakan") + `', '2026-09-20 00:00+00')`,
		`INSERT INTO brief_job (id, brief_id, status, mulai) VALUES
			('` + g4JobB + `', 'PB-2026-10', 'BERJALAN', '2026-10-20 00:00+00'),
			('` + g4JobA + `', 'PB-2026-09', 'SELESAI', '2026-09-20 00:00+00')`,

		// Rujukan lunak (audit_log append-only; fixture ditulis sebelum migrasi).
		`INSERT INTO audit_log (id, aktor_id, aksi, entitas, entitas_id, hasil) VALUES
			('G4-A1', '` + uuidPengguna("superadmin") + `', 'PROMOSI_MODEL', 'model_registry', '` + g4Model2 + `', 'BERHASIL'),
			('G4-A2', '` + uuidPengguna("superadmin") + `', 'GENERATE_BRIEF', 'policy_brief', 'PB-2026-10', 'BERHASIL'),
			('G4-A3', '` + uuidPengguna("superadmin") + `', 'SIMULASI_SKENARIO', 'skenario_simulasi', '` + g4SimA + `', 'BERHASIL'),
			('G4-A4', '` + uuidPengguna("superadmin") + `', 'EVALUASI_PREDIKSI', 'prediksi_evaluasi', '-', 'BERHASIL'),
			('G4-A5', '` + uuidPengguna("superadmin") + `', 'EKSPOR_DATA', 'proyeksi_harga', 'CBM-KRT', 'BERHASIL'),
			('G4-A6', '` + uuidPengguna("superadmin") + `', 'PROMOSI_MODEL', 'model_registry', 'model-sudah-dihapus', 'BERHASIL'),
			('G4-A7', '` + uuidPengguna("superadmin") + `', 'LAIN', 'entri_harga', '` + g4Model2 + `', 'BERHASIL')`,
	}
	for _, s := range sql {
		exec(t, db, s)
	}
}

func g4TipeKolom(t *testing.T, db *gorm.DB, tabel, kolom string) string {
	t.Helper()
	return scanString(t, db, `SELECT format_type(atttypid, atttypmod) FROM pg_attribute
		WHERE attrelid = to_regclass(?) AND attname = ? AND NOT attisdropped`, tabel, kolom)
}

func g4Identitas(t *testing.T, db *gorm.DB, tabel string) string {
	t.Helper()
	return scanString(t, db, `SELECT attidentity::text FROM pg_attribute WHERE attrelid = to_regclass(?) AND attname = 'id'`, tabel)
}

func g4AturanHapus(t *testing.T, db *gorm.DB, tabel, konstrain string) string {
	t.Helper()
	return scanString(t, db, `SELECT CASE confdeltype WHEN 'a' THEN 'NO ACTION' WHEN 'r' THEN 'RESTRICT' WHEN 'c' THEN 'CASCADE'
			WHEN 'n' THEN 'SET NULL' ELSE 'SET DEFAULT' END
		  FROM pg_constraint WHERE conrelid = to_regclass(?) AND conname = ?`, tabel, konstrain)
}

func g4AdaIndeks(t *testing.T, db *gorm.DB, nama string) bool {
	t.Helper()
	return scanCount(t, db, `SELECT count(*) FROM pg_indexes WHERE schemaname = 'public' AND indexname = ?`, nama) > 0
}

func g4AdaKonstrain(t *testing.T, db *gorm.DB, nama string) bool {
	t.Helper()
	return scanCount(t, db, `SELECT count(*) FROM pg_constraint WHERE conname = ?`, nama) > 0
}

// g4Coba menjalankan fn di dalam transaksi yang selalu dibatalkan, supaya uji aturan hapus tidak mengubah fixture.
func g4Coba(t *testing.T, db *gorm.DB, fn func(tx *gorm.DB)) {
	t.Helper()
	tx := db.Begin()
	require.NoError(t, tx.Error)
	defer tx.Rollback()
	fn(tx)
}

// TestMigrasiPrediksiBrief_DataLamaIkutDanBisaDibalik: migrasi 000114 memberi ID baru berurutan waktu pada tabel
// BIGINT, UUID stabil pada policy_brief, memindahkan semua FK anak, memetakan rujukan lunak audit_log, memasang
// aturan ON DELETE dan indeks baru; Down memulihkan semuanya.
func TestMigrasiPrediksiBrief_DataLamaIkutDanBisaDibalik(t *testing.T) {
	db := pg.NewDatabase(t, "migrate_prediksi_brief")
	migrator := pgtest.Migrator(t, db)
	require.NoError(t, migrator.UpTo(t.Context(), versiSebelumPrediksiBrief))
	g4IsiDataLama(t, db)
	require.Equal(t, "character varying(255)", g4TipeKolom(t, db, "model_registry", "id"))
	require.Equal(t, "character varying(255)", g4TipeKolom(t, db, "policy_brief", "id"))

	require.NoError(t, migrator.UpTo(t.Context(), versiPrediksiBrief), "up 000114")

	t.Run("tipe kunci primer: bigint identity ALWAYS dan uuid acak untuk policy_brief", func(t *testing.T) {
		for _, tabel := range []string{"model_registry", "prediksi_harga", "prediksi_evaluasi", "model_promosi", "skenario_simulasi", "policy_brief_versi", "brief_job"} {
			assert.Equal(t, "bigint", g4TipeKolom(t, db, tabel, "id"), tabel)
			assert.Equal(t, "a", g4Identitas(t, db, tabel), tabel+": GENERATED ALWAYS")
			assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM pg_index i WHERE i.indrelid = to_regclass(?) AND i.indisprimary
				AND (SELECT attname FROM pg_attribute WHERE attrelid = i.indrelid AND attnum = i.indkey[0]) = 'id' AND i.indnkeyatts = 1`, tabel), tabel+": PK satu kolom id")
		}
		assert.Equal(t, "uuid", g4TipeKolom(t, db, "policy_brief", "id"))
		assert.Equal(t, "gen_random_uuid()", scanString(t, db, `SELECT pg_get_expr(adbin, adrelid) FROM pg_attrdef WHERE adrelid = 'policy_brief'::regclass
			AND adnum = (SELECT attnum FROM pg_attribute WHERE attrelid = 'policy_brief'::regclass AND attname = 'id')`))
	})

	t.Run("ID BIGINT berurutan menurut waktu dan ID lama pemutus seri, kode menyimpan ID lama", func(t *testing.T) {
		urut := func(sql string) string { return scanString(t, db, sql) }
		assert.Equal(t, strings.Join([]string{g4Model1, g4Model2, g4Model3, g4Model4}, ","),
			urut(`SELECT string_agg(kode, ',' ORDER BY id) FROM model_registry`), "model_registry menurut dilatih_pada")
		assert.Equal(t, "1,2,3,4", urut(`SELECT string_agg(id::text, ',' ORDER BY id) FROM model_registry`), "tanpa lubang")
		assert.Equal(t, strings.Join([]string{g4Pred2, g4Pred1, g4Pred3}, ","),
			urut(`SELECT string_agg(kode, ',' ORDER BY id) FROM prediksi_harga`), "prediksi_harga: dibuat_pada, lalu id lama")
		assert.Equal(t, strings.Join([]string{g4Prm1, g4Prm2}, ","), urut(`SELECT string_agg(kode, ',' ORDER BY id) FROM model_promosi`), "menurut waktu")
		assert.Equal(t, strings.Join([]string{g4SimA, g4SimB}, ","), urut(`SELECT string_agg(kode, ',' ORDER BY id) FROM skenario_simulasi`), "menurut dibuat_pada")
		assert.Equal(t, strings.Join([]string{g4JobA, g4JobB}, ","), urut(`SELECT string_agg(kode, ',' ORDER BY id) FROM brief_job`), "menurut mulai")
		assert.Equal(t, "PB-2026-09/1,PB-2026-09/2,PB-2026-10/1", urut(`SELECT string_agg(b.kode || '/' || v.versi, ',' ORDER BY v.id)
			FROM policy_brief_versi v JOIN policy_brief b ON b.id = v.brief_id`), "versi menurut waktu")
		assert.Equal(t, "3,2", urut(`SELECT string_agg(prediksi_id::text, ',' ORDER BY id) FROM prediksi_evaluasi`), "evaluasi menurut dievaluasi_pada: proyeksi id 3 dulu, lalu id 2")
	})

	t.Run("policy_brief: UUID stabil, kode PB-YYYY-MM unik dan wajib", func(t *testing.T) {
		for _, kode := range []string{"PB-2026-08", "PB-2026-09", "PB-2026-10"} {
			assert.Equal(t, g4UUIDBrief(kode), scanString(t, db, `SELECT id::text FROM policy_brief WHERE kode = ?`, kode), kode)
		}
		assert.EqualValues(t, 3, scanCount(t, db, `SELECT count(*) FROM policy_brief`))
		assert.Equal(t, "YES", scanString(t, db, `SELECT CASE WHEN attnotnull THEN 'YES' ELSE 'NO' END FROM pg_attribute WHERE attrelid = 'policy_brief'::regclass AND attname = 'kode'`))
		assert.True(t, g4AdaKonstrain(t, db, "uq_policy_brief_kode"))
		assert.True(t, g4AdaKonstrain(t, db, "chk_policy_brief_kode_format"))
	})

	t.Run("FK anak menunjuk ID baru yang benar", func(t *testing.T) {
		kasus := []struct{ nama, sql, want string }{
			{"prediksi_harga.model_id (komoditas, versi)", `SELECT string_agg(m.kode, ',' ORDER BY p.id) FROM prediksi_harga p JOIN model_registry m ON m.id = p.model_id`, strings.Join([]string{g4Model2, g4Model2, g4Model4}, ",")},
			{"prediksi_harga.model_versi tetap", `SELECT string_agg(model_versi, ',' ORDER BY id) FROM prediksi_harga`, "v2.3.1,v2.3.1,ml-20261105-140312"},
			{"prediksi_evaluasi.prediksi_id", `SELECT string_agg(p.kode, ',' ORDER BY e.id) FROM prediksi_evaluasi e JOIN prediksi_harga p ON p.id = e.prediksi_id`, g4Pred3 + "," + g4Pred1},
			{"model_promosi.model_id", `SELECT string_agg(m.kode, ',' ORDER BY t.id) FROM model_promosi t JOIN model_registry m ON m.id = t.model_id`, g4Model2 + "," + g4Model3},
			{"model_promosi.model_digantikan_id", `SELECT string_agg(COALESCE(m.kode, '-'), ',' ORDER BY t.id) FROM model_promosi t LEFT JOIN model_registry m ON m.id = t.model_digantikan_id`, "-," + g4Model2},
			{"brief_job.brief_id", `SELECT string_agg(b.kode, ',' ORDER BY j.id) FROM brief_job j JOIN policy_brief b ON b.id = j.brief_id`, "PB-2026-09,PB-2026-10"},
			{"policy_brief_versi.brief_id", `SELECT string_agg(DISTINCT b.kode, ',') FROM policy_brief_versi v JOIN policy_brief b ON b.id = v.brief_id`, "PB-2026-09,PB-2026-10"},
		}
		for _, k := range kasus {
			assert.Equal(t, k.want, scanString(t, db, k.sql), k.nama)
		}
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM model_promosi WHERE model_digantikan_id IS NULL`), "NULL tetap NULL")
		assert.Equal(t, "bigint", g4TipeKolom(t, db, "prediksi_harga", "model_id"))
		assert.Equal(t, "bigint", g4TipeKolom(t, db, "model_promosi", "model_id"))
		assert.Equal(t, "bigint", g4TipeKolom(t, db, "model_promosi", "model_digantikan_id"))
		assert.Equal(t, "bigint", g4TipeKolom(t, db, "prediksi_evaluasi", "prediksi_id"))
		assert.Equal(t, "uuid", g4TipeKolom(t, db, "brief_job", "brief_id"))
		assert.Equal(t, "uuid", g4TipeKolom(t, db, "policy_brief_versi", "brief_id"))
		assert.Equal(t, "YES", scanString(t, db, `SELECT CASE WHEN attnotnull THEN 'YES' ELSE 'NO' END FROM pg_attribute WHERE attrelid = 'prediksi_harga'::regclass AND attname = 'model_id'`), "model_id NOT NULL seperti model_versi")
	})

	t.Run("rujukan lunak audit_log dipetakan hanya untuk ID lama yang ada", func(t *testing.T) {
		assert.Equal(t, "2", scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'G4-A1'`), "model_registry -> id::text")
		assert.Equal(t, g4UUIDBrief("PB-2026-10"), scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'G4-A2'`), "policy_brief -> uuid")
		assert.Equal(t, "1", scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'G4-A3'`), "skenario_simulasi -> id::text")
		assert.Equal(t, "-", scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'G4-A4'`), "bukan ID")
		assert.Equal(t, "CBM-KRT", scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'G4-A5'`), "kode komoditas bukan ID")
		assert.Equal(t, "model-sudah-dihapus", scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'G4-A6'`), "ID yang tidak ada tidak dikarang")
		assert.Equal(t, g4Model2, scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'G4-A7'`), "entitas lain tidak disentuh")
		assert.Equal(t, "O", scanString(t, db, `SELECT tgenabled::text FROM pg_trigger WHERE tgname = 'trg_audit_log_append_only'`))
		err := db.Exec(`UPDATE audit_log SET aksi = 'DIUBAH' WHERE id = 'G4-A1'`).Error
		require.Error(t, err)
		assert.Contains(t, err.Error(), "append-only")
	})

	t.Run("ON DELETE eksplisit dan FK bertipe sama dengan induknya", func(t *testing.T) {
		kasus := []struct{ tabel, konstrain, want string }{
			{"prediksi_harga", "prediksi_harga_model_id_fkey", "RESTRICT"},
			{"prediksi_harga", "prediksi_harga_kabupaten_kode_fkey", "RESTRICT"},
			{"prediksi_harga", "prediksi_harga_komoditas_kode_fkey", "RESTRICT"},
			{"prediksi_evaluasi", "prediksi_evaluasi_prediksi_id_fkey", "CASCADE"},
			{"model_registry", "model_registry_komoditas_kode_fkey", "RESTRICT"},
			{"model_promosi", "model_promosi_model_id_fkey", "RESTRICT"},
			{"model_promosi", "model_promosi_model_digantikan_id_fkey", "RESTRICT"},
			{"model_promosi", "model_promosi_komoditas_kode_fkey", "RESTRICT"},
			{"skenario_simulasi", "skenario_simulasi_komoditas_kode_fkey", "RESTRICT"},
			{"skenario_simulasi", "skenario_simulasi_kabupaten_kode_fkey", "RESTRICT"},
			{"brief_job", "brief_job_brief_id_fkey", "CASCADE"},
			{"policy_brief_versi", "policy_brief_versi_brief_id_fkey", "CASCADE"},
		}
		for _, k := range kasus {
			assert.Equal(t, k.want, g4AturanHapus(t, db, k.tabel, k.konstrain), k.konstrain)
		}
		assert.False(t, g4AdaKonstrain(t, db, "fk_prediksi_harga_model"), "FK komposit varchar dilepas")
		assert.EqualValues(t, 0, scanCount(t, db, `SELECT count(*) FROM pg_constraint WHERE contype = 'f' AND conrelid = 'prediksi_harga'::regclass AND cardinality(conkey) > 1`))
	})

	t.Run("indeks dan constraint baru ada, yang redundan dibuang", func(t *testing.T) {
		for _, nama := range []string{
			"idx_prediksi_harga_model_id", "idx_prediksi_harga_terbaru", "idx_prediksi_harga_kabupaten_horizon",
			"idx_model_promosi_model_digantikan_id", "idx_skenario_simulasi_komoditas_kode", "idx_skenario_simulasi_kabupaten_kode",
			"idx_brief_job_berjalan", "idx_brief_job_brief", "idx_model_promosi_model", "idx_model_promosi_komoditas",
		} {
			assert.True(t, g4AdaIndeks(t, db, nama), nama)
		}
		assert.False(t, g4AdaIndeks(t, db, "idx_prediksi_harga_lookup"), "menjadi awalan uq_prediksi_harga_proyeksi")
		for _, nama := range []string{
			"uq_model_registry_kode", "uq_prediksi_harga_kode", "uq_model_promosi_kode", "uq_skenario_simulasi_kode", "uq_brief_job_kode",
			"uq_prediksi_evaluasi_prediksi", "uq_policy_brief_versi_alami", "uq_prediksi_harga_proyeksi", "model_registry_komoditas_kode_versi_key",
		} {
			assert.True(t, g4AdaKonstrain(t, db, nama), nama)
		}
		assert.Contains(t, scanString(t, db, `SELECT indexdef FROM pg_indexes WHERE indexname = 'idx_brief_job_berjalan'`), "WHERE ((status)::text = 'BERJALAN'::text)")
		assert.True(t, g4AdaIndeks(t, db, "uq_model_registry_produksi"), "indeks parsial satu produksi per komoditas tetap")
	})

	t.Run("aturan hapus bekerja: RESTRICT menolak, CASCADE menghapus anak", func(t *testing.T) {
		g4Coba(t, db, func(tx *gorm.DB) {
			// model_registry yang dirujuk proyeksi atau riwayat promosi tidak bisa dihapus.
			err := tx.Exec(`DELETE FROM model_registry WHERE kode = ?`, g4Model2).Error
			require.Error(t, err)
			assert.Contains(t, err.Error(), "23503")
		})
		g4Coba(t, db, func(tx *gorm.DB) {
			err := tx.Exec(`DELETE FROM model_registry WHERE kode = ?`, g4Model4).Error
			require.Error(t, err, "dirujuk proyeksi MODEL")
			assert.Contains(t, err.Error(), "23503")
		})
		g4Coba(t, db, func(tx *gorm.DB) {
			// rc1 tidak dirujuk proyeksi tetapi dirujuk model_promosi.model_id.
			err := tx.Exec(`DELETE FROM model_registry WHERE kode = ?`, g4Model3).Error
			require.Error(t, err)
			assert.Contains(t, err.Error(), "23503")
		})
		g4Coba(t, db, func(tx *gorm.DB) {
			err := tx.Exec(`DELETE FROM komoditas WHERE kode = 'BWM'`).Error
			require.Error(t, err, "dirujuk model, proyeksi, skenario")
			assert.Contains(t, err.Error(), "23503")
		})
		g4Coba(t, db, func(tx *gorm.DB) {
			err := tx.Exec(`DELETE FROM kabupaten WHERE kode = '1174'`).Error
			require.Error(t, err, "dirujuk proyeksi dan skenario")
			assert.Contains(t, err.Error(), "23503")
		})
		g4Coba(t, db, func(tx *gorm.DB) {
			require.NoError(t, tx.Exec(`DELETE FROM prediksi_harga WHERE kode = ?`, g4Pred1).Error)
			assert.EqualValues(t, 1, scanCount(t, tx, `SELECT count(*) FROM prediksi_evaluasi`), "evaluasi proyeksi itu ikut terhapus")
		})
		g4Coba(t, db, func(tx *gorm.DB) {
			require.NoError(t, tx.Exec(`DELETE FROM policy_brief WHERE kode = 'PB-2026-09'`).Error)
			assert.EqualValues(t, 1, scanCount(t, tx, `SELECT count(*) FROM policy_brief_versi`), "versi brief itu ikut terhapus")
			assert.EqualValues(t, 1, scanCount(t, tx, `SELECT count(*) FROM brief_job`), "job brief itu ikut terhapus")
		})
		assert.EqualValues(t, 3, scanCount(t, db, `SELECT count(*) FROM prediksi_harga`), "transaksi uji dibatalkan")
	})

	t.Run("kunci unik dan CHECK: kode brief berbentuk PB-YYYY-MM", func(t *testing.T) {
		kolom := `(periode, judul, ringkasan, rekomendasi, halaman, disusun_oleh_id, kode)`
		nilai := `('X', 'X', 'x', '[]'::jsonb, 1, '` + uuidPengguna("superadmin") + `', `
		g4Coba(t, db, func(tx *gorm.DB) {
			err := tx.Exec(`INSERT INTO policy_brief ` + kolom + ` VALUES ` + nilai + `'PB-2026-10')`).Error
			require.Error(t, err, "satu brief per periode")
			assert.Contains(t, err.Error(), "23505")
		})
		for _, salah := range []string{"PB-2026-13", "PB-2026-00", "pb-2026-10", "PB-26-10", "BRIEF-1", "PB-2026-10 "} {
			g4Coba(t, db, func(tx *gorm.DB) {
				err := tx.Exec(`INSERT INTO policy_brief ` + kolom + ` VALUES ` + nilai + `'` + salah + `')`).Error
				require.Error(t, err, salah)
				assert.Contains(t, err.Error(), "chk_policy_brief_kode_format", salah)
			})
		}
		g4Coba(t, db, func(tx *gorm.DB) {
			err := tx.Exec(`INSERT INTO policy_brief (periode, judul, ringkasan, rekomendasi, halaman, disusun_oleh_id) VALUES ` + nilai[:len(nilai)-2] + `)`).Error
			require.Error(t, err, "kode wajib")
			assert.Contains(t, err.Error(), "23502")
		})
		g4Coba(t, db, func(tx *gorm.DB) {
			require.NoError(t, tx.Exec(`INSERT INTO policy_brief `+kolom+` VALUES `+nilai+`'PB-2026-11')`).Error)
			baru := scanString(t, tx, `SELECT id::text FROM policy_brief WHERE kode = 'PB-2026-11'`)
			assert.True(t, idx.ValidUUID(baru), "id baris baru dari gen_random_uuid: "+baru)
			assert.NotEqual(t, g4UUIDBrief("PB-2026-11"), baru)
		})
	})

	t.Run("identity berlanjut dari max+1, id eksplisit ditolak, baris baru boleh tanpa kode", func(t *testing.T) {
		exec(t, db, `INSERT INTO model_registry (komoditas_kode, nama, versi, mape, dilatih_pada) VALUES ('CBM-KRT', 'Baru', 'v9', 1, now())`)
		assert.Equal(t, "5", scanString(t, db, `SELECT id::text FROM model_registry WHERE versi = 'v9'`))
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM model_registry WHERE versi = 'v9' AND kode IS NULL`))
		exec(t, db, `INSERT INTO prediksi_harga (komoditas_kode, kabupaten_kode, tanggal_target, horizon_hari, nilai, bawah, atas, harga_dasar, model_versi, model_id, faktor)
			SELECT 'CBM-KRT', '1171', '2026-12-01', 7, 52000, 51000, 53000, 50000, 'v9', id, '[]' FROM model_registry WHERE versi = 'v9'`)
		assert.Equal(t, "4", scanString(t, db, `SELECT id::text FROM prediksi_harga WHERE tanggal_target = '2026-12-01'`))
		exec(t, db, `INSERT INTO prediksi_evaluasi (prediksi_id, realisasi, galat_pct) SELECT id, 51000, 1 FROM prediksi_harga WHERE tanggal_target = '2026-12-01'`)
		assert.Equal(t, "3", scanString(t, db, `SELECT max(id)::text FROM prediksi_evaluasi`))
		exec(t, db, `INSERT INTO model_promosi (model_id, komoditas_kode, aksi, status_lama, status_baru, oleh)
			SELECT id, 'CBM-KRT', 'ARSIP', 'KANDIDAT', 'ARSIP', '`+uuidPengguna("superadmin")+`' FROM model_registry WHERE versi = 'v9'`)
		assert.Equal(t, "3", scanString(t, db, `SELECT max(id)::text FROM model_promosi`))
		exec(t, db, `INSERT INTO skenario_simulasi (komoditas_kode, kabupaten_kode, horizon_hari, parameter, hasil, dibuat_oleh)
			VALUES ('CBM-KRT', '1171', 7, '{}'::jsonb, '{}'::jsonb, '`+uuidPengguna("superadmin")+`')`)
		assert.Equal(t, "3", scanString(t, db, `SELECT max(id)::text FROM skenario_simulasi`))
		exec(t, db, `INSERT INTO policy_brief_versi (brief_id, versi, konten, oleh) VALUES (?, 3, '{}'::jsonb, '`+uuidPengguna("superadmin")+`')`, g4UUIDBrief("PB-2026-09"))
		assert.Equal(t, "4", scanString(t, db, `SELECT max(id)::text FROM policy_brief_versi`))
		exec(t, db, `INSERT INTO brief_job (brief_id, status) VALUES (?, 'GAGAL')`, g4UUIDBrief("PB-2026-10"))
		assert.Equal(t, "3", scanString(t, db, `SELECT max(id)::text FROM brief_job`))
		exec(t, db, `INSERT INTO policy_brief (periode, judul, ringkasan, rekomendasi, halaman, disusun_oleh_id, kode)
			VALUES ('November 2026', 'Baru', 'x', '[]'::jsonb, 1, '`+uuidPengguna("superadmin")+`', 'PB-2026-11')`)

		for _, tabel := range []string{"model_registry", "prediksi_harga", "prediksi_evaluasi", "model_promosi", "skenario_simulasi", "policy_brief_versi", "brief_job"} {
			err := db.Exec(`INSERT INTO ` + tabel + ` (id) VALUES (999)`).Error
			require.Error(t, err, tabel)
			assert.Contains(t, err.Error(), "428C9", tabel+": id eksplisit ditolak (GENERATED ALWAYS) sebelum kolom lain diperiksa")
		}
		// Versi ganda untuk brief yang sama ditolak (UNIQUE pengganti PK komposit).
		err := db.Exec(`INSERT INTO policy_brief_versi (brief_id, versi, konten, oleh) VALUES (?, 1, '{}'::jsonb, '`+uuidPengguna("superadmin")+`')`, g4UUIDBrief("PB-2026-09")).Error
		require.Error(t, err)
		assert.Contains(t, err.Error(), "23505")
		// Evaluasi ganda untuk proyeksi yang sama ditolak (1:1).
		err = db.Exec(`INSERT INTO prediksi_evaluasi (prediksi_id, realisasi, galat_pct) SELECT id, 1, 1 FROM prediksi_harga WHERE tanggal_target = '2026-12-01'`).Error
		require.Error(t, err)
		assert.Contains(t, err.Error(), "23505")
	})

	require.NoError(t, migrator.DownTo(t.Context(), versiSebelumPrediksiBrief), "down 000114")

	t.Run("down memulihkan tipe, ID lama, FK komposit lama, dan aturan semula", func(t *testing.T) {
		for _, tabel := range []string{"model_registry", "prediksi_harga", "model_promosi", "skenario_simulasi", "policy_brief", "brief_job"} {
			assert.Equal(t, "character varying(255)", g4TipeKolom(t, db, tabel, "id"), tabel)
			assert.EqualValues(t, 0, scanCount(t, db, `SELECT count(*) FROM pg_attribute WHERE attrelid = to_regclass(?) AND attname = 'kode' AND NOT attisdropped`, tabel), tabel+": kolom kode dibuang")
		}
		assert.Equal(t, "character varying(255)", g4TipeKolom(t, db, "prediksi_evaluasi", "prediksi_id"))
		assert.Equal(t, "character varying(255)", g4TipeKolom(t, db, "policy_brief_versi", "brief_id"))
		assert.Equal(t, "character varying(255)", g4TipeKolom(t, db, "brief_job", "brief_id"))
		assert.Equal(t, "character varying(255)", g4TipeKolom(t, db, "model_promosi", "model_id"))
		assert.Equal(t, "character varying(255)", g4TipeKolom(t, db, "model_promosi", "model_digantikan_id"))

		for _, id := range []string{g4Model1, g4Model2, g4Model3, g4Model4} {
			assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM model_registry WHERE id = ?`, id), id)
		}
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM model_registry WHERE id = '5' AND versi = 'v9'`), "baris baru tanpa kode memakai id::text")
		for _, id := range []string{g4Pred1, g4Pred2, g4Pred3} {
			assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM prediksi_harga WHERE id = ?`, id), id)
		}
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM prediksi_harga WHERE id = '4' AND model_versi = 'v9'`))
		assert.Equal(t, "4,"+g4Pred1+","+g4Pred3, scanString(t, db, `SELECT string_agg(prediksi_id, ',' ORDER BY prediksi_id) FROM prediksi_evaluasi`))
		assert.Equal(t, g4Model2+","+g4Model3+",5", scanString(t, db, `SELECT string_agg(model_id, ',' ORDER BY waktu, id) FROM model_promosi WHERE aksi = 'PROMOSI' OR model_id = '5'`))
		assert.Equal(t, g4Model2, scanString(t, db, `SELECT model_digantikan_id FROM model_promosi WHERE id = ?`, g4Prm2))
		assert.EqualValues(t, 2, scanCount(t, db, `SELECT count(*) FROM skenario_simulasi WHERE id IN (?, ?)`, g4SimA, g4SimB), "kedua skenario lama kembali ber-ID lama")
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM brief_job WHERE id = ? AND brief_id = 'PB-2026-09'`, g4JobA))
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM brief_job WHERE id = ? AND brief_id = 'PB-2026-10'`, g4JobB))
		assert.EqualValues(t, 4, scanCount(t, db, `SELECT count(*) FROM policy_brief_versi`))
		assert.EqualValues(t, 3, scanCount(t, db, `SELECT count(*) FROM policy_brief_versi WHERE brief_id = 'PB-2026-09'`))
		assert.EqualValues(t, 4, scanCount(t, db, `SELECT count(*) FROM policy_brief`))
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM policy_brief WHERE id = 'PB-2026-11'`), "brief baru: id = kode")

		assert.Equal(t, "model-sudah-dihapus", scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'G4-A6'`))
		assert.Equal(t, g4Model2, scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'G4-A1'`), "rujukan lunak kembali ke ID lama")
		assert.Equal(t, "PB-2026-10", scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'G4-A2'`))
		assert.Equal(t, g4SimA, scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'G4-A3'`))
		assert.Equal(t, "O", scanString(t, db, `SELECT tgenabled::text FROM pg_trigger WHERE tgname = 'trg_audit_log_append_only'`))

		assert.True(t, g4AdaKonstrain(t, db, "fk_prediksi_harga_model"), "FK komposit lama pulih")
		assert.EqualValues(t, 0, scanCount(t, db, `SELECT count(*) FROM pg_attribute WHERE attrelid = 'prediksi_harga'::regclass AND attname = 'model_id' AND NOT attisdropped`))
		assert.True(t, g4AdaIndeks(t, db, "idx_prediksi_harga_lookup"))
		for _, nama := range []string{"idx_prediksi_harga_model_id", "idx_prediksi_harga_terbaru", "idx_prediksi_harga_kabupaten_horizon",
			"idx_model_promosi_model_digantikan_id", "idx_skenario_simulasi_komoditas_kode", "idx_skenario_simulasi_kabupaten_kode", "idx_brief_job_berjalan"} {
			assert.False(t, g4AdaIndeks(t, db, nama), nama+" dibuang")
		}
		for _, nama := range []string{"uq_model_registry_kode", "uq_prediksi_harga_kode", "uq_model_promosi_kode", "uq_skenario_simulasi_kode", "uq_brief_job_kode",
			"uq_prediksi_evaluasi_prediksi", "uq_policy_brief_versi_alami", "uq_policy_brief_kode", "chk_policy_brief_kode_format"} {
			assert.False(t, g4AdaKonstrain(t, db, nama), nama+" dibuang")
		}
		assert.EqualValues(t, 2, scanCount(t, db, `SELECT cardinality(conkey) FROM pg_constraint WHERE conname = 'policy_brief_versi_pkey'`), "PK komposit pulih")
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT cardinality(conkey) FROM pg_constraint WHERE conname = 'prediksi_evaluasi_pkey'`))
		assert.Equal(t, "prediksi_id", scanString(t, db, `SELECT a.attname FROM pg_constraint c JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = c.conkey[1] WHERE c.conname = 'prediksi_evaluasi_pkey'`))

		for _, k := range []struct{ tabel, konstrain string }{
			{"prediksi_harga", "prediksi_harga_kabupaten_kode_fkey"}, {"prediksi_harga", "prediksi_harga_komoditas_kode_fkey"},
			{"model_registry", "model_registry_komoditas_kode_fkey"}, {"model_promosi", "model_promosi_model_id_fkey"},
			{"model_promosi", "model_promosi_model_digantikan_id_fkey"}, {"model_promosi", "model_promosi_komoditas_kode_fkey"},
			{"skenario_simulasi", "skenario_simulasi_komoditas_kode_fkey"}, {"skenario_simulasi", "skenario_simulasi_kabupaten_kode_fkey"},
			{"brief_job", "brief_job_brief_id_fkey"}, {"policy_brief_versi", "policy_brief_versi_brief_id_fkey"},
		} {
			assert.Equal(t, "NO ACTION", g4AturanHapus(t, db, k.tabel, k.konstrain), k.konstrain)
		}
		assert.Equal(t, "CASCADE", g4AturanHapus(t, db, "prediksi_evaluasi", "prediksi_evaluasi_prediksi_id_fkey"))
		assert.EqualValues(t, 0, scanCount(t, db, `SELECT count(*) FROM pg_proc WHERE proname LIKE 'g4\_%'`), "fungsi bantu dibuang")
	})

	t.Run("up ulang atas data yang sudah dibalik berhasil dan UUID brief tetap sama", func(t *testing.T) {
		require.NoError(t, migrator.UpTo(t.Context(), versiPrediksiBrief), "up kedua")
		assert.Equal(t, g4UUIDBrief("PB-2026-10"), scanString(t, db, `SELECT id::text FROM policy_brief WHERE kode = 'PB-2026-10'`))
		assert.Equal(t, g4UUIDBrief("PB-2026-11"), scanString(t, db, `SELECT id::text FROM policy_brief WHERE kode = 'PB-2026-11'`), "brief yang dibuat sesudah Up pertama: id = id stabil dari kode")
		assert.EqualValues(t, 5, scanCount(t, db, `SELECT count(*) FROM model_registry`))
		assert.Equal(t, "bigint", g4TipeKolom(t, db, "model_registry", "id"))
		assert.Equal(t, g4Model1, scanString(t, db, `SELECT kode FROM model_registry WHERE id = 1`), "urutan dilatih_pada sama")
		assert.EqualValues(t, 4, scanCount(t, db, `SELECT count(*) FROM prediksi_harga p JOIN model_registry m ON m.id = p.model_id AND m.versi = p.model_versi AND m.komoditas_kode = p.komoditas_kode`), "model_id konsisten dengan (komoditas, versi)")
		assert.EqualValues(t, 3, scanCount(t, db, `SELECT count(*) FROM prediksi_evaluasi`))
		assert.Equal(t, g4UUIDBrief("PB-2026-10"), scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'G4-A2'`))
	})
}

// TestMigrasiPrediksiBrief_DataKosong: tabel kosong menghasilkan identity mulai dari 1 dan Up bersih tanpa data.
func TestMigrasiPrediksiBrief_DataKosong(t *testing.T) {
	db := pg.NewDatabase(t, "migrate_prediksi_brief_kosong")
	require.NoError(t, pgtest.Migrator(t, db).UpTo(t.Context(), versiPrediksiBrief))

	exec(t, db, `INSERT INTO provinsi (kode, nama) VALUES ('11', 'Aceh')`)
	exec(t, db, `INSERT INTO komoditas (kode, nama, satuan, kategori, harga_acuan, shelf_life_hari) VALUES ('CBM-KRT', 'Cabai', 'kg', 'cabai', 50000, 7)`)
	exec(t, db, `INSERT INTO model_registry (komoditas_kode, nama, versi, mape, dilatih_pada) VALUES ('CBM-KRT', 'M', 'v1', 1, now())`)
	assert.Equal(t, "1", scanString(t, db, `SELECT id::text FROM model_registry`))
}

// TestMigrasiPrediksiBrief_BriefBukanPBYYYYMMDitolak: brief ber-ID di luar bentuk PB-YYYY-MM menghentikan migrasi
// dengan pesan jelas, dan transaksi dibatalkan sehingga skema lama utuh.
func TestMigrasiPrediksiBrief_BriefBukanPBYYYYMMDitolak(t *testing.T) {
	db := pg.NewDatabase(t, "migrate_prediksi_brief_salah")
	migrator := pgtest.Migrator(t, db)
	require.NoError(t, migrator.UpTo(t.Context(), versiSebelumPrediksiBrief))
	exec(t, db, `INSERT INTO pengguna (id, nama, email, peran, lapis, lembaga) VALUES ('`+uuidPengguna("superadmin")+`', 'S', 's@x.id', 'ADMIN_SISTEM', 'SISTEM', 'X')`)
	exec(t, db, `INSERT INTO policy_brief (id, periode, judul, ringkasan, rekomendasi, halaman, disusun_oleh_id)
		VALUES ('BRIEF-LIAR', 'X', 'X', 'x', '[]'::jsonb, 1, '`+uuidPengguna("superadmin")+`')`)

	err := migrator.UpTo(t.Context(), versiPrediksiBrief)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "bukan PB-YYYY-MM")
	assert.Equal(t, "character varying(255)", g4TipeKolom(t, db, "policy_brief", "id"), "dibatalkan seluruhnya")
	assert.Equal(t, "character varying(255)", g4TipeKolom(t, db, "model_registry", "id"))
}

// TestMigrasiPrediksiBrief_IDLamaTerlaluPanjangDitolak: ID lama yang tidak muat di kode varchar(64) menghentikan migrasi.
func TestMigrasiPrediksiBrief_IDLamaTerlaluPanjangDitolak(t *testing.T) {
	db := pg.NewDatabase(t, "migrate_prediksi_brief_panjang")
	migrator := pgtest.Migrator(t, db)
	require.NoError(t, migrator.UpTo(t.Context(), versiSebelumPrediksiBrief))
	exec(t, db, `INSERT INTO komoditas (kode, nama, satuan, kategori, harga_acuan, shelf_life_hari) VALUES ('CBM-KRT', 'Cabai', 'kg', 'cabai', 50000, 7)`)
	exec(t, db, `INSERT INTO model_registry (id, komoditas_kode, nama, versi, mape, dilatih_pada)
		VALUES ('`+strings.Repeat("M", 70)+`', 'CBM-KRT', 'M', 'v1', 1, now())`)

	err := migrator.UpTo(t.Context(), versiPrediksiBrief)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "lebih dari 64 karakter")
	assert.Equal(t, "character varying(255)", g4TipeKolom(t, db, "model_registry", "id"))
}
