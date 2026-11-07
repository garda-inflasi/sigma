// Package refresher menyegarkan agregat dashboard (materialized view) atas
// entri_harga: secara berkala dan setelah data harga berubah.
package refresher

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/garda-inflasi/api/pkg/logger"
	"gorm.io/gorm"
)

// Views berurutan menurut ketergantungan: agregat lebih dulu, lalu turunannya.
var Views = []string{"agregat_harga_harian", "mv_heatmap_harga", "mv_kpi_operasional", "gold_harga_harian"}

const refreshTimeout = 2 * time.Minute

// Refresher menjalankan REFRESH MATERIALIZED VIEW CONCURRENTLY lewat garda_segarkan_mv (pembaca tidak
// terblokir). Permintaan yang datang saat penyegaran berjalan digabung menjadi
// satu penyegaran lanjutan.
type Refresher struct {
	db  *gorm.DB
	log *logger.Logger

	mu      sync.Mutex
	running bool
	dirty   bool
}

var (
	sharedMu sync.Mutex
	shared   = map[*gorm.DB]*Refresher{}
)

// Shared mengembalikan satu Refresher per koneksi basis data, agar modul yang
// berbeda saling menggabungkan permintaannya.
func Shared(db *gorm.DB, log *logger.Logger) *Refresher {
	sharedMu.Lock()
	defer sharedMu.Unlock()
	if r, ok := shared[db]; ok {
		return r
	}
	r := &Refresher{db: db, log: log}
	shared[db] = r
	return r
}

// Refresh menyegarkan semua view dan menunggu selesai.
func (r *Refresher) Refresh(ctx context.Context) error {
	for _, view := range Views {
		// Lewat fungsi SECURITY DEFINER (migrasi 000119): REFRESH butuh pemilik view, sedangkan pengguna DB aplikasi
		// sengaja bukan pemilik (hak minimum).
		if err := r.db.WithContext(ctx).Exec("SELECT garda_segarkan_mv(?, true)", view).Error; err != nil {
			return fmt.Errorf("refresh %s: %w", view, err)
		}
	}
	return nil
}

// Trigger meminta penyegaran di latar belakang tanpa menunggu. Galat dicatat di log.
func (r *Refresher) Trigger() {
	r.mu.Lock()
	if r.running {
		r.dirty = true
		r.mu.Unlock()
		return
	}
	r.running = true
	r.mu.Unlock()

	go r.loop()
}

func (r *Refresher) loop() {
	for {
		ctx, cancel := context.WithTimeout(context.Background(), refreshTimeout)
		if err := r.Refresh(ctx); err != nil {
			r.log.Error("dashboard: gagal menyegarkan agregat", map[string]interface{}{"error": err.Error()})
		}
		cancel()

		r.mu.Lock()
		if !r.dirty {
			r.running = false
			r.mu.Unlock()
			return
		}
		r.dirty = false
		r.mu.Unlock()
	}
}

// Run menyegarkan segera lalu tiap interval sampai ctx selesai.
func (r *Refresher) Run(ctx context.Context, interval time.Duration) {
	r.Trigger()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.Trigger()
		}
	}
}
