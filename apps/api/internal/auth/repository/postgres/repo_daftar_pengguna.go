package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/garda-inflasi/api/internal/auth/domain"
	"gorm.io/gorm"
)

// ── Daftar pengguna sisi server ───────────────────────────────────────

var _ domain.DaftarPenggunaRepository = (*Repository)(nil)

// ekspresiUrutPengguna memetakan nilai `sort` (daftar putih domain.KolomUrutPengguna) ke ekspresi SQL. Nilai
// dari klien TIDAK PERNAH disambung ke SQL: hanya string di peta ini yang dipakai.
//
//   - nama, email, lembaga: tak peka huruf besar-kecil (LOWER) supaya urutannya sama di semua collation.
//   - peran, status: urutan deklarasi enum basis data. Untuk peran itu hierarki lapis (Strategis, Manajer,
//     Operasional, Sistem), bukan abjad id.
//   - terakhir_masuk: akun yang belum pernah masuk (NULL) selalu paling akhir, apa pun arahnya.
var ekspresiUrutPengguna = map[string]string{
	domain.UrutNama:          "LOWER(nama)",
	domain.UrutEmail:         "LOWER(email)",
	domain.UrutPeran:         "peran",
	domain.UrutLembaga:       "LOWER(lembaga)",
	domain.UrutStatus:        "status",
	domain.UrutTerakhirMasuk: "terakhir_masuk",
	domain.UrutDibuatPada:    "dibuat_pada",
}

// hashEmailKunciSQL menghitung kunci_login.email_hash dari kolom email di SQL: SHA-256 heksadesimal dari email
// huruf kecil tanpa spasi tepi, sama dengan usecase.HashEmailKunci. Dipakai filter `terkunci` supaya penyaringan
// dan paginasi terjadi di basis data, bukan di memori. Perbedaan dengan Go hanya untuk email berspasi
// Unicode atau huruf non-ASCII yang aturan huruf kecilnya berbeda; email pengguna tervalidasi ASCII tanpa spasi.
const hashEmailKunciSQL = `encode(sha256(convert_to(lower(btrim(pengguna.email, E' \t\r\n')), 'UTF8')), 'hex')`

// lepasLike meloloskan metakarakter LIKE (\, %, _) supaya kata kunci dicocokkan apa adanya: mencari "50%"
// atau "a_b" tidak boleh menjadi pola.
func lepasLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// terapkanFilterPengguna menambahkan syarat filter ke query (nilai kosong = tanpa syarat). Satu-satunya tempat
// kriteria daftar pengguna ditulis: ListUsers (GET /v1/users) dan ListUsersUntukEkspor (GET /v1/users/ekspor)
// sama-sama memakainya, jadi ekspor selalu memuat persis yang tampil di daftar dengan filter yang sama.
// sekarang adalah waktu acuan filter Terkunci.
func terapkanFilterPengguna(db *gorm.DB, f domain.FilterPengguna, sekarang time.Time) *gorm.DB {
	if f.Cari != "" {
		kata := "%" + lepasLike(strings.ToLower(f.Cari)) + "%"
		// peran bertipe enum PostgreSQL, jadi di-cast ke text sebelum LOWER(). Versi dengan spasi menolong
		// pencarian "admin sistem" menemukan ADMIN_SISTEM, seperti label yang dilihat pengguna di tabel.
		db = db.Where(
			"(LOWER(nama) LIKE ? OR LOWER(email) LIKE ? OR LOWER(peran::text) LIKE ? OR LOWER(REPLACE(peran::text, '_', ' ')) LIKE ? OR LOWER(lembaga) LIKE ?)",
			kata, kata, kata, kata, kata,
		)
	}
	if f.Peran != "" {
		db = db.Where("peran = ?", f.Peran)
	}
	if f.Status != "" {
		db = db.Where("status = ?", f.Status)
	}
	if f.Lapis != "" {
		db = db.Where("lapis = ?", f.Lapis)
	}
	if f.MFA != nil {
		db = db.Where("mfa_aktif = ?", *f.MFA)
	}
	if f.Terkunci != nil {
		terkunci := "EXISTS (SELECT 1 FROM kunci_login k WHERE k.email_hash = " + hashEmailKunciSQL + " AND k.terkunci_sampai > ?)"
		if !*f.Terkunci {
			terkunci = "NOT " + terkunci
		}
		db = db.Where(terkunci, sekarang)
	}
	if len(f.IDs) > 0 {
		db = db.Where("id IN ?", f.IDs)
	}
	return db
}

// urutanPengguna menyusun ORDER BY: kolom pilihan lalu id pada arah yang sama. Tiebreaker id membuat urutan
// total, jadi paginasi (OFFSET) stabil walau nilai kolom urut kembar, dan membalik arah membalik seluruh urutan.
func urutanPengguna(q domain.DaftarPenggunaQuery) string {
	ekspresi, ok := ekspresiUrutPengguna[q.Urut]
	if !ok {
		ekspresi = ekspresiUrutPengguna[domain.UrutDibuatPada]
	}
	arah := "ASC"
	if q.Arah == domain.ArahTurun {
		arah = "DESC"
	}
	urutan := ekspresi + " " + arah
	if q.Urut == domain.UrutTerakhirMasuk {
		urutan += " NULLS LAST"
	}
	return urutan + ", id " + arah
}

func (r *Repository) ListUsers(ctx context.Context, q domain.DaftarPenggunaQuery) ([]domain.Pengguna, int64, error) {
	var users []domain.Pengguna
	var total int64

	// Session membuat query dasar dipakai ulang oleh Count dan Find tanpa saling mengubah statement.
	query := terapkanFilterPengguna(r.db.WithContext(ctx).Model(&domain.Pengguna{}), q.Filter, q.Sekarang).Session(&gorm.Session{})
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	if err := query.Order(urutanPengguna(q)).Offset(q.Offset()).Limit(q.Limit).Find(&users).Error; err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	return users, total, nil
}
