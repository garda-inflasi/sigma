package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/audit/domain"
	"gorm.io/gorm"
)

// batchPindai adalah jumlah baris per pembacaan saat memindai rantai atau mengekspor partisi.
const batchPindai = 2000

var _ domain.RepositoryIntegritas = (*Repository)(nil)

// PindaiRantai memakai satu snapshot untuk seluruh pemindaian: baris dan segmen arsip konsisten satu sama lain, dan
// kunci baca yang dipegang sampai selesai mencegah partisi dilepas di tengah pemindaian.
func (r *Repository) PindaiRantai(ctx context.Context, awal func([]domain.SegmenArsip) error, periksa func(domain.AuditLog) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var segmen []domain.SegmenArsip
		if err := tx.Raw(`
			SELECT arsip_id, id_awal, id_akhir, jumlah_baris, hash_masuk, hash_keluar
			  FROM audit_log_arsip_segmen ORDER BY id_awal, id`).Scan(&segmen).Error; err != nil {
			return fmt.Errorf("baca segmen arsip: %w", err)
		}
		if err := awal(segmen); err != nil {
			return err
		}
		var setelah int64
		for {
			var batch []domain.AuditLog
			if err := tx.Where("id > ?", setelah).Order("id").Limit(batchPindai).Find(&batch).Error; err != nil {
				return fmt.Errorf("baca rantai audit: %w", err)
			}
			for _, row := range batch {
				if err := periksa(row); err != nil {
					return err
				}
			}
			if len(batch) < batchPindai {
				return nil
			}
			setelah = int64(batch[len(batch)-1].ID)
		}
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
}

func (r *Repository) SimpanVerifikasi(ctx context.Context, v *domain.Verifikasi) error {
	if err := r.db.WithContext(ctx).Create(v).Error; err != nil {
		return fmt.Errorf("simpan verifikasi audit: %w", err)
	}
	return nil
}

func (r *Repository) VerifikasiTerakhir(ctx context.Context) (*domain.Verifikasi, error) {
	var rows []domain.Verifikasi
	if err := r.db.WithContext(ctx).Order("mulai DESC, id DESC").Limit(1).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("verifikasi audit terakhir: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

func (r *Repository) DaftarPartisi(ctx context.Context) ([]domain.Partisi, error) {
	var rows []struct {
		Nama      string  `gorm:"column:nama"`
		Perkiraan float64 `gorm:"column:perkiraan"`
	}
	err := r.db.WithContext(ctx).Raw(`
		SELECT c.relname AS nama, c.reltuples AS perkiraan
		  FROM pg_inherits i JOIN pg_class c ON c.oid = i.inhrelid
		 WHERE i.inhparent = 'public.audit_log'::regclass
		 ORDER BY c.relname DESC`).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("daftar partisi audit: %w", err)
	}
	out := make([]domain.Partisi, len(rows))
	for i, row := range rows {
		p := domain.Partisi{Nama: row.Nama}
		if dari, sampai, ok := domain.RentangPartisi(row.Nama); ok {
			p.Dari, p.Sampai = &dari, &sampai
		}
		// reltuples -1: belum pernah dianalisis (perkiraan tidak ada).
		if row.Perkiraan >= 0 {
			p.PerkiraanBaris = new(int64(row.Perkiraan))
		}
		out[i] = p
	}
	return out, nil
}

func (r *Repository) DaftarArsip(ctx context.Context) ([]domain.Arsip, error) {
	var rows []domain.Arsip
	if err := r.db.WithContext(ctx).Order("dari DESC, id DESC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("daftar arsip audit: %w", err)
	}
	return rows, nil
}

func (r *Repository) SegmenArsip(ctx context.Context, arsipID int64) ([]domain.SegmenArsip, error) {
	query := r.db.WithContext(ctx).Table("audit_log_arsip_segmen").
		Select("arsip_id, id_awal, id_akhir, jumlah_baris, hash_masuk, hash_keluar").Order("id_awal, id")
	if arsipID > 0 {
		query = query.Where("arsip_id = ?", arsipID)
	}
	var out []domain.SegmenArsip
	if err := query.Scan(&out).Error; err != nil {
		return nil, fmt.Errorf("segmen arsip audit: %w", err)
	}
	return out, nil
}

func (r *Repository) SiapkanPartisi(ctx context.Context, dari time.Time, jumlahBulan int) ([]string, error) {
	var nama []string
	err := r.db.WithContext(ctx).Raw(`SELECT nama FROM audit_log_siapkan_partisi(?, ?) AS nama`, dari, jumlahBulan).Scan(&nama).Error
	if err != nil {
		return nil, fmt.Errorf("siapkan partisi audit: %w", err)
	}
	return nama, nil
}

func (r *Repository) BarisRentang(ctx context.Context, dari, sampai time.Time, fn func(domain.AuditLog) error) error {
	var setelah int64
	for {
		var batch []domain.AuditLog
		err := r.db.WithContext(ctx).
			Where("waktu >= ? AND waktu < ? AND id > ?", dari, sampai, setelah).
			Order("id").Limit(batchPindai).Find(&batch).Error
		if err != nil {
			return fmt.Errorf("baca partisi audit: %w", err)
		}
		for _, row := range batch {
			if err := fn(row); err != nil {
				return err
			}
		}
		if len(batch) < batchPindai {
			return nil
		}
		setelah = int64(batch[len(batch)-1].ID)
	}
}

func (r *Repository) LepasPartisi(ctx context.Context, in domain.LepasPartisi) (int64, error) {
	var id int64
	err := r.db.WithContext(ctx).Raw(`SELECT audit_log_lepas_partisi(?, ?, ?, ?, ?, ?)`,
		in.Partisi, in.RetensiBulan, in.JumlahBaris, in.Ringkasan, in.Berkas, in.SHA256Berkas).Scan(&id).Error
	if err != nil {
		return 0, fmt.Errorf("lepas partisi %s: %w", in.Partisi, err)
	}
	return id, nil
}

func (r *Repository) DenganKunciPemeliharaan(ctx context.Context, fn func(context.Context) error) error {
	return r.db.WithContext(ctx).Connection(func(conn *gorm.DB) error {
		const kunci = "hashtextextended('garda.audit_log.pemeliharaan', 0)"
		if err := conn.Exec("SELECT pg_advisory_lock(" + kunci + ")").Error; err != nil {
			return fmt.Errorf("kunci pemeliharaan audit: %w", err)
		}
		// Dilepas walau ctx sudah selesai; bila koneksinya putus, sesi berakhir dan kunci ikut lepas.
		defer conn.WithContext(context.WithoutCancel(ctx)).Exec("SELECT pg_advisory_unlock(" + kunci + ")")
		return fn(ctx)
	})
}
