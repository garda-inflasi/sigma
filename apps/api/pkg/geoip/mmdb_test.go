package geoip

import (
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/maxmind/mmdbwriter"
	"github.com/maxmind/mmdbwriter/mmdbtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// nama membuat peta nama berbahasa MMDB: nama("en", "Jakarta", "id", "Jakarta Pusat").
func nama(pasangan ...string) mmdbtype.Map {
	m := mmdbtype.Map{}
	for i := 0; i+1 < len(pasangan); i += 2 {
		m[mmdbtype.String(pasangan[i])] = mmdbtype.String(pasangan[i+1])
	}
	return m
}

func tempat(pasangan ...string) mmdbtype.Map {
	return mmdbtype.Map{"names": nama(pasangan...)}
}

// rekamanUji adalah satu jaringan dalam basis data MMDB buatan tes.
type rekamanUji struct {
	cidr string
	data mmdbtype.Map
}

// tulisMMDB membuat berkas MMDB kecil di direktori sementara dengan tipe basis data jenis, dan mengembalikan path-nya.
// Jaringan dokumentasi (TEST-NET, 2001:db8::/32) dipakai sebagai "alamat publik" tes, sehingga tidak ada data
// lokasi nyata yang terlibat.
func tulisMMDB(t *testing.T, jenis string, rekaman []rekamanUji) string {
	t.Helper()
	tree, err := mmdbwriter.New(mmdbwriter.Options{
		DatabaseType:            jenis,
		RecordSize:              24,
		IPVersion:               6,
		IncludeReservedNetworks: true,
		Languages:               []string{"en", "id"},
		Description:             map[string]string{"en": "basis data uji"},
	})
	require.NoError(t, err)
	for _, r := range rekaman {
		_, cidr, err := net.ParseCIDR(r.cidr)
		require.NoError(t, err, r.cidr)
		require.NoError(t, tree.Insert(cidr, r.data), r.cidr)
	}
	path := filepath.Join(t.TempDir(), "uji.mmdb")
	f, err := os.Create(path)
	require.NoError(t, err)
	_, err = tree.WriteTo(f)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return path
}

func rekamanKota() []rekamanUji {
	return []rekamanUji{
		// Nama Indonesia tersedia: dipakai lebih dulu daripada Inggris.
		{"203.0.113.0/25", mmdbtype.Map{
			"city":         tempat("en", "Banda Aceh", "id", "Kota Banda Aceh"),
			"subdivisions": mmdbtype.Slice{tempat("en", "Aceh", "id", "Provinsi Aceh")},
			"country":      tempat("en", "Indonesia", "id", "Indonesia"),
		}},
		// Hanya nama Inggris: cadangan.
		{"203.0.113.128/25", mmdbtype.Map{
			"city":         tempat("en", "Medan", "de", "Medan (de)"),
			"subdivisions": mmdbtype.Slice{tempat("en", "North Sumatra")},
			"country":      tempat("en", "Indonesia"),
		}},
		// Kota sama dengan negara: tanpa duplikat.
		{"198.51.100.0/24", mmdbtype.Map{
			"city":         tempat("en", "Singapore"),
			"subdivisions": mmdbtype.Slice{},
			"country":      tempat("en", "Singapore"),
		}},
		// Hanya negara.
		{"2001:db8::/32", mmdbtype.Map{
			"country": tempat("en", "Japan"),
		}},
		// Dua subdivisi: yang pertama (paling umum) dipakai.
		{"192.0.2.0/25", mmdbtype.Map{
			"city":         tempat("en", "Cambridge"),
			"subdivisions": mmdbtype.Slice{tempat("en", "England"), tempat("en", "Cambridgeshire")},
			"country":      tempat("en", "United Kingdom"),
		}},
		// Subdivisi pertama tanpa nama yang dikenal: lanjut ke subdivisi berikutnya.
		{"192.0.2.128/25", mmdbtype.Map{
			"city":         tempat("en", "Lyon"),
			"subdivisions": mmdbtype.Slice{tempat("zh-CN", "罗讷"), tempat("en", "Rhone")},
			"country":      tempat("en", "France"),
		}},
		// Ada rekaman tetapi tanpa nama dalam bahasa yang dipakai: tidak diketahui.
		{"100.100.0.0/16", mmdbtype.Map{
			"country": tempat("ja", "日本"),
		}},
		// Spasi dan duplikat berbeda huruf besar-kecil dirapikan.
		{"2001:db8:ffff::/48", mmdbtype.Map{
			"city":         tempat("en", "  Paris "),
			"subdivisions": mmdbtype.Slice{tempat("en", "PARIS")},
			"country":      tempat("en", "France"),
		}},
	}
}

func bukaUji(t *testing.T, jenis string, rekaman []rekamanUji) *MMDB {
	t.Helper()
	m, err := BukaMMDB(tulisMMDB(t, jenis, rekaman))
	require.NoError(t, err)
	t.Cleanup(func() { _ = m.Tutup() })
	return m
}

func TestMMDB_Lokasi(t *testing.T) {
	m := bukaUji(t, "GeoLite2-City", rekamanKota())
	kasus := []struct {
		nama, ip, want string
	}{
		{"nama Indonesia didahulukan", "203.0.113.5", "Kota Banda Aceh, Provinsi Aceh, Indonesia"},
		{"cadangan nama Inggris", "203.0.113.200", "Medan, North Sumatra, Indonesia"},
		{"kota sama dengan negara", "198.51.100.77", "Singapore"},
		{"hanya negara IPv6", "2001:db8::1234", "Japan"},
		{"subdivisi pertama dipakai", "192.0.2.10", "Cambridge, England, United Kingdom"},
		{"subdivisi tanpa nama dilewati", "192.0.2.200", "Lyon, Rhone, France"},
		{"tanpa nama dalam bahasa yang dipakai", "100.100.1.1", ""},
		{"spasi dan duplikat huruf besar-kecil", "2001:db8:ffff::5", "Paris, France"},
		{"IPv4 dipetakan IPv6 dicari sebagai IPv4", "::ffff:203.0.113.5", "Kota Banda Aceh, Provinsi Aceh, Indonesia"},
		{"IPv4 dipetakan IPv6 bentuk heksa", "::ffff:cb00:7105", "Kota Banda Aceh, Provinsi Aceh, Indonesia"},
		{"spasi di tepi", " 203.0.113.5 ", "Kota Banda Aceh, Provinsi Aceh, Indonesia"},
		{"publik tidak ada di basis data", "8.8.8.8", ""},
		{"IPv6 tidak ada di basis data", "2606:4700::1111", ""},
		{"kosong", "", ""},
		{"tidak valid", "bukan-ip", ""},
		{"host:port", "203.0.113.5:443", ""},
		{"tak-tentu", "0.0.0.0", ""},
	}
	for _, k := range kasus {
		t.Run(k.nama, func(t *testing.T) {
			assert.Equal(t, k.want, m.Lokasi(k.ip))
		})
	}
}

func TestMMDB_JenisBasisDataYangDiterima(t *testing.T) {
	rek := []rekamanUji{{"203.0.113.0/24", mmdbtype.Map{"country": tempat("en", "Indonesia")}}}
	for _, jenis := range []string{"GeoLite2-City", "GeoIP2-City", "GeoLite2-Country", "DBIP-City-Lite", "DBIP-Country-Lite", "DBIP-Location (compat=City)"} {
		t.Run(jenis, func(t *testing.T) {
			m := bukaUji(t, jenis, rek)
			assert.Equal(t, "Indonesia", m.Lokasi("203.0.113.9"))
		})
	}
}

func TestBukaMMDB_Galat(t *testing.T) {
	rek := []rekamanUji{{"203.0.113.0/24", mmdbtype.Map{"country": tempat("en", "Indonesia")}}}
	dir := t.TempDir()
	tulis := func(nama, isi string) string {
		p := filepath.Join(dir, nama)
		require.NoError(t, os.WriteFile(p, []byte(isi), 0o600))
		return p
	}

	kasus := []struct {
		nama, path, pesan string
	}{
		{"berkas tidak ada", filepath.Join(dir, "tidak-ada.mmdb"), "tidak-ada.mmdb"},
		{"direktori", dir, dir},
		{"berkas kosong", tulis("kosong.mmdb", ""), "kosong.mmdb"},
		{"bukan MMDB", tulis("teks.mmdb", "ini bukan basis data MMDB"), "teks.mmdb"},
		{"basis data ASN bukan basis data lokasi", tulisMMDB(t, "GeoLite2-ASN", rek), "GeoLite2-ASN"},
	}
	for _, k := range kasus {
		t.Run(k.nama, func(t *testing.T) {
			m, err := BukaMMDB(k.path)
			require.Error(t, err)
			assert.Nil(t, m)
			assert.Contains(t, err.Error(), k.pesan, "galat menyebut berkas atau jenis yang salah")
		})
	}
}

func TestMMDB_AmanDipanggilParalel(t *testing.T) {
	m := bukaUji(t, "GeoLite2-City", rekamanKota())
	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() {
			for range 200 {
				assert.Equal(t, "Kota Banda Aceh, Provinsi Aceh, Indonesia", m.Lokasi("203.0.113.5"))
				assert.Equal(t, "Japan", m.Lokasi("2001:db8::1"))
				assert.Equal(t, "", m.Lokasi("8.8.8.8"))
			}
		})
	}
	wg.Wait()
}

func TestMMDB_SetelahDitutupTidakPanik(t *testing.T) {
	m, err := BukaMMDB(tulisMMDB(t, "GeoLite2-City", rekamanKota()))
	require.NoError(t, err)
	require.NoError(t, m.Tutup())
	assert.Equal(t, "", m.Lokasi("203.0.113.5"), "basis data tertutup: lokasi tidak diketahui, bukan panik")
}

func TestGabungTanpaGanda(t *testing.T) {
	kasus := []struct {
		nama   string
		bagian []string
		want   string
	}{
		{"semua bagian", []string{"Bandung", "Jawa Barat", "Indonesia"}, "Bandung, Jawa Barat, Indonesia"},
		{"bagian kosong dibuang", []string{"", "Jawa Barat", "Indonesia"}, "Jawa Barat, Indonesia"},
		{"hanya negara", []string{"", "", "Indonesia"}, "Indonesia"},
		{"duplikat berurutan", []string{"Jakarta", "Jakarta", "Indonesia"}, "Jakarta, Indonesia"},
		{"duplikat tidak berurutan", []string{"Luxembourg", "Capellen", "luxembourg"}, "Luxembourg, Capellen"},
		{"semua kosong", []string{"", " ", ""}, ""},
		{"tanpa bagian", nil, ""},
	}
	for _, k := range kasus {
		t.Run(k.nama, func(t *testing.T) {
			assert.Equal(t, k.want, gabungTanpaGanda(k.bagian))
		})
	}
}

func TestDariBerkas(t *testing.T) {
	t.Run("tanpa path: hanya klasifikasi lokal", func(t *testing.T) {
		r, err := DariBerkas("")
		require.NoError(t, err)
		assert.Equal(t, LabelJaringanLokal, r.Lokasi("192.168.1.1"))
		assert.Equal(t, "", r.Lokasi("203.0.113.5"), "alamat publik tanpa basis data: lokasi tidak ditampilkan")
		assert.Equal(t, "", r.Lokasi(""))
	})

	t.Run("dengan basis data: lokal dahulu, lalu MMDB", func(t *testing.T) {
		// Basis data yang (keliru) memuat rentang privat tetap kalah dari klasifikasi lokal.
		path := tulisMMDB(t, "GeoLite2-City", append(rekamanKota(), rekamanUji{
			"10.0.0.0/8", mmdbtype.Map{"country": tempat("en", "Negeri Palsu")},
		}))
		r, err := DariBerkas(path)
		require.NoError(t, err)
		assert.Equal(t, LabelJaringanLokal, r.Lokasi("10.1.2.3"))
		assert.Equal(t, "Kota Banda Aceh, Provinsi Aceh, Indonesia", r.Lokasi("203.0.113.5"))
		assert.Equal(t, "", r.Lokasi("8.8.8.8"))
		assert.Equal(t, "", r.Lokasi("tidak-valid"))
	})

	t.Run("path salah mengembalikan galat, bukan resolver diam-diam", func(t *testing.T) {
		r, err := DariBerkas(filepath.Join(t.TempDir(), "tidak-ada.mmdb"))
		require.Error(t, err)
		assert.Nil(t, r)
	})
}

func TestBerantai(t *testing.T) {
	a := resolverTetap("")
	b := resolverTetap("dari B")
	c := resolverTetap("dari C")
	assert.Equal(t, "dari B", Berantai{a, b, c}.Lokasi("1.2.3.4"), "jawaban tak-kosong pertama")
	assert.Equal(t, "dari B", Berantai{nil, a, b}.Lokasi("1.2.3.4"), "elemen nil dilewati")
	assert.Equal(t, "", Berantai{a, a}.Lokasi("1.2.3.4"))
	assert.Equal(t, "", Berantai(nil).Lokasi("1.2.3.4"))
}

type resolverTetap string

func (r resolverTetap) Lokasi(string) string { return string(r) }
