// Package scheduler menjalankan sumber data menurut sumber_data.jadwal_cron
// (zona waktu WIB). Penjadwal hanya berjalan bila diaktifkan lewat lingkungan
// (INGEST_SCHEDULER), sehingga lingkungan lokal dan demo tidak memanggil internet.
package scheduler

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/garda-inflasi/api/internal/ingestion/cron"
	"github.com/garda-inflasi/api/internal/ingestion/domain"
	"github.com/garda-inflasi/api/internal/ingestion/engine"
	"github.com/garda-inflasi/api/pkg/logger"
)

// Interval adalah jarak pemeriksaan jadwal; lebih rapat dari semenit agar tidak ada menit terlewat.
const Interval = 20 * time.Second

// Pelari adalah bagian Runner yang dibutuhkan penjadwal.
type Pelari interface {
	Punya(sumberID string) bool
	Mulai(ctx context.Context, sumberID, pemicu string, aktorID *string) (*engine.Run, error)
}

// Penjadwal memulai eksekusi sumber yang jadwalnya jatuh pada menit berjalan.
type Penjadwal struct {
	repo   domain.Repository
	pelari Pelari
	log    *logger.Logger
	zona   *time.Location

	// Sekarang bisa diganti pada tes.
	Sekarang func() time.Time

	mu       sync.Mutex
	terakhir map[string]time.Time // menit jadwal terakhir yang sudah diproses per sumber
	wg       sync.WaitGroup
}

// New membuat Penjadwal. Zona waktu jadwal adalah Asia/Jakarta.
func New(repo domain.Repository, pelari Pelari, log *logger.Logger) *Penjadwal {
	zona, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		zona = time.FixedZone("WIB", 7*60*60)
	}
	return &Penjadwal{repo: repo, pelari: pelari, log: log, zona: zona, Sekarang: time.Now, terakhir: map[string]time.Time{}}
}

// Periksa memulai eksekusi untuk sumber yang jadwalnya cocok dengan menit ini dan
// mengembalikan jumlah yang dimulai. Eksekusi berjalan di latar belakang (lihat Tunggu).
func (p *Penjadwal) Periksa(ctx context.Context) int {
	menit := p.Sekarang().In(p.zona).Truncate(time.Minute)
	sumber, err := p.repo.SumberAktif(ctx)
	if err != nil {
		p.log.Error("ingestion: gagal membaca sumber aktif", map[string]interface{}{"error": err.Error()})
		return 0
	}

	dimulai := 0
	for _, s := range sumber {
		if !p.pelari.Punya(s.ID) {
			continue
		}
		jadwal, err := cron.Parse(s.JadwalCron)
		if err != nil {
			p.log.Warn("ingestion: jadwal sumber tidak valid", map[string]interface{}{"sumber": s.ID, "error": err.Error()})
			continue
		}
		if !jadwal.Cocok(menit) || !p.tandai(s.ID, menit) {
			continue
		}

		run, err := p.pelari.Mulai(ctx, s.ID, domain.PemicuJadwal, nil)
		switch {
		case errors.Is(err, domain.ErrSudahBerjalan), errors.Is(err, domain.ErrDijeda):
			p.log.Info("ingestion: jadwal dilewati", map[string]interface{}{"sumber": s.ID, "alasan": err.Error()})
			continue
		case err != nil:
			p.log.Error("ingestion: gagal memulai eksekusi terjadwal", map[string]interface{}{"sumber": s.ID, "error": err.Error()})
			continue
		}

		dimulai++
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			runCtx, cancel := context.WithTimeout(ctx, engine.BatasWaktu)
			defer cancel()
			if _, err := run.Jalankan(runCtx); err != nil {
				p.log.Error("ingestion: gagal mencatat eksekusi terjadwal", map[string]interface{}{"sumber": s.ID, "error": err.Error()})
			}
		}()
	}
	return dimulai
}

// tandai mencatat bahwa jadwal sumber pada menit itu diproses; salah bila sudah.
func (p *Penjadwal) tandai(sumberID string, menit time.Time) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.terakhir[sumberID].Equal(menit) {
		return false
	}
	p.terakhir[sumberID] = menit
	return true
}

// Jalankan memeriksa jadwal tiap interval sampai ctx selesai.
func (p *Penjadwal) Jalankan(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		p.Periksa(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Tunggu menunggu eksekusi yang sedang berjalan selesai (dipanggil saat shutdown).
func (p *Penjadwal) Tunggu() { p.wg.Wait() }
