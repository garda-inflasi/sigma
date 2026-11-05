package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/ews/domain"
)

var _ domain.EventRepository = (*Repository)(nil)

func (r *Repository) LatestEventID(ctx context.Context) (int64, error) {
	var id int64
	if err := r.db.WithContext(ctx).Raw("SELECT COALESCE(MAX(id), 0) FROM ews_event").Scan(&id).Error; err != nil {
		return 0, fmt.Errorf("event ews terakhir: %w", err)
	}
	return id, nil
}

func (r *Repository) EventsAfter(ctx context.Context, after int64, limit int) ([]domain.Peristiwa, error) {
	var rows []struct {
		EventID int64 `gorm:"column:event_id"`
		domain.Alert
	}
	err := r.db.WithContext(ctx).Table("ews_event AS e").
		Select("e.id AS event_id, a.*, k.nama AS komoditas_nama, w.nama AS kabupaten_nama").
		Joins("JOIN ews_alert a ON a.id = e.alert_id").
		Joins("LEFT JOIN komoditas k ON k.kode = a.komoditas_kode").
		Joins("JOIN kabupaten w ON w.kode = a.kabupaten_kode").
		Where("e.id > ?", after).
		Order("e.id").Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("event ews: %w", err)
	}
	out := make([]domain.Peristiwa, len(rows))
	for i, row := range rows {
		out[i] = domain.Peristiwa{ID: row.EventID, Alert: row.Alert}
	}
	return out, nil
}

func (r *Repository) PurgeEventsBefore(ctx context.Context, t time.Time) (int64, error) {
	res := r.db.WithContext(ctx).Exec("DELETE FROM ews_event WHERE dibuat_pada < ?", t)
	if res.Error != nil {
		return 0, fmt.Errorf("buang event ews lama: %w", res.Error)
	}
	return res.RowsAffected, nil
}
