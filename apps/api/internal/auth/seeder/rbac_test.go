package seeder

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/garda-inflasi/api/internal/auth/usecase"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
)

func rbacDB(t *testing.T, nama string) *gorm.DB {
	t.Helper()
	db := pg.NewDatabase(t, nama)
	pgtest.MigrateUp(t, db)
	return db
}

func izinPeran(t *testing.T, db *gorm.DB, peran string) []string {
	t.Helper()
	var izin []string
	require.NoError(t, db.Raw("SELECT permission_nama FROM role_permissions WHERE role_kunci = ? ORDER BY permission_nama", peran).Scan(&izin).Error)
	return izin
}

func TestSeedRBAC_MengisiEmpatBelasPeranTigaPuluhLimaIzinDanMatriksBawaan(t *testing.T) {
	db := rbacDB(t, "auth_seed_rbac_awal")

	sum, err := SeedRBAC(t.Context(), db)

	require.NoError(t, err)
	assert.Equal(t, 14, sum.PeranBaru)
	assert.Equal(t, 35, sum.IzinBaru)
	assert.EqualValues(t, 14, rowCount(t, db, "roles"))
	assert.EqualValues(t, 35, rowCount(t, db, "permissions"))

	total := 0
	for _, p := range usecase.DaftarPeran() {
		want := slices.Sorted(slices.Values(usecase.IzinBawaan(p.Kunci)))
		assert.Equal(t, want, izinPeran(t, db, p.Kunci), "matriks bawaan %s", p.Kunci)
		total += len(want)
	}
	assert.Equal(t, total, sum.PenugasanBaru)
	assert.EqualValues(t, total, rowCount(t, db, "role_permissions"))

	var label, lapis string
	var versi int64
	require.NoError(t, db.Raw("SELECT label, lapis::text, versi FROM roles WHERE kunci = 'MANAJER_DATA'").Row().Scan(&label, &lapis, &versi))
	assert.Equal(t, "Unit Data & Statistik", label)
	assert.Equal(t, "MANAJERIAL", lapis)
	assert.EqualValues(t, 1, versi)
	assert.EqualValues(t, 0, rowCount(t, db, "user_permissions"), "seed tidak memberi izin langsung")
}

func TestSeedRBAC_IdempotenDanTidakMenimpaPerubahanAdmin(t *testing.T) {
	db := rbacDB(t, "auth_seed_rbac_idempoten")
	_, err := SeedRBAC(t.Context(), db)
	require.NoError(t, err)

	// Perubahan admin: mencabut izin bawaan, menambah izin lain, dan menaikkan versi lewat UI.
	require.NoError(t, db.Exec("DELETE FROM role_permissions WHERE role_kunci = 'OPS_MAHASISWA' AND permission_nama = 'tugas.lihat_sendiri'").Error)
	require.NoError(t, db.Exec("INSERT INTO role_permissions (role_kunci, permission_nama) VALUES ('OPS_MAHASISWA', 'kpi.lihat')").Error)
	require.NoError(t, db.Exec("UPDATE roles SET versi = 7, diperbarui_pada = now() WHERE kunci = 'OPS_MAHASISWA'").Error)
	setelahAdmin := izinPeran(t, db, "OPS_MAHASISWA")

	again, err := SeedRBAC(t.Context(), db)

	require.NoError(t, err)
	assert.Equal(t, RBACSummary{}, again, "seed ulang tidak menulis apa pun")
	assert.Equal(t, setelahAdmin, izinPeran(t, db, "OPS_MAHASISWA"), "izin yang dicabut admin tidak dikembalikan dan yang ditambah tidak dihapus")
	var versi int64
	require.NoError(t, db.Raw("SELECT versi FROM roles WHERE kunci = 'OPS_MAHASISWA'").Row().Scan(&versi))
	assert.EqualValues(t, 7, versi, "versi milik admin tidak diubah seed")
}

func TestSeedRBAC_KatalogDariKodeDisegarkanTanpaMenyentuhKolomAdmin(t *testing.T) {
	db := rbacDB(t, "auth_seed_rbac_katalog")
	_, err := SeedRBAC(t.Context(), db)
	require.NoError(t, err)
	require.NoError(t, db.Exec("UPDATE roles SET label = 'usang', versi = 4 WHERE kunci = 'ADMIN_SISTEM'").Error)
	require.NoError(t, db.Exec("UPDATE permissions SET deskripsi = 'usang', kelompok = 'usang' WHERE nama = 'kpi.lihat'").Error)

	_, err = SeedRBAC(t.Context(), db)
	require.NoError(t, err)

	var label string
	var versi int64
	require.NoError(t, db.Raw("SELECT label, versi FROM roles WHERE kunci = 'ADMIN_SISTEM'").Row().Scan(&label, &versi))
	assert.Equal(t, "Administrator Sistem", label, "label peran adalah katalog di kode")
	assert.EqualValues(t, 4, versi)
	var deskripsi string
	require.NoError(t, db.Raw("SELECT deskripsi FROM permissions WHERE nama = 'kpi.lihat'").Row().Scan(&deskripsi))
	assert.Equal(t, "Melihat indikator kinerja (KPI)", deskripsi)
}

func TestSeedRBAC_IzinBaruDiKatalogDiberikanKePeranBawaannyaSekali(t *testing.T) {
	db := rbacDB(t, "auth_seed_rbac_izin_baru")
	_, err := SeedRBAC(t.Context(), db)
	require.NoError(t, err)

	// Rilis lama belum mengenal kpi.lihat, lalu admin mengubah matriks peran lain.
	require.NoError(t, db.Exec("DELETE FROM permissions WHERE nama = 'kpi.lihat'").Error) // CASCADE ke role_permissions
	require.NoError(t, db.Exec("DELETE FROM role_permissions WHERE role_kunci = 'OPS_MAHASISWA' AND permission_nama = 'harga.input'").Error)
	dicabutAdmin := izinPeran(t, db, "OPS_MAHASISWA")

	sum, err := SeedRBAC(t.Context(), db)

	require.NoError(t, err)
	assert.Equal(t, 0, sum.PeranBaru)
	assert.Equal(t, 1, sum.IzinBaru)
	assert.Equal(t, len(usecase.RolesWithPermission("kpi.lihat")), sum.PenugasanBaru, "hanya penugasan bawaan izin baru itu")
	assert.Equal(t, usecase.RolesWithPermission("kpi.lihat"), peranPemilik(t, db, "kpi.lihat"))
	assert.Equal(t, dicabutAdmin, izinPeran(t, db, "OPS_MAHASISWA"), "pencabutan admin atas izin lain tetap")

	// Sudah ada: pencabutan admin berikutnya atas izin baru itu tidak dikembalikan.
	require.NoError(t, db.Exec("DELETE FROM role_permissions WHERE role_kunci = 'MANAJER_DATA' AND permission_nama = 'kpi.lihat'").Error)
	_, err = SeedRBAC(t.Context(), db)
	require.NoError(t, err)
	assert.NotContains(t, izinPeran(t, db, "MANAJER_DATA"), "kpi.lihat")
}

func peranPemilik(t *testing.T, db *gorm.DB, izin string) []string {
	t.Helper()
	var peran []string
	require.NoError(t, db.Raw(`SELECT r.kunci::text FROM role_permissions rp JOIN roles r ON r.kunci = rp.role_kunci
		WHERE rp.permission_nama = ? ORDER BY r.urutan`, izin).Scan(&peran).Error)
	return peran
}

func TestSeedDemo_JugaMemastikanMatriksBawaanAda(t *testing.T) {
	db := rbacDB(t, "auth_seed_demo_rbac")
	seedWilayahDemo(t, db)

	_, err := SeedDemo(t.Context(), db)

	require.NoError(t, err)
	assert.EqualValues(t, 14, rowCount(t, db, "roles"))
	assert.Equal(t, slices.Sorted(slices.Values(usecase.IzinBawaan("ADMIN_SISTEM"))), izinPeran(t, db, "ADMIN_SISTEM"))
}

func TestSeedRBAC_FKDanConstraintMenjagaIntegritas(t *testing.T) {
	db := rbacDB(t, "auth_seed_rbac_constraint")
	_, err := SeedRBAC(t.Context(), db)
	require.NoError(t, err)

	assert.Error(t, db.Exec("INSERT INTO role_permissions (role_kunci, permission_nama) VALUES ('OPS_MAHASISWA', 'izin.palsu')").Error, "izin harus ada di katalog")
	assert.Error(t, db.Exec("INSERT INTO permissions (nama, deskripsi, kelompok) VALUES ('BukanFormat', 'x', 'y')").Error, "nama izin berformat domain.aksi")
	assert.Error(t, db.Exec("INSERT INTO user_permissions (pengguna_id, permission_nama, alasan) VALUES (garda_uuid_stabil('pengguna', 'hantu'), 'kpi.lihat', 'x')").Error, "pengguna harus ada")

	require.NoError(t, db.Exec(`INSERT INTO pengguna (id, nama, email, peran, lapis, lembaga) VALUES (garda_uuid_stabil('pengguna', 'u1'), 'U', 'u1@x.id', 'OPS_MAHASISWA', 'OPERASIONAL', 'L')`).Error)
	assert.Error(t, db.Exec("INSERT INTO user_permissions (pengguna_id, permission_nama, alasan) VALUES (garda_uuid_stabil('pengguna', 'u1'), 'kpi.lihat', '   ')").Error, "alasan wajib")
	require.NoError(t, db.Exec("INSERT INTO user_permissions (pengguna_id, permission_nama, alasan) VALUES (garda_uuid_stabil('pengguna', 'u1'), 'kpi.lihat', 'uji')").Error)
	assert.Error(t, db.Exec("INSERT INTO user_permissions (pengguna_id, permission_nama, alasan) VALUES (garda_uuid_stabil('pengguna', 'u1'), 'kpi.lihat', 'ganda')").Error, "satu baris per (pengguna, izin)")

	require.NoError(t, db.Exec("DELETE FROM pengguna WHERE id = garda_uuid_stabil('pengguna', 'u1')").Error)
	assert.EqualValues(t, 0, rowCount(t, db, "user_permissions"), "izin langsung ikut terhapus bersama penggunanya")
}
