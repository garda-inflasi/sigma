package seeder

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	authseeder "github.com/garda-inflasi/api/internal/auth/seeder"
	masterseeder "github.com/garda-inflasi/api/internal/master/seeder"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/internal/policy-brief/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var pg *pgtest.Server

func TestMain(m *testing.M) {
	os.Exit(pgtest.Main(m, &pg))
}

var now = time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC)

func TestBuildBrief_MirrorsMockBriefs(t *testing.T) {
	rows, err := BuildBrief(now)
	require.NoError(t, err)

	// Sama dengan mockBriefs di apps/web/src/lib/api/mock.ts: id, status, jumlah rekomendasi, halaman.
	want := []struct {
		id, periode, status string
		rekomendasi, hal    int
	}{
		{"PB-2026-10", "Oktober 2026", "DRAF", 5, 8},
		{"PB-2026-09", "September 2026", "DISETUJUI", 4, 7},
		{"PB-2026-08", "Agustus 2026", "DIDISTRIBUSIKAN", 3, 6},
		{"PB-2026-07", "Juli 2026", "DIDISTRIBUSIKAN", 4, 9},
		{"PB-2026-06", "Juni 2026", "DIDISTRIBUSIKAN", 6, 10},
	}
	require.Len(t, rows, len(want))
	for i, w := range want {
		assert.Equal(t, w.id, rows[i].ID)
		assert.Equal(t, w.periode, rows[i].Periode)
		assert.Equal(t, w.status, rows[i].Status)
		assert.Equal(t, w.hal, rows[i].Halaman)
		assert.Equal(t, PenyusunID, rows[i].DisusunOlehID)

		var rekomendasi []string
		require.NoError(t, json.Unmarshal([]byte(rows[i].Rekomendasi), &rekomendasi))
		assert.Len(t, rekomendasi, w.rekomendasi, w.id)
		assert.False(t, rows[i].DibuatPada.After(rows[i].Diperbarui), w.id)
	}
	assert.Equal(t, now.Add(-45*time.Minute), rows[0].Diperbarui, "waktu relatif terhadap now")
}

func TestSeedDemo_IsIdempotentAndKeepsEdits(t *testing.T) {
	db := pg.NewDatabase(t, "brief_seeder")
	pgtest.MigrateUp(t, db)
	_, err := masterseeder.SeedReference(t.Context(), db)
	require.NoError(t, err)
	_, err = authseeder.SeedDemo(t.Context(), db)
	require.NoError(t, err)

	sum, err := SeedDemo(t.Context(), db, now)
	require.NoError(t, err)
	assert.Equal(t, 5, sum.Brief)

	require.NoError(t, db.Exec("UPDATE policy_brief SET status = 'TELAAH' WHERE id = 'PB-2026-10'").Error)
	_, err = SeedDemo(t.Context(), db, now.Add(time.Hour))
	require.NoError(t, err)

	var n int64
	require.NoError(t, db.Table("policy_brief").Count(&n).Error)
	assert.EqualValues(t, 5, n)
	var status string
	require.NoError(t, db.Raw("SELECT status::text FROM policy_brief WHERE id = 'PB-2026-10'").Scan(&status).Error)
	assert.Equal(t, "TELAAH", status, "seed ulang tidak menimpa perubahan")
}

func TestSeedDemo_BriefContohBerlabelDemoDanDilengkapiBilaKontenKosong(t *testing.T) {
	db := pg.NewDatabase(t, "brief_seeder_konten")
	pgtest.MigrateUp(t, db)
	_, err := masterseeder.SeedReference(t.Context(), db)
	require.NoError(t, err)
	_, err = authseeder.SeedDemo(t.Context(), db)
	require.NoError(t, err)
	_, err = SeedDemo(t.Context(), db, now)
	require.NoError(t, err)

	var jumlahDemo int64
	require.NoError(t, db.Table("policy_brief").Where("sumber_narasi = 'DEMO' AND konten->>'label' = ?", domain.LabelDemo).Count(&jumlahDemo).Error)
	assert.EqualValues(t, 5, jumlahDemo)

	// Brief contoh dari sebelum kolom konten ada (konten kosong) dilengkapi; yang sudah berisi tidak ditimpa.
	require.NoError(t, db.Exec(`UPDATE policy_brief SET konten = '{}', sumber_narasi = 'TEMPLAT' WHERE id = 'PB-2026-10'`).Error)
	require.NoError(t, db.Exec(`UPDATE policy_brief SET konten = '{"label":"diubah"}' WHERE id = 'PB-2026-09'`).Error)
	_, err = SeedDemo(t.Context(), db, now)
	require.NoError(t, err)

	var label, sumber string
	require.NoError(t, db.Raw(`SELECT konten->>'label' FROM policy_brief WHERE id = 'PB-2026-10'`).Scan(&label).Error)
	require.NoError(t, db.Raw(`SELECT sumber_narasi FROM policy_brief WHERE id = 'PB-2026-10'`).Scan(&sumber).Error)
	assert.Equal(t, domain.LabelDemo, label)
	assert.Equal(t, "DEMO", sumber)
	require.NoError(t, db.Raw(`SELECT konten->>'label' FROM policy_brief WHERE id = 'PB-2026-09'`).Scan(&label).Error)
	assert.Equal(t, "diubah", label)
}

func TestSeedDemo_RequiresDemoAccount(t *testing.T) {
	db := pg.NewDatabase(t, "brief_seeder_noaccount")
	pgtest.MigrateUp(t, db)

	_, err := SeedDemo(t.Context(), db, now)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "auth.demo")
}
