package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/pkg/idx"
	"gorm.io/gorm"
)

// Implementasi domain.ImpersonasiRepository (migrasi 000129, docs/impersonasi.md).
var _ domain.ImpersonasiRepository = (*Repository)(nil)

// ── Kebijakan ─────────────────────────────────────────────────────────

func (r *Repository) GetKebijakan(ctx context.Context) (*domain.Pengaturan, error) {
	var rows []struct {
		Aktif              bool       `gorm:"column:aktif"`
		WajibAlasan        bool       `gorm:"column:wajib_alasan"`
		DurasiMaksMenit    *int       `gorm:"column:durasi_maks_menit"`
		Versi              int64      `gorm:"column:versi"`
		DiperbaruiPada     *time.Time `gorm:"column:diperbarui_pada"`
		DiperbaruiOleh     *string    `gorm:"column:diperbarui_oleh"`
		DiperbaruiOlehNama *string    `gorm:"column:diperbarui_oleh_nama"`
	}
	if err := r.db.WithContext(ctx).Raw(`
		SELECT k.aktif, k.wajib_alasan, k.durasi_maks_menit, k.versi, k.diperbarui_pada,
		       k.diperbarui_oleh::text AS diperbarui_oleh, p.nama AS diperbarui_oleh_nama
		  FROM impersonasi_kebijakan k
		  LEFT JOIN pengguna p ON p.id = k.diperbarui_oleh
		 WHERE k.id = 1`).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("baca kebijakan impersonasi: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("kebijakan impersonasi: %w", gorm.ErrRecordNotFound)
	}
	b := rows[0]
	return &domain.Pengaturan{
		Aktif: b.Aktif, WajibAlasan: b.WajibAlasan, DurasiMaksMenit: b.DurasiMaksMenit, Versi: b.Versi,
		DiperbaruiPada: b.DiperbaruiPada, DiperbaruiOleh: b.DiperbaruiOleh, DiperbaruiOlehNama: b.DiperbaruiOlehNama,
	}, nil
}

// baris pelaku atau target hasil gabungan dengan pengguna.
type barisPihak struct {
	PelakuID      string  `gorm:"column:pelaku_id"`
	ID            string  `gorm:"column:id"`
	Tipe          string  `gorm:"column:tipe"`
	Peran         *string `gorm:"column:peran"`
	PenggunaID    *string `gorm:"column:pengguna_id"`
	Aktif         bool    `gorm:"column:aktif"`
	PenggunaNama  *string `gorm:"column:pengguna_nama"`
	PenggunaEmail *string `gorm:"column:pengguna_email"`
	PenggunaPeran *string `gorm:"column:pengguna_peran"`
}

func (b barisPihak) pihak() domain.PihakKebijakan {
	return domain.PihakKebijakan{
		Tipe: b.Tipe, Peran: deref(b.Peran), PenggunaID: deref(b.PenggunaID),
		PenggunaNama: deref(b.PenggunaNama), PenggunaEmail: deref(b.PenggunaEmail), PenggunaPeran: deref(b.PenggunaPeran),
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (r *Repository) ListPelaku(ctx context.Context) ([]domain.PelakuKebijakan, error) {
	return r.muatPelaku(ctx, "TRUE", nil)
}

func (r *Repository) AturanPelaku(ctx context.Context, penggunaID, peran string) ([]domain.PelakuKebijakan, error) {
	if !idx.ValidUUID(penggunaID) {
		return nil, nil
	}
	return r.muatPelaku(ctx,
		"p.aktif AND ((p.tipe = 'PENGGUNA' AND p.pengguna_id = ?) OR (p.tipe = 'PERAN' AND p.peran = ?))",
		[]any{penggunaID, peran})
}

// muatPelaku membaca pelaku yang memenuhi syarat beserta sasarannya (dua pernyataan, tanpa N+1). Syarat dan argumennya
// berasal dari konstanta di atas, bukan masukan klien.
func (r *Repository) muatPelaku(ctx context.Context, syarat string, args []any) ([]domain.PelakuKebijakan, error) {
	var pelaku []barisPihak
	if err := r.db.WithContext(ctx).Raw(`
		SELECT p.id::text AS id, p.id::text AS pelaku_id, p.tipe, p.peran::text AS peran, p.pengguna_id::text AS pengguna_id, p.aktif,
		       u.nama AS pengguna_nama, u.email AS pengguna_email, u.peran::text AS pengguna_peran
		  FROM impersonasi_pelaku p
		  LEFT JOIN pengguna u ON u.id = p.pengguna_id
		 WHERE `+syarat+`
		 ORDER BY p.dibuat_pada, p.id`, args...).Scan(&pelaku).Error; err != nil {
		return nil, fmt.Errorf("baca pelaku impersonasi: %w", err)
	}
	out := make([]domain.PelakuKebijakan, len(pelaku))
	ids := make([]string, len(pelaku))
	posisi := make(map[string]int, len(pelaku))
	for i, p := range pelaku {
		out[i] = domain.PelakuKebijakan{ID: p.ID, PihakKebijakan: p.pihak(), Aktif: p.Aktif, Target: []domain.PihakKebijakan{}}
		ids[i] = p.ID
		posisi[p.ID] = i
	}
	if len(ids) == 0 {
		return out, nil
	}

	var target []barisPihak
	if err := r.db.WithContext(ctx).Raw(`
		SELECT t.id::text AS id, t.pelaku_id::text AS pelaku_id, t.tipe, t.peran::text AS peran, t.pengguna_id::text AS pengguna_id,
		       TRUE AS aktif, u.nama AS pengguna_nama, u.email AS pengguna_email, u.peran::text AS pengguna_peran
		  FROM impersonasi_target t
		  LEFT JOIN pengguna u ON u.id = t.pengguna_id
		 WHERE t.pelaku_id IN ?
		 ORDER BY t.tipe DESC, t.peran, lower(u.nama), t.id`, ids).Scan(&target).Error; err != nil {
		return nil, fmt.Errorf("baca sasaran impersonasi: %w", err)
	}
	for _, t := range target {
		i := posisi[t.PelakuID]
		out[i].Target = append(out[i].Target, t.pihak())
	}
	return out, nil
}

func (r *Repository) SimpanKebijakan(ctx context.Context, in domain.SimpanKebijakan) (*domain.Pengaturan, error) {
	now := time.Now()
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var saatIni []struct{ Versi int64 }
		if err := tx.Raw("SELECT versi FROM impersonasi_kebijakan WHERE id = 1 FOR UPDATE").Scan(&saatIni).Error; err != nil {
			return fmt.Errorf("kunci kebijakan: %w", err)
		}
		if len(saatIni) == 0 {
			if in.Versi != 0 {
				return domain.ErrVersiKebijakanBerubah
			}
			res := tx.Exec(`
				INSERT INTO impersonasi_kebijakan (id, aktif, wajib_alasan, durasi_maks_menit, versi, diperbarui_pada, diperbarui_oleh)
				VALUES (1, ?, ?, ?, 1, ?, ?) ON CONFLICT (id) DO NOTHING`,
				in.Aktif, in.WajibAlasan, in.DurasiMaksMenit, now, in.Oleh)
			if res.Error != nil {
				return fmt.Errorf("simpan kebijakan: %w", res.Error)
			}
			if res.RowsAffected == 0 { // admin lain menyimpan pertama kali di sela-sela
				return domain.ErrVersiKebijakanBerubah
			}
		} else {
			if saatIni[0].Versi != in.Versi {
				return domain.ErrVersiKebijakanBerubah
			}
			if err := tx.Exec(`
				UPDATE impersonasi_kebijakan
				   SET aktif = ?, wajib_alasan = ?, durasi_maks_menit = ?, versi = versi + 1, diperbarui_pada = ?, diperbarui_oleh = ?
				 WHERE id = 1`, in.Aktif, in.WajibAlasan, in.DurasiMaksMenit, now, in.Oleh).Error; err != nil {
				return fmt.Errorf("ubah kebijakan: %w", err)
			}
		}

		// Daftar pelaku diganti seluruhnya; sasaran ikut terhapus lewat ON DELETE CASCADE.
		if err := tx.Exec("DELETE FROM impersonasi_pelaku").Error; err != nil {
			return fmt.Errorf("hapus pelaku lama: %w", err)
		}
		for _, p := range in.Pelaku {
			row := domain.ImpersonasiPelaku{Tipe: p.Tipe, Aktif: p.Aktif, DibuatPada: now}
			isiPihak(p.PihakKebijakan, &row.Peran, &row.PenggunaID)
			if err := tx.Create(&row).Error; err != nil {
				return fmt.Errorf("simpan pelaku: %w", err)
			}
			for _, t := range p.Target {
				target := domain.ImpersonasiTarget{PelakuID: row.ID, Tipe: t.Tipe}
				isiPihak(t, &target.Peran, &target.PenggunaID)
				if err := tx.Create(&target).Error; err != nil {
					return fmt.Errorf("simpan sasaran: %w", err)
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.GetKebijakan(ctx)
}

// isiPihak mengisi kolom peran atau pengguna_id sesuai jenis pihak (yang lain nil, sesuai CHECK tabel).
func isiPihak(p domain.PihakKebijakan, peran, penggunaID **string) {
	switch p.Tipe {
	case domain.TipePeran:
		*peran = &p.Peran
	case domain.TipePengguna:
		*penggunaID = &p.PenggunaID
	}
}

func (r *Repository) RingkasPengguna(ctx context.Context, ids []string) ([]domain.PenggunaRingkas, error) {
	valid := make([]string, 0, len(ids))
	for _, id := range ids {
		if idx.ValidUUID(id) {
			valid = append(valid, id)
		}
	}
	if len(valid) == 0 {
		return nil, nil
	}
	var rows []domain.PenggunaRingkas
	if err := r.db.WithContext(ctx).Raw(
		"SELECT id::text AS id, nama, email, peran::text AS peran FROM pengguna WHERE id IN ?", valid).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("ringkas pengguna: %w", err)
	}
	return rows, nil
}

func (r *Repository) ListAkunSasaran(ctx context.Context, f domain.FilterAkunSasaran) ([]domain.AkunSasaran, error) {
	q := r.db.WithContext(ctx).Table("pengguna").
		Select("id::text AS id, nama, email, peran::text AS peran, lembaga, kabupaten_kode").
		Where("status = 'AKTIF' AND peran <> ?", domain.PeranSuperadmin)
	if f.KecualiID != "" && idx.ValidUUID(f.KecualiID) {
		q = q.Where("id <> ?", f.KecualiID)
	}
	if f.HanyaPeran != "" {
		q = q.Where("peran = ?", f.HanyaPeran)
	}
	if !f.Semua {
		peran, ids := f.Peran, make([]string, 0, len(f.PenggunaID))
		for _, id := range f.PenggunaID {
			if idx.ValidUUID(id) {
				ids = append(ids, id)
			}
		}
		switch {
		case len(peran) > 0 && len(ids) > 0:
			q = q.Where("(peran IN ? OR id IN ?)", peran, ids)
		case len(peran) > 0:
			q = q.Where("peran IN ?", peran)
		case len(ids) > 0:
			q = q.Where("id IN ?", ids)
		default:
			return []domain.AkunSasaran{}, nil
		}
	}
	batas := f.BatasJumlah
	if batas <= 0 {
		batas = 1000
	}
	var rows []domain.AkunSasaran
	if err := q.Order("peran, lower(nama), id").Limit(batas).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("daftar akun sasaran: %w", err)
	}
	return rows, nil
}

// ── Sesi ──────────────────────────────────────────────────────────────

func (r *Repository) MulaiImpersonasi(ctx context.Context, in domain.MulaiImpersonasi) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		switch {
		case in.Ganti != nil:
			if err := tx.Exec(
				"UPDATE impersonasi_sesi SET berakhir_pada = ?, alasan_berakhir = ? WHERE id = ? AND berakhir_pada IS NULL",
				in.Waktu, domain.AkhirBerganti, in.Ganti.ID).Error; err != nil {
				return fmt.Errorf("akhiri impersonasi lama: %w", err)
			}
			if err := tx.Exec("UPDATE refresh_tokens SET revoked_at = ? WHERE family_id = ? AND revoked_at IS NULL",
				in.Waktu, in.Ganti.FamilyID).Error; err != nil {
				return fmt.Errorf("cabut sesi impersonasi lama: %w", err)
			}
		case in.CabutSesiAsal != "":
			if err := tx.Exec(
				"UPDATE refresh_tokens SET revoked_at = ? WHERE user_id = ? AND family_id = ? AND revoked_at IS NULL",
				in.Waktu, in.PelakuID, in.CabutSesiAsal).Error; err != nil {
				return fmt.Errorf("cabut sesi asal: %w", err)
			}
		}
		if err := tx.Create(in.Sesi).Error; err != nil {
			return fmt.Errorf("simpan impersonasi: %w", err)
		}
		in.Token.ImpersonasiID = &in.Sesi.ID
		if err := tx.Create(in.Token).Error; err != nil {
			return fmt.Errorf("simpan token impersonasi: %w", err)
		}
		return nil
	})
}

func (r *Repository) GetImpersonasi(ctx context.Context, id string) (*domain.ImpersonasiSesi, error) {
	if !idx.ValidUUID(id) {
		return nil, fmt.Errorf("impersonasi: %w", gorm.ErrRecordNotFound)
	}
	var rec domain.ImpersonasiSesi
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&rec).Error; err != nil {
		return nil, fmt.Errorf("baca impersonasi: %w", err)
	}
	return &rec, nil
}

func (r *Repository) AkhiriImpersonasi(ctx context.Context, in domain.AkhiriImpersonasi) (*domain.ImpersonasiSesi, error) {
	var rec domain.ImpersonasiSesi
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Raw(
			"UPDATE impersonasi_sesi SET berakhir_pada = ?, alasan_berakhir = ? WHERE id = ? AND berakhir_pada IS NULL RETURNING *",
			in.Waktu, in.Alasan, in.ID).Scan(&rec)
		if res.Error != nil {
			return fmt.Errorf("akhiri impersonasi: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return domain.ErrImpersonasiTidakAktif
		}
		if err := tx.Exec("UPDATE refresh_tokens SET revoked_at = ? WHERE family_id = ? AND revoked_at IS NULL",
			in.Waktu, rec.FamilyID).Error; err != nil {
			return fmt.Errorf("cabut sesi impersonasi: %w", err)
		}
		if in.Pengganti != nil {
			if err := tx.Create(in.Pengganti).Error; err != nil {
				return fmt.Errorf("simpan sesi pengganti: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, domain.ErrImpersonasiTidakAktif) {
			return nil, domain.ErrImpersonasiTidakAktif
		}
		return nil, err
	}
	return &rec, nil
}

// riwayatRow adalah satu baris riwayat beserta keadaan sesinya (family refresh token).
type riwayatRow struct {
	domain.ImpersonasiSesi `gorm:"embedded"`
	Hidup                  bool       `gorm:"column:hidup"`
	DicabutPada            *time.Time `gorm:"column:dicabut_pada"`
	HabisPada              *time.Time `gorm:"column:habis_pada"`
}

func (r *Repository) ListRiwayatImpersonasi(ctx context.Context, f domain.FilterRiwayatImpersonasi, now time.Time) ([]domain.RiwayatImpersonasi, error) {
	// hidup: catatan belum ditutup, belum kedaluwarsa, dan family-nya masih punya refresh token yang hidup. Catatan yang
	// sesinya dicabut di luar alurnya (paksa keluar, ganti kata sandi sasaran) tampil berakhir DICABUT.
	query := `SELECT * FROM (
		SELECT s.*,
		       (s.berakhir_pada IS NULL
		        AND (s.kedaluwarsa_pada IS NULL OR s.kedaluwarsa_pada > @now)
		        AND EXISTS (SELECT 1 FROM refresh_tokens r WHERE r.family_id = s.family_id AND r.revoked_at IS NULL AND r.expires_at > @now)) AS hidup,
		       (SELECT MAX(r.revoked_at) FROM refresh_tokens r WHERE r.family_id = s.family_id AND r.rotated_at IS NULL) AS dicabut_pada,
		       (SELECT MAX(r.expires_at) FROM refresh_tokens r WHERE r.family_id = s.family_id) AS habis_pada
		  FROM impersonasi_sesi s`
	args := map[string]any{"now": now, "limit": f.Limit}
	if f.After != nil {
		query += " WHERE (s.mulai_pada, s.id) < (@after_mulai, CAST(@after_id AS uuid))"
		args["after_mulai"], args["after_id"] = f.After.MulaiPada, f.After.ID
	}
	query += ") x"
	if f.HanyaAktif {
		query += " WHERE x.hidup"
	}
	query += " ORDER BY x.mulai_pada DESC, x.id DESC LIMIT @limit"

	var rows []riwayatRow
	if err := r.db.WithContext(ctx).Raw(query, args).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("riwayat impersonasi: %w", err)
	}
	out := make([]domain.RiwayatImpersonasi, len(rows))
	for i, row := range rows {
		item := domain.RiwayatImpersonasi{ImpersonasiSesi: row.ImpersonasiSesi, Aktif: row.Hidup}
		switch {
		case row.BerakhirPada != nil:
			item.BerakhirEfektif, item.AlasanAkhirEfektif = row.BerakhirPada, row.AlasanBerakhir
		case row.Hidup:
		case row.KedaluwarsaPada != nil && !row.KedaluwarsaPada.After(now):
			item.BerakhirEfektif, item.AlasanAkhirEfektif = row.KedaluwarsaPada, new(domain.AkhirKedaluwarsa)
		case row.DicabutPada != nil:
			// Dicabut di luar alurnya (paksa keluar, ganti kata sandi, keluar dari semua perangkat); token yang hanya
			// dirotasi tidak dihitung (rotated_at terisi).
			item.BerakhirEfektif, item.AlasanAkhirEfektif = row.DicabutPada, new(domain.AkhirDicabut)
		default:
			// Tidak ada pencabutan: masa berlaku refresh token habis (atau barisnya sudah dibersihkan job).
			berakhir := row.HabisPada
			if berakhir == nil {
				berakhir = &row.MulaiPada
			}
			item.BerakhirEfektif, item.AlasanAkhirEfektif = berakhir, new(domain.AkhirKedaluwarsa)
		}
		out[i] = item
	}
	return out, nil
}
