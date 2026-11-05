package engine

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/ingestion/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fetcherFunc membuat domain.Fetcher dari fungsi.
type fetcherFunc func(ctx context.Context, p domain.Permintaan) ([]byte, error)

func (f fetcherFunc) Get(ctx context.Context, p domain.Permintaan) ([]byte, error) { return f(ctx, p) }

func TestHTTPFetcher_MengambilBadanDanMenyuntingURLPadaGalat(t *testing.T) {
	var ua atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua.Store(r.UserAgent())
		switch r.URL.Path {
		case "/ok":
			_, _ = w.Write([]byte(`{"a":1}`))
		case "/rahasia":
			http.Error(w, "boom", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	f := NewHTTPFetcher("garda-test/1.0", 0, time.Second)
	body, err := f.Get(t.Context(), domain.Permintaan{Kunci: "k", URL: srv.URL + "/ok?key=RAHASIA"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"a":1}`, string(body))
	assert.Equal(t, "garda-test/1.0", ua.Load())

	_, err = f.Get(t.Context(), domain.Permintaan{Kunci: "k", URL: srv.URL + "/rahasia?key=RAHASIA"})
	var gh *GalatHTTP
	require.ErrorAs(t, err, &gh)
	assert.Equal(t, 500, gh.Status)
	assert.True(t, gh.Sementara)
	assert.NotContains(t, err.Error(), "RAHASIA", "kunci API tidak boleh bocor ke pesan galat")

	_, err = f.Get(t.Context(), domain.Permintaan{Kunci: "k", URL: srv.URL + "/tidak-ada"})
	require.ErrorAs(t, err, &gh)
	assert.False(t, gh.Sementara, "404 tidak layak diulang")

	// Kegagalan jaringan juga menyunting URL.
	srv.Close()
	_, err = f.Get(t.Context(), domain.Permintaan{Kunci: "k", URL: srv.URL + "/ok?key=RAHASIA"})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "RAHASIA")
	assert.True(t, Sementara(err))
}

func TestHTTPFetcher_MenyamarkanRahasiaDiJalur(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "x", http.StatusForbidden) }))
	defer srv.Close()
	f := NewHTTPFetcher("garda-test", 0, time.Second)

	_, err := f.Get(t.Context(), domain.Permintaan{Kunci: "k", URL: srv.URL + "/data/key/RAHASIA", Samarkan: []string{"RAHASIA"}})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "/data/key/***")
	assert.NotContains(t, err.Error(), "RAHASIA")
}

func TestHTTPFetcher_MenolakURLTakValidDanBadanBesar(t *testing.T) {
	f := NewHTTPFetcher("garda-test", 0, time.Second)
	_, err := f.Get(t.Context(), domain.Permintaan{Kunci: "k", URL: "bukan url"})
	assert.Error(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(make([]byte, 100)) }))
	defer srv.Close()
	f.BatasBadan = 10
	_, err = f.Get(t.Context(), domain.Permintaan{Kunci: "k", URL: srv.URL})
	assert.ErrorContains(t, err, "melebihi")
}

func TestHTTPFetcher_RobotsTxt(t *testing.T) {
	var diminta atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			_, _ = w.Write([]byte("User-agent: *\nDisallow: /privat\nAllow: /privat/terbuka\n"))
		default:
			diminta.Add(1)
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	f := NewHTTPFetcher("garda-test/1.0", 0, time.Second)
	_, err := f.Get(t.Context(), domain.Permintaan{Kunci: "k", URL: srv.URL + "/publik", PatuhRobots: true})
	require.NoError(t, err)
	_, err = f.Get(t.Context(), domain.Permintaan{Kunci: "k", URL: srv.URL + "/privat/data", PatuhRobots: true})
	assert.ErrorIs(t, err, ErrRobots)
	_, err = f.Get(t.Context(), domain.Permintaan{Kunci: "k", URL: srv.URL + "/privat/terbuka/x", PatuhRobots: true})
	require.NoError(t, err)
	assert.EqualValues(t, 2, diminta.Load(), "permintaan yang dilarang tidak pernah sampai ke layanan")

	// Sumber yang tidak wajib patuh tidak memeriksa robots.txt.
	_, err = f.Get(t.Context(), domain.Permintaan{Kunci: "k", URL: srv.URL + "/privat/data", PatuhRobots: false})
	require.NoError(t, err)
}

func TestHTTPFetcher_RobotsTxtTidakAdaDiperbolehkanTetapiGalatServerTidak(t *testing.T) {
	tanpa := httptest.NewServer(http.NotFoundHandler())
	defer tanpa.Close()
	f := NewHTTPFetcher("garda-test", 0, time.Second)
	_, err := f.Get(t.Context(), domain.Permintaan{Kunci: "k", URL: tanpa.URL + "/x", PatuhRobots: true})
	// robots.txt 404 berarti tanpa pembatasan; permintaan berikutnya (juga 404) gagal karena dirinya sendiri.
	var gh *GalatHTTP
	require.ErrorAs(t, err, &gh)
	assert.Equal(t, 404, gh.Status)
	assert.NotErrorIs(t, err, ErrRobots)

	rusak := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadGateway) }))
	defer rusak.Close()
	_, err = f.Get(t.Context(), domain.Permintaan{Kunci: "k", URL: rusak.URL + "/x", PatuhRobots: true})
	require.Error(t, err)
	assert.ErrorContains(t, err, "robots.txt")
}

func TestHTTPFetcher_JedaMinimumAntarPermintaan(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{}`)) }))
	defer srv.Close()

	f := NewHTTPFetcher("garda-test", 60*time.Millisecond, time.Second)
	mulai := time.Now()
	for range 3 {
		_, err := f.Get(t.Context(), domain.Permintaan{Kunci: "k", URL: srv.URL})
		require.NoError(t, err)
	}
	assert.GreaterOrEqual(t, time.Since(mulai), 110*time.Millisecond, "tiga permintaan ke host sama berjarak >= 60ms")

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := f.Get(ctx, domain.Permintaan{Kunci: "k", URL: srv.URL})
	assert.Error(t, err)
}

func TestParseRobots(t *testing.T) {
	isi := "# komentar\nUser-agent: garda-inflasi\nDisallow: /khusus\n\nUser-agent: googlebot\nUser-agent: *\nDisallow: /\nAllow: /publik\nDisallow:\n"
	khusus := parseRobots(isi, "Garda-Inflasi/1.0 (+kontak)")
	assert.False(t, khusus.boleh("/khusus/x"))
	assert.True(t, khusus.boleh("/lain"))

	umum := parseRobots(isi, "bot-lain/2")
	assert.False(t, umum.boleh("/data"))
	assert.True(t, umum.boleh("/publik/a"), "Allow yang lebih spesifik menang")

	assert.True(t, parseRobots("", "x").boleh("/apa-saja"))
	assert.True(t, parseRobots("User-agent: *\nDisallow:\n", "x").boleh("/apa-saja"), "Disallow kosong berarti semua boleh")
}

func TestRetry_MengulangGalatSementaraDenganBackoff(t *testing.T) {
	var panggilan int
	var jeda []time.Duration
	next := fetcherFunc(func(context.Context, domain.Permintaan) ([]byte, error) {
		panggilan++
		if panggilan < 3 {
			return nil, &GalatHTTP{Status: 503, Sementara: true}
		}
		return []byte("ok"), nil
	})
	r := &Retry{Next: next, Percobaan: 4, Awal: 100 * time.Millisecond, Tidur: func(_ context.Context, d time.Duration) error {
		jeda = append(jeda, d)
		return nil
	}}

	body, err := r.Get(t.Context(), domain.Permintaan{})
	require.NoError(t, err)
	assert.Equal(t, "ok", string(body))
	assert.Equal(t, 3, panggilan)
	assert.Equal(t, []time.Duration{100 * time.Millisecond, 200 * time.Millisecond}, jeda)
}

func TestRetry_BerhentiPadaGalatPermanenAtauBatasPercobaan(t *testing.T) {
	tanpaJeda := func(context.Context, time.Duration) error { return nil }

	var n int
	permanen := fetcherFunc(func(context.Context, domain.Permintaan) ([]byte, error) {
		n++
		return nil, &GalatHTTP{Status: 404}
	})
	_, err := (&Retry{Next: permanen, Percobaan: 5, Awal: time.Millisecond, Tidur: tanpaJeda}).Get(t.Context(), domain.Permintaan{})
	require.Error(t, err)
	assert.Equal(t, 1, n, "404 tidak diulang")

	n = 0
	selalu := fetcherFunc(func(context.Context, domain.Permintaan) ([]byte, error) {
		n++
		return nil, &GalatHTTP{Status: 429, Sementara: true}
	})
	_, err = (&Retry{Next: selalu, Percobaan: 3, Awal: time.Millisecond, Tidur: tanpaJeda}).Get(t.Context(), domain.Permintaan{})
	require.Error(t, err)
	assert.Equal(t, 3, n)

	n = 0
	batal := errors.New("tidur dibatalkan")
	_, err = (&Retry{Next: selalu, Percobaan: 3, Awal: time.Millisecond, Tidur: func(context.Context, time.Duration) error { return batal }}).Get(t.Context(), domain.Permintaan{})
	assert.ErrorIs(t, err, batal)
	assert.Equal(t, 1, n)
}

func TestSementara(t *testing.T) {
	assert.False(t, Sementara(nil))
	assert.False(t, Sementara(context.Canceled))
	assert.False(t, Sementara(ErrRobots))
	assert.False(t, Sementara(errors.New("lain")))
	assert.True(t, Sementara(&GalatHTTP{Status: 502, Sementara: true}))
	assert.False(t, Sementara(&GalatHTTP{Status: 400}))
	assert.True(t, Sementara(context.DeadlineExceeded))
}

func TestBreaker_MembukaSetelahKegagalanBeruntunLaluMencobaLagi(t *testing.T) {
	sekarang := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var panggilan int
	gagal := true
	next := fetcherFunc(func(context.Context, domain.Permintaan) ([]byte, error) {
		panggilan++
		if gagal {
			return nil, errors.New("down")
		}
		return []byte("ok"), nil
	})
	b := &Breaker{Next: next, Ambang: 3, Tunggu: time.Minute, Sekarang: func() time.Time { return sekarang }}
	p := domain.Permintaan{SumberID: "SRC-X"}

	for range 3 {
		_, err := b.Get(t.Context(), p)
		require.Error(t, err)
	}
	_, err := b.Get(t.Context(), p)
	require.ErrorIs(t, err, ErrSirkitTerbuka)
	assert.Equal(t, 3, panggilan, "sirkit terbuka tidak menghubungi layanan")

	// Sumber lain tidak terpengaruh.
	_, err = b.Get(t.Context(), domain.Permintaan{SumberID: "SRC-Y"})
	assert.NotErrorIs(t, err, ErrSirkitTerbuka)

	// Setelah masa tunggu: satu percobaan; gagal sekali lagi membuka sirkit.
	sekarang = sekarang.Add(2 * time.Minute)
	_, err = b.Get(t.Context(), p)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrSirkitTerbuka)
	_, err = b.Get(t.Context(), p)
	assert.ErrorIs(t, err, ErrSirkitTerbuka)

	// Pulih setelah percobaan berhasil.
	sekarang = sekarang.Add(2 * time.Minute)
	gagal = false
	_, err = b.Get(t.Context(), p)
	require.NoError(t, err)
	_, err = b.Get(t.Context(), p)
	require.NoError(t, err)
}

func TestBreaker_LarangRobotsDanPembatalanBukanKegagalanSumber(t *testing.T) {
	galat := ErrRobots
	next := fetcherFunc(func(context.Context, domain.Permintaan) ([]byte, error) { return nil, galat })
	b := &Breaker{Next: next, Ambang: 2, Tunggu: time.Minute}
	for range 5 {
		_, err := b.Get(t.Context(), domain.Permintaan{SumberID: "S"})
		assert.NotErrorIs(t, err, ErrSirkitTerbuka)
	}
	galat = context.Canceled
	for range 5 {
		_, err := b.Get(t.Context(), domain.Permintaan{SumberID: "S"})
		assert.NotErrorIs(t, err, ErrSirkitTerbuka)
	}
}
