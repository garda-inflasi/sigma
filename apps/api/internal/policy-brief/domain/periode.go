package domain

import (
	"fmt"
	"time"
)

// FormatPeriode adalah format periode di API: tahun-bulan, mis. "2026-10".
const FormatPeriode = "2006-01"

var namaBulan = [...]string{
	"Januari", "Februari", "Maret", "April", "Mei", "Juni",
	"Juli", "Agustus", "September", "Oktober", "November", "Desember",
}

// KodeBrief membentuk kode brief dari periode, mis. "PB-2026-10". Kode itu kode tampilan sekaligus alias
// tautan (/v1/policy-brief/PB-2026-10), UNIQUE dan dijaga CHECK berbentuk PB-YYYY-MM di basis data, sehingga satu
// periode hanya punya satu brief. ID brief sendiri UUID yang diberikan basis data.
func KodeBrief(periode time.Time) string {
	return "PB-" + periode.Format(FormatPeriode)
}

// LabelPeriode membentuk nama periode yang tampil di UI, mis. "Oktober 2026".
func LabelPeriode(periode time.Time) string {
	return fmt.Sprintf("%s %d", namaBulan[periode.Month()-1], periode.Year())
}
