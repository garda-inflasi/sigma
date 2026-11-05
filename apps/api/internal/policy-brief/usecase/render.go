package usecase

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/garda-inflasi/api/internal/policy-brief/domain"
)

// ErrNarasiTidakValid berarti narasi dari Generator melanggar aturan anti-halusinasi atau bentuk.
var ErrNarasiTidakValid = errors.New("narasi tidak valid")

const (
	maksButirPerBagian   = 8
	maksPanjangButir     = 700
	maksPanjangRingkasan = 1500
)

// placeholderFakta mengenali {{ID}} (spasi di dalam kurung diperbolehkan).
var placeholderFakta = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_]+)\s*\}\}`)

// judulBagian menetapkan urutan dan judul bagian brief.
var judulBagian = []struct{ kode, judul string }{
	{domain.BagianRingkasan, "Ringkasan"},
	{domain.BagianTemuan, "Temuan utama"},
	{domain.BagianRisiko, "Risiko"},
	{domain.BagianRekomendasiPemda, "Rekomendasi untuk Pemerintah Daerah"},
	{domain.BagianRekomendasiTPID, "Rekomendasi untuk TPID"},
}

// Render mengubah narasi ber-placeholder menjadi konten final, dengan aturan anti-halusinasi:
//   - setiap placeholder {{ID}} harus menunjuk fakta di Konteks dan diganti nilainya;
//   - di luar placeholder tidak boleh ada angka sama sekali, sehingga semua angka di narasi
//     pasti berasal dari data sumber;
//   - lampiran memuat sitasi sumber tiap fakta yang dipakai.
//
// catatan (boleh kosong) disimpan di konten, mis. alasan LLM digantikan templat.
func Render(n *domain.Narasi, k domain.Konteks, catatan string) (*domain.Konten, error) {
	pakai := &pemakaianFakta{}
	isi := func(teks string, maks int) (string, error) {
		hasil, err := isiTeks(strings.TrimSpace(teks), k, pakai)
		if err != nil {
			return "", err
		}
		if hasil == "" {
			return "", fmt.Errorf("%w: butir kosong", ErrNarasiTidakValid)
		}
		if utf8.RuneCountInString(hasil) > maks {
			return "", fmt.Errorf("%w: butir melebihi %d karakter", ErrNarasiTidakValid, maks)
		}
		return hasil, nil
	}

	ringkasan, err := isi(n.Ringkasan, maksPanjangRingkasan)
	if err != nil {
		return nil, fmt.Errorf("ringkasan: %w", err)
	}
	sumberButir := map[string][]string{
		domain.BagianTemuan:           n.Temuan,
		domain.BagianRisiko:           n.Risiko,
		domain.BagianRekomendasiPemda: n.RekomendasiPemda,
		domain.BagianRekomendasiTPID:  n.RekomendasiTPID,
	}

	konten := &domain.Konten{Label: domain.LabelPenyusun(n.Meta.Penyusun), Penyusun: n.Meta.Penyusun, Catatan: catatan}
	for _, b := range judulBagian {
		bagian := domain.BagianKonten{Kode: b.kode, Judul: b.judul}
		if b.kode == domain.BagianRingkasan {
			bagian.Butir = []string{ringkasan}
			konten.Bagian = append(konten.Bagian, bagian)
			continue
		}
		butir := sumberButir[b.kode]
		if len(butir) == 0 || len(butir) > maksButirPerBagian {
			return nil, fmt.Errorf("%w: bagian %s harus berisi 1 sampai %d butir", ErrNarasiTidakValid, b.kode, maksButirPerBagian)
		}
		for _, teks := range butir {
			final, err := isi(teks, maksPanjangButir)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", b.kode, err)
			}
			bagian.Butir = append(bagian.Butir, final)
		}
		konten.Bagian = append(konten.Bagian, bagian)
	}
	konten.Lampiran = pakai.lampiran()
	return konten, nil
}

// pemakaianFakta mengumpulkan fakta yang dipakai narasi, berurut kemunculan pertama.
type pemakaianFakta struct {
	urut []domain.Fakta
	ada  map[string]bool
}

func (p *pemakaianFakta) catat(f domain.Fakta) {
	if p.ada == nil {
		p.ada = map[string]bool{}
	}
	if !p.ada[f.ID] {
		p.ada[f.ID] = true
		p.urut = append(p.urut, f)
	}
}

// lampiran mengembalikan fakta yang dipakai dan punya sumber (fakta penanda seperti periode tidak disitasi).
func (p *pemakaianFakta) lampiran() []domain.Fakta {
	out := []domain.Fakta{}
	for _, f := range p.urut {
		if f.Sumber != "" {
			out = append(out, f)
		}
	}
	return out
}

// isiTeks mengganti tiap placeholder dengan nilai faktanya dan memeriksa bahwa teks di luar
// placeholder tidak memuat angka atau kurung placeholder yang rusak.
func isiTeks(teks string, k domain.Konteks, pakai *pemakaianFakta) (string, error) {
	var b strings.Builder
	akhir := 0
	for _, m := range placeholderFakta.FindAllStringSubmatchIndex(teks, -1) {
		polos := teks[akhir:m[0]]
		if err := periksaPolos(polos); err != nil {
			return "", err
		}
		id := teks[m[2]:m[3]]
		fakta, ada := k.Cari(id)
		if !ada {
			return "", fmt.Errorf("%w: placeholder {{%s}} tidak ada di daftar fakta", ErrNarasiTidakValid, id)
		}
		b.WriteString(polos)
		b.WriteString(fakta.Nilai)
		pakai.catat(fakta)
		akhir = m[1]
	}
	sisa := teks[akhir:]
	if err := periksaPolos(sisa); err != nil {
		return "", err
	}
	b.WriteString(sisa)
	return b.String(), nil
}

func periksaPolos(s string) error {
	for _, r := range s {
		if unicode.IsDigit(r) {
			return fmt.Errorf("%w: angka ditulis di luar placeholder (%q)", ErrNarasiTidakValid, cuplik(s))
		}
	}
	if strings.Contains(s, "{{") || strings.Contains(s, "}}") {
		return fmt.Errorf("%w: placeholder rusak (%q)", ErrNarasiTidakValid, cuplik(s))
	}
	return nil
}

func cuplik(s string) string {
	const maks = 60
	if utf8.RuneCountInString(s) <= maks {
		return s
	}
	return string([]rune(s)[:maks]) + "..."
}
