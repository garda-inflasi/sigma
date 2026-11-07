package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/master/seeder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHurufJudul(t *testing.T) {
	assert.Equal(t, "Kuta Alam", hurufJudul("KUTA ALAM"))
	assert.Equal(t, "Grong-Grong", hurufJudul("GRONG-GRONG"))
	assert.Equal(t, "T. Jambo Aye", hurufJudul("T. JAMBO AYE"))
	assert.Equal(t, "Meuraxa", hurufJudul("meuraxa"))
}

func TestUbahKecamatan(t *testing.T) {
	t.Run("nama_dagri dipakai bila ada, selain itu nama_bps berhuruf judul", func(t *testing.T) {
		rows, err := ubahKecamatan("1171", []wilayahBPS{
			{KodeBPS: "1171010", NamaBPS: "MEURAXA", KodeDagri: "117103", NamaDagri: "Meuraxa"},
			{KodeBPS: "1171020", NamaBPS: "BAITURRAHMAN", KodeDagri: "", NamaDagri: "  "},
			{KodeBPS: "1171030", NamaBPS: "KUTA   ALAM", NamaDagri: "Kuta  Alam"},
		})
		require.NoError(t, err)
		require.Len(t, rows, 3)
		assert.Equal(t, "Meuraxa", rows[0].Nama)
		require.NotNil(t, rows[0].KodeDagri)
		assert.Equal(t, "117103", *rows[0].KodeDagri)
		assert.Equal(t, "Baiturrahman", rows[1].Nama)
		assert.Nil(t, rows[1].KodeDagri)
		assert.Equal(t, "Kuta Alam", rows[2].Nama, "spasi ganda dirapikan")
		assert.Equal(t, "1171", rows[2].KabupatenKode)
	})

	t.Run("jawaban kosong ditolak", func(t *testing.T) {
		_, err := ubahKecamatan("1171", nil)
		assert.Error(t, err)
	})

	t.Run("kode di luar kabupaten, ganda, atau salah bentuk ditolak", func(t *testing.T) {
		for name, items := range map[string][]wilayahBPS{
			"awalan lain": {{KodeBPS: "1101010", NamaBPS: "X"}},
			"ganda":       {{KodeBPS: "1171010", NamaBPS: "A"}, {KodeBPS: "1171010", NamaBPS: "B"}},
			"enam digit":  {{KodeBPS: "117101", NamaBPS: "X"}},
			"tanpa nama":  {{KodeBPS: "1171010"}},
		} {
			_, err := ubahKecamatan("1171", items)
			assert.Error(t, err, name)
		}
	})
}

// serverBPS menjawab seperti getwilayah: satu kecamatan per kabupaten. gagalDulu menolak sekian permintaan pertama.
func serverBPS(t *testing.T, gagalDulu int32) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var panggilan atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := panggilan.Add(1)
		assert.Equal(t, userAgent, r.UserAgent())
		assert.Equal(t, "kecamatan", r.URL.Query().Get("level"))
		assert.Equal(t, "2024_1.2022", r.URL.Query().Get("periode_merge"))
		if n <= gagalDulu {
			http.Error(w, "gangguan", http.StatusBadGateway)
			return
		}
		parent := r.URL.Query().Get("parent")
		_ = json.NewEncoder(w).Encode([]wilayahBPS{{KodeBPS: parent + "010", NamaBPS: "KECAMATAN " + parent, KodeDagri: "900001"}})
	}))
	t.Cleanup(srv.Close)
	return srv, &panggilan
}

func konfigUji(t *testing.T, base string) konfig {
	t.Helper()
	return konfig{
		out: filepath.Join(t.TempDir(), "kecamatan.csv"), base: base, periode: "2024_1.2022",
		timeout: 5 * time.Second, jeda: 0,
		sekarang: func() time.Time { return time.Date(2026, 11, 7, 23, 0, 0, 0, time.UTC) },
	}
}

func TestAmbilKecamatan_UlangSekali(t *testing.T) {
	t.Run("gagal sekali lalu berhasil", func(t *testing.T) {
		srv, panggilan := serverBPS(t, 1)
		items, err := ambilKecamatan(t.Context(), srv.Client(), konfigUji(t, srv.URL), "1171")
		require.NoError(t, err)
		assert.Len(t, items, 1)
		assert.EqualValues(t, 2, panggilan.Load())
	})

	t.Run("gagal dua kali menyerah", func(t *testing.T) {
		srv, panggilan := serverBPS(t, 5)
		_, err := ambilKecamatan(t.Context(), srv.Client(), konfigUji(t, srv.URL), "1171")
		require.Error(t, err)
		assert.EqualValues(t, 2, panggilan.Load(), "tepat dua percobaan")
	})

	t.Run("bukan JSON daftar dianggap gagal", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("<html>")) }))
		defer srv.Close()
		_, err := ambilSekali(t.Context(), srv.Client(), konfigUji(t, srv.URL), "1171")
		assert.Error(t, err)
	})
}

func TestImpor_MenulisCSVYangDiterimaSeeder(t *testing.T) {
	srv, panggilan := serverBPS(t, 0)
	cfg := konfigUji(t, srv.URL)

	var out bytes.Buffer
	require.NoError(t, impor(t.Context(), cfg, srv.Client(), &out))
	assert.EqualValues(t, 23, panggilan.Load(), "satu permintaan per kabupaten/kota, berurutan")

	raw, err := os.ReadFile(cfg.out)
	require.NoError(t, err)
	meta, rows, err := seeder.BacaKecamatanCSV(bytes.NewReader(raw), seeder.KodeKabupaten())
	require.NoError(t, err)
	assert.Len(t, rows, 23)
	assert.Equal(t, 23, meta.Jumlah)
	assert.Equal(t, "2024_1.2022", meta.PeriodeMerge)
	assert.Equal(t, "2026-11-07", meta.Diambil)
	assert.Equal(t, "Kecamatan 1101", rows[0].Nama)

	// Jalan ulang menghasilkan berkas yang sama persis.
	cfg2 := cfg
	cfg2.out = filepath.Join(t.TempDir(), "lagi.csv")
	require.NoError(t, impor(t.Context(), cfg2, srv.Client(), &bytes.Buffer{}))
	lagi, err := os.ReadFile(cfg2.out)
	require.NoError(t, err)
	assert.Equal(t, string(raw), string(lagi))
}

func TestImpor_GagalTidakMerusakBerkasLama(t *testing.T) {
	srv, _ := serverBPS(t, 1000) // selalu gagal
	cfg := konfigUji(t, srv.URL)
	require.NoError(t, os.WriteFile(cfg.out, []byte("lama"), 0o644))

	err := impor(t.Context(), cfg, srv.Client(), &bytes.Buffer{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "1101")

	isi, readErr := os.ReadFile(cfg.out)
	require.NoError(t, readErr)
	assert.Equal(t, "lama", string(isi))
}

func TestImpor_BerhentiBilaKonteksDibatalkan(t *testing.T) {
	srv, _ := serverBPS(t, 0)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := impor(ctx, konfigUji(t, srv.URL), srv.Client(), &bytes.Buffer{})
	assert.Error(t, err)
}
