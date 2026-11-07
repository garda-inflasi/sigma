// Package jobs adalah penjadwal job berkala untuk proses di luar API (cmd/worker).
//
// Setiap job berjalan pada intervalnya sendiri dan dilindungi advisory lock
// Postgres (pg_try_advisory_xact_lock) sehingga, bila dua instance worker (atau
// worker dan API) hidup bersamaan, satu job tidak pernah berjalan ganda: instance
// yang kalah mengambil lock melewati putaran itu. Lock dilepas otomatis saat
// transaksi selesai, termasuk ketika proses mati.
//
// Mendaftarkan job baru: lihat bagian "Penjadwal" di apps/api/README.md.
package jobs

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/garda-inflasi/api/pkg/logger"
	"gorm.io/gorm"
)

// DefaultTimeout adalah batas waktu satu putaran job bila Job.Timeout kosong.
const DefaultTimeout = 5 * time.Minute

var nameRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,48}$`)

// Job adalah satu pekerjaan berkala.
type Job struct {
	// Name unik, huruf kecil/angka/garis bawah. Dipakai sebagai kunci advisory lock
	// dan sebagai bagian nama variabel env (JOB_<NAME>_INTERVAL).
	Name string
	// Interval antar putaran. Nol atau negatif = job dinonaktifkan (tidak dijadwalkan).
	Interval time.Duration
	// Timeout satu putaran (bawaan DefaultTimeout).
	Timeout time.Duration
	// RunOnStart menjalankan putaran pertama segera saat scheduler mulai.
	RunOnStart bool
	// Run harus idempoten: boleh dijalankan ulang kapan saja tanpa efek ganda.
	Run func(ctx context.Context) error
}

// Status adalah ringkasan putaran terakhir sebuah job (untuk endpoint kesehatan).
type Status struct {
	Name      string     `json:"name"`
	Interval  string     `json:"interval"`
	Runs      int        `json:"runs"`
	Skipped   int        `json:"skipped"`
	LastRun   *time.Time `json:"last_run,omitempty"`
	LastError string     `json:"last_error,omitempty"`
}

// Scheduler menjalankan sekumpulan Job.
type Scheduler struct {
	db  *gorm.DB
	log *logger.Logger
	now func() time.Time

	mu     sync.Mutex
	jobs   []Job
	status map[string]*Status
}

// New membuat scheduler. db dipakai untuk advisory lock.
func New(db *gorm.DB, log *logger.Logger) *Scheduler {
	return &Scheduler{db: db, log: log, now: time.Now, status: map[string]*Status{}}
}

// Add mendaftarkan job. Gagal bila nama tidak valid, ganda, atau Run kosong.
func (s *Scheduler) Add(j Job) error {
	if !nameRe.MatchString(j.Name) {
		return fmt.Errorf("jobs: nama %q tidak valid (a-z, 0-9, _)", j.Name)
	}
	if j.Run == nil {
		return fmt.Errorf("jobs: job %q tanpa fungsi Run", j.Name)
	}
	if j.Timeout <= 0 {
		j.Timeout = DefaultTimeout
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, other := range s.jobs {
		if other.Name == j.Name {
			return fmt.Errorf("jobs: job %q terdaftar ganda", j.Name)
		}
	}
	s.jobs = append(s.jobs, j)
	s.status[j.Name] = &Status{Name: j.Name, Interval: j.Interval.String()}
	return nil
}

// Names mengembalikan nama semua job terdaftar (termasuk yang dinonaktifkan).
func (s *Scheduler) Names() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	names := make([]string, 0, len(s.jobs))
	for _, j := range s.jobs {
		names = append(names, j.Name)
	}
	return names
}

// Statuses mengembalikan salinan status tiap job.
func (s *Scheduler) Statuses() []Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Status, 0, len(s.jobs))
	for _, j := range s.jobs {
		out = append(out, *s.status[j.Name])
	}
	return out
}

// ErrUnknownJob dikembalikan RunOnce untuk nama yang tidak terdaftar.
var ErrUnknownJob = errors.New("jobs: job tidak dikenal")

type kunciManual struct{}

// Manual melaporkan apakah putaran job dijalankan manual (RunOnce, mis. `worker run <job>`), bukan oleh jadwal.
func Manual(ctx context.Context) bool {
	manual, _ := ctx.Value(kunciManual{}).(bool)
	return manual
}

// RunOnce menjalankan satu putaran job sekarang (tetap dengan advisory lock).
// ran=false berarti putaran dilewati karena instance lain memegang lock.
// Context job ditandai Manual.
func (s *Scheduler) RunOnce(ctx context.Context, name string) (ran bool, err error) {
	ctx = context.WithValue(ctx, kunciManual{}, true)
	s.mu.Lock()
	var job *Job
	for i := range s.jobs {
		if s.jobs[i].Name == name {
			job = &s.jobs[i]
		}
	}
	s.mu.Unlock()
	if job == nil {
		return false, fmt.Errorf("%w: %s", ErrUnknownJob, name)
	}
	return s.runLocked(ctx, *job)
}

// Run memulai semua job aktif dan memblokir sampai ctx selesai; lalu menunggu
// putaran yang sedang berjalan selesai (masing-masing dibatasi Timeout).
func (s *Scheduler) Run(ctx context.Context) {
	s.mu.Lock()
	jobs := append([]Job(nil), s.jobs...)
	s.mu.Unlock()

	var wg sync.WaitGroup
	for _, j := range jobs {
		if j.Interval <= 0 {
			s.log.Info("jobs: dinonaktifkan", map[string]interface{}{"job": j.Name})
			continue
		}
		wg.Add(1)
		go func(j Job) {
			defer wg.Done()
			s.loop(ctx, j)
		}(j)
	}
	wg.Wait()
}

func (s *Scheduler) loop(ctx context.Context, j Job) {
	s.log.Info("jobs: dijadwalkan", map[string]interface{}{"job": j.Name, "interval": j.Interval.String(), "run_on_start": j.RunOnStart})
	if j.RunOnStart {
		s.tick(ctx, j)
	}
	ticker := time.NewTicker(j.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx, j)
		}
	}
}

func (s *Scheduler) tick(ctx context.Context, j Job) {
	if ctx.Err() != nil {
		return
	}
	ran, err := s.runLocked(ctx, j)
	switch {
	case err != nil:
		s.log.Error("jobs: gagal", map[string]interface{}{"job": j.Name, "error": err.Error()})
	case !ran:
		s.log.Debug("jobs: dilewati, dipegang instance lain", map[string]interface{}{"job": j.Name})
	}
}

// runLocked menjalankan job di dalam transaksi yang memegang advisory lock.
// Transaksi hanya membawa lock; job memakai koneksinya sendiri.
func (s *Scheduler) runLocked(ctx context.Context, j Job) (bool, error) {
	ran := false
	started := s.now()
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var got bool
		if err := tx.Raw("SELECT pg_try_advisory_xact_lock(hashtextextended(?, 0))", lockKey(j.Name)).Scan(&got).Error; err != nil {
			return fmt.Errorf("ambil advisory lock: %w", err)
		}
		if !got {
			return nil
		}
		ran = true
		jobCtx, cancel := context.WithTimeout(ctx, j.Timeout)
		defer cancel()
		return safeRun(jobCtx, j.Run)
	})
	s.record(j.Name, started, ran, err)
	return ran, err
}

func (s *Scheduler) record(name string, started time.Time, ran bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.status[name]
	if !ran && err == nil {
		st.Skipped++
		return
	}
	st.Runs++
	t := started
	st.LastRun = &t
	st.LastError = ""
	if err != nil {
		st.LastError = err.Error()
	}
}

func lockKey(name string) string { return "garda.job." + name }

func safeRun(ctx context.Context, fn func(context.Context) error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return fn(ctx)
}

// IntervalFromEnv membaca JOB_<NAMA>_INTERVAL (mis. JOB_DASHBOARD_REFRESH_INTERVAL).
// Kosong -> def. "0", "off", "false", atau "disabled" -> 0 (job dinonaktifkan).
// Selain itu harus durasi Go (30s, 5m, 1h) minimal 1 detik.
func IntervalFromEnv(getenv func(string) string, name string, def time.Duration) (time.Duration, error) {
	key := "JOB_" + strings.ToUpper(name) + "_INTERVAL"
	raw := strings.TrimSpace(getenv(key))
	switch strings.ToLower(raw) {
	case "":
		return def, nil
	case "0", "off", "false", "disabled":
		return 0, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s=%q bukan durasi yang valid: %w", key, raw, err)
	}
	if d < time.Second {
		return 0, fmt.Errorf("%s=%q terlalu kecil (minimal 1s; pakai off untuk menonaktifkan)", key, raw)
	}
	return d, nil
}
