package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKodeKomoditas_Scan(t *testing.T) {
	cases := map[string]struct {
		src  any
		want KodeKomoditas
	}{
		"teks":   {"{CBM-KRT,BWM}", KodeKomoditas{"CBM-KRT", "BWM"}},
		"bytes":  {[]byte("{TLR}"), KodeKomoditas{"TLR"}},
		"kosong": {"{}", KodeKomoditas{}},
		"quoted": {`{"CBM-KRT","BWM"}`, KodeKomoditas{"CBM-KRT", "BWM"}},
		"null":   {nil, nil},
		"spasi":  {"  {A,B}  ", KodeKomoditas{"A", "B"}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			var got KodeKomoditas
			require.NoError(t, got.Scan(c.src))
			assert.Equal(t, c.want, got)
		})
	}
}

func TestKodeKomoditas_ScanRejectsMalformed(t *testing.T) {
	var got KodeKomoditas
	assert.Error(t, got.Scan("CBM-KRT,BWM"))
	assert.Error(t, got.Scan(42))
}

func TestKodeKomoditas_ValueRoundTrip(t *testing.T) {
	value, err := KodeKomoditas{"CBM-KRT", "BWM"}.Value()
	require.NoError(t, err)
	assert.Equal(t, "{CBM-KRT,BWM}", value)

	var back KodeKomoditas
	require.NoError(t, back.Scan(value))
	assert.Equal(t, KodeKomoditas{"CBM-KRT", "BWM"}, back)
}

func TestKodeKomoditas_ValueRejectsUnsafeCodes(t *testing.T) {
	for _, kode := range []string{"", "A,B", "A}", `A"B`, "A B", `A\B`} {
		_, err := KodeKomoditas{kode}.Value()
		assert.Error(t, err, "kode %q", kode)
	}
}
