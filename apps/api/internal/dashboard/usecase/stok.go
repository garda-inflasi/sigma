package usecase

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/garda-inflasi/api/internal/dashboard/cache"
	"github.com/garda-inflasi/api/internal/dashboard/domain"
)

// GetStok merangkum stok cold storage yang berlaku saat ini per komoditas, beserta kapasitas dan utilisasi, untuk
// fasilitas di kabupaten dalam cakupan pemanggil (dan filter kabupaten/komoditas bila diberikan).
//
// "Berlaku" memakai definisi KPI cs (migrasi 000124) dan modul cold-storage: lot yang belum HABIS di fasilitas
// OPERASIONAL, masuk paling lambat hari ini dan kedaluwarsa setelah hari ini menurut tanggal Aceh; lihat RakitStok
// untuk rekonsiliasinya dengan halaman cold storage.
//
// Data dibaca langsung dari tabel (bukan view), jadi keterkinian dibatasi TTL cache (TTLHasil, 20 detik) ditambah jeda
// polling UI (30 detik): paling tua sekitar 50 detik pada kasus terburuk, tipikalnya kurang dari satu menit.
// Cakupan tertutup atau filter kabupaten di luar cakupan menghasilkan stok nol tanpa menyentuh basis data.
func (uc *UseCase) GetStok(ctx context.Context, cakupan domain.Cakupan, req domain.PermintaanSaringan) (*domain.Stok, error) {
	_, hari := uc.saatIni()
	s, err := domain.NormalisasiSaringan(req)
	if err != nil {
		return nil, err
	}
	if _, ada := cakupan.Irisi(s.KabupatenKode); !ada {
		return RakitStok(hari, domain.FaktaStok{}), nil
	}

	return cache.Ambil(ctx, uc.cache, kunciSaringan("stok", cakupan, s, hari), func(ctx context.Context) (*domain.Stok, error) {
		// Tengah malam sesudah hari ini: batas lot berlaku (masuk < batas <= kedaluwarsa) dan, ditambah ambang, batas
		// mendekati kedaluwarsa.
		batas := awalHariWIB(hari).AddDate(0, 0, 1)
		fakta, err := uc.repo.FaktaStok(ctx, domain.StokQuery{
			Cakupan: cakupan, KabupatenKode: s.KabupatenKode, KomoditasKode: s.KomoditasKode,
			Batas: batas, BatasDekat: batas.AddDate(0, 0, domain.AmbangMendekatiKedaluwarsaHari),
		})
		if err != nil {
			uc.log.Error("dashboard: get stok failed", map[string]interface{}{"error": err.Error()})
			return nil, fmt.Errorf("get stok: %w", err)
		}
		return RakitStok(hari, fakta), nil
	})
}

// RakitStok menyusun ringkasan dan stok per komoditas dari fakta pada grain komoditas x fasilitas x kelas lot.
// Semua jumlah berasal dari fakta yang sama, jadi ringkasan dan per komoditas selalu cocok; tonase dijumlahkan dulu
// (tanpa pembulatan) lalu dibulatkan satu desimal sekali, jadi jumlah baris yang tampil boleh berselisih 0,1 dari
// total (aturan pembulatan KPI yang sama). Utilisasi dihitung dari tonase dan kapasitas yang belum dibulatkan, sama
// dengan kartu cs.
//
// Rekonsiliasi dengan halaman cold storage, yang menghitung semua lot yang belum habis di semua fasilitas:
// TotalTon + KedaluwarsaTon + DiLuarOperasionalTon = total volume di sana (kelas BELUM_MASUK, lot yang masuknya
// setelah hari ini, tidak ikut karena tidak ada pada data wajar).
func RakitStok(hari time.Time, f domain.FaktaStok) *domain.Stok {
	type agregat struct {
		nama               string
		lot                int
		ton, dekat, risiko float64
		fasilitas          map[string]struct{}
	}
	perKomoditas := map[string]*agregat{}
	fasilitasBerlaku := map[string]struct{}{}

	var r domain.RingkasanStok
	var total, dekat, risiko, kedaluwarsa, luar float64
	for _, b := range f.Baris {
		switch b.Kelas {
		case domain.KelasStokBerlaku:
			a := perKomoditas[b.KomoditasKode]
			if a == nil {
				a = &agregat{nama: b.KomoditasNama, fasilitas: map[string]struct{}{}}
				perKomoditas[b.KomoditasKode] = a
			}
			a.lot += b.JumlahLot
			a.ton += b.VolumeTon
			a.dekat += b.MendekatiTon
			a.risiko += b.RisikoTinggiTon
			a.fasilitas[b.FasilitasID] = struct{}{}

			r.JumlahLot += b.JumlahLot
			total += b.VolumeTon
			dekat += b.MendekatiTon
			risiko += b.RisikoTinggiTon
			fasilitasBerlaku[b.FasilitasID] = struct{}{}
		case domain.KelasStokKedaluwarsa:
			r.KedaluwarsaLot += b.JumlahLot
			kedaluwarsa += b.VolumeTon
		case domain.KelasStokLuarOperasional:
			luar += b.VolumeTon
		}
	}

	r.TotalTon = bulatkan(total)
	r.JumlahFasilitas = len(fasilitasBerlaku)
	r.KapasitasTon = f.Kapasitas.Ton
	r.FasilitasOperasional = f.Kapasitas.Fasilitas
	r.UtilisasiPct = bulatkan(persen(total, float64(f.Kapasitas.Ton)))
	r.MendekatiKedaluwarsaTon = bulatkan(dekat)
	r.RisikoTinggiTon = bulatkan(risiko)
	r.KedaluwarsaTon = bulatkan(kedaluwarsa)
	r.DiLuarOperasionalTon = bulatkan(luar)

	kode := make([]string, 0, len(perKomoditas))
	for k := range perKomoditas {
		kode = append(kode, k)
	}
	// Terbesar lebih dulu menurut tonase yang belum dibulatkan; kode memutus seri supaya urutan selalu sama.
	sort.Slice(kode, func(i, j int) bool {
		a, b := perKomoditas[kode[i]], perKomoditas[kode[j]]
		if a.ton != b.ton {
			return a.ton > b.ton
		}
		return kode[i] < kode[j]
	})
	komoditas := make([]domain.StokKomoditas, 0, len(kode))
	for _, k := range kode {
		a := perKomoditas[k]
		komoditas = append(komoditas, domain.StokKomoditas{
			KomoditasKode: k, KomoditasNama: a.nama,
			TotalTon: bulatkan(a.ton), JumlahLot: a.lot, JumlahFasilitas: len(a.fasilitas),
			MendekatiKedaluwarsaTon: bulatkan(a.dekat), RisikoTinggiTon: bulatkan(a.risiko),
		})
	}
	return &domain.Stok{Tanggal: hari, Ringkasan: r, Komoditas: komoditas}
}
