package main

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func env(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

func TestRun_RejectsBadArguments(t *testing.T) {
	for _, args := range [][]string{{"-bulan", "0"}, {"-bulan", "37"}, {"-bulan", "abc"}, {"-tak-dikenal"}} {
		err := run(args, env(map[string]string{"DATABASE_URL": "postgres://x"}), &bytes.Buffer{}, time.Now())
		assert.Error(t, err, "%v", args)
	}
}

func TestRun_NeedsDatabaseURL(t *testing.T) {
	err := run(nil, env(nil), &bytes.Buffer{}, time.Now())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "DATABASE_URL")
}
