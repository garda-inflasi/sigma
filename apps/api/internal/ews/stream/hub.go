// Package stream menyajikan aliran real-time EWS (Server-Sent Events). Sumber
// kebenaran adalah tabel ews_event; Bus hanya membangunkan pembaca (Redis pub/sub
// antar-instance bila tersedia), dan polling basis data menjadi jaring pengaman.
package stream

import (
	"context"
	"sync"

	"github.com/garda-inflasi/api/internal/ews/domain"
)

// Hub meneruskan sinyal "ada peristiwa baru" ke semua koneksi SSE di proses ini.
type Hub struct {
	mu   sync.Mutex
	subs map[chan struct{}]struct{}
}

func NewHub() *Hub {
	return &Hub{subs: map[chan struct{}]struct{}{}}
}

// Subscribe mendaftarkan pelanggan. Sinyal yang menumpuk digabung menjadi satu.
// Panggil cancel saat koneksi selesai.
func (h *Hub) Subscribe() (wake <-chan struct{}, cancel func()) {
	ch := make(chan struct{}, 1)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.subs, ch)
		h.mu.Unlock()
	}
}

// Wake membangunkan semua pelanggan tanpa menunggu.
func (h *Hub) Wake() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- struct{}{}:
		default: // sudah ada sinyal tertunda
		}
	}
}

// Subscribers mengembalikan jumlah pelanggan aktif.
func (h *Hub) Subscribers() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs)
}

// Bus menyebarkan sinyal peristiwa baru ke seluruh instance API.
type Bus interface {
	// Notify memberi tahu semua instance (termasuk proses ini) bahwa ada peristiwa baru.
	Notify(ctx context.Context)
	// Run menerima sinyal dari instance lain sampai ctx selesai.
	Run(ctx context.Context)
}

// LocalBus membangunkan hub di proses ini saja (tanpa Redis); instance lain
// mengandalkan polling.
type LocalBus struct{ hub *Hub }

func NewLocalBus(hub *Hub) *LocalBus { return &LocalBus{hub: hub} }

func (b *LocalBus) Notify(context.Context) { b.hub.Wake() }

func (b *LocalBus) Run(ctx context.Context) { <-ctx.Done() }

// Publisher adalah domain.Observer yang membangunkan pembaca setiap alert dibuat atau naik tingkat.
type Publisher struct{ bus Bus }

var _ domain.Observer = (*Publisher)(nil)

func NewPublisher(bus Bus) *Publisher { return &Publisher{bus: bus} }

func (p *Publisher) AlertDitulis(ctx context.Context, _ domain.Alert, _ domain.HasilTulis) {
	p.bus.Notify(ctx)
}
