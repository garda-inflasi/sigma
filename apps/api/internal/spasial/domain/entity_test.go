package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStatusDariDelta(t *testing.T) {
	cases := []struct {
		delta float64
		want  string
	}{
		{21.4, StatusDefisit},
		{10.1, StatusDefisit},
		{10, StatusWaspada}, // batas atas tidak termasuk
		{5.1, StatusWaspada},
		{5, StatusNormal},
		{0, StatusNormal},
		{-6, StatusNormal},
		{-6.1, StatusSurplus},
		{-18, StatusSurplus},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, StatusDariDelta(tc.delta), "delta %v", tc.delta)
	}
}
