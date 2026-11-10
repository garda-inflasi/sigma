package usecase

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/dashboard/cache"
	"github.com/garda-inflasi/api/internal/dashboard/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Ringkasan 08-B menerima kabupaten_kode dan komoditas_kode: tiap kombinasi dihitung sendiri, dan mengulang kombinasi
// yang sama tidak menghitung ulang.
func TestCache_SaringanOperasionalMenghasilkanHasilTerpisah(t *testing.T) {
	variasi := []domain.PermintaanSaringan{
		{KabupatenKode: "1171", KomoditasKode: "TMT"},
		{KabupatenKode: "1108", KomoditasKode: "TMT"},
		{KomoditasKode: "TMT"},
		{KabupatenKode: "1171", KomoditasKode: "BWM"},
		{KabupatenKode: "1171"},
		{},
	}
	endpoint := []struct {
		nama    string
		metode  string
		panggil func(uc *UseCase, req domain.PermintaanSaringan) error
	}{
		{"stok", "FaktaStok", func(uc *UseCase, req domain.PermintaanSaringan) error {
			_, err := uc.GetStok(t.Context(), semuaAceh, req)
			return err
		}},
		{"produsen", "Produsen", func(uc *UseCase, req domain.PermintaanSaringan) error {
			_, err := uc.GetProdusen(t.Context(), semuaAceh, req)
			return err
		}},
		{"logistik", "Logistik", func(uc *UseCase, req domain.PermintaanSaringan) error {
			_, err := uc.GetLogistik(t.Context(), semuaAceh, req)
			return err
		}},
	}
	for _, e := range endpoint {
		t.Run(e.nama, func(t *testing.T) {
			repo := &fakeRepo{}
			uc := newUC(repo)

			for _, v := range variasi {
				require.NoError(t, e.panggil(uc, v))
			}
			assert.Equal(t, len(variasi), repo.panggilan(e.metode), "tiap kombinasi filter dihitung sendiri")

			for _, v := range variasi {
				require.NoError(t, e.panggil(uc, v))
			}
			assert.Equal(t, len(variasi), repo.panggilan(e.metode), "mengulang kombinasi yang sama membaca cache")
		})
	}
}

// Aktivitas per petugas hanya membaca kabupaten_kode; komoditas_kode yang diabaikan tidak boleh membelah cache.
func TestCache_AktivitasPetugasHanyaMembedakanKabupaten(t *testing.T) {
	repo := &fakeRepo{}
	uc := newUC(repo)

	for _, kab := range []string{"", "1171", "1108", "", "1171"} {
		_, err := uc.GetAktivitasPetugas(t.Context(), semuaAceh, kab)
		require.NoError(t, err)
	}

	assert.Equal(t, 3, repo.panggilan("AktivitasPetugas"), "tiga kabupaten berbeda, sisanya dari cache")
}

// Ringkasan produsen bersumber dari agregat harga: begitu refresher selesai menyegarkan (epoch naik), hasilnya dihitung
// ulang tanpa menunggu TTL. Stok dan logistik dibaca dari tabel, jadi hanya TTL yang membatasi umurnya.
func TestCache_ProdusenDibuangSaatEpochNaik(t *testing.T) {
	var epoch atomic.Uint64
	repo := &fakeRepo{}
	uc := newUC(repo, WithEpoch(epoch.Load))

	_, _ = uc.GetProdusen(t.Context(), semuaAceh, domain.PermintaanSaringan{})
	_, _ = uc.GetProdusen(t.Context(), semuaAceh, domain.PermintaanSaringan{})
	require.Equal(t, 1, repo.panggilan("Produsen"))

	epoch.Add(1)
	_, _ = uc.GetProdusen(t.Context(), semuaAceh, domain.PermintaanSaringan{})

	assert.Equal(t, 2, repo.panggilan("Produsen"), "penyegaran agregat membuang hasil lama")
}

// Keterkinian stok: tepat sampai TTL (20 detik) hasil yang sama disajikan, sesudahnya dihitung ulang dari tabel.
func TestCache_StokDanLogistikDihitungUlangSetelahTTL(t *testing.T) {
	jam := time.Date(2026, 10, 16, 20, 0, 0, 0, time.UTC)
	var mu sync.Mutex
	repo := &fakeRepo{}
	uc := newUC(repo, DenganCache(func(o *cache.Options) {
		o.Now = func() time.Time { mu.Lock(); defer mu.Unlock(); return jam }
	}))
	maju := func(d time.Duration) { mu.Lock(); jam = jam.Add(d); mu.Unlock() }
	panggil := func() {
		_, _ = uc.GetStok(t.Context(), banda, domain.PermintaanSaringan{})
		_, _ = uc.GetLogistik(t.Context(), banda, domain.PermintaanSaringan{})
	}

	panggil()
	maju(TTLHasil - time.Second)
	panggil()
	assert.Equal(t, 1, repo.panggilan("FaktaStok"), "dalam TTL")
	assert.Equal(t, 1, repo.panggilan("Logistik"), "dalam TTL")

	maju(time.Second)
	panggil()
	assert.Equal(t, 2, repo.panggilan("FaktaStok"), "TTL habis: stok dibaca ulang dari tabel")
	assert.Equal(t, 2, repo.panggilan("Logistik"))
	assert.Less(t, TTLHasil+30*time.Second, time.Minute, "kasus terburuk: TTL + jeda polling UI < 1 menit")
}
