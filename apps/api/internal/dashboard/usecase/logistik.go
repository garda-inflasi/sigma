package usecase

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/garda-inflasi/api/internal/dashboard/cache"
	"github.com/garda-inflasi/api/internal/dashboard/domain"
)

// GetLogistik merangkum pergerakan logistik dalam JendelaOperasionalHari hari terakhir (hari ini termasuk, waktu
// Aceh): tonase dan jumlah transaksi stok (MASUK, KELUAR, PINDAH) per jenis dan komoditas, transaksi terbaru, dan
// rute distribusi aktif.
//
// Cakupan baris: pergerakan masuk cakupan bila fasilitas asal ATAU tujuannya di kabupaten dalam cakupan (Koordinator di
// kabupaten tujuan melihat kiriman yang datang, yang di kabupaten asal melihat kiriman yang pergi); rute bila kabupaten
// asal ATAU tujuannya di sana. Satu PINDAH antarkabupaten karenanya tampak di kedua kabupaten, dan satu kali pada
// tampilan seluruh Aceh. Pelaku transaksi dan alasannya tidak pernah dikembalikan.
//
// Data dibaca langsung dari tabel, sama dengan GetStok soal keterkiniannya. Cakupan tertutup atau filter kabupaten di
// luar cakupan menghasilkan ringkasan nol tanpa menyentuh basis data.
func (uc *UseCase) GetLogistik(ctx context.Context, cakupan domain.Cakupan, req domain.PermintaanSaringan) (*domain.Logistik, error) {
	_, hari := uc.saatIni()
	dari := awalJendela(hari, domain.JendelaOperasionalHari)
	s, err := domain.NormalisasiSaringan(req)
	if err != nil {
		return nil, err
	}
	if _, ada := cakupan.Irisi(s.KabupatenKode); !ada {
		return RakitLogistik(dari, hari, domain.LogistikFakta{}), nil
	}

	return cache.Ambil(ctx, uc.cache, kunciSaringan("logistik", cakupan, s, hari), func(ctx context.Context) (*domain.Logistik, error) {
		fakta, err := uc.repo.Logistik(ctx, domain.LogistikQuery{
			Cakupan: cakupan, KabupatenKode: s.KabupatenKode, KomoditasKode: s.KomoditasKode,
			Awal: awalHariWIB(dari), Akhir: awalHariWIB(hari).AddDate(0, 0, 1),
			MaksTerbaru: domain.MaksPergerakanTerbaru, MaksRute: domain.MaksRuteDitampilkan,
		})
		if err != nil {
			uc.log.Error("dashboard: get logistik failed", map[string]interface{}{"error": err.Error()})
			return nil, fmt.Errorf("get logistik: %w", err)
		}
		return RakitLogistik(dari, hari, fakta), nil
	})
}

// RakitLogistik menyusun ringkasan dari fakta repository.
//
//   - Ringkasan dan baris per komoditas dijumlahkan dari fakta yang sama (tonase belum dibulatkan, lalu dibulatkan
//     satu desimal sekali), jadi keduanya selalu cocok. Ketiga jenis selalu ada, bernilai nol bila tidak ada transaksi.
//   - Komoditas diurutkan menurut tonase seluruh jenis (terbesar dulu), seri diputus kode.
//   - Tonase satu transaksi dibulatkan ke kilogram (3 desimal), bukan satu desimal: transaksi kecil tidak boleh
//     tampil 0,0 ton. Tonase satu rute sudah satu desimal di basis data.
func RakitLogistik(dari, sampai time.Time, f domain.LogistikFakta) *domain.Logistik {
	type jenis struct {
		jumlah int
		ton    float64
	}
	type baris struct {
		nama                  string
		masuk, keluar, pindah jenis
	}
	perKomoditas := map[string]*baris{}
	var masuk, keluar, pindah jenis

	for _, g := range f.Agregat {
		b := perKomoditas[g.KomoditasKode]
		if b == nil {
			b = &baris{nama: g.KomoditasNama}
			perKomoditas[g.KomoditasKode] = b
		}
		switch g.Jenis {
		case domain.JenisMasuk:
			b.masuk.jumlah, b.masuk.ton = b.masuk.jumlah+g.Jumlah, b.masuk.ton+g.VolumeTon
			masuk.jumlah, masuk.ton = masuk.jumlah+g.Jumlah, masuk.ton+g.VolumeTon
		case domain.JenisKeluar:
			b.keluar.jumlah, b.keluar.ton = b.keluar.jumlah+g.Jumlah, b.keluar.ton+g.VolumeTon
			keluar.jumlah, keluar.ton = keluar.jumlah+g.Jumlah, keluar.ton+g.VolumeTon
		case domain.JenisPindah:
			b.pindah.jumlah, b.pindah.ton = b.pindah.jumlah+g.Jumlah, b.pindah.ton+g.VolumeTon
			pindah.jumlah, pindah.ton = pindah.jumlah+g.Jumlah, pindah.ton+g.VolumeTon
		}
	}
	bulat := func(j jenis) domain.GerakanJenis {
		return domain.GerakanJenis{Jumlah: j.jumlah, VolumeTon: bulatkan(j.ton)}
	}

	kode := make([]string, 0, len(perKomoditas))
	for k := range perKomoditas {
		kode = append(kode, k)
	}
	total := func(b *baris) float64 { return b.masuk.ton + b.keluar.ton + b.pindah.ton }
	sort.Slice(kode, func(i, j int) bool {
		a, b := perKomoditas[kode[i]], perKomoditas[kode[j]]
		if ta, tb := total(a), total(b); ta != tb {
			return ta > tb
		}
		return kode[i] < kode[j]
	})
	komoditas := make([]domain.GerakanKomoditas, 0, len(kode))
	for _, k := range kode {
		b := perKomoditas[k]
		komoditas = append(komoditas, domain.GerakanKomoditas{
			KomoditasKode: k, KomoditasNama: b.nama,
			Masuk: bulat(b.masuk), Keluar: bulat(b.keluar), Pindah: bulat(b.pindah),
		})
	}

	terbaru := make([]domain.Pergerakan, len(f.Terbaru))
	for i, p := range f.Terbaru {
		p.VolumeTon = bulatkanKg(p.VolumeTon)
		terbaru[i] = p
	}
	// Urutan dari repository (waktu menurun, ID menurun) dijaga ulang di sini: tidak bergantung pada basis data.
	sort.SliceStable(terbaru, func(i, j int) bool {
		if !terbaru[i].Waktu.Equal(terbaru[j].Waktu) {
			return terbaru[i].Waktu.After(terbaru[j].Waktu)
		}
		return terbaru[i].ID > terbaru[j].ID
	})

	rute := domain.RuteRingkas{
		Jumlah: f.Rute.Jumlah, VolumeTon: bulatkan(f.Rute.VolumeTon), Biaya: f.Rute.Biaya,
		Items: make([]domain.RuteAktif, len(f.Rute.Items)),
	}
	copy(rute.Items, f.Rute.Items)
	sort.SliceStable(rute.Items, func(i, j int) bool {
		if rute.Items[i].Biaya != rute.Items[j].Biaya {
			return rute.Items[i].Biaya > rute.Items[j].Biaya
		}
		return rute.Items[i].ID < rute.Items[j].ID
	})

	return &domain.Logistik{
		Dari: dari, Sampai: sampai,
		Ringkasan: domain.RingkasanGerakan{Masuk: bulat(masuk), Keluar: bulat(keluar), Pindah: bulat(pindah)},
		Komoditas: komoditas, Terbaru: terbaru, Rute: rute,
	}
}
