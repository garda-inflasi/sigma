package cache_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/dashboard/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// jam adalah jam palsu yang bisa dimajukan.
type jam struct {
	mu sync.Mutex
	t  time.Time
}

func (j *jam) Now() time.Time {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.t
}

func (j *jam) Maju(d time.Duration) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.t = j.t.Add(d)
}

func baru(t *testing.T, ubah ...func(*cache.Options)) (*cache.Cache[string], *jam, *atomic.Uint64) {
	t.Helper()
	j := &jam{t: time.Date(2026, 11, 10, 8, 0, 0, 0, time.UTC)}
	var epoch atomic.Uint64
	opts := cache.Options{TTL: 20 * time.Second, MaksEntri: 100, Now: j.Now, Epoch: epoch.Load}
	for _, u := range ubah {
		u(&opts)
	}
	return cache.New[string](opts), j, &epoch
}

// pemuat menghitung panggilan dan mengembalikan nilai berurutan: "v1", "v2", ...
type pemuat struct{ n atomic.Int64 }

func (p *pemuat) muat(context.Context) (string, error) {
	return "v" + string(rune('0'+p.n.Add(1))), nil
}

func TestAmbil_SimpanDanPakaiUlang(t *testing.T) {
	c, _, _ := baru(t)
	var p pemuat

	a, err := cache.Ambil(t.Context(), c, "k", p.muat)
	require.NoError(t, err)
	b, err := cache.Ambil(t.Context(), c, "k", p.muat)
	require.NoError(t, err)
	lain, err := cache.Ambil(t.Context(), c, "lain", p.muat)
	require.NoError(t, err)

	assert.Equal(t, "v1", a)
	assert.Equal(t, "v1", b, "kunci yang sama memakai hasil tersimpan")
	assert.Equal(t, "v2", lain, "kunci lain dimuat sendiri")
	assert.EqualValues(t, 2, p.n.Load())
}

func TestAmbil_TTLHabis(t *testing.T) {
	c, j, _ := baru(t)
	var p pemuat
	_, _ = cache.Ambil(t.Context(), c, "k", p.muat)

	j.Maju(19 * time.Second)
	v, _ := cache.Ambil(t.Context(), c, "k", p.muat)
	assert.Equal(t, "v1", v, "belum kedaluwarsa")

	j.Maju(time.Second)
	v, _ = cache.Ambil(t.Context(), c, "k", p.muat)
	assert.Equal(t, "v2", v, "tepat TTL: dimuat ulang")
}

func TestAmbil_GalatTidakDisimpan(t *testing.T) {
	c, _, _ := baru(t)
	var panggilan atomic.Int64
	boom := errors.New("db mati")
	muat := func(context.Context) (string, error) {
		if panggilan.Add(1) == 1 {
			return "", boom
		}
		return "pulih", nil
	}

	_, err := cache.Ambil(t.Context(), c, "k", muat)
	require.ErrorIs(t, err, boom)
	assert.Zero(t, c.Ukuran(), "galat tidak disimpan")

	v, err := cache.Ambil(t.Context(), c, "k", muat)
	require.NoError(t, err)
	assert.Equal(t, "pulih", v, "permintaan berikutnya mencoba lagi, tidak mengulang galat")
	assert.EqualValues(t, 2, panggilan.Load())
}

func TestAmbil_EpochBaruMembuangHasilLama(t *testing.T) {
	c, _, epoch := baru(t)
	var p pemuat
	v, _ := cache.Ambil(t.Context(), c, "k", p.muat)
	assert.Equal(t, "v1", v)

	epoch.Add(1) // agregat disegarkan

	v, _ = cache.Ambil(t.Context(), c, "k", p.muat)
	assert.Equal(t, "v2", v, "hasil sebelum penyegaran tidak boleh tersaji sesudahnya, walau TTL belum habis")
	v, _ = cache.Ambil(t.Context(), c, "k", p.muat)
	assert.Equal(t, "v2", v, "hasil baru disimpan pada epoch baru")
}

func TestAmbil_HasilYangBasiSelamaPemuatanTidakDisimpan(t *testing.T) {
	c, _, epoch := baru(t)
	var n atomic.Int64
	muat := func(context.Context) (string, error) {
		if n.Add(1) == 1 {
			epoch.Add(1) // penyegaran selesai saat hasil sedang dihitung dari data lama
			return "lama", nil
		}
		return "baru", nil
	}

	v, err := cache.Ambil(t.Context(), c, "k", muat)
	require.NoError(t, err)
	assert.Equal(t, "lama", v, "pemanggil yang sedang menunggu tetap dijawab")
	assert.Zero(t, c.Ukuran(), "tetapi hasil yang sudah basi tidak disimpan")

	v, _ = cache.Ambil(t.Context(), c, "k", muat)
	assert.Equal(t, "baru", v)
}

func TestAmbil_SatuPemuatPerKunci(t *testing.T) {
	c, _, _ := baru(t)
	const penelepon = 50
	var panggilan atomic.Int64
	lepas := make(chan struct{})
	muat := func(context.Context) (string, error) {
		panggilan.Add(1)
		<-lepas
		return "hasil", nil
	}

	var wg sync.WaitGroup
	hasil := make([]string, penelepon)
	for i := range penelepon {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hasil[i], _ = cache.Ambil(t.Context(), c, "k", muat)
		}()
	}
	// Beri waktu semua penelepon bergabung pada pemuat yang sedang berjalan sebelum dilepas.
	require.Eventually(t, func() bool { return panggilan.Load() >= 1 }, 2*time.Second, time.Millisecond)
	time.Sleep(100 * time.Millisecond)
	close(lepas)
	wg.Wait()

	assert.EqualValues(t, 1, panggilan.Load(), "50 permintaan kembar hanya memuat sekali (tanpa thundering herd)")
	for _, h := range hasil {
		assert.Equal(t, "hasil", h)
	}
}

func TestAmbil_PermintaanSesudahInvalidasiTidakBergabungDenganPemuatLama(t *testing.T) {
	c, _, epoch := baru(t)
	lepasLama := make(chan struct{})
	var n atomic.Int64
	muat := func(context.Context) (string, error) {
		if n.Add(1) == 1 {
			<-lepasLama
			return "lama", nil
		}
		return "baru", nil
	}

	lama := make(chan string, 1)
	go func() { v, _ := cache.Ambil(t.Context(), c, "k", muat); lama <- v }()
	require.Eventually(t, func() bool { return n.Load() == 1 }, 2*time.Second, time.Millisecond)

	epoch.Add(1) // penyegaran selesai selagi pemuat lama masih berjalan
	v, err := cache.Ambil(t.Context(), c, "k", muat)
	require.NoError(t, err)
	assert.Equal(t, "baru", v, "permintaan sesudah invalidasi tidak menunggu pemuat yang membawa data lama")

	close(lepasLama)
	assert.Equal(t, "lama", <-lama)
}

func TestAmbil_PembatalanPemanggilPertamaTidakMenularKePemanggilLain(t *testing.T) {
	c, _, _ := baru(t)
	lepas := make(chan struct{})
	var konteksPemuatBatal atomic.Bool
	var mulai atomic.Int64
	muat := func(ctx context.Context) (string, error) {
		mulai.Add(1)
		<-lepas
		konteksPemuatBatal.Store(ctx.Err() != nil)
		return "hasil", ctx.Err()
	}

	ctx1, batal1 := context.WithCancel(t.Context())
	hasil1 := make(chan error, 1)
	go func() { _, err := cache.Ambil(ctx1, c, "k", muat); hasil1 <- err }()
	require.Eventually(t, func() bool { return mulai.Load() == 1 }, 2*time.Second, time.Millisecond)

	hasil2 := make(chan string, 1)
	go func() { v, _ := cache.Ambil(t.Context(), c, "k", muat); hasil2 <- v }()
	time.Sleep(50 * time.Millisecond) // pemanggil kedua bergabung

	batal1() // klien pertama putus
	assert.ErrorIs(t, <-hasil1, context.Canceled, "pemanggil yang dibatalkan berhenti menunggu")

	close(lepas)
	assert.Equal(t, "hasil", <-hasil2, "pemanggil lain tetap mendapat hasil, bukan galat konteks milik klien yang putus")
	assert.False(t, konteksPemuatBatal.Load(), "pemuat berjalan dengan konteks yang lepas dari pembatalan pemanggil")
	assert.EqualValues(t, 1, mulai.Load())
}

func TestAmbil_PemuatDibatasiWaktu(t *testing.T) {
	c, _, _ := baru(t, func(o *cache.Options) { o.BatasMuat = 30 * time.Millisecond })
	muat := func(ctx context.Context) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	}

	_, err := cache.Ambil(t.Context(), c, "k", muat)

	assert.ErrorIs(t, err, context.DeadlineExceeded, "pemuat yang menggantung tidak menahan permintaan selamanya")
	assert.Zero(t, c.Ukuran())
}

func TestAmbil_PanikDiPemuatMenjadiGalat(t *testing.T) {
	c, _, _ := baru(t)

	_, err := cache.Ambil(t.Context(), c, "k", func(context.Context) (string, error) {
		panic("pemuat rusak")
	})

	require.Error(t, err, "panik di goroutine singleflight akan mematikan proses; harus jadi galat biasa")
	assert.Contains(t, err.Error(), "panik")
	assert.Zero(t, c.Ukuran())
	v, err := cache.Ambil(t.Context(), c, "k", (&pemuat{}).muat)
	require.NoError(t, err)
	assert.Equal(t, "v1", v, "cache tetap berfungsi sesudahnya")
}

func TestAmbil_BatasJumlahEntri(t *testing.T) {
	c, j, _ := baru(t, func(o *cache.Options) { o.MaksEntri = 3 })
	var p pemuat

	for _, k := range []string{"a", "b", "c"} {
		_, _ = cache.Ambil(t.Context(), c, k, p.muat)
		j.Maju(time.Second)
	}
	require.Equal(t, 3, c.Ukuran())

	// Penuh: entri tertua (a) dibuang, bukan memori yang tumbuh.
	_, _ = cache.Ambil(t.Context(), c, "d", p.muat)
	assert.Equal(t, 3, c.Ukuran())
	n := p.n.Load()
	_, _ = cache.Ambil(t.Context(), c, "b", p.muat)
	_, _ = cache.Ambil(t.Context(), c, "c", p.muat)
	_, _ = cache.Ambil(t.Context(), c, "d", p.muat)
	assert.Equal(t, n, p.n.Load(), "b, c, d masih tersimpan")
	_, _ = cache.Ambil(t.Context(), c, "a", p.muat)
	assert.Equal(t, n+1, p.n.Load(), "a sudah dibuang")

	// Banjir kunci karangan: ukuran tidak pernah melewati batas.
	for i := range 1000 {
		_, _ = cache.Ambil(t.Context(), c, "karangan-"+string(rune(i)), p.muat)
		require.LessOrEqual(t, c.Ukuran(), 3)
	}
}

func TestAmbil_EntriKedaluwarsaDibuangSebelumYangMasihBerlaku(t *testing.T) {
	c, j, _ := baru(t, func(o *cache.Options) { o.MaksEntri = 2 })
	var p pemuat
	_, _ = cache.Ambil(t.Context(), c, "a", p.muat)
	j.Maju(15 * time.Second)
	_, _ = cache.Ambil(t.Context(), c, "b", p.muat) // b lebih muda
	j.Maju(10 * time.Second)                        // a kedaluwarsa, b belum

	_, _ = cache.Ambil(t.Context(), c, "c", p.muat)

	n := p.n.Load()
	v, _ := cache.Ambil(t.Context(), c, "b", p.muat)
	assert.Equal(t, "v2", v)
	assert.Equal(t, n, p.n.Load(), "b yang masih berlaku dipertahankan")
}

func TestAmbil_TanpaCache(t *testing.T) {
	var p pemuat
	for _, c := range []*cache.Cache[string]{nil, cache.New[string](cache.Options{TTL: 0})} {
		a, _ := cache.Ambil(t.Context(), c, "k", p.muat)
		b, _ := cache.Ambil(t.Context(), c, "k", p.muat)
		assert.NotEqual(t, a, b, "tanpa cache setiap panggilan memuat sendiri")
	}
}

func TestAmbil_KunciStructBernilai(t *testing.T) {
	type kunci struct {
		a, b string
	}
	c := cache.New[kunci](cache.Options{TTL: time.Minute})
	var p pemuat

	x, _ := cache.Ambil(t.Context(), c, kunci{"1171", ""}, p.muat)
	y, _ := cache.Ambil(t.Context(), c, kunci{"117", "1"}, p.muat)
	x2, _ := cache.Ambil(t.Context(), c, kunci{"1171", ""}, p.muat)

	assert.NotEqual(t, x, y, "pemisah tidak membuat dua kunci berbeda bertabrakan")
	assert.Equal(t, x, x2)
}

func TestAmbil_TipeNilaiBerbedaUntukKunciSamaTidakPanik(t *testing.T) {
	c, _, _ := baru(t)
	s, err := cache.Ambil(t.Context(), c, "k", func(context.Context) (string, error) { return "teks", nil })
	require.NoError(t, err)
	assert.Equal(t, "teks", s)

	// Salah pakai: kunci yang sama untuk tipe lain. Diperlakukan sebagai tidak ada, bukan panik.
	n, err := cache.Ambil(t.Context(), c, "k", func(context.Context) (int, error) { return 42, nil })
	require.NoError(t, err)
	assert.Equal(t, 42, n)
}

func TestAmbil_NilaiPointerNilDanInterfaceNil(t *testing.T) {
	c, _, _ := baru(t)

	p, err := cache.Ambil(t.Context(), c, "ptr", func(context.Context) (*int, error) { return nil, nil })
	require.NoError(t, err)
	assert.Nil(t, p)

	var n atomic.Int64
	i, err := cache.Ambil(t.Context(), c, "iface", func(context.Context) (any, error) { n.Add(1); return nil, nil })
	require.NoError(t, err)
	assert.Nil(t, i)
	_, _ = cache.Ambil(t.Context(), c, "iface", func(context.Context) (any, error) { n.Add(1); return nil, nil })
	assert.EqualValues(t, 1, n.Load(), "hasil nil pun disimpan dan dipulihkan tanpa panik")
}

func TestBersihkan(t *testing.T) {
	c, _, _ := baru(t)
	var p pemuat
	_, _ = cache.Ambil(t.Context(), c, "k", p.muat)
	require.Equal(t, 1, c.Ukuran())

	c.Bersihkan()

	assert.Zero(t, c.Ukuran())
}
