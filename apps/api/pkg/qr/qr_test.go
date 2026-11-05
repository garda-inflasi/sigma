package qr

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSVG_ProducesSquareDocumentWithQuietZone(t *testing.T) {
	svg, err := SVG("otpauth://totp/Garda%20Inflasi:a@b.id?secret=JBSWY3DPEHPK3PXP&issuer=Garda+Inflasi")

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(svg, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 `))
	m := regexp.MustCompile(`viewBox="0 0 (\d+) (\d+)"`).FindStringSubmatch(svg)
	require.Len(t, m, 3)
	assert.Equal(t, m[1], m[2], "persegi")
	assert.Contains(t, svg, `fill="#000"`)
	assert.Contains(t, svg, "M4 4", "modul pertama berada di dalam zona tenang 4 modul (sudut kiri atas pola penanda)")
}

func TestSVG_IsDeterministicAndDiffersByContent(t *testing.T) {
	a1, err := SVG("abc")
	require.NoError(t, err)
	a2, _ := SVG("abc")
	b, _ := SVG("abd")

	assert.Equal(t, a1, a2)
	assert.NotEqual(t, a1, b)
}
