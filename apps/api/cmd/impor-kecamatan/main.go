// Perintah impor-kecamatan mengambil kecamatan 23 kabupaten/kota Aceh dari API bridging wilayah BPS dan menulis
// internal/master/seeder/kecamatan_aceh.csv, yang ditanam seeder master.reference. Jalankan lewat
//
//	make db/impor/kecamatan
//
// atau `go run ./cmd/impor-kecamatan [-out berkas.csv] [-periode 2024_1.2022]` dari apps/api. Berkas CSV dihasilkan,
// bukan ditulis tangan; sumber, lisensi, dan cara menyegarkannya: internal/master/seeder/SUMBER.md.
//
// Permintaan dibuat berurutan dengan jeda, batas waktu, dan satu kali ulang, supaya tidak membebani server BPS.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/garda-inflasi/api/internal/master/domain"
	"github.com/garda-inflasi/api/internal/master/seeder"
)

const (
	bawaanBase    = "https://sig.bps.go.id/rest-bridging/getwilayah"
	bawaanPeriode = "2024_1.2022"
	bawaanOut     = "internal/master/seeder/kecamatan_aceh.csv"
	userAgent     = "garda-inflasi-impor-kecamatan/1.0"
	// maksBadan membatasi ukuran jawaban; jawaban sebuah kabupaten hanya beberapa KB.
	maksBadan = 4 << 20
)

// konfig adalah pengaturan satu kali jalan.
type konfig struct {
	out, base, periode string
	timeout, jeda      time.Duration
	// sekarang memberi tanggal pengambilan (bisa diganti tes).
	sekarang func() time.Time
}

// wilayahBPS adalah satu elemen jawaban getwilayah.
type wilayahBPS struct {
	KodeBPS   string `json:"kode_bps"`
	NamaBPS   string `json:"nama_bps"`
	KodeDagri string `json:"kode_dagri"`
	NamaDagri string `json:"nama_dagri"`
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		log.Fatalf("impor-kecamatan: %v", err)
	}
}

func run(ctx context.Context, args []string, out io.Writer) error {
	cfg := konfig{sekarang: time.Now}
	fs := flag.NewFlagSet("impor-kecamatan", flag.ContinueOnError)
	fs.StringVar(&cfg.out, "out", bawaanOut, "berkas CSV keluaran")
	fs.StringVar(&cfg.base, "base", bawaanBase, "URL getwilayah BPS")
	fs.StringVar(&cfg.periode, "periode", bawaanPeriode, "periode_merge BPS")
	fs.DurationVar(&cfg.timeout, "timeout", 30*time.Second, "batas waktu tiap permintaan")
	fs.DurationVar(&cfg.jeda, "jeda", time.Second, "jeda antarpermintaan")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return impor(ctx, cfg, &http.Client{}, out)
}

func impor(ctx context.Context, cfg konfig, client *http.Client, out io.Writer) error {
	var semua []domain.Kecamatan
	kabupaten := seeder.Kabupaten()
	for i, kab := range kabupaten {
		if i > 0 {
			if err := tidur(ctx, cfg.jeda); err != nil {
				return err
			}
		}
		items, err := ambilKecamatan(ctx, client, cfg, kab.Kode)
		if err != nil {
			return fmt.Errorf("kabupaten %s (%s): %w", kab.Kode, kab.Nama, err)
		}
		rows, err := ubahKecamatan(kab.Kode, items)
		if err != nil {
			return fmt.Errorf("kabupaten %s (%s): %w", kab.Kode, kab.Nama, err)
		}
		fmt.Fprintf(out, "%s %-28s %3d kecamatan\n", kab.Kode, kab.Nama, len(rows))
		semua = append(semua, rows...)
	}

	meta := seeder.MetaKecamatan{
		URL:          polaURL(cfg.base, cfg.periode),
		PeriodeMerge: cfg.periode,
		Diambil:      cfg.sekarang().UTC().Format("2006-01-02"),
	}
	isi, err := susunCSV(meta, semua)
	if err != nil {
		return err
	}
	if err := tulisAtomik(cfg.out, isi); err != nil {
		return err
	}
	fmt.Fprintf(out, "ditulis %s: %d kecamatan\n", cfg.out, len(semua))
	return nil
}

// susunCSV menghasilkan isi berkas lalu membacanya kembali dengan pembaca seeder, sehingga berkas yang tidak
// akan diterima seeder tidak pernah ditulis.
func susunCSV(meta seeder.MetaKecamatan, rows []domain.Kecamatan) ([]byte, error) {
	if err := seeder.ValidasiKecamatan(rows, seeder.KodeKabupaten()); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := seeder.TulisKecamatanCSV(&buf, meta, rows); err != nil {
		return nil, fmt.Errorf("susun csv: %w", err)
	}
	if _, _, err := seeder.BacaKecamatanCSV(bytes.NewReader(buf.Bytes()), seeder.KodeKabupaten()); err != nil {
		return nil, fmt.Errorf("csv hasil tidak lolos pembacaan ulang: %w", err)
	}
	return buf.Bytes(), nil
}

func polaURL(base, periode string) string {
	return base + "?level=kecamatan&parent=<kode kabupaten>&periode_merge=" + periode
}

// ambilKecamatan meminta kecamatan satu kabupaten; satu kali ulang bila gagal.
func ambilKecamatan(ctx context.Context, client *http.Client, cfg konfig, parent string) ([]wilayahBPS, error) {
	items, err := ambilSekali(ctx, client, cfg, parent)
	if err == nil || ctx.Err() != nil {
		return items, err
	}
	if err := tidur(ctx, max(cfg.jeda, time.Second)); err != nil {
		return nil, err
	}
	items, err2 := ambilSekali(ctx, client, cfg, parent)
	if err2 != nil {
		return nil, fmt.Errorf("gagal dua kali (pertama: %v): %w", err, err2)
	}
	return items, nil
}

func ambilSekali(ctx context.Context, client *http.Client, cfg konfig, parent string) ([]wilayahBPS, error) {
	u, err := url.Parse(cfg.base)
	if err != nil {
		return nil, fmt.Errorf("url dasar: %w", err)
	}
	q := u.Query()
	q.Set("level", "kecamatan")
	q.Set("parent", parent)
	q.Set("periode_merge", cfg.periode)
	u.RawQuery = q.Encode()

	ctx, cancel := context.WithTimeout(ctx, cfg.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", res.StatusCode)
	}
	var items []wilayahBPS
	if err := json.NewDecoder(io.LimitReader(res.Body, maksBadan)).Decode(&items); err != nil {
		return nil, fmt.Errorf("jawaban bukan JSON daftar wilayah: %w", err)
	}
	return items, nil
}

// ubahKecamatan memetakan jawaban BPS satu kabupaten ke baris kecamatan. Nama dari nama_dagri bila ada, selain itu
// nama_bps berhuruf kapital diubah ke huruf judul. Kabupaten tanpa kecamatan dianggap kesalahan sumber.
func ubahKecamatan(kabupaten string, items []wilayahBPS) ([]domain.Kecamatan, error) {
	if len(items) == 0 {
		return nil, errors.New("BPS tidak mengembalikan kecamatan")
	}
	rows := make([]domain.Kecamatan, 0, len(items))
	for _, it := range items {
		nama := rapikanSpasi(it.NamaDagri)
		if nama == "" {
			nama = hurufJudul(rapikanSpasi(it.NamaBPS))
		}
		row := domain.Kecamatan{Kode: strings.TrimSpace(it.KodeBPS), Nama: nama, KabupatenKode: kabupaten}
		if dagri := strings.TrimSpace(it.KodeDagri); dagri != "" {
			row.KodeDagri = &dagri
		}
		rows = append(rows, row)
	}
	if err := seeder.ValidasiKecamatan(rows, map[string]bool{kabupaten: true}); err != nil {
		return nil, err
	}
	return rows, nil
}

func rapikanSpasi(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// hurufJudul mengubah "KUTA ALAM" menjadi "Kuta Alam": huruf pertama tiap kata (dipisah spasi, '-', '/', '.',
// atau '(') kapital, sisanya kecil.
func hurufJudul(s string) string {
	awal := true
	var b strings.Builder
	for _, r := range s {
		if awal {
			b.WriteRune(unicode.ToUpper(r))
		} else {
			b.WriteRune(unicode.ToLower(r))
		}
		awal = strings.ContainsRune(" -/.(", r)
	}
	return b.String()
}

func tidur(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// tulisAtomik menulis ke berkas sementara lalu menggantinya, supaya kegagalan di tengah tidak merusak CSV lama.
func tulisAtomik(path string, isi []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".kecamatan-*.csv")
	if err != nil {
		return fmt.Errorf("berkas sementara: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(isi); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
