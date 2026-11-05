package engine

import (
	"bufio"
	"context"
	"errors"
	"net/url"
	"strings"
	"sync"
	"time"
)

// aturanRobots adalah aturan satu grup user-agent. Pencocokan memakai awalan jalur
// (tanpa wildcard); aturan terpanjang menang dan Allow menang bila seri.
type aturanRobots struct {
	izin, larang []string
}

func (a aturanRobots) boleh(path string) bool {
	if path == "" {
		path = "/"
	}
	panjangIzin, panjangLarang := -1, -1
	for _, p := range a.izin {
		if strings.HasPrefix(path, p) && len(p) > panjangIzin {
			panjangIzin = len(p)
		}
	}
	for _, p := range a.larang {
		if p != "" && strings.HasPrefix(path, p) && len(p) > panjangLarang {
			panjangLarang = len(p)
		}
	}
	return panjangLarang < 0 || panjangIzin >= panjangLarang
}

// parseRobots membaca robots.txt dan memilih grup untuk agen: grup yang menyebut
// token agen kita bila ada, selain itu grup "*".
func parseRobots(isi, agen string) aturanRobots {
	grup := map[string]*aturanRobots{}
	var aktif []*aturanRobots
	baruMulai := true
	sc := bufio.NewScanner(strings.NewReader(isi))
	for sc.Scan() {
		baris, _, _ := strings.Cut(sc.Text(), "#")
		kunci, nilai, ada := strings.Cut(baris, ":")
		if !ada {
			continue
		}
		kunci, nilai = strings.ToLower(strings.TrimSpace(kunci)), strings.TrimSpace(nilai)
		switch kunci {
		case "user-agent":
			if baruMulai {
				aktif = nil
			}
			baruMulai = false
			nama := strings.ToLower(nilai)
			if grup[nama] == nil {
				grup[nama] = &aturanRobots{}
			}
			aktif = append(aktif, grup[nama])
		case "allow":
			baruMulai = true
			for _, g := range aktif {
				g.izin = append(g.izin, nilai)
			}
		case "disallow":
			baruMulai = true
			for _, g := range aktif {
				g.larang = append(g.larang, nilai)
			}
		}
	}
	token := strings.ToLower(strings.SplitN(agen, "/", 2)[0])
	if g, ok := grup[token]; ok {
		return *g
	}
	if g, ok := grup["*"]; ok {
		return *g
	}
	return aturanRobots{}
}

type catatanRobots struct {
	aturan      aturanRobots
	kedaluwarsa time.Time
}

// robotsCache menyimpan robots.txt per host selama ttl.
type robotsCache struct {
	agen string
	ttl  time.Duration

	mu    sync.Mutex
	hosts map[string]catatanRobots
}

func newRobotsCache(agen string, ttl time.Duration) *robotsCache {
	return &robotsCache{agen: agen, ttl: ttl, hosts: map[string]catatanRobots{}}
}

// boleh memeriksa apakah u boleh diambil. robots.txt yang tidak ada (4xx) berarti
// tanpa pembatasan; kegagalan lain (jaringan, 5xx) dikembalikan sebagai galat
// agar pengambilan tidak berjalan tanpa kepastian.
func (c *robotsCache) boleh(ctx context.Context, ambil func(context.Context, *url.URL) ([]byte, error), u *url.URL) (bool, error) {
	kunci := u.Scheme + "://" + u.Host
	c.mu.Lock()
	catatan, ada := c.hosts[kunci]
	c.mu.Unlock()

	if !ada || time.Now().After(catatan.kedaluwarsa) {
		robotsURL := &url.URL{Scheme: u.Scheme, Host: u.Host, Path: "/robots.txt"}
		isi, err := ambil(ctx, robotsURL)
		var gh *GalatHTTP
		switch {
		case err == nil:
			catatan.aturan = parseRobots(string(isi), c.agen)
		case errors.As(err, &gh) && gh.Status >= 400 && gh.Status < 500:
			catatan.aturan = aturanRobots{}
		default:
			return false, err
		}
		catatan.kedaluwarsa = time.Now().Add(c.ttl)
		c.mu.Lock()
		c.hosts[kunci] = catatan
		c.mu.Unlock()
	}
	return catatan.aturan.boleh(u.Path), nil
}
