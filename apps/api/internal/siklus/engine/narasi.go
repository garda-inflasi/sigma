package engine

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/garda-inflasi/api/internal/siklus/domain"
)

// Kalimat alasan keputusan dalam bahasa Indonesia. Semua fungsi murni; urutan dan bentuk kalimat dikunci
// narasi_test.go karena alasan ini tampil di antarmuka dan jejak audit.

// ringkasanGanti menjelaskan pergantian siklus. pemicu adalah aturan yang memicu siklus terpilih, ditekan aturan siklus
// lain yang ikut memicu tetapi kalah prioritas.
func ringkasanGanti(ke string, s sebab, dari string, pemicu, ditekan []domain.HasilAturan, pulihJam int) string {
	var sb strings.Builder
	switch s {
	case sebabAwal:
		if ke == domain.SiklusNormal {
			return "Siklus awal NORMAL: tidak ada aturan yang memicu."
		}
		fmt.Fprintf(&sb, "Siklus awal %s: %s.", ke, daftar(pemicu))
	case sebabMasuk:
		fmt.Fprintf(&sb, "Berpindah dari NORMAL ke %s: %s.", ke, daftar(pemicu))
	case sebabGeser:
		fmt.Fprintf(&sb, "Berpindah dari %s ke %s: aturan %s berprioritas lebih tinggi memicu (%s).", dari, ke, ke, daftar(pemicu))
	case sebabPulih:
		if ke == domain.SiklusNormal {
			fmt.Fprintf(&sb, "Kembali ke NORMAL: tidak ada aturan %s yang memicu selama %d jam.", dari, pulihJam)
		} else {
			fmt.Fprintf(&sb, "Berpindah dari %s ke %s: tidak ada aturan %s yang memicu selama %d jam, sedangkan aturan %s memicu (%s).",
				dari, ke, dari, pulihJam, ke, daftar(pemicu))
		}
	}
	sb.WriteString(kalimatDitekan(ditekan))
	return sb.String()
}

// ringkasanTetapDipicu menjelaskan siklus yang dipertahankan karena aturannya masih memicu; semua adalah seluruh
// aturan yang memicu (termasuk siklus lain yang tidak diutamakan).
func ringkasanTetapDipicu(c string, memicuC, semua []domain.HasilAturan) string {
	lain := pilih(semua, func(h domain.HasilAturan) bool { return h.Aturan.Siklus != c })
	return fmt.Sprintf("Siklus tetap %s: %s.", c, daftar(memicuC)) + kalimatDitekan(lain)
}

// ringkasanTetapPulih menjelaskan siklus yang dipertahankan selama masa pulih. sisa adalah sisa masa pulih; memicu
// aturan siklus lain yang sedang memicu (menunggu masa pulih habis).
func ringkasanTetapPulih(c string, pulihJam int, sisa time.Duration, memicu []domain.HasilAturan) string {
	s := fmt.Sprintf("Siklus tetap %s: tidak ada aturan %s yang memicu; masa pulih %d jam, sisa sekitar %d jam.",
		c, c, pulihJam, int(math.Ceil(max(sisa, 0).Hours())))
	if len(memicu) > 0 {
		s += fmt.Sprintf(" Aturan %s memicu (%s) dan menunggu masa pulih selesai.", daftarSiklus(memicu), daftar(memicu))
	}
	return s
}

func kalimatDitekan(ditekan []domain.HasilAturan) string {
	if len(ditekan) == 0 {
		return ""
	}
	return fmt.Sprintf(" Aturan %s ikut memicu (%s) tetapi tidak diutamakan karena prioritasnya lebih rendah.",
		daftarSiklus(ditekan), daftar(ditekan))
}

// daftar menulis aturan yang memicu, dipisah titik koma, mis. "3 alert LONJAKAN_HARGA ≥ SIAGA; utilisasi stok 91,2% ≥ 90%".
func daftar(hasil []domain.HasilAturan) string {
	bagian := make([]string, len(hasil))
	for i, h := range hasil {
		bagian[i] = deskripsi(h)
	}
	return strings.Join(bagian, "; ")
}

func deskripsi(h domain.HasilAturan) string {
	a, p := h.Aturan, h.Pengukuran
	switch a.Indikator {
	case domain.IndikatorAlertEws:
		s := fmt.Sprintf("%s alert %s ≥ %s", domain.FormatAngka(p.Nilai), nilaiTeks(a.JenisEws), nilaiTeks(a.SeverityMin))
		if a.JumlahMin != nil && *a.JumlahMin > 1 {
			s += fmt.Sprintf(" (minimal %d)", *a.JumlahMin)
		}
		return s
	case domain.IndikatorUtilisasiStok:
		ambang := 0.0
		if a.Ambang != nil {
			ambang = *a.Ambang
		}
		return fmt.Sprintf("utilisasi stok %s%% ≥ %s%%", domain.FormatAngka(p.Nilai), domain.FormatAngka(ambang))
	}
	return a.Kode
}

// daftarSiklus menulis siklus yang berbeda dari kumpulan aturan, mis. "SURPLUS" atau "TEKANAN dan SURPLUS".
func daftarSiklus(hasil []domain.HasilAturan) string {
	var out []string
	for _, h := range hasil {
		ada := false
		for _, s := range out {
			ada = ada || s == h.Aturan.Siklus
		}
		if !ada {
			out = append(out, h.Aturan.Siklus)
		}
	}
	return strings.Join(out, " dan ")
}

func nilaiTeks(p *string) string {
	if p == nil {
		return "?"
	}
	return *p
}

// pemicuAlasan menyusun baris alasan terstruktur untuk satu aturan.
func pemicuAlasan(h domain.HasilAturan) domain.PemicuAlasan {
	a, p := h.Aturan, h.Pengukuran
	out := domain.PemicuAlasan{
		Aturan: a.Kode, Siklus: a.Siklus, Indikator: a.Indikator, Nilai: p.Nilai, Prioritas: a.Prioritas,
		Rujukan: p.Rujukan, Rincian: p.Rincian,
	}
	switch a.Indikator {
	case domain.IndikatorAlertEws:
		out.Satuan = "alert"
		if a.JumlahMin != nil {
			out.Ambang = float64(*a.JumlahMin)
		}
	case domain.IndikatorUtilisasiStok:
		out.Satuan = "%"
		if a.Ambang != nil {
			out.Ambang = *a.Ambang
		}
	}
	return out
}
