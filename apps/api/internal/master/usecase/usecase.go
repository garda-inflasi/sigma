// Package usecase implements the business logic for the master module.
package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/master/domain"
	"github.com/garda-inflasi/api/pkg/logger"
)

var ErrInvalidAction = errors.New("aksi tidak valid")

type UseCase struct {
	repo  domain.Repository
	audit audit.Writer
	log   *logger.Logger
}

func New(repo domain.Repository, auditWriter audit.Writer, log *logger.Logger) *UseCase {
	return &UseCase{repo: repo, audit: auditWriter, log: log}
}

func (uc *UseCase) ListKomoditas(ctx context.Context, q domain.ListQuery) ([]domain.Komoditas, int64, error) {
	return uc.repo.ListKomoditas(ctx, q)
}

func (uc *UseCase) ListWilayah(ctx context.Context, provinsiKode string, q domain.ListQuery) ([]domain.Kabupaten, int64, error) {
	return uc.repo.ListWilayah(ctx, provinsiKode, q)
}

func (uc *UseCase) ListPasar(ctx context.Context, kabupatenKode string, q domain.ListQuery) ([]domain.Pasar, int64, error) {
	return uc.repo.ListPasar(ctx, kabupatenKode, q)
}

// BulkKomoditas menjalankan aksi massal pada komoditas. Tiap kode dinilai sendiri:
// kode tak dikenal atau status yang tak boleh bertransisi menjadi item gagal beserta
// alasannya, tanpa menggagalkan item lain. Kode ganda hanya diproses sekali.
// Izin per aksi diperiksa pemanggil (lapisan delivery).
func (uc *UseCase) BulkKomoditas(ctx context.Context, kode []string, action, actorID string) (domain.HasilBulk, error) {
	domainAction := domain.BulkAction(action)
	transisi, ok := domain.TransisiUntuk(domainAction)
	if !ok {
		return domain.HasilBulk{}, ErrInvalidAction
	}

	unik := uniq(kode)
	statusSaatIni, err := uc.repo.StatusKomoditas(ctx, unik)
	if err != nil {
		return domain.HasilBulk{}, fmt.Errorf("bulk komoditas: %w", err)
	}

	hasil := domain.HasilBulk{Rincian: make([]domain.RincianBulk, 0, len(unik))}
	var berhasil, gagal []string
	for _, k := range unik {
		item := domain.RincianBulk{Kode: k}
		status, ada := statusSaatIni[k]
		switch {
		case !ada:
			item.Pesan = "Komoditas tidak ditemukan"
		case !transisi.Boleh(status):
			item.Pesan = fmt.Sprintf("Status %s tidak sesuai: %s", status, transisi.Syarat)
		default:
			changed, err := uc.repo.UbahStatusKomoditas(ctx, k, transisi)
			if err != nil {
				return hasil, fmt.Errorf("bulk komoditas: %w", err)
			}
			if changed {
				item.Berhasil = true
			} else {
				item.Pesan = "Status berubah saat diproses; muat ulang lalu coba lagi"
			}
		}
		if item.Berhasil {
			hasil.Berhasil++
			berhasil = append(berhasil, k)
		} else {
			hasil.Gagal++
			gagal = append(gagal, k)
		}
		hasil.Rincian = append(hasil.Rincian, item)
	}

	if hasil.Berhasil > 0 {
		err = uc.audit.Write(ctx, audit.Entry{
			AktorID: actorID, Aksi: "BULK_" + action, Entitas: "komoditas", EntitasID: fmt.Sprintf("%d komoditas", hasil.Berhasil),
			Hasil: audit.HasilBerhasil, Metadata: map[string]any{"kode": berhasil, "gagal": gagal},
		})
		if err != nil {
			uc.log.Warn("master: gagal mencatat audit", map[string]interface{}{"aksi": action, "error": err.Error()})
		}
	}
	return hasil, nil
}

// uniq membuang kode kosong dan ganda dengan mempertahankan urutan.
func uniq(kode []string) []string {
	seen := make(map[string]struct{}, len(kode))
	out := make([]string, 0, len(kode))
	for _, k := range kode {
		if k == "" {
			continue
		}
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	return out
}
