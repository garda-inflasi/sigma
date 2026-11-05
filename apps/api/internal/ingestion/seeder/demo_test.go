package seeder

import (
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/ingestion/domain"
	masterseeder "github.com/garda-inflasi/api/internal/master/seeder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var waktuTetap = time.Date(2026, 11, 5, 3, 0, 0, 0, time.UTC) // 10.00 WIB

func newDemoDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db := newDB(t, name)
	_, err := masterseeder.SeedReference(t.Context(), db)
	require.NoError(t, err)
	return db
}

func hitung(t *testing.T, db *gorm.DB, tabel string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Table(tabel).Count(&n).Error)
	return n
}

func TestBuatDemo_DeterministikDanBerbentukBenar(t *testing.T) {
	hariIni := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)

	harga := BuatHargaDemo(hariIni, 7, 9)
	assert.Equal(t, harga, BuatHargaDemo(hariIni, 7, 9))
	assert.Len(t, harga, len(kotaHargaDemo)*len(komoditasHargaDemo)*hariHarga)
	for _, h := range harga {
		assert.Equal(t, sumberHarga, h.SumberID)
		assert.Positive(t, h.Harga)
		assert.True(t, h.Tanggal.Before(hariIni), "harga eksternal berakhir kemarin")
		require.NotNil(t, h.EksekusiID)
		assert.EqualValues(t, 7, *h.EksekusiID)
	}
	assert.Nil(t, BuatHargaDemo(hariIni, 0, 0)[0].EksekusiID, "tanpa lineage bila tak ada run")

	cuaca := BuatCuacaDemo(hariIni, 1, 2)
	assert.Equal(t, cuaca, BuatCuacaDemo(hariIni, 1, 2))
	assert.Len(t, cuaca, 23*hariCuaca)
	anomali := 0
	for _, c := range cuaca {
		assert.Equal(t, c.Anomali, c.CurahHujanMm >= 50, "anomali = hujan lebat, sama dengan konektor BMKG")
		assert.Equal(t, domain.IDCuaca(c.KabupatenKode, c.Tanggal, c.Sumber), c.ID)
		if c.Anomali {
			anomali++
		}
	}
	assert.Positive(t, anomali, "data contoh memuat contoh anomali")
}

func TestSeedDemo_IdempotenDalamSatuHari(t *testing.T) {
	db := newDemoDB(t, "ingestion_demo_idem")

	sum, err := SeedDemo(t.Context(), db, waktuTetap)
	require.NoError(t, err)
	assert.Equal(t, len(jadwalDemo), sum.Eksekusi)
	tabel := []string{"eksekusi_sumber", "data_mentah", "harga_eksternal", "cuaca_harian", "gold_harga_harian"}
	pertama := map[string]int64{}
	for _, tb := range tabel {
		pertama[tb] = hitung(t, db, tb)
		assert.Positive(t, pertama[tb], tb)
	}

	_, err = SeedDemo(t.Context(), db, waktuTetap.Add(2*time.Hour))
	require.NoError(t, err)
	for _, tb := range tabel {
		assert.Equal(t, pertama[tb], hitung(t, db, tb), "%s: jalan ulang tidak menambah baris", tb)
	}

	var bukanDemo int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM eksekusi_sumber WHERE pemicu <> 'DEMO'`).Scan(&bukanDemo).Error)
	assert.Zero(t, bukanDemo)
}

func TestSeedDemo_TidakMenimpaHasilIngestionNyata(t *testing.T) {
	db := newDemoDB(t, "ingestion_demo_nyata")
	_, err := SeedDemo(t.Context(), db, waktuTetap)
	require.NoError(t, err)

	// Satu baris harga dan satu baris cuaca digantikan hasil run nyata (eksekusi bukan DEMO).
	var nyata int64
	require.NoError(t, db.Raw(`
		INSERT INTO eksekusi_sumber (sumber_id, pemicu, status, mulai, selesai) VALUES ('SRC-BI', 'CLI', 'SUKSES', now(), now()) RETURNING id`).Scan(&nyata).Error)
	require.NoError(t, db.Exec(`UPDATE harga_eksternal SET harga = 12345, eksekusi_id = ? WHERE (komoditas_kode, kabupaten_kode, tanggal) = (SELECT komoditas_kode, kabupaten_kode, MIN(tanggal) FROM harga_eksternal WHERE komoditas_kode = 'BRS-MED' AND kabupaten_kode = '1171' GROUP BY 1, 2)`, nyata).Error)
	require.NoError(t, db.Exec(`UPDATE cuaca_harian SET curah_hujan_mm = 1.5, eksekusi_id = ? WHERE kabupaten_kode = '1171' AND tanggal = (SELECT MIN(tanggal) FROM cuaca_harian)`, nyata).Error)

	_, err = SeedDemo(t.Context(), db, waktuTetap)
	require.NoError(t, err)

	var harga float64
	require.NoError(t, db.Raw(`SELECT harga FROM harga_eksternal WHERE eksekusi_id = ?`, nyata).Scan(&harga).Error)
	assert.Equal(t, 12345.0, harga)
	var hujan float64
	require.NoError(t, db.Raw(`SELECT curah_hujan_mm FROM cuaca_harian WHERE eksekusi_id = ?`, nyata).Scan(&hujan).Error)
	assert.Equal(t, 1.5, hujan)
}

func TestSeedDemo_EksekusiBPSJujurGagalKarenaBelumDikonfigurasi(t *testing.T) {
	db := newDemoDB(t, "ingestion_demo_bps")
	_, err := SeedDemo(t.Context(), db, waktuTetap)
	require.NoError(t, err)

	var gagal []domain.EksekusiSumber
	require.NoError(t, db.Where("sumber_id = 'SRC-BPS'").Find(&gagal).Error)
	require.NotEmpty(t, gagal)
	for _, e := range gagal {
		assert.Equal(t, domain.StatusGagal, e.Status, "tidak ada klaim ingestion BPS sukses yang belum pernah terjadi")
		require.NotNil(t, e.Galat)
		assert.Contains(t, *e.Galat, "belum lengkap")
	}
}
