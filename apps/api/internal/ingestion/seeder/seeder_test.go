package seeder

import (
	"os"
	"testing"

	"github.com/garda-inflasi/api/internal/ingestion/domain"
	masterseeder "github.com/garda-inflasi/api/internal/master/seeder"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	sumberseeder "github.com/garda-inflasi/api/internal/sumber-data/seeder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var pg *pgtest.Server

func TestMain(m *testing.M) {
	os.Exit(pgtest.Main(m, &pg))
}

func TestPemetaan_KunciUnikDanTujuanKonsisten(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range Pemetaan() {
		kunci := r.SumberID + "|" + r.Jenis + "|" + r.Alias
		assert.False(t, seen[kunci], "pemetaan ganda %s", kunci)
		seen[kunci] = true

		assert.Equal(t, domain.NormalisasiAlias(r.Alias), r.Alias, "alias harus sudah ternormalisasi: %s", kunci)
		assert.Contains(t, []string{domain.JenisKomoditas, domain.JenisWilayah}, r.Jenis, kunci)
		assert.Equal(t, r.Diabaikan, r.KodeMaster == nil, "diabaikan berpasangan dengan tanpa kode: %s", kunci)
	}
	assert.NotEmpty(t, seen)
}

func TestPemetaan_SumberIDAdalahUUIDStabilDariKode(t *testing.T) {
	assert.Equal(t, sumberseeder.IDSumber(kodeBI), sumberBI)
	assert.Equal(t, sumberseeder.IDSumber(kodeBMKG), sumberBMKG)
	assert.Equal(t, sumberseeder.IDSumber(kodeBPS), sumberBPS)
	for _, r := range Pemetaan() {
		assert.Contains(t, []string{sumberBI, sumberBMKG, sumberBPS}, r.SumberID)
	}
}

func TestPemetaan_KodeMasterAdaDiMaster(t *testing.T) {
	komoditas := map[string]bool{}
	for _, k := range masterseeder.Komoditas() {
		komoditas[k.Kode] = true
	}
	wilayah := map[string]bool{masterseeder.ProvinsiAceh.Kode: true}
	for _, k := range masterseeder.Kabupaten() {
		wilayah[k.Kode] = true
	}

	for _, r := range Pemetaan() {
		if r.KodeMaster == nil {
			continue
		}
		daftar := komoditas
		if r.Jenis == domain.JenisWilayah {
			daftar = wilayah
		}
		assert.True(t, daftar[*r.KodeMaster], "%s %s %q -> %s tidak ada di master", r.SumberID, r.Jenis, r.Alias, *r.KodeMaster)
	}
}

func TestPemetaan_TitikPantauBMKGMencakupSemuaKabupaten(t *testing.T) {
	dicakup := map[string]bool{}
	for _, r := range Pemetaan() {
		if r.SumberID == sumberBMKG && r.Jenis == domain.JenisWilayah {
			dicakup[*r.KodeMaster] = true
		}
	}
	for _, k := range masterseeder.Kabupaten() {
		assert.True(t, dicakup[k.Kode], "kabupaten/kota %s belum punya titik pantau BMKG", k.Kode)
	}
}

func newDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db := pg.NewDatabase(t, name)
	pgtest.MigrateUp(t, db)
	_, err := sumberseeder.SeedReference(t.Context(), db)
	require.NoError(t, err)
	return db
}

func TestSeedReference_IdempotenDanTidakMenimpaPemetaanOperator(t *testing.T) {
	db := newDB(t, "ingestion_seed_ref")

	sum, err := SeedReference(t.Context(), db)
	require.NoError(t, err)
	assert.Equal(t, len(Pemetaan()), sum.Pemetaan)
	var n int64
	require.NoError(t, db.Model(&domain.PemetaanSumber{}).Count(&n).Error)
	assert.EqualValues(t, len(Pemetaan()), n)

	// Operator mengalihkan satu alias dan mengabaikan alias lain.
	require.NoError(t, db.Exec(`UPDATE pemetaan_sumber SET kode_master = 'BRS-PRM' WHERE sumber_id = ? AND alias = 'beras kualitas medium i'`, sumberBI).Error)
	require.NoError(t, db.Exec(`UPDATE pemetaan_sumber SET kode_master = NULL, diabaikan = true WHERE sumber_id = ? AND alias = 'cabai merah keriting'`, sumberBI).Error)

	_, err = SeedReference(t.Context(), db)
	require.NoError(t, err)

	require.NoError(t, db.Model(&domain.PemetaanSumber{}).Count(&n).Error)
	assert.EqualValues(t, len(Pemetaan()), n, "seed ulang tidak menambah baris")
	var medium domain.PemetaanSumber
	require.NoError(t, db.First(&medium, "sumber_id = ? AND alias = ?", sumberBI, "beras kualitas medium i").Error)
	assert.Equal(t, "BRS-PRM", *medium.KodeMaster, "pemetaan hasil operasi tidak ditimpa")
	var keriting domain.PemetaanSumber
	require.NoError(t, db.First(&keriting, "sumber_id = ? AND alias = ?", sumberBI, "cabai merah keriting").Error)
	assert.True(t, keriting.Diabaikan)
}
