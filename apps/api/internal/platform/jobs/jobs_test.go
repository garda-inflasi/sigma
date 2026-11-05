package jobs

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var pg *pgtest.Server

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m, &pg)) }

func quietLogger() *logger.Logger { return logger.New(logger.ERROR, os.Stderr) }

func TestIntervalFromEnv(t *testing.T) {
	env := func(v string) func(string) string {
		return func(k string) string {
			if k == "JOB_DASHBOARD_REFRESH_INTERVAL" {
				return v
			}
			return ""
		}
	}
	def := 5 * time.Minute

	d, err := IntervalFromEnv(env(""), "dashboard_refresh", def)
	require.NoError(t, err)
	assert.Equal(t, def, d)

	d, err = IntervalFromEnv(env("90s"), "dashboard_refresh", def)
	require.NoError(t, err)
	assert.Equal(t, 90*time.Second, d)

	for _, off := range []string{"0", "off", "OFF", "false", "disabled"} {
		d, err = IntervalFromEnv(env(off), "dashboard_refresh", def)
		require.NoError(t, err, off)
		assert.Zero(t, d, off)
	}

	_, err = IntervalFromEnv(env("lima menit"), "dashboard_refresh", def)
	assert.Error(t, err)
	_, err = IntervalFromEnv(env("10ms"), "dashboard_refresh", def)
	assert.Error(t, err)
}

func TestAddValidation(t *testing.T) {
	s := New(nil, quietLogger())
	ok := func(context.Context) error { return nil }
	require.NoError(t, s.Add(Job{Name: "a_job", Interval: time.Minute, Run: ok}))
	assert.Error(t, s.Add(Job{Name: "a_job", Interval: time.Minute, Run: ok}), "ganda")
	assert.Error(t, s.Add(Job{Name: "Bad-Name", Run: ok}), "nama tidak valid")
	assert.Error(t, s.Add(Job{Name: "no_run"}), "tanpa Run")
	assert.Equal(t, []string{"a_job"}, s.Names())
}

func newScheduler(t *testing.T, name string) (*Scheduler, *gorm.DB) {
	t.Helper()
	db := pg.NewDatabase(t, name)
	return New(db, quietLogger()), db
}

func TestRunOnceRunsAndRecords(t *testing.T) {
	s, _ := newScheduler(t, "jobs_runonce")
	var n int32
	require.NoError(t, s.Add(Job{Name: "hitung", Interval: time.Minute, Run: func(context.Context) error {
		atomic.AddInt32(&n, 1)
		return nil
	}}))

	ran, err := s.RunOnce(context.Background(), "hitung")
	require.NoError(t, err)
	assert.True(t, ran)
	assert.EqualValues(t, 1, n)

	st := s.Statuses()[0]
	assert.Equal(t, 1, st.Runs)
	assert.NotNil(t, st.LastRun)
	assert.Empty(t, st.LastError)

	_, err = s.RunOnce(context.Background(), "tidak_ada")
	assert.ErrorIs(t, err, ErrUnknownJob)
}

func TestRunOnceReportsErrorsAndPanics(t *testing.T) {
	s, _ := newScheduler(t, "jobs_errors")
	require.NoError(t, s.Add(Job{Name: "gagal", Run: func(context.Context) error { return errors.New("boom") }}))
	require.NoError(t, s.Add(Job{Name: "panik", Run: func(context.Context) error { panic("ups") }}))

	ran, err := s.RunOnce(context.Background(), "gagal")
	assert.True(t, ran)
	assert.EqualError(t, err, "boom")

	ran, err = s.RunOnce(context.Background(), "panik")
	assert.True(t, ran)
	assert.ErrorContains(t, err, "panic: ups")

	assert.Equal(t, "boom", s.Statuses()[0].LastError)
}

// Dua scheduler (mewakili dua instance) pada DB yang sama: job yang sedang
// berjalan di satu instance tidak boleh berjalan di instance lain.
func TestAdvisoryLockPreventsConcurrentRuns(t *testing.T) {
	db := pg.NewDatabase(t, "jobs_lock")
	a, b := New(db, quietLogger()), New(db, quietLogger())

	started := make(chan struct{})
	release := make(chan struct{})
	var running, maxRunning, total int32
	body := func(context.Context) error {
		cur := atomic.AddInt32(&running, 1)
		for {
			old := atomic.LoadInt32(&maxRunning)
			if cur <= old || atomic.CompareAndSwapInt32(&maxRunning, old, cur) {
				break
			}
		}
		atomic.AddInt32(&total, 1)
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		atomic.AddInt32(&running, -1)
		return nil
	}
	require.NoError(t, a.Add(Job{Name: "berat", Run: body}))
	require.NoError(t, b.Add(Job{Name: "berat", Run: body}))

	var wg sync.WaitGroup
	var ranA bool
	wg.Add(1)
	go func() {
		defer wg.Done()
		var err error
		ranA, err = a.RunOnce(context.Background(), "berat")
		assert.NoError(t, err)
	}()
	<-started // A memegang lock

	ranB, err := b.RunOnce(context.Background(), "berat")
	require.NoError(t, err)
	assert.False(t, ranB, "instance B harus melewati putaran")
	assert.Equal(t, 1, b.Statuses()[0].Skipped)

	close(release)
	wg.Wait()
	assert.True(t, ranA)
	assert.EqualValues(t, 1, maxRunning)
	assert.EqualValues(t, 1, total)

	// Setelah dilepas, B bisa menjalankan lagi.
	ranB, err = b.RunOnce(context.Background(), "berat")
	require.NoError(t, err)
	assert.True(t, ranB)
}

func TestRunSchedulesAndStopsOnCancel(t *testing.T) {
	s, _ := newScheduler(t, "jobs_loop")
	var n int32
	require.NoError(t, s.Add(Job{Name: "berkala", Interval: 50 * time.Millisecond, RunOnStart: true, Run: func(context.Context) error {
		atomic.AddInt32(&n, 1)
		return nil
	}}))
	require.NoError(t, s.Add(Job{Name: "mati", Interval: 0, RunOnStart: true, Run: func(context.Context) error {
		t.Error("job nonaktif tidak boleh jalan")
		return nil
	}}))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Run(ctx); close(done) }()

	require.Eventually(t, func() bool { return atomic.LoadInt32(&n) >= 3 }, 5*time.Second, 20*time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run tidak berhenti setelah ctx dibatalkan")
	}
}
