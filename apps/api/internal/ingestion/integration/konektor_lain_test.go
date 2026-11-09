package integration

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/dashboard/refresher"
	"github.com/garda-inflasi/api/internal/ingestion"
	"github.com/garda-inflasi/api/internal/ingestion/connector/bapanas"
	"github.com/garda-inflasi/api/internal/ingestion/connector/bmkg"
	"github.com/garda-inflasi/api/internal/ingestion/connector/bps"
	"github.com/garda-inflasi/api/internal/ingestion/connector/openweather"
	"github.com/garda-inflasi/api/internal/ingestion/connector/pihps"
	"github.com/garda-inflasi/api/internal/ingestion/domain"
	"github.com/garda-inflasi/api/internal/ingestion/engine"
	sumber "github.com/garda-inflasi/api/internal/sumber-data/domain"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (s *setup) konektor() []domain.Konektor {
	return []domain.Konektor{
		pihps.New(),
		bmkg.New(),
		bps.New("kunci-uji"),
		openweather.New("kunci-owm"),
		bapanas.New(),
	}
}

// runnerSemua adalah Runner dengan kelima konektor di atas Fetcher fixture yang sama.
func (s *setup) runnerSemua() *engine.Runner {
	r := engine.New(s.repo, s.fetcher, logger.NewDefault(), s.konektor()...)
	r.Sekarang = func() time.Time { return time.Date(2025, 10, 5, 3, 0, 0, 0, time.UTC) }
	r.PadaEksekusiSelesai = func(ctx context.Context, e *domain.EksekusiSumber, sd *sumber.SumberData) {
		ingestion.PeriksaAlertKegagalan(ctx, s.db, logger.NewDefault(), e, sd)
	}
	return r
}

func (s *setup) jalankanSumber(t *testing.T, r *engine.Runner, id string) *domain.EksekusiSumber {
	t.Helper()
	run, err := r.Mulai(t.Context(), id, domain.PemicuCLI, nil)
	require.NoError(t, err)
	e, err := run.Jalankan(t.Context())
	require.NoError(t, err)
	return e
}

func TestBMKG_CuacaHarianIdempotenDanDiperbarui(t *testing.T) {
	s := newSetup(t, "ingest_bmkg")
	// Satu titik pantau (Banda Aceh) agar fixture cukup.
	require.NoError(t, s.db.Exec(`DELETE FROM pemetaan_sumber WHERE sumber_id = (SELECT id FROM sumber_data WHERE kode = 'SRC-BMKG') AND kode_master <> '1171'`).Error)
	grid := fixture(t, "bmkg/testdata/prakiraan_banda_aceh.json")
	s.fetcher.Respons["wilayah=11.71.01.2001"] = grid
	r := s.runnerSemua()

	e := s.jalankanSumber(t, r, bmkg.Kode)

	assert.Equal(t, domain.StatusSukses, e.Status)
	assert.Equal(t, 2, e.BarisDiambil)
	assert.Equal(t, 1, e.BarisTidakValid, "hari prakiraan parsial dilaporkan")
	assert.EqualValues(t, 2, s.hitung(t, "cuaca_harian"))

	var lebat struct {
		CurahHujanMm float64
		Anomali      bool
		EksekusiID   *idx.ID
		DataMentahID *idx.ID
	}
	require.NoError(t, s.db.Raw(`SELECT curah_hujan_mm, anomali, eksekusi_id, data_mentah_id FROM cuaca_harian WHERE tanggal = '2025-10-16'`).Scan(&lebat).Error)
	assert.Equal(t, 60.0, lebat.CurahHujanMm)
	assert.True(t, lebat.Anomali)
	require.NotNil(t, lebat.EksekusiID)
	assert.Equal(t, e.ID, *lebat.EksekusiID)
	require.NotNil(t, lebat.DataMentahID)

	// Prakiraan diperbarui: nilai baru menggantikan, jumlah baris tetap.
	s.fetcher.Respons["wilayah=11.71.01.2001"] = []byte(strings.ReplaceAll(string(grid), `"tp": 15.5`, `"tp": 0`))
	e2 := s.jalankanSumber(t, r, bmkg.Kode)
	assert.Equal(t, domain.StatusSukses, e2.Status)
	assert.EqualValues(t, 2, s.hitung(t, "cuaca_harian"))
	require.NoError(t, s.db.Raw(`SELECT curah_hujan_mm, anomali, eksekusi_id, data_mentah_id FROM cuaca_harian WHERE tanggal = '2025-10-16'`).Scan(&lebat).Error)
	assert.InDelta(t, 44.5, lebat.CurahHujanMm, 1e-9)
	assert.False(t, lebat.Anomali)
	assert.Equal(t, e2.ID, *lebat.EksekusiID)
}

func konfigurasiBPS() string {
	return `{"domain":"1100","tahun_surut":0,"variabel":{"ihk":"10","inflasi_mtm":"11","inflasi_yoy":"12","inflasi_ytd":"13","andil":"20","andil_inflasi_pct":"21"}}`
}

func (s *setup) siapkanBPS(t *testing.T) {
	t.Helper()
	require.NoError(t, s.db.Exec(`UPDATE sumber_data SET konfigurasi = ? WHERE kode = 'SRC-BPS'`, konfigurasiBPS()).Error)
	for _, nama := range []string{"ihk", "inflasi_mtm", "inflasi_yoy", "inflasi_ytd", "andil", "andil_inflasi_pct"} {
		s.fetcher.Respons[fmt.Sprintf("var=%s&th=2025", nama)] = fixture(t, "bps/testdata/"+nama+"_2025.json")
	}
}

func TestBPS_InflasiResmiMenggantikanContohTanpaMenimpaSumberLain(t *testing.T) {
	s := newSetup(t, "ingest_bps")
	s.siapkanBPS(t)
	// Baris contoh pada bulan yang sama dengan data BPS, dan angka resmi sumber lain yang harus dibiarkan.
	require.NoError(t, s.db.Exec(`
		INSERT INTO inflasi_bulanan (wilayah_kode, periode, ihk, inflasi_mtm, inflasi_yoy, inflasi_ytd, sumber) VALUES
		('11', '2025-09-01', 100, 9, 9, 9, 'DEMO'),
		('1171', '2025-09-01', 100, 5, 5, 5, 'BI'),
		('11', '2025-01-01', 100, 1, 1, 1, 'DEMO')`).Error)
	require.NoError(t, s.db.Exec(`
		INSERT INTO andil_inflasi (periode, wilayah_kode, komoditas_kode, andil, inflasi_pct, sumber) VALUES
		('2025-09-01', '11', 'CBM-KRT', 9, 9, 'DEMO')`).Error)
	r := s.runnerSemua()

	e := s.jalankanSumber(t, r, bps.Kode)

	assert.Equal(t, domain.StatusSukses, e.Status, "%v", e.Galat)
	assert.Equal(t, 6, e.Dokumen)
	assert.Equal(t, 5, e.BarisDiambil, "3 inflasi + 2 andil")

	type baris struct {
		Wilayah, Sumber string
		Mtm, Yoy        float64
	}
	var sep baris
	require.NoError(t, s.db.Raw(`SELECT wilayah_kode AS wilayah, sumber, inflasi_mtm AS mtm, inflasi_yoy AS yoy FROM inflasi_bulanan WHERE wilayah_kode = '11' AND periode = '2025-09-01'`).Scan(&sep).Error)
	assert.Equal(t, baris{Wilayah: "11", Sumber: "BPS", Mtm: 0.4, Yoy: 2.84}, sep, "angka resmi menggantikan baris contoh")

	var banda baris
	require.NoError(t, s.db.Raw(`SELECT wilayah_kode AS wilayah, sumber, inflasi_mtm AS mtm, inflasi_yoy AS yoy FROM inflasi_bulanan WHERE wilayah_kode = '1171' AND periode = '2025-09-01'`).Scan(&banda).Error)
	assert.Equal(t, baris{Wilayah: "1171", Sumber: "BI", Mtm: 5, Yoy: 5}, banda, "angka resmi sumber lain tidak ditimpa")

	var demo int64
	require.NoError(t, s.db.Raw(`SELECT COUNT(*) FROM inflasi_bulanan WHERE sumber = 'DEMO'`).Scan(&demo).Error)
	assert.EqualValues(t, 1, demo, "baris contoh bulan lain tetap berlabel contoh")

	var andil struct {
		Sumber string
		Andil  float64
	}
	require.NoError(t, s.db.Raw(`SELECT sumber, andil FROM andil_inflasi WHERE komoditas_kode = 'CBM-KRT' AND periode = '2025-09-01'`).Scan(&andil).Error)
	assert.Equal(t, "BPS", andil.Sumber)
	assert.Equal(t, 0.18, andil.Andil)

	// Jalan ulang: idempoten.
	s.jalankanSumber(t, r, bps.Kode)
	var n int64
	require.NoError(t, s.db.Raw(`SELECT COUNT(*) FROM inflasi_bulanan WHERE sumber = 'BPS'`).Scan(&n).Error)
	assert.EqualValues(t, 2, n, "Agustus dan September wilayah 11; Banda Aceh tetap milik BI")
	assert.EqualValues(t, 6, s.hitung(t, "data_mentah"), "payload identik tidak menggandakan bronze")
}

func TestBPS_TanpaKonfigurasiGagalDenganPesanJelas(t *testing.T) {
	s := newSetup(t, "ingest_bps_cfg")
	r := s.runnerSemua()

	e := s.jalankanSumber(t, r, bps.Kode)

	assert.Equal(t, domain.StatusGagal, e.Status)
	require.NotNil(t, e.Galat)
	assert.Contains(t, *e.Galat, "ihk, inflasi_mtm, inflasi_yoy, inflasi_ytd")
	assert.Empty(t, s.fetcher.Dipanggil())
}

func TestBPS_KunciAPITidakBocorKeRiwayat(t *testing.T) {
	s := newSetup(t, "ingest_bps_kunci")
	s.siapkanBPS(t)
	r := engine.New(s.repo, s.fetcher, logger.NewDefault(), bps.New("RAHASIA-API"))
	r.Sekarang = func() time.Time { return time.Date(2025, 10, 5, 3, 0, 0, 0, time.UTC) }

	s.jalankanSumber(t, r, bps.Kode)

	var bocor int64
	require.NoError(t, s.db.Raw(`SELECT (SELECT COUNT(*) FROM data_mentah WHERE kunci_permintaan LIKE '%RAHASIA%' OR payload::text LIKE '%RAHASIA%')
		+ (SELECT COUNT(*) FROM eksekusi_sumber WHERE galat LIKE '%RAHASIA%' OR ringkasan::text LIKE '%RAHASIA%')`).Scan(&bocor).Error)
	assert.Zero(t, bocor)
}

func TestGold_MembandingkanHargaLapanganDenganEksternal(t *testing.T) {
	s := newSetup(t, "ingest_gold")
	s.jalankan(t) // PIHPS: BRS-MED Kota Banda Aceh 2025-10-13 = 13.900
	require.NoError(t, s.db.Exec(`
		INSERT INTO entri_harga (kode, komoditas_kode, pasar_id, tipe, harga, satuan, status, dicatat_pada)
		VALUES ('EH-GOLD-1', 'BRS-MED', 'PSR-1171-01', 'KONSUMEN', 15000, 'kg', 'DIVALIDASI', '2025-10-13 05:00:00+00'),
		       ('EH-GOLD-2', 'CBM-KRT', 'PSR-1171-01', 'KONSUMEN', 70000, 'kg', 'DIVALIDASI', '2025-10-20 05:00:00+00')`).Error)

	require.NoError(t, refresher.Shared(s.db, logger.NewDefault()).Refresh(t.Context()))

	var gold []struct {
		KomoditasKode  string
		Tanggal        string
		HargaLapangan  *float64
		HargaEksternal *float64
		SelisihPct     *float64
	}
	require.NoError(t, s.db.Raw(`
		SELECT komoditas_kode, tanggal::text, harga_lapangan::float8 AS harga_lapangan, harga_eksternal::float8 AS harga_eksternal, selisih_pct::float8 AS selisih_pct
		  FROM gold_harga_harian WHERE kabupaten_kode = '1171' AND komoditas_kode IN ('BRS-MED', 'CBM-KRT', 'BRS-PRM') ORDER BY tanggal, komoditas_kode`).Scan(&gold).Error)

	by := map[string]int{}
	for i, g := range gold {
		by[g.KomoditasKode+"@"+g.Tanggal] = i
	}
	pasangan := gold[by["BRS-MED@2025-10-13"]]
	require.NotNil(t, pasangan.HargaLapangan)
	require.NotNil(t, pasangan.HargaEksternal)
	assert.Equal(t, 15000.0, *pasangan.HargaLapangan)
	assert.Equal(t, 13900.0, *pasangan.HargaEksternal)
	require.NotNil(t, pasangan.SelisihPct)
	assert.InDelta(t, 7.9, *pasangan.SelisihPct, 0.001, "(15000-13900)/13900")

	hanyaLapangan := gold[by["CBM-KRT@2025-10-20"]]
	assert.NotNil(t, hanyaLapangan.HargaLapangan)
	assert.Nil(t, hanyaLapangan.HargaEksternal)
	assert.Nil(t, hanyaLapangan.SelisihPct, "tanpa pembanding tidak ada selisih")

	hanyaEksternal := gold[by["BRS-PRM@2025-10-13"]]
	assert.Nil(t, hanyaEksternal.HargaLapangan)
	assert.NotNil(t, hanyaEksternal.HargaEksternal)
	assert.Nil(t, hanyaEksternal.SelisihPct)
}

func TestOpenWeather_CuacaHarian(t *testing.T) {
	s := newSetup(t, "ingest_owm")
	require.NoError(t, s.db.Exec(`DELETE FROM pemetaan_sumber WHERE sumber_id = (SELECT id FROM sumber_data WHERE kode = 'SRC-OWM') AND alias <> 'banda aceh'`).Error)
	grid := fixture(t, "openweather/testdata/cuaca_banda_aceh.json")
	s.fetcher.Respons["wilayah=banda aceh"] = grid
	r := s.runnerSemua()

	e := s.jalankanSumber(t, r, openweather.Kode)
	assert.Equal(t, domain.StatusSukses, e.Status)
	assert.Equal(t, 1, e.BarisDiambil)

	var cuaca struct {
		KabupatenKode string
		SuhuC         float64
		CurahHujanMm  float64
		Sumber        string
	}
	require.NoError(t, s.db.Raw(`SELECT kabupaten_kode, suhu_c, curah_hujan_mm, sumber FROM cuaca_harian WHERE sumber = 'OpenWeather'`).Scan(&cuaca).Error)
	assert.Equal(t, "1171", cuaca.KabupatenKode)
	assert.Equal(t, 28.5, cuaca.SuhuC)
	assert.Equal(t, 15.5, cuaca.CurahHujanMm)
	assert.Equal(t, "OpenWeather", cuaca.Sumber)
}

func TestBapanas_HargaEksternal(t *testing.T) {
	s := newSetup(t, "ingest_bapanas")
	grid := fixture(t, "bapanas/testdata/harga_bapanas_aceh.json")
	s.fetcher.Respons["provinsi=11"] = grid
	r := s.runnerSemua()

	e := s.jalankanSumber(t, r, bapanas.Kode)
	assert.Equal(t, domain.StatusSukses, e.Status)
	assert.GreaterOrEqual(t, e.BarisDiambil, 1)

	var cnt int64
	require.NoError(t, s.db.Raw(`SELECT count(*) FROM harga_eksternal WHERE sumber_id = (SELECT id FROM sumber_data WHERE kode = 'SRC-BAPANAS')`).Scan(&cnt).Error)
	assert.GreaterOrEqual(t, cnt, int64(1))
}

func TestIngestion_AlertEWSKegagalanBerulang(t *testing.T) {
	s := newSetup(t, "ingest_ews_alert")
	require.NoError(t, s.db.Exec(`DELETE FROM ews_alert WHERE dedup_key = 'ingestion:sumber:SRC-BI'`).Error)

	r := s.runnerSemua()
	// Buat 3 kali gagal berturut-turut
	s.fetcher.Galat = map[string]error{"wilayah=101": fmt.Errorf("layanan mati")}

	for i := 0; i < 3; i++ {
		run, err := r.Mulai(t.Context(), pihps.Kode, domain.PemicuCLI, nil)
		require.NoError(t, err)
		e, _ := run.Jalankan(t.Context())
		require.Equal(t, domain.StatusGagal, e.Status)
	}

	// Pastikan alert ews ANOMALI_DATA dibuat
	var alert struct {
		ID       idx.ID
		Severity string
		Jenis    string
		Status   string
		DedupKey string
	}
	err := s.db.Raw(`SELECT id, severity, jenis, status, dedup_key FROM ews_alert WHERE dedup_key = 'ingestion:sumber:SRC-BI' AND status = 'BARU'`).Scan(&alert).Error
	require.NoError(t, err)
	assert.Equal(t, "WASPADA", alert.Severity)
	assert.Equal(t, "ANOMALI_DATA", alert.Jenis)
	assert.Equal(t, "BARU", alert.Status)

	// Jalankan sekali sukses, pastikan alert otomatis diselesaikan (SELESAI)
	s.fetcher.Galat = nil
	s.fetcher.Respons["wilayah=101"] = fixture(t, "pihps/testdata/grid_kota.json")
	run, err := r.Mulai(t.Context(), pihps.Kode, domain.PemicuCLI, nil)
	require.NoError(t, err)
	e, err := run.Jalankan(t.Context())
	require.NoError(t, err)
	require.Equal(t, domain.StatusSukses, e.Status)

	var status string
	require.NoError(t, s.db.Raw(`SELECT status FROM ews_alert WHERE id = ?`, alert.ID).Scan(&status).Error)
	assert.Equal(t, "SELESAI", status)
}
