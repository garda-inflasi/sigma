package engine

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/garda-inflasi/api/internal/ingestion/domain"
)

// ErrSirkitTerbuka: terlalu banyak kegagalan beruntun pada sumber ini, permintaan
// ditolak tanpa menghubungi layanan sampai masa tunggu habis.
var ErrSirkitTerbuka = errors.New("circuit breaker terbuka: sumber gagal berulang")

// Breaker memutus permintaan ke sebuah sumber setelah Ambang kegagalan beruntun
// selama Tunggu. Sesudah itu satu percobaan diizinkan; gagal lagi, sirkit terbuka kembali.
type Breaker struct {
	Next   domain.Fetcher
	Ambang int
	Tunggu time.Duration
	// Sekarang bisa diganti pada tes.
	Sekarang func() time.Time

	mu     sync.Mutex
	sumber map[string]*kondisiSirkit
}

type kondisiSirkit struct {
	gagal         int
	terbukaSampai time.Time
}

func (b *Breaker) sekarang() time.Time {
	if b.Sekarang != nil {
		return b.Sekarang()
	}
	return time.Now()
}

func (b *Breaker) Get(ctx context.Context, p domain.Permintaan) ([]byte, error) {
	b.mu.Lock()
	if b.sumber == nil {
		b.sumber = map[string]*kondisiSirkit{}
	}
	k := b.sumber[p.SumberID]
	if k == nil {
		k = &kondisiSirkit{}
		b.sumber[p.SumberID] = k
	}
	if k.gagal >= b.Ambang {
		if b.sekarang().Before(k.terbukaSampai) {
			b.mu.Unlock()
			return nil, ErrSirkitTerbuka
		}
		// Setengah terbuka: satu percobaan; gagal sekali lagi langsung membuka sirkit.
		k.gagal = b.Ambang - 1
	}
	b.mu.Unlock()

	body, err := b.Next.Get(ctx, p)

	b.mu.Lock()
	defer b.mu.Unlock()
	switch {
	case err == nil:
		k.gagal = 0
	case errors.Is(err, context.Canceled), errors.Is(err, ErrRobots):
		// Bukan kegagalan sumber.
	default:
		k.gagal++
		if k.gagal >= b.Ambang {
			k.terbukaSampai = b.sekarang().Add(b.Tunggu)
		}
	}
	return body, err
}
