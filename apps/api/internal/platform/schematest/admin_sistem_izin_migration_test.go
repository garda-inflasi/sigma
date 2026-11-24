package schematest

import (
	"context"
	"testing"

	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Migrasi 000130 mengisi role_permissions ADMIN_SISTEM dari seluruh baris permissions. Dipatok ke versi 129 dan 130.
const (
	versiSebelumAdminPenuh = 129
	versiAdminPenuh        = 130
)

var izinAdminLama = []string{
	"audit.lihat", "impersonasi.kelola", "master.ajukan", "master.lihat", "pengguna.kelola", "sumber.kelola", "sumber.lihat",
}

func izinAdminDiTabel(t *testing.T, db *gorm.DB) []string {
	t.Helper()
	var izin []string
	require.NoError(t, db.Raw(`SELECT permission_nama FROM role_permissions WHERE role_kunci = 'ADMIN_SISTEM' ORDER BY 1`).Scan(&izin).Error)
	return izin
}

func versiPeranAdmin(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	return scanCount(t, db, `SELECT versi FROM roles WHERE kunci = 'ADMIN_SISTEM'`)
}

func TestMigrasiAdminSistemSeluruhIzin(t *testing.T) {
	ctx := context.Background()
	if pg == nil {
		t.Skip("tes migrasi dilewati (-short, tanpa Docker)")
	}
	db := pg.NewDatabase(t, "migrasi_admin_penuh")
	m := pgtest.Migrator(t, db)
	require.NoError(t, m.UpTo(ctx, versiSebelumAdminPenuh))

	require.NoError(t, db.Exec(`INSERT INTO roles (kunci, label, lapis, urutan) VALUES
		('ADMIN_SISTEM', 'Administrator Sistem', 'SISTEM', 14), ('OPS_MAHASISWA', 'Agen', 'OPERASIONAL', 13)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO permissions (nama, deskripsi, kelompok)
		SELECT nama, nama, 'Uji' FROM unnest(CAST(? AS text[])) AS nama`,
		"{audit.lihat,impersonasi.kelola,master.ajukan,master.lihat,pengguna.kelola,sumber.kelola,sumber.lihat,harga.validasi,kpi.lihat}").Error)
	require.NoError(t, db.Exec(`INSERT INTO role_permissions (role_kunci, permission_nama)
		SELECT 'ADMIN_SISTEM', nama FROM unnest(CAST(? AS text[])) AS nama`,
		"{audit.lihat,impersonasi.kelola,master.ajukan,master.lihat,pengguna.kelola,sumber.kelola,sumber.lihat}").Error)
	require.NoError(t, db.Exec(`INSERT INTO role_permissions (role_kunci, permission_nama) VALUES ('OPS_MAHASISWA', 'kpi.lihat')`).Error)

	require.NoError(t, m.UpTo(ctx, versiAdminPenuh))
	assert.Len(t, izinAdminDiTabel(t, db), 9, "seluruh baris permissions")
	assert.EqualValues(t, 2, versiPeranAdmin(t, db), "versi naik karena isinya berubah")
	assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM role_permissions WHERE role_kunci = 'OPS_MAHASISWA'`), "peran lain tidak disentuh")

	require.NoError(t, m.DownTo(ctx, versiSebelumAdminPenuh))
	assert.Equal(t, izinAdminLama, izinAdminDiTabel(t, db), "down kembali ke 7 izin lama")
	assert.EqualValues(t, 3, versiPeranAdmin(t, db))
	assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM role_permissions WHERE role_kunci = 'OPS_MAHASISWA'`))

	// Idempoten: bila isinya sudah lengkap, versi tidak naik (draf matriks yang terbuka tidak dibuang tanpa sebab).
	require.NoError(t, db.Exec(`INSERT INTO role_permissions (role_kunci, permission_nama) VALUES
		('ADMIN_SISTEM', 'harga.validasi'), ('ADMIN_SISTEM', 'kpi.lihat')`).Error)
	require.NoError(t, m.UpTo(ctx, versiAdminPenuh))
	assert.Len(t, izinAdminDiTabel(t, db), 9)
	assert.EqualValues(t, 3, versiPeranAdmin(t, db))
}

func TestMigrasiAdminSistemSeluruhIzin_BasisDataKosongTetapKosong(t *testing.T) {
	ctx := context.Background()
	if pg == nil {
		t.Skip("tes migrasi dilewati (-short, tanpa Docker)")
	}
	db := pg.NewDatabase(t, "migrasi_admin_kosong")
	m := pgtest.Migrator(t, db)
	require.NoError(t, m.UpTo(ctx, versiAdminPenuh))
	assert.Empty(t, izinAdminDiTabel(t, db))
	require.NoError(t, m.DownTo(ctx, versiSebelumAdminPenuh))
	assert.Empty(t, izinAdminDiTabel(t, db))
}
