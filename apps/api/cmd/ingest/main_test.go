package main

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/garda-inflasi/api/internal/ingestion/ingestiontest"
	ingestionseeder "github.com/garda-inflasi/api/internal/ingestion/seeder"
	masterseeder "github.com/garda-inflasi/api/internal/master/seeder"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	sumberseeder "github.com/garda-inflasi/api/internal/sumber-data/seeder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var pg *pgtest.Server

func TestMain(m *testing.M) {
	os.Exit(pgtest.Main(m, &pg))
}

func env(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

func TestRun_ArgumenTidakValidDitolakTanpaBasisData(t *testing.T) {
	ctx := context.Background()
	assert.ErrorContains(t, run(ctx, nil, env(nil), &bytes.Buffer{}, nil), "perintah wajib")
	assert.ErrorContains(t, run(ctx, []string{"SRC-BI"}, env(nil), &bytes.Buffer{}, nil), "DATABASE_URL")
}

func siapkan(t *testing.T, name string) (*gorm.DB, func(args ...string) (string, error), *ingestiontest.Fetcher) {
	t.Helper()
	db := pg.NewDatabase(t, name)
	pgtest.MigrateUp(t, db)
	for _, err := range []error{
		func() error { _, err := masterseeder.SeedReference(t.Context(), db); return err }(),
		func() error { _, err := sumberseeder.SeedReference(t.Context(), db); return err }(),
		func() error { _, err := ingestionseeder.SeedReference(t.Context(), db); return err }(),
	} {
		require.NoError(t, err)
	}
	grid, err := os.ReadFile("../../internal/ingestion/connector/pihps/testdata/grid_kota.json")
	require.NoError(t, err)
	fetcher := &ingestiontest.Fetcher{Respons: map[string][]byte{"wilayah=101": grid}}

	jalan := func(args ...string) (string, error) {
		var out bytes.Buffer
		err := run(t.Context(), args, env(map[string]string{"DATABASE_URL": pg.DSN(name)}), &out, fetcher)
		return out.String(), err
	}
	return db, jalan, fetcher
}

func TestJalankan_KonfigurasiDanPemetaanLewatCLI(t *testing.T) {
	db, jalan, fetcher := siapkan(t, "cmd_ingest_alur")

	// Belum dikonfigurasi: run tercatat GAGAL dan perintah keluar dengan galat.
	out, err := jalan("SRC-BI")
	require.Error(t, err)
	assert.Contains(t, out, "status=GAGAL")
	assert.Contains(t, out, "province_id")

	out, err = jalan("konfigurasi", "SRC-BI", `{"province_id":"7"}`)
	require.NoError(t, err, out)
	out, err = jalan("pemetaan", "SRC-BI", "wilayah", "101", "1171")
	require.NoError(t, err, out)
	assert.Contains(t, out, `"101" -> 1171`)

	out, err = jalan("jalankan", "SRC-BI")
	require.NoError(t, err, out)
	assert.Contains(t, out, "status=SUKSES")
	assert.Contains(t, out, "diambil=14")
	assert.Contains(t, out, "tak_terpetakan=3")
	assert.Equal(t, []string{"wilayah=101"}, fetcher.Dipanggil())

	var n int64
	require.NoError(t, db.Table("harga_eksternal").Count(&n).Error)
	assert.EqualValues(t, 14, n)
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM eksekusi_sumber WHERE pemicu = 'CLI'`).Scan(&n).Error)
	assert.EqualValues(t, 2, n)

	// Perubahan konfigurasi dan pemetaan oleh operator diaudit.
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM audit_log WHERE aksi IN ('UBAH_KONFIGURASI_SUMBER', 'UBAH_PEMETAAN_SUMBER', 'JALANKAN_SUMBER')`).Scan(&n).Error)
	assert.EqualValues(t, 4, n)
}

func TestPemetaan_MenolakMasukanSalah(t *testing.T) {
	_, jalan, _ := siapkan(t, "cmd_ingest_pemetaan")

	for nama, args := range map[string][]string{
		"jumlah argumen":           {"pemetaan", "SRC-BI", "WILAYAH", "101"},
		"jenis asing":              {"pemetaan", "SRC-BI", "DESA", "x", "1171"},
		"alias kosong":             {"pemetaan", "SRC-BI", "WILAYAH", "   ", "1171"},
		"kode tak ada":             {"pemetaan", "SRC-BI", "WILAYAH", "101", "9999"},
		"komoditas asing":          {"pemetaan", "SRC-BI", "KOMODITAS", "ikan", "XXX"},
		"sumber tak ada":           {"pemetaan", "SRC-ASING", "WILAYAH", "101", "1171"},
		"konfigurasi bukan objek":  {"konfigurasi", "SRC-BI", `[1]`},
		"konfigurasi sumber asing": {"konfigurasi", "SRC-ASING", `{}`},
		"sumber tak diberikan":     {"jalankan"},
	} {
		_, err := jalan(args...)
		assert.Error(t, err, nama)
	}

	// Alias diabaikan ("-") dan dinormalisasi.
	out, err := jalan("pemetaan", "SRC-BI", "KOMODITAS", "  Ikan   SEGAR Baru ", "-")
	require.NoError(t, err, out)
	assert.Contains(t, out, `"ikan segar baru" -> -`)
}

func TestDaftar_MenampilkanKetersediaanKonektor(t *testing.T) {
	_, jalan, _ := siapkan(t, "cmd_ingest_daftar")

	out, err := jalan("daftar")

	require.NoError(t, err)
	assert.Regexp(t, `SRC-BI\s+\S+\s+konektor tersedia`, out)
	assert.Regexp(t, `SRC-DJPB\s+\S+\s+tanpa konektor`, out)
}

func TestJalankan_SumberTanpaKonektorDitolak(t *testing.T) {
	_, jalan, _ := siapkan(t, "cmd_ingest_tanpa")

	_, err := jalan("SRC-DJPB")
	assert.ErrorContains(t, err, "konektor")
	_, err = jalan("SRC-ASING")
	assert.ErrorContains(t, err, "tidak ditemukan")
}
