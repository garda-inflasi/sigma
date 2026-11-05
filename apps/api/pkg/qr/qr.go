// Package qr membuat kode QR sebagai SVG, supaya halaman setup MFA menampilkannya tanpa pustaka di browser.
package qr

import (
	"fmt"
	"strings"

	"github.com/boombuler/barcode/qr"
)

// quietZone adalah tepi kosong di sekeliling kode, dalam modul (syarat minimum standar QR adalah 4).
const quietZone = 4

// SVG menyandikan teks menjadi dokumen SVG hitam di atas putih dengan koreksi galat tingkat M.
func SVG(text string) (string, error) {
	code, err := qr.Encode(text, qr.M, qr.Auto)
	if err != nil {
		return "", fmt.Errorf("sandikan QR: %w", err)
	}

	bounds := code.Bounds()
	ukuran := bounds.Dx() + 2*quietZone
	var path strings.Builder
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if r, _, _, _ := code.At(x, y).RGBA(); r == 0 {
				fmt.Fprintf(&path, "M%d %dh1v1h-1z", x-bounds.Min.X+quietZone, y-bounds.Min.Y+quietZone)
			}
		}
	}
	return fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %[1]d %[1]d" shape-rendering="crispEdges"><rect width="%[1]d" height="%[1]d" fill="#fff"/><path d="%[2]s" fill="#000"/></svg>`,
		ukuran, path.String()), nil
}
