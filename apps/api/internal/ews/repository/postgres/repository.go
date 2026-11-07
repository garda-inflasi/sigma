// Package postgres adalah repository EWS di atas PostgreSQL.
package postgres

import (
	"context"
	"fmt"

	"github.com/garda-inflasi/api/internal/ews/domain"
	"github.com/garda-inflasi/api/pkg/idx"
	"gorm.io/gorm"
)

// bulkSQL menerapkan satu aksi massal dan mencatatnya di ews_riwayat dalam satu
// pernyataan; WHERE membatasi transisi yang sah (BARU -> DIAKUI, BARU/DIAKUI ->
// DIESKALASI, selain SELESAI -> SELESAI). Nilai yang disisipkan adalah konstanta.
var bulkSQL = map[domain.BulkAction]string{
	domain.BulkAkui: bulkQuery(
		"status = 'BARU'", "DIAKUI", "diakui_oleh_id = @user, diakui_pada = NOW()"),
	domain.BulkEskalasi: bulkQuery(
		"status IN ('BARU', 'DIAKUI')", "DIESKALASI", "dieskalasi_oleh_id = @user, dieskalasi_pada = NOW()"),
	domain.BulkSelesai: bulkQuery(
		"status <> 'SELESAI'", "SELESAI", "selesai_pada = NOW()"),
}

func bulkQuery(kondisi, ke, setTambahan string) string {
	return fmt.Sprintf(`WITH prev AS (
			SELECT id, status FROM ews_alert WHERE id IN @ids AND %s FOR UPDATE
		), upd AS (
			UPDATE ews_alert a SET status = '%s', %s
			  FROM prev WHERE a.id = prev.id
			RETURNING a.id, prev.status AS dari
		), riwayat AS (
			INSERT INTO ews_riwayat (alert_id, dari, ke, oleh)
			SELECT id, dari, '%s', @user FROM upd
		)
		SELECT id FROM upd`, kondisi, ke, setTambahan, ke)
}

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// kolomAlert dan gabungAlert melengkapi alert dengan nama komoditas, nama kabupaten, dan kode aturan pemicunya.
const kolomAlert = "a.*, k.nama AS komoditas_nama, w.nama AS kabupaten_nama, t.kode AS aturan_kode"

func gabungAlert(q *gorm.DB) *gorm.DB {
	return q.
		Joins("LEFT JOIN komoditas k ON k.kode = a.komoditas_kode").
		Joins("JOIN kabupaten w ON w.kode = a.kabupaten_kode").
		Joins("LEFT JOIN aturan_ews t ON t.id = a.aturan_id")
}

// alertQuery menggabungkan nama komoditas dan kabupaten serta kode aturan.
func (r *Repository) alertQuery(ctx context.Context) *gorm.DB {
	return gabungAlert(r.db.WithContext(ctx).Table("ews_alert AS a")).Select(kolomAlert)
}

func (r *Repository) List(ctx context.Context, q domain.ListQuery) ([]domain.Alert, int64, error) {
	filtered := r.db.WithContext(ctx).Table("ews_alert AS a")
	if q.Status != "" {
		filtered = filtered.Where("a.status = ?", q.Status)
	}
	if q.Severity != "" {
		filtered = filtered.Where("a.severity = ?", q.Severity)
	}
	if q.Jenis != "" {
		filtered = filtered.Where("a.jenis = ?", q.Jenis)
	}
	if q.KabupatenKode != "" {
		filtered = filtered.Where("a.kabupaten_kode = ?", q.KabupatenKode)
	}

	var total int64
	if err := filtered.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("hitung alert ews: %w", err)
	}

	var items []domain.Alert
	err := gabungAlert(filtered.Session(&gorm.Session{})).
		Select(kolomAlert).
		Order("a.dibuat_pada DESC, a.id").
		Limit(q.Limit).Offset((q.Page - 1) * q.Limit).
		Scan(&items).Error
	if err != nil {
		return nil, 0, fmt.Errorf("daftar alert ews: %w", err)
	}
	return items, total, nil
}

func (r *Repository) GetByID(ctx context.Context, ref idx.RujukanID) (*domain.Alert, error) {
	var item domain.Alert
	res := r.alertQuery(ctx).Where(ref.Kondisi("a.id", "a.kode")).Limit(1).Scan(&item)
	if res.Error != nil {
		return nil, fmt.Errorf("ambil alert ews: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &item, nil
}

func (r *Repository) ListRiwayat(ctx context.Context, alertID idx.ID) ([]domain.Riwayat, error) {
	var items []domain.Riwayat
	err := r.db.WithContext(ctx).Table("ews_riwayat AS r").
		Select("r.*, u.nama AS oleh_nama").
		Joins("LEFT JOIN pengguna u ON u.id = r.oleh").
		Where("r.alert_id = ?", alertID).
		Order("r.waktu, r.id").
		Scan(&items).Error
	if err != nil {
		return nil, fmt.Errorf("riwayat alert ews: %w", err)
	}
	return items, nil
}

// aturanQuery menggabungkan nama komoditas ke aturan.
func aturanQuery(db *gorm.DB) *gorm.DB {
	return db.Table("aturan_ews AS t").
		Select("t.*, k.nama AS komoditas_nama").
		Joins("LEFT JOIN komoditas k ON k.kode = t.komoditas_kode")
}

func (r *Repository) ListAturan(ctx context.Context) ([]domain.Aturan, error) {
	var items []domain.Aturan
	err := aturanQuery(r.db.WithContext(ctx)).Order("t.jenis, t.komoditas_kode, t.kabupaten_kode, t.id").Scan(&items).Error
	if err != nil {
		return nil, fmt.Errorf("daftar aturan ews: %w", err)
	}
	return items, nil
}

func (r *Repository) GetAturan(ctx context.Context, ref idx.RujukanID) (*domain.Aturan, error) {
	return getAturan(aturanQuery(r.db.WithContext(ctx)), ref)
}

func getAturan(q *gorm.DB, ref idx.RujukanID) (*domain.Aturan, error) {
	var item domain.Aturan
	res := q.Where(ref.Kondisi("t.id", "t.kode")).Limit(1).Scan(&item)
	if res.Error != nil {
		return nil, fmt.Errorf("ambil aturan ews: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, domain.ErrAturanNotFound
	}
	return &item, nil
}

func (r *Repository) UpdateAturan(ctx context.Context, id idx.ID, upd domain.AturanUpdate, userID string) (*domain.Aturan, error) {
	var updated *domain.Aturan
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Exec(`
			UPDATE aturan_ews
			   SET ambang_waspada = @waspada, ambang_siaga = @siaga, ambang_kritis = @kritis,
			       jendela_hari = @jendela, cooldown_jam = @cooldown, kanal = CAST(@kanal AS text[]),
			       aktif = @aktif, keterangan = @keterangan,
			       versi = versi + 1, diubah_oleh_id = @user, diubah_pada = NOW()
			 WHERE id = @id AND versi = @versi`, map[string]any{
			"waspada": upd.AmbangWaspada, "siaga": upd.AmbangSiaga, "kritis": upd.AmbangKritis,
			"jendela": upd.JendelaHari, "cooldown": upd.CooldownJam, "kanal": upd.Kanal,
			"aktif": upd.Aktif, "keterangan": upd.Keterangan, "user": userID, "id": id, "versi": upd.Versi,
		})
		if res.Error != nil {
			return fmt.Errorf("ubah aturan ews: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			var ada bool
			if err := tx.Raw("SELECT EXISTS (SELECT 1 FROM aturan_ews WHERE id = ?)", id).Scan(&ada).Error; err != nil {
				return fmt.Errorf("periksa aturan ews: %w", err)
			}
			if !ada {
				return domain.ErrAturanNotFound
			}
			return domain.ErrVersiUsang
		}
		err := tx.Exec(`
			INSERT INTO aturan_ews_riwayat (aturan_id, versi, ambang_waspada, ambang_siaga, ambang_kritis,
			                                jendela_hari, cooldown_jam, kanal, aktif, keterangan, diubah_oleh_id)
			SELECT id, versi, ambang_waspada, ambang_siaga, ambang_kritis,
			       jendela_hari, cooldown_jam, kanal, aktif, keterangan, diubah_oleh_id
			  FROM aturan_ews WHERE id = ?`, id).Error
		if err != nil {
			return fmt.Errorf("riwayat aturan ews: %w", err)
		}
		updated, err = getAturan(aturanQuery(tx), idx.RujukanID{ID: id})
		return err
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (r *Repository) BulkAction(ctx context.Context, ids []idx.ID, action domain.BulkAction, userID string) ([]idx.ID, error) {
	query, ok := bulkSQL[action]
	if !ok {
		return nil, fmt.Errorf("aksi massal tidak dikenal: %s", action)
	}
	var changed []idx.ID
	err := r.db.WithContext(ctx).Raw(query, map[string]any{"ids": ids, "user": userID}).Scan(&changed).Error
	if err != nil {
		return nil, fmt.Errorf("aksi massal %s: %w", action, err)
	}
	return changed, nil
}
