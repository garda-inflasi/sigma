package seeder

import (
	"encoding/json"
	"flag"
	"os"
	"strings"
	"testing"

	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const fixturePath = "../../../../../packages/contracts/master-reference.json"

var (
	pg            *pgtest.Server
	updateFixture = flag.Bool("update-fixture", false, "tulis ulang packages/contracts/master-reference.json dari seed")
)

func TestMain(m *testing.M) {
	os.Exit(pgtest.Main(m, &pg))
}

// Fixture adalah cermin seed yang dibaca tes web: reference.ts harus sama dengannya.
type (
	fixtureKabupaten struct {
		Kode    string  `json:"kode"`
		Nama    string  `json:"nama"`
		Jenis   string  `json:"jenis"`
		Ibukota string  `json:"ibukota"`
		Lat     float64 `json:"lat"`
		Lng     float64 `json:"lng"`
	}
	fixturePasar struct {
		ID            string  `json:"id"`
		Nama          string  `json:"nama"`
		KabupatenKode string  `json:"kabupaten_kode"`
		Lat           float64 `json:"lat"`
		Lng           float64 `json:"lng"`
		RadiusMeter   int     `json:"radius_meter"`
	}
	fixtureKomoditas struct {
		Kode          string `json:"kode"`
		Nama          string `json:"nama"`
		Satuan        string `json:"satuan"`
		Kategori      string `json:"kategori"`
		HargaAcuan    int    `json:"harga_acuan"`
		Pilot         bool   `json:"pilot"`
		ShelfLifeHari int    `json:"shelf_life_hari"`
	}
	referenceFixture struct {
		Kabupaten []fixtureKabupaten `json:"kabupaten"`
		Pasar     []fixturePasar     `json:"pasar"`
		Komoditas []fixtureKomoditas `json:"komoditas"`
	}
)

func currentFixture() referenceFixture {
	var f referenceFixture
	for _, k := range Kabupaten() {
		f.Kabupaten = append(f.Kabupaten, fixtureKabupaten{k.Kode, k.Nama, k.Jenis, k.Ibukota, k.Lat, k.Lng})
	}
	for _, p := range Pasar() {
		f.Pasar = append(f.Pasar, fixturePasar{p.ID, p.Nama, p.KabupatenKode, p.Lat, p.Lng, p.RadiusMeter})
	}
	for _, k := range Komoditas() {
		f.Komoditas = append(f.Komoditas, fixtureKomoditas{k.Kode, k.Nama, k.Satuan, k.Kategori, k.HargaAcuan, k.Pilot, k.ShelfLifeHari})
	}
	return f
}

func TestFixtureFileMatchesSeed(t *testing.T) {
	want := currentFixture()

	if *updateFixture {
		raw, err := json.MarshalIndent(want, "", "  ")
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(fixturePath, append(raw, '\n'), 0o644))
	}

	raw, err := os.ReadFile(fixturePath)
	require.NoError(t, err)
	var got referenceFixture
	require.NoError(t, json.Unmarshal(raw, &got))

	assert.Equal(t, want, got, "master-reference.json basi; jalankan: go test ./internal/master/seeder -run TestFixtureFileMatchesSeed -update-fixture")
}

func TestSeedData_Shape(t *testing.T) {
	kabupaten, pasar, komoditas := Kabupaten(), Pasar(), Komoditas()

	t.Run("23 kabupaten/kota se-Aceh dengan kode BPS unik", func(t *testing.T) {
		require.Len(t, kabupaten, 23)
		kode := map[string]bool{}
		var kota int
		for _, k := range kabupaten {
			assert.False(t, kode[k.Kode], "kode ganda %s", k.Kode)
			kode[k.Kode] = true
			assert.Equal(t, "11", k.ProvinsiKode)
			assert.True(t, strings.HasPrefix(k.Kode, "11"))
			assert.Contains(t, []string{"Kabupaten", "Kota"}, k.Jenis)
			prefix := map[string]string{"Kabupaten": "KABUPATEN ", "Kota": "KOTA "}[k.Jenis]
			assert.True(t, strings.HasPrefix(k.Nama, prefix), "%s: nama %q harus berawalan %q", k.Kode, k.Nama, prefix)
			if k.Jenis == "Kota" {
				kota++
			}
		}
		assert.Equal(t, 5, kota)
	})

	t.Run("setiap kabupaten punya pasar dan pasar merujuk kabupaten yang ada", func(t *testing.T) {
		kode := map[string]bool{}
		for _, k := range kabupaten {
			kode[k.Kode] = true
		}
		withMarket, ids := map[string]bool{}, map[string]bool{}
		for _, p := range pasar {
			assert.True(t, kode[p.KabupatenKode], "pasar %s merujuk kabupaten %s yang tidak ada", p.ID, p.KabupatenKode)
			assert.False(t, ids[p.ID], "id pasar ganda %s", p.ID)
			ids[p.ID] = true
			withMarket[p.KabupatenKode] = true
			assert.Positive(t, p.RadiusMeter)
		}
		assert.Len(t, withMarket, 23)
	})

	t.Run("komoditas: tiga pilot dan nilai positif", func(t *testing.T) {
		var pilot []string
		for _, k := range komoditas {
			assert.Positive(t, k.HargaAcuan, k.Kode)
			assert.Positive(t, k.ShelfLifeHari, k.Kode)
			if k.Pilot {
				pilot = append(pilot, k.Kode)
			}
		}
		assert.ElementsMatch(t, []string{"BRS-MED", "CBM-KRT", "BWM"}, pilot, "beras, cabai merah, dan bawang merah")
	})

	t.Run("cakup komoditas yang dipakai cold storage", func(t *testing.T) {
		kode := map[string]bool{}
		for _, k := range komoditas {
			kode[k.Kode] = true
		}
		assert.True(t, kode["TMT"] && kode["KTG"], "tomat dan kentang kini bersumber dari master.reference")
	})
}

func TestSeedReference_IsIdempotentAndKeepsOperationalColumns(t *testing.T) {
	db := pg.NewDatabase(t, "master_seed_reference")
	pgtest.MigrateUp(t, db)

	first, err := SeedReference(t.Context(), db)
	require.NoError(t, err)
	assert.Equal(t, ReferenceSummary{Provinsi: 1, Kabupaten: 23, Pasar: 24, Komoditas: 14}, first)

	afterFirst := rowCounts(t, db)

	// Perubahan operasional (persetujuan, nonaktif) dan koreksi referensi.
	require.NoError(t, db.Exec(`UPDATE komoditas SET status = 'MENUNGGU_PERSETUJUAN', versi = 3, harga_acuan = 1 WHERE kode = 'BRS-MED'`).Error)
	require.NoError(t, db.Exec(`UPDATE pasar SET aktif = false WHERE id = 'PSR-1171-01'`).Error)

	_, err = SeedReference(t.Context(), db)
	require.NoError(t, err)

	assert.Equal(t, afterFirst, rowCounts(t, db), "menjalankan ulang tidak menambah baris")
	var row struct {
		Status     string
		Versi      int
		HargaAcuan int
	}
	require.NoError(t, db.Raw(`SELECT status::text AS status, versi, harga_acuan FROM komoditas WHERE kode = 'BRS-MED'`).Scan(&row).Error)
	assert.Equal(t, "MENUNGGU_PERSETUJUAN", row.Status, "status operasional tidak ditimpa")
	assert.Equal(t, 3, row.Versi)
	assert.Equal(t, 13900, row.HargaAcuan, "kolom referensi dikembalikan")
	var aktif bool
	require.NoError(t, db.Raw(`SELECT aktif FROM pasar WHERE id = 'PSR-1171-01'`).Scan(&aktif).Error)
	assert.False(t, aktif, "status aktif pasar tidak ditimpa")
}

func rowCounts(t *testing.T, db *gorm.DB) map[string]int64 {
	t.Helper()
	counts := map[string]int64{}
	for _, table := range []string{"provinsi", "kabupaten", "pasar", "komoditas"} {
		counts[table] = rowCount(t, db, table)
	}
	return counts
}

func rowCount(t *testing.T, db *gorm.DB, table string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Table(table).Count(&n).Error)
	return n
}

func TestSeedReference_DoesNotTouchNonReferenceData(t *testing.T) {
	db := pg.NewDatabase(t, "master_seed_non_reference")
	pgtest.MigrateUp(t, db)
	require.NoError(t, db.Exec(`
		INSERT INTO pengguna (id, nama, email, peran, lapis, lembaga)
		VALUES ('u-uji-seed', 'U', 'u-uji-seed@gardainflasi.id', 'OPS_MAHASISWA', 'OPERASIONAL', 'X')`).Error)
	before := rowCount(t, db, "pengguna")

	_, err := SeedReference(t.Context(), db)
	require.NoError(t, err)

	assert.Equal(t, before, rowCount(t, db, "pengguna"))
}
