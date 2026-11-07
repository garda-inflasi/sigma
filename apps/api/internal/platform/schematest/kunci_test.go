package schematest

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Invarian kunci primer, kunci asing, dan indeks (docs/SKEMA-DATABASE.md). Tes ini sumber kebenaran matriks
// kunci: tabel baru wajib didaftarkan di kategoriPK, dan setiap pelanggaran menggagalkan tes kecuali tabelnya
// masih tercantum di tabelBelumSelaras (konversi bertahap). Pola daftarnya sama dengan pendingModules: tabel
// yang sudah selaras tetapi masih tercantum juga menggagalkan tes.

// KategoriPK adalah bentuk kunci primer yang ditetapkan untuk sebuah tabel.
type KategoriPK string

const (
	// PKUUID: satu kolom id uuid DEFAULT gen_random_uuid(). Entitas bisnis dan identitas yang terlihat klien.
	PKUUID KategoriPK = "UUID"
	// PKBigint: satu kolom id bigint identity atau serial. Transaksional, time-series, log, operasional.
	PKBigint KategoriPK = "BIGINT"
	// PKKode: satu kolom kode alami (varchar/enum/integer). Data referensi nasional dan konfigurasi berkode.
	PKKode KategoriPK = "KODE"
	// PKHash: satu kolom varchar berisi hash atau token id. Tabel keamanan tanpa entitas bisnis.
	PKHash KategoriPK = "HASH"
	// PKKomposit: gabungan kolom FK (tabel penghubung) atau induk + urutan (entitas lemah).
	PKKomposit KategoriPK = "KOMPOSIT"
)

// kategoriPK adalah matriks kunci primer seluruh tabel aplikasi.
var kategoriPK = map[string]KategoriPK{
	// UUID: entitas bisnis dan identitas.
	"pengguna":     PKUUID,
	"intervensi":   PKUUID,
	"policy_brief": PKUUID,
	"sumber_data":  PKUUID,
	"cold_storage": PKUUID,

	// BIGINT: transaksional, time-series, log, operasional.
	"entri_harga":            PKBigint,
	"riwayat_status_entri":   PKBigint,
	"tugas_lapangan":         PKBigint,
	"batas_harga":            PKBigint,
	"perubahan_master":       PKBigint,
	"audit_log":              PKBigint,
	"aturan_ews":             PKBigint,
	"aturan_ews_riwayat":     PKBigint,
	"ews_alert":              PKBigint,
	"ews_riwayat":            PKBigint,
	"ews_event":              PKBigint,
	"ews_notifikasi":         PKBigint,
	"langganan_push":         PKBigint,
	"intervensi_riwayat":     PKBigint,
	"intervensi_persetujuan": PKBigint,
	"stok_cold_storage":      PKBigint,
	"transaksi_stok":         PKBigint,
	"telemetri_suhu":         PKBigint,
	"kandidat_cold_storage":  PKBigint,
	"optimasi_rute":          PKBigint,
	"rute_distribusi":        PKBigint,
	"prediksi_harga":         PKBigint,
	"prediksi_evaluasi":      PKBigint,
	"model_registry":         PKBigint,
	"model_promosi":          PKBigint,
	"skenario_simulasi":      PKBigint,
	"policy_brief_versi":     PKBigint,
	"brief_job":              PKBigint,
	"eksekusi_sumber":        PKBigint,
	"data_mentah":            PKBigint,
	"harga_eksternal":        PKBigint,
	"cuaca_harian":           PKBigint,
	"andil_inflasi":          PKBigint,
	"inflasi_bulanan":        PKBigint,
	"neraca_pangan":          PKBigint,
	"neraca_wilayah":         PKBigint,
	"ruang_fiskal":           PKBigint,
	"siklus_ekonomi":         PKBigint,
	"kpi_nilai":              PKBigint,
	"kpi_target":             PKBigint,
	"target_kpi":             PKBigint,
	"refresh_tokens":         PKBigint,
	"login_attempts":         PKBigint,
	"password_reset_tokens":  PKBigint,
	"mfa_kode_cadangan":      PKBigint,
	"undangan_pengguna":      PKBigint,

	// KODE: referensi nasional (kode BPS), master berkode, dan konfigurasi berkode.
	"provinsi":         PKKode,
	"kabupaten":        PKKode,
	"komoditas":        PKKode,
	"pasar":            PKKode,
	"wilayah_geometri": PKKode, // 1:1 dengan kabupaten: PK sekaligus FK ke kabupaten.kode
	"permissions":      PKKode,
	"roles":            PKKode,
	"kpi_definisi":     PKKode,
	"alur_persetujuan": PKKode,
	"target_inflasi":   PKKode, // satu baris per tahun anggaran
	"pengguna_id_lama": PKKode, // 1:1 dengan pengguna (pemetaan ID lama untuk rollback 000110)

	// HASH: keamanan, kuncinya hash atau token id.
	"jwt_blacklist":   PKHash,
	"kunci_login":     PKHash,
	"sso_login_state": PKHash,

	// KOMPOSIT: tabel penghubung dan entitas lemah.
	"role_permissions":               PKKomposit,
	"user_permissions":               PKKomposit,
	"intervensi_pj":                  PKKomposit,
	"tugas_entri":                    PKKomposit,
	"preferensi_notifikasi":          PKKomposit,
	"pemetaan_sumber":                PKKomposit,
	"alur_persetujuan_langkah":       PKKomposit,
	"intervensi_persetujuan_langkah": PKKomposit,
	"tahap_intervensi_def":           PKKomposit, // definisi tahap per siklus (referensi berkode)
	"jarak_antar_wilayah":            PKKomposit, // matriks referensi asal x tujuan
}

// tabelBelumSelaras: tabel yang masih melanggar invarian sampai migrasi konversinya diterapkan. Hapus entri
// tabel begitu migrasinya selesai; nilai mencatat migrasi yang bertanggung jawab.
var tabelBelumSelaras = map[string]string{
	// 000111 harga, lapangan, master
	"entri_harga":      "000111",
	"tugas_lapangan":   "000111",
	"batas_harga":      "000111",
	"perubahan_master": "000111",
	"pasar":            "000111",
	"kabupaten":        "000111",
	// 000112 ews, intervensi
	"aturan_ews":             "000112",
	"aturan_ews_riwayat":     "000112",
	"ews_alert":              "000112",
	"ews_riwayat":            "000112",
	"ews_event":              "000112",
	"intervensi":             "000112",
	"intervensi_persetujuan": "000112",
	"alur_persetujuan":       "000112",
	// 000113 cold-storage, spasial
	"cold_storage":          "000113",
	"stok_cold_storage":     "000113",
	"transaksi_stok":        "000113",
	"kandidat_cold_storage": "000113",
	"optimasi_rute":         "000113",
	"rute_distribusi":       "000113",
	"neraca_wilayah":        "000113",
	"jarak_antar_wilayah":   "000113",
	"wilayah_geometri":      "000113",
	// 000114 prediksi, policy-brief
	"prediksi_harga":     "000114",
	"prediksi_evaluasi":  "000114",
	"model_registry":     "000114",
	"model_promosi":      "000114",
	"skenario_simulasi":  "000114",
	"policy_brief":       "000114",
	"policy_brief_versi": "000114",
	"brief_job":          "000114",
}

type kolomPK struct {
	Tabel     string `gorm:"column:tabel"`
	Kolom     string `gorm:"column:kolom"`
	Tipe      string `gorm:"column:tipe"`
	Identity  string `gorm:"column:identitas"`
	Bawaan    string `gorm:"column:bawaan"`
	JumlahKol int    `gorm:"column:jumlah_kolom"`
}

type kunciAsing struct {
	Nama       string `gorm:"column:nama"`
	Anak       string `gorm:"column:anak"`
	Kolom      string `gorm:"column:kolom"`
	TipeAnak   string `gorm:"column:tipe_anak"`
	Induk      string `gorm:"column:induk"`
	KolomInduk string `gorm:"column:kolom_induk"`
	TipeInduk  string `gorm:"column:tipe_induk"`
	SaatHapus  string `gorm:"column:saat_hapus"`
	Terindeks  bool   `gorm:"column:terindeks"`
}

func bacaPK(t *testing.T, db *gorm.DB) map[string]kolomPK {
	t.Helper()
	var rows []kolomPK
	require.NoError(t, db.Raw(`
		SELECT c.relname AS tabel,
		       string_agg(a.attname, ',' ORDER BY k.ord) AS kolom,
		       string_agg(format_type(a.atttypid, a.atttypmod), ',' ORDER BY k.ord) AS tipe,
		       max(a.attidentity::text) AS identitas,
		       coalesce(max(pg_get_expr(d.adbin, d.adrelid)), '') AS bawaan,
		       count(*) AS jumlah_kolom
		  FROM pg_class c
		  JOIN pg_namespace n ON n.oid = c.relnamespace AND n.nspname = 'public'
		  JOIN pg_index i ON i.indrelid = c.oid AND i.indisprimary
		  CROSS JOIN LATERAL unnest(i.indkey) WITH ORDINALITY k(attnum, ord)
		  JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum = k.attnum
		  LEFT JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
		 WHERE c.relkind IN ('r', 'p') AND NOT c.relispartition
		 GROUP BY c.relname`).Scan(&rows).Error)
	out := make(map[string]kolomPK, len(rows))
	for _, r := range rows {
		out[r.Tabel] = r
	}
	return out
}

func bacaTabel(t *testing.T, db *gorm.DB) []string {
	t.Helper()
	var tabel []string
	require.NoError(t, db.Raw(`
		SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		 WHERE n.nspname = 'public' AND c.relkind IN ('r', 'p') AND NOT c.relispartition
		   AND c.relname <> 'goose_db_version'
		 ORDER BY 1`).Scan(&tabel).Error)
	return tabel
}

func bacaFK(t *testing.T, db *gorm.DB) []kunciAsing {
	t.Helper()
	var rows []kunciAsing
	require.NoError(t, db.Raw(`
		SELECT con.conname AS nama,
		       con.conrelid::regclass::text AS anak,
		       (SELECT string_agg(a.attname, ',' ORDER BY k.ord) FROM unnest(con.conkey) WITH ORDINALITY k(n, ord)
		          JOIN pg_attribute a ON a.attrelid = con.conrelid AND a.attnum = k.n) AS kolom,
		       (SELECT string_agg(format_type(a.atttypid, a.atttypmod), ',' ORDER BY k.ord) FROM unnest(con.conkey) WITH ORDINALITY k(n, ord)
		          JOIN pg_attribute a ON a.attrelid = con.conrelid AND a.attnum = k.n) AS tipe_anak,
		       con.confrelid::regclass::text AS induk,
		       (SELECT string_agg(a.attname, ',' ORDER BY k.ord) FROM unnest(con.confkey) WITH ORDINALITY k(n, ord)
		          JOIN pg_attribute a ON a.attrelid = con.confrelid AND a.attnum = k.n) AS kolom_induk,
		       (SELECT string_agg(format_type(a.atttypid, a.atttypmod), ',' ORDER BY k.ord) FROM unnest(con.confkey) WITH ORDINALITY k(n, ord)
		          JOIN pg_attribute a ON a.attrelid = con.confrelid AND a.attnum = k.n) AS tipe_induk,
		       CASE con.confdeltype WHEN 'a' THEN 'NO ACTION' WHEN 'r' THEN 'RESTRICT' WHEN 'c' THEN 'CASCADE'
		                            WHEN 'n' THEN 'SET NULL' ELSE 'SET DEFAULT' END AS saat_hapus,
		       EXISTS (SELECT 1 FROM pg_index i
		                WHERE i.indrelid = con.conrelid
		                  AND (i.indkey::int2[])[0:cardinality(con.conkey) - 1] = con.conkey) AS terindeks
		  FROM pg_constraint con
		  JOIN pg_namespace n ON n.oid = con.connamespace AND n.nspname = 'public'
		 WHERE con.contype = 'f'
		 ORDER BY 2, 3`).Scan(&rows).Error)
	return rows
}

// pelanggaranPK memeriksa bentuk kunci primer terhadap kategorinya.
func pelanggaranPK(tabel string, kat KategoriPK, pk kolomPK, ada bool) []string {
	if !ada {
		return []string{tabel + ": tidak punya kunci primer"}
	}
	salah := func(format string, args ...any) []string {
		return []string{fmt.Sprintf("%s: PK %s (%s) bukan kategori %s: %s", tabel, pk.Kolom, pk.Tipe, kat, fmt.Sprintf(format, args...))}
	}
	switch kat {
	case PKUUID:
		if pk.JumlahKol != 1 || pk.Kolom != "id" || pk.Tipe != "uuid" {
			return salah("harus satu kolom id bertipe uuid")
		}
		if pk.Bawaan != "gen_random_uuid()" {
			return salah("bawaan harus gen_random_uuid(), bukan %q", pk.Bawaan)
		}
	case PKBigint:
		if pk.JumlahKol != 1 || pk.Kolom != "id" || pk.Tipe != "bigint" {
			return salah("harus satu kolom id bertipe bigint")
		}
		if pk.Identity == "" && !strings.HasPrefix(pk.Bawaan, "nextval(") {
			return salah("harus GENERATED AS IDENTITY atau BIGSERIAL")
		}
	case PKKode, PKHash:
		if pk.JumlahKol != 1 {
			return salah("harus satu kolom kode")
		}
	case PKKomposit:
		if pk.JumlahKol < 2 {
			return salah("harus gabungan kolom")
		}
	}
	return nil
}

// pelanggaranFK memeriksa satu kunci asing: tipe sama dengan kolom induk, aturan hapus eksplisit, dan indeks
// pada kolom anak.
func pelanggaranFK(fk kunciAsing) []string {
	var out []string
	if fk.TipeAnak != fk.TipeInduk {
		out = append(out, fmt.Sprintf("%s: FK %s(%s) bertipe %s, induk %s(%s) bertipe %s",
			fk.Anak, fk.Nama, fk.Kolom, fk.TipeAnak, fk.Induk, fk.KolomInduk, fk.TipeInduk))
	}
	if fk.SaatHapus == "NO ACTION" || fk.SaatHapus == "SET DEFAULT" {
		out = append(out, fmt.Sprintf("%s: FK %s(%s) memakai ON DELETE %s; tetapkan RESTRICT, CASCADE, atau SET NULL",
			fk.Anak, fk.Nama, fk.Kolom, fk.SaatHapus))
	}
	if !fk.Terindeks {
		out = append(out, fmt.Sprintf("%s: FK %s(%s) tidak punya indeks dengan kolom awal (%s)",
			fk.Anak, fk.Nama, fk.Kolom, fk.Kolom))
	}
	return out
}

func TestInvarianKunci(t *testing.T) {
	db := pg.NewDatabase(t, "invarian_kunci")
	pgtest.MigrateUp(t, db)

	semua := bacaTabel(t, db)
	pks := bacaPK(t, db)
	fks := bacaFK(t, db)

	perTabel := map[string][]string{}
	for _, tabel := range semua {
		kat, terdaftar := kategoriPK[tabel]
		if !terdaftar {
			perTabel[tabel] = append(perTabel[tabel], tabel+": belum terdaftar di kategoriPK (kunci_test.go dan docs/SKEMA-DATABASE.md)")
			continue
		}
		pk, ada := pks[tabel]
		perTabel[tabel] = append(perTabel[tabel], pelanggaranPK(tabel, kat, pk, ada)...)
	}
	for _, fk := range fks {
		perTabel[fk.Anak] = append(perTabel[fk.Anak], pelanggaranFK(fk)...)
	}
	for tabel := range kategoriPK {
		if !slices.Contains(semua, tabel) {
			perTabel[tabel] = append(perTabel[tabel], tabel+": terdaftar di kategoriPK tetapi tidak ada di migrasi")
		}
	}

	var gagal []string
	for _, tabel := range sortedKeys(perTabel) {
		masalah := perTabel[tabel]
		migrasi, ditunda := tabelBelumSelaras[tabel]
		switch {
		case len(masalah) > 0 && !ditunda:
			gagal = append(gagal, masalah...)
		case len(masalah) == 0 && ditunda:
			gagal = append(gagal, fmt.Sprintf("%s: sudah selaras; hapus dari tabelBelumSelaras (%s)", tabel, migrasi))
		}
	}
	for tabel, migrasi := range tabelBelumSelaras {
		if _, ada := perTabel[tabel]; !ada && !slices.Contains(semua, tabel) {
			gagal = append(gagal, fmt.Sprintf("%s: tercantum di tabelBelumSelaras (%s) tetapi tidak ada", tabel, migrasi))
		}
	}
	sort.Strings(gagal)
	assert.Empty(t, gagal, "invarian kunci dilanggar:\n%s", strings.Join(gagal, "\n"))
}

// TestFKPenggunaSelaras: setiap FK ke pengguna(id), di tabel mana pun (termasuk tabel yang konversi PK-nya masih
// tercantum di tabelBelumSelaras), sudah bertipe uuid, punya ON DELETE eksplisit, dan terindeks (migrasi 000110).
// Pelanggaran baru, mis. FK ke pengguna di tabel yang dibuat migrasi berikutnya dengan VARCHAR, ditolak di sini.
func TestFKPenggunaSelaras(t *testing.T) {
	db := pg.NewDatabase(t, "fk_pengguna")
	pgtest.MigrateUp(t, db)

	var jumlah int
	var gagal []string
	for _, fk := range bacaFK(t, db) {
		if fk.Induk != "pengguna" {
			continue
		}
		jumlah++
		gagal = append(gagal, pelanggaranFK(fk)...)
		if fk.TipeAnak != "uuid" {
			gagal = append(gagal, fmt.Sprintf("%s: FK %s(%s) bertipe %s, harus uuid", fk.Anak, fk.Nama, fk.Kolom, fk.TipeAnak))
		}
	}
	assert.Greater(t, jumlah, 40, "FK ke pengguna seharusnya puluhan; bacaFK mungkin salah")
	assert.Empty(t, gagal, "FK ke pengguna melanggar standar:\n%s", strings.Join(gagal, "\n"))
}

// TestUUIDStabil_SamaDenganSQL: idx.UUIDStabil (Go) dan garda_uuid_stabil (SQL, migrasi 000109) harus
// menghasilkan UUID yang sama, karena seed dan migrasi data bergantung pada kesamaan itu.
func TestUUIDStabil_SamaDenganSQL(t *testing.T) {
	db := pg.NewDatabase(t, "uuid_stabil")
	pgtest.MigrateUp(t, db)

	kasus := [][2]string{
		{"pengguna", "superadmin"},
		{"pengguna", "test-user-001"},
		{"intervensi", "INT-1000"},
		{"policy_brief", "PB-2026-10"},
		{"sumber_data", "SRC-BAPANAS"},
		{"cold_storage", "CS-01"},
		{"x", "é ñ 漢字"},
		{"", ""},
	}
	for _, k := range kasus {
		var dariSQL string
		require.NoError(t, db.Raw("SELECT garda_uuid_stabil(?, ?)::text", k[0], k[1]).Scan(&dariSQL).Error)
		assert.Equal(t, idx.UUIDStabil(k[0], k[1]), dariSQL, "%s:%s", k[0], k[1])
	}
}
