package dto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEksporPenggunaRequest_Filter(t *testing.T) {
	kasus := map[string]struct {
		req    EksporPenggunaRequest
		search string
		ids    []string
	}{
		"kosong":                     {EksporPenggunaRequest{}, "", nil},
		"pencarian dirapikan":        {EksporPenggunaRequest{Search: "  budi  "}, "budi", nil},
		"id dipisah koma":            {EksporPenggunaRequest{IDs: "a,b,c"}, "", []string{"a", "b", "c"}},
		"spasi, kosong, dan ganda":   {EksporPenggunaRequest{IDs: " a , ,b,a,,c, "}, "", []string{"a", "b", "c"}},
		"hanya pemisah":              {EksporPenggunaRequest{IDs: ", ,,"}, "", nil},
		"pencarian dan id bersamaan": {EksporPenggunaRequest{Search: "x", IDs: "u1"}, "x", []string{"u1"}},
	}
	for nama, k := range kasus {
		t.Run(nama, func(t *testing.T) {
			f := k.req.Filter()
			assert.Equal(t, k.search, f.Search)
			assert.Equal(t, k.ids, f.IDs)
		})
	}
}
