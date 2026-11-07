package seeder

import (
	"os"
	"testing"
	"time"

	authseeder "github.com/garda-inflasi/api/internal/auth/seeder"
	"github.com/garda-inflasi/api/internal/lapangan/domain"
	masterseeder "github.com/garda-inflasi/api/internal/master/seeder"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var pg *pgtest.Server

func TestMain(m *testing.M) {
	os.Exit(pgtest.Main(m, &pg))
}

var seedNow = time.Date(2026, 10, 16, 7, 30, 0, 0, time.UTC)

func sampleInput() Input {
	return Input{
		Petugas: []Petugas{
			{ID: uid("mahasiswa_1"), PasarID: "PSR-1171-01", PasarNama: "Pasar Peunayong"},
			{ID: uid("mahasiswa_2"), PasarID: "PSR-1108-01", PasarNama: "Pasar Induk Lambaro"},
		},
		Pembuat: uid("manajer_program"),
	}
}

func TestBuild_IsDeterministicAndUnique(t *testing.T) {
	first := Build(sampleInput(), seedNow)

	assert.Equal(t, first, Build(sampleInput(), seedNow))
	require.Len(t, first, 2*len(templates))
	kode := map[string]bool{}
	for _, tugas := range first {
		// ID BIGINT diberi basis data; kunci idempotensi seeder adalah kode (sama dengan ID lama).
		assert.Zero(t, tugas.ID)
		require.NotNil(t, tugas.Kode)
		assert.False(t, kode[*tugas.Kode], "kode ganda: %s", *tugas.Kode)
		kode[*tugas.Kode] = true
	}
	assert.Contains(t, kode, "TGS-mahasiswa_1-1", "bentuk kode sama dengan ID lama")
	assert.Contains(t, kode, "TGS-mahasiswa_2-5")
}

func TestBuild_EveryPetugasHasAnActiveTask(t *testing.T) {
	aktif := map[string]int{}
	for _, tugas := range Build(sampleInput(), seedNow) {
		if tugas.Status != domain.StatusSelesai {
			aktif[tugas.PetugasID]++
		}
	}
	assert.GreaterOrEqual(t, aktif[uid("mahasiswa_1")], 1)
	assert.GreaterOrEqual(t, aktif[uid("mahasiswa_2")], 1)
}

func TestBuild_MixOfStatusesWithRelativeDeadlines(t *testing.T) {
	perStatus := map[string]int{}
	for _, tugas := range Build(sampleInput(), seedNow) {
		perStatus[tugas.Status]++
		assert.Positive(t, tugas.Target)
		assert.LessOrEqual(t, tugas.JumlahSelesai, tugas.Target)
		assert.NotEmpty(t, tugas.Komoditas)
		require.NotNil(t, tugas.DibuatOlehID)

		switch tugas.Status {
		case domain.StatusTerlambat:
			assert.True(t, tugas.Tenggat.Before(seedNow), "terlambat berarti tenggat sudah lewat")
		case domain.StatusBelum, domain.StatusBerjalan:
			assert.True(t, tugas.Tenggat.After(seedNow), "tugas aktif belum lewat tenggat")
		case domain.StatusSelesai:
			require.NotNil(t, tugas.SelesaiPada)
			assert.Equal(t, tugas.Target, tugas.JumlahSelesai)
		}
		if tugas.Status != domain.StatusSelesai {
			assert.Nil(t, tugas.SelesaiPada)
		}
	}
	for _, status := range []string{domain.StatusBelum, domain.StatusBerjalan, domain.StatusSelesai, domain.StatusTerlambat} {
		assert.Positive(t, perStatus[status], status)
	}
}

func TestSeedDemo_RequiresAccounts(t *testing.T) {
	db := pg.NewDatabase(t, "lapangan_seed_kosong")
	pgtest.MigrateUp(t, db)

	_, err := SeedDemo(t.Context(), db, seedNow)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "auth.demo")
}

func TestSeedDemo_IsIdempotentAndRoundTripsKomoditas(t *testing.T) {
	db := pg.NewDatabase(t, "lapangan_seed")
	pgtest.MigrateUp(t, db)
	_, err := masterseeder.SeedReference(t.Context(), db)
	require.NoError(t, err)
	_, err = authseeder.SeedDemo(t.Context(), db)
	require.NoError(t, err)

	first, err := SeedDemo(t.Context(), db, seedNow)
	require.NoError(t, err)
	// koordinator, agregator, dan 10 mahasiswa berkabupaten; akun nonaktif dan tanpa kabupaten dilewati.
	assert.Equal(t, 12*len(templates), first.Tugas)

	second, err := SeedDemo(t.Context(), db, seedNow.Add(30*time.Hour))
	require.NoError(t, err)
	assert.Equal(t, first.Tugas, second.Tugas)

	var n int64
	require.NoError(t, db.Raw("SELECT count(*) FROM tugas_lapangan").Scan(&n).Error)
	assert.EqualValues(t, first.Tugas, n)

	var tugas domain.TugasLapangan
	require.NoError(t, db.First(&tugas, "kode = ?", "TGS-mahasiswa_1-1").Error)
	assert.Equal(t, domain.KodeKomoditas(templates[0].Komoditas), tugas.Komoditas)
	assert.Equal(t, "PSR-1171-01", tugas.PasarID)
	assert.Positive(t, tugas.ID)
}

// TestSeedDemo_IDStabilDanDiAtasBasisDataHasilMigrasi: kode (ID lama) menjadi kunci idempotensi. ID BIGINT tidak
// berubah saat seeder diulang, dan baris hasil migrasi 000111 (kode = ID lama, ID BIGINT milik basis data) diperbarui
// tanpa digandakan.
func TestSeedDemo_IDStabilDanDiAtasBasisDataHasilMigrasi(t *testing.T) {
	db := pg.NewDatabase(t, "lapangan_seed_migrasi")
	pgtest.MigrateUp(t, db)
	_, err := masterseeder.SeedReference(t.Context(), db)
	require.NoError(t, err)
	_, err = authseeder.SeedDemo(t.Context(), db)
	require.NoError(t, err)

	// Baris lama hasil migrasi: kode = ID lama, jumlah_selesai berbeda dari templat.
	require.NoError(t, db.Exec(`
		INSERT INTO tugas_lapangan (id, kode, judul, pasar_id, petugas_id, komoditas, tipe, tenggat, status, target, jumlah_selesai)
		OVERRIDING SYSTEM VALUE
		VALUES (777, 'TGS-mahasiswa_1-1', 'Judul lama', 'PSR-1171-01', garda_uuid_stabil('pengguna', 'mahasiswa_1'),
		        ARRAY['BWM'], 'KONSUMEN', now(), 'BELUM', 9, 0)`).Error)

	first, err := SeedDemo(t.Context(), db, seedNow)
	require.NoError(t, err)

	scan := func(sql string, args ...any) (n int64) {
		require.NoError(t, db.Raw(sql, args...).Scan(&n).Error, sql)
		return n
	}
	assert.EqualValues(t, first.Tugas, scan("SELECT count(*) FROM tugas_lapangan"), "baris lama diperbarui, tidak digandakan")
	assert.EqualValues(t, 777, scan("SELECT id FROM tugas_lapangan WHERE kode = 'TGS-mahasiswa_1-1'"), "ID baris lama dipertahankan")
	assert.EqualValues(t, templates[0].Target, scan("SELECT target FROM tugas_lapangan WHERE kode = 'TGS-mahasiswa_1-1'"), "isi disegarkan oleh seeder")

	id2 := scan("SELECT id FROM tugas_lapangan WHERE kode = 'TGS-mahasiswa_2-3'")
	_, err = SeedDemo(t.Context(), db, seedNow.Add(30*time.Hour))
	require.NoError(t, err)
	assert.Equal(t, id2, scan("SELECT id FROM tugas_lapangan WHERE kode = 'TGS-mahasiswa_2-3'"), "ID tidak berubah saat seeder diulang")
	assert.EqualValues(t, first.Tugas, scan("SELECT count(*) FROM tugas_lapangan"))
	assert.Zero(t, scan("SELECT count(*) FROM tugas_lapangan WHERE kode IS NULL"))
}
