package integration

// Kontrak OpenAPI (packages/contracts/openapi.yaml) untuk ringkasan operasional 08-B. Pola yang sama dengan tes kontrak
// modul cold storage: respons sungguhan diperiksa terhadap skema yang didokumentasikan (setiap properti terdokumentasi
// ada, tidak ada properti yang tidak terdokumentasi, null hanya bila skemanya nullable).

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/garda-inflasi/api/pkg/seed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const jalurKontrak = "../../../../../packages/contracts/openapi.yaml"

var jalurRingkasan = []string{"aktivitas-petugas", "stok", "produsen", "logistik"}

func kontrakMuat(t *testing.T) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(jalurKontrak)
	require.NoError(t, err)
	var spec map[string]any
	require.NoError(t, yaml.Unmarshal(raw, &spec), "openapi.yaml harus YAML yang valid")
	return spec
}

// kontrakGali menelusuri peta bersarang menurut kunci.
func kontrakGali(t *testing.T, node any, kunci ...string) any {
	t.Helper()
	for _, k := range kunci {
		m, ok := node.(map[string]any)
		require.True(t, ok, "tidak dapat menelusuri %q: bukan objek", k)
		node, ok = m[k]
		require.True(t, ok, "kunci %q tidak ada pada spesifikasi", k)
	}
	return node
}

// kontrakUraikan mengikuti $ref lokal sampai skema sebenarnya.
func kontrakUraikan(t *testing.T, spec map[string]any, skema any) map[string]any {
	t.Helper()
	m, ok := skema.(map[string]any)
	require.True(t, ok, "skema bukan objek: %v", skema)
	if ref, ok := m["$ref"].(string); ok {
		require.True(t, strings.HasPrefix(ref, "#/"), "hanya $ref lokal yang didukung: %s", ref)
		return kontrakUraikan(t, spec, kontrakGali(t, spec, strings.Split(strings.TrimPrefix(ref, "#/"), "/")...))
	}
	return m
}

func kontrakSkemaRespons(t *testing.T, spec map[string]any, jalur, status string) map[string]any {
	t.Helper()
	return kontrakUraikan(t, spec, kontrakGali(t, spec, "paths", jalur, "get", "responses", status, "content", "application/json", "schema"))
}

// kontrakSesuai memeriksa nilai JSON terhadap skema: setiap properti terdokumentasi harus ada, tidak boleh ada properti
// yang tidak terdokumentasi, dan null hanya bila skemanya nullable.
func kontrakSesuai(t *testing.T, spec map[string]any, skema map[string]any, nilai any, jalur string) {
	t.Helper()
	skema = kontrakUraikan(t, spec, skema)

	if nilai == nil {
		assert.Equal(t, true, skema["nullable"], "%s: null padahal skema tidak nullable", jalur)
		return
	}
	if enum, ok := skema["enum"].([]any); ok {
		assert.Contains(t, enum, nilai, "%s: di luar enum", jalur)
	}

	switch skema["type"] {
	case "object":
		obj, ok := nilai.(map[string]any)
		require.True(t, ok, "%s: harus objek, dapat %T", jalur, nilai)
		props, _ := skema["properties"].(map[string]any)
		// Bila skema mendaftar `required`, hanya properti itu yang wajib ada (mis. lampiran `ml` pipeline ML yang hanya
		// dipasang router penuh); tanpa `required` setiap properti terdokumentasi wajib ada.
		wajib := map[string]bool{}
		if req, ok := skema["required"].([]any); ok {
			for _, r := range req {
				wajib[r.(string)] = true
			}
		}
		for nama := range props {
			if len(wajib) > 0 && !wajib[nama] {
				continue
			}
			_, ada := obj[nama]
			assert.True(t, ada, "%s.%s terdokumentasi tetapi tidak ada pada respons", jalur, nama)
		}
		for nama, v := range obj {
			prop, terdokumentasi := props[nama]
			if !assert.True(t, terdokumentasi, "%s.%s ada pada respons tetapi tidak terdokumentasi", jalur, nama) {
				continue
			}
			kontrakSesuai(t, spec, prop.(map[string]any), v, jalur+"."+nama)
		}
	case "array":
		arr, ok := nilai.([]any)
		require.True(t, ok, "%s: harus array, dapat %T", jalur, nilai)
		item := skema["items"].(map[string]any)
		for i, e := range arr {
			kontrakSesuai(t, spec, item, e, fmt.Sprintf("%s[%d]", jalur, i))
		}
	case "string":
		str, ok := nilai.(string)
		require.True(t, ok, "%s: harus string, dapat %T", jalur, nilai)
		switch skema["format"] {
		case "date-time":
			_, err := time.Parse(time.RFC3339Nano, str)
			assert.NoError(t, err, "%s: bukan date-time", jalur)
		case "date":
			_, err := time.Parse("2006-01-02", str)
			assert.NoError(t, err, "%s: bukan date", jalur)
		case "uuid":
			assert.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`, str, "%s: bukan UUID huruf kecil", jalur)
		}
		if pola, ok := skema["pattern"].(string); ok {
			assert.Regexp(t, pola, str, "%s: tidak cocok pola", jalur)
		}
	case "integer":
		n, ok := nilai.(float64)
		require.True(t, ok, "%s: harus integer, dapat %T", jalur, nilai)
		assert.Equal(t, float64(int64(n)), n, "%s: harus bilangan bulat", jalur)
	case "number":
		_, ok := nilai.(float64)
		assert.True(t, ok, "%s: harus number, dapat %T", jalur, nilai)
	case "boolean":
		_, ok := nilai.(bool)
		assert.True(t, ok, "%s: harus boolean, dapat %T", jalur, nilai)
	}
}

var (
	kontrakOnce    sync.Once
	kontrakBersama *suite
)

// kontrakSuite mengembalikan satu basis data (dibuat sekali, hanya dibaca) yang memunculkan setiap bentuk respons: blok
// yang ada maupun yang null, jenis transaksi dengan dan tanpa asal atau tujuan, rute berkode dan tanpa kode, dan
// petugas yang belum pernah mencatat.
func kontrakSuite(t *testing.T) *suite {
	t.Helper()
	perluDocker(t)
	kontrakOnce.Do(func() { kontrakBersama = bangunKontrakSuite(t) })
	require.NotNil(t, kontrakBersama, "suite kontrak gagal dibuat")
	return kontrakBersama
}

func bangunKontrakSuite(t *testing.T) *suite {
	t.Helper()
	s := terpisah(t, "dashboard_kontrak", true)
	require.NoError(t, seed.SeedFiles(t.Context(), s.db, "demo/09_spasial_neraca.sql", "demo/14_eksekutif_data.sql"))
	h := hariIni()

	// Petugas yang belum pernah mencatat (entri_terakhir_pada null).
	s.sisipPengguna(t, "Petugas Baru", "baru@gardainflasi.id", "OPS_MAHASISWA", "AKTIF", "1171")
	// Komoditas berharga produsen tanpa neraca (produksi null) dan tanpa konsumen pembanding (konsumen dan margin null):
	// pasar Simeulue tidak punya entri konsumen.
	require.NoError(t, s.db.Exec(`INSERT INTO pasar (kode, nama, kabupaten_kode, lat, lng) VALUES ('PSR-1101-99', 'Pasar Uji Simeulue', '1101', 2.5, 96.0)`).Error)
	s.sisipHargaTetap(t, h, "PSR-1101-99", "MYK", "PRODUSEN", "DIPUBLIKASI", 15000, 15500)
	// Transaksi KELUAR dan PINDAH (demo hanya punya MASUK) dalam jendela, dan satu rute hasil optimasi tanpa kode.
	lot := s.sisipLotPasti(t, "CS-01", "TMT", 100, pukul(h.AddDate(0, 0, -30), 8, 0), pukul(h.AddDate(0, 0, 60), 8, 0), 0.1, "AKTIF")
	s.sisipTransaksiPasti(t, lot, "KELUAR", 4, "CS-01", "", pukul(h, 6, 0), "", "")
	s.sisipTransaksiPasti(t, lot, "PINDAH", 6, "CS-01", "CS-03", pukul(h, 6, 5), "", "")
	require.NoError(t, s.db.Exec(`
		INSERT INTO rute_distribusi (asal_kode, tujuan_kode, komoditas_kode, volume_ton, jarak_km, jam_tempuh, biaya, status, sumber)
		VALUES ('1106', '1108', 'BWM', 12.5, 268, 6, 5000000, 'USULAN', 'OPTIMASI')`).Error)
	s.segarkan(t)
	return s
}

func TestKontrak_ResponsMengikutiOpenAPI(t *testing.T) {
	s := kontrakSuite(t)
	spec := kontrakMuat(t)

	// Query yang memunculkan bentuk kosong maupun terfilter juga diperiksa.
	kasus := map[string][]string{
		"aktivitas-petugas": {"", "?kabupaten_kode=1171", "?kabupaten_kode=1101"},
		"stok":              {"", "?komoditas_kode=TMT", "?kabupaten_kode=1107", "?kabupaten_kode=1171"},
		"produsen":          {"", "?kabupaten_kode=1171", "?komoditas_kode=MYK", "?komoditas_kode=TIDAK-ADA"},
		"logistik":          {"", "?kabupaten_kode=1108", "?komoditas_kode=TMT", "?kabupaten_kode=1171"},
	}
	for _, nama := range jalurRingkasan {
		jalur := "/v1/dashboard/operasional/" + nama
		skema := kontrakSkemaRespons(t, spec, jalur, "200")
		for _, query := range kasus[nama] {
			t.Run(nama+query, func(t *testing.T) {
				w := s.call(t, http.MethodGet, jalur+query, emailBerhak, nil)
				require.Equal(t, http.StatusOK, w.Code, w.Body.String())
				var badan any
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &badan))
				kontrakSesuai(t, spec, skema, badan, "$")
			})
		}
	}
}

// Respons di atas harus benar-benar memuat setiap bentuk yang didokumentasikan, kalau tidak tes kontrak tidak menguji
// skema bagian itu (larik kosong dan null tidak memeriksa isi skemanya).
func TestKontrak_SemuaBentukTerwakili(t *testing.T) {
	s := kontrakSuite(t)

	petugas := s.petugasDari(t, emailBerhak, "")
	var tanpaEntri, denganEntri bool
	for _, p := range petugas.Petugas {
		tanpaEntri = tanpaEntri || p.EntriTerakhirPada == nil
		denganEntri = denganEntri || p.EntriTerakhirPada != nil
	}
	assert.True(t, tanpaEntri && denganEntri, "petugas dengan dan tanpa entri terakhir")

	stok := s.stokDari(t, emailBerhak, "")
	assert.NotEmpty(t, stok.Komoditas)

	produsen := s.produsenDari(t, emailBerhak, "")
	var adaMargin, tanpaMargin, adaProduksi, tanpaProduksi, tanpaProdusen bool
	for _, k := range produsen.Komoditas {
		adaMargin = adaMargin || k.Margin != nil
		tanpaMargin = tanpaMargin || (k.Margin == nil && k.Produsen != nil)
		adaProduksi = adaProduksi || k.Produksi != nil
		tanpaProduksi = tanpaProduksi || k.Produksi == nil
		tanpaProdusen = tanpaProdusen || k.Produsen == nil
	}
	assert.True(t, adaMargin && tanpaMargin && adaProduksi && tanpaProduksi && tanpaProdusen,
		"margin ada/null, produksi ada/null, dan baris produksi tanpa harga produsen: %+v", produsen.kodeKomoditas())

	logistik := s.logistikDari(t, emailBerhak, "")
	jenis, tanpaAsal, tanpaTujuan, denganKeduanya := map[string]bool{}, false, false, false
	for _, p := range logistik.Terbaru {
		jenis[p.Jenis] = true
		tanpaAsal = tanpaAsal || p.Asal == nil
		tanpaTujuan = tanpaTujuan || p.Tujuan == nil
		denganKeduanya = denganKeduanya || (p.Asal != nil && p.Tujuan != nil)
	}
	assert.Equal(t, map[string]bool{"MASUK": true, "KELUAR": true, "PINDAH": true}, jenis)
	assert.True(t, tanpaAsal && tanpaTujuan && denganKeduanya)
	var berkode, tanpaKode bool
	for _, r := range logistik.Rute.Items {
		berkode = berkode || r.Kode != nil
		tanpaKode = tanpaKode || r.Kode == nil
	}
	assert.True(t, berkode && tanpaKode, "rute dengan dan tanpa kode lama")
	assert.NotEmpty(t, logistik.Komoditas)
}

// Parameter yang didokumentasikan sama dengan yang diterima API, dan batas bentuk yang didokumentasikan ditegakkan.
func TestKontrak_ParameterSesuaiPerilaku(t *testing.T) {
	s := sharedSuite(t)
	spec := kontrakMuat(t)

	nama := func(jalur string) []string {
		var daftar []string
		for _, p := range kontrakGali(t, spec, "paths", jalur, "get", "parameters").([]any) {
			ref := p.(map[string]any)["$ref"].(string)
			param := kontrakUraikan(t, spec, map[string]any{"$ref": ref})
			// cycle_id hanya memilih siklus lampiran `ml` (pengaya modul ML, diuji di internal/ml/integration).
			if param["name"] == "cycle_id" {
				continue
			}
			daftar = append(daftar, param["name"].(string))
		}
		return daftar
	}
	assert.ElementsMatch(t, []string{"kabupaten_kode"}, nama("/v1/dashboard/operasional/aktivitas-petugas"))
	for _, j := range []string{"stok", "produsen", "logistik"} {
		assert.ElementsMatch(t, []string{"kabupaten_kode", "komoditas_kode"}, nama("/v1/dashboard/operasional/"+j), j)
	}

	// Pola kode yang didokumentasikan sama dengan yang ditegakkan: contoh dokumentasi diterima, pelanggar pola ditolak.
	kab := kontrakUraikan(t, spec, kontrakGali(t, spec, "components", "parameters", "DashboardKabupatenKode", "schema").(map[string]any))
	kom := kontrakUraikan(t, spec, kontrakGali(t, spec, "components", "parameters", "DashboardKomoditasKode", "schema").(map[string]any))
	polaKab, polaKom := regexp.MustCompile(kab["pattern"].(string)), regexp.MustCompile(kom["pattern"].(string))
	for _, j := range jalurRingkasan {
		jalur := awalan + "/" + j
		assert.True(t, polaKab.MatchString(kab["example"].(string)))
		assert.Equal(t, http.StatusOK, s.call(t, http.MethodGet, jalur+"?kabupaten_kode="+kab["example"].(string), emailBerhak, nil).Code, j)
		for _, buruk := range []string{"abc", "117", "11711"} {
			assert.False(t, polaKab.MatchString(buruk))
			assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodGet, jalur+"?kabupaten_kode="+buruk, emailBerhak, nil).Code, j+" kabupaten "+buruk)
		}
		if j == "aktivitas-petugas" {
			continue
		}
		assert.Equal(t, http.StatusOK, s.call(t, http.MethodGet, jalur+"?komoditas_kode="+kom["example"].(string), emailBerhak, nil).Code, j)
		for _, buruk := range []string{"cabai", "A", "-AB"} {
			assert.False(t, polaKom.MatchString(buruk))
			assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodGet, jalur+"?komoditas_kode="+buruk, emailBerhak, nil).Code, j+" komoditas "+buruk)
		}
	}
}

// Setiap status yang didokumentasikan ada untuk tiap rute, dan seluruh $ref yang dipakai rute dan skemanya terpecahkan.
func TestKontrak_StatusDanRujukanLengkap(t *testing.T) {
	spec := kontrakMuat(t)
	diperiksa := map[string]bool{}

	var telusuri func(node any)
	telusuri = func(node any) {
		switch v := node.(type) {
		case map[string]any:
			if ref, ok := v["$ref"].(string); ok && !diperiksa[ref] {
				diperiksa[ref] = true
				telusuri(kontrakUraikan(t, spec, map[string]any{"$ref": ref}))
			}
			for _, anak := range v {
				telusuri(anak)
			}
		case []any:
			for _, anak := range v {
				telusuri(anak)
			}
		}
	}
	for _, nama := range jalurRingkasan {
		jalur := "/v1/dashboard/operasional/" + nama
		operasi := kontrakGali(t, spec, "paths", jalur, "get").(map[string]any)
		respons := operasi["responses"].(map[string]any)
		assert.Contains(t, respons, "200", jalur)
		assert.Contains(t, respons, "403", jalur)
		assert.Contains(t, respons, "400", jalur)
		assert.Contains(t, operasi["description"], "dashboard.operasional", jalur)
		assert.Contains(t, operasi["description"], "Cakupan baris (02-B)", jalur)
		assert.NotEmpty(t, operasi["operationId"], jalur)
		assert.Equal(t, []any{"Dashboard"}, operasi["tags"], jalur)
		telusuri(operasi)
	}
	assert.Contains(t, diperiksa, "#/components/schemas/StokResponse")
	assert.Contains(t, diperiksa, "#/components/schemas/LogistikRute")
	assert.Contains(t, diperiksa, "#/components/schemas/IdBigint")
	assert.Contains(t, diperiksa, "#/components/schemas/ErrorResponse")

	// Izin tambahan aktivitas-petugas terdokumentasi.
	deskripsi := kontrakGali(t, spec, "paths", "/v1/dashboard/operasional/aktivitas-petugas", "get", "description").(string)
	assert.Contains(t, deskripsi, "`harga.lihat`")
}
