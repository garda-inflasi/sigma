// Package cron mem-parse ekspresi cron lima kolom (menit jam tanggal bulan hari)
// dan memeriksa apakah sebuah waktu cocok. Dukungan: "*", angka, daftar "a,b",
// rentang "a-b", dan langkah "*/n", "a-b/n". Hari 0 dan 7 sama-sama Minggu.
// Bila tanggal dan hari sama-sama dibatasi, salah satu cocok sudah cukup (semantik cron baku).
package cron

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Schedule adalah jadwal hasil parse.
type Schedule struct {
	menit, jam, tanggal, bulan, hari field
}

// field menyimpan nilai yang cocok sebagai bitmask; bintang menandai kolom tak dibatasi.
type field struct {
	bits    uint64
	bintang bool
}

type batas struct{ min, max int }

var batasKolom = [5]batas{{0, 59}, {0, 23}, {1, 31}, {1, 12}, {0, 7}}

// Parse mem-parse ekspresi cron lima kolom.
func Parse(expr string) (Schedule, error) {
	kolom := strings.Fields(expr)
	if len(kolom) != 5 {
		return Schedule{}, fmt.Errorf("cron %q: harus lima kolom, ada %d", expr, len(kolom))
	}
	var f [5]field
	for i, k := range kolom {
		parsed, err := parseField(k, batasKolom[i])
		if err != nil {
			return Schedule{}, fmt.Errorf("cron %q kolom %d: %w", expr, i+1, err)
		}
		f[i] = parsed
	}
	// Minggu boleh ditulis 7: samakan dengan 0.
	if f[4].bits&(1<<7) != 0 {
		f[4].bits |= 1
	}
	return Schedule{menit: f[0], jam: f[1], tanggal: f[2], bulan: f[3], hari: f[4]}, nil
}

func parseField(s string, b batas) (field, error) {
	var res field
	for _, term := range strings.Split(s, ",") {
		langkah := 1
		if rentang, step, ada := strings.Cut(term, "/"); ada {
			n, err := strconv.Atoi(step)
			if err != nil || n < 1 {
				return field{}, fmt.Errorf("langkah %q tidak valid", step)
			}
			langkah, term = n, rentang
		}

		lo, hi := b.min, b.max
		switch {
		case term == "*":
			if langkah == 1 {
				res.bintang = true
			}
		case strings.Contains(term, "-"):
			a, z, _ := strings.Cut(term, "-")
			var err error
			if lo, err = strconv.Atoi(a); err != nil {
				return field{}, fmt.Errorf("rentang %q tidak valid", term)
			}
			if hi, err = strconv.Atoi(z); err != nil {
				return field{}, fmt.Errorf("rentang %q tidak valid", term)
			}
		default:
			n, err := strconv.Atoi(term)
			if err != nil {
				return field{}, fmt.Errorf("nilai %q tidak valid", term)
			}
			lo, hi = n, n
			// "5/10" berarti dari 5 sampai batas atas, tiap 10.
			if langkah > 1 {
				hi = b.max
			}
		}
		if lo < b.min || hi > b.max || lo > hi {
			return field{}, fmt.Errorf("%q di luar batas %d-%d", term, b.min, b.max)
		}
		for v := lo; v <= hi; v += langkah {
			res.bits |= 1 << uint(v)
		}
	}
	return res, nil
}

func (f field) cocok(v int) bool { return f.bits&(1<<uint(v)) != 0 }

// Cocok memeriksa apakah t (dibaca dalam zona waktunya sendiri) jatuh pada jadwal,
// dengan ketelitian menit.
func (s Schedule) Cocok(t time.Time) bool {
	if !s.menit.cocok(t.Minute()) || !s.jam.cocok(t.Hour()) || !s.bulan.cocok(int(t.Month())) {
		return false
	}
	tanggalCocok := s.tanggal.cocok(t.Day())
	hariCocok := s.hari.cocok(int(t.Weekday()))
	switch {
	case s.tanggal.bintang && s.hari.bintang:
		return true
	case s.tanggal.bintang:
		return hariCocok
	case s.hari.bintang:
		return tanggalCocok
	default:
		return tanggalCocok || hariCocok
	}
}

// Berikutnya mengembalikan waktu cocok pertama setelah t (ketelitian menit),
// atau zero bila tidak ada dalam delapan hari ke depan.
func (s Schedule) Berikutnya(t time.Time) time.Time {
	t = t.Truncate(time.Minute).Add(time.Minute)
	batas := t.Add(8 * 24 * time.Hour)
	for ; t.Before(batas); t = t.Add(time.Minute) {
		if s.Cocok(t) {
			return t
		}
	}
	return time.Time{}
}

// Selang adalah jarak antara dua kemunculan jadwal setelah t; nol bila tak ada.
func (s Schedule) Selang(t time.Time) time.Duration {
	pertama := s.Berikutnya(t)
	if pertama.IsZero() {
		return 0
	}
	kedua := s.Berikutnya(pertama)
	if kedua.IsZero() {
		return 0
	}
	return kedua.Sub(pertama)
}
