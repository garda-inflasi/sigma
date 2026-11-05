package domain

import (
	"context"
	"time"
)

// Peristiwa adalah satu entri aliran real-time: alert yang baru dibuat atau naik
// tingkat. ID berurut dan dipakai sebagai Last-Event-ID.
type Peristiwa struct {
	ID    int64
	Alert Alert
}

// EventRepository adalah akses data aliran peristiwa EWS.
type EventRepository interface {
	// LatestEventID mengembalikan id peristiwa terakhir (0 bila belum ada).
	LatestEventID(ctx context.Context) (int64, error)
	// EventsAfter mengembalikan paling banyak limit peristiwa dengan id > after, berurut naik,
	// beserta keadaan alert terkini.
	EventsAfter(ctx context.Context, after int64, limit int) ([]Peristiwa, error)
	// PurgeEventsBefore membuang peristiwa yang dibuat sebelum t dan mengembalikan jumlahnya.
	PurgeEventsBefore(ctx context.Context, t time.Time) (int64, error)
}
