package integration

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/dashboard/refresher"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Filter wilayah, komoditas, dan periode pada KPI dan heatmap (08-B). Hasil API dibandingkan dengan perhitungan langsung
// dari tabel dasar (entri_harga, agregat harian, stok cold storage) yang tidak melewati mv_kpi_operasional maupun
// penyatuan di usecase, dan dengan angka hasil hitungan tangan pada data terkendali.

const (
	// Kabupaten demo: 1171 Kota Banda Aceh (Koordinator dan Agregator), 1108 dan 1106 punya cold storage operasional,
	// 1107 hanya punya fasilitas PERAWATAN.
	kabBanda   = "1171"
	kabSelatan = "1108"
)

// hariIni adalah tanggal Aceh hari ini (DATE, tengah malam UTC), dengan jam yang sama dengan yang dipakai API.
func hariIni() time.Time {
	t := time.Now().In(time.FixedZone("WIB", 7*3600))
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func tanggal(t time.Time) string { return t.Format("2006-01-02") }

// kpiDari memanggil GET /v1/dashboard/operasional/kpi?query sebagai email dan mengurai kartunya.
func (s *suite) kpiDari(t *testing.T, email, query string) []kpi {
	t.Helper()
	w := s.call(t, http.MethodGet, "/v1/dashboard/operasional/kpi"+query, email, nil)
	require.Equal(t, http.StatusOK, w.Code, "%s: %s", query, w.Body.String())
	return data[[]kpi](t, w)
}

// petaPanas adalah respons heatmap lengkap dengan sumber sel.
type petaPanas struct {
	Kabupaten []struct {
		Kode string `json:"kode"`
		Nama string `json:"nama"`
	} `json:"kabupaten"`
	Komoditas []struct {
		Kode  string `json:"kode"`
		Pilot bool   `json:"pilot"`
	} `json:"komoditas"`
	Sel []struct {
		KomoditasKode string  `json:"komoditas_kode"`
		KabupatenKode string  `json:"kabupaten_kode"`
		Delta         float64 `json:"delta"`
		Harga         float64 `json:"harga"`
		Sumber        string  `json:"sumber"`
	} `json:"sel"`
}

func (s *suite) heatmapDari(t *testing.T, email, query string) petaPanas {
	t.Helper()
	w := s.call(t, http.MethodGet, "/v1/dashboard/operasional/heatmap"+query, email, nil)
	require.Equal(t, http.StatusOK, w.Code, "%s: %s", query, w.Body.String())
	return data[petaPanas](t, w)
}

func kartuByID(t *testing.T, kpis []kpi, id string) kpi {
	t.Helper()
	for _, k := range kpis {
		if k.ID == id {
			return k
		}
	}
	require.Failf(t, "kartu tidak ada", "id %s", id)
	return kpi{}
}

func bulat(v float64) float64 { return math.Round(v*10) / 10 }

// deretUji adalah nilai harian (dari..sampai) tiap indikator.
type deretUji map[string][]float64

func query(kab, kom string, dari, sampai time.Time) string {
	q := url.Values{}
	if kab != "" {
		q.Set("kabupaten_kode", kab)
	}
	if kom != "" {
		q.Set("komoditas_kode", kom)
	}
	if !dari.IsZero() {
		q.Set("dari", tanggal(dari))
	}
	if !sampai.IsZero() {
		q.Set("sampai", tanggal(sampai))
	}
	if len(q) == 0 {
		return ""
	}
	return "?" + q.Encode()
}

// dariTabelDasar menghitung deret harian empat indikator langsung dari entri_harga, agregat_harga_harian, dan stok
// cold storage menurut definisi KPI: tanpa mv_kpi_operasional. kab dan kom kosong berarti tanpa filter.
func (s *suite) dariTabelDasar(t *testing.T, kab, kom string, dari, sampai time.Time) deretUji {
	t.Helper()
	hasil := deretUji{}
	baca := func(indikator, sql string, args ...any) {
		var rows []struct {
			Tanggal string
			Nilai   float64
		}
		require.NoError(t, s.db.Raw(sql, args...).Scan(&rows).Error, indikator)
		sort.Slice(rows, func(i, j int) bool { return rows[i].Tanggal < rows[j].Tanggal })
		for _, r := range rows {
			hasil[indikator] = append(hasil[indikator], r.Nilai)
		}
		require.Len(t, hasil[indikator], int(sampai.Sub(dari).Hours()/24)+1, indikator)
	}

	// Pasangan (kab, kom, kab, kom) berulang karena setiap kemunculan penanda ? butuh argumennya sendiri.
	baca("entri", `
		SELECT to_char(d, 'YYYY-MM-DD') AS tanggal,
		       (SELECT count(*) FROM entri_harga e JOIN pasar p ON p.kode = e.pasar_id
		         WHERE e.status <> 'DRAF' AND (e.dicatat_pada AT TIME ZONE 'Asia/Jakarta')::date = d::date
		           AND (?::text = '' OR p.kabupaten_kode = ?) AND (?::text = '' OR e.komoditas_kode = ?))::float8 AS nilai
		  FROM generate_series(?::date, ?::date, interval '1 day') d`,
		kab, kab, kom, kom, tanggal(dari), tanggal(sampai))

	baca("validasi", `
		SELECT to_char(d, 'YYYY-MM-DD') AS tanggal,
		       COALESCE((SELECT 100.0 * count(*) FILTER (WHERE e.status IN ('DIVALIDASI', 'DIPUBLIKASI')) / NULLIF(count(*), 0)
		                   FROM entri_harga e JOIN pasar p ON p.kode = e.pasar_id
		                  WHERE e.status <> 'DRAF'
		                    AND (e.dicatat_pada AT TIME ZONE 'Asia/Jakarta')::date BETWEEN d::date - 6 AND d::date
		                    AND (?::text = '' OR p.kabupaten_kode = ?) AND (?::text = '' OR e.komoditas_kode = ?)), 0)::float8 AS nilai
		  FROM generate_series(?::date, ?::date, interval '1 day') d`,
		kab, kab, kom, kom, tanggal(dari), tanggal(sampai))

	baca("waspada", `
		SELECT to_char(d, 'YYYY-MM-DD') AS tanggal,
		       (SELECT count(DISTINCT a.komoditas_kode)
		          FROM agregat_harga_harian a
		         WHERE a.tipe = 'KONSUMEN' AND a.tanggal = d::date
		           AND (?::text = '' OR a.kabupaten_kode = ?) AND (?::text = '' OR a.komoditas_kode = ?)
		           AND (SELECT SUM(b.rata_rata * b.jumlah) / SUM(b.jumlah) FROM agregat_harga_harian b
		                 WHERE b.tipe = 'KONSUMEN' AND b.komoditas_kode = a.komoditas_kode AND b.kabupaten_kode = a.kabupaten_kode
		                   AND b.tanggal BETWEEN a.tanggal - 7 AND a.tanggal - 1) > 0
		           AND (a.rata_rata - (SELECT SUM(b.rata_rata * b.jumlah) / SUM(b.jumlah) FROM agregat_harga_harian b
		                 WHERE b.tipe = 'KONSUMEN' AND b.komoditas_kode = a.komoditas_kode AND b.kabupaten_kode = a.kabupaten_kode
		                   AND b.tanggal BETWEEN a.tanggal - 7 AND a.tanggal - 1))
		               / (SELECT SUM(b.rata_rata * b.jumlah) / SUM(b.jumlah) FROM agregat_harga_harian b
		                 WHERE b.tipe = 'KONSUMEN' AND b.komoditas_kode = a.komoditas_kode AND b.kabupaten_kode = a.kabupaten_kode
		                   AND b.tanggal BETWEEN a.tanggal - 7 AND a.tanggal - 1) > 0.07)::float8 AS nilai
		  FROM generate_series(?::date, ?::date, interval '1 day') d`,
		kab, kab, kom, kom, tanggal(dari), tanggal(sampai))

	baca("cs", `
		SELECT to_char(d, 'YYYY-MM-DD') AS tanggal,
		       CASE WHEN k.ton > 0 THEN
		            100.0 * COALESCE((SELECT SUM(s.volume_ton)
		                                FROM stok_cold_storage s
		                                JOIN cold_storage c ON c.id = s.cold_storage_id AND c.status = 'OPERASIONAL'
		                               WHERE (s.masuk_pada AT TIME ZONE 'Asia/Jakarta')::date <= d::date
		                                 AND (s.kedaluwarsa_pada AT TIME ZONE 'Asia/Jakarta')::date > d::date
		                                 AND (?::text = '' OR c.kabupaten_kode = ?) AND (?::text = '' OR s.komoditas_kode = ?)), 0) / k.ton
		       ELSE 0 END::float8 AS nilai
		  FROM generate_series(?::date, ?::date, interval '1 day') d
		 CROSS JOIN (SELECT COALESCE(SUM(kapasitas_ton), 0) AS ton FROM cold_storage
		              WHERE status = 'OPERASIONAL' AND (?::text = '' OR kabupaten_kode = ?)) k`,
		kab, kab, kom, kom, tanggal(dari), tanggal(sampai), kab, kab)
	return hasil
}

// samaDenganTabelDasar memastikan tiap kartu (tren dan nilai) cocok dengan deret hasil hitungan langsung. Hitungan
// bulat dibandingkan persis; persentase dalam 0,1 (pembulatan satu desimal di kedua sisi).
func samaDenganTabelDasar(t *testing.T, nama string, kpis []kpi, want deretUji) {
	t.Helper()
	require.Len(t, kpis, 4, nama)
	for _, id := range []string{"entri", "validasi", "waspada", "cs"} {
		k := kartuByID(t, kpis, id)
		w := want[id]
		require.Len(t, k.Tren, len(w), "%s %s: panjang tren", nama, id)
		for i := range w {
			if id == "entri" || id == "waspada" {
				assert.Equal(t, w[i], k.Tren[i], "%s %s hari ke-%d", nama, id, i)
			} else {
				assert.InDelta(t, w[i], k.Tren[i], 0.11, "%s %s hari ke-%d", nama, id, i)
			}
		}
		assert.Equal(t, k.Tren[len(k.Tren)-1], k.Nilai, "%s %s: nilai adalah titik terakhir tren", nama, id)
	}
}

// ── Definisi lama ─────────────────────────────────────────────────────

// kpiDefinisiLama adalah SELECT view mv_kpi_operasional sebelum 000124 (nilai jadi tingkat provinsi, 14 hari), salinan
// dari 000122. Tampilan tanpa filter harus tetap sama dengannya.
const kpiDefinisiLama = `
WITH hari AS (
    SELECT d::date AS tanggal
      FROM generate_series((now() AT TIME ZONE 'Asia/Jakarta')::date - 13, (now() AT TIME ZONE 'Asia/Jakarta')::date, interval '1 day') AS d
),
entri AS (
    SELECT (dicatat_pada AT TIME ZONE 'Asia/Jakarta')::date AS tanggal, status
      FROM entri_harga
     WHERE status <> 'DRAF' AND dicatat_pada >= now() - interval '25 days'
),
harian AS (
    SELECT komoditas_kode, kabupaten_kode, tanggal, rata_rata,
           (SELECT SUM(b.rata_rata * b.jumlah) / SUM(b.jumlah)
              FROM agregat_harga_harian b
             WHERE b.tipe = 'KONSUMEN' AND b.komoditas_kode = a.komoditas_kode AND b.kabupaten_kode = a.kabupaten_kode
               AND b.tanggal BETWEEN a.tanggal - 7 AND a.tanggal - 1) AS basis
      FROM agregat_harga_harian a
     WHERE a.tipe = 'KONSUMEN' AND a.tanggal >= (now() AT TIME ZONE 'Asia/Jakarta')::date - 13
),
kapasitas AS (
    SELECT COALESCE(SUM(kapasitas_ton), 0) AS ton FROM cold_storage WHERE status = 'OPERASIONAL'
)
SELECT 'entri'::text AS indikator, h.tanggal,
       (SELECT COUNT(*) FROM entri e WHERE e.tanggal = h.tanggal)::double precision AS nilai
  FROM hari h
UNION ALL
SELECT 'validasi', h.tanggal,
       COALESCE((SELECT 100.0 * COUNT(*) FILTER (WHERE status IN ('DIVALIDASI', 'DIPUBLIKASI')) / NULLIF(COUNT(*), 0)
                   FROM entri e
                  WHERE e.tanggal BETWEEN h.tanggal - 6 AND h.tanggal), 0)::double precision
  FROM hari h
UNION ALL
SELECT 'waspada', h.tanggal,
       (SELECT COUNT(DISTINCT komoditas_kode)
          FROM harian a
         WHERE a.tanggal = h.tanggal AND a.basis > 0 AND (a.rata_rata - a.basis) / a.basis > 0.07)::double precision
  FROM hari h
UNION ALL
SELECT 'cs', h.tanggal,
       CASE WHEN k.ton > 0 THEN
           100.0 * COALESCE((SELECT SUM(s.volume_ton)
                               FROM stok_cold_storage s
                               JOIN cold_storage c ON c.id = s.cold_storage_id AND c.status = 'OPERASIONAL'
                              WHERE (s.masuk_pada AT TIME ZONE 'Asia/Jakarta')::date <= h.tanggal
                                AND (s.kedaluwarsa_pada AT TIME ZONE 'Asia/Jakarta')::date > h.tanggal), 0) / k.ton
       ELSE 0 END::double precision
  FROM hari h CROSS JOIN kapasitas k`

func TestKpiFilter_TanpaFilterTidakBerubahDariDefinisiLama(t *testing.T) {
	s := sharedSuite(t)
	var lama []struct {
		Indikator string
		Tanggal   string
		Nilai     float64
	}
	require.NoError(t, s.db.Raw(`SELECT indikator, to_char(tanggal, 'YYYY-MM-DD') AS tanggal, nilai FROM (`+kpiDefinisiLama+`) x ORDER BY indikator, tanggal`).Scan(&lama).Error)
	require.Len(t, lama, 56)

	kpis := s.kpiDari(t, emailBerhak, "")

	require.Len(t, kpis, 4)
	for _, id := range []string{"entri", "validasi", "waspada", "cs"} {
		var want []float64
		for _, r := range lama {
			if r.Indikator == id {
				want = append(want, r.Nilai)
			}
		}
		k := kartuByID(t, kpis, id)
		require.Len(t, k.Tren, 14, id)
		for i := range want {
			assert.InDelta(t, want[i], k.Tren[i], 0.051, "%s hari ke-%d: tampilan tanpa filter harus sama dengan definisi lama", id, i)
		}
		assert.InDelta(t, want[13], k.Nilai, 0.051, id)
	}
}

// ── Filter terhadap tabel dasar (data demo) ───────────────────────────

func TestKpiFilter_SesuaiHitunganDariTabelDasar(t *testing.T) {
	s := sharedSuite(t)
	hari := hariIni()
	tabel := []struct {
		nama         string
		kab, kom     string
		dari, sampai time.Time
	}{
		{"tanpa filter, 14 hari", "", "", hari.AddDate(0, 0, -13), hari},
		{"kabupaten Banda Aceh", kabBanda, "", hari.AddDate(0, 0, -13), hari},
		{"kabupaten dengan cold storage", kabSelatan, "", hari.AddDate(0, 0, -13), hari},
		{"komoditas cabai", "", "CBM-KRT", hari.AddDate(0, 0, -13), hari},
		{"kabupaten dan komoditas, cold storage berisi komoditas itu", kabSelatan, "BWM", hari.AddDate(0, 0, -13), hari},
		{"kabupaten dan komoditas, cold storage tanpa komoditas itu", kabSelatan, "TMT", hari.AddDate(0, 0, -13), hari},
		{"kabupaten berfasilitas perawatan: tanpa kapasitas operasional", "1107", "", hari.AddDate(0, 0, -13), hari},
		{"kabupaten tanpa data sama sekali", "1101", "", hari.AddDate(0, 0, -13), hari},
		{"periode 30 hari", "", "", hari.AddDate(0, 0, -29), hari},
		{"periode 7 hari", kabBanda, "TMT", hari.AddDate(0, 0, -6), hari},
		{"periode satu hari", "", "", hari, hari},
		{"tanggal acuan di masa lalu", "", "", hari.AddDate(0, 0, -30), hari.AddDate(0, 0, -20)},
		{"tanggal acuan di masa lalu dengan filter", kabSelatan, "CBM-KRT", hari.AddDate(0, 0, -45), hari.AddDate(0, 0, -25)},
		{"periode terpanjang (90 hari)", "", "", hari.AddDate(0, 0, -89), hari},
	}
	for _, c := range tabel {
		t.Run(c.nama, func(t *testing.T) {
			kpis := s.kpiDari(t, emailBerhak, query(c.kab, c.kom, c.dari, c.sampai))
			want := s.dariTabelDasar(t, c.kab, c.kom, c.dari, c.sampai)
			samaDenganTabelDasar(t, c.nama, kpis, want)
		})
	}
}

// Sepanjang periode, delta dibandingkan terhadap tanggal acuan, jadi memendekkan periode tidak mengubahnya.
func TestKpiFilter_DeltaDanNilaiTidakBergantungPadaPanjangPeriode(t *testing.T) {
	s := sharedSuite(t)
	hari := hariIni()

	panjang := s.kpiDari(t, emailBerhak, query(kabSelatan, "", hari.AddDate(0, 0, -29), hari))
	pendek := s.kpiDari(t, emailBerhak, query(kabSelatan, "", hari.AddDate(0, 0, -2), hari))

	for _, id := range []string{"entri", "validasi", "waspada", "cs"} {
		assert.Len(t, kartuByID(t, pendek, id).Tren, 3, id)
		assert.Len(t, kartuByID(t, panjang, id).Tren, 30, id)
		assert.Equal(t, kartuByID(t, panjang, id).Nilai, kartuByID(t, pendek, id).Nilai, id)
		assert.Equal(t, kartuByID(t, panjang, id).Delta, kartuByID(t, pendek, id).Delta, "delta %s", id)
	}
}

func TestKpiFilter_LabelDanHrefMengikutiTanggalAcuan(t *testing.T) {
	s := sharedSuite(t)
	hari := hariIni()

	hariIniKpi := s.kpiDari(t, emailBerhak, "")
	lampau := s.kpiDari(t, emailBerhak, query("", "", time.Time{}, hari.AddDate(0, 0, -10)))

	assert.Equal(t, "Entri harga hari ini", kartuByID(t, hariIniKpi, "entri").Label)
	assert.NotEqual(t, "Entri harga hari ini", kartuByID(t, lampau, "entri").Label, "tanggal lampau tidak boleh berlabel 'hari ini'")
	assert.Contains(t, kartuByID(t, lampau, "entri").Label, fmt.Sprint(hari.AddDate(0, 0, -10).Year()))
	assert.Equal(t, "/data/harga?status=DIAJUKAN", kartuByID(t, lampau, "validasi").Href, "bentuk respons tidak berubah")
}

// Nada kartu mengikuti target hanya untuk seluruh Aceh tanpa filter: target_kpi adalah target provinsi.
func TestKpiFilter_NadaHanyaUntukSeluruhAcehTanpaFilter(t *testing.T) {
	s := sharedSuite(t)

	tanpa := s.kpiDari(t, emailBerhak, "")
	assert.Equal(t, "normal", kartuByID(t, tanpa, "cs").Tone, "utilisasi demo jauh di atas target 75%")
	assert.NotEqual(t, "data", kartuByID(t, tanpa, "validasi").Tone, "provinsi punya target validasi")
	assert.NotEqual(t, "data", kartuByID(t, tanpa, "waspada").Tone, "provinsi punya target waspada")

	for _, q := range []string{"?kabupaten_kode=" + kabSelatan, "?komoditas_kode=CBM-KRT", "?kabupaten_kode=" + kabBanda + "&komoditas_kode=TMT"} {
		for _, k := range s.kpiDari(t, emailBerhak, q) {
			assert.Equal(t, "data", k.Tone, "%s %s: target provinsi tidak dipakai untuk tampilan terfilter", q, k.ID)
		}
	}
	// Periode saja tidak mengubah tingkat: tetap seluruh Aceh.
	periode := s.kpiDari(t, emailBerhak, "?dari="+tanggal(hariIni().AddDate(0, 0, -6)))
	assert.NotEqual(t, "data", kartuByID(t, periode, "validasi").Tone)
}

// ── Filter terhadap angka hasil hitung tangan (data terkendali) ───────

const (
	pasarBanda   = "PSR-1171-01"
	pasarSelatan = "PSR-1108-01"
)

// sisipEntri menyisipkan n entri KONSUMEN pada tanggal hari (pukul 09.00 WIB dan menit berikutnya).
func (s *suite) sisipEntri(t *testing.T, hari time.Time, pasar, komoditas, status string, harga, n int) {
	t.Helper()
	require.NoError(t, s.db.Exec(`
		INSERT INTO entri_harga (komoditas_kode, pasar_id, tipe, harga, satuan, status, dicatat_pada)
		SELECT ?, ?, 'KONSUMEN', ?, 'kg', ?::status_entri,
		       ((?::date + time '09:00')::timestamp AT TIME ZONE 'Asia/Jakarta') + (g * interval '1 minute')
		  FROM generate_series(1, ?) g`, komoditas, pasar, harga, status, tanggal(hari), n).Error)
}

// sisipLot menyisipkan satu lot stok di fasilitas (kode) yang berlaku dari hari masuk sampai kedaluwarsa.
func (s *suite) sisipLot(t *testing.T, fasilitas, komoditas string, ton float64, masuk, kedaluwarsa time.Time) {
	t.Helper()
	require.NoError(t, s.db.Exec(`
		INSERT INTO stok_cold_storage (cold_storage_id, komoditas_kode, volume_ton, masuk_pada, pemilik, risiko_food_loss, kedaluwarsa_pada)
		SELECT id, ?, ?, ((?::date + time '08:00')::timestamp AT TIME ZONE 'Asia/Jakarta'), 'KUD', 0.1,
		       ((?::date + time '08:00')::timestamp AT TIME ZONE 'Asia/Jakarta')
		  FROM cold_storage WHERE kode = ?`, komoditas, ton, tanggal(masuk), tanggal(kedaluwarsa), fasilitas).Error)
}

func (s *suite) segarkan(t *testing.T) {
	t.Helper()
	require.NoError(t, refresher.Shared(s.db, logger.NewDefault()).Refresh(t.Context()))
}

// kpiTerkendali membangun data kecil yang angkanya bisa dihitung dengan tangan. Hari ini = H.
//
// Entri (semua KONSUMEN):
//   - H-8..H-1, tiap hari: Banda Aceh (1171) 1 entri TMT, 1 CBM-KRT, 1 DAG-AYM, dan Aceh Selatan (1108) 1 CBM-KRT, semuanya
//     DIPUBLIKASI dengan harga datar: TMT 5000, CBM-KRT 10000, DAG-AYM 30000.
//   - H: 1171 TMT 5100 (naik 2%), CBM-KRT 12000 (naik 20%), DAG-AYM 40000 (naik 33%), 1108 CBM-KRT 12000 (naik 20%),
//     semuanya DIPUBLIKASI; ditambah 1108: 99 entri BWM DIAJUKAN dan 5 entri BWM DRAF.
//
// Stok: CS-01 (1108, kapasitas 400 t) 100 t BWM; CS-03 (1106, 250 t) 25 t TMT; CS-04 (1107, PERAWATAN) 50 t BWM yang tidak
// dihitung. CS-02 (1174, 300 t) kosong. Semua lot berlaku dari H-10 sampai H+20.
func kpiTerkendali(t *testing.T, nama string) (*suite, time.Time) {
	t.Helper()
	s := newSuite(t, nama, false)
	h := hariIni()

	for d := 8; d >= 1; d-- {
		hari := h.AddDate(0, 0, -d)
		s.sisipEntri(t, hari, pasarBanda, "TMT", "DIPUBLIKASI", 5000, 1)
		s.sisipEntri(t, hari, pasarBanda, "CBM-KRT", "DIPUBLIKASI", 10000, 1)
		s.sisipEntri(t, hari, pasarBanda, "DAG-AYM", "DIPUBLIKASI", 30000, 1)
		s.sisipEntri(t, hari, pasarSelatan, "CBM-KRT", "DIPUBLIKASI", 10000, 1)
	}
	s.sisipEntri(t, h, pasarBanda, "TMT", "DIPUBLIKASI", 5100, 1)
	s.sisipEntri(t, h, pasarBanda, "CBM-KRT", "DIPUBLIKASI", 12000, 1)
	s.sisipEntri(t, h, pasarBanda, "DAG-AYM", "DIPUBLIKASI", 40000, 1)
	s.sisipEntri(t, h, pasarSelatan, "CBM-KRT", "DIPUBLIKASI", 12000, 1)
	s.sisipEntri(t, h, pasarSelatan, "BWM", "DIAJUKAN", 30000, 99)
	s.sisipEntri(t, h, pasarSelatan, "BWM", "DRAF", 30000, 5)

	s.sisipLot(t, "CS-01", "BWM", 100, h.AddDate(0, 0, -10), h.AddDate(0, 0, 20))
	s.sisipLot(t, "CS-03", "TMT", 25, h.AddDate(0, 0, -10), h.AddDate(0, 0, 20))
	s.sisipLot(t, "CS-04", "BWM", 50, h.AddDate(0, 0, -10), h.AddDate(0, 0, 20))
	s.segarkan(t)
	return s, h
}

// Hitungan tangan hari ini (H), jendela validasi H-6..H:
//
//	entri:    1171 = 3, 1108 = 1 + 99 = 100, seluruh Aceh = 103 (DRAF tidak dihitung).
//	validasi: 1171 = 6x3 + 3 = 21 sah dari 21 entri (100%); 1108 = 6x1 + 1 = 7 sah dari 6 + 100 = 106 (6,6%);
//	          seluruh Aceh = 28 sah dari 127 = 22,0%. Rata-rata persentase kabupaten = 53,3%: SALAH.
//	waspada:  CBM-KRT melampaui di 1171 dan 1108 (satu komoditas), DAG-AYM di 1171, TMT tidak: seluruh Aceh = 2,
//	          1171 = 2, 1108 = 1.
//	cs:       seluruh Aceh 125 / 950 = 13,2%; 1108 = 100 / 400 = 25%; 1106 = 25 / 250 = 10%; 1174 = 0 / 300 = 0%;
//	          1107 = 0 (fasilitas perawatan tidak punya kapasitas operasional, lotnya tidak dihitung). Rata-rata persentase
//	          fasilitas yang berisi = 17,5%, atau seluruh fasilitas operasional = 11,7%: SALAH.
func TestKpiFilter_AngkaHasilHitungTangan(t *testing.T) {
	s, h := kpiTerkendali(t, "dashboard_kpi_terkendali")

	tabel := []struct {
		nama     string
		kab, kom string
		// nilai hari ini per indikator
		entri, validasi, waspada, cs float64
	}{
		{"seluruh Aceh", "", "", 103, 22.0, 2, 13.2},
		{"Banda Aceh", "1171", "", 3, 100, 2, 0},
		{"Aceh Selatan", "1108", "", 100, 6.6, 1, 25},
		{"1106: cold storage tanpa entri harga", "1106", "", 0, 0, 0, 10},
		{"1174: kapasitas tanpa stok", "1174", "", 0, 0, 0, 0},
		{"1107: fasilitas perawatan", "1107", "", 0, 0, 0, 0},
		// Filter komoditas: pembilang cs hanya volume komoditas itu, penyebutnya tetap kapasitas.
		{"komoditas CBM-KRT", "", "CBM-KRT", 2, 100, 1, 0},
		{"komoditas TMT", "", "TMT", 1, 100, 0, 2.6},
		{"komoditas BWM", "", "BWM", 99, 0, 0, 10.5},
		{"komoditas DAG-AYM", "", "DAG-AYM", 1, 100, 1, 0},
		{"1171 dan CBM-KRT", "1171", "CBM-KRT", 1, 100, 1, 0},
		{"1108 dan CBM-KRT", "1108", "CBM-KRT", 1, 100, 1, 0},
		{"1108 dan BWM", "1108", "BWM", 99, 0, 0, 25},
		{"1108 dan TMT: kapasitas ada, stok TMT tidak", "1108", "TMT", 0, 0, 0, 0},
		{"1171 dan BWM: tidak ada entri", "1171", "BWM", 0, 0, 0, 0},
		{"1106 dan TMT", "1106", "TMT", 0, 0, 0, 10},
	}
	for _, c := range tabel {
		t.Run(c.nama, func(t *testing.T) {
			kpis := s.kpiDari(t, emailBerhak, query(c.kab, c.kom, time.Time{}, time.Time{}))
			assert.Equal(t, c.entri, kartuByID(t, kpis, "entri").Nilai, "entri")
			assert.Equal(t, c.validasi, kartuByID(t, kpis, "validasi").Nilai, "validasi")
			assert.Equal(t, c.waspada, kartuByID(t, kpis, "waspada").Nilai, "waspada")
			assert.Equal(t, c.cs, kartuByID(t, kpis, "cs").Nilai, "cs")
			for _, k := range kpis {
				assert.Len(t, k.Tren, 14, k.ID)
			}
		})
	}

	t.Run("tren: hari-hari sebelumnya", func(t *testing.T) {
		kpis := s.kpiDari(t, emailBerhak, "")
		entri := kartuByID(t, kpis, "entri").Tren
		// H-13..H-9 kosong, H-8..H-1 masing-masing 4 entri, H 103.
		assert.Equal(t, []float64{0, 0, 0, 0, 0, 4, 4, 4, 4, 4, 4, 4, 4, 103}, entri)
		// Lot berlaku sejak H-10: tiga hari pertama nol, sesudahnya 125 / 950.
		cs := kartuByID(t, kpis, "cs").Tren
		assert.Equal(t, []float64{0, 0, 0, 13.2, 13.2, 13.2, 13.2, 13.2, 13.2, 13.2, 13.2, 13.2, 13.2, 13.2}, cs)
		waspada := kartuByID(t, kpis, "waspada").Tren
		assert.Equal(t, []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2}, waspada)
	})

	t.Run("periode: kemarin sebagai tanggal acuan", func(t *testing.T) {
		kemarin := h.AddDate(0, 0, -1)
		kpis := s.kpiDari(t, emailBerhak, query("", "", time.Time{}, kemarin))
		assert.Equal(t, 4.0, kartuByID(t, kpis, "entri").Nilai, "H-1: 4 entri")
		assert.Equal(t, 100.0, kartuByID(t, kpis, "validasi").Nilai, "sebelum 99 entri DIAJUKAN masuk, semua sah")
		assert.Zero(t, kartuByID(t, kpis, "waspada").Nilai)
		assert.Len(t, kartuByID(t, kpis, "entri").Tren, 14)
		assert.Equal(t, 4.0, kartuByID(t, kpis, "entri").Tren[13])
	})

	t.Run("delta mengikuti definisi", func(t *testing.T) {
		kpis := s.kpiDari(t, emailBerhak, "")
		// entri: H-6..H = 6 x 4 + 103 = 127 terhadap H-13..H-7 = 2 x 4 = 8 (hanya H-8 dan H-7 yang berentri).
		assert.Equal(t, bulat((127.0-8.0)/8.0*100), kartuByID(t, kpis, "entri").Delta)
		assert.Equal(t, 2.0, kartuByID(t, kpis, "waspada").Delta, "dari 0 kemarin menjadi 2")
		assert.Zero(t, kartuByID(t, kpis, "cs").Delta, "lot sudah berlaku sejak H-10, jadi sepekan lalu (H-7) sama dengan hari ini")
	})
}

// ── Heatmap ───────────────────────────────────────────────────────────

type selHeat struct {
	komoditas, kabupaten string
	delta, harga         float64
}

func sel(h petaPanas) []selHeat {
	out := make([]selHeat, len(h.Sel))
	for i, c := range h.Sel {
		out[i] = selHeat{c.KomoditasKode, c.KabupatenKode, c.Delta, c.Harga}
	}
	return out
}

// heatmapDariAgregat menghitung sel lapangan langsung dari agregat_harga_harian untuk tanggal acuan (rumus
// mv_heatmap_harga): harga terakhir dalam 3 hari sampai acuan terhadap rata-rata tertimbang 7 hari sebelum tanggal itu.
func (s *suite) heatmapDariAgregat(t *testing.T, kab, kom string, acuan time.Time) []selHeat {
	t.Helper()
	var rows []struct {
		KomoditasKode string
		KabupatenKode string
		Delta         float64
		Harga         float64
	}
	require.NoError(t, s.db.Raw(`
		WITH terakhir AS (
		    SELECT DISTINCT ON (komoditas_kode, kabupaten_kode) komoditas_kode, kabupaten_kode, tanggal, rata_rata
		      FROM agregat_harga_harian
		     WHERE tipe = 'KONSUMEN' AND tanggal BETWEEN ?::date - 3 AND ?::date
		       AND (?::text = '' OR kabupaten_kode = ?) AND (?::text = '' OR komoditas_kode = ?)
		     ORDER BY komoditas_kode, kabupaten_kode, tanggal DESC
		)
		SELECT t.komoditas_kode, t.kabupaten_kode,
		       ROUND((t.rata_rata - b.basis) / b.basis * 100, 1)::float8 AS delta, t.rata_rata::float8 AS harga
		  FROM terakhir t
		  CROSS JOIN LATERAL (
		        SELECT SUM(k.rata_rata * k.jumlah) / SUM(k.jumlah) AS basis
		          FROM agregat_harga_harian k
		         WHERE k.tipe = 'KONSUMEN' AND k.komoditas_kode = t.komoditas_kode AND k.kabupaten_kode = t.kabupaten_kode
		           AND k.tanggal >= t.tanggal - 7 AND k.tanggal < t.tanggal) b
		 WHERE b.basis IS NOT NULL
		 ORDER BY t.kabupaten_kode, t.komoditas_kode`,
		tanggal(acuan), tanggal(acuan), kab, kab, kom, kom).Scan(&rows).Error)
	out := make([]selHeat, len(rows))
	for i, r := range rows {
		out[i] = selHeat{r.KomoditasKode, r.KabupatenKode, r.Delta, r.Harga}
	}
	return out
}

func TestHeatmapFilter_SesuaiHitunganDariAgregat(t *testing.T) {
	s := sharedSuite(t)
	hari := hariIni()
	tabel := []struct {
		nama     string
		kab, kom string
		sampai   time.Time
	}{
		{"kabupaten", kabBanda, "", hari},
		{"komoditas", "", "CBM-KRT", hari},
		{"kabupaten dan komoditas", kabSelatan, "TMT", hari},
		{"tanggal acuan lima hari lalu", "", "", hari.AddDate(0, 0, -5)},
		{"tanggal acuan 20 hari lalu dengan filter", kabBanda, "CBM-KRT", hari.AddDate(0, 0, -20)},
		{"tanggal acuan 60 hari lalu", "", "", hari.AddDate(0, 0, -60)},
		{"kode yang tidak ada", "1101", "", hari},
	}
	for _, c := range tabel {
		t.Run(c.nama, func(t *testing.T) {
			got := sel(s.heatmapDari(t, emailBerhak, query(c.kab, c.kom, time.Time{}, c.sampai)))
			want := s.heatmapDariAgregat(t, c.kab, c.kom, c.sampai)
			if len(want) == 0 {
				assert.Empty(t, got)
				return
			}
			assert.Equal(t, want, got)
		})
	}
}

// Jalur bawaan (dari mv_heatmap_harga) dan jalur langsung untuk hari ini menghasilkan sel yang sama, selama view
// baru disegarkan.
func TestHeatmapFilter_JalurViewSamaDenganJalurLangsungHariIni(t *testing.T) {
	s := sharedSuite(t)
	hari := hariIni()

	bawaan := s.heatmapDari(t, emailBerhak, "")
	langsung := s.heatmapDari(t, emailBerhak, "?sampai="+tanggal(hari))

	require.NotEmpty(t, bawaan.Sel)
	assert.Equal(t, sel(bawaan), sel(langsung))
	assert.Equal(t, len(bawaan.Kabupaten), len(langsung.Kabupaten))
	assert.Equal(t, len(bawaan.Komoditas), len(langsung.Komoditas))

	for _, kab := range []string{kabBanda, kabSelatan} {
		assert.Equal(t, sel(s.heatmapDari(t, emailBerhak, "?kabupaten_kode="+kab)),
			sel(s.heatmapDari(t, emailBerhak, "?sampai="+tanggal(hari)+"&kabupaten_kode="+kab)), "kabupaten %s", kab)
	}
}

func TestHeatmapFilter_DaftarKabupatenDanKomoditasMengikutiSel(t *testing.T) {
	s := sharedSuite(t)

	h := s.heatmapDari(t, emailBerhak, "?kabupaten_kode="+kabBanda+"&komoditas_kode=CBM-KRT")

	require.Len(t, h.Sel, 1)
	require.Len(t, h.Kabupaten, 1)
	assert.Equal(t, kabBanda, h.Kabupaten[0].Kode)
	assert.NotEmpty(t, h.Kabupaten[0].Nama)
	require.Len(t, h.Komoditas, 1)
	assert.Equal(t, "CBM-KRT", h.Komoditas[0].Kode)
	assert.True(t, h.Komoditas[0].Pilot)
	assert.Equal(t, kabBanda, h.Sel[0].KabupatenKode)
}

func TestHeatmapFilter_DariMembatasiTanggalHargaTerakhir(t *testing.T) {
	s, h := kpiTerkendali(t, "dashboard_heatmap_terkendali")

	// Harga terakhir semua sel jatuh pada H. Dari H: tetap ada; dari H berikutnya ditolak validasi.
	semua := s.heatmapDari(t, emailBerhak, "?dari="+tanggal(h))
	assert.Len(t, semua.Sel, 4, "TMT/1171, CBM-KRT/1171, DAG-AYM/1171, CBM-KRT/1108 punya harga pada H")

	// Acuan kemarin tanpa dari: harga terakhir kemarin; dengan dari = acuan masih sama.
	kemarin := h.AddDate(0, 0, -1)
	a := s.heatmapDari(t, emailBerhak, "?sampai="+tanggal(kemarin))
	b := s.heatmapDari(t, emailBerhak, "?dari="+tanggal(kemarin)+"&sampai="+tanggal(kemarin))
	assert.Equal(t, sel(a), sel(b))
	assert.Len(t, a.Sel, 4)

	// Acuan H dengan dari = H: sel yang harga terakhirnya H-1 gugur. Tidak ada sel seperti itu di data ini, jadi
	// buat satu: harga CBR-RWT hanya sampai H-2.
	for d := 9; d >= 2; d-- {
		s.sisipEntri(t, h.AddDate(0, 0, -d), pasarBanda, "CBR-RWT", "DIPUBLIKASI", 70000+d, 1)
	}
	s.segarkan(t)
	tanpaDari := s.heatmapDari(t, emailBerhak, "?sampai="+tanggal(h))
	denganDari := s.heatmapDari(t, emailBerhak, "?dari="+tanggal(h)+"&sampai="+tanggal(h))
	assert.Len(t, tanpaDari.Sel, 5, "CBR-RWT ikut karena harga terakhirnya H-2 masih dalam jendela 3 hari")
	assert.Len(t, denganDari.Sel, 4, "dari = H membuang sel yang harga terakhirnya lebih lama")
	for _, c := range denganDari.Sel {
		assert.NotEqual(t, "CBR-RWT", c.KomoditasKode)
	}
	// Dari yang lebih awal dari jendela tidak melebarkan jendela 3 hari.
	lebar := s.heatmapDari(t, emailBerhak, "?dari="+tanggal(h.AddDate(0, 0, -30))+"&sampai="+tanggal(h))
	assert.Equal(t, sel(tanpaDari), sel(lebar))
}

// Sel eksternal (hasil ingestion) mengikuti tanggal acuan dan filter, dan tidak menggantikan sel lapangan.
func TestHeatmapFilter_SelEksternalMengikutiTanggalAcuanDanFilter(t *testing.T) {
	s := newSuite(t, "dashboard_heatmap_eksternal_filter", true)
	h := hariIni()

	// Harga eksternal Simeulue (1101), DAG-SPI: sepekan di 150.000 (H-8..H-2), lalu 160.000 pada H-1. Dan Aceh Singkil
	// (1102), CBM-KRT: datar 90.000 H-8..H-1.
	require.NoError(t, s.db.Exec(`
		INSERT INTO harga_eksternal (sumber_id, komoditas_kode, kabupaten_kode, tanggal, tipe, harga, satuan)
		SELECT (SELECT id FROM sumber_data WHERE kode = 'SRC-BI'), 'DAG-SPI', '1101', d::date, 'KONSUMEN',
		       CASE WHEN d::date = ?::date - 1 THEN 160000 ELSE 150000 END, 'kg'
		  FROM generate_series(?::date - 8, ?::date - 1, interval '1 day') d`, tanggal(h), tanggal(h), tanggal(h)).Error)
	require.NoError(t, s.db.Exec(`
		INSERT INTO harga_eksternal (sumber_id, komoditas_kode, kabupaten_kode, tanggal, tipe, harga, satuan)
		SELECT (SELECT id FROM sumber_data WHERE kode = 'SRC-BI'), 'CBM-KRT', '1102', d::date, 'KONSUMEN', 90000, 'kg'
		  FROM generate_series(?::date - 8, ?::date - 1, interval '1 day') d`, tanggal(h), tanggal(h)).Error)
	s.segarkan(t)

	eksternal := func(hm petaPanas) map[string]selHeat {
		out := map[string]selHeat{}
		for _, c := range hm.Sel {
			if c.Sumber == "EKSTERNAL" {
				out[c.KomoditasKode+"/"+c.KabupatenKode] = selHeat{c.KomoditasKode, c.KabupatenKode, c.Delta, c.Harga}
			}
		}
		return out
	}

	t.Run("bawaan: harga terakhir kemarin", func(t *testing.T) {
		e := eksternal(s.heatmapDari(t, emailBerhak, ""))
		require.Contains(t, e, "DAG-SPI/1101")
		assert.Equal(t, 160000.0, e["DAG-SPI/1101"].harga)
		assert.InDelta(t, 6.7, e["DAG-SPI/1101"].delta, 0.001)
		assert.Contains(t, e, "CBM-KRT/1102")
	})
	t.Run("tanggal acuan hari ini sama dengan bawaan", func(t *testing.T) {
		assert.Equal(t, eksternal(s.heatmapDari(t, emailBerhak, "")), eksternal(s.heatmapDari(t, emailBerhak, "?sampai="+tanggal(h))))
	})
	t.Run("tanggal acuan H-3: harga terakhir ikut mundur", func(t *testing.T) {
		e := eksternal(s.heatmapDari(t, emailBerhak, "?sampai="+tanggal(h.AddDate(0, 0, -3))))
		require.Contains(t, e, "DAG-SPI/1101")
		assert.Equal(t, 150000.0, e["DAG-SPI/1101"].harga, "harga 160.000 pada H-1 belum ada pada H-3")
		assert.Zero(t, e["DAG-SPI/1101"].delta)
	})
	t.Run("tanggal acuan sebelum data eksternal ada: sel hilang", func(t *testing.T) {
		assert.Empty(t, eksternal(s.heatmapDari(t, emailBerhak, "?sampai="+tanggal(h.AddDate(0, 0, -12)))))
	})
	t.Run("filter kabupaten", func(t *testing.T) {
		e := eksternal(s.heatmapDari(t, emailBerhak, "?kabupaten_kode=1101"))
		assert.Len(t, e, 1)
		assert.Contains(t, e, "DAG-SPI/1101")
		assert.Empty(t, eksternal(s.heatmapDari(t, emailBerhak, "?kabupaten_kode="+kabBanda)), "Banda Aceh sudah punya sel lapangan")
	})
	t.Run("filter komoditas", func(t *testing.T) {
		e := eksternal(s.heatmapDari(t, emailBerhak, "?komoditas_kode=CBM-KRT"))
		assert.Len(t, e, 1)
		assert.Contains(t, e, "CBM-KRT/1102")
	})
	t.Run("sel lapangan tidak digantikan eksternal pada tanggal acuan lampau", func(t *testing.T) {
		// Harga eksternal untuk sel yang punya data lapangan pada H-5: tidak boleh menggantikan sel lapangan.
		require.NoError(t, s.db.Exec(`
			INSERT INTO harga_eksternal (sumber_id, komoditas_kode, kabupaten_kode, tanggal, tipe, harga, satuan)
			SELECT (SELECT id FROM sumber_data WHERE kode = 'SRC-BI'), 'CBM-KRT', '1171', d::date, 'KONSUMEN', 1000, 'kg'
			  FROM generate_series(?::date - 12, ?::date - 1, interval '1 day') d`, tanggal(h), tanggal(h)).Error)
		s.segarkan(t)
		hm := s.heatmapDari(t, emailBerhak, "?sampai="+tanggal(h.AddDate(0, 0, -5))+"&kabupaten_kode="+kabBanda+"&komoditas_kode=CBM-KRT")
		require.Len(t, hm.Sel, 1)
		assert.Equal(t, "LAPANGAN", hm.Sel[0].Sumber)
		assert.Greater(t, hm.Sel[0].Harga, 1000.0)
	})
}

// ── Validasi parameter lewat HTTP ─────────────────────────────────────

func TestFilter_ParameterTidakValidDijawab400(t *testing.T) {
	s := sharedSuite(t)
	hari := hariIni()
	cases := []struct {
		query string
		pesan string
	}{
		{"?kabupaten_kode=abc", "kabupaten_kode"},
		{"?kabupaten_kode=117", "kabupaten_kode"},
		{"?komoditas_kode=cabai%20merah", "komoditas_kode"},
		{"?dari=kemarin", "dari harus berformat YYYY-MM-DD"},
		{"?sampai=10-11-2026", "sampai harus berformat YYYY-MM-DD"},
		{"?sampai=2026-02-30", "sampai harus berformat YYYY-MM-DD"},
		{"?sampai=" + tanggal(hari.AddDate(0, 0, 1)), "sampai tidak boleh setelah hari ini"},
		{"?dari=" + tanggal(hari.AddDate(0, 0, 1)), "dari tidak boleh setelah hari ini"},
		{"?dari=" + tanggal(hari) + "&sampai=" + tanggal(hari.AddDate(0, 0, -1)), "dari tidak boleh setelah sampai"},
		{"?dari=" + tanggal(hari.AddDate(0, 0, -90)) + "&sampai=" + tanggal(hari), "paling lama 90 hari"},
		{"?dari=2020-01-01&sampai=2020-01-31", "di luar riwayat dashboard"},
	}
	for _, jalur := range []string{"kpi", "heatmap"} {
		for _, c := range cases {
			w := s.call(t, http.MethodGet, "/v1/dashboard/operasional/"+jalur+c.query, emailBerhak, nil)
			require.Equal(t, http.StatusBadRequest, w.Code, "%s%s: %s", jalur, c.query, w.Body.String())
			assert.Contains(t, w.Body.String(), c.pesan, jalur+c.query)
		}
	}
}

func TestFilter_BatasValidTepat(t *testing.T) {
	s := sharedSuite(t)
	hari := hariIni()

	for _, q := range []string{
		"?dari=" + tanggal(hari.AddDate(0, 0, -89)) + "&sampai=" + tanggal(hari), // 90 hari, tepat di awal riwayat
		"?dari=" + tanggal(hari) + "&sampai=" + tanggal(hari),                    // satu hari
		"?sampai=" + tanggal(hari),                                               // sampai hari ini
		"?sampai=" + tanggal(hari.AddDate(0, 0, -89)),                            // acuan di awal riwayat, dari dipangkas
		"?kabupaten_kode=1101&komoditas_kode=CBM-KRT&sampai=" + tanggal(hari),    // kode ada tetapi tak berdata
		"?komoditas_kode=ABCDEFGHIJKLMNOPQRSTUVWXYZABC",                          // 29 karakter, kode yang tak ada
	} {
		for _, jalur := range []string{"kpi", "heatmap"} {
			w := s.call(t, http.MethodGet, "/v1/dashboard/operasional/"+jalur+q, emailBerhak, nil)
			assert.Equal(t, http.StatusOK, w.Code, "%s%s: %s", jalur, q, w.Body.String())
		}
	}
	// Periode yang dipangkas ke awal riwayat tetap menghasilkan tren (satu hari) dengan nilai yang benar.
	kpis := s.kpiDari(t, emailBerhak, "?sampai="+tanggal(hari.AddDate(0, 0, -89)))
	assert.Len(t, kartuByID(t, kpis, "entri").Tren, 1)
	samaDenganTabelDasar(t, "awal riwayat", kpis, s.dariTabelDasar(t, "", "", hari.AddDate(0, 0, -89), hari.AddDate(0, 0, -89)))
}

func TestFilter_KodeTidakAdaMenghasilkanKosongBukanGalat(t *testing.T) {
	s := sharedSuite(t)

	kpis := s.kpiDari(t, emailBerhak, "?kabupaten_kode=9999&komoditas_kode=TIDAK-ADA")
	require.Len(t, kpis, 4)
	for _, k := range kpis {
		assert.Zero(t, k.Nilai, k.ID)
	}
	h := s.heatmapDari(t, emailBerhak, "?kabupaten_kode=9999&komoditas_kode=TIDAK-ADA")
	assert.Empty(t, h.Sel)
	assert.Empty(t, h.Kabupaten)
	assert.Empty(t, h.Komoditas)
}

func TestFilter_DatabaseKosongTidakGalat(t *testing.T) {
	s := newSuite(t, "dashboard_filter_kosong", false)
	hari := hariIni()

	for _, q := range []string{
		"", "?kabupaten_kode=1171", "?komoditas_kode=CBM-KRT", "?kabupaten_kode=1171&komoditas_kode=CBM-KRT",
		query("", "", hari.AddDate(0, 0, -29), hari), query(kabBanda, "TMT", hari.AddDate(0, 0, -5), hari.AddDate(0, 0, -2)),
	} {
		kpis := s.kpiDari(t, emailBerhak, q)
		require.Len(t, kpis, 4, q)
		for _, k := range kpis {
			assert.Zero(t, k.Nilai, "%s %s", q, k.ID)
			assert.Zero(t, k.Delta, "%s %s", q, k.ID)
		}
		h := s.heatmapDari(t, emailBerhak, q)
		assert.Empty(t, h.Sel, q)
	}
}
