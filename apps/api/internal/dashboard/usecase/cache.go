package usecase

import (
	"time"

	"github.com/garda-inflasi/api/internal/dashboard/cache"
	"github.com/garda-inflasi/api/internal/dashboard/domain"
)

const (
	// TTLHasil adalah umur hasil agregasi di cache: lebih pendek dari jeda polling UI (30 detik), jadi pengguna yang
	// polling sendirian tetap mendapat data segar tiap kali, sedangkan banyak pengguna dengan cakupan dan filter yang
	// sama dalam jendela itu berbagi satu perhitungan. Data publikasi harga tidak menunggu TTL: refresher menaikkan
	// epoch dan cache membuang hasil lama (lihat WithEpoch).
	TTLHasil = 20 * time.Second
	// MaksEntriHasil membatasi jumlah hasil yang disimpan (kunci berasal dari parameter permintaan). Hasil terbesar
	// (heatmap seluruh Aceh) puluhan KB, jadi memori cache paling banyak beberapa MB.
	MaksEntriHasil = 256
)

// Kunci mengidentifikasi satu hasil agregasi di cache. SEMUA yang memengaruhi isi hasil wajib ada di sini: kunci yang
// kurang satu bidang membuat hasil satu cakupan atau filter tersaji ke yang lain, dan kebocoran lintas cakupan adalah
// cacat keamanan. Kunci berupa struct bernilai, bukan teks yang disambung, jadi dua kunci berbeda tidak mungkin
// bertabrakan karena pemisah. Tambahkan bidang baru di sini setiap kali endpoint menerima parameter baru (mis. stok
// per produsen), dan isi lewat kunciFilter/kunciCakupan, bukan dengan menyusun Kunci sendiri.
//
// Bagian "invalidasi" tidak ada di kunci: ia ditangani epoch di cache.Cache.
type Kunci struct {
	// Titik adalah nama endpoint, mis. "kpi" atau "heatmap".
	Titik string

	// Cakupan pemanggil (02-B): seluruh Aceh atau satu kabupaten. Cakupan tertutup tidak pernah sampai ke cache.
	Semua            bool
	KabupatenCakupan string

	// Filter permintaan yang sudah dinormalisasi; kosong untuk endpoint tanpa filter.
	Kabupaten string
	Komoditas string
	Dari      string
	Sampai    string
	// Periode: pemanggil mengisi dari atau sampai (jalur heatmap langsung, bukan dari mv_heatmap_harga).
	Periode bool

	// Hari adalah tanggal hari ini menurut waktu Aceh saat permintaan: hasil kemarin tidak boleh tersaji hari ini,
	// untuk endpoint yang acuannya hari ini (aktivitas, pembanding, cuaca, dan KPI/heatmap tanpa periode).
	Hari string
}

// kunciCakupan membuat kunci untuk endpoint yang hanya bergantung pada cakupan dan hari ini.
func kunciCakupan(titik string, c domain.Cakupan, hari time.Time) Kunci {
	return Kunci{Titik: titik, Semua: c.Semua, KabupatenCakupan: c.KabupatenKode, Hari: hari.Format(domain.TanggalLayout)}
}

// kunciSaringan membuat kunci untuk ringkasan operasional yang menerima filter kabupaten dan komoditas tetapi tidak
// punya periode pilihan (jendelanya relatif terhadap Hari). Parameter yang diabaikan sebuah endpoint tidak boleh
// sampai ke sini (kosongkan di pemanggil), kalau tidak setiap nilai sembarang pada parameter itu menjadi entri cache sendiri.
func kunciSaringan(titik string, c domain.Cakupan, s domain.Saringan, hari time.Time) Kunci {
	k := kunciCakupan(titik, c, hari)
	k.Kabupaten, k.Komoditas = s.KabupatenKode, s.KomoditasKode
	return k
}

// kunciFilter membuat kunci untuk endpoint yang juga menerima filter kabupaten, komoditas, dan periode.
func kunciFilter(titik string, c domain.Cakupan, f domain.Filter, hari time.Time) Kunci {
	k := kunciCakupan(titik, c, hari)
	k.Kabupaten, k.Komoditas = f.KabupatenKode, f.KomoditasKode
	k.Dari, k.Sampai = f.Dari.Format(domain.TanggalLayout), f.Sampai.Format(domain.TanggalLayout)
	k.Periode = f.Periode
	return k
}

// Option mengubah pengaturan cache hasil saat UseCase dibuat.
type Option func(*cache.Options)

// WithEpoch memasang sumber epoch (refresher.Epoch): hasil yang dimuat pada epoch lama dibuang begitu epoch naik.
// Cache ada di memori satu proses; epoch hanya melihat penyegaran di proses itu (lihat paket cache untuk konsekuensi
// multi-instance).
func WithEpoch(epoch func() uint64) Option { return func(o *cache.Options) { o.Epoch = epoch } }

// TanpaCache mematikan cache: setiap permintaan menghitung ulang.
func TanpaCache() Option { return func(o *cache.Options) { o.TTL = 0 } }

// DenganCache mengubah pengaturan cache secara bebas (jam, TTL, batas entri); untuk tes.
func DenganCache(ubah func(*cache.Options)) Option { return func(o *cache.Options) { ubah(o) } }
