package domain

import (
	"strconv"
	"strings"
)

// FormatAngka menulis bilangan gaya Indonesia untuk kalimat alasan: satu desimal dengan koma, tanpa ",0" di
// ujung (3, 85,6, 90).
func FormatAngka(v float64) string {
	s := strings.TrimSuffix(strconv.FormatFloat(v, 'f', 1, 64), ".0")
	return strings.Replace(s, ".", ",", 1)
}
