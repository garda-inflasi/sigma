// Package jobs merakit mesin penentuan siklus untuk dipakai bersama oleh route HTTP (cmd/api) dan pekerjaan latar
// belakang (cmd/worker): evaluasi terjadwal dan pemicu setelah alert EWS ditulis.
package jobs

import (
	"context"
	"sync"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	ewsdomain "github.com/garda-inflasi/api/internal/ews/domain"
	platformjobs "github.com/garda-inflasi/api/internal/platform/jobs"
	"github.com/garda-inflasi/api/internal/siklus/domain"
	"github.com/garda-inflasi/api/internal/siklus/engine"
	"github.com/garda-inflasi/api/internal/siklus/repository/postgres"
	"github.com/garda-inflasi/api/pkg/logger"
	"gorm.io/gorm"
)

const (
	// NamaJob adalah nama job terjadwal; interval-nya dari JOB_SIKLUS_EVALUASI_INTERVAL ("off" mematikan).
	NamaJob = "siklus_evaluasi"
	// DefaultInterval adalah jeda bawaan evaluasi terjadwal. Satu putaran hanya beberapa kueri ringan; jedanya
	// menentukan seberapa cepat pemulihan (alert diselesaikan, stok turun) terdeteksi.
	DefaultInterval = 5 * time.Minute
	// JedaPemicu menggabungkan alert yang ditulis berturutan oleh satu putaran rule engine EWS menjadi satu evaluasi.
	JedaPemicu = 3 * time.Second
)

// IntervalFromEnv membaca JOB_SIKLUS_EVALUASI_INTERVAL (durasi Go, minimal 1s; "off" mematikan; kosong = bawaan).
func IntervalFromEnv(getenv func(string) string) (time.Duration, error) {
	return platformjobs.IntervalFromEnv(getenv, NamaJob, DefaultInterval)
}

// Jobs menampung komponen siklus bersama untuk satu koneksi basis data.
type Jobs struct {
	Engine *engine.Engine
	// Pemicu mengevaluasi ulang siklus setelah alert EWS baru ditulis. Daftarkan ke rule engine EWS
	// (Engine.AddObserver) dan jalankan Pemicu.Run.
	Pemicu *Pemicu

	log *logger.Logger
}

var (
	sharedMu sync.Mutex
	shared   = map[*gorm.DB]*Jobs{}
)

// Shared mengembalikan satu Jobs per koneksi basis data, agar route dan job dalam satu proses memakai mesin yang sama.
func Shared(db *gorm.DB, log *logger.Logger) *Jobs {
	sharedMu.Lock()
	defer sharedMu.Unlock()
	if j, ok := shared[db]; ok {
		return j
	}
	eng := engine.New(postgres.New(db), audit.NewWriter(db), log)
	j := &Jobs{Engine: eng, Pemicu: NewPemicu(eng, domain.WilayahBawaan, JedaPemicu, log), log: log}
	shared[db] = j
	return j
}

// Evaluasi menjalankan satu putaran terjadwal untuk provinsi bawaan. Idempoten: aman diulang dan aman berjalan
// bersamaan dengan evaluasi lain.
func (j *Jobs) Evaluasi(ctx context.Context) error {
	hasil, err := j.Engine.Evaluasi(ctx, domain.WilayahBawaan, domain.OpsiEvaluasi{Pemicu: domain.PemicuJadwal})
	if err != nil {
		return err
	}
	j.log.Debug("siklus: evaluasi terjadwal selesai", map[string]interface{}{
		"wilayah": hasil.WilayahKode, "dilewati": hasil.Dilewati, "berubah": hasil.Berubah, "ringkasan": hasil.Ringkasan,
	})
	return nil
}

// RunBerkala mengevaluasi segera lalu tiap interval sampai ctx selesai; dipakai api bila tidak ada worker.
func (j *Jobs) RunBerkala(ctx context.Context, interval time.Duration) {
	j.Engine.Run(ctx, domain.WilayahBawaan, interval)
}

// Pemicu mengevaluasi siklus segera setelah rule engine EWS menulis alert, tanpa menunggu jadwal berikutnya.
// Ia pengamat EWS (ewsdomain.Observer) yang hanya menandai "perlu evaluasi"; pekerjaan sebenarnya berjalan di
// Run, sehingga rule engine tidak pernah diperlambat atau digagalkan oleh evaluasi siklus.
//
// Rentetan alert dari satu putaran EWS digabung (JedaPemicu) menjadi satu evaluasi. Hanya alert yang dibuat atau naik
// tingkat yang memberi sinyal; alert yang diselesaikan pengguna tidak, dan pemulihannya diambil evaluasi terjadwal.
type Pemicu struct {
	evaluator domain.Evaluator
	wilayah   string
	jeda      time.Duration
	log       *logger.Logger
	sinyal    chan struct{}
}

var _ ewsdomain.Observer = (*Pemicu)(nil)

// NewPemicu membuat Pemicu untuk satu wilayah; jeda adalah masa menunggu sebelum mengevaluasi sesudah sinyal pertama.
func NewPemicu(evaluator domain.Evaluator, wilayahKode string, jeda time.Duration, log *logger.Logger) *Pemicu {
	return &Pemicu{evaluator: evaluator, wilayah: wilayahKode, jeda: jeda, log: log, sinyal: make(chan struct{}, 1)}
}

// AlertDitulis menandai bahwa evaluasi diperlukan. Tidak pernah memblokir.
func (p *Pemicu) AlertDitulis(_ context.Context, _ ewsdomain.Alert, _ ewsdomain.HasilTulis) {
	select {
	case p.sinyal <- struct{}{}:
	default: // sudah ada sinyal yang menunggu; satu evaluasi akan mencakup keduanya
	}
}

// Run melayani sinyal sampai ctx selesai: tunggu jeda, kuras sinyal yang masuk selama jeda (sudah tercakup),
// lalu evaluasi. Sinyal yang tiba selama evaluasi tetap tersimpan dan memicu evaluasi berikutnya.
func (p *Pemicu) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.sinyal:
		}
		timer := time.NewTimer(p.jeda)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		select {
		case <-p.sinyal:
		default:
		}
		_, err := p.evaluator.Evaluasi(ctx, p.wilayah, domain.OpsiEvaluasi{Pemicu: domain.PemicuAlert})
		if err != nil && ctx.Err() == nil {
			p.log.Error("siklus: evaluasi setelah alert gagal", map[string]interface{}{"wilayah": p.wilayah, "error": err.Error()})
		}
	}
}
