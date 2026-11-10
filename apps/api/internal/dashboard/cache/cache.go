// Package cache menyimpan hasil agregasi dashboard yang berat dalam memori satu instance.
//
// Sifatnya:
//   - TTL pendek: UI memuat ulang tiap 30 detik, jadi hasil yang disimpan lebih lama dari itu hanya menunda data baru.
//     Manfaatnya untuk banyak pengguna yang melihat cakupan dan filter yang sama dalam jendela TTL, bukan untuk satu
//     pengguna yang polling sendirian.
//   - Satu pemuat per kunci (singleflight): permintaan kembar yang datang saat hasil belum ada menunggu pemuat
//     yang sama, bukan menyerbu basis data bersamaan (thundering herd), termasuk tepat setelah invalidasi.
//   - Galat tidak pernah disimpan, dan galat konteks satu pemanggil tidak menular ke pemanggil lain: pemuat berjalan
//     dengan konteks yang lepas dari pembatalan pemanggil pertama (tetapi dibatasi waktu), sedangkan tiap pemanggil
//     tetap berhenti menunggu bila konteksnya sendiri dibatalkan.
//   - Jumlah entri dibatasi (MaksEntri): kunci berasal dari parameter permintaan, jadi tanpa batas pengguna yang
//     berhak bisa menumbuhkan memori dengan mengarang filter. Saat penuh, entri kedaluwarsa dibuang lebih dulu, lalu
//     yang tertua.
//   - Invalidasi lewat epoch: Options.Epoch mengembalikan "versi data" (naik setiap agregat disegarkan, lihat
//     refresher.Epoch). Entri yang disimpan pada epoch lama dianggap tidak ada, jadi harga yang baru diterbitkan
//     langsung tampil tanpa menunggu TTL.
//
// Konsekuensi multi-instance: cache ada di memori tiap proses API dan epoch hanya naik di proses yang menjalankan
// penyegaran. Instance lain (atau penyegaran oleh worker di proses lain) baru terlihat setelah TTL, jadi data bisa
// tertinggal paling lama TTL di atas umur view itu sendiri. Untuk dashboard yang polling per 30 detik itu wajar;
// kebenaran cakupan tidak bergantung padanya, karena kunci memuat cakupan pemanggil.
//
// Nilai yang dikembalikan DIPAKAI BERSAMA oleh semua pemanggil kunci yang sama: pemanggil dilarang mengubahnya.
package cache

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

const (
	// BatasMuatBawaan membatasi waktu satu pemuat (konteks pemuat lepas dari pembatalan pemanggil).
	BatasMuatBawaan = 15 * time.Second
	// MaksEntriBawaan adalah batas jumlah entri bila Options.MaksEntri tidak diisi.
	MaksEntriBawaan = 256
)

// Options mengatur Cache.
type Options struct {
	// TTL adalah umur entri. TTL <= 0 mematikan cache: pemuat dipanggil pada setiap permintaan.
	TTL time.Duration
	// MaksEntri membatasi jumlah entri; <= 0 memakai MaksEntriBawaan.
	MaksEntri int
	// BatasMuat membatasi waktu satu pemuat; <= 0 memakai BatasMuatBawaan.
	BatasMuat time.Duration
	// Epoch mengembalikan versi data saat ini. Entri hanya berlaku pada epoch yang sama dengan saat dimuat. Nil:
	// epoch tetap (tanpa invalidasi selain TTL).
	Epoch func() uint64
	// Now menyediakan jam; nil memakai time.Now. Untuk tes.
	Now func() time.Time
}

type entri struct {
	nilai       any
	kedaluwarsa time.Time
	epoch       uint64
}

// Cache menyimpan hasil per kunci K. K harus comparable; kunci berupa struct bernilai (bukan string yang disambung)
// membuat dua kunci berbeda tidak mungkin bertabrakan karena pemisah.
type Cache[K comparable] struct {
	opts Options

	mu    sync.Mutex
	items map[K]entri
	sf    singleflight.Group
}

// New membuat Cache. Nilai Options yang kosong diganti bawaannya.
func New[K comparable](opts Options) *Cache[K] {
	if opts.MaksEntri <= 0 {
		opts.MaksEntri = MaksEntriBawaan
	}
	if opts.BatasMuat <= 0 {
		opts.BatasMuat = BatasMuatBawaan
	}
	if opts.Epoch == nil {
		opts.Epoch = func() uint64 { return 0 }
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return &Cache[K]{opts: opts, items: make(map[K]entri)}
}

// Ukuran mengembalikan jumlah entri yang sedang tersimpan (termasuk yang sudah kedaluwarsa tetapi belum dibuang).
func (c *Cache[K]) Ukuran() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.items)
}

// Bersihkan membuang semua entri.
func (c *Cache[K]) Bersihkan() {
	c.mu.Lock()
	defer c.mu.Unlock()
	clear(c.items)
}

// hasil membungkus nilai yang melewati singleflight supaya V yang berupa interface nil tetap aman dipulihkan.
type hasil[V any] struct{ nilai V }

// Ambil mengembalikan nilai yang tersimpan untuk kunci, atau memuatnya dengan muat lalu menyimpannya. Pemuat berjalan
// paling banyak satu per kunci pada satu waktu; galatnya diteruskan ke semua penunggu dan tidak disimpan. Nilai
// dikembalikan apa adanya dan dipakai bersama: jangan diubah.
//
// c nil atau TTL <= 0: tanpa cache, muat dipanggil langsung dengan ctx pemanggil.
func Ambil[K comparable, V any](ctx context.Context, c *Cache[K], kunci K, muat func(context.Context) (V, error)) (V, error) {
	var nol V
	if c == nil || c.opts.TTL <= 0 {
		return muat(ctx)
	}

	epoch := c.opts.Epoch()
	if h, ada := cariTipe[V](c, kunci, epoch); ada {
		return h.nilai, nil
	}

	// Kunci singleflight memuat epoch: permintaan sesudah invalidasi tidak bergabung dengan pemuat yang mulai
	// sebelumnya dan bisa membawa data lama.
	kunciMuat := fmt.Sprintf("%d|%#v", epoch, kunci)
	ch := c.sf.DoChan(kunciMuat, func() (any, error) {
		if h, ada := cariTipe[V](c, kunci, epoch); ada {
			return h, nil
		}
		// Lepas dari pembatalan pemanggil pertama: kalau kliennya putus, penunggu lain tidak boleh ikut gagal.
		muatCtx, batal := context.WithTimeout(context.WithoutCancel(ctx), c.opts.BatasMuat)
		defer batal()
		nilai, err := muatAman(muatCtx, muat)
		if err != nil {
			return nil, err
		}
		h := hasil[V]{nilai: nilai}
		c.simpan(kunci, h, epoch)
		return h, nil
	})

	select {
	case res := <-ch:
		if res.Err != nil {
			return nol, res.Err
		}
		return res.Val.(hasil[V]).nilai, nil
	case <-ctx.Done():
		return nol, ctx.Err()
	}
}

// cariTipe mencari entri yang berlaku dan bertipe hasil[V]. Entri bertipe lain (dua pemanggil memakai kunci yang
// sama untuk tipe nilai berbeda: salah pakai) diperlakukan sebagai tidak ada dan akan ditimpa, bukan membuat panik.
func cariTipe[V any, K comparable](c *Cache[K], kunci K, epoch uint64) (hasil[V], bool) {
	v, ada := c.cari(kunci, epoch)
	if !ada {
		return hasil[V]{}, false
	}
	h, ok := v.(hasil[V])
	return h, ok
}

// muatAman memanggil muat dan mengubah panik menjadi galat: panik di dalam goroutine singleflight tidak bisa
// ditangkap pemanggil dan akan mematikan seluruh proses, bukan sekadar permintaan yang gagal.
func muatAman[V any](ctx context.Context, muat func(context.Context) (V, error)) (nilai V, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("cache: pemuat panik: %v\n%s", r, debug.Stack())
		}
	}()
	return muat(ctx)
}

// cari mengembalikan entri yang masih berlaku (belum kedaluwarsa dan sepoch). Entri yang tidak berlaku dibuang.
func (c *Cache[K]) cari(kunci K, epoch uint64) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ada := c.items[kunci]
	if !ada {
		return nil, false
	}
	if e.epoch != epoch || !c.opts.Now().Before(e.kedaluwarsa) {
		delete(c.items, kunci)
		return nil, false
	}
	return e.nilai, true
}

// simpan menyimpan nilai bila epoch pemuatnya masih terkini: nilai yang sudah basi sebelum sempat disimpan (agregat
// disegarkan selama pemuatan) tidak ada gunanya.
func (c *Cache[K]) simpan(kunci K, nilai any, epoch uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	sekarang := c.opts.Now()
	if epoch != c.opts.Epoch() {
		return
	}
	if _, ada := c.items[kunci]; !ada && len(c.items) >= c.opts.MaksEntri {
		c.bersihkanLocked(sekarang)
		if len(c.items) >= c.opts.MaksEntri {
			c.buangTertuaLocked()
		}
	}
	c.items[kunci] = entri{nilai: nilai, kedaluwarsa: sekarang.Add(c.opts.TTL), epoch: epoch}
}

// bersihkanLocked membuang entri yang kedaluwarsa atau berepoch lama. c.mu harus dipegang.
func (c *Cache[K]) bersihkanLocked(sekarang time.Time) {
	epoch := c.opts.Epoch()
	for k, e := range c.items {
		if e.epoch != epoch || !sekarang.Before(e.kedaluwarsa) {
			delete(c.items, k)
		}
	}
}

// buangTertuaLocked membuang satu entri yang paling dulu kedaluwarsa (TTL sama, jadi yang paling lama disimpan).
// c.mu harus dipegang.
func (c *Cache[K]) buangTertuaLocked() {
	var (
		tertua K
		waktu  time.Time
		ada    bool
	)
	for k, e := range c.items {
		if !ada || e.kedaluwarsa.Before(waktu) {
			tertua, waktu, ada = k, e.kedaluwarsa, true
		}
	}
	if ada {
		delete(c.items, tertua)
	}
}
