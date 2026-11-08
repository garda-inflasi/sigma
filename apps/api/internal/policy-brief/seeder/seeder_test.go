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
	"github.com/garda-inflasi/api/pkg/idx"
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

	// Sama dengan mockBriefs di apps/web/src/lib/api/mock.ts: kode, status, jumlah rekomendasi, halaman.
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
		assert.Equal(t, w.id, rows[i].Kode, "kode = ID lama PB-YYYY-MM")
		assert.Equal(t, idx.UUIDStabil("policy_brief", w.id), rows[i].ID, "UUID stabil, sama dengan hasil migrasi 000114")
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

	require.NoError(t, db.Exec("UPDATE policy_brief SET status = 'TELAAH' WHERE kode = 'PB-2026-10'").Error)
	_, err = SeedDemo(t.Context(), db, now.Add(time.Hour))
	require.NoError(t, err)

	var n int64
	require.NoError(t, db.Table("policy_brief").Count(&n).Error)
	assert.EqualValues(t, 5, n)
	var status string
	require.NoError(t, db.Raw("SELECT status::text FROM policy_brief WHERE kode = 'PB-2026-10'").Scan(&status).Error)
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
	require.NoError(t, db.Exec(`UPDATE policy_brief SET konten = '{}', sumber_narasi = 'TEMPLAT' WHERE kode = 'PB-2026-10'`).Error)
	require.NoError(t, db.Exec(`UPDATE policy_brief SET konten = '{"label":"diubah"}' WHERE kode = 'PB-2026-09'`).Error)
	_, err = SeedDemo(t.Context(), db, now)
	require.NoError(t, err)

	var label, sumber string
	require.NoError(t, db.Raw(`SELECT konten->>'label' FROM policy_brief WHERE kode = 'PB-2026-10'`).Scan(&label).Error)
	require.NoError(t, db.Raw(`SELECT sumber_narasi FROM policy_brief WHERE kode = 'PB-2026-10'`).Scan(&sumber).Error)
	assert.Equal(t, domain.LabelDemo, label)
	assert.Equal(t, "DEMO", sumber)
	require.NoError(t, db.Raw(`SELECT konten->>'label' FROM policy_brief WHERE kode = 'PB-2026-09'`).Scan(&label).Error)
	assert.Equal(t, "diubah", label)
}

func TestSeedDemo_RequiresDemoAccount(t *testing.T) {
	db := pg.NewDatabase(t, "brief_seeder_noaccount")
	pgtest.MigrateUp(t, db)

	_, err := SeedDemo(t.Context(), db, now)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "auth.demo")
}

func TestSeedDemo_UUIDStabilSamaDenganMigrasiDanTidakDigandakanDiDBHasilMigrasi(t *testing.T) {
	db := pg.NewDatabase(t, "brief_seeder_migrasi")
	migrator := pgtest.Migrator(t, db)
	require.NoError(t, migrator.UpTo(t.Context(), 113))
	_, err := masterseeder.SeedReference(t.Context(), db)
	require.NoError(t, err)
	// Seeder akun demo menulis pengguna.telepon (000123, slice 20-A), kolom yang belum ada pada skema 113; tes ini
	// berhenti di 000114, jadi kolomnya ditambahkan di sini supaya seeder akun tetap bisa dipakai pada skema lama.
	require.NoError(t, db.Exec(`ALTER TABLE pengguna ADD COLUMN telepon varchar(20)`).Error)
	_, err = authseeder.SeedDemo(t.Context(), db)
	require.NoError(t, err)

	// Brief demo berbentuk lama: id varchar PB-YYYY-MM, konten kosong (dari sebelum 15-B).
	rows, err := BuildBrief(now)
	require.NoError(t, err)
	for _, r := range rows {
		require.NoError(t, db.Exec(`INSERT INTO policy_brief (id, periode, judul, status, ringkasan, rekomendasi, halaman, disusun_oleh_id)
			VALUES (?, ?, ?, ?::status_brief, ?, ?::jsonb, ?, ?)`, r.Kode, r.Periode, r.Judul, r.Status, r.Ringkasan, r.Rekomendasi, r.Halaman, r.DisusunOlehID).Error)
	}
	require.NoError(t, migrator.UpTo(t.Context(), 114))
	for _, r := range rows {
		var id string
		require.NoError(t, db.Raw(`SELECT id::text FROM policy_brief WHERE kode = ?`, r.Kode).Scan(&id).Error)
		assert.Equal(t, r.ID, id, "UUID hasil migrasi sama dengan UUID seed: "+r.Kode)
	}

	_, err = SeedDemo(t.Context(), db, now)
	require.NoError(t, err)

	var n int64
	require.NoError(t, db.Table("policy_brief").Count(&n).Error)
	assert.EqualValues(t, 5, n, "seed pada DB hasil migrasi tidak menggandakan brief")
	var label string
	require.NoError(t, db.Raw(`SELECT konten->>'label' FROM policy_brief WHERE kode = 'PB-2026-10'`).Scan(&label).Error)
	assert.Equal(t, domain.LabelDemo, label, "konten kosong dilengkapi")
}

func TestBuildBrief_KodeUnikBerbentukPBYYYYMM(t *testing.T) {
	rows, err := BuildBrief(now)
	require.NoError(t, err)

	terlihat := map[string]bool{}
	for _, r := range rows {
		assert.Regexp(t, `^PB-[0-9]{4}-(0[1-9]|1[0-2])$`, r.Kode)
		assert.True(t, idx.ValidUUID(r.ID), r.ID)
		assert.False(t, terlihat[r.Kode], "kode ganda: "+r.Kode)
		terlihat[r.Kode] = true
	}
}
