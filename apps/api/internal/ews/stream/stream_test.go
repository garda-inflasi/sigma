package stream

import (
	"bufio"
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/garda-inflasi/api/internal/ews/domain"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeStore adalah EventRepository dalam memori.
type fakeStore struct {
	mu     sync.Mutex
	events []domain.Peristiwa
	err    error
}

// add menambah peristiwa id untuk alert ber-ID 100+id dan kode tampilan alertKode.
func (f *fakeStore) add(id int64, alertKode, severity string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, domain.Peristiwa{ID: id, Alert: domain.Alert{EwsAlert: domain.EwsAlert{ID: idx.ID(100 + id), Kode: &alertKode, Severity: severity, Status: "BARU"}, KabupatenNama: "KOTA BANDA ACEH"}})
}

func (f *fakeStore) LatestEventID(context.Context) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.events) == 0 {
		return 0, f.err
	}
	return f.events[len(f.events)-1].ID, f.err
}

func (f *fakeStore) EventsAfter(_ context.Context, after int64, limit int) ([]domain.Peristiwa, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	var out []domain.Peristiwa
	for _, e := range f.events {
		if e.ID > after && len(out) < limit {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *fakeStore) PurgeEventsBefore(context.Context, time.Time) (int64, error) { return 0, nil }

// lines menjalankan Serve ke pipe dan mengirim tiap baris yang ditulis ke channel.
func serve(t *testing.T, s *Streamer, after int64) (lines <-chan string, stop func() error) {
	t.Helper()
	pr, pw := io.Pipe()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		err := s.Serve(ctx, pw, func() {}, after)
		_ = pw.Close()
		done <- err
	}()
	out := make(chan string, 256)
	go func() {
		sc := bufio.NewScanner(pr)
		for sc.Scan() {
			out <- sc.Text()
		}
		close(out)
	}()
	return out, func() error { cancel(); return <-done }
}

func next(t *testing.T, lines <-chan string) string {
	t.Helper()
	select {
	case l, ok := <-lines:
		require.True(t, ok, "aliran berakhir")
		return l
	case <-time.After(3 * time.Second):
		t.Fatal("tidak ada baris dalam 3 detik")
		return ""
	}
}

// frame membaca satu frame (sampai baris kosong) dan mengembalikan field-nya.
func frame(t *testing.T, lines <-chan string) map[string]string {
	t.Helper()
	f := map[string]string{}
	for {
		l := next(t, lines)
		if l == "" {
			return f
		}
		k, v, _ := strings.Cut(l, ": ")
		f[k] = v
	}
}

func newStreamer(store *fakeStore) (*Streamer, *Hub) {
	hub := NewHub()
	s := NewStreamer(store, hub, time.Hour)
	return s, hub
}

func TestServe_SendsRetryThenOnlyEventsAfterCursor(t *testing.T) {
	store := &fakeStore{}
	store.add(1, "EWS-A", "WASPADA")
	store.add(2, "EWS-B", "KRITIS")
	store.add(3, "EWS-C", "SIAGA")
	s, _ := newStreamer(store)

	lines, stop := serve(t, s, 1)
	assert.Equal(t, map[string]string{"retry": "5000"}, frame(t, lines))
	f2, f3 := frame(t, lines), frame(t, lines)

	assert.Equal(t, "2", f2["id"])
	assert.Equal(t, "alert", f2["event"])
	assert.Contains(t, f2["data"], `"id":"102"`, "ID alert di payload adalah string desimal")
	assert.Contains(t, f2["data"], `"kode":"EWS-B"`)
	assert.Contains(t, f2["data"], `"wilayah":"KOTA BANDA ACEH"`)
	assert.Equal(t, "3", f3["id"])
	require.NoError(t, stop())
}

func TestServe_DeliversNewEventsOnWakeWithoutDuplicates(t *testing.T) {
	store := &fakeStore{}
	s, hub := newStreamer(store)
	lines, stop := serve(t, s, 0)
	frame(t, lines) // retry

	store.add(1, "EWS-A", "KRITIS")
	hub.Wake()
	assert.Equal(t, "1", frame(t, lines)["id"])

	hub.Wake() // tanpa peristiwa baru: tidak ada frame
	store.add(2, "EWS-B", "SIAGA")
	hub.Wake()
	assert.Equal(t, "2", frame(t, lines)["id"], "tidak mengulang peristiwa 1")
	require.NoError(t, stop())
}

func TestServe_PollingCatchesEventsWithoutWake(t *testing.T) {
	store := &fakeStore{}
	hub := NewHub()
	s := NewStreamer(store, hub, 20*time.Millisecond)
	lines, stop := serve(t, s, 0)
	frame(t, lines)

	store.add(1, "EWS-A", "KRITIS")

	assert.Equal(t, "1", frame(t, lines)["id"], "instance tanpa sinyal Redis tetap menerima lewat polling")
	require.NoError(t, stop())
}

func TestServe_HeartbeatKeepsConnectionAlive(t *testing.T) {
	s, _ := newStreamer(&fakeStore{})
	s.Heartbeat = 20 * time.Millisecond
	lines, stop := serve(t, s, 0)
	frame(t, lines)

	assert.Equal(t, ": ping", next(t, lines))
	require.NoError(t, stop())
}

func TestServe_PagesThroughMoreThanOneBatch(t *testing.T) {
	store := &fakeStore{}
	for i := int64(1); i <= batchSize+5; i++ {
		store.add(i, "EWS", "INFO")
	}
	s, _ := newStreamer(store)
	lines, stop := serve(t, s, 0)
	frame(t, lines)

	for i := int64(1); i <= batchSize+5; i++ {
		assert.Equal(t, i, mustInt(t, frame(t, lines)["id"]))
	}
	require.NoError(t, stop())
}

func mustInt(t *testing.T, s string) int64 {
	t.Helper()
	id, ok := ParseLastEventID(s)
	require.True(t, ok, s)
	return id
}

func TestServe_StoreErrorEndsStreamSoClientReconnectsWithLastID(t *testing.T) {
	store := &fakeStore{err: assert.AnError}
	s, _ := newStreamer(store)

	err := s.Serve(t.Context(), io.Discard, func() {}, 0)

	assert.ErrorIs(t, err, assert.AnError)
}

func TestServe_WriteErrorEndsStream(t *testing.T) {
	store := &fakeStore{}
	store.add(1, "EWS-A", "INFO")
	s, _ := newStreamer(store)
	pr, pw := io.Pipe()
	require.NoError(t, pr.Close())

	err := s.Serve(t.Context(), pw, func() {}, 0)

	assert.Error(t, err)
}

func TestServe_UnsubscribesWhenDone(t *testing.T) {
	s, hub := newStreamer(&fakeStore{})
	lines, stop := serve(t, s, 0)
	frame(t, lines)
	require.Equal(t, 1, hub.Subscribers())

	require.NoError(t, stop())

	assert.Zero(t, hub.Subscribers())
}

func TestStartID(t *testing.T) {
	store := &fakeStore{}
	store.add(7, "EWS-A", "INFO")
	s, _ := newStreamer(store)

	cases := map[string]int64{"": 7, "abc": 7, "-3": 7, "2": 2, " 5 ": 5, "0": 0}
	for header, want := range cases {
		got, err := s.StartID(t.Context(), header)
		require.NoError(t, err)
		assert.Equal(t, want, got, "Last-Event-ID %q", header)
	}
}

func TestHub_WakeCoalescesAndRespectsCancel(t *testing.T) {
	hub := NewHub()
	a, cancelA := hub.Subscribe()
	b, cancelB := hub.Subscribe()
	defer cancelB()

	hub.Wake()
	hub.Wake() // digabung: pelanggan tidak terblokir
	<-a
	select {
	case <-a:
		t.Fatal("sinyal ganda harus digabung")
	default:
	}

	cancelA()
	hub.Wake()
	select {
	case <-a:
		t.Fatal("pelanggan yang dibatalkan tak boleh dibangunkan")
	default:
	}
	<-b
}

func TestRedisBus_SignalReachesHubsOfEveryInstance(t *testing.T) {
	mr := miniredis.RunT(t)
	log := logger.NewDefault()
	hubA, hubB := NewHub(), NewHub()
	busA, err := NewRedisBus(t.Context(), "redis://"+mr.Addr(), hubA, log)
	require.NoError(t, err)
	busB, err := NewRedisBus(t.Context(), "redis://"+mr.Addr(), hubB, log)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	var wg sync.WaitGroup
	for _, b := range []*RedisBus{busA, busB} {
		wg.Add(1)
		go func() { defer wg.Done(); b.Run(ctx) }()
	}
	t.Cleanup(func() { cancel(); wg.Wait() })

	wakeA, cancelSubA := hubA.Subscribe()
	defer cancelSubA()
	wakeB, cancelSubB := hubB.Subscribe()
	defer cancelSubB()

	// Berlangganan Redis berjalan asinkron: kirim ulang sampai kedua instance bangun.
	require.Eventually(t, func() bool {
		busA.Notify(t.Context())
		select {
		case <-wakeB:
			return true
		case <-time.After(50 * time.Millisecond):
			return false
		}
	}, 3*time.Second, 10*time.Millisecond, "instance B menerima sinyal dari instance A")
	select {
	case <-wakeA:
	case <-time.After(time.Second):
		t.Fatal("instance pengirim juga menerima sinyalnya sendiri lewat Redis")
	}
}

func TestRedisBus_NotifyFallsBackToLocalWakeWhenRedisIsDown(t *testing.T) {
	mr := miniredis.RunT(t)
	hub := NewHub()
	bus, err := NewRedisBus(t.Context(), "redis://"+mr.Addr(), hub, logger.NewDefault())
	require.NoError(t, err)
	wake, cancel := hub.Subscribe()
	defer cancel()

	mr.Close()
	bus.Notify(t.Context())

	select {
	case <-wake:
	case <-time.After(2 * time.Second):
		t.Fatal("proses sendiri tetap dibangunkan walau Redis mati")
	}
}

func TestNewRedisBus_Errors(t *testing.T) {
	_, err := NewRedisBus(t.Context(), "bukan-url", NewHub(), logger.NewDefault())
	assert.ErrorContains(t, err, "REDIS_URL")

	mr := miniredis.RunT(t)
	addr := mr.Addr()
	mr.Close()
	_, err = NewRedisBus(t.Context(), "redis://"+addr, NewHub(), logger.NewDefault())
	assert.ErrorContains(t, err, "redis tidak menjawab")
}

func TestPublisher_NotifiesBusOnAlertWritten(t *testing.T) {
	hub := NewHub()
	wake, cancel := hub.Subscribe()
	defer cancel()

	NewPublisher(NewLocalBus(hub)).AlertDitulis(t.Context(), domain.Alert{}, domain.HasilDibuat)

	select {
	case <-wake:
	default:
		t.Fatal("publisher harus membangunkan pelanggan")
	}
}
