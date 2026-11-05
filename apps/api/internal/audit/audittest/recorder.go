// Package audittest menyediakan Writer palsu untuk tes modul lain.
package audittest

import (
	"context"
	"sync"

	"github.com/garda-inflasi/api/internal/audit"
)

// Recorder menampung entri yang ditulis; Err (bila diisi) dikembalikan oleh Write.
type Recorder struct {
	mu      sync.Mutex
	entries []audit.Entry
	Err     error
}

var _ audit.Writer = (*Recorder)(nil)

func (r *Recorder) Write(_ context.Context, entry audit.Entry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, entry)
	return r.Err
}

// Entries mengembalikan salinan entri yang tercatat, berurutan.
func (r *Recorder) Entries() []audit.Entry {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]audit.Entry(nil), r.entries...)
}

// Last mengembalikan entri terakhir; ok false bila belum ada.
func (r *Recorder) Last() (entry audit.Entry, ok bool) {
	entries := r.Entries()
	if len(entries) == 0 {
		return audit.Entry{}, false
	}
	return entries[len(entries)-1], true
}
