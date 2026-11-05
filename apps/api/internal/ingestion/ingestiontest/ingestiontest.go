// Package ingestiontest menyediakan Fetcher palsu untuk tes ingestion: respons
// diambil dari fixture di repo sehingga tes tidak pernah memanggil internet.
package ingestiontest

import (
	"context"
	"fmt"
	"sync"

	"github.com/garda-inflasi/api/internal/ingestion/domain"
)

// Fetcher menjawab Permintaan menurut Kunci-nya. Kunci tanpa isian di Respons
// dan Galat menghasilkan galat (tes tidak boleh meminta yang tidak dirancang).
type Fetcher struct {
	mu sync.Mutex
	// Respons: Permintaan.Kunci -> badan respons.
	Respons map[string][]byte
	// Galat: Permintaan.Kunci -> galat yang dikembalikan (didahulukan dari Respons).
	Galat map[string]error
	// Tahan, bila tidak nil, membuat Get menunggu sampai channel ditutup (menguji run tumpang tindih).
	Tahan chan struct{}

	dipanggil []string
}

var _ domain.Fetcher = (*Fetcher)(nil)

func (f *Fetcher) Get(ctx context.Context, p domain.Permintaan) ([]byte, error) {
	if f.Tahan != nil {
		select {
		case <-f.Tahan:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dipanggil = append(f.dipanggil, p.Kunci)
	if err, ok := f.Galat[p.Kunci]; ok {
		return nil, err
	}
	body, ok := f.Respons[p.Kunci]
	if !ok {
		return nil, fmt.Errorf("ingestiontest: tidak ada respons untuk kunci %q", p.Kunci)
	}
	return body, nil
}

// Dipanggil mengembalikan kunci permintaan yang sudah dilayani, berurutan.
func (f *Fetcher) Dipanggil() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.dipanggil...)
}
