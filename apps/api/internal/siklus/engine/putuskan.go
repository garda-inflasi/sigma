package engine

import (
	"sort"
	"time"

	"github.com/garda-inflasi/api/internal/siklus/domain"
)

// Kondisi adalah masukan keputusan: waktu, siklus aktif wilayah, dan hasil ukur tiap aturan.
type Kondisi struct {
	Sekarang time.Time
	// Aktif adalah siklus aktif wilayah; nil bila belum ada.
	Aktif *domain.SiklusEkonomi
	// Hasil memuat hasil ukur tiap aturan. Aturan nonaktif dan aturan yang bukan pemicu (siklus NORMAL) diabaikan.
	Hasil []domain.HasilAturan
}

// sebab menjelaskan mengapa siklus diganti; dipakai untuk menyusun kalimat alasan.
type sebab int

const (
	// sebabAwal: belum ada siklus aktif sama sekali.
	sebabAwal sebab = iota
	// sebabMasuk: dari NORMAL ke siklus yang dipicu.
	sebabMasuk
	// sebabGeser: aturan siklus lain yang lebih diutamakan memicu dan merebut siklus aktif.
	sebabGeser
	// sebabPulih: masa pulih siklus aktif habis.
	sebabPulih
)

// Putuskan menentukan siklus yang harus aktif. Fungsi ini murni: hanya bergantung pada Kondisi, tanpa basis data
// dan tanpa jam sendiri, sehingga seluruh tabel keputusannya bisa diuji langsung.
//
// Aturan keputusan:
//
//  1. Urutan aturan: prioritas (angka kecil lebih dulu), lalu TEKANAN sebelum SURPLUS, lalu ID. Aturan yang memicu
//     paling depan dalam urutan itu menentukan siklus terkuat ("pemenang").
//  2. Belum ada siklus aktif atau siklus NORMAL: masuk ke siklus pemenang bila ada yang memicu. Masuk selalu
//     langsung, tanpa penundaan: alert EWS sudah punya peredaman sendiri, dan menunda berarti warna ambient
//     terlambat mengikuti kondisi yang sudah resmi berstatus siaga.
//  3. Siklus TEKANAN atau SURPLUS yang masih dipicu salah satu aturannya dipertahankan. Bila pemenang berasal dari
//     siklus lain (prioritasnya lebih tinggi), ia merebut siklus aktif saat itu juga.
//  4. Siklus yang tidak lagi dipicu masuk masa pulih (histeresis): penanda PulihSejak dimulai pada evaluasi pertama
//     tanpa pemicu dan dihapus bila pemicu muncul lagi. Siklus baru dilepas setelah masa pulih (pulih_jam terbesar
//     di antara aturan aktif siklus itu) penuh, ke siklus pemenang bila ada pemicu lain, selain itu ke NORMAL. Selama
//     masa pulih, pemicu siklus lain yang prioritasnya sama atau lebih rendah menunggu; yang lebih tinggi
//     merebut segera. Dengan begitu satu kali TEKANAN tidak bisa berganti ke SURPLUS dan kembali dalam waktu singkat.
//  5. Siklus aktif yang tidak punya aturan aktif sama sekali tidak bisa dibenarkan mesin: masa pulihnya nol.
func Putuskan(k Kondisi) domain.Keputusan {
	hasil := aturanBerlaku(k.Hasil)
	var memicu []domain.HasilAturan
	for _, h := range hasil {
		if h.Memicu {
			memicu = append(memicu, h)
		}
	}

	aktif := k.Aktif
	if aktif == nil || aktif.Siklus == domain.SiklusNormal {
		switch {
		case len(memicu) > 0:
			alasan := sebabMasuk
			dari := domain.SiklusNormal
			if aktif == nil {
				alasan, dari = sebabAwal, ""
			}
			return ganti(memicu[0].Aturan.Siklus, alasan, dari, memicu, 0)
		case aktif == nil:
			return ganti(domain.SiklusNormal, sebabAwal, "", nil, 0)
		}
		return tetap(domain.SiklusNormal, nil, "Siklus tetap NORMAL: tidak ada aturan yang memicu.")
	}

	c := aktif.Siklus
	aturanC := pilih(hasil, func(h domain.HasilAturan) bool { return h.Aturan.Siklus == c })
	memicuC := pilih(memicu, func(h domain.HasilAturan) bool { return h.Aturan.Siklus == c })

	if len(memicuC) > 0 {
		if pemenang := memicu[0]; pemenang.Aturan.Siklus != c {
			return ganti(pemenang.Aturan.Siklus, sebabGeser, c, memicu, 0)
		}
		return tetap(c, nil, ringkasanTetapDipicu(c, memicuC, memicu))
	}

	// Tidak ada aturan siklus ini yang memicu.
	if len(memicu) > 0 && (len(aturanC) == 0 || lebihUtama(memicu[0].Aturan, aturanC[0].Aturan)) {
		return ganti(memicu[0].Aturan.Siklus, sebabGeser, c, memicu, 0)
	}
	pulihJam := maksPulihJam(aturanC)
	pulih := time.Duration(pulihJam) * time.Hour
	sejak := k.Sekarang
	if aktif.PulihSejak != nil && !aktif.PulihSejak.After(k.Sekarang) {
		sejak = *aktif.PulihSejak
	}
	if k.Sekarang.Sub(sejak) >= pulih {
		if len(memicu) > 0 {
			return ganti(memicu[0].Aturan.Siklus, sebabPulih, c, memicu, pulihJam)
		}
		return ganti(domain.SiklusNormal, sebabPulih, c, nil, pulihJam)
	}
	return tetap(c, &sejak, ringkasanTetapPulih(c, pulihJam, pulih-k.Sekarang.Sub(sejak), memicu))
}

// aturanBerlaku menyaring aturan aktif yang bisa memicu siklus (bukan NORMAL) dan mengurutkannya menurut prioritas.
// Masukan tidak diubah.
func aturanBerlaku(masuk []domain.HasilAturan) []domain.HasilAturan {
	out := make([]domain.HasilAturan, 0, len(masuk))
	for _, h := range masuk {
		if !h.Aturan.Aktif || (h.Aturan.Siklus != domain.SiklusTekanan && h.Aturan.Siklus != domain.SiklusSurplus) {
			continue
		}
		out = append(out, h)
	}
	sort.SliceStable(out, func(i, j int) bool { return lebihUtama(out[i].Aturan, out[j].Aturan) })
	return out
}

// lebihUtama: a diutamakan atas b menurut prioritas, lalu siklus (TEKANAN dulu), lalu ID.
func lebihUtama(a, b domain.AturanSiklus) bool {
	if a.Prioritas != b.Prioritas {
		return a.Prioritas < b.Prioritas
	}
	if ra, rb := domain.RankSiklus(a.Siklus), domain.RankSiklus(b.Siklus); ra != rb {
		return ra < rb
	}
	return a.ID < b.ID
}

func pilih(hasil []domain.HasilAturan, cocok func(domain.HasilAturan) bool) []domain.HasilAturan {
	var out []domain.HasilAturan
	for _, h := range hasil {
		if cocok(h) {
			out = append(out, h)
		}
	}
	return out
}

// maksPulihJam adalah masa pulih siklus: pulih_jam terbesar di antara aturannya (0 bila tidak ada aturan).
func maksPulihJam(aturan []domain.HasilAturan) int {
	var maks int
	for _, h := range aturan {
		maks = max(maks, h.Aturan.PulihJam)
	}
	return maks
}

// ganti menyusun keputusan mengganti siklus ke "ke". Pemicu yang bersiklus sama dengan "ke" menjadi alasan utama;
// pemicu siklus lain tercatat sebagai ditekan.
func ganti(ke string, s sebab, dari string, memicu []domain.HasilAturan, pulihJam int) domain.Keputusan {
	pemicu, ditekan := []domain.PemicuAlasan{}, []domain.PemicuAlasan{}
	var hPemicu, hDitekan []domain.HasilAturan
	for _, h := range memicu {
		if h.Aturan.Siklus == ke {
			pemicu = append(pemicu, pemicuAlasan(h))
			hPemicu = append(hPemicu, h)
		} else {
			ditekan = append(ditekan, pemicuAlasan(h))
			hDitekan = append(hDitekan, h)
		}
	}
	return domain.Keputusan{
		Siklus: ke,
		Ganti:  true,
		Alasan: domain.Alasan{
			Ringkasan: ringkasanGanti(ke, s, dari, hPemicu, hDitekan, pulihJam),
			Pemicu:    pemicu,
			Ditekan:   ditekan,
		},
	}
}

// tetap menyusun keputusan mempertahankan siklus aktif dengan penanda masa pulih sejak (nil = tanpa penanda).
func tetap(siklus string, sejak *time.Time, ringkasan string) domain.Keputusan {
	return domain.Keputusan{Siklus: siklus, PulihSejak: sejak, Alasan: domain.Alasan{
		Ringkasan: ringkasan, Pemicu: []domain.PemicuAlasan{}, Ditekan: []domain.PemicuAlasan{},
	}}
}
