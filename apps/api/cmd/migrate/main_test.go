package main

import (
	"bytes"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/platform/pgtest"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var pg *pgtest.Server

func TestMain(m *testing.M) {
	os.Exit(pgtest.Main(m, &pg))
}

func env(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

func TestRun_RejectsBadInvocation(t *testing.T) {
	assert.ErrorContains(t, run(nil, env(nil), &bytes.Buffer{}), "pemakaian")
	assert.ErrorContains(t, run([]string{"sideways"}, env(nil), &bytes.Buffer{}), "tidak dikenal")
	assert.ErrorContains(t, run([]string{"up"}, env(nil), &bytes.Buffer{}), "DATABASE_URL")
}

func TestRun_UpAppliesEverythingAndReportsVersion(t *testing.T) {
	pg.NewDatabase(t, "cmd_migrate_up")
	vars := map[string]string{"DATABASE_URL": pg.DSN("cmd_migrate_up"), "MIGRATIONS_DIR": pgtest.MigrationsDir()}

	var out bytes.Buffer
	require.NoError(t, run([]string{"up"}, env(vars), &out))
	assert.Regexp(t, `versi skema: [1-9]\d*\n`, out.String())

	out.Reset()
	require.NoError(t, run([]string{"up"}, env(vars), &out), "up kedua tidak boleh gagal")
	require.NoError(t, run([]string{"down"}, env(vars), &out))
	require.NoError(t, run([]string{"version"}, env(vars), &out))
}

func TestRun_BrokenMigrationFailsHard(t *testing.T) {
	pg.NewDatabase(t, "cmd_migrate_broken")
	dir := t.TempDir()
	broken := "-- +goose Up\nCREATE TABLE ok_dulu (id int);\nSELECT * FROM tabel_yang_tidak_ada;\n-- +goose Down\nDROP TABLE ok_dulu;\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "000001_rusak.sql"), []byte(broken), 0o600))
	vars := map[string]string{"DATABASE_URL": pg.DSN("cmd_migrate_broken"), "MIGRATIONS_DIR": dir}

	err := run([]string{"up"}, env(vars), &bytes.Buffer{})

	require.Error(t, err, "migrasi rusak harus membuat perintah gagal agar deploy berhenti")
	assert.ErrorContains(t, err, "tabel_yang_tidak_ada")
}

func TestRun_MissingMigrationsDirFails(t *testing.T) {
	pg.NewDatabase(t, "cmd_migrate_nodir")
	vars := map[string]string{"DATABASE_URL": pg.DSN("cmd_migrate_nodir"), "MIGRATIONS_DIR": "/tidak/ada"}

	assert.ErrorContains(t, run([]string{"up"}, env(vars), &bytes.Buffer{}), "direktori migrasi")
}

func TestWaitForDatabase(t *testing.T) {
	pg.NewDatabase(t, "cmd_migrate_wait")

	up, err := sql.Open("pgx", pg.DSN("cmd_migrate_wait"))
	require.NoError(t, err)
	defer up.Close()
	assert.NoError(t, waitForDatabase(t.Context(), up, 5*time.Second, 50*time.Millisecond))

	down, err := sql.Open("pgx", "postgres://garda:x@127.0.0.1:1/none?sslmode=disable")
	require.NoError(t, err)
	defer down.Close()
	assert.ErrorContains(t, waitForDatabase(t.Context(), down, 300*time.Millisecond, 50*time.Millisecond), "tidak dapat dihubungi")
}
