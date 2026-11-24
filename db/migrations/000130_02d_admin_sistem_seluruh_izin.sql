-- Slice 02-D: ADMIN_SISTEM memegang seluruh katalog izin.
--
-- Migrasi ini menyamakan isi role_permissions dengan aturan yang sudah dijamin saat runtime (terapkanIzinEksklusif di
-- apps/api/internal/auth/usecase/impersonasi.go), supaya matriks, audit, dan laporan yang membaca tabel tidak menyimpang.
-- Pemisahan tugas per aksi tidak berubah: pencatat, validator, dan penerbit harga; pengaju dan penyetuju master;
-- pengaju dan pemutus intervensi; penyusun dan penyetuju policy brief tetap orang yang berbeda.
--
-- Idempoten: pada basis data kosong (katalog belum diseed) tidak ada baris yang ditulis, dan versi peran hanya naik bila
-- isinya benar-benar berubah, supaya draf matriks yang sedang terbuka tidak dibuang tanpa sebab. Cache izin efektif
-- berumur paling lama 15 detik, dan token dengan klaim lama dijawab 401 PERMISSIONS_CHANGED lalu disegarkan otomatis.

-- +goose Up
-- +goose StatementBegin
WITH baru AS (
    INSERT INTO role_permissions (role_kunci, permission_nama)
    SELECT r.kunci, p.nama
      FROM roles r CROSS JOIN permissions p
     WHERE r.kunci = 'ADMIN_SISTEM'
    ON CONFLICT (role_kunci, permission_nama) DO NOTHING
    RETURNING 1
)
UPDATE roles
   SET versi = versi + 1, diperbarui_pada = now(), diperbarui_oleh = NULL
 WHERE kunci = 'ADMIN_SISTEM' AND EXISTS (SELECT 1 FROM baru);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Kembali ke 7 izin lama. Catatan: selama kode 02-D masih berjalan, izin efektif ADMIN_SISTEM tetap seluruh katalog.
WITH lama (nama) AS (
    VALUES ('pengguna.kelola'), ('impersonasi.kelola'), ('master.lihat'), ('master.ajukan'),
           ('sumber.lihat'), ('sumber.kelola'), ('audit.lihat')
), dicabut AS (
    DELETE FROM role_permissions
     WHERE role_kunci = 'ADMIN_SISTEM' AND permission_nama NOT IN (SELECT nama FROM lama)
    RETURNING 1
), dipulihkan AS (
    INSERT INTO role_permissions (role_kunci, permission_nama)
    SELECT r.kunci, p.nama
      FROM roles r JOIN permissions p ON p.nama IN (SELECT nama FROM lama)
     WHERE r.kunci = 'ADMIN_SISTEM'
    ON CONFLICT (role_kunci, permission_nama) DO NOTHING
    RETURNING 1
)
UPDATE roles
   SET versi = versi + 1, diperbarui_pada = now(), diperbarui_oleh = NULL
 WHERE kunci = 'ADMIN_SISTEM'
   AND (EXISTS (SELECT 1 FROM dicabut) OR EXISTS (SELECT 1 FROM dipulihkan));
-- +goose StatementEnd
