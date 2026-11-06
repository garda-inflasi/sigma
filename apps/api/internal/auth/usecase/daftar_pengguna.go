package usecase

import (
	"context"
	"fmt"

	"github.com/garda-inflasi/api/internal/auth/domain"
)

// peranDikenal melaporkan apakah peran ada di katalog peran backend (enum peran, 14 nilai). Katalog peran
// tetap didefinisikan di kode (rbac_katalog.go); hanya matriks izinnya yang tinggal di basis data (02-C).
func peranDikenal(peran string) bool { return PeranDikenal(peran) }

// ListUsers mengembalikan satu halaman pengguna beserta jumlah seluruh yang cocok dengan filter. Seluruh
// masukan divalidasi di sini (domain.DaftarPenggunaQuery.Normal + peran terhadap katalog): galatnya
// *domain.FilterPenggunaError, yang HTTP jawab 400. Ekspor CSV memakai pintu yang sama supaya filternya identik.
func (uc *UseCase) ListUsers(ctx context.Context, q domain.DaftarPenggunaQuery) ([]domain.Pengguna, int64, error) {
	q, err := q.Normal()
	if err != nil {
		return nil, 0, err
	}
	if q.Filter.Peran != "" && !peranDikenal(q.Filter.Peran) {
		return nil, 0, &domain.FilterPenggunaError{Param: "peran", Pesan: fmt.Sprintf("peran tidak dikenal: %q", q.Filter.Peran)}
	}
	if q.Sekarang.IsZero() {
		q.Sekarang = jam()
	}
	return uc.repo.ListUsers(ctx, q)
}
