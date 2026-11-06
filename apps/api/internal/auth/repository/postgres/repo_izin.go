package postgres

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/garda-inflasi/api/internal/auth/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Implementasi domain.IzinRepository: RBAC berbasis basis data (Opsi B, migrasi 000107).
var _ domain.IzinRepository = (*Repository)(nil)

// kunciPenasihatPemegang menyerialkan semua perubahan yang dapat mengurangi pemegang IzinPengelolaan (matriks
// peran, ganti peran, penonaktifan, penghapusan). Dua admin yang serentak menurunkan dua pemegang terakhir
// tidak boleh sama-sama lolos: tanpa kunci keduanya melihat "masih ada satu lagi". Selalu diambil lebih dulu
// sebelum kunci baris mana pun, supaya urutan kunci konsisten dan tidak ada deadlock.
const kunciPenasihatPemegang int64 = 7020301

// kunciPemegang mengambil kunci penasihat tingkat transaksi (dilepas saat commit atau rollback).
func kunciPemegang(tx *gorm.DB) error {
	if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", kunciPenasihatPemegang).Error; err != nil {
		return fmt.Errorf("kunci pemegang %s: %w", domain.IzinPengelolaan, err)
	}
	return nil
}

// hitungPemegang menghitung pengguna AKTIF yang memegang IzinPengelolaan lewat perannya. Izin langsung tidak
// dihitung: ia bisa kedaluwarsa atau dicabut tanpa melewati penjaga ini, jadi tidak boleh menjadi jaminan.
func hitungPemegang(tx *gorm.DB) (int64, error) {
	var n int64
	err := tx.Raw(`SELECT count(*) FROM pengguna p
		 WHERE p.status = 'AKTIF'
		   AND EXISTS (SELECT 1 FROM role_permissions rp WHERE rp.role_kunci = p.peran AND rp.permission_nama = ?)`,
		domain.IzinPengelolaan).Scan(&n).Error
	if err != nil {
		return 0, fmt.Errorf("hitung pemegang %s: %w", domain.IzinPengelolaan, err)
	}
	return n, nil
}

// jagaPemegang menjalankan tulis di dalam transaksi tx dan menolaknya (ErrPemegangTerakhir, transaksi dibatalkan
// oleh pemanggil) bila sebelum perubahan ada pemegang tetapi sesudahnya tidak ada. Bila sebelumnya memang tidak
// ada pemegang (basis data tanpa admin, mis. tes) tidak ada yang dilindungi dan perubahan lain tidak boleh terhalang.
func jagaPemegang(tx *gorm.DB, tulis func() error) error {
	if err := kunciPemegang(tx); err != nil {
		return err
	}
	sebelum, err := hitungPemegang(tx)
	if err != nil {
		return err
	}
	if err := tulis(); err != nil {
		return err
	}
	if sebelum == 0 {
		return nil
	}
	sesudah, err := hitungPemegang(tx)
	if err != nil {
		return err
	}
	if sesudah == 0 {
		return domain.ErrPemegangTerakhir
	}
	return nil
}

// ── Izin efektif (jalur panas: dibaca JWTAuth lewat cache) ────────────

func (r *Repository) IzinEfektifPengguna(ctx context.Context, userID string) (*domain.IzinEfektif, error) {
	var baris []struct {
		Sumber    string
		Izin      string
		SisaDetik *float64
	}
	// Satu pernyataan: akun (harus AKTIF), izin peran, dan izin langsung yang masih berlaku. Sisa waktu dihitung
	// oleh jam basis data supaya selisih jam aplikasi dan basis data tidak menggeser batas kedaluwarsa.
	err := r.db.WithContext(ctx).Raw(`
		WITH u AS (SELECT id, peran FROM pengguna WHERE id = ? AND status = 'AKTIF')
		SELECT 'akun' AS sumber, u.peran::text AS izin, NULL::float8 AS sisa_detik FROM u
		UNION ALL
		SELECT 'peran', rp.permission_nama, NULL::float8
		  FROM u JOIN role_permissions rp ON rp.role_kunci = u.peran
		UNION ALL
		SELECT 'langsung', up.permission_nama, EXTRACT(EPOCH FROM (up.berlaku_sampai - now()))::float8
		  FROM u JOIN user_permissions up ON up.pengguna_id = u.id
		 WHERE up.berlaku_sampai IS NULL OR up.berlaku_sampai > now()`, userID).Scan(&baris).Error
	if err != nil {
		return nil, fmt.Errorf("baca izin efektif: %w", err)
	}

	hasil := &domain.IzinEfektif{}
	ada := false
	seen := map[string]bool{}
	for _, b := range baris {
		if b.Sumber == "akun" {
			ada = true
			hasil.Peran = b.Izin
			continue
		}
		if !seen[b.Izin] {
			seen[b.Izin] = true
			hasil.Izin = append(hasil.Izin, b.Izin)
		}
		if b.SisaDetik != nil {
			sisa := time.Duration(*b.SisaDetik * float64(time.Second))
			if hasil.KedaluwarsaDalam == 0 || sisa < hasil.KedaluwarsaDalam {
				hasil.KedaluwarsaDalam = sisa
			}
		}
	}
	if !ada {
		return nil, gorm.ErrRecordNotFound
	}
	slices.Sort(hasil.Izin)
	if hasil.Izin == nil {
		hasil.Izin = []string{}
	}
	return hasil, nil
}

// ── Matriks peran x izin ──────────────────────────────────────────────

func (r *Repository) ListPeran(ctx context.Context) ([]domain.PeranDetail, error) {
	var peran []domain.Peran
	if err := r.db.WithContext(ctx).Order("urutan").Find(&peran).Error; err != nil {
		return nil, fmt.Errorf("list peran: %w", err)
	}
	var penugasan []domain.PeranIzin
	if err := r.db.WithContext(ctx).Order("role_kunci, permission_nama").Find(&penugasan).Error; err != nil {
		return nil, fmt.Errorf("list izin peran: %w", err)
	}
	izin := map[string][]string{}
	for _, p := range penugasan {
		izin[p.RoleKunci] = append(izin[p.RoleKunci], p.PermissionNama)
	}
	out := make([]domain.PeranDetail, len(peran))
	for i, p := range peran {
		out[i] = domain.PeranDetail{Peran: p, Izin: izin[p.Kunci]}
		if out[i].Izin == nil {
			out[i].Izin = []string{}
		}
	}
	return out, nil
}

func (r *Repository) ListIzin(ctx context.Context) ([]domain.Izin, error) {
	var izin []domain.Izin
	if err := r.db.WithContext(ctx).Order("kelompok, nama").Find(&izin).Error; err != nil {
		return nil, fmt.Errorf("list izin: %w", err)
	}
	return izin, nil
}

func (r *Repository) SetIzinPeran(ctx context.Context, p domain.SetIzinPeran) (*domain.HasilSetIzinPeran, error) {
	var hasil *domain.HasilSetIzinPeran
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := kunciPemegang(tx); err != nil {
			return err
		}
		var peran domain.Peran
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("kunci = ?", p.Peran).First(&peran).Error; err != nil {
			return fmt.Errorf("baca peran: %w", err)
		}
		if peran.Versi != p.Versi {
			return domain.ErrVersiPeranBerubah
		}

		var lama []string
		if err := tx.Model(&domain.PeranIzin{}).Where("role_kunci = ?", p.Peran).Order("permission_nama").
			Pluck("permission_nama", &lama).Error; err != nil {
			return fmt.Errorf("baca izin peran: %w", err)
		}
		baru := slices.Clone(p.Izin)
		slices.Sort(baru)
		baru = slices.Compact(baru)
		tambah, cabut := selisihIzin(lama, baru)
		if len(tambah) == 0 && len(cabut) == 0 {
			hasil = &domain.HasilSetIzinPeran{Peran: peran, Sebelum: lama, Sesudah: lama, DiperbaruiPada: peran.DiperbaruiPada}
			return nil
		}

		tulis := func() error {
			if len(cabut) > 0 {
				if err := tx.Where("role_kunci = ? AND permission_nama IN ?", p.Peran, cabut).Delete(&domain.PeranIzin{}).Error; err != nil {
					return fmt.Errorf("cabut izin peran: %w", err)
				}
			}
			if len(tambah) > 0 {
				baris := make([]domain.PeranIzin, len(tambah))
				for i, nama := range tambah {
					baris[i] = domain.PeranIzin{RoleKunci: p.Peran, PermissionNama: nama}
				}
				if err := tx.Create(&baris).Error; err != nil {
					if errors.Is(err, gorm.ErrForeignKeyViolated) {
						return domain.ErrIzinTidakDikenal
					}
					return fmt.Errorf("tambah izin peran: %w", err)
				}
			}
			return tx.Exec(`UPDATE roles SET versi = versi + 1, diperbarui_pada = now(),
				diperbarui_oleh = (SELECT id FROM pengguna WHERE id = ?) WHERE kunci = ?`, p.Oleh, p.Peran).Error
		}
		if slices.Contains(cabut, domain.IzinPengelolaan) {
			if err := jagaPemegangDalam(tx, tulis); err != nil {
				return err
			}
		} else if err := tulis(); err != nil {
			return err
		}

		if err := tx.Where("kunci = ?", p.Peran).First(&peran).Error; err != nil {
			return fmt.Errorf("baca ulang peran: %w", err)
		}
		hasil = &domain.HasilSetIzinPeran{Peran: peran, Sebelum: lama, Sesudah: baru, Berubah: true, DiperbaruiPada: peran.DiperbaruiPada}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return hasil, nil
}

// jagaPemegangDalam seperti jagaPemegang untuk transaksi yang kunci penasihatnya sudah dipegang.
func jagaPemegangDalam(tx *gorm.DB, tulis func() error) error {
	sebelum, err := hitungPemegang(tx)
	if err != nil {
		return err
	}
	if err := tulis(); err != nil {
		return err
	}
	if sebelum == 0 {
		return nil
	}
	sesudah, err := hitungPemegang(tx)
	if err != nil {
		return err
	}
	if sesudah == 0 {
		return domain.ErrPemegangTerakhir
	}
	return nil
}

// selisihIzin menghitung izin yang ditambah dan dicabut dari dua himpunan terurut tanpa duplikat.
func selisihIzin(lama, baru []string) (tambah, cabut []string) {
	for _, nama := range baru {
		if _, ok := slices.BinarySearch(lama, nama); !ok {
			tambah = append(tambah, nama)
		}
	}
	for _, nama := range lama {
		if _, ok := slices.BinarySearch(baru, nama); !ok {
			cabut = append(cabut, nama)
		}
	}
	return tambah, cabut
}

// ── Izin langsung ─────────────────────────────────────────────────────

func (r *Repository) ListIzinLangsung(ctx context.Context, userID string) (*domain.IzinPengguna, error) {
	var peran []string
	if err := r.db.WithContext(ctx).Raw("SELECT peran::text FROM pengguna WHERE id = ?", userID).Scan(&peran).Error; err != nil {
		return nil, fmt.Errorf("baca peran pengguna: %w", err)
	}
	if len(peran) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	hasil := &domain.IzinPengguna{Peran: peran[0], IzinPeran: []string{}, Langsung: []domain.IzinLangsungDetail{}}
	if err := r.db.WithContext(ctx).Model(&domain.PeranIzin{}).Where("role_kunci = ?", peran[0]).Order("permission_nama").
		Pluck("permission_nama", &hasil.IzinPeran).Error; err != nil {
		return nil, fmt.Errorf("baca izin peran: %w", err)
	}
	if err := r.db.WithContext(ctx).Raw(sqlDetailLangsung+`
		 WHERE up.pengguna_id = ? ORDER BY up.diberikan_pada DESC, up.permission_nama`, userID).Scan(&hasil.Langsung).Error; err != nil {
		return nil, fmt.Errorf("baca izin langsung: %w", err)
	}
	return hasil, nil
}

const sqlDetailLangsung = `SELECT up.pengguna_id, up.permission_nama, up.berlaku_sampai, up.alasan, up.diberikan_oleh,
		up.diberikan_pada, p.deskripsi, p.kelompok, g.nama AS diberikan_oleh_nama
	  FROM user_permissions up
	  JOIN permissions p ON p.nama = up.permission_nama
	  LEFT JOIN pengguna g ON g.id = up.diberikan_oleh`

func (r *Repository) BerikanIzinLangsung(ctx context.Context, g domain.BerikanIzin) (*domain.IzinLangsungDetail, *domain.IzinLangsung, error) {
	var sesudah *domain.IzinLangsungDetail
	var sebelumnya *domain.IzinLangsung
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Kunci baris pengguna (bersama) supaya peran tidak berganti di tengah pemeriksaan "sudah punya lewat peran".
		var peran []string
		if err := tx.Raw("SELECT peran::text FROM pengguna WHERE id = ? FOR SHARE", g.PenggunaID).Scan(&peran).Error; err != nil {
			return fmt.Errorf("baca pengguna: %w", err)
		}
		if len(peran) == 0 {
			return gorm.ErrRecordNotFound
		}

		var ada int64
		if err := tx.Model(&domain.Izin{}).Where("nama = ?", g.Izin).Count(&ada).Error; err != nil {
			return fmt.Errorf("baca katalog izin: %w", err)
		}
		if ada == 0 {
			return domain.ErrIzinTidakDikenal
		}
		if err := tx.Model(&domain.PeranIzin{}).Where("role_kunci = ? AND permission_nama = ?", peran[0], g.Izin).Count(&ada).Error; err != nil {
			return fmt.Errorf("baca izin peran: %w", err)
		}
		if ada > 0 {
			return domain.ErrIzinDariPeran
		}

		var lama []domain.IzinLangsung
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("pengguna_id = ? AND permission_nama = ?", g.PenggunaID, g.Izin).Find(&lama).Error; err != nil {
			return fmt.Errorf("baca izin langsung: %w", err)
		}
		if len(lama) > 0 {
			sebelumnya = &lama[0]
		}

		// Baris kedaluwarsa diganti; baris yang masih berlaku membuat DO UPDATE tidak menyentuh apa pun.
		res := tx.Exec(`INSERT INTO user_permissions (pengguna_id, permission_nama, berlaku_sampai, alasan, diberikan_oleh, diberikan_pada)
			VALUES (?, ?, ?, ?, (SELECT id FROM pengguna WHERE id = ?), now())
			ON CONFLICT (pengguna_id, permission_nama) DO UPDATE
			   SET berlaku_sampai = EXCLUDED.berlaku_sampai, alasan = EXCLUDED.alasan,
			       diberikan_oleh = EXCLUDED.diberikan_oleh, diberikan_pada = EXCLUDED.diberikan_pada
			 WHERE user_permissions.berlaku_sampai IS NOT NULL AND user_permissions.berlaku_sampai <= now()`,
			g.PenggunaID, g.Izin, g.BerlakuSampai, g.Alasan, g.Oleh)
		if res.Error != nil {
			if errors.Is(res.Error, gorm.ErrForeignKeyViolated) {
				return domain.ErrIzinTidakDikenal
			}
			return fmt.Errorf("simpan izin langsung: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return domain.ErrIzinLangsungAda
		}

		var baris []domain.IzinLangsungDetail
		if err := tx.Raw(sqlDetailLangsung+" WHERE up.pengguna_id = ? AND up.permission_nama = ?", g.PenggunaID, g.Izin).
			Scan(&baris).Error; err != nil {
			return fmt.Errorf("baca ulang izin langsung: %w", err)
		}
		if len(baris) == 0 {
			return fmt.Errorf("izin langsung tidak terbaca setelah disimpan")
		}
		sesudah = &baris[0]
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return sesudah, sebelumnya, nil
}

func (r *Repository) CabutIzinLangsung(ctx context.Context, userID, izin string) (*domain.IzinLangsung, error) {
	var baris []domain.IzinLangsung
	err := r.db.WithContext(ctx).Raw("DELETE FROM user_permissions WHERE pengguna_id = ? AND permission_nama = ? RETURNING *", userID, izin).
		Scan(&baris).Error
	if err != nil {
		return nil, fmt.Errorf("cabut izin langsung: %w", err)
	}
	if len(baris) == 0 {
		return nil, domain.ErrIzinLangsungTidakAda
	}
	return &baris[0], nil
}

func (r *Repository) HitungPemegangPengelolaan(ctx context.Context) (int64, error) {
	return hitungPemegang(r.db.WithContext(ctx))
}
