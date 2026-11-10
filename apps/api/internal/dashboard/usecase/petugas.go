package usecase

import (
	"context"
	"fmt"
	"sort"

	"github.com/garda-inflasi/api/internal/dashboard/cache"
	"github.com/garda-inflasi/api/internal/dashboard/domain"
)

// GetAktivitasPetugas merangkum aktivitas tiap petugas lapangan aktif (mahasiswa agen) yang kabupaten tugasnya dalam
// cakupan pemanggil, dan filter kabupaten bila diberikan: entri hari ini terhadap target harian, antrean validasi,
// penolakan sepekan terakhir, entri terakhir, dan tugas lapangan (lihat domain.AktivitasPetugas). Hanya nama tampilan
// dan ID publik petugas yang dikembalikan, tanpa email atau telepon.
//
// Urutan tetap: nama kabupaten, nama petugas, lalu ID. Petugas tanpa aktivitas tetap tampil dengan nilai nol (itu
// justru informasi). Parameter yang tidak valid dijawab ErrValidasi; cakupan tertutup atau kabupaten di luar cakupan
// menghasilkan daftar kosong tanpa menyentuh basis data.
func (uc *UseCase) GetAktivitasPetugas(ctx context.Context, cakupan domain.Cakupan, kabupatenKode string) (*domain.AktivitasPetugasHasil, error) {
	sekarang, hari := uc.saatIni()
	s, err := domain.NormalisasiSaringan(domain.PermintaanSaringan{KabupatenKode: kabupatenKode})
	if err != nil {
		return nil, err
	}
	if _, ada := cakupan.Irisi(s.KabupatenKode); !ada {
		return &domain.AktivitasPetugasHasil{Tanggal: hari, Petugas: []domain.AktivitasPetugas{}}, nil
	}

	return cache.Ambil(ctx, uc.cache, kunciSaringan("aktivitas-petugas", cakupan, s, hari), func(ctx context.Context) (*domain.AktivitasPetugasHasil, error) {
		awal := awalHariWIB(hari)
		items, err := uc.repo.AktivitasPetugas(ctx, domain.AktivitasPetugasQuery{
			Cakupan: cakupan, KabupatenKode: s.KabupatenKode,
			AwalHari: awal, AkhirHari: awal.AddDate(0, 0, 1),
			AwalJendela: awalHariWIB(awalJendela(hari, domain.JendelaOperasionalHari)),
			Sekarang:    sekarang,
		})
		if err != nil {
			uc.log.Error("dashboard: get aktivitas petugas failed", map[string]interface{}{"error": err.Error()})
			return nil, fmt.Errorf("get aktivitas petugas: %w", err)
		}
		return &domain.AktivitasPetugasHasil{Tanggal: hari, Petugas: urutkanPetugas(items)}, nil
	})
}

// urutkanPetugas mengurutkan menurut nama kabupaten, nama petugas, lalu ID, dan mengembalikan irisan tidak-nil.
func urutkanPetugas(items []domain.AktivitasPetugas) []domain.AktivitasPetugas {
	out := make([]domain.AktivitasPetugas, len(items))
	copy(out, items)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.KabupatenNama != b.KabupatenNama {
			return kurang(a.KabupatenNama, b.KabupatenNama)
		}
		if a.Nama != b.Nama {
			return kurang(a.Nama, b.Nama)
		}
		return a.ID < b.ID
	})
	return out
}
