package integration

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/ingestion/domain"
	"github.com/garda-inflasi/api/internal/master"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	sumberdomain "github.com/garda-inflasi/api/internal/sumber-data/domain"
	"github.com/garda-inflasi/api/pkg/seed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Tes kontrak seed ingestion: reference/04_sumber_data.sql, reference/05_ingestion_mapping.sql, demo/04_cuaca_harian.sql,
// dan demo/05_ingestion_demo.sql. Seeder Go lama (internal/ingestion/seeder dan internal/sumber-data/seeder) punya tes
// bentuk data, idempotensi, dan "tidak menimpa hasil ingestion nyata"; tes itu ikut terhapus bersama paketnya, jadi
// kontraknya dijaga di sini terhadap PostgreSQL sungguhan. Pemetaan dibandingkan dengan data Go (domain.SemuaPemetaan);
// nilai harga dan cuaca demo dibandingkan dengan PRNG tetap seeder lama yang diulang di bawah.

func seedIngestion(t *testing.T, nama string, berkas ...string) *gorm.DB {
	t.Helper()
	db := pg.NewDatabase(t, nama)
	pgtest.MigrateUp(t, db)
	require.NoError(t, seed.SeedFiles(t.Context(), db, berkas...))
	return db
}

func hitungSeed(t *testing.T, db *gorm.DB, sql string, args ...any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Raw(sql, args...).Scan(&n).Error)
	return n
}

var berkasReferensi = []string{"reference/01_master.sql", "reference/04_sumber_data.sql", "reference/05_ingestion_mapping.sql"}

func TestSeed_KatalogSumberDanPemetaanSamaDenganData(t *testing.T) {
	db := seedIngestion(t, "ingest_seed_referensi", berkasReferensi...)

	t.Run("katalog sumber: delapan sumber dengan UUID stabil, belum aktif", func(t *testing.T) {
		var got []struct{ Kode, ID, Status string }
		require.NoError(t, db.Raw(`SELECT kode, id::text AS id, status::text AS status FROM sumber_data ORDER BY kode`).Scan(&got).Error)
		kode := []string{"SRC-BAPANAS", "SRC-BI", "SRC-BMKG", "SRC-BPS", "SRC-CS", "SRC-DJPB", "SRC-OWM", "SRC-PORTAL"}
		require.Len(t, got, len(kode))
		for i, k := range kode {
			assert.Equal(t, k, got[i].Kode)
			assert.Equal(t, sumberdomain.IDSumber(k), got[i].ID, "UUID sumber %s = garda_uuid_stabil('sumber_data', kode)", k)
			assert.Equal(t, "BELUM_AKTIF", got[i].Status, "tier referensi jujur: belum ada pengambilan data nyata")
		}
		assert.Equal(t, `{"domain": "1100"}`, func() string {
			var s string
			require.NoError(t, db.Raw(`SELECT konfigurasi::text FROM sumber_data WHERE kode = 'SRC-BPS'`).Scan(&s).Error)
			return s
		}(), "domain BPS Provinsi Aceh")
	})

	t.Run("pemetaan sama dengan data Go", func(t *testing.T) {
		type baris struct {
			SumberID, Jenis, Alias, KodeMaster string
			Diabaikan                          bool
			Catatan                            string
		}
		var got []baris
		require.NoError(t, db.Raw(`SELECT sumber_id::text AS sumber_id, jenis::text AS jenis, alias, coalesce(kode_master, '') AS kode_master,
			diabaikan, coalesce(catatan, '') AS catatan FROM pemetaan_sumber`).Scan(&got).Error)
		gotSet := map[baris]bool{}
		for _, g := range got {
			gotSet[g] = true
		}
		want := domain.SemuaPemetaan()
		require.Len(t, got, len(want), "jumlah baris pemetaan")
		for _, w := range want {
			b := baris{SumberID: w.SumberID, Jenis: w.Jenis, Alias: w.Alias, Diabaikan: w.Diabaikan}
			if w.KodeMaster != nil {
				b.KodeMaster = *w.KodeMaster
			}
			if w.Catatan != nil {
				b.Catatan = *w.Catatan
			}
			assert.True(t, gotSet[b], "pemetaan %s/%s/%s tidak sama dengan data Go", w.SumberID, w.Jenis, w.Alias)
		}
	})

	t.Run("kode master pemetaan ada di master", func(t *testing.T) {
		assert.Zero(t, hitungSeed(t, db, `SELECT count(*) FROM pemetaan_sumber p WHERE p.kode_master IS NOT NULL AND (
			(p.jenis = 'KOMODITAS' AND NOT EXISTS (SELECT 1 FROM komoditas k WHERE k.kode = p.kode_master)) OR
			(p.jenis = 'WILAYAH' AND p.kode_master <> '11' AND NOT EXISTS (SELECT 1 FROM kabupaten k WHERE k.kode = p.kode_master)))`))
	})
}

func TestSeed_ReferensiIdempotenDanTidakMenimpaKeadaanOperasional(t *testing.T) {
	db := seedIngestion(t, "ingest_seed_operasional", berkasReferensi...)
	jumlahPemetaan := hitungSeed(t, db, `SELECT count(*) FROM pemetaan_sumber`)

	// Operator mengaktifkan sumber, mengisi konfigurasi, dan mengubah satu pemetaan; seed ulang menyegarkan identitas
	// katalog (nama) tetapi tidak menyentuh status, konfigurasi, maupun pemetaan hasil operasi.
	require.NoError(t, db.Exec(`UPDATE sumber_data SET status = 'SEHAT', konfigurasi = '{"province_id":"7"}', nama = 'Nama basi' WHERE kode = 'SRC-BI'`).Error)
	require.NoError(t, db.Exec(`UPDATE pemetaan_sumber SET kode_master = 'TLR' WHERE alias = 'beras kualitas medium i'`).Error)

	require.NoError(t, seed.SeedFiles(t.Context(), db, berkasReferensi[1:]...))

	assert.Equal(t, jumlahPemetaan, hitungSeed(t, db, `SELECT count(*) FROM pemetaan_sumber`))
	assert.EqualValues(t, 8, hitungSeed(t, db, `SELECT count(*) FROM sumber_data`))
	assert.EqualValues(t, 1, hitungSeed(t, db, `SELECT count(*) FROM sumber_data WHERE kode = 'SRC-BI' AND status = 'SEHAT' AND konfigurasi = '{"province_id":"7"}'::jsonb AND nama = 'PIHPS Nasional — harga pangan'`))
	assert.EqualValues(t, 1, hitungSeed(t, db, `SELECT count(*) FROM pemetaan_sumber WHERE alias = 'beras kualitas medium i' AND kode_master = 'TLR'`), "pemetaan operator tidak ditimpa")
}

// Titik pantau BMKG lama adalah tebakan: alias adm4 memakai awalan kode BPS (11.01.01.2001 -> 1101), padahal BMKG
// memakai kode Kemendagri (11.01 = Aceh Selatan, bukan Simeulue). Basis data yang sudah ter-seed menyimpan baris itu
// (seed memakai ON CONFLICT DO NOTHING), jadi seed ulang harus membuangnya, tanpa menyentuh titik pilihan operator.
func TestSeed_TitikBMKGTebakanLamaDibersihkanTanpaMenyentuhPilihanOperator(t *testing.T) {
	db := seedIngestion(t, "ingest_seed_bmkg_lama", berkasReferensi...)
	const bmkg = "(SELECT id FROM sumber_data WHERE kode = 'SRC-BMKG')"

	// Keadaan basis data lama: 23 titik tebakan (awalan BPS), satu di antaranya sudah dipilih ulang operator.
	require.NoError(t, db.Exec(`DELETE FROM pemetaan_sumber WHERE sumber_id = `+bmkg+` AND jenis = 'WILAYAH'`).Error)
	require.NoError(t, db.Exec(`INSERT INTO pemetaan_sumber (sumber_id, jenis, alias, kode_master, diabaikan, catatan)
		SELECT `+bmkg+`, 'WILAYAH', substr(kode, 1, 2) || '.' || substr(kode, 3) || '.01.2001', kode, FALSE,
		       'titik pantau awal, belum terverifikasi' FROM kabupaten WHERE kode <> '11'`).Error)
	require.EqualValues(t, 23, hitungSeed(t, db, `SELECT count(*) FROM pemetaan_sumber WHERE sumber_id = `+bmkg+` AND jenis = 'WILAYAH'`))
	require.NoError(t, db.Exec(`UPDATE pemetaan_sumber SET alias = '11.71.01.1002', catatan = 'diatur lewat gardactl ingest'
		WHERE sumber_id = `+bmkg+` AND alias = '11.71.01.2001'`).Error)

	require.NoError(t, seed.SeedFiles(t.Context(), db, "reference/05_ingestion_mapping.sql"))

	assert.EqualValues(t, 24, hitungSeed(t, db, `SELECT count(*) FROM pemetaan_sumber WHERE sumber_id = `+bmkg+` AND jenis = 'WILAYAH'`),
		"23 titik terverifikasi + 1 titik operator (titik tebakan Banda Aceh digantikan pilihan operator tetapi alias bawaannya kembali dari seed)")
	assert.Zero(t, hitungSeed(t, db, `SELECT count(*) FROM pemetaan_sumber WHERE catatan = 'titik pantau awal, belum terverifikasi'`),
		"tidak ada baris tebakan yang tersisa")
	assert.EqualValues(t, 1, hitungSeed(t, db, `SELECT count(*) FROM pemetaan_sumber WHERE alias = '11.71.01.1002' AND kode_master = '1171' AND catatan = 'diatur lewat gardactl ingest'`),
		"pilihan operator tidak disentuh")

	// Contoh yang salah di data lama sekarang benar: Simeulue (1101) tidak lagi dipetakan dari 11.01.* (Aceh Selatan).
	assert.EqualValues(t, 1, hitungSeed(t, db, `SELECT count(*) FROM pemetaan_sumber WHERE sumber_id = `+bmkg+` AND alias = '11.09.02.2001' AND kode_master = '1101'`))
	assert.Zero(t, hitungSeed(t, db, `SELECT count(*) FROM pemetaan_sumber WHERE sumber_id = `+bmkg+` AND alias = '11.01.01.2001' AND kode_master = '1101'`))
	assert.EqualValues(t, 1, hitungSeed(t, db, `SELECT count(*) FROM pemetaan_sumber WHERE sumber_id = `+bmkg+` AND alias = '11.74.03.2001' AND kode_master = '1173'`), "Langsa")
	assert.EqualValues(t, 1, hitungSeed(t, db, `SELECT count(*) FROM pemetaan_sumber WHERE sumber_id = `+bmkg+` AND alias = '11.73.01.2002' AND kode_master = '1174'`), "Lhokseumawe")

	// Seed ulang lagi tidak mengubah apa pun.
	require.NoError(t, seed.SeedFiles(t.Context(), db, "reference/05_ingestion_mapping.sql"))
	assert.EqualValues(t, 24, hitungSeed(t, db, `SELECT count(*) FROM pemetaan_sumber WHERE sumber_id = `+bmkg+` AND jenis = 'WILAYAH'`))
}

// hariIniWIB adalah tanggal hari ini menurut WIB, patokan seed demo ingestion.
func hariIniWIB(t *testing.T, db *gorm.DB) time.Time {
	t.Helper()
	var d time.Time
	require.NoError(t, db.Raw(`SELECT (now() AT TIME ZONE 'Asia/Jakarta')::date`).Scan(&d).Error)
	return d
}

func TestSeed_DemoIngestionSamaDenganSeederLama(t *testing.T) {
	db := seedIngestion(t, "ingest_seed_demo", append(berkasReferensi, "demo/04_cuaca_harian.sql", "demo/05_ingestion_demo.sql")...)

	t.Run("sembilan eksekusi contoh dan tujuh payload bronze", func(t *testing.T) {
		var status []struct {
			Status string
			N      int
		}
		require.NoError(t, db.Raw(`SELECT status, count(*) AS n FROM eksekusi_sumber WHERE pemicu = 'DEMO' GROUP BY 1 ORDER BY 1`).Scan(&status).Error)
		assert.Equal(t, []struct {
			Status string
			N      int
		}{{"GAGAL", 2}, {"SEBAGIAN", 2}, {"SUKSES", 5}}, status)
		assert.EqualValues(t, 9, hitungSeed(t, db, `SELECT count(*) FROM eksekusi_sumber`), "tidak ada eksekusi selain contoh")
		assert.EqualValues(t, 7, hitungSeed(t, db, `SELECT count(*) FROM data_mentah`))
		assert.Zero(t, hitungSeed(t, db, `SELECT count(*) FROM eksekusi_sumber WHERE status = 'BERJALAN'`))

		// Eksekusi BPS gagal jujur karena variabel belum dikonfigurasi, bukan karena data contoh.
		assert.EqualValues(t, 2, hitungSeed(t, db, `SELECT count(*) FROM eksekusi_sumber e JOIN sumber_data s ON s.id = e.sumber_id
			WHERE s.kode = 'SRC-BPS' AND e.status = 'GAGAL' AND e.dokumen = 0 AND e.galat LIKE 'konfigurasi.variabel SRC-BPS belum lengkap%'`))
		// Jumlah baris konsisten: dibaca = diambil + tak terpetakan + tidak valid, ringkasan memuat permintaan.
		assert.Zero(t, hitungSeed(t, db, `SELECT count(*) FROM eksekusi_sumber WHERE baris_dibaca <> baris_diambil + baris_tak_terpetakan + baris_tidak_valid`))
		assert.Zero(t, hitungSeed(t, db, `SELECT count(*) FROM eksekusi_sumber WHERE (ringkasan->'permintaan'->>'berhasil')::int <> dokumen`))

		// Checksum adalah SHA-256 teks payload.
		var payload []struct {
			Kode, Diambil, Checksum string
		}
		require.NoError(t, db.Raw(`SELECT s.kode, to_char(m.diambil_pada AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS diambil, m.checksum
			FROM data_mentah m JOIN sumber_data s ON s.id = m.sumber_id`).Scan(&payload).Error)
		require.Len(t, payload, 7)
		for _, p := range payload {
			teks := fmt.Sprintf(`{"demo":true,"sumber":%q,"catatan":"payload contoh, bukan respons layanan nyata","diambil":%q}`, p.Kode, p.Diambil)
			sum := sha256.Sum256([]byte(teks))
			assert.Equal(t, hex.EncodeToString(sum[:]), p.Checksum, p.Kode)
		}
	})

	t.Run("keadaan eksekusi sumber di katalog", func(t *testing.T) {
		var got []struct {
			Kode, Status string
			MenitLalu    float64
			Latensi      int
		}
		require.NoError(t, db.Raw(`SELECT kode, status::text AS status, extract(epoch FROM now() - terakhir_jalan) / 60 AS menit_lalu, latensi_ms AS latensi
			FROM sumber_data ORDER BY kode`).Scan(&got).Error)
		want := map[string]struct {
			status  string
			menit   float64
			latensi int
		}{
			"SRC-BPS": {"SEHAT", 142, 640}, "SRC-BI": {"SEHAT", 38, 910}, "SRC-DJPB": {"SEHAT", 2880, 1220}, "SRC-BMKG": {"SEHAT", 51, 480},
			"SRC-OWM": {"SEHAT", 12, 210}, "SRC-BAPANAS": {"TERTUNDA", 325, 3400}, "SRC-PORTAL": {"GAGAL", 1460, 0}, "SRC-CS": {"SEHAT", 3, 95},
		}
		require.Len(t, got, len(want))
		for _, g := range got {
			assert.Equal(t, want[g.Kode].status, g.Status, g.Kode)
			assert.InDelta(t, want[g.Kode].menit, g.MenitLalu, 1, g.Kode)
			assert.Equal(t, want[g.Kode].latensi, g.Latensi, g.Kode)
		}
	})

	t.Run("harga eksternal sama dengan PRNG seeder lama, berakhir kemarin", func(t *testing.T) {
		acuan := map[string]float64{}
		for _, k := range master.Komoditas() {
			acuan[k.Kode] = float64(k.HargaAcuan)
		}
		rng := rand.New(rand.NewPCG(0x50494850, 0x53444d4f))
		type kunci struct {
			Kota, Kode string
			H          int
		}
		want := map[kunci]float64{}
		for _, kota := range []string{"1171", "1108", "1174", "1110", "1109"} {
			for _, kode := range []string{"BRS-MED", "CBM-KRT", "BWM", "TLR"} {
				for h := 14; h >= 1; h-- {
					tren := 1 + 0.002*float64(14-h)
					noise := 1 + (rng.Float64()-0.5)*0.06
					want[kunci{kota, kode, h}] = math.Round(acuan[kode]*tren*noise/100) * 100
				}
			}
		}
		hariIni := hariIniWIB(t, db)
		var got []struct {
			KabupatenKode, KomoditasKode string
			Tanggal                      time.Time
			Harga                        float64
			Satuan                       string
		}
		require.NoError(t, db.Raw(`SELECT kabupaten_kode, komoditas_kode, tanggal, harga::float8 AS harga, satuan FROM harga_eksternal`).Scan(&got).Error)
		require.Len(t, got, len(want))
		for _, g := range got {
			h := int(hariIni.Sub(g.Tanggal).Hours() / 24)
			assert.Equal(t, want[kunci{g.KabupatenKode, g.KomoditasKode, h}], g.Harga, "%s/%s h=%d", g.KabupatenKode, g.KomoditasKode, h)
			assert.Equal(t, "kg", g.Satuan)
		}
		// Lineage ke eksekusi dan payload PIHPS contoh terakhir.
		assert.EqualValues(t, 280, hitungSeed(t, db, `SELECT count(*) FROM harga_eksternal h JOIN eksekusi_sumber e ON e.id = h.eksekusi_id JOIN data_mentah m ON m.id = h.data_mentah_id
			JOIN sumber_data s ON s.id = e.sumber_id AND s.id = h.sumber_id WHERE s.kode = 'SRC-BI' AND e.pemicu = 'DEMO' AND m.eksekusi_id = e.id
			AND e.mulai = (SELECT max(mulai) FROM eksekusi_sumber WHERE sumber_id = s.id)`))
	})

	t.Run("cuaca tujuh hari sama dengan PRNG seeder lama, dengan lineage BMKG", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(0x424d4b47, 0x44454d4f))
		type kunci struct {
			Kab string
			H   int
		}
		type nilai struct {
			Hujan, Suhu float64
			Lembap      int
		}
		want := map[kunci]nilai{}
		for _, kab := range master.Kabupaten() {
			for h := 6; h >= 0; h-- {
				hujan := 0.0
				if rng.Float64() < 0.45 {
					hujan = math.Round(rng.Float64()*600) / 10
				}
				suhu := math.Round((25+rng.Float64()*7)*10) / 10
				want[kunci{kab.Kode, h}] = nilai{hujan, suhu, 70 + rng.IntN(26)}
			}
		}
		hariIni := hariIniWIB(t, db)
		var got []struct {
			KabupatenKode string
			Tanggal       time.Time
			CurahHujanMm  float64
			SuhuC         float64
			Kelembapan    int
			Anomali       bool
		}
		require.NoError(t, db.Raw(`SELECT kabupaten_kode, tanggal, curah_hujan_mm, suhu_c, kelembapan, anomali FROM cuaca_harian`).Scan(&got).Error)
		require.Len(t, got, len(want), "23 kabupaten x 7 hari")
		for _, g := range got {
			h := int(hariIni.Sub(g.Tanggal).Hours() / 24)
			assert.Equal(t, want[kunci{g.KabupatenKode, h}], nilai{g.CurahHujanMm, g.SuhuC, g.Kelembapan}, "%s h=%d", g.KabupatenKode, h)
			assert.Equal(t, g.CurahHujanMm >= 50, g.Anomali, "hujan lebat (>= 50 mm) ditandai anomali")
		}
		assert.EqualValues(t, 161, hitungSeed(t, db, `SELECT count(*) FROM cuaca_harian c JOIN eksekusi_sumber e ON e.id = c.eksekusi_id JOIN data_mentah m ON m.id = c.data_mentah_id
			JOIN sumber_data s ON s.id = e.sumber_id WHERE s.kode = 'SRC-BMKG' AND e.pemicu = 'DEMO' AND m.eksekusi_id = e.id
			AND e.mulai = (SELECT max(mulai) FROM eksekusi_sumber WHERE sumber_id = s.id)`), "lineage ke eksekusi BMKG contoh terakhir")
	})
}

func TestSeed_DemoIngestionIdempotenDanTidakMenimpaHasilIngestionNyata(t *testing.T) {
	berkas := append(append([]string{}, berkasReferensi...), "demo/04_cuaca_harian.sql", "demo/05_ingestion_demo.sql")
	db := seedIngestion(t, "ingest_seed_demo_ulang", berkas...)
	hitung := func() [5]int64 {
		return [5]int64{
			hitungSeed(t, db, `SELECT count(*) FROM eksekusi_sumber`), hitungSeed(t, db, `SELECT count(*) FROM data_mentah`),
			hitungSeed(t, db, `SELECT count(*) FROM harga_eksternal`), hitungSeed(t, db, `SELECT count(*) FROM cuaca_harian`),
			hitungSeed(t, db, `SELECT count(*) FROM cuaca_harian WHERE eksekusi_id IS NOT NULL`),
		}
	}
	awal := hitung()

	// Hasil ingestion nyata: satu harga eksternal dan satu cuaca milik eksekusi JADWAL, pada tanggal yang juga dicakup seed.
	require.NoError(t, db.Exec(`INSERT INTO eksekusi_sumber (sumber_id, pemicu, status, selesai) VALUES
		((SELECT id FROM sumber_data WHERE kode = 'SRC-BI'), 'JADWAL', 'SUKSES', now()),
		((SELECT id FROM sumber_data WHERE kode = 'SRC-BMKG'), 'JADWAL', 'SUKSES', now())`).Error)
	require.NoError(t, db.Exec(`UPDATE harga_eksternal SET harga = 1, eksekusi_id = (SELECT max(id) - 1 FROM eksekusi_sumber)
		WHERE kabupaten_kode = '1171' AND komoditas_kode = 'BRS-MED' AND tanggal = (now() AT TIME ZONE 'Asia/Jakarta')::date - 2`).Error)
	require.NoError(t, db.Exec(`UPDATE cuaca_harian SET curah_hujan_mm = 1, eksekusi_id = (SELECT max(id) FROM eksekusi_sumber)
		WHERE kabupaten_kode = '1171' AND tanggal = (now() AT TIME ZONE 'Asia/Jakarta')::date - 2`).Error)

	require.NoError(t, seed.SeedFiles(t.Context(), db, "demo/04_cuaca_harian.sql", "demo/05_ingestion_demo.sql"))

	sesudah := hitung()
	assert.Equal(t, [5]int64{awal[0] + 2, awal[1], awal[2], awal[3], awal[4]}, sesudah, "hanya dua eksekusi JADWAL yang ditambahkan tes; seed ulang pada hari yang sama tidak menambah apa pun")
	assert.EqualValues(t, 1, hitungSeed(t, db, `SELECT count(*) FROM harga_eksternal WHERE harga = 1`), "harga hasil ingestion nyata tidak ditimpa")
	assert.EqualValues(t, 1, hitungSeed(t, db, `SELECT count(*) FROM cuaca_harian WHERE curah_hujan_mm = 1`), "cuaca hasil ingestion nyata tidak ditimpa")
	assert.EqualValues(t, 1, hitungSeed(t, db, `SELECT count(*) FROM cuaca_harian c JOIN eksekusi_sumber e ON e.id = c.eksekusi_id WHERE e.pemicu = 'JADWAL'`), "lineage cuaca nyata tidak dipindah ke eksekusi contoh")
}
