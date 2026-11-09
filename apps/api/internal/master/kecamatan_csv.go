package master

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
	URL          string
	PeriodeMerge string
	Diambil      string
	Jumlah       int
}

const (
	kunciSumber  = "Sumber"
	kunciURL     = "URL"
	kunciPeriode = "periode_merge"
	kunciDiambil = "Diambil"
	kunciJumlah  = "Jumlah baris"

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

// KecamatanAceh membaca kecamatan se-Aceh dari berkas tersemat kecamatan_aceh.csv.
func KecamatanAceh() ([]domain.Kecamatan, MetaKecamatan, error) {
	kecamatanSekali.Do(func() {
		kecamatanMeta, kecamatanData, kecamatanErr = BacaKecamatanCSV(bytes.NewReader(kecamatanCSV), KodeKabupaten())
	})
	if kecamatanErr != nil {
		return nil, MetaKecamatan{}, kecamatanErr
	}
	out := make([]domain.Kecamatan, len(kecamatanData))
	copy(out, kecamatanData)
	return out, kecamatanMeta, nil
}

// ResetCacheKecamatan mengosongkan cache in-memory; dipakai tes.
func ResetCacheKecamatan() {
	kecamatanSekali = sync.Once{}
	kecamatanData = nil
	kecamatanMeta = MetaKecamatan{}
	kecamatanErr = nil
}

// KodeKabupaten mengembalikan himpunan kode kabupaten/kota yang sah untuk validasi kecamatan.
func KodeKabupaten() map[string]bool {
	m := make(map[string]bool, len(kabupatenAceh))
	for _, k := range kabupatenAceh {
		m[k.Kode] = true
	}
	return m
}

// BacaKecamatanCSV mem-parsing CSV kecamatan ber-header komentar #.
func BacaKecamatanCSV(r io.Reader, kabupatenSah map[string]bool) (MetaKecamatan, []domain.Kecamatan, error) {
	var meta MetaKecamatan
	scanner := bufio.NewScanner(r)
	var body bytes.Buffer

	barisKe := 0
	for scanner.Scan() {
		barisKe++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			isi := strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
			k, v, ok := strings.Cut(isi, ":")
			if ok {
				k, v = strings.TrimSpace(k), strings.TrimSpace(v)
				switch k {
				case kunciURL:
					meta.URL = v
				case kunciPeriode:
					meta.PeriodeMerge = v
				case kunciDiambil:
					meta.Diambil = v
				case kunciJumlah:
					if n, err := strconv.Atoi(v); err == nil {
						meta.Jumlah = n
					}
				}
			}
			continue
		}
		if trimmed == "" {
			continue
		}
		body.WriteString(line)
		body.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil {
		return MetaKecamatan{}, nil, fmt.Errorf("pindai berkas CSV: %w", err)
	}

	cr := csv.NewReader(&body)
	header, err := cr.Read()
	if err != nil {
		return MetaKecamatan{}, nil, fmt.Errorf("baca header CSV: %w", err)
	}
	if len(header) < 3 || header[0] != "kode" || header[1] != "nama" || header[2] != "kabupaten_kode" {
		return MetaKecamatan{}, nil, fmt.Errorf("header CSV tidak sesuai: dapat %v, ingin %v", header, KecamatanCSVHeader)
	}
	adaDagri := len(header) >= 4 && header[3] == "kode_dagri"

	var rows []domain.Kecamatan
	for {
		record, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return MetaKecamatan{}, nil, fmt.Errorf("baca baris CSV: %w", err)
		}
		if len(record) < 3 {
			continue
		}
		row := domain.Kecamatan{
			Kode:          strings.TrimSpace(record[0]),
			Nama:          strings.TrimSpace(record[1]),
			KabupatenKode: strings.TrimSpace(record[2]),
		}
		if adaDagri && len(record) >= 4 {
			dagri := strings.TrimSpace(record[3])
			if dagri != "" {
				row.KodeDagri = &dagri
			}
		}
		rows = append(rows, row)
	}

	if err := ValidasiKecamatan(rows, kabupatenSah); err != nil {
		return MetaKecamatan{}, nil, err
	}
	if meta.Jumlah > 0 && len(rows) != meta.Jumlah {
		return MetaKecamatan{}, nil, fmt.Errorf("jumlah baris (%d) tidak cocok dengan kepala berkas (%d)", len(rows), meta.Jumlah)
	}
	return meta, rows, nil
}

// TulisKecamatanCSV menulis metadata komentar dan baris kecamatan ke format CSV kanonik.
func TulisKecamatanCSV(w io.Writer, meta MetaKecamatan, rows []domain.Kecamatan) error {
	sort.Slice(rows, func(i, j int) bool { return rows[i].Kode < rows[j].Kode })

	fmt.Fprintf(w, "# %s\n", catatanSumber)
	if meta.URL != "" {
		fmt.Fprintf(w, "# %s: %s\n", kunciURL, meta.URL)
	}
	if meta.PeriodeMerge != "" {
		fmt.Fprintf(w, "# %s: %s\n", kunciPeriode, meta.PeriodeMerge)
	}
	if meta.Diambil != "" {
		fmt.Fprintf(w, "# %s: %s\n", kunciDiambil, meta.Diambil)
	}
	fmt.Fprintf(w, "# %s: %d\n", kunciJumlah, len(rows))
	fmt.Fprintln(w, "# Dibuat oleh: make db/impor/kecamatan (apps/api/cmd/gardactl master impor-kecamatan). Jangan disunting tangan; lihat SUMBER.md.")

	cw := csv.NewWriter(w)
	if err := cw.Write(KecamatanCSVHeader); err != nil {
		return err
	}
	for _, r := range rows {
		dagri := ""
		if r.KodeDagri != nil {
			dagri = *r.KodeDagri
		}
		rec := []string{r.Kode, r.Nama, r.KabupatenKode, dagri}
		if err := cw.Write(rec); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// ValidasiKecamatan memeriksa konsistensi internal daftar kecamatan.
func ValidasiKecamatan(rows []domain.Kecamatan, kabupatenSah map[string]bool) error {
	seen := make(map[string]bool, len(rows))
	for i, r := range rows {
		if !kodeKecamatanRe.MatchString(r.Kode) {
			return fmt.Errorf("baris %d: kode kecamatan %q tidak valid (wajib 7 digit BPS)", i+1, r.Kode)
		}
		if seen[r.Kode] {
			return fmt.Errorf("baris %d: kode kecamatan %q ganda", i+1, r.Kode)
		}
		seen[r.Kode] = true

		if strings.TrimSpace(r.Nama) == "" {
			return fmt.Errorf("baris %d: nama kecamatan kosong untuk kode %s", i+1, r.Kode)
		}
		if len(kabupatenSah) > 0 && !kabupatenSah[r.KabupatenKode] {
			return fmt.Errorf("baris %d: kabupaten_kode %q tidak dikenal (kode kecamatan %s)", i+1, r.KabupatenKode, r.Kode)
		}
		if !strings.HasPrefix(r.Kode, r.KabupatenKode) {
			return fmt.Errorf("baris %d: 4 digit awal kode %q tidak cocok dengan kabupaten_kode %q", i+1, r.Kode, r.KabupatenKode)
		}
	}
	return nil
}
