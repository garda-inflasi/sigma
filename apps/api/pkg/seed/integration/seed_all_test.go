// Package integration menguji seluruh tier seed SQL (reference, admin, demo) terhadap PostgreSQL sungguhan.
package integration

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/pkg/seed"
)

var pg *pgtest.Server

func TestMain(m *testing.M) {
	os.Exit(pgtest.Main(m, &pg))
}

// TestSeedAll_DuaKaliTanpaGalat menjalankan urutan `seed all` dua kali di basis data yang sama. Urutan ini
// pernah patah di demo/15_audit_trail.sql: tier admin menanam superadmin dengan id lain, lalu akun demo
// ber-email sama dilewati sehingga aktor_id audit merujuk pengguna yang tidak ada.
func TestSeedAll_DuaKaliTanpaGalat(t *testing.T) {
	if pg == nil {
		t.Skip("butuh PostgreSQL (jalankan tanpa -short)")
	}
	db := pg.NewDatabase(t, "seed_all")
	pgtest.MigrateUp(t, db)

	require.NoError(t, seed.SeedAll(t.Context(), db), "seed all pertama")
	require.NoError(t, seed.SeedAll(t.Context(), db), "seed all kedua")

	var audit struct {
		Total      int64
		TanpaAktor int64
		Superadmin int64
		MasaDepan  int64
	}
	require.NoError(t, db.Raw(`
		SELECT count(*)                                                  AS total,
		       count(*) FILTER (WHERE a.aktor_id IS NULL)                AS tanpa_aktor,
		       count(*) FILTER (WHERE p.email = 'superadmin@gardainflasi.id') AS superadmin,
		       count(*) FILTER (WHERE a.waktu > now())                   AS masa_depan
		FROM audit_log a
		LEFT JOIN pengguna p ON p.id = a.aktor_id
		WHERE a.kode LIKE 'demo-audit-%'`).Scan(&audit).Error)

	require.EqualValues(t, 80, audit.Total, "jejak audit demo tidak boleh terduplikasi")
	require.Zero(t, audit.TanpaAktor, "setiap jejak audit demo harus punya aktor")
	require.EqualValues(t, 3, audit.Superadmin, "aksi superadmin harus menunjuk akun dari tier admin")
	require.Zero(t, audit.MasaDepan, "waktu jejak audit demo tidak boleh di masa depan")

	var pengguna int64
	require.NoError(t, db.Raw(`SELECT count(*) FROM pengguna WHERE email = 'superadmin@gardainflasi.id'`).
		Scan(&pengguna).Error)
	require.EqualValues(t, 1, pengguna)
}
