// Package integration menguji engine ingestion terhadap PostgreSQL sungguhan
// dengan Fetcher palsu berisi fixture (tanpa internet).
package integration

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/ingestion/connector/pihps"
	"github.com/garda-inflasi/api/internal/ingestion/domain"
	"github.com/garda-inflasi/api/internal/ingestion/engine"
	"github.com/garda-inflasi/api/internal/ingestion/ingestiontest"
	"github.com/garda-inflasi/api/internal/ingestion/repository/postgres"
	ingestionseeder "github.com/garda-inflasi/api/internal/ingestion/seeder"
	masterseeder "github.com/garda-inflasi/api/internal/master/seeder"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	sumberseeder "github.com/garda-inflasi/api/internal/sumber-data/seeder"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var pg *pgtest.Server

func TestMain(m *testing.M) {
	os.Exit(pgtest.Main(m, &pg))
}

const kunciPIHPS = "wilayah=101"

type setup struct {
	db      *gorm.DB
	fetcher *ingestiontest.Fetcher
	runner  *engine.Runner
	repo    *postgres.Repository
}

func fixture(t *testing.T, nama string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "connector", nama))
	require.NoError(t, err)
	return raw
}

// newSetup menyiapkan basis data bermigrasi dengan master, katalog sumber, pemetaan referensi,
// satu wilayah PIHPS (regency 101 = Kota Banda Aceh), dan konfigurasi PIHPS minimal.
func newSetup(t *testing.T, name string) *setup {
	t.Helper()
	db := pg.NewDatabase(t, name)
	pgtest.MigrateUp(t, db)
	for _, seed := range []func() error{
		func() error { _, err := masterseeder.SeedReference(t.Context(), db); return err },
		func() error { _, err := sumberseeder.SeedReference(t.Context(), db); return err },
		func() error { _, err := ingestionseeder.SeedReference(t.Context(), db); return err },
	} {
		require.NoError(t, seed())
	}
	require.NoError(t, db.Exec(`UPDATE sumber_data SET konfigurasi = '{"province_id":"7"}' WHERE id = ?`, pihps.SumberID).Error)
	require.NoError(t, db.Exec(`INSERT INTO pemetaan_sumber (sumber_id, jenis, alias, kode_master) VALUES (?, 'WILAYAH', '101', '1171')`, pihps.SumberID).Error)

	fetcher := &ingestiontest.Fetcher{Respons: map[string][]byte{kunciPIHPS: fixture(t, "pihps/testdata/grid_kota.json")}}
	repo := postgres.New(db)
	return &setup{db: db, fetcher: fetcher, repo: repo, runner: engine.New(repo, fetcher, logger.NewDefault(), pihps.New())}
}

func (s *setup) jalankan(t *testing.T) *domain.EksekusiSumber {
	t.Helper()
	run, err := s.runner.Mulai(t.Context(), pihps.SumberID, domain.PemicuCLI, nil)
	require.NoError(t, err)
	e, err := run.Jalankan(t.Context())
	require.NoError(t, err)
	return e
}

func (s *setup) hitung(t *testing.T, tabel string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, s.db.Table(tabel).Count(&n).Error)
	return n
}

func TestRun_AlirkanPIHPSDariBronzeKeSilver(t *testing.T) {
	s := newSetup(t, "ingest_alir")

	e := s.jalankan(t)

	assert.Equal(t, domain.StatusSukses, e.Status)
	assert.Nil(t, e.Galat)
	assert.Equal(t, 1, e.Dokumen)
	assert.Equal(t, 14, e.BarisDiambil)
	assert.Equal(t, 3, e.BarisTakTerpetakan)
	assert.Equal(t, 4, e.BarisTidakValid)
	assert.Equal(t, 21, e.BarisDibaca, "dibaca = diambil + ditolak")
	require.NotNil(t, e.Selesai)
	require.NotNil(t, e.DurasiMs)
	require.NotNil(t, e.LatensiMs)

	// Bronze: payload asli tersimpan beserta checksum.
	assert.EqualValues(t, 1, s.hitung(t, "data_mentah"))
	var bronze domain.DataMentah
	require.NoError(t, s.db.First(&bronze).Error)
	assert.Equal(t, kunciPIHPS, bronze.KunciPermintaan)
	assert.Len(t, bronze.Checksum, 64)
	assert.JSONEq(t, string(fixture(t, "pihps/testdata/grid_kota.json")), bronze.Payload)

	// Silver: harga terstandar dengan lineage ke bronze dan eksekusi.
	assert.EqualValues(t, 14, s.hitung(t, "harga_eksternal"))
	var sampel domain.HargaEksternal
	require.NoError(t, s.db.First(&sampel, "komoditas_kode = 'BRS-MED' AND tanggal = '2025-10-13'").Error)
	assert.Equal(t, 13900.0, sampel.Harga)
	assert.Equal(t, "1171", sampel.KabupatenKode)
	assert.Equal(t, "kg", sampel.Satuan)
	require.NotNil(t, sampel.DataMentahID)
	assert.Equal(t, bronze.ID, *sampel.DataMentahID)
	require.NotNil(t, sampel.EksekusiID)
	assert.Equal(t, e.ID, *sampel.EksekusiID)

	// Ringkasan melaporkan alias yang tak terpetakan, bukan membuangnya diam-diam.
	var ringkasan struct {
		Permintaan    struct{ Berhasil, Gagal int }
		TakTerpetakan map[string]int `json:"tak_terpetakan"`
		TidakValid    map[string]int `json:"tidak_valid"`
	}
	var tersimpan domain.EksekusiSumber
	require.NoError(t, s.db.First(&tersimpan, e.ID).Error)
	require.NoError(t, json.Unmarshal([]byte(tersimpan.Ringkasan), &ringkasan))
	assert.Equal(t, map[string]int{"KOMODITAS:ikan segar baru": 3}, ringkasan.TakTerpetakan)
	assert.Equal(t, 3, ringkasan.TidakValid["harga kosong atau bukan angka"])
	assert.Equal(t, 7, tersimpan.BarisDitolak, "kolom turunan = tak terpetakan + tidak valid")
}

func TestRun_DuaKaliTidakMenggandakanData(t *testing.T) {
	s := newSetup(t, "ingest_idempoten")

	s.jalankan(t)
	pertama := s.hitung(t, "harga_eksternal")
	e2 := s.jalankan(t)

	assert.Equal(t, domain.StatusSukses, e2.Status)
	assert.Equal(t, pertama, s.hitung(t, "harga_eksternal"), "silver idempoten menurut kunci alami")
	assert.EqualValues(t, 1, s.hitung(t, "data_mentah"), "payload identik tidak menggandakan bronze")
	assert.EqualValues(t, 2, s.hitung(t, "eksekusi_sumber"), "tiap run tetap tercatat")

	// Lineage menunjuk eksekusi terakhir yang menulis baris itu.
	var sampel domain.HargaEksternal
	require.NoError(t, s.db.First(&sampel, "komoditas_kode = 'BRS-MED' AND tanggal = '2025-10-13'").Error)
	assert.Equal(t, e2.ID, *sampel.EksekusiID)
}

func TestRun_HargaBerubahDiperbaruiBukanDitambah(t *testing.T) {
	s := newSetup(t, "ingest_update")
	s.jalankan(t)

	baru := string(fixture(t, "pihps/testdata/grid_kota.json"))
	require.Contains(t, baru, `"13.900"`)
	baru = strings.Replace(baru, `"13.900"`, `"14.100"`, 1)
	s.fetcher.Respons[kunciPIHPS] = []byte(baru)
	s.jalankan(t)

	assert.EqualValues(t, 14, s.hitung(t, "harga_eksternal"))
	assert.EqualValues(t, 2, s.hitung(t, "data_mentah"), "payload berbeda disimpan sebagai bronze baru")
	var h domain.HargaEksternal
	require.NoError(t, s.db.First(&h, "komoditas_kode = 'BRS-MED' AND tanggal = '2025-10-13'").Error)
	assert.Equal(t, 14100.0, h.Harga)
}

func TestRun_StatistikSumberDihitungDariEksekusi(t *testing.T) {
	s := newSetup(t, "ingest_stat")

	var sumber struct {
		Status         string
		TerakhirJalan  *time.Time
		Keberhasilan7h *float64 `gorm:"column:keberhasilan_7h"`
		LatensiMs      int
	}
	baca := func() {
		require.NoError(t, s.db.Raw(`SELECT status::text AS status, terakhir_jalan, keberhasilan_7h, latensi_ms FROM sumber_data WHERE id = ?`, pihps.SumberID).Scan(&sumber).Error)
	}

	baca()
	assert.Equal(t, "BELUM_AKTIF", sumber.Status)
	assert.Nil(t, sumber.Keberhasilan7h)

	s.jalankan(t)
	baca()
	assert.Equal(t, "SEHAT", sumber.Status)
	require.NotNil(t, sumber.TerakhirJalan)
	require.NotNil(t, sumber.Keberhasilan7h)
	assert.InDelta(t, 100, *sumber.Keberhasilan7h, 0.001)

	s.fetcher.Galat = map[string]error{kunciPIHPS: errors.New("layanan mati")}
	e := s.jalankan(t)
	assert.Equal(t, domain.StatusGagal, e.Status)
	baca()
	assert.Equal(t, "GAGAL", sumber.Status)
	assert.InDelta(t, 50, *sumber.Keberhasilan7h, 0.001, "satu sukses dari dua run")
}

func TestRun_GagalDiambilTercatatDenganGalat(t *testing.T) {
	s := newSetup(t, "ingest_gagal")
	s.fetcher.Galat = map[string]error{kunciPIHPS: errors.New("GET https://x/y: status 503")}

	e := s.jalankan(t)

	assert.Equal(t, domain.StatusGagal, e.Status)
	require.NotNil(t, e.Galat)
	assert.Contains(t, *e.Galat, "wilayah=101")
	assert.Contains(t, *e.Galat, "503")
	assert.Zero(t, e.BarisDiambil)
	assert.Nil(t, e.LatensiMs, "tanpa permintaan berhasil tidak ada latensi")
	assert.Zero(t, s.hitung(t, "harga_eksternal"))
	assert.Zero(t, s.hitung(t, "data_mentah"))
}

func TestRun_SebagianBilaSebagianPermintaanGagal(t *testing.T) {
	s := newSetup(t, "ingest_sebagian")
	require.NoError(t, s.db.Exec(`INSERT INTO pemetaan_sumber (sumber_id, jenis, alias, kode_master) VALUES (?, 'WILAYAH', '102', '1174')`, pihps.SumberID).Error)
	s.fetcher.Galat = map[string]error{"wilayah=102": errors.New("timeout")}

	e := s.jalankan(t)

	assert.Equal(t, domain.StatusSebagian, e.Status)
	assert.Equal(t, 14, e.BarisDiambil, "wilayah yang berhasil tetap masuk")
	require.NotNil(t, e.Galat)
	assert.Contains(t, *e.Galat, "wilayah=102")
	assert.ElementsMatch(t, []string{"wilayah=101", "wilayah=102"}, s.fetcher.Dipanggil())

	var status string
	require.NoError(t, s.db.Raw(`SELECT status::text FROM sumber_data WHERE id = ?`, pihps.SumberID).Scan(&status).Error)
	assert.Equal(t, "TERTUNDA", status)
}

func TestRun_ResponsBukanJSONTidakMasukBronze(t *testing.T) {
	s := newSetup(t, "ingest_html")
	s.fetcher.Respons[kunciPIHPS] = []byte("<html>Maintenance</html>")

	e := s.jalankan(t)

	assert.Equal(t, domain.StatusGagal, e.Status)
	assert.Contains(t, *e.Galat, "bukan JSON")
	assert.Zero(t, s.hitung(t, "data_mentah"))
}

func TestRun_SemuaBarisDitolakDianggapGagal(t *testing.T) {
	s := newSetup(t, "ingest_tolak_semua")
	s.fetcher.Respons[kunciPIHPS] = []byte(`{"data":[{"name":"Komoditas Asing","13/10/2025":"1.000"}]}`)

	e := s.jalankan(t)

	assert.Equal(t, domain.StatusGagal, e.Status)
	assert.Equal(t, 1, e.BarisTakTerpetakan)
	assert.Contains(t, *e.Galat, "pemetaan_sumber")
}

func TestRun_KonfigurasiTakLengkapGagalDenganPesanJelas(t *testing.T) {
	s := newSetup(t, "ingest_konfigurasi")
	require.NoError(t, s.db.Exec(`UPDATE sumber_data SET konfigurasi = '{}' WHERE id = ?`, pihps.SumberID).Error)

	e := s.jalankan(t)

	assert.Equal(t, domain.StatusGagal, e.Status)
	assert.Contains(t, *e.Galat, "province_id")
	assert.Empty(t, s.fetcher.Dipanggil(), "tanpa konfigurasi tidak ada permintaan keluar")
}

func TestMulai_Penolakan(t *testing.T) {
	s := newSetup(t, "ingest_mulai")
	ctx := t.Context()

	_, err := s.runner.Mulai(ctx, "TIDAK-ADA", domain.PemicuManual, nil)
	assert.ErrorIs(t, err, domain.ErrSumberTidakAda)

	_, err = s.runner.Mulai(ctx, "SRC-DJPB", domain.PemicuManual, nil)
	assert.ErrorIs(t, err, domain.ErrTanpaKonektor)

	require.NoError(t, s.db.Exec(`UPDATE sumber_data SET status = 'DIJEDA' WHERE id = ?`, pihps.SumberID).Error)
	_, err = s.runner.Mulai(ctx, pihps.SumberID, domain.PemicuManual, nil)
	assert.ErrorIs(t, err, domain.ErrDijeda)
	assert.Zero(t, s.hitung(t, "eksekusi_sumber"), "penolakan tidak meninggalkan baris eksekusi")
}

func TestMulai_TidakBolehTumpangTindihLaluPulihBilaMacet(t *testing.T) {
	s := newSetup(t, "ingest_tumpang")
	ctx := t.Context()

	run, err := s.runner.Mulai(ctx, pihps.SumberID, domain.PemicuJadwal, nil)
	require.NoError(t, err)
	_, err = s.runner.Mulai(ctx, pihps.SumberID, domain.PemicuManual, nil)
	assert.ErrorIs(t, err, domain.ErrSudahBerjalan)

	// Proses mati sebelum menyelesaikan run: eksekusi lama dianggap macet dan ditutup.
	require.NoError(t, s.db.Exec(`UPDATE eksekusi_sumber SET mulai = now() - interval '2 hours' WHERE id = ?`, run.Eksekusi().ID).Error)
	baru, err := s.runner.Mulai(ctx, pihps.SumberID, domain.PemicuManual, nil)
	require.NoError(t, err)
	assert.NotEqual(t, run.Eksekusi().ID, baru.Eksekusi().ID)

	var macet domain.EksekusiSumber
	require.NoError(t, s.db.First(&macet, run.Eksekusi().ID).Error)
	assert.Equal(t, domain.StatusGagal, macet.Status)
	require.NotNil(t, macet.Selesai)
	assert.Contains(t, *macet.Galat, "terhenti")
}

func TestRun_SaatBerjalanSumberDijedaTidakDitimpa(t *testing.T) {
	s := newSetup(t, "ingest_jeda_saat_jalan")
	tahan := make(chan struct{})
	s.fetcher.Tahan = tahan

	run, err := s.runner.Mulai(t.Context(), pihps.SumberID, domain.PemicuManual, nil)
	require.NoError(t, err)
	selesai := make(chan *domain.EksekusiSumber, 1)
	go func() {
		e, _ := run.Jalankan(context.Background())
		selesai <- e
	}()

	require.NoError(t, s.db.Exec(`UPDATE sumber_data SET status = 'DIJEDA' WHERE id = ?`, pihps.SumberID).Error)
	close(tahan)
	e := <-selesai

	require.NotNil(t, e)
	assert.Equal(t, domain.StatusSukses, e.Status)
	var status string
	require.NoError(t, s.db.Raw(`SELECT status::text FROM sumber_data WHERE id = ?`, pihps.SumberID).Scan(&status).Error)
	assert.Equal(t, "DIJEDA", status, "jeda operator lebih kuat daripada hasil run")
}

func TestRun_KonteksDibatalkanMencatatGagalTetapTersimpan(t *testing.T) {
	s := newSetup(t, "ingest_batal")
	s.fetcher.Tahan = make(chan struct{})

	run, err := s.runner.Mulai(t.Context(), pihps.SumberID, domain.PemicuManual, nil)
	require.NoError(t, err)
	ctx, batal := context.WithCancel(t.Context())
	batal()
	e, err := run.Jalankan(ctx)

	require.NoError(t, err, "hasil akhir tetap bisa dicatat walau konteks run dibatalkan")
	assert.Equal(t, domain.StatusGagal, e.Status)
	var tersimpan domain.EksekusiSumber
	require.NoError(t, s.db.First(&tersimpan, e.ID).Error)
	assert.Equal(t, domain.StatusGagal, tersimpan.Status)
	assert.NotNil(t, tersimpan.Selesai)
}

func TestRepository_RiwayatDanSampel(t *testing.T) {
	s := newSetup(t, "ingest_riwayat")
	s.jalankan(t)
	s.fetcher.Galat = map[string]error{kunciPIHPS: errors.New("mati")}
	terakhir := s.jalankan(t)

	riwayat, err := s.repo.Eksekusi(t.Context(), pihps.SumberID, 10)
	require.NoError(t, err)
	require.Len(t, riwayat, 2)
	assert.Equal(t, terakhir.ID, riwayat[0].ID, "terbaru lebih dulu")

	satu, err := s.repo.Eksekusi(t.Context(), pihps.SumberID, 1)
	require.NoError(t, err)
	assert.Len(t, satu, 1)

	sampel, err := s.repo.SampelMentah(t.Context(), pihps.SumberID, 5)
	require.NoError(t, err)
	require.Len(t, sampel, 1)
	assert.Equal(t, kunciPIHPS, sampel[0].KunciPermintaan)
	assert.Positive(t, sampel[0].Ukuran)
	assert.LessOrEqual(t, len(sampel[0].Cuplikan), 2000)
}

func TestRepository_SimpanSilverMembuangDuplikatKunciDalamSatuKiriman(t *testing.T) {
	s := newSetup(t, "ingest_dedupe")
	tgl := time.Date(2025, 10, 13, 0, 0, 0, 0, time.UTC)
	baris := func(harga float64) domain.HargaEksternal {
		return domain.HargaEksternal{SumberID: pihps.SumberID, KomoditasKode: "GLA", KabupatenKode: "1171", Tanggal: tgl, Tipe: domain.TipeHargaKonsumen, Harga: harga, Satuan: "kg"}
	}

	require.NoError(t, s.repo.SimpanSilver(t.Context(), domain.Hasil{Harga: []domain.HargaEksternal{baris(18000), baris(18100)}}))

	var h domain.HargaEksternal
	require.NoError(t, s.db.First(&h, "komoditas_kode = 'GLA'").Error)
	assert.Equal(t, 18100.0, h.Harga, "yang terakhir menang")
	assert.EqualValues(t, 1, s.hitung(t, "harga_eksternal"))
}
