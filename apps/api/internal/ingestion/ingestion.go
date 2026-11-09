// Package ingestion merakit engine ingestion untuk proses nyata (cmd/api, cmd/worker,
// dan cmd/ingest): konektor terdaftar, Fetcher HTTP berlapis (batas laju, robots.txt,
// retry, circuit breaker), dan penyegaran agregat gold setelah data masuk.
package ingestion

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/garda-inflasi/api/internal/dashboard/refresher"
	"github.com/garda-inflasi/api/internal/ingestion/connector/bapanas"
	"github.com/garda-inflasi/api/internal/ingestion/connector/bmkg"
	"github.com/garda-inflasi/api/internal/ingestion/connector/bps"
	"github.com/garda-inflasi/api/internal/ingestion/connector/openweather"
	"github.com/garda-inflasi/api/internal/ingestion/connector/pihps"
	"github.com/garda-inflasi/api/internal/ingestion/domain"
	"github.com/garda-inflasi/api/internal/ingestion/engine"
	"github.com/garda-inflasi/api/internal/ingestion/repository/postgres"
	"github.com/garda-inflasi/api/internal/ingestion/scheduler"
	sumber "github.com/garda-inflasi/api/internal/sumber-data/domain"
	"github.com/garda-inflasi/api/pkg/logger"
	"gorm.io/gorm"
)

// Variabel lingkungan ingestion (semua opsional).
const (
	// EnvUserAgent mengganti User-Agent permintaan keluar.
	EnvUserAgent = "INGEST_USER_AGENT"
	// EnvScheduler menyalakan penjadwal cron ingestion di cmd/worker ("on", "true", atau "1").
	// Bawaan mati agar lingkungan lokal dan demo tidak memanggil layanan eksternal.
	EnvScheduler = "INGEST_SCHEDULER"
	// EnvJedaMs mengganti jeda minimum (milidetik) antar permintaan ke host yang sama.
	EnvJedaMs = "INGEST_JEDA_MS"
)

const (
	userAgentBawaan = "garda-inflasi-ingest/1.0 (+https://gardainflasi.id; pemantauan inflasi pangan Aceh)"
	jedaBawaan      = 1200 * time.Millisecond
	timeoutHTTP     = 30 * time.Second

	percobaan    = 3
	backoffAwal  = 2 * time.Second
	ambangSirkit = 3
	tungguSirkit = 5 * time.Minute
)

// Konektor mengembalikan konektor yang dikenal engine. Kunci rahasia dibaca dari lingkungan.
func Konektor() []domain.Konektor {
	return []domain.Konektor{
		pihps.New(),
		bmkg.New(),
		bps.New(os.Getenv(bps.EnvKunci)),
		openweather.New(os.Getenv(openweather.EnvKunci)),
		bapanas.New(),
	}
}

// NewFetcher merakit Fetcher nyata: HTTP -> retry -> circuit breaker.
func NewFetcher() domain.Fetcher {
	ua := os.Getenv(EnvUserAgent)
	if ua == "" {
		ua = userAgentBawaan
	}
	jeda := jedaBawaan
	if ms, err := strconv.Atoi(os.Getenv(EnvJedaMs)); err == nil && ms >= 0 {
		jeda = time.Duration(ms) * time.Millisecond
	}
	httpFetcher := engine.NewHTTPFetcher(ua, jeda, timeoutHTTP)
	return &engine.Breaker{
		Next:   &engine.Retry{Next: httpFetcher, Percobaan: percobaan, Awal: backoffAwal},
		Ambang: ambangSirkit, Tunggu: tungguSirkit,
	}
}

// NewRunner merakit Runner nyata di atas db. Setelah baris silver ditulis, agregat
// dashboard (termasuk gold_harga_harian) disegarkan di latar belakang.
// Bila eksekusi sebuah sumber mengalami kegagalan berulang kali, alert peringatan
// dini (EWS) otomatis diterbitkan.
func NewRunner(db *gorm.DB, log *logger.Logger) *engine.Runner {
	r := engine.New(postgres.New(db), NewFetcher(), log, Konektor()...)
	r.SetelahSilver = refresher.Shared(db, log).Trigger
	r.PadaEksekusiSelesai = func(ctx context.Context, e *domain.EksekusiSumber, s *sumber.SumberData) {
		PeriksaAlertKegagalan(ctx, db, log, e, s)
	}
	return r
}

// PeriksaAlertKegagalan memantau status akhir eksekusi sumber. Bila sumber gagal 3 kali
// berturut-turut, alert ANOMALI_DATA diterbitkan ke tabel ews_alert. Bila sumber pulih
// dan sukses, alert tersebut diselesaikan secara otomatis.
func PeriksaAlertKegagalan(ctx context.Context, db *gorm.DB, log *logger.Logger, e *domain.EksekusiSumber, s *sumber.SumberData) {
	if s == nil || db == nil {
		return
	}
	dedup := "ingestion:sumber:" + s.Kode

	if e.Status == domain.StatusGagal {
		var gagalTerakhir int64
		err := db.WithContext(ctx).Raw(`
			SELECT count(*) FROM (
				SELECT status FROM eksekusi_sumber 
				 WHERE sumber_id = ? AND status <> 'BERJALAN' 
				 ORDER BY mulai DESC, id DESC LIMIT 3
			) sub WHERE status = 'GAGAL'`, s.ID).Scan(&gagalTerakhir).Error
		if err != nil {
			log.Warn("ingestion: gagal memeriksa riwayat kegagalan sumber untuk ews alert", map[string]interface{}{"sumber": s.Kode, "error": err.Error()})
			return
		}

		if gagalTerakhir >= 3 {
			var ada int64
			_ = db.WithContext(ctx).Raw(`SELECT count(*) FROM ews_alert WHERE dedup_key = ? AND status <> 'SELESAI'`, dedup).Scan(&ada).Error
			if ada == 0 {
				galat := "kegagalan berulang tanpa rincian"
				if e.Galat != nil && *e.Galat != "" {
					galat = *e.Galat
				}
				pesan := fmt.Sprintf("Sumber data %s (%s) mengalami kegagalan dalam 3 eksekusi terakhir berturut-turut. Galat terakhir: %s", s.Nama, s.Kode, galat)
				judul := "Gangguan Ingestion: " + s.Nama

				_ = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
					var alertID int64
					err := tx.Raw(`
						INSERT INTO ews_alert (kode, severity, jenis, judul, pesan, kabupaten_kode, nilai, satuan_nilai, skor_dampak, status, sumber, dibuat_pada, dedup_key)
						VALUES ('EWS-' || nextval('ews_alert_kode_seq')::text, CAST('WASPADA' AS severity), CAST('ANOMALI_DATA' AS jenis_ews), ?, ?, '1171', 3, 'kali', 40, 'BARU', 'Ingestion Engine', now(), ?)
						ON CONFLICT (dedup_key) WHERE dedup_key IS NOT NULL AND status <> 'SELESAI' DO NOTHING
						RETURNING id`, judul, pesan, dedup).Scan(&alertID).Error
					if err != nil || alertID == 0 {
						return err
					}
					return tx.Exec(`
						INSERT INTO ews_riwayat (alert_id, dari, ke, oleh, waktu, catatan)
						VALUES (?, NULL, 'BARU', NULL, now(), 'Dibuat otomatis oleh ingestion engine karena kegagalan berulang')`, alertID).Error
				})
			}
		}
	} else if e.Status == domain.StatusSukses {
		_ = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var ids []int64
			tx.Raw(`
				UPDATE ews_alert
				   SET status = 'SELESAI', selesai_pada = now()
				 WHERE dedup_key = ? AND status <> 'SELESAI'
				RETURNING id`, dedup).Scan(&ids)
			for _, id := range ids {
				_ = tx.Exec(`
					INSERT INTO ews_riwayat (alert_id, dari, ke, oleh, waktu, catatan)
					VALUES (?, 'BARU', 'SELESAI', NULL, now(), 'Sumber data kembali pulih dan berjalan sukses')`, id).Error
			}
			return nil
		})
	}
}

// SchedulerAktif benar bila penjadwal diaktifkan lewat lingkungan.
func SchedulerAktif(getenv func(string) string) bool {
	switch strings.ToLower(strings.TrimSpace(getenv(EnvScheduler))) {
	case "on", "true", "1":
		return true
	}
	return false
}

// NewScheduler merakit penjadwal di atas Runner nyata.
func NewScheduler(db *gorm.DB, log *logger.Logger) *scheduler.Penjadwal {
	return scheduler.New(postgres.New(db), NewRunner(db, log), log)
}
