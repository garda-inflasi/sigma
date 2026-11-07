package migrate_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/dashboard/refresher"
	"github.com/garda-inflasi/api/internal/platform/migrate"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var pg *pgtest.Server

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m, &pg)) }

// Pengguna DB aplikasi berhak minimum (03-C, 19 §7): api dan worker bisa bekerja normal, tetapi tidak bisa mengubah
// jejak audit, menjalankan DDL, atau menyentuh tabel migrasi. Login memakai verifier SCRAM yang dihitung di Go.
func TestPenggunaAplikasi_HakMinimum(t *testing.T) {
	const nama, sandi = "garda_app_uji", "sandi-uji-cukup-panjang-1"
	ctx := t.Context()
	pemilik := pg.NewDatabase(t, "pengguna_aplikasi")
	pgtest.MigrateUp(t, pemilik)
	sqlDB, err := pemilik.DB()
	require.NoError(t, err)

	require.NoError(t, migrate.SiapkanPenggunaAplikasi(ctx, sqlDB, nama, sandi))
	app := pg.BukaSebagai(t, "pengguna_aplikasi", nama, sandi)

	// Jejak audit: tulis dan baca boleh, ubah/hapus tidak (ditolak hak akses, sebelum pemicu).
	require.NoError(t, audit.NewWriter(app).Write(ctx, audit.Entry{Aksi: "APP", Entitas: "uji", EntitasID: "1", Hasil: audit.HasilBerhasil}))
	for _, stmt := range []string{
		`UPDATE audit_log SET hasil = 'GAGAL'`,
		`DELETE FROM audit_log`,
		`TRUNCATE audit_log`,
		`UPDATE audit_log_verifikasi SET status = 'RUSAK'`,
		`DELETE FROM audit_log_arsip`,
		`UPDATE entri_harga_versi SET harga = 1`,
		`SELECT count(*) FROM ` + "audit_log_p" + time.Now().UTC().Format("200601"),
		`CREATE TABLE coba (a int)`,
		`ALTER TABLE audit_log DISABLE TRIGGER trg_audit_log_append_only`,
		`SELECT * FROM goose_db_version`,
		`SELECT audit_log_buat_partisi('2030-01-01')`,
	} {
		err := app.Exec(stmt).Error
		require.Error(t, err, stmt)
		assert.Contains(t, err.Error(), "42501", "ditolak hak akses: %s", stmt)
	}

	// Operasi aplikasi biasa tetap jalan.
	require.NoError(t, app.Exec(`UPDATE pengguna SET nama = nama WHERE false`).Error)
	require.NoError(t, refresher.Shared(app, logger.New(logger.ERROR, os.Stderr)).Refresh(ctx), "refresh MV lewat garda_segarkan_mv")
	integ := audit.NewIntegritas(app)
	_, err = integ.SiapkanPartisi(ctx, 6)
	require.NoError(t, err)
	v, err := integ.Verifikasi(ctx, "MANUAL")
	require.NoError(t, err)
	assert.Equal(t, "VALID", v.Status)

	// Retensi penuh di bawah hak minimum: partisi lama (dibuat pemilik) diekspor, diverifikasi, lalu dilepas.
	require.NoError(t, pemilik.Exec(`SELECT audit_log_buat_partisi('2024-01-01')`).Error)
	require.NoError(t, audit.NewWriter(app).Write(ctx, audit.Entry{Aksi: "LAMA", Entitas: "uji", EntitasID: "1", Hasil: audit.HasilBerhasil, Waktu: time.Date(2024, 1, 9, 0, 0, 0, 0, time.UTC)}))
	// Kepala rantai tidak pernah diarsipkan: tulis satu baris baru sesudah baris lama.
	require.NoError(t, audit.NewWriter(app).Write(ctx, audit.Entry{Aksi: "BARU", Entitas: "uji", EntitasID: "1", Hasil: audit.HasilBerhasil}))
	hasil, err := integ.Retensi(ctx, audit.KonfigRetensi{Bulan: 12, Dir: t.TempDir()})
	require.NoError(t, err)
	require.Len(t, hasil, 1)
	assert.Equal(t, "audit_log_p202401", hasil[0].Partisi)

	// Memperbarui kata sandi: sandi lama tidak berlaku lagi.
	const sandiBaru = "sandi-baru-yang-panjang-2"
	require.NoError(t, migrate.SiapkanPenggunaAplikasi(ctx, sqlDB, nama, sandiBaru))
	baru := pg.BukaSebagai(t, "pengguna_aplikasi", nama, sandiBaru)
	require.NoError(t, baru.Exec("SELECT 1").Error)
}

func TestPenggunaAplikasi_MenolakKonfigurasiBerbahaya(t *testing.T) {
	ctx := context.Background()
	db := pg.NewDatabase(t, "pengguna_aplikasi_tolak")
	pgtest.MigrateUp(t, db)
	sqlDB, err := db.DB()
	require.NoError(t, err)

	assert.Error(t, migrate.SiapkanPenggunaAplikasi(ctx, sqlDB, "garda", "sandi-uji-cukup-panjang-1"), "pemilik skema")
	assert.Error(t, migrate.SiapkanPenggunaAplikasi(ctx, sqlDB, "app_pendek", "pendek"), "sandi pendek")
	assert.Error(t, migrate.SiapkanPenggunaAplikasi(ctx, sqlDB, "app_spasi", "sandi dengan spasi panjang"), "spasi")
	assert.Error(t, migrate.SiapkanPenggunaAplikasi(ctx, sqlDB, `x"; DROP ROLE garda; --`, "sandi-uji-cukup-panjang-1"), "nama tidak valid")
	require.NoError(t, db.Exec(`CREATE ROLE super_uji SUPERUSER`).Error)
	assert.Error(t, migrate.SiapkanPenggunaAplikasi(ctx, sqlDB, "super_uji", "sandi-uji-cukup-panjang-1"), "superuser")
}
