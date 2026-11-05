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
			{ID: "mahasiswa_1", PasarID: "PSR-1171-01", PasarNama: "Pasar Peunayong"},
			{ID: "mahasiswa_2", PasarID: "PSR-1108-01", PasarNama: "Pasar Induk Lambaro"},
		},
		Pembuat: "manajer_program",
	}
}

func TestBuild_IsDeterministicAndUnique(t *testing.T) {
	first := Build(sampleInput(), seedNow)

	assert.Equal(t, first, Build(sampleInput(), seedNow))
	require.Len(t, first, 2*len(templates))
	ids := map[string]bool{}
	for _, tugas := range first {
		assert.False(t, ids[tugas.ID], "id ganda: %s", tugas.ID)
		ids[tugas.ID] = true
	}
}

func TestBuild_EveryPetugasHasAnActiveTask(t *testing.T) {
	aktif := map[string]int{}
	for _, tugas := range Build(sampleInput(), seedNow) {
		if tugas.Status != domain.StatusSelesai {
			aktif[tugas.PetugasID]++
		}
	}
	assert.GreaterOrEqual(t, aktif["mahasiswa_1"], 1)
	assert.GreaterOrEqual(t, aktif["mahasiswa_2"], 1)
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
	require.NoError(t, db.First(&tugas, "id = ?", "TGS-mahasiswa_1-1").Error)
	assert.Equal(t, domain.KodeKomoditas(templates[0].Komoditas), tugas.Komoditas)
	assert.Equal(t, "PSR-1171-01", tugas.PasarID)
}
