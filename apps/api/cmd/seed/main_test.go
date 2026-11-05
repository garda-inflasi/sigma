package main

import (
	"bytes"
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
		err := run(args, env(nil), &bytes.Buffer{})
		require.Error(t, err, "%v", args)
		assert.Contains(t, err.Error(), "pemakaian: seed reference|admin|demo|all")
	}
}

func TestRun_DemoIsRefusedInProductionBeforeConnecting(t *testing.T) {
	// DATABASE_URL sengaja kosong: penolakan harus terjadi sebelum koneksi dicoba.
	for _, arg := range []string{"demo", "all"} {
		err := run([]string{arg}, env(map[string]string{"APP_ENV": "production"}), &bytes.Buffer{})
		require.ErrorIs(t, err, seedkit.ErrDemoInProduction, arg)
	}
}

func TestRun_NeedsDatabaseURL(t *testing.T) {
	err := run([]string{"reference"}, env(nil), &bytes.Buffer{})
	assert.ErrorContains(t, err, "DATABASE_URL")
}

// Menjaga seluruh pendaftaran (dari semua berkas register_*.go) tetap utuh:
// dependensi lengkap, tanpa putaran, dan urutan tiap modul terpenuhi.
func TestRegisteredSeedersFormAValidPlan(t *testing.T) {
	plan, err := seedkit.Plan(seedkit.AllTiers...)
	require.NoError(t, err)

	position := map[string]int{}
	for i, s := range plan {
		position[s.Name] = i
	}
	for _, s := range plan {
		for _, dep := range s.DependsOn {
			assert.Less(t, position[dep], position[s.Name], "%s harus setelah %s", s.Name, dep)
		}
	}
	assert.Contains(t, position, "coldstorage.reference")
	assert.Contains(t, position, "coldstorage.demo")
}
