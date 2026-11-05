package cron

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var wib = time.FixedZone("WIB", 7*3600)

func at(y int, m time.Month, d, h, min int) time.Time { return time.Date(y, m, d, h, min, 0, 0, wib) }

func TestParse_RejectsMalformedExpressions(t *testing.T) {
	for _, expr := range []string{"", "* * * *", "* * * * * *", "60 * * * *", "* 24 * * *", "*/0 * * * *", "a * * * *", "5-2 * * * *", "* * 0 * *", "* * * 13 *", "*/x * * * *"} {
		_, err := Parse(expr)
		assert.Error(t, err, expr)
	}
}

func TestSchedule_Cocok(t *testing.T) {
	cases := []struct {
		expr string
		t    time.Time
		want bool
	}{
		{"0 6 * * *", at(2026, 11, 5, 6, 0), true},
		{"0 6 * * *", at(2026, 11, 5, 6, 1), false},
		{"0 */3 * * *", at(2026, 11, 5, 9, 0), true},
		{"0 */3 * * *", at(2026, 11, 5, 10, 0), false},
		{"*/5 * * * *", at(2026, 11, 5, 10, 35), true},
		{"*/5 * * * *", at(2026, 11, 5, 10, 36), false},
		{"0 * * * *", at(2026, 11, 5, 23, 0), true},
		// 2026-11-02 Senin, 2026-11-08 Minggu.
		{"0 7 * * 1", at(2026, 11, 2, 7, 0), true},
		{"0 7 * * 1", at(2026, 11, 3, 7, 0), false},
		{"0 7 * * 7", at(2026, 11, 8, 7, 0), true},
		{"0 7 * * 0", at(2026, 11, 8, 7, 0), true},
		{"0 7 * * 1-5", at(2026, 11, 6, 7, 0), true},
		{"0 7 * * 1-5", at(2026, 11, 7, 7, 0), false},
		{"15,45 8 * * *", at(2026, 11, 5, 8, 45), true},
		{"30 8 1 * *", at(2026, 12, 1, 8, 30), true},
		{"30 8 1 * *", at(2026, 12, 2, 8, 30), false},
		// Tanggal dan hari sama-sama dibatasi: salah satu cukup.
		{"0 0 1 * 1", at(2026, 11, 2, 0, 0), true},
		{"0 0 1 * 1", at(2026, 12, 1, 0, 0), true},
		{"0 0 1 * 1", at(2026, 12, 2, 0, 0), false},
		{"0 0 * 12 *", at(2026, 11, 5, 0, 0), false},
	}
	for _, c := range cases {
		s, err := Parse(c.expr)
		require.NoError(t, err, c.expr)
		assert.Equal(t, c.want, s.Cocok(c.t), "%s @ %s", c.expr, c.t.Format(time.RFC3339))
	}
}

func TestSchedule_BerikutnyaDanSelang(t *testing.T) {
	s, err := Parse("0 */3 * * *")
	require.NoError(t, err)
	assert.Equal(t, at(2026, 11, 5, 12, 0), s.Berikutnya(at(2026, 11, 5, 10, 17)))
	assert.Equal(t, at(2026, 11, 5, 12, 0), s.Berikutnya(at(2026, 11, 5, 9, 0).Add(2*time.Hour+59*time.Minute)))
	assert.Equal(t, 3*time.Hour, s.Selang(at(2026, 11, 5, 10, 17)))

	harian, err := Parse("0 6 * * *")
	require.NoError(t, err)
	assert.Equal(t, 24*time.Hour, harian.Selang(at(2026, 11, 5, 7, 0)))

	mingguan, err := Parse("0 7 * * 1")
	require.NoError(t, err)
	assert.Equal(t, 7*24*time.Hour, mingguan.Selang(at(2026, 11, 5, 7, 0)))
}

func TestSchedule_TanpaKemunculan(t *testing.T) {
	s, err := Parse("0 0 31 2 *")
	require.NoError(t, err)
	assert.True(t, s.Berikutnya(at(2026, 11, 5, 0, 0)).IsZero())
	assert.Zero(t, s.Selang(at(2026, 11, 5, 0, 0)))
}
