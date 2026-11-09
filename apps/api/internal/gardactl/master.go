package gardactl

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/garda-inflasi/api/internal/master"
	"github.com/garda-inflasi/api/internal/master/domain"
)

const (
	bawaanBase    = "https://sig.bps.go.id/rest-bridging/getwilayah"
	bawaanPeriode = "2024_1.2022"
	bawaanOut     = "internal/master/kecamatan_aceh.csv"
	userAgent     = "garda-inflasi-impor-kecamatan/1.0"
	maksBadan     = 4 << 20
)

type konfig struct {
	out, base, periode string
	timeout, jeda      time.Duration
	sekarang           func() time.Time
}

type wilayahBPS struct {
	KodeBPS   string `json:"kode_bps"`
	NamaBPS   string `json:"nama_bps"`
	KodeDagri string `json:"kode_dagri"`
	NamaDagri string `json:"nama_dagri"`
}

func runMaster(ctx context.Context, args []string, opts Options) error {
	if len(args) == 0 {
		return errors.New("sub-perintah master wajib: impor-kecamatan")
	}

	switch args[0] {
	case "impor-kecamatan":
		return runImporKecamatan(ctx, args[1:], opts)
	default:
		return fmt.Errorf("sub-perintah master %q tidak dikenal (tersedia: impor-kecamatan)", args[0])
	}
}

func runImporKecamatan(ctx context.Context, args []string, opts Options) error {
	cfg := konfig{sekarang: opts.now}
	fs := flag.NewFlagSet("master impor-kecamatan", flag.ContinueOnError)
	fs.StringVar(&cfg.out, "out", bawaanOut, "berkas CSV keluaran")
	fs.StringVar(&cfg.base, "base", bawaanBase, "URL getwilayah BPS")
	fs.StringVar(&cfg.periode, "periode", bawaanPeriode, "periode_merge BPS")
	fs.DurationVar(&cfg.timeout, "timeout", 30*time.Second, "batas waktu tiap permintaan")
	fs.DurationVar(&cfg.jeda, "jeda", time.Second, "jeda antarpermintaan")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	return impor(ctx, cfg, opts.httpClient(), opts.out())
}

func impor(ctx context.Context, cfg konfig, client *http.Client, out io.Writer) error {
	var semua []domain.Kecamatan
	kabupaten := master.Kabupaten()
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

	meta := master.MetaKecamatan{
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

func susunCSV(meta master.MetaKecamatan, rows []domain.Kecamatan) ([]byte, error) {
	if err := master.ValidasiKecamatan(rows, master.KodeKabupaten()); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := master.TulisKecamatanCSV(&buf, meta, rows); err != nil {
		return nil, fmt.Errorf("susun csv: %w", err)
	}
	if _, _, err := master.BacaKecamatanCSV(bytes.NewReader(buf.Bytes()), master.KodeKabupaten()); err != nil {
		return nil, fmt.Errorf("csv hasil tidak lolos pembacaan ulang: %w", err)
	}
	return buf.Bytes(), nil
}

func polaURL(base, periode string) string {
	return base + "?level=kecamatan&parent=<kode kabupaten>&periode_merge=" + periode
}

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
	if err := master.ValidasiKecamatan(rows, map[string]bool{kabupaten: true}); err != nil {
		return nil, err
	}
	return rows, nil
}

func rapikanSpasi(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

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
