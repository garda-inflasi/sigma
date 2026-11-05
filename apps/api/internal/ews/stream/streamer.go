package stream

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/garda-inflasi/api/internal/ews/domain"
	"github.com/garda-inflasi/api/internal/ews/dto"
)

// Nilai bawaan Streamer.
const (
	DefaultHeartbeat = 25 * time.Second
	batchSize        = 100
	// RetryMs adalah jeda sambung ulang yang disarankan ke EventSource.
	RetryMs = 5000
)

// Streamer menulis peristiwa EWS sebagai frame SSE ke satu koneksi.
type Streamer struct {
	store domain.EventRepository
	hub   *Hub
	// Poll adalah jeda membaca basis data walau tak ada sinyal (jaring pengaman
	// bila Redis tidak ada atau terputus).
	Poll time.Duration
	// Heartbeat menjaga koneksi tetap hidup melewati proxy.
	Heartbeat time.Duration
}

func NewStreamer(store domain.EventRepository, hub *Hub, poll time.Duration) *Streamer {
	return &Streamer{store: store, hub: hub, Poll: poll, Heartbeat: DefaultHeartbeat}
}

// ParseLastEventID membaca header Last-Event-ID; ok false bila kosong atau bukan bilangan bulat tak-negatif.
func ParseLastEventID(raw string) (id int64, ok bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	return id, err == nil && id >= 0
}

// StartID menentukan peristiwa pertama yang dikirim: klien yang menyambung ulang
// melanjutkan setelah lastEventID; klien baru hanya menerima peristiwa setelah sekarang.
func (s *Streamer) StartID(ctx context.Context, lastEventID string) (int64, error) {
	if id, ok := ParseLastEventID(lastEventID); ok {
		return id, nil
	}
	return s.store.LatestEventID(ctx)
}

// Serve mengirim peristiwa dengan id > after ke w sampai ctx selesai (kembali nil)
// atau penulisan/pembacaan gagal. flush dipanggil setelah tiap kelompok frame.
func (s *Streamer) Serve(ctx context.Context, w io.Writer, flush func(), after int64) error {
	wake, cancel := s.hub.Subscribe()
	defer cancel()

	if _, err := fmt.Fprintf(w, "retry: %d\n\n", RetryMs); err != nil {
		return err
	}
	flush()

	poll, heartbeat := time.NewTicker(s.Poll), time.NewTicker(s.Heartbeat)
	defer poll.Stop()
	defer heartbeat.Stop()
	for {
		if err := s.drain(ctx, w, flush, &after); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-wake:
		case <-poll.C:
		case <-heartbeat.C:
			if _, err := io.WriteString(w, ": ping\n\n"); err != nil {
				return err
			}
			flush()
		}
	}
}

// drain menulis semua peristiwa tertunda; *after maju hanya setelah frame tertulis.
func (s *Streamer) drain(ctx context.Context, w io.Writer, flush func(), after *int64) error {
	for {
		events, err := s.store.EventsAfter(ctx, *after, batchSize)
		if err != nil {
			return err
		}
		for _, ev := range events {
			payload, err := json.Marshal(dto.ToAlertResponse(ev.Alert))
			if err != nil {
				return fmt.Errorf("encode alert: %w", err)
			}
			if _, err := fmt.Fprintf(w, "id: %d\nevent: alert\ndata: %s\n\n", ev.ID, payload); err != nil {
				return err
			}
			*after = ev.ID
		}
		if len(events) > 0 {
			flush()
		}
		if len(events) < batchSize {
			return nil
		}
	}
}
