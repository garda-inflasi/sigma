package seed

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/garda-inflasi/api/internal/seedkit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func env(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

func TestRun_RejectsBadArguments(t *testing.T) {
	for _, args := range [][]string{nil, {"reference", "test"}, {"test"}} {
		err := Run(args, env(nil), &bytes.Buffer{})
		require.Error(t, err, "%v", args)
		assert.Contains(t, err.Error(), "pemakaian: seed reference|admin|demo|all")
	}
}

func TestRun_DemoIsRefusedInProductionBeforeConnecting(t *testing.T) {
	// DATABASE_URL sengaja kosong: penolakan harus terjadi sebelum koneksi dicoba.
	for _, arg := range []string{"demo", "all"} {
		err := Run([]string{arg}, env(map[string]string{"APP_ENV": "production"}), &bytes.Buffer{})
		require.ErrorIs(t, err, seedkit.ErrDemoInProduction, arg)
	}
}

func TestRun_NeedsDatabaseURL(t *testing.T) {
	err := Run([]string{"reference"}, env(nil), &bytes.Buffer{})
	assert.ErrorContains(t, err, "DATABASE_URL")
}

func TestLocateSeedsDir(t *testing.T) {
	dir, err := LocateSeedsDir()
	require.NoError(t, err)
	assert.NotEmpty(t, dir)

	for _, tier := range []string{"reference", "admin", "demo"} {
		stat, err := os.Stat(filepath.Join(dir, tier))
		require.NoError(t, err, "tier folder %s harus ada", tier)
		assert.True(t, stat.IsDir())
	}
}

func TestSQLSeedFilesExistenceAndNonEmpty(t *testing.T) {
	dir, err := LocateSeedsDir()
	require.NoError(t, err)

	expectedTiers := map[string][]string{
		"reference": {
			"01_master.sql", "02_kecamatan.sql", "03_auth_rbac.sql", "04_sumber_data.sql",
			"05_ingestion_mapping.sql", "06_harga_pasar.sql", "07_ews_aturan.sql",
			"08_intervensi_tahap.sql", "09_cold_storage.sql", "10_spasial_geometri.sql",
			"11_prediksi_model.sql", "12_policy_brief_tpl.sql", "13_kpi_definisi.sql",
		},
		"admin": {
			"01_admin_user.sql",
		},
		"demo": {
			"01_auth_users.sql", "02_master_pedagang.sql", "03_harga_harian.sql", "04_cuaca_harian.sql",
			"05_ingestion_demo.sql", "06_ews_alerts.sql", "07_intervensi_kartu.sql", "08_lapangan_tugas.sql",
			"09_spasial_neraca.sql", "10_prediksi_harga.sql", "11_cold_storage_ops.sql", "12_policy_brief.sql",
			"13_kpi_rekam.sql", "14_eksekutif_data.sql", "15_audit_trail.sql", "16_refresh_views.sql",
		},
	}

	for tier, files := range expectedTiers {
		for _, f := range files {
			path := filepath.Join(dir, tier, f)
			info, err := os.Stat(path)
			require.NoError(t, err, "file %s/%s harus ada", tier, f)
			assert.Greater(t, info.Size(), int64(0), "file %s/%s tidak boleh kosong", tier, f)
		}
	}
}

func TestBuildDemoAkun(t *testing.T) {
	akuns := BuildDemoAkun()
	assert.Len(t, akuns, 25, "harus ada 25 akun demo")

	var slugs []string
	for _, a := range akuns {
		assert.NotEmpty(t, a.ID)
		assert.NotEmpty(t, a.Email)
		assert.NotEmpty(t, a.Peran)
		slug := seedkit.SlugPengguna(a.ID)
		slugs = append(slugs, slug)
	}

	assert.Contains(t, slugs, "superadmin")
	assert.Contains(t, slugs, "user_nonaktif")
	assert.Contains(t, slugs, "user_mfa")
}
