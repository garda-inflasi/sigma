// Package kit berisi bantuan parsing yang dipakai bersama konektor: angka gaya
// Indonesia, tanggal, dan nilai JSON yang bisa berupa teks atau angka.
package kit

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Angka membaca bilangan dari teks gaya Indonesia atau Inggris: "14.350", "14,350",
// "Rp 14.350,50", "14350.5". Pemisah terakhir adalah desimal kecuali bila ia
// satu-satunya jenis pemisah dan diikuti tepat tiga digit atau muncul berulang
// (pemisah ribuan). ok salah untuk teks kosong, "-", atau bukan bilangan.
func Angka(s string) (nilai float64, ok bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimSpace(strings.TrimPrefix(s, "Rp"))
	s = strings.ReplaceAll(s, " ", "")
	if s == "" || s == "-" {
		return 0, false
	}

	titik, koma := strings.LastIndex(s, "."), strings.LastIndex(s, ",")
	var desimal string // pemisah desimal; kosong bila semua pemisah adalah ribuan
	switch {
	case titik >= 0 && koma >= 0:
		desimal = "."
		if koma > titik {
			desimal = ","
		}
	case titik >= 0 || koma >= 0:
		sep := "."
		if koma >= 0 {
			sep = ","
		}
		if strings.Count(s, sep) == 1 && len(s)-strings.LastIndex(s, sep)-1 != 3 {
			desimal = sep
		}
	}

	if desimal == "" {
		s = strings.NewReplacer(".", "", ",", "").Replace(s)
	} else {
		ribuan := "."
		if desimal == "." {
			ribuan = ","
		}
		s = strings.Replace(strings.ReplaceAll(s, ribuan, ""), desimal, ".", 1)
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// tataTanggal adalah format tanggal yang dikenali Tanggal.
var tataTanggal = []string{"02/01/2006", "2/1/2006", "2006-01-02", "02-01-2006"}

// Tanggal membaca tanggal (tanpa jam) dari teks; ok salah bila bukan tanggal.
// Bagian waktu setelah spasi atau "T" diabaikan. Hasilnya tengah malam UTC pada
// tanggal kalender itu, siap disimpan ke kolom DATE.
func Tanggal(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, " T"); i > 0 {
		s = s[:i]
	}
	for _, tata := range tataTanggal {
		if t, err := time.Parse(tata, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// Teks adalah nilai JSON yang boleh berupa teks, angka, atau null ("1", 1, null).
type Teks string

func (t *Teks) UnmarshalJSON(b []byte) error {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch x := v.(type) {
	case nil:
		*t = ""
	case string:
		*t = Teks(x)
	case float64:
		*t = Teks(strconv.FormatFloat(x, 'f', -1, 64))
	default:
		return fmt.Errorf("nilai %s bukan teks atau angka", string(b))
	}
	return nil
}

// NilaiSel mengubah nilai sel JSON (teks, angka, atau null) menjadi teks;
// nilai bertipe lain menjadi teks kosong.
func NilaiSel(raw json.RawMessage) string {
	var t Teks
	if err := json.Unmarshal(raw, &t); err != nil {
		return ""
	}
	return string(t)
}
