package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/dashboard/cache"
	"github.com/garda-inflasi/api/internal/dashboard/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// repoPerCakupan membuat repository palsu yang jawabannya BERBEDA untuk tiap cakupan, supaya kebocoran cache lintas
// cakupan langsung terlihat: seluruh Aceh menjawab "ACEH", cakupan kabupaten menjawab kodenya.
func repoPerCakupan() *fakeRepo {
	label := func(c domain.Cakupan) string {
		if c.Semua {
			return "ACEH"
		}
		return c.KabupatenKode
	}
	angka := map[string]float64{"ACEH": 1000, "1171": 11, "1108": 8}
	return &fakeRepo{
		deretKpi: func(q domain.DeretKpiQuery) ([]domain.TitikKpi, error) {
			return []domain.TitikKpi{{Indikator: domain.IndikatorEntri, Tanggal: q.Akhir, Pembilang: angka[label(q.Cakupan)]}}, nil
		},
		heatmap: func(q domain.HeatmapQuery) (*domain.HeatmapData, error) {
			return &domain.HeatmapData{Sel: []domain.SelHeatmap{{KabupatenKode: label(q.Cakupan), KomoditasKode: q.KomoditasKode}}}, nil
		},
		pembanding: func(_ time.Time, c domain.Cakupan) ([]domain.PembandingHarga, error) {
			return []domain.PembandingHarga{{KabupatenKode: label(c), KomoditasKode: "TMT"}}, nil
		},
		cuaca: func(_, _ time.Time, c domain.Cakupan) ([]domain.CuacaKabupaten, error) {
			return []domain.CuacaKabupaten{{KabupatenKode: label(c)}}, nil
		},
		aktivitas: func(_, _ time.Time, c domain.Cakupan) ([]domain.AktivitasKabupaten, error) {
			return []domain.AktivitasKabupaten{{KabupatenKode: label(c)}}, nil
		},
		petugas: func(q domain.AktivitasPetugasQuery) ([]domain.AktivitasPetugas, error) {
			return []domain.AktivitasPetugas{{ID: "p-" + label(q.Cakupan), KabupatenKode: q.KabupatenKode}}, nil
		},
		stok: func(q domain.StokQuery) (domain.FaktaStok, error) {
			return domain.FaktaStok{Baris: []domain.BarisStok{
				{KomoditasKode: q.KomoditasKode, KomoditasNama: label(q.Cakupan), FasilitasID: "F", Kelas: domain.KelasStokBerlaku, JumlahLot: 1, VolumeTon: 1},
			}}, nil
		},
		produsen: func(q domain.ProdusenQuery) ([]domain.ProdusenFakta, error) {
			return []domain.ProdusenFakta{{KomoditasKode: q.KomoditasKode, KomoditasNama: label(q.Cakupan), HargaProdusen: f64(1)}}, nil
		},
		logistik: func(q domain.LogistikQuery) (domain.LogistikFakta, error) {
			return domain.LogistikFakta{Agregat: []domain.GerakanAgregat{
				{KomoditasKode: q.KomoditasKode, KomoditasNama: label(q.Cakupan), Jenis: domain.JenisMasuk, Jumlah: 1, VolumeTon: 1},
			}}, nil
		},
	}
}

// endpoint membungkus satu endpoint agregat untuk tes lintas endpoint.
type endpoint struct {
	nama string
	// panggil mengembalikan label cakupan yang MENGHASILKAN data (ACEH, 1171, 1108), dibaca dari isi hasil.
	panggil func(uc *UseCase, c domain.Cakupan, req domain.PermintaanFilter) (string, error)
	// metode di fakeRepo yang melayaninya.
	metode string
}

var endpoints = []endpoint{
	{"kpi", func(uc *UseCase, c domain.Cakupan, req domain.PermintaanFilter) (string, error) {
		k, err := uc.GetKpiOperasional(context.Background(), c, req)
		if err != nil {
			return "", err
		}
		// Fake menjawab entri 1000 untuk seluruh Aceh, 11 untuk 1171, dan 8 untuk 1108.
		return map[float64]string{1000: "ACEH", 11: "1171", 8: "1108"}[kartu(k, "entri").Nilai], nil
	}, "DeretKpi"},
	{"heatmap", func(uc *UseCase, c domain.Cakupan, req domain.PermintaanFilter) (string, error) {
		h, err := uc.GetHeatmap(context.Background(), c, req)
		if err != nil {
			return "", err
		}
		return h.Sel[0].KabupatenKode, nil
	}, "Heatmap"},
	{"pembanding-harga", func(uc *UseCase, c domain.Cakupan, _ domain.PermintaanFilter) (string, error) {
		p, err := uc.GetPembandingHarga(context.Background(), c)
		if err != nil {
			return "", err
		}
		return p.Items[0].KabupatenKode, nil
	}, "PembandingHarga"},
	{"cuaca", func(uc *UseCase, c domain.Cakupan, _ domain.PermintaanFilter) (string, error) {
		items, err := uc.GetCuaca(context.Background(), c)
		if err != nil {
			return "", err
		}
		return items[0].KabupatenKode, nil
	}, "CuacaTerbaru"},
	{"aktivitas-lapangan", func(uc *UseCase, c domain.Cakupan, _ domain.PermintaanFilter) (string, error) {
		items, err := uc.GetAktivitasLapangan(context.Background(), c)
		if err != nil {
			return "", err
		}
		return items[0].KabupatenKode, nil
	}, "AktivitasLapangan"},
	// Ringkasan 08-B. Fake menjawab dengan label cakupan di kolom nama (atau ID) supaya kebocoran lintas cakupan
	// langsung terlihat.
	{"aktivitas-petugas", func(uc *UseCase, c domain.Cakupan, _ domain.PermintaanFilter) (string, error) {
		h, err := uc.GetAktivitasPetugas(context.Background(), c, "")
		if err != nil {
			return "", err
		}
		return strings.TrimPrefix(h.Petugas[0].ID, "p-"), nil
	}, "AktivitasPetugas"},
	{"stok", func(uc *UseCase, c domain.Cakupan, req domain.PermintaanFilter) (string, error) {
		s, err := uc.GetStok(context.Background(), c, domain.PermintaanSaringan{KomoditasKode: req.KomoditasKode})
		if err != nil {
			return "", err
		}
		return s.Komoditas[0].KomoditasNama, nil
	}, "FaktaStok"},
	{"produsen", func(uc *UseCase, c domain.Cakupan, req domain.PermintaanFilter) (string, error) {
		p, err := uc.GetProdusen(context.Background(), c, domain.PermintaanSaringan{KomoditasKode: req.KomoditasKode})
		if err != nil {
			return "", err
		}
		return p.Komoditas[0].KomoditasNama, nil
	}, "Produsen"},
	{"logistik", func(uc *UseCase, c domain.Cakupan, req domain.PermintaanFilter) (string, error) {
		l, err := uc.GetLogistik(context.Background(), c, domain.PermintaanSaringan{KomoditasKode: req.KomoditasKode})
		if err != nil {
			return "", err
		}
		return l.Komoditas[0].KomoditasNama, nil
	}, "Logistik"},
}

// Cacat keamanan yang dijaga tes ini: hasil yang dihitung untuk satu cakupan tersaji ke cakupan lain lewat cache.
// Tiga cakupan memanggil kelima endpoint bergantian dengan filter yang sama; masing-masing harus selalu mendapat
// hasil cakupannya sendiri, baik pada panggilan pertama (dihitung) maupun berikutnya (dari cache).
func TestCache_HasilSatuCakupanTidakPernahTersajiKeCakupanLain(t *testing.T) {
	cakupan := []struct {
		nama  string
		c     domain.Cakupan
		label string
	}{
		{"seluruh Aceh", semuaAceh, "ACEH"},
		{"Banda Aceh", banda, "1171"},
		{"Aceh Selatan", domain.CakupanKabupaten("1108"), "1108"},
	}
	for _, e := range endpoints {
		t.Run(e.nama, func(t *testing.T) {
			repo := repoPerCakupan()
			uc := newUC(repo)
			req := domain.PermintaanFilter{KomoditasKode: "TMT"}

			// Dua putaran dengan urutan berbeda: putaran pertama mengisi cache, putaran kedua membacanya.
			for _, urutan := range [][]int{{0, 1, 2}, {2, 1, 0}, {1, 2, 0}} {
				for _, i := range urutan {
					got, err := e.panggil(uc, cakupan[i].c, req)
					require.NoError(t, err)
					assert.Equal(t, cakupan[i].label, got, "%s harus mendapat hasil cakupannya sendiri, bukan hasil cakupan lain", cakupan[i].nama)
				}
			}
			assert.Equal(t, 3, repo.panggilan(e.metode), "tiga cakupan dihitung sekali masing-masing, sisanya dari cache")
		})
	}
}

// Kunci tidak boleh memperlakukan "seluruh Aceh dengan filter kabupaten X" sama dengan "cakupan kabupaten X": hasilnya
// kebetulan sama hari ini, tetapi memisahkan keduanya menjaga kunci tetap sederhana dan tidak bergantung pada irisan.
func TestCache_KunciMemuatCakupanDanSemuaParameter(t *testing.T) {
	f := domain.Filter{KabupatenKode: "1171", KomoditasKode: "TMT", Dari: tgl("2026-10-04"), Sampai: tgl("2026-10-17")}
	dasar := kunciFilter("kpi", semuaAceh, f, hariUji)

	variasi := map[string]Kunci{
		"titik":               kunciFilter("heatmap", semuaAceh, f, hariUji),
		"cakupan kabupaten":   kunciFilter("kpi", banda, f, hariUji),
		"cakupan tertutup":    kunciFilter("kpi", domain.Cakupan{}, f, hariUji),
		"kabupaten filter":    kunciFilter("kpi", semuaAceh, func() domain.Filter { g := f; g.KabupatenKode = "1108"; return g }(), hariUji),
		"komoditas filter":    kunciFilter("kpi", semuaAceh, func() domain.Filter { g := f; g.KomoditasKode = "BWM"; return g }(), hariUji),
		"komoditas kosong":    kunciFilter("kpi", semuaAceh, func() domain.Filter { g := f; g.KomoditasKode = ""; return g }(), hariUji),
		"dari":                kunciFilter("kpi", semuaAceh, func() domain.Filter { g := f; g.Dari = tgl("2026-10-05"); return g }(), hariUji),
		"sampai":              kunciFilter("kpi", semuaAceh, func() domain.Filter { g := f; g.Sampai = tgl("2026-10-16"); return g }(), hariUji),
		"periode eksplisit":   kunciFilter("kpi", semuaAceh, func() domain.Filter { g := f; g.Periode = true; return g }(), hariUji),
		"hari ini":            kunciFilter("kpi", semuaAceh, f, hariUji.AddDate(0, 0, 1)),
		"kabupaten dan semua": kunciFilter("kpi", domain.Cakupan{Semua: true, KabupatenKode: "1171"}, f, hariUji),
	}
	for nama, k := range variasi {
		assert.NotEqual(t, dasar, k, "kunci harus berbeda untuk perbedaan pada: %s", nama)
	}
	assert.Equal(t, dasar, kunciFilter("kpi", semuaAceh, f, hariUji), "kunci deterministik")

	// Kunci saringan (ringkasan 08-B): cakupan, kabupaten, komoditas, titik, dan hari semuanya membedakan.
	saringan := domain.Saringan{KabupatenKode: "1171", KomoditasKode: "TMT"}
	dasarSaringan := kunciSaringan("stok", semuaAceh, saringan, hariUji)
	variasiSaringan := map[string]Kunci{
		"titik":             kunciSaringan("logistik", semuaAceh, saringan, hariUji),
		"cakupan kabupaten": kunciSaringan("stok", banda, saringan, hariUji),
		"cakupan tertutup":  kunciSaringan("stok", domain.Cakupan{}, saringan, hariUji),
		"kabupaten":         kunciSaringan("stok", semuaAceh, domain.Saringan{KabupatenKode: "1108", KomoditasKode: "TMT"}, hariUji),
		"komoditas":         kunciSaringan("stok", semuaAceh, domain.Saringan{KabupatenKode: "1171", KomoditasKode: "BWM"}, hariUji),
		"tanpa filter":      kunciSaringan("stok", semuaAceh, domain.Saringan{}, hariUji),
		"hari ini":          kunciSaringan("stok", semuaAceh, saringan, hariUji.AddDate(0, 0, 1)),
	}
	for nama, k := range variasiSaringan {
		assert.NotEqual(t, dasarSaringan, k, "kunci saringan harus berbeda untuk perbedaan pada: %s", nama)
	}
	assert.Equal(t, dasarSaringan, kunciSaringan("stok", semuaAceh, saringan, hariUji), "kunci deterministik")

	// Kunci cakupan (endpoint tanpa filter) juga membedakan cakupan dan hari.
	assert.NotEqual(t, kunciCakupan("cuaca", semuaAceh, hariUji), kunciCakupan("cuaca", banda, hariUji))
	assert.NotEqual(t, kunciCakupan("cuaca", banda, hariUji), kunciCakupan("cuaca", domain.CakupanKabupaten("1108"), hariUji))
	assert.NotEqual(t, kunciCakupan("cuaca", banda, hariUji), kunciCakupan("cuaca", banda, hariUji.AddDate(0, 0, 1)))
	assert.NotEqual(t, kunciCakupan("cuaca", banda, hariUji), kunciCakupan("aktivitas-lapangan", banda, hariUji))
}

func TestCache_SetiapParameterFilterMenghasilkanHasilTerpisah(t *testing.T) {
	repo := repoPerCakupan()
	uc := newUC(repo)
	req := func(ubah func(*domain.PermintaanFilter)) domain.PermintaanFilter {
		r := domain.PermintaanFilter{KabupatenKode: "1171", KomoditasKode: "TMT", Dari: "2026-10-10", Sampai: "2026-10-15"}
		ubah(&r)
		return r
	}
	variasi := []domain.PermintaanFilter{
		req(func(*domain.PermintaanFilter) {}),
		req(func(r *domain.PermintaanFilter) { r.KabupatenKode = "1108" }),
		req(func(r *domain.PermintaanFilter) { r.KabupatenKode = "" }),
		req(func(r *domain.PermintaanFilter) { r.KomoditasKode = "BWM" }),
		req(func(r *domain.PermintaanFilter) { r.KomoditasKode = "" }),
		req(func(r *domain.PermintaanFilter) { r.Dari = "2026-10-11" }),
		req(func(r *domain.PermintaanFilter) { r.Sampai = "2026-10-14" }),
	}

	for _, v := range variasi {
		_, err := uc.GetKpiOperasional(t.Context(), semuaAceh, v)
		require.NoError(t, err)
		_, err = uc.GetHeatmap(t.Context(), semuaAceh, v)
		require.NoError(t, err)
	}
	assert.Equal(t, len(variasi), repo.panggilan("DeretKpi"), "tiap kombinasi filter dihitung sendiri")
	assert.Equal(t, len(variasi), repo.panggilan("Heatmap"))

	// Mengulang kombinasi yang sama tidak menghitung ulang.
	for _, v := range variasi {
		_, _ = uc.GetKpiOperasional(t.Context(), semuaAceh, v)
		_, _ = uc.GetHeatmap(t.Context(), semuaAceh, v)
	}
	assert.Equal(t, len(variasi), repo.panggilan("DeretKpi"))
	assert.Equal(t, len(variasi), repo.panggilan("Heatmap"))
}

func TestCache_PerubahanHariMembuangHasilKemarin(t *testing.T) {
	for _, e := range endpoints {
		t.Run(e.nama, func(t *testing.T) {
			repo := repoPerCakupan()
			uc := newUC(repo, DenganCache(func(o *cache.Options) { o.TTL = 24 * time.Hour }))    // TTL tidak menolong: kunci harus beda
			uc.now = func() time.Time { return time.Date(2026, 10, 16, 16, 59, 0, 0, time.UTC) } // 16 Okt 23.59 WIB

			_, err := e.panggil(uc, banda, domain.PermintaanFilter{})
			require.NoError(t, err)
			_, _ = e.panggil(uc, banda, domain.PermintaanFilter{})
			require.Equal(t, 1, repo.panggilan(e.metode))

			uc.now = func() time.Time { return time.Date(2026, 10, 16, 17, 1, 0, 0, time.UTC) } // 17 Okt 00.01 WIB
			_, err = e.panggil(uc, banda, domain.PermintaanFilter{})
			require.NoError(t, err)

			assert.Equal(t, 2, repo.panggilan(e.metode), "hasil kemarin tidak boleh tersaji sebagai hasil hari ini")
		})
	}
}

func TestCache_GalatRepositoryTidakDisimpan(t *testing.T) {
	var gagal atomic.Bool
	gagal.Store(true)
	repo := repoPerCakupan()
	asli := repo.deretKpi
	repo.deretKpi = func(q domain.DeretKpiQuery) ([]domain.TitikKpi, error) {
		if gagal.Load() {
			return nil, errors.New("db mati")
		}
		return asli(q)
	}
	uc := newUC(repo)

	_, err := uc.GetKpiOperasional(t.Context(), semuaAceh, domain.PermintaanFilter{})
	require.Error(t, err)
	assert.Zero(t, uc.cache.Ukuran(), "galat tidak disimpan")

	gagal.Store(false)
	kpis, err := uc.GetKpiOperasional(t.Context(), semuaAceh, domain.PermintaanFilter{})
	require.NoError(t, err, "basis data pulih: permintaan berikutnya berhasil, tidak mengulang galat lama")
	assert.Equal(t, 1000.0, kartu(kpis, "entri").Nilai)
	assert.Equal(t, 2, repo.panggilan("DeretKpi"))
}

func TestCache_EpochBaruMembuangHasilSebelumPenyegaran(t *testing.T) {
	var epoch atomic.Uint64
	repo := repoPerCakupan()
	var nilai atomic.Int64
	nilai.Store(1)
	repo.deretKpi = func(q domain.DeretKpiQuery) ([]domain.TitikKpi, error) {
		return []domain.TitikKpi{{Indikator: domain.IndikatorEntri, Tanggal: q.Akhir, Pembilang: float64(nilai.Load())}}, nil
	}
	uc := newUC(repo, WithEpoch(epoch.Load))

	a, _ := uc.GetKpiOperasional(t.Context(), semuaAceh, domain.PermintaanFilter{})
	nilai.Store(2) // data berubah, tetapi belum ada penyegaran
	b, _ := uc.GetKpiOperasional(t.Context(), semuaAceh, domain.PermintaanFilter{})
	epoch.Add(1) // refresher selesai menyegarkan
	c, _ := uc.GetKpiOperasional(t.Context(), semuaAceh, domain.PermintaanFilter{})

	assert.Equal(t, 1.0, kartu(a, "entri").Nilai)
	assert.Equal(t, 1.0, kartu(b, "entri").Nilai, "sebelum penyegaran: hasil tersimpan masih berlaku")
	assert.Equal(t, 2.0, kartu(c, "entri").Nilai, "setelah penyegaran: langsung data baru, tanpa menunggu TTL")
}

func TestCache_PermintaanKembarHanyaMemuatSekali(t *testing.T) {
	repo := repoPerCakupan()
	lepas := make(chan struct{})
	asli := repo.deretKpi
	repo.deretKpi = func(q domain.DeretKpiQuery) ([]domain.TitikKpi, error) {
		<-lepas
		return asli(q)
	}
	uc := newUC(repo)

	var wg sync.WaitGroup
	for range 30 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			kpis, err := uc.GetKpiOperasional(t.Context(), banda, domain.PermintaanFilter{})
			assert.NoError(t, err)
			assert.Equal(t, 11.0, kartu(kpis, "entri").Nilai)
		}()
	}
	require.Eventually(t, func() bool { return repo.panggilan("DeretKpi") >= 1 }, 2*time.Second, time.Millisecond)
	time.Sleep(100 * time.Millisecond)
	close(lepas)
	wg.Wait()

	assert.Equal(t, 1, repo.panggilan("DeretKpi"), "30 permintaan kembar menunggu satu perhitungan")
}

func TestCache_TanpaCacheMenghitungTiapKali(t *testing.T) {
	repo := repoPerCakupan()
	uc := newUC(repo, TanpaCache())

	for range 3 {
		_, err := uc.GetCuaca(t.Context(), banda)
		require.NoError(t, err)
	}

	assert.Equal(t, 3, repo.panggilan("CuacaTerbaru"))
}

func TestCache_BatasJumlahEntri(t *testing.T) {
	repo := repoPerCakupan()
	uc := newUC(repo, DenganCache(func(o *cache.Options) { o.MaksEntri = 5 }))

	// Pengguna berhak mengarang filter sebanyak-banyaknya; memori cache tidak ikut tumbuh.
	for i := range 200 {
		_, err := uc.GetHeatmap(t.Context(), semuaAceh, domain.PermintaanFilter{KomoditasKode: fmt.Sprintf("K%03d", i)})
		require.NoError(t, err)
		require.LessOrEqual(t, uc.cache.Ukuran(), 5)
	}
}

func TestCache_TTLHabis(t *testing.T) {
	jam := time.Date(2026, 10, 16, 20, 0, 0, 0, time.UTC)
	var mu sync.Mutex
	repo := repoPerCakupan()
	uc := newUC(repo, DenganCache(func(o *cache.Options) {
		o.Now = func() time.Time { mu.Lock(); defer mu.Unlock(); return jam }
	}))

	_, _ = uc.GetCuaca(t.Context(), banda)
	mu.Lock()
	jam = jam.Add(TTLHasil - time.Second)
	mu.Unlock()
	_, _ = uc.GetCuaca(t.Context(), banda)
	assert.Equal(t, 1, repo.panggilan("CuacaTerbaru"), "dalam TTL")

	mu.Lock()
	jam = jam.Add(time.Second)
	mu.Unlock()
	_, _ = uc.GetCuaca(t.Context(), banda)
	assert.Equal(t, 2, repo.panggilan("CuacaTerbaru"), "TTL habis")
}

// TTL harus lebih pendek dari jeda polling UI (30 detik), kalau tidak pengguna yang polling sendirian menerima data
// basi bertumpuk.
func TestTTLHasilLebihPendekDariPollingUI(t *testing.T) {
	assert.Less(t, TTLHasil, 30*time.Second)
	assert.Positive(t, TTLHasil)
}
