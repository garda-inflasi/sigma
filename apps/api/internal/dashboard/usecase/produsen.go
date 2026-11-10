package usecase

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/garda-inflasi/api/internal/dashboard/cache"
	"github.com/garda-inflasi/api/internal/dashboard/domain"
)

// GetProdusen merangkum harga di tingkat produsen per komoditas dalam JendelaOperasionalHari hari terakhir (hari ini
// termasuk, waktu Aceh), untuk kabupaten dalam cakupan pemanggil (dan filter kabupaten/komoditas bila diberikan):
// harga produsen, harga konsumen pembanding pada jendela yang sama, margin keduanya, dan produksi neraca pangan
// periode terakhir. Hanya entri DIVALIDASI dan DIPUBLIKASI yang dihitung, sama dengan agregat_harga_harian.
//
// Cakupan tertutup atau filter kabupaten di luar cakupan menghasilkan daftar kosong tanpa menyentuh basis data.
func (uc *UseCase) GetProdusen(ctx context.Context, cakupan domain.Cakupan, req domain.PermintaanSaringan) (*domain.Produsen, error) {
	_, hari := uc.saatIni()
	dari := awalJendela(hari, domain.JendelaOperasionalHari)
	s, err := domain.NormalisasiSaringan(req)
	if err != nil {
		return nil, err
	}
	if _, ada := cakupan.Irisi(s.KabupatenKode); !ada {
		return RakitProdusen(dari, hari, nil), nil
	}

	return cache.Ambil(ctx, uc.cache, kunciSaringan("produsen", cakupan, s, hari), func(ctx context.Context) (*domain.Produsen, error) {
		fakta, err := uc.repo.Produsen(ctx, domain.ProdusenQuery{
			Cakupan: cakupan, KabupatenKode: s.KabupatenKode, KomoditasKode: s.KomoditasKode,
			Dari: dari, Sampai: hari,
		})
		if err != nil {
			uc.log.Error("dashboard: get produsen failed", map[string]interface{}{"error": err.Error()})
			return nil, fmt.Errorf("get produsen: %w", err)
		}
		return RakitProdusen(dari, hari, fakta), nil
	})
}

// RakitProdusen menyusun baris per komoditas dari fakta repository.
//
//   - Harga dibulatkan ke rupiah penuh dan margin dihitung dari harga yang SUDAH dibulatkan, jadi angka yang tampil
//     selalu saling menjumlah (konsumen - produsen = selisih) dan tidak berselisih Rp 1 karena pembulatan.
//   - Margin hanya ada bila harga produsen dan konsumen sama-sama ada dan positif. Persen terhadap harga produsen
//     (markup) dan terhadap harga konsumen (definisi EKO-MARGIN di modul KPI) dua-duanya disediakan.
//   - Komoditas pilot lebih dulu, lalu menurut nama; seri diputus kode.
func RakitProdusen(dari, sampai time.Time, fakta []domain.ProdusenFakta) *domain.Produsen {
	komoditas := make([]domain.ProdusenKomoditas, 0, len(fakta))
	pilot := map[string]bool{}
	for _, f := range fakta {
		pilot[f.KomoditasKode] = f.Pilot
		k := domain.ProdusenKomoditas{KomoditasKode: f.KomoditasKode, KomoditasNama: f.KomoditasNama, Satuan: f.Satuan}
		if f.HargaProdusen != nil {
			k.Produsen = &domain.SisiHarga{Harga: bulatkanRupiah(*f.HargaProdusen), JumlahEntri: f.EntriProdusen, JumlahKabupaten: f.KabProdusen}
		}
		if f.HargaKonsumen != nil {
			k.Konsumen = &domain.SisiHarga{Harga: bulatkanRupiah(*f.HargaKonsumen), JumlahEntri: f.EntriKonsumen, JumlahKabupaten: f.KabKonsumen}
		}
		if k.Produsen != nil && k.Konsumen != nil && k.Produsen.Harga > 0 && k.Konsumen.Harga > 0 {
			selisih := k.Konsumen.Harga - k.Produsen.Harga
			k.Margin = &domain.Margin{
				Selisih:        selisih,
				PersenProdusen: bulatkan(100 * selisih / k.Produsen.Harga),
				PersenKonsumen: bulatkan(100 * selisih / k.Konsumen.Harga),
			}
		}
		if p := f.Produksi(); p != nil {
			p.Ton = bulatkan(p.Ton)
			k.Produksi = p
		}
		komoditas = append(komoditas, k)
	}
	sort.SliceStable(komoditas, func(i, j int) bool {
		a, b := komoditas[i], komoditas[j]
		if pilot[a.KomoditasKode] != pilot[b.KomoditasKode] {
			return pilot[a.KomoditasKode]
		}
		if a.KomoditasNama != b.KomoditasNama {
			return kurang(a.KomoditasNama, b.KomoditasNama)
		}
		return a.KomoditasKode < b.KomoditasKode
	})
	return &domain.Produsen{Dari: dari, Sampai: sampai, Komoditas: komoditas}
}
