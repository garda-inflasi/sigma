package seeder

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	_ "embed"

	"github.com/garda-inflasi/api/internal/master/domain"
)

// kecamatan_aceh.csv dihasilkan skrip gardactl master impor-kecamatan (make db/impor/kecamatan) dari API bridging wilayah BPS;
// jangan disunting tangan. Sumber, lisensi, dan cara menyegarkannya ada di SUMBER.md.
//
//go:embed kecamatan_aceh.csv
var kecamatanCSV []byte

// KecamatanCSVHeader adalah baris kolom berkas CSV kecamatan.
var KecamatanCSVHeader = []string{"kode", "nama", "kabupaten_kode", "kode_dagri"}

// MetaKecamatan adalah catatan sumber pada baris komentar (#) di awal CSV.
type MetaKecamatan struct {
	// URL adalah pola URL sumber, mis. https://sig.bps.go.id/rest-bridging/getwilayah?level=kecamatan&parent=<kode kabupaten>&periode_merge=<periode>.
	URL          string
	PeriodeMerge string
	// Diambil adalah tanggal pengambilan (YYYY-MM-DD).
	Diambil string
	// Jumlah adalah jumlah baris data yang dinyatakan di kepala berkas.
	Jumlah int
}

const (
	kunciSumber  = "Sumber"
	kunciURL     = "URL"
	kunciPeriode = "periode_merge"
	kunciDiambil = "Diambil"
	kunciJumlah  = "Jumlah baris"

	// catatanSumber dicantumkan di kepala CSV: BPS mengizinkan pemakaian data dengan mencantumkan sumber.
	catatanSumber = "Sumber: Badan Pusat Statistik (BPS), API bridging wilayah sig.bps.go.id. " +
		"Data dipakai dengan mencantumkan sumber (syarat dan ketentuan BPS: https://www.bps.go.id/en/term-of-use)."
)

var (
	kodeKecamatanRe = regexp.MustCompile(`^[0-9]{7}$`)

	kecamatanSekali sync.Once
	kecamatanData   []domain.Kecamatan
	kecamatanMeta   MetaKecamatan
	kecamatanErr    error
)

// Kecamatan mengembalikan kecamatan se-Aceh dari CSV yang tertanam (terurut menurut kode), sudah divalidasi
// terhadap 23 kabupaten/kota seed.
func Kecamatan() ([]domain.Kecamatan, error) {
	kecamatanSekali.Do(func() {
		kecamatanMeta, kecamatanData, kecamatanErr = BacaKecamatanCSV(bytes.NewReader(kecamatanCSV), KodeKabupaten())
	})
	if kecamatanErr != nil {
		return nil, kecamatanErr
	}
	return append([]domain.Kecamatan(nil), kecamatanData...), nil
}

// MetaKecamatanTertanam mengembalikan catatan sumber CSV yang tertanam.
func MetaKecamatanTertanam() (MetaKecamatan, error) {
	if _, err := Kecamatan(); err != nil {
		return MetaKecamatan{}, err
	}
	return kecamatanMeta, nil
}

// KodeKabupaten mengembalikan himpunan kode kabupaten/kota seed (23 se-Aceh).
func KodeKabupaten() map[string]bool {
	out := make(map[string]bool, len(kabupatenAceh))
	for _, k := range kabupatenAceh {
		out[k.Kode] = true
	}
	return out
}

// ValidasiKecamatan memastikan baris kecamatan layak dimasukkan: kode tujuh digit, awalannya kode kabupaten
// yang dikenal, tanpa kode ganda, nama tidak kosong, dan kode Dagri (bila ada) muat di kolomnya.
func ValidasiKecamatan(rows []domain.Kecamatan, kabupaten map[string]bool) error {
	var problems []string
	seen := make(map[string]bool, len(rows))
	for i, r := range rows {
		label := fmt.Sprintf("baris %d (%q)", i+1, r.Kode)
		switch {
		case !kodeKecamatanRe.MatchString(r.Kode):
			problems = append(problems, label+": kode harus tujuh digit")
		case r.KabupatenKode == "" || r.Kode[:4] != r.KabupatenKode:
			problems = append(problems, fmt.Sprintf("%s: empat digit awal harus sama dengan kabupaten_kode %q", label, r.KabupatenKode))
		}
		if !kabupaten[r.KabupatenKode] {
			problems = append(problems, fmt.Sprintf("%s: kabupaten_kode %q bukan kabupaten/kota seed", label, r.KabupatenKode))
		}
		if seen[r.Kode] {
			problems = append(problems, label+": kode ganda")
		}
		seen[r.Kode] = true
		if strings.TrimSpace(r.Nama) == "" {
			problems = append(problems, label+": nama kosong")
		}
		if len(r.Nama) > 255 {
			problems = append(problems, label+": nama lebih dari 255 karakter")
		}
		if r.KodeDagri != nil && (strings.TrimSpace(*r.KodeDagri) == "" || len(*r.KodeDagri) > 10) {
			problems = append(problems, label+": kode_dagri harus kosong atau 1-10 karakter")
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("kecamatan tidak valid:\n  %s", strings.Join(problems, "\n  "))
	}
	return nil
}

// TulisKecamatanCSV menulis CSV deterministik: baris komentar sumber, kepala kolom, lalu baris terurut menurut
// kode. meta.Jumlah diisi dari jumlah baris.
func TulisKecamatanCSV(w io.Writer, meta MetaKecamatan, rows []domain.Kecamatan) error {
	sorted := append([]domain.Kecamatan(nil), rows...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Kode < sorted[j].Kode })

	bw := bufio.NewWriter(w)
	for _, line := range []string{
		catatanSumber,
		kunciURL + ": " + meta.URL,
		kunciPeriode + ": " + meta.PeriodeMerge,
		kunciDiambil + ": " + meta.Diambil,
		kunciJumlah + ": " + strconv.Itoa(len(sorted)),
		"Dibuat oleh: make db/impor/kecamatan (gardactl master impor-kecamatan). Jangan disunting tangan; lihat SUMBER.md.",
	} {
		if _, err := bw.WriteString("# " + line + "\n"); err != nil {
			return err
		}
	}
	cw := csv.NewWriter(bw)
	if err := cw.Write(KecamatanCSVHeader); err != nil {
		return err
	}
	for _, r := range sorted {
		dagri := ""
		if r.KodeDagri != nil {
			dagri = *r.KodeDagri
		}
		if err := cw.Write([]string{r.Kode, r.Nama, r.KabupatenKode, dagri}); err != nil {
			return err
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return err
	}
	return bw.Flush()
}

// BacaKecamatanCSV membaca CSV kecamatan: catatan sumber dari baris komentar, baris data tervalidasi terhadap
// kabupaten, dan jumlah baris dicocokkan dengan "Jumlah baris" di kepala.
func BacaKecamatanCSV(r io.Reader, kabupaten map[string]bool) (MetaKecamatan, []domain.Kecamatan, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return MetaKecamatan{}, nil, fmt.Errorf("baca csv kecamatan: %w", err)
	}

	meta, err := bacaMetaKecamatan(raw)
	if err != nil {
		return MetaKecamatan{}, nil, err
	}

	cr := csv.NewReader(bytes.NewReader(raw))
	cr.Comment = '#'
	cr.FieldsPerRecord = len(KecamatanCSVHeader)
	records, err := cr.ReadAll()
	if err != nil {
		return MetaKecamatan{}, nil, fmt.Errorf("csv kecamatan: %w", err)
	}
	if len(records) == 0 || strings.Join(records[0], ",") != strings.Join(KecamatanCSVHeader, ",") {
		return MetaKecamatan{}, nil, fmt.Errorf("csv kecamatan: kepala kolom harus %s", strings.Join(KecamatanCSVHeader, ","))
	}

	rows := make([]domain.Kecamatan, 0, len(records)-1)
	for _, rec := range records[1:] {
		k := domain.Kecamatan{Kode: rec[0], Nama: rec[1], KabupatenKode: rec[2]}
		if rec[3] != "" {
			dagri := rec[3]
			k.KodeDagri = &dagri
		}
		rows = append(rows, k)
	}
	if len(rows) != meta.Jumlah {
		return MetaKecamatan{}, nil, fmt.Errorf("csv kecamatan: kepala menyebut %d baris, isinya %d", meta.Jumlah, len(rows))
	}
	if err := ValidasiKecamatan(rows, kabupaten); err != nil {
		return MetaKecamatan{}, nil, err
	}
	for i := 1; i < len(rows); i++ {
		if rows[i-1].Kode >= rows[i].Kode {
			return MetaKecamatan{}, nil, fmt.Errorf("csv kecamatan: baris harus terurut menurut kode (%s sebelum %s)", rows[i-1].Kode, rows[i].Kode)
		}
	}
	return meta, rows, nil
}

// bacaMetaKecamatan membaca baris "# Kunci: nilai" di kepala berkas.
func bacaMetaKecamatan(raw []byte) (MetaKecamatan, error) {
	var meta MetaKecamatan
	var adaSumber, adaJumlah bool
	sc := bufio.NewScanner(bytes.NewReader(raw))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "#") {
			continue
		}
		kunci, nilai, ok := strings.Cut(strings.TrimSpace(strings.TrimPrefix(line, "#")), ":")
		if !ok {
			continue
		}
		nilai = strings.TrimSpace(nilai)
		switch strings.TrimSpace(kunci) {
		case kunciSumber:
			adaSumber = true
		case kunciURL:
			// nilai URL memuat ':' lagi; Cut hanya memotong di yang pertama.
			meta.URL = nilai
		case kunciPeriode:
			meta.PeriodeMerge = nilai
		case kunciDiambil:
			meta.Diambil = nilai
		case kunciJumlah:
			n, err := strconv.Atoi(nilai)
			if err != nil {
				return MetaKecamatan{}, fmt.Errorf("csv kecamatan: %q bukan angka", kunciJumlah)
			}
			meta.Jumlah, adaJumlah = n, true
		}
	}
	if err := sc.Err(); err != nil {
		return MetaKecamatan{}, err
	}
	switch {
	case !adaSumber:
		return MetaKecamatan{}, errors.New("csv kecamatan: catatan sumber (# Sumber: ...) wajib ada di kepala")
	case meta.URL == "" || meta.PeriodeMerge == "" || meta.Diambil == "":
		return MetaKecamatan{}, errors.New("csv kecamatan: kepala wajib memuat URL, periode_merge, dan Diambil")
	case !adaJumlah:
		return MetaKecamatan{}, errors.New("csv kecamatan: kepala wajib memuat Jumlah baris")
	}
	return meta, nil
}
