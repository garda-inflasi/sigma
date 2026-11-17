package usecase

import (
	"context"
	"errors"
	"fmt"

	hargadomain "github.com/garda-inflasi/api/internal/harga/domain"
	"github.com/garda-inflasi/api/internal/intervensi/domain"
	"github.com/garda-inflasi/api/pkg/idx"
)

// alasanDiLuarCakupan sama dengan kartu yang tidak ada: pemanggil tidak bisa membedakannya, jadi keberadaan
// kartu di wilayah lain tidak bocor (pola 02-B).
const alasanDiLuarCakupan = "Kartu tidak ditemukan"

// cakupanAktor membaca peran dan kabupaten tugas aktor dari basis data. Pengguna yang tidak ada diperlakukan
// tanpa cakupan sama sekali (gagal-tutup).
func (uc *UseCase) cakupanAktor(ctx context.Context, aktorID string) (hargadomain.Cakupan, error) {
	pengguna, err := uc.cakupan.PenggunaCakupan(ctx, aktorID)
	if errors.Is(err, domain.ErrPenggunaTidakAda) {
		return hargadomain.Cakupan{}, nil
	}
	if err != nil {
		uc.log.Error("intervensi: baca cakupan gagal", map[string]interface{}{"error": err.Error()})
		return hargadomain.Cakupan{}, fmt.Errorf("baca cakupan: %w", err)
	}
	// PemilikID sengaja kosong: kartu tidak punya pencatat, jadi hanya kabupaten yang membuka cakupan bagi
	// Koordinator dan Agregator (kartu tingkat provinsi tertutup bagi mereka).
	c := hargadomain.CakupanPeran(pengguna.Peran, pengguna.KabupatenKode, "")
	return c, nil
}

// dalamCakupan melaporkan apakah kartu di kabupatenKode masuk cakupan aktor.
func dalamCakupan(c hargadomain.Cakupan, kabupatenKode string) bool {
	return c.Mengizinkan(kabupatenKode, nil)
}

// kartuDalamCakupan mengambil kartu dan memastikan ia masuk cakupan wilayah aktor; di luar cakupan dijawab
// ErrNotFound, sama dengan kartu yang tidak ada.
func (uc *UseCase) kartuDalamCakupan(ctx context.Context, ref idx.RujukanUUID, aktorID string) (*domain.Intervensi, error) {
	card, err := uc.kartu(ctx, ref)
	if err != nil {
		return nil, err
	}
	c, err := uc.cakupanAktor(ctx, aktorID)
	if err != nil {
		return nil, err
	}
	if !dalamCakupan(c, card.KabupatenKode) {
		return nil, ErrNotFound
	}
	return card, nil
}

// saringCakupan memisahkan id kartu yang boleh diproses aktor dari yang di luar cakupannya. Aktor tingkat
// provinsi ke atas tidak memicu pembacaan kartu sama sekali.
func (uc *UseCase) saringCakupan(ctx context.Context, ids []string, aktorID string) (diproses []string, luar map[string]bool, err error) {
	c, err := uc.cakupanAktor(ctx, aktorID)
	if err != nil {
		return nil, nil, err
	}
	if c.Semua {
		return ids, nil, nil
	}
	luar = map[string]bool{}
	for _, id := range ids {
		card, err := uc.repo.GetByID(ctx, idx.RujukanUUID{ID: id})
		if err != nil || !dalamCakupan(c, card.KabupatenKode) {
			// Tidak ada atau di luar cakupan: sama-sama dilaporkan tidak ditemukan. Galat basis data yang
			// sebenarnya muncul lagi saat repository memproses kartu yang lolos.
			luar[id] = true
			continue
		}
		diproses = append(diproses, id)
	}
	return diproses, luar, nil
}

// gabungHasil menyusun hasil per kartu sesuai urutan ids: kartu di luar cakupan dilaporkan tidak ditemukan.
func gabungHasil(ids []string, hasil []domain.HasilKartu, luar map[string]bool) []domain.HasilKartu {
	if len(luar) == 0 {
		return hasil
	}
	out := make([]domain.HasilKartu, 0, len(ids))
	i := 0
	for _, id := range ids {
		if luar[id] {
			out = append(out, domain.HasilKartu{ID: id, Alasan: alasanDiLuarCakupan})
			continue
		}
		out = append(out, hasil[i])
		i++
	}
	return out
}
