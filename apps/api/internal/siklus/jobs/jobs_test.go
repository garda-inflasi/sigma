package jobs

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	ewsdomain "github.com/garda-inflasi/api/internal/ews/domain"
	"github.com/garda-inflasi/api/internal/siklus/domain"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func env(kv map[string]string) func(string) string { return func(k string) string { return kv[k] } }

func TestIntervalFromEnv(t *testing.T) {
	d, err := IntervalFromEnv(env(nil))
	require.NoError(t, err)
	assert.Equal(t, DefaultInterval, d, "bawaan 5 menit")

	d, err = IntervalFromEnv(env(map[string]string{"JOB_SIKLUS_EVALUASI_INTERVAL": "30s"}))
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, d)

	for _, off := range []string{"off", "OFF", "0", "false", "disabled"} {
		d, err = IntervalFromEnv(env(map[string]string{"JOB_SIKLUS_EVALUASI_INTERVAL": off}))
		require.NoError(t, err, off)
		assert.Zero(t, d, off)
	}

	for _, buruk := range []string{"sering", "10ms", "-5m"} {
		_, err = IntervalFromEnv(env(map[string]string{"JOB_SIKLUS_EVALUASI_INTERVAL": buruk}))
		assert.Error(t, err, buruk)
	}
}

// fakeEvaluator menghitung evaluasi dan, bila gerbang diisi, menahan evaluasi sampai gerbang dibuka.
type fakeEvaluator struct {
	mu      sync.Mutex
	opsi    []domain.OpsiEvaluasi
	jumlah  atomic.Int32
	gerbang chan struct{}
	err     error
}

func (f *fakeEvaluator) Evaluasi(ctx context.Context, wilayah string, opsi domain.OpsiEvaluasi) (domain.HasilEvaluasi, error) {
	f.mu.Lock()
	f.opsi = append(f.opsi, opsi)
	f.mu.Unlock()
	f.jumlah.Add(1)
	if f.gerbang != nil {
		select {
		case <-f.gerbang:
		case <-ctx.Done():
		}
	}
	return domain.HasilEvaluasi{WilayahKode: wilayah}, f.err
}

func baruPemicu(ev domain.Evaluator, jeda time.Duration) *Pemicu {
	return NewPemicu(ev, domain.WilayahBawaan, jeda, logger.New(logger.ERROR, nil))
}

func jalankan(t *testing.T, p *Pemicu) context.CancelFunc {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	selesai := make(chan struct{})
	go func() { p.Run(ctx); close(selesai) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-selesai:
		case <-time.After(5 * time.Second):
			t.Error("Pemicu.Run tidak berhenti setelah ctx dibatalkan")
		}
	})
	return cancel
}

var alertContoh = ewsdomain.Alert{}

// Tanpa sinyal tidak ada evaluasi; rentetan alert dalam satu putaran EWS digabung menjadi satu evaluasi bertanda alert.
func TestPemicu_RentetanAlertDigabungMenjadiSatuEvaluasi(t *testing.T) {
	ev := &fakeEvaluator{}
	p := baruPemicu(ev, 100*time.Millisecond)
	jalankan(t, p)

	time.Sleep(150 * time.Millisecond)
	assert.Zero(t, ev.jumlah.Load(), "tanpa alert tidak ada evaluasi")

	for range 20 {
		p.AlertDitulis(t.Context(), alertContoh, ewsdomain.HasilDibuat)
	}
	require.Eventually(t, func() bool { return ev.jumlah.Load() >= 1 }, 5*time.Second, 5*time.Millisecond)
	time.Sleep(300 * time.Millisecond)

	assert.EqualValues(t, 1, ev.jumlah.Load(), "dua puluh sinyal beruntun cukup dilayani satu evaluasi")
	assert.Equal(t, []domain.OpsiEvaluasi{{Pemicu: domain.PemicuAlert}}, ev.opsi)
}

// Sinyal yang tiba selagi evaluasi berjalan tidak hilang: evaluasi berikutnya menyusul.
func TestPemicu_SinyalSelamaEvaluasiMemicuEvaluasiBerikutnya(t *testing.T) {
	ev := &fakeEvaluator{gerbang: make(chan struct{})}
	p := baruPemicu(ev, 10*time.Millisecond)
	jalankan(t, p)

	p.AlertDitulis(t.Context(), alertContoh, ewsdomain.HasilDibuat)
	require.Eventually(t, func() bool { return ev.jumlah.Load() == 1 }, 5*time.Second, 5*time.Millisecond)
	p.AlertDitulis(t.Context(), alertContoh, ewsdomain.HasilDinaikkan) // selagi evaluasi pertama tertahan
	close(ev.gerbang)

	require.Eventually(t, func() bool { return ev.jumlah.Load() == 2 }, 5*time.Second, 5*time.Millisecond)
	time.Sleep(100 * time.Millisecond)
	assert.EqualValues(t, 2, ev.jumlah.Load())
}

// AlertDitulis dipanggil dari dalam rule engine EWS: tidak boleh memblokir walau tidak ada yang melayani sinyal.
func TestPemicu_AlertDitulisTidakPernahMemblokir(t *testing.T) {
	p := baruPemicu(&fakeEvaluator{}, time.Millisecond) // Run tidak dijalankan
	selesai := make(chan struct{})
	go func() {
		for range 1000 {
			p.AlertDitulis(t.Context(), alertContoh, ewsdomain.HasilDibuat)
		}
		close(selesai)
	}()
	select {
	case <-selesai:
	case <-time.After(5 * time.Second):
		t.Fatal("AlertDitulis memblokir")
	}
}

// Galat evaluasi hanya dilog; pemicu tetap melayani alert berikutnya.
func TestPemicu_GalatEvaluasiTidakMenghentikanPemicu(t *testing.T) {
	ev := &fakeEvaluator{err: errors.New("db mati")}
	p := baruPemicu(ev, time.Millisecond)
	jalankan(t, p)

	p.AlertDitulis(t.Context(), alertContoh, ewsdomain.HasilDibuat)
	require.Eventually(t, func() bool { return ev.jumlah.Load() == 1 }, 5*time.Second, 5*time.Millisecond)
	p.AlertDitulis(t.Context(), alertContoh, ewsdomain.HasilDibuat)
	require.Eventually(t, func() bool { return ev.jumlah.Load() == 2 }, 5*time.Second, 5*time.Millisecond)
}

// ctx yang dibatalkan menghentikan Run, baik saat menunggu sinyal maupun saat menunggu jeda.
func TestPemicu_BerhentiBilaKonteksDibatalkan(t *testing.T) {
	for nama, kirimSinyal := range map[string]bool{"saat menunggu sinyal": false, "saat menunggu jeda": true} {
		t.Run(nama, func(t *testing.T) {
			ev := &fakeEvaluator{}
			p := baruPemicu(ev, time.Hour)
			ctx, cancel := context.WithCancel(t.Context())
			selesai := make(chan struct{})
			go func() { p.Run(ctx); close(selesai) }()
			if kirimSinyal {
				p.AlertDitulis(ctx, alertContoh, ewsdomain.HasilDibuat)
				time.Sleep(50 * time.Millisecond)
			}

			cancel()

			select {
			case <-selesai:
			case <-time.After(5 * time.Second):
				t.Fatal("Run tidak berhenti")
			}
			assert.Zero(t, ev.jumlah.Load(), "evaluasi yang belum dimulai tidak dijalankan setelah pembatalan")
		})
	}
}
