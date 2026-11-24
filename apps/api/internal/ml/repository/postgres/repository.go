// Package postgres adalah akses PostgreSQL modul ML: cakupan baris pemanggil, tautan sinyal ke alert, dan keadaan
// jembatan EWS (ews_sinyal_ml).
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/ml/domain"
	"github.com/garda-inflasi/api/pkg/idx"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

var _ domain.PostgresRepository = (*Repository)(nil)

func New(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) PenggunaCakupan(ctx context.Context, userID string) (string, string, bool, error) {
	var rows []struct {
		Peran         string `gorm:"column:peran"`
		KabupatenKode string `gorm:"column:kabupaten_kode"`
	}
	err := r.db.WithContext(ctx).Raw(
		`SELECT peran::text AS peran, COALESCE(kabupaten_kode, '') AS kabupaten_kode FROM pengguna WHERE id = ?`, userID,
	).Scan(&rows).Error
	if err != nil {
		return "", "", false, fmt.Errorf("baca peran pengguna: %w", err)
	}
	if len(rows) == 0 {
		return "", "", false, nil
	}
	return rows[0].Peran, rows[0].KabupatenKode, true, nil
}

func (r *Repository) TautanAlert(ctx context.Context, kunci []string) (map[string]domain.TautanAlert, error) {
	out := map[string]domain.TautanAlert{}
	if len(kunci) == 0 {
		return out, nil
	}
	var rows []domain.TautanAlert
	err := r.db.WithContext(ctx).Raw(`
		SELECT s.sinyal_kunci, a.id AS alert_id, a.kode AS alert_kode, a.status::text AS status
		  FROM ews_sinyal_ml s JOIN ews_alert a ON a.id = s.alert_id
		 WHERE s.sinyal_kunci IN ?`, kunci).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("tautan sinyal ke alert: %w", err)
	}
	for _, row := range rows {
		out[row.SinyalKunci] = row
	}
	return out, nil
}

// ── Keadaan jembatan EWS ─────────────────────────────────────────────

// MuatKeadaan mengembalikan keadaan jembatan untuk kunci yang diminta.
func (r *Repository) MuatKeadaan(ctx context.Context, kunci []string) (map[string]domain.EwsSinyalML, error) {
	out := map[string]domain.EwsSinyalML{}
	if len(kunci) == 0 {
		return out, nil
	}
	var rows []domain.EwsSinyalML
	if err := r.db.WithContext(ctx).Where("sinyal_kunci IN ?", kunci).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("muat keadaan jembatan: %w", err)
	}
	for _, row := range rows {
		out[row.SinyalKunci] = row
	}
	return out, nil
}

// SimpanKeadaan menulis (upsert) keadaan satu kunci.
func (r *Repository) SimpanKeadaan(ctx context.Context, k domain.EwsSinyalML) error {
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "sinyal_kunci"}},
		DoUpdates: clause.AssignmentColumns([]string{"jenis_ml", "severity", "aktif", "cycle_id", "cycle_timestamp", "alert_id", "diperbarui_pada"}),
	}).Create(&k).Error
	if err != nil {
		return fmt.Errorf("simpan keadaan jembatan %s: %w", k.SinyalKunci, err)
	}
	return nil
}

// AlertAktif mengembalikan id alert yang belum SELESAI untuk dedup key, nil bila tidak ada.
func (r *Repository) AlertAktif(ctx context.Context, dedupKey string) (*idx.ID, error) {
	var ids []idx.ID
	err := r.db.WithContext(ctx).Raw(`SELECT id FROM ews_alert WHERE dedup_key = ? AND status <> 'SELESAI' LIMIT 1`, dedupKey).
		Scan(&ids).Error
	if err != nil {
		return nil, fmt.Errorf("cari alert aktif: %w", err)
	}
	if len(ids) == 0 {
		return nil, nil
	}
	return &ids[0], nil
}

// CatatMereda menambah baris riwayat pada alert yang masih terbuka bahwa kondisinya tidak lagi terdeteksi pipeline.
// Status tidak diubah: alert ditutup oleh pengguna, tidak pernah otomatis. Mengembalikan false bila alert sudah SELESAI.
func (r *Repository) CatatMereda(ctx context.Context, alertID idx.ID, catatan string, now time.Time) (bool, error) {
	res := r.db.WithContext(ctx).Exec(`
		INSERT INTO ews_riwayat (alert_id, dari, ke, oleh, waktu, catatan)
		SELECT id, status, status, NULL, ?, ? FROM ews_alert WHERE id = ? AND status <> 'SELESAI'`, now, catatan, alertID)
	if res.Error != nil {
		return false, fmt.Errorf("catat kondisi mereda: %w", res.Error)
	}
	return res.RowsAffected > 0, nil
}

// KodeMaster mengembalikan kode komoditas dan kabupaten yang ada di master (rujukan FK ews_alert).
func (r *Repository) KodeMaster(ctx context.Context) (komoditas, kabupaten map[string]bool, err error) {
	var kom, kab []string
	if err := r.db.WithContext(ctx).Raw(`SELECT kode FROM komoditas`).Scan(&kom).Error; err != nil {
		return nil, nil, fmt.Errorf("baca kode komoditas: %w", err)
	}
	if err := r.db.WithContext(ctx).Raw(`SELECT kode FROM kabupaten`).Scan(&kab).Error; err != nil {
		return nil, nil, fmt.Errorf("baca kode kabupaten: %w", err)
	}
	komoditas, kabupaten = make(map[string]bool, len(kom)), make(map[string]bool, len(kab))
	for _, k := range kom {
		komoditas[k] = true
	}
	for _, k := range kab {
		kabupaten[k] = true
	}
	return komoditas, kabupaten, nil
}
