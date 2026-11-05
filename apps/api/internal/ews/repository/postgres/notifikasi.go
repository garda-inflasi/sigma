package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/ews/domain"
	"gorm.io/gorm/clause"
)

var _ domain.NotifikasiRepository = (*Repository)(nil)

// antrekanSQL memasukkan notifikasi untuk pengguna aktif berperan @peran. Kanal dibatasi
// kanal aturan alert (aturan_ews.kanal) dan kanal outbox; pengguna tanpa preferensi memakai
// bawaan (aktif, tingkat minimum SIAGA). Urutan enum severity menjadikan >= valid.
const antrekanSQL = `
INSERT INTO ews_notifikasi (alert_id, pengguna_id, kanal, tujuan, jadwal_berikut)
SELECT a.id, u.id, k.kanal, CASE k.kanal WHEN 'EMAIL' THEN u.email ELSE u.id END, @now
  FROM ews_alert a
  JOIN aturan_ews r ON r.id = a.aturan_id
  CROSS JOIN LATERAL unnest(r.kanal) AS k(kanal)
  JOIN pengguna u ON u.status = 'AKTIF' AND u.peran::text IN @peran
  LEFT JOIN preferensi_notifikasi p ON p.pengguna_id = u.id AND p.kanal = k.kanal
 WHERE a.id = @alert AND k.kanal IN @kanal
   AND COALESCE(p.aktif, true)
   AND a.severity >= COALESCE(p.severity_min, CAST(@bawaan AS severity))
ON CONFLICT (alert_id, pengguna_id, kanal) DO NOTHING
RETURNING id`

func (r *Repository) Antrekan(ctx context.Context, alertID string, peran []string, now time.Time) (int, error) {
	if len(peran) == 0 {
		return 0, nil
	}
	var ids []int64
	err := r.db.WithContext(ctx).Raw(antrekanSQL, map[string]any{
		"now": now, "peran": peran, "alert": alertID, "kanal": domain.KanalOutbox, "bawaan": domain.SeverityMinBawaan,
	}).Scan(&ids).Error
	if err != nil {
		return 0, fmt.Errorf("antrekan notifikasi: %w", err)
	}
	return len(ids), nil
}

func (r *Repository) Klaim(ctx context.Context, now time.Time, limit int, lease time.Duration) ([]domain.Pesan, error) {
	var out []domain.Pesan
	err := r.db.WithContext(ctx).Raw(`
		WITH jatuh_tempo AS (
			SELECT id FROM ews_notifikasi
			 WHERE status = 'MENUNGGU' AND jadwal_berikut <= @now
			 ORDER BY jadwal_berikut, id LIMIT @limit
			   FOR UPDATE SKIP LOCKED
		), diklaim AS (
			UPDATE ews_notifikasi n SET jadwal_berikut = @sampai
			  FROM jatuh_tempo j WHERE n.id = j.id
			RETURNING n.id, n.alert_id, n.pengguna_id, n.kanal, n.tujuan, n.percobaan
		)
		SELECT d.id AS notifikasi_id, d.kanal, d.percobaan, d.pengguna_id, d.tujuan, d.alert_id,
		       a.severity, a.judul, a.pesan AS isi
		  FROM diklaim d JOIN ews_alert a ON a.id = d.alert_id
		 ORDER BY d.id`,
		map[string]any{"now": now, "limit": limit, "sampai": now.Add(lease)}).Scan(&out).Error
	if err != nil {
		return nil, fmt.Errorf("klaim notifikasi: %w", err)
	}
	return out, nil
}

func (r *Repository) Tandai(ctx context.Context, id int64, status, galat string, jadwalBerikut, now time.Time) error {
	res := r.db.WithContext(ctx).Exec(`
		UPDATE ews_notifikasi
		   SET status = @status, percobaan = percobaan + 1, galat = NULLIF(@galat, ''),
		       jadwal_berikut = CASE WHEN @status = 'MENUNGGU' THEN @jadwal ELSE jadwal_berikut END,
		       dikirim_pada = CASE WHEN @status = 'TERKIRIM' THEN @now ELSE dikirim_pada END
		 WHERE id = @id`,
		map[string]any{"status": status, "galat": galat, "jadwal": jadwalBerikut, "now": now, "id": id})
	if res.Error != nil {
		return fmt.Errorf("tandai notifikasi: %w", res.Error)
	}
	return nil
}

func (r *Repository) ListPreferensi(ctx context.Context, penggunaID string) ([]domain.PreferensiNotifikasi, error) {
	var items []domain.PreferensiNotifikasi
	if err := r.db.WithContext(ctx).Where("pengguna_id = ?", penggunaID).Order("kanal").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("preferensi notifikasi: %w", err)
	}
	return items, nil
}

func (r *Repository) SimpanPreferensi(ctx context.Context, penggunaID string, prefs []domain.PreferensiNotifikasi) error {
	if len(prefs) == 0 {
		return nil
	}
	rows := make([]domain.PreferensiNotifikasi, len(prefs))
	for i, p := range prefs {
		p.PenggunaID = penggunaID
		p.DiubahPada = time.Now()
		rows[i] = p
	}
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "pengguna_id"}, {Name: "kanal"}},
		DoUpdates: clause.AssignmentColumns([]string{"severity_min", "aktif", "diubah_pada"}),
	}).Create(&rows).Error
	if err != nil {
		return fmt.Errorf("simpan preferensi notifikasi: %w", err)
	}
	return nil
}

func (r *Repository) SimpanLangganan(ctx context.Context, l domain.LanggananPush) error {
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "endpoint"}},
		DoUpdates: clause.AssignmentColumns([]string{"pengguna_id", "p256dh", "auth"}),
	}).Create(&l).Error
	if err != nil {
		return fmt.Errorf("simpan langganan push: %w", err)
	}
	return nil
}

func (r *Repository) HapusLangganan(ctx context.Context, penggunaID, endpoint string) (bool, error) {
	res := r.db.WithContext(ctx).Where("pengguna_id = ? AND endpoint = ?", penggunaID, endpoint).Delete(&domain.LanggananPush{})
	if res.Error != nil {
		return false, fmt.Errorf("hapus langganan push: %w", res.Error)
	}
	return res.RowsAffected > 0, nil
}

func (r *Repository) ListLangganan(ctx context.Context, penggunaID string) ([]domain.LanggananPush, error) {
	var items []domain.LanggananPush
	if err := r.db.WithContext(ctx).Where("pengguna_id = ?", penggunaID).Order("id").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("langganan push: %w", err)
	}
	return items, nil
}

func (r *Repository) HapusLanggananKadaluwarsa(ctx context.Context, endpoint string) error {
	if err := r.db.WithContext(ctx).Where("endpoint = ?", endpoint).Delete(&domain.LanggananPush{}).Error; err != nil {
		return fmt.Errorf("hapus langganan push kadaluwarsa: %w", err)
	}
	return nil
}

func (r *Repository) EskalasiOtomatis(ctx context.Context, now time.Time, batas time.Duration, minimum string) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).Raw(`
		WITH prev AS (
			SELECT id FROM ews_alert
			 WHERE status = 'BARU' AND dibuat_pada <= @batas AND severity >= CAST(@minimum AS severity)
			   FOR UPDATE
		), upd AS (
			UPDATE ews_alert a SET status = 'DIESKALASI', dieskalasi_pada = @now
			  FROM prev WHERE a.id = prev.id
			RETURNING a.id
		), riwayat AS (
			INSERT INTO ews_riwayat (alert_id, dari, ke, oleh, waktu, catatan)
			SELECT id, 'BARU', 'DIESKALASI', NULL, @now, @catatan FROM upd
		)
		SELECT id FROM upd ORDER BY id`,
		map[string]any{
			"batas": now.Add(-batas), "minimum": minimum, "now": now,
			"catatan": fmt.Sprintf("Eskalasi otomatis: tidak diakui dalam %d menit", int(batas.Minutes())),
		}).Scan(&ids).Error
	if err != nil {
		return nil, fmt.Errorf("eskalasi otomatis: %w", err)
	}
	return ids, nil
}
