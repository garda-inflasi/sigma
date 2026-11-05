package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/garda-inflasi/api/internal/ingestion/domain"
)

// ErrRobots: robots.txt host melarang jalur yang diminta.
var ErrRobots = errors.New("dilarang robots.txt")

// GalatHTTP adalah respons non-2xx. Sementara benar untuk 429 dan 5xx (layak diulang).
type GalatHTTP struct {
	Status    int
	Lokasi    string
	Sementara bool
}

func (e *GalatHTTP) Error() string { return fmt.Sprintf("GET %s: status %d", e.Lokasi, e.Status) }

// redaksi menampilkan URL tanpa query/userinfo, karena kunci API biasanya ada di query.
func redaksi(u *url.URL) string { return u.Scheme + "://" + u.Host + u.Path }

// lokasiAman menampilkan URL tanpa query dan dengan potongan rahasia (mis. kunci API di jalur) disamarkan.
func lokasiAman(u *url.URL, samarkan []string) string {
	lokasi := redaksi(u)
	for _, rahasia := range samarkan {
		if rahasia != "" {
			lokasi = strings.ReplaceAll(lokasi, rahasia, "***")
		}
	}
	return lokasi
}

// HTTPFetcher mengambil URL lewat HTTP GET dengan pembatasan laju per host dan
// (bila diminta Permintaan.PatuhRobots) kepatuhan robots.txt. Retry dan circuit
// breaker ada di dekorator terpisah (Retry, Breaker).
type HTTPFetcher struct {
	Client    *http.Client
	UserAgent string
	// JedaMin adalah jarak minimum antar permintaan ke host yang sama.
	JedaMin time.Duration
	// BatasBadan membatasi ukuran respons (byte).
	BatasBadan int64

	mu       sync.Mutex
	terakhir map[string]time.Time
	robots   *robotsCache
}

// NewHTTPFetcher membuat HTTPFetcher dengan Client ber-timeout.
func NewHTTPFetcher(userAgent string, jedaMin, timeout time.Duration) *HTTPFetcher {
	return &HTTPFetcher{
		Client:     &http.Client{Timeout: timeout},
		UserAgent:  userAgent,
		JedaMin:    jedaMin,
		BatasBadan: 16 << 20,
		terakhir:   map[string]time.Time{},
		robots:     newRobotsCache(userAgent, 24*time.Hour),
	}
}

// Get mengambil badan respons 2xx.
func (f *HTTPFetcher) Get(ctx context.Context, p domain.Permintaan) ([]byte, error) {
	u, err := url.Parse(p.URL)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("url permintaan %q tidak valid", p.Kunci)
	}
	if p.PatuhRobots {
		boleh, err := f.robots.boleh(ctx, f.ambilTanpaRobots, u)
		if err != nil {
			return nil, fmt.Errorf("robots.txt %s: %w", u.Host, err)
		}
		if !boleh {
			return nil, fmt.Errorf("%s: %w", lokasiAman(u, p.Samarkan), ErrRobots)
		}
	}
	return f.ambil(ctx, u, p.Header, p.Samarkan)
}

// ambilTanpaRobots mengambil URL tanpa pemeriksaan robots (dipakai untuk robots.txt sendiri).
func (f *HTTPFetcher) ambilTanpaRobots(ctx context.Context, u *url.URL) ([]byte, error) {
	return f.ambil(ctx, u, nil, nil)
}

func (f *HTTPFetcher) ambil(ctx context.Context, u *url.URL, header http.Header, samarkan []string) ([]byte, error) {
	lokasi := lokasiAman(u, samarkan)
	if err := f.tunggu(ctx, u.Host); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", lokasi, err)
	}
	req.Header.Set("User-Agent", f.UserAgent)
	req.Header.Set("Accept", "application/json")
	for k, vs := range header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}

	res, err := f.Client.Do(req)
	if err != nil {
		// url.Error memuat URL lengkap (termasuk kunci API); ganti dengan versi tersunting.
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return nil, fmt.Errorf("GET %s: %w", lokasi, err)
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, &GalatHTTP{Status: res.StatusCode, Lokasi: lokasi, Sementara: res.StatusCode == http.StatusTooManyRequests || res.StatusCode >= 500}
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, f.BatasBadan+1))
	if err != nil {
		return nil, fmt.Errorf("GET %s: baca badan: %w", lokasi, err)
	}
	if int64(len(body)) > f.BatasBadan {
		return nil, fmt.Errorf("GET %s: respons melebihi %d byte", lokasi, f.BatasBadan)
	}
	return body, nil
}

// tunggu menjaga JedaMin antar permintaan ke host yang sama.
func (f *HTTPFetcher) tunggu(ctx context.Context, host string) error {
	if f.JedaMin <= 0 {
		return nil
	}
	f.mu.Lock()
	giliran := time.Now()
	if t, ok := f.terakhir[host]; ok && t.Add(f.JedaMin).After(giliran) {
		giliran = t.Add(f.JedaMin)
	}
	f.terakhir[host] = giliran
	f.mu.Unlock()

	return tidur(ctx, time.Until(giliran))
}

func tidur(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// Sementara menilai apakah galat layak diulang: 429/5xx, kegagalan jaringan, atau
// timeout. Pembatalan konteks dan larangan robots tidak.
func Sementara(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, ErrRobots) {
		return false
	}
	var gh *GalatHTTP
	if errors.As(err, &gh) {
		return gh.Sementara
	}
	var ne net.Error
	return errors.As(err, &ne) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, context.DeadlineExceeded)
}

// Retry mengulang permintaan yang gagal sementara dengan backoff eksponensial.
type Retry struct {
	Next      domain.Fetcher
	Percobaan int           // total percobaan termasuk yang pertama
	Awal      time.Duration // jeda sebelum percobaan kedua; berlipat dua tiap percobaan
	// Tidur bisa diganti pada tes; nil memakai timer sungguhan.
	Tidur func(ctx context.Context, d time.Duration) error
}

func (r *Retry) Get(ctx context.Context, p domain.Permintaan) ([]byte, error) {
	tidurFn := r.Tidur
	if tidurFn == nil {
		tidurFn = tidur
	}
	jeda := r.Awal
	var err error
	for i := 1; ; i++ {
		var body []byte
		if body, err = r.Next.Get(ctx, p); err == nil {
			return body, nil
		}
		if i >= r.Percobaan || !Sementara(err) {
			return nil, err
		}
		if serr := tidurFn(ctx, jeda); serr != nil {
			return nil, serr
		}
		jeda *= 2
	}
}
