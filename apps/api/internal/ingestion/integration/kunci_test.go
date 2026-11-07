package integration

import (
	"testing"

	"github.com/garda-inflasi/api/internal/ingestion/connector/bmkg"
	"github.com/garda-inflasi/api/internal/ingestion/connector/pihps"
	"github.com/garda-inflasi/api/internal/ingestion/domain"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Kunci primer dan kunci asing ingestion (migrasi 000115): sumber_data ber-UUID dan dirujuk lewat UUID atau kode;
// eksekusi, bronze, dan silver ber-ID BIGINT; aturan ON DELETE mengikuti docs/SKEMA-DATABASE.md.

func TestMulai_SumberDirujukLewatUUIDAtauKode(t *testing.T) {
	s := newSetup(t, "ingest_ref")
	ctx := t.Context()

	lewatKode, err := s.runner.Mulai(ctx, pihps.Kode, domain.PemicuManual, nil)
	require.NoError(t, err)
	assert.Equal(t, idBI, lewatKode.Eksekusi().SumberID, "eksekusi menunjuk UUID sumber walau dirujuk lewat kode")
	_, err = lewatKode.Jalankan(ctx)
	require.NoError(t, err)

	lewatUUID, err := s.runner.Mulai(ctx, idBI, domain.PemicuManual, nil)
	require.NoError(t, err)
	assert.Equal(t, idBI, lewatUUID.Eksekusi().SumberID)
	assert.Greater(t, lewatUUID.Eksekusi().ID, lewatKode.Eksekusi().ID, "identity menaik")

	for _, ref := range []string{"", "bukan-sumber", "1; DROP TABLE sumber_data", idx.UUIDStabil("sumber_data", "SRC-X")} {
		_, err := s.runner.Mulai(ctx, ref, domain.PemicuManual, nil)
		assert.ErrorIs(t, err, domain.ErrSumberTidakAda, "ref %q", ref)
	}
	assert.EqualValues(t, 2, s.hitung(t, "eksekusi_sumber"), "penolakan tidak meninggalkan eksekusi")
}

func TestKonektor_DikenaliMenurutKodeBukanUUID(t *testing.T) {
	s := newSetup(t, "ingest_kode")

	assert.True(t, s.runner.Punya("SRC-BI"))
	assert.False(t, s.runner.Punya(idBI))
	assert.False(t, s.runner.Punya("SRC-BMKG"), "runner ini hanya memuat konektor PIHPS")
	assert.Equal(t, "SRC-BI", pihps.Kode)
	assert.Equal(t, "SRC-BMKG", bmkg.Kode)
}

func TestSilver_IDIdentityDanKunciAlamiUnik(t *testing.T) {
	s := newSetup(t, "ingest_silver_id")
	s.jalankan(t)

	var h domain.HargaEksternal
	require.NoError(t, s.db.First(&h, "komoditas_kode = 'BRS-MED' AND tanggal = '2025-10-13'").Error)
	assert.Positive(t, h.ID)
	idAwal := h.ID

	// Upsert menurut kunci alami mengembalikan baris yang sama: ID tidak berganti dan tidak ada baris baru.
	s.jalankan(t)
	require.NoError(t, s.db.First(&h, "komoditas_kode = 'BRS-MED' AND tanggal = '2025-10-13'").Error)
	assert.Equal(t, idAwal, h.ID, "upsert memperbarui baris yang ada")
	assert.EqualValues(t, 14, s.hitung(t, "harga_eksternal"))

	// Kunci alami ganda ditolak basis data dengan pelanggaran unik 23505.
	err := s.db.Exec(`INSERT INTO harga_eksternal (sumber_id, komoditas_kode, kabupaten_kode, tanggal, tipe, harga, satuan)
		VALUES (?, 'BRS-MED', '1171', '2025-10-13', 'KONSUMEN', 1, 'kg')`, idBI).Error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "23505")
}

func (s *setup) hitungWhere(t *testing.T, sql string, args ...any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, s.db.Raw(sql, args...).Scan(&n).Error)
	return n
}

func TestAturanHapus_SumberDanTurunannya(t *testing.T) {
	s := newSetup(t, "ingest_hapus")
	e := s.jalankan(t)
	require.Equal(t, domain.StatusSukses, e.Status)
	require.EqualValues(t, 14, s.hitung(t, "harga_eksternal"))

	// Cuaca ikut dari eksekusi dan bronze yang sama, supaya SET NULL cuaca_harian teruji.
	require.NoError(t, s.db.Exec(`
		INSERT INTO cuaca_harian (kabupaten_kode, tanggal, curah_hujan_mm, suhu_c, kelembapan, sumber, eksekusi_id, data_mentah_id)
		SELECT '1171', '2025-10-13', 1, 27, 80, 'BMKG', ?, id FROM data_mentah`, e.ID).Error)

	t.Run("sumber yang punya harga eksternal tidak boleh dihapus (RESTRICT)", func(t *testing.T) {
		err := s.db.Exec(`DELETE FROM sumber_data WHERE kode = 'SRC-BI'`).Error
		require.Error(t, err)
		assert.Contains(t, err.Error(), "23503")
		assert.EqualValues(t, 14, s.hitung(t, "harga_eksternal"))
	})

	t.Run("menghapus eksekusi: bronze ikut (CASCADE), harga dan cuaca dilepas (SET NULL)", func(t *testing.T) {
		require.NoError(t, s.db.Exec(`DELETE FROM eksekusi_sumber WHERE id = ?`, e.ID).Error)

		assert.Zero(t, s.hitung(t, "data_mentah"), "bronze milik eksekusi")
		assert.EqualValues(t, 14, s.hitung(t, "harga_eksternal"), "harga tetap ada")
		assert.Zero(t, s.hitungWhere(t, `SELECT count(*) FROM harga_eksternal WHERE eksekusi_id IS NOT NULL OR data_mentah_id IS NOT NULL`),
			"lineage dikosongkan")
		assert.EqualValues(t, 1, s.hitungWhere(t, `SELECT count(*) FROM cuaca_harian WHERE eksekusi_id IS NULL AND data_mentah_id IS NULL`))
	})

	t.Run("menghapus data_mentah saja mengosongkan rujukannya (SET NULL)", func(t *testing.T) {
		e2 := s.jalankan(t)
		require.Equal(t, domain.StatusSukses, e2.Status)
		require.NotZero(t, s.hitungWhere(t, `SELECT count(*) FROM harga_eksternal WHERE data_mentah_id IS NOT NULL`))

		require.NoError(t, s.db.Exec(`DELETE FROM data_mentah`).Error)

		assert.Zero(t, s.hitungWhere(t, `SELECT count(*) FROM harga_eksternal WHERE data_mentah_id IS NOT NULL`))
		assert.EqualValues(t, 14, s.hitung(t, "harga_eksternal"))
		assert.EqualValues(t, 1, s.hitung(t, "eksekusi_sumber"), "eksekusi tidak ikut terhapus")
	})

	t.Run("sumber tanpa harga: konfigurasi dan riwayat ikut terhapus (CASCADE)", func(t *testing.T) {
		require.NoError(t, s.db.Exec(`DELETE FROM harga_eksternal`).Error)
		require.NotZero(t, s.hitung(t, "pemetaan_sumber"))

		require.NoError(t, s.db.Exec(`DELETE FROM sumber_data WHERE kode = 'SRC-BI'`).Error)

		assert.Zero(t, s.hitungWhere(t, `SELECT count(*) FROM pemetaan_sumber WHERE sumber_id = ?`, idBI))
		assert.Zero(t, s.hitungWhere(t, `SELECT count(*) FROM eksekusi_sumber WHERE sumber_id = ?`, idBI))
		assert.NotZero(t, s.hitung(t, "pemetaan_sumber"), "pemetaan sumber lain tidak tersentuh")
	})
}

func TestAturanHapus_KomoditasYangDirujukSilverDitahan(t *testing.T) {
	s := newSetup(t, "ingest_hapus_komoditas")
	s.jalankan(t)
	// Rujukan ke master ditahan RESTRICT: harga eksternal adalah bukti, bukan turunan master.
	err := s.db.Exec(`DELETE FROM komoditas WHERE kode = 'BRS-MED'`).Error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "harga_eksternal_komoditas_kode_fkey")
	assert.Contains(t, err.Error(), "23503")
}

func TestIdentity_IDEksplisitDitolakDanIdentityBerlanjut(t *testing.T) {
	s := newSetup(t, "ingest_identity")
	e := s.jalankan(t)

	err := s.db.Exec(`INSERT INTO harga_eksternal (id, sumber_id, komoditas_kode, kabupaten_kode, tanggal, tipe, harga, satuan)
		VALUES (999, ?, 'GLA', '1171', '2025-10-01', 'KONSUMEN', 1, 'kg')`, idBI).Error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "428C9", "kolom GENERATED ALWAYS menolak nilai eksplisit")

	err = s.db.Exec(`INSERT INTO cuaca_harian (id, kabupaten_kode, tanggal, curah_hujan_mm, suhu_c, kelembapan)
		VALUES (999, '1171', '2025-10-01', 0, 25, 80)`).Error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "428C9", "kolom GENERATED ALWAYS menolak nilai eksplisit")

	run, err := s.runner.Mulai(t.Context(), pihps.Kode, domain.PemicuManual, nil)
	require.NoError(t, err)
	assert.Greater(t, run.Eksekusi().ID, e.ID)
}
