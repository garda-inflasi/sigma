package seeder

import (
	"encoding/json"
	"flag"
	"os"
	"strings"
	"testing"
	"time"

	authseeder "github.com/garda-inflasi/api/internal/auth/seeder"
	ewsseeder "github.com/garda-inflasi/api/internal/ews/seeder"
	"github.com/garda-inflasi/api/internal/intervensi/domain"
	masterseeder "github.com/garda-inflasi/api/internal/master/seeder"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const fixturePath = "../../../../../packages/contracts/intervensi-tahap.json"

var (
	pg            *pgtest.Server
	seedNow       = time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
	updateFixture = flag.Bool("update-fixture", false, "tulis ulang packages/contracts/intervensi-tahap.json dari seed")
)

func TestMain(m *testing.M) {
	os.Exit(pgtest.Main(m, &pg))
}

// fixtureTahap adalah cermin seed yang dibaca tes web: SIKLUS_TAHAP di
// apps/web/src/lib/data/siklus.ts harus sama dengannya.
type fixtureTahap struct {
	Siklus         string `json:"siklus"`
	Kode           string `json:"kode"`
	Urutan         int    `json:"urutan"`
	Label          string `json:"label"`
	Petunjuk       string `json:"petunjuk"`
	IzinDibutuhkan string `json:"izin_dibutuhkan"`
}

func currentFixture() []fixtureTahap {
	var out []fixtureTahap
	for _, t := range BuildTahap() {
		out = append(out, fixtureTahap{strings.ToLower(t.Siklus), t.KodeTahap, t.Urutan, t.Label, t.Petunjuk, t.IzinDibutuhkan})
	}
	return out
}

func TestFixtureFileMatchesSeed(t *testing.T) {
	want := currentFixture()

	if *updateFixture {
		raw, err := json.MarshalIndent(want, "", "  ")
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(fixturePath, append(raw, '\n'), 0o644))
	}

	raw, err := os.ReadFile(fixturePath)
	require.NoError(t, err)
	var got []fixtureTahap
	require.NoError(t, json.Unmarshal(raw, &got))

	assert.Equal(t, want, got, "intervensi-tahap.json basi; jalankan: go test ./internal/intervensi/seeder -run TestFixtureFileMatchesSeed -update-fixture")
}

func TestBuildTahap_Shape(t *testing.T) {
	tahap := BuildTahap()

	require.Len(t, tahap, 16)
	perSiklus := map[string][]domain.TahapDef{}
	for _, d := range tahap {
		perSiklus[d.Siklus] = append(perSiklus[d.Siklus], d)
	}
	assert.Len(t, perSiklus[domain.SiklusNormal], 5)
	assert.Len(t, perSiklus[domain.SiklusTekanan], 6)
	assert.Len(t, perSiklus[domain.SiklusSurplus], 5)

	for siklus, list := range perSiklus {
		for i, d := range list {
			assert.Equal(t, i+1, d.Urutan, "%s/%s: urutan harus berurutan dari 1", siklus, d.KodeTahap)
			assert.NotEmpty(t, d.Label)
			assert.True(t, strings.HasSuffix(list[len(list)-1].KodeTahap, "-selesai"), "tahap terakhir %s harus penutup", siklus)
		}
		assert.Nil(t, list[len(list)-1].SLAHari, "tahap penutup tidak punya SLA")
	}

	izin := map[string]string{}
	for _, d := range tahap {
		izin[d.KodeTahap] = d.IzinDibutuhkan
	}
	assert.Equal(t, "intervensi.setujui", izin["t-petakan"])
	assert.Equal(t, "intervensi.setujui_fiskal", izin["t-fiskal"])
	assert.Equal(t, "intervensi.setujui", izin["t-selesai"])
	assert.Equal(t, "intervensi.kelola", izin["t-ews"])
}

func TestBuildKartu_ShapeAndReferences(t *testing.T) {
	kartu, pj := BuildKartu(seedNow)

	require.Len(t, kartu, 19)
	tahap := map[string]string{} // kode -> siklus
	for _, d := range BuildTahap() {
		tahap[d.KodeTahap] = d.Siklus
	}
	ids, pos := map[string]bool{}, map[string]bool{}
	perSiklus := map[string]int{}
	for _, k := range kartu {
		assert.False(t, ids[k.ID], "id ganda %s", k.ID)
		ids[k.ID] = true
		assert.Equal(t, tahap[k.Tahap], k.Siklus, "%s: tahap %s bukan milik siklus %s", k.ID, k.Tahap, k.Siklus)
		key := k.Tahap + "/" + string(rune('0'+k.Urutan))
		assert.False(t, pos[key], "%s: urutan ganda di tahap", k.ID)
		pos[key] = true
		assert.LessOrEqual(t, k.ChecklistSelesai, k.ChecklistTotal, k.ID)
		perSiklus[k.Siklus]++
		if strings.HasSuffix(k.Tahap, "-selesai") {
			assert.NotNil(t, k.SelesaiPada, "%s berada di tahap penutup", k.ID)
			assert.Equal(t, k.ChecklistTotal, k.ChecklistSelesai, "%s: checklist tahap penutup harus tuntas", k.ID)
		} else {
			assert.Nil(t, k.SelesaiPada, k.ID)
		}
	}
	assert.Equal(t, map[string]int{"TEKANAN": 7, "NORMAL": 6, "SURPLUS": 6}, perSiklus)

	assigned := map[string]int{}
	for _, p := range pj {
		assert.True(t, ids[p.IntervensiID])
		assigned[p.IntervensiID]++
	}
	for id := range ids {
		assert.GreaterOrEqual(t, assigned[id], 1, "%s harus punya penanggung jawab", id)
		assert.LessOrEqual(t, assigned[id], 2, id)
	}
}

func seededDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db := pg.NewDatabase(t, name)
	pgtest.MigrateUp(t, db)
	ctx := t.Context()
	_, err := masterseeder.SeedReference(ctx, db)
	require.NoError(t, err)
	_, err = authseeder.SeedDemo(ctx, db)
	require.NoError(t, err)
	_, err = ewsseeder.SeedReference(ctx, db)
	require.NoError(t, err)
	_, err = ewsseeder.SeedDemo(ctx, db, seedNow)
	require.NoError(t, err)
	return db
}

func count(t *testing.T, db *gorm.DB, table string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Table(table).Count(&n).Error)
	return n
}

func TestSeed_IsIdempotent(t *testing.T) {
	db := seededDB(t, "intervensi_seed_idempotent")
	ctx := t.Context()

	for range 2 {
		ref, err := SeedReference(ctx, db)
		require.NoError(t, err)
		assert.Equal(t, 16, ref.Tahap)
		demo, err := SeedDemo(ctx, db, seedNow)
		require.NoError(t, err)
		assert.Equal(t, 19, demo.Kartu)
		assert.Equal(t, 19+jumlahPindah(), demo.Riwayat)
	}

	assert.EqualValues(t, 16, count(t, db, "tahap_intervensi_def"))
	assert.EqualValues(t, 19+jumlahPindah(), count(t, db, "intervensi_riwayat"), "riwayat tidak boleh bertambah")
	assert.EqualValues(t, 19, count(t, db, "intervensi"))
	_, pj := BuildKartu(seedNow)
	assert.EqualValues(t, len(pj), count(t, db, "intervensi_pj"), "penanggung jawab tidak boleh bertambah")
}

// jumlahPindah adalah banyaknya pemindahan tahap pada riwayat demo: tiap kartu dipindah
// dari tahap pertama sampai tahap sekarang, jadi sebanyak (posisi tahap - 1).
func jumlahPindah() int {
	posisi := map[string]int{}
	n := map[string]int{}
	for _, t := range tahapSeeds {
		n[t.siklus]++
		posisi[t.kode] = n[t.siklus]
	}
	total := 0
	for _, k := range kartuSeeds {
		total += posisi[k.tahap] - 1
	}
	return total
}

func TestBuildRiwayat_EachCardStartsCreatedAndWalksToItsCurrentTahap(t *testing.T) {
	kartu, pj := BuildKartu(seedNow)
	rows := BuildRiwayat(kartu, pj)

	byCard := map[string][]domain.IntervensiRiwayat{}
	for _, r := range rows {
		byCard[r.IntervensiID] = append(byCard[r.IntervensiID], r)
	}
	for _, k := range kartu {
		path := byCard[k.ID]
		require.NotEmpty(t, path, k.ID)
		assert.Equal(t, domain.AksiBuat, path[0].Aksi, k.ID)
		last := path[len(path)-1]
		assert.Equal(t, k.Tahap, *last.Ke, "%s berakhir di tahapnya sekarang", k.ID)
		for i := 1; i < len(path); i++ {
			assert.Equal(t, domain.AksiPindahTahap, path[i].Aksi, k.ID)
			assert.Equal(t, *path[i-1].Ke, *path[i].Dari, "%s: rantai tahap tak putus", k.ID)
			assert.True(t, path[i].Waktu.After(path[i-1].Waktu), k.ID)
		}
	}
}

func TestSeedDemo_ResetsAssigneesToTheirInitialState(t *testing.T) {
	db := seededDB(t, "intervensi_seed_reset")
	ctx := t.Context()
	_, err := SeedReference(ctx, db)
	require.NoError(t, err)
	_, err = SeedDemo(ctx, db, seedNow)
	require.NoError(t, err)
	require.NoError(t, db.Exec(`INSERT INTO intervensi_pj (intervensi_id, pengguna_id) VALUES ('INT-301', garda_uuid_stabil('pengguna', 'strategis_bi'))`).Error)

	_, err = SeedDemo(ctx, db, seedNow)
	require.NoError(t, err)

	var n int64
	require.NoError(t, db.Table("intervensi_pj").Where("intervensi_id = 'INT-301' AND pengguna_id = garda_uuid_stabil('pengguna', 'strategis_bi')").Count(&n).Error)
	assert.Zero(t, n)
}

func TestEveryCardTahapIsDefinedForItsSiklus(t *testing.T) {
	db := seededDB(t, "intervensi_tahap_fk")
	ctx := t.Context()
	_, err := SeedReference(ctx, db)
	require.NoError(t, err)
	_, err = SeedDemo(ctx, db, seedNow)
	require.NoError(t, err)

	var orphans int64
	require.NoError(t, db.Raw(`
		SELECT COUNT(*) FROM intervensi i
		 WHERE NOT EXISTS (SELECT 1 FROM tahap_intervensi_def d WHERE d.siklus = i.siklus AND d.kode_tahap = i.tahap)`).Scan(&orphans).Error)
	assert.Zero(t, orphans)

	t.Run("tahap tak terdefinisi ditolak basis data", func(t *testing.T) {
		err := db.Exec(`UPDATE intervensi SET tahap = 'n-pantau' WHERE id = 'INT-301'`).Error // INT-301 siklus TEKANAN
		assert.ErrorContains(t, err, "fk_intervensi_tahap")
	})
}
