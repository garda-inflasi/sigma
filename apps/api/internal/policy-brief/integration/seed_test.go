package integration

import (
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/internal/policy-brief/domain"
	"github.com/garda-inflasi/api/pkg/seed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Tes seed SQL policy brief (db/seeds/demo/12_policy_brief.sql). Pengganti tes seeder Go lama: lapis produksi tidak
// punya brief contoh, brief contoh berlabel demo dengan waktu relatif terhadap saat seed, seed ulang idempoten dan
// tidak menimpa perubahan, tetapi melengkapi brief contoh yang kontennya masih kosong.

func hitungBrief(t *testing.T, db *gorm.DB, kondisi string, args ...any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Table("policy_brief").Where(kondisi, args...).Count(&n).Error)
	return n
}

func labelDanSumber(t *testing.T, db *gorm.DB, kode string) (label, sumber string) {
	t.Helper()
	require.NoError(t, db.Raw(`SELECT konten->>'label' FROM policy_brief WHERE kode = ?`, kode).Scan(&label).Error)
	require.NoError(t, db.Raw(`SELECT sumber_narasi FROM policy_brief WHERE kode = ?`, kode).Scan(&sumber).Error)
	return label, sumber
}

func TestSeed_LapisReferenceTidakMembawaBriefContoh(t *testing.T) {
	db := pg.NewDatabase(t, "brief_seed_reference")
	pgtest.MigrateUp(t, db)
	require.NoError(t, seed.SeedReference(t.Context(), db))

	assert.Zero(t, hitungBrief(t, db, "TRUE"), "brief contoh hanya ada di lapis demo")
}

func TestSeed_BriefContohBerlabelDemoDanWaktuRelatifTerhadapSaatSeed(t *testing.T) {
	db := pg.NewDatabase(t, "brief_seed_waktu")
	pgtest.MigrateUp(t, db)
	ctx := t.Context()
	require.NoError(t, seed.SeedReference(ctx, db))

	sebelum := jamDB(t, db)
	require.NoError(t, seed.SeedFiles(ctx, db, "demo/01_auth_users.sql", "demo/12_policy_brief.sql"))
	sesudah := jamDB(t, db)

	assert.EqualValues(t, 5, hitungBrief(t, db, "TRUE"))
	assert.EqualValues(t, 5, hitungBrief(t, db, "sumber_narasi = 'DEMO' AND konten->>'label' = ?", domain.LabelDemo))
	assert.EqualValues(t, 5, hitungBrief(t, db, "disusun_oleh_id = garda_uuid_stabil('pengguna', 'manajer_kebijakan')"))
	assert.EqualValues(t, 5, hitungBrief(t, db, "dibuat_pada = diperbarui - interval '72 hours'"))

	// PB-2026-10 diperbarui 45 menit sebelum seed dijalankan; PB-2026-06 138.000 menit (95 hari 20 jam) sebelumnya.
	var diperbarui, lama time.Time
	require.NoError(t, db.Raw(`SELECT diperbarui FROM policy_brief WHERE kode = 'PB-2026-10'`).Scan(&diperbarui).Error)
	require.NoError(t, db.Raw(`SELECT diperbarui FROM policy_brief WHERE kode = 'PB-2026-06'`).Scan(&lama).Error)
	assert.False(t, diperbarui.Before(sebelum.Add(-45*time.Minute-time.Second)), "diperbarui tidak lebih awal dari 45 menit sebelum seed: %s", diperbarui)
	assert.False(t, diperbarui.After(sesudah.Add(-45*time.Minute+time.Second)), "diperbarui tidak lebih lambat dari 45 menit sebelum seed selesai: %s", diperbarui)
	assert.InDelta(t, (138000*time.Minute - 45*time.Minute).Seconds(), diperbarui.Sub(lama).Seconds(), 1)
}

func TestSeed_BriefContohIdempotenTidakMenimpaPerubahanDanMelengkapiKontenKosong(t *testing.T) {
	db := pg.NewDatabase(t, "brief_seed_idempoten")
	pgtest.MigrateUp(t, db)
	ctx := t.Context()
	require.NoError(t, seed.SeedReference(ctx, db))
	berkas := []string{"demo/01_auth_users.sql", "demo/12_policy_brief.sql"}
	require.NoError(t, seed.SeedFiles(ctx, db, berkas...))

	require.NoError(t, db.Exec("UPDATE policy_brief SET status = 'TELAAH' WHERE kode = 'PB-2026-10'").Error)
	require.NoError(t, seed.SeedFiles(ctx, db, "demo/12_policy_brief.sql"))

	assert.EqualValues(t, 5, hitungBrief(t, db, "TRUE"))
	var status string
	require.NoError(t, db.Raw("SELECT status::text FROM policy_brief WHERE kode = 'PB-2026-10'").Scan(&status).Error)
	assert.Equal(t, "TELAAH", status, "seed ulang tidak menimpa perubahan")

	// Brief contoh dari sebelum kolom konten ada (konten kosong) dilengkapi; yang sudah berisi tidak ditimpa.
	require.NoError(t, db.Exec(`UPDATE policy_brief SET konten = '{}', sumber_narasi = 'TEMPLAT' WHERE kode = 'PB-2026-10'`).Error)
	require.NoError(t, db.Exec(`UPDATE policy_brief SET konten = '{"label":"diubah"}' WHERE kode = 'PB-2026-09'`).Error)

	require.NoError(t, seed.SeedFiles(ctx, db, "demo/12_policy_brief.sql"))

	label, sumber := labelDanSumber(t, db, "PB-2026-10")
	assert.Equal(t, domain.LabelDemo, label)
	assert.Equal(t, "DEMO", sumber)
	label, _ = labelDanSumber(t, db, "PB-2026-09")
	assert.Equal(t, "diubah", label)
	assert.EqualValues(t, 5, hitungBrief(t, db, "TRUE"), "brief tidak digandakan")
}

// jamDB membaca jam PostgreSQL. Waktu seed dihitung dari now() basis data, jadi pembandingnya harus jam yang sama:
// jam container Docker bisa tertinggal puluhan detik sampai menit dari host (mis. setelah Mac tidur).
func jamDB(t *testing.T, db *gorm.DB) time.Time {
	t.Helper()
	var jam time.Time
	require.NoError(t, db.Raw(`SELECT clock_timestamp()`).Scan(&jam).Error)
	return jam
}
