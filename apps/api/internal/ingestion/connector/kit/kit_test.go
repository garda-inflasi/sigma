package kit

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAngka(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		ok   bool
	}{
		{"14.350", 14350, true},
		{"14,350", 14350, true},
		{"Rp 14.350", 14350, true},
		{"14.350,50", 14350.5, true},
		{"14,350.50", 14350.5, true},
		{"1.234.567", 1234567, true},
		{"14350", 14350, true},
		{"14350.5", 14350.5, true},
		{"14,5", 14.5, true},
		{" 52000 ", 52000, true},
		{"-", 0, false},
		{"", 0, false},
		{"n/a", 0, false},
		{"1.2.3x", 0, false},
	}
	for _, c := range cases {
		got, ok := Angka(c.in)
		assert.Equal(t, c.ok, ok, c.in)
		if c.ok {
			assert.InDelta(t, c.want, got, 1e-9, c.in)
		}
	}
}

func TestTanggal(t *testing.T) {
	want := time.Date(2025, 10, 3, 0, 0, 0, 0, time.UTC)
	for _, in := range []string{"03/10/2025", "3/10/2025", "2025-10-03", "03-10-2025", "2025-10-03T00:00:00Z", "03/10/2025 00:00"} {
		got, ok := Tanggal(in)
		require.True(t, ok, in)
		assert.True(t, want.Equal(got), in)
	}
	for _, in := range []string{"", "no", "name", "32/13/2025", "level"} {
		_, ok := Tanggal(in)
		assert.False(t, ok, in)
	}
}

func TestTeksAndNilaiSel(t *testing.T) {
	var v struct {
		A Teks `json:"a"`
		B Teks `json:"b"`
		C Teks `json:"c"`
	}
	require.NoError(t, json.Unmarshal([]byte(`{"a":"x","b":12.5,"c":null}`), &v))
	assert.Equal(t, Teks("x"), v.A)
	assert.Equal(t, Teks("12.5"), v.B)
	assert.Equal(t, Teks(""), v.C)
	assert.Error(t, json.Unmarshal([]byte(`{"a":{"x":1}}`), &v))

	assert.Equal(t, "7", NilaiSel(json.RawMessage(`7`)))
	assert.Equal(t, "", NilaiSel(json.RawMessage(`[1]`)))
}
