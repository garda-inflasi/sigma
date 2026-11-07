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
	// perubahan (opsional) menghubungkan aksi massal dengan antrean usulan (04-B); nil = hanya status.
	perubahan domain.RepositoryPerubahan
}

func New(repo domain.Repository, auditWriter audit.Writer, log *logger.Logger) *UseCase {
	return &UseCase{repo: repo, audit: auditWriter, log: log}
}

// DenganPerubahan menghubungkan aksi massal dengan antrean usulan: ajukan membuat usulan TINJAU, setujui
// menerapkan usulan yang menunggu (pemisahan tugas berlaku), arsipkan menutup usulan yang menunggu.
func (uc *UseCase) DenganPerubahan(repo domain.RepositoryPerubahan) *UseCase {
	uc.perubahan = repo
	return uc
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
			pesan, err := uc.jalankanBulk(ctx, domainAction, transisi, k, actorID)
			if err != nil {
				return hasil, fmt.Errorf("bulk komoditas: %w", err)
			}
			item.Berhasil, item.Pesan = pesan == "", pesan
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

// pesanBalapan dipakai bila status berubah di antara pemeriksaan dan pembaruan.
const pesanBalapan = "Status berubah saat diproses; muat ulang lalu coba lagi"

// jalankanBulk menerapkan satu aksi massal pada satu kode. Pesan kosong berarti berhasil; galat hanya untuk
// kegagalan sistem.
func (uc *UseCase) jalankanBulk(ctx context.Context, aksi domain.BulkAction, transisi domain.Transisi, kode, aktorID string) (string, error) {
	gagal := func(err error) (string, error) {
		if pesan, ok := PesanItemGagal(err); ok {
			return pesan, nil
		}
		return "", err
	}
	if uc.perubahan != nil {
		switch aksi {
		case domain.BulkAjukanPerubahan:
			_, err := uc.perubahan.Ajukan(ctx, domain.UsulanBaru{
				Entitas: domain.EntitasKomoditas, Jenis: domain.JenisTinjau, EntitasID: kode,
				Perubahan: map[string]any{}, Alasan: "Diajukan lewat aksi massal", AktorID: aktorID,
			})
			if err != nil {
				return gagal(err)
			}
			return "", nil
		case domain.BulkSetujui:
			_, err := uc.perubahan.PutuskanMenunggu(ctx, domain.EntitasKomoditas, kode,
				domain.Keputusan{Aksi: domain.KeputusanSetujui, AktorID: aktorID})
			if err == nil {
				return "", nil
			}
			if !errors.Is(err, domain.ErrUsulanTidakAda) {
				return gagal(err)
			}
			// Status menunggu tanpa baris usulan (data sebelum 04-B): setujui statusnya saja.
		case domain.BulkArsipkan:
			ok, err := uc.perubahan.ArsipkanKomoditas(ctx, kode, aktorID)
			if err != nil {
				return "", err
			}
			if !ok {
				return pesanBalapan, nil
			}
			return "", nil
		}
	}
	changed, err := uc.repo.UbahStatusKomoditas(ctx, kode, transisi)
	if err != nil {
		return "", err
	}
	if !changed {
		return pesanBalapan, nil
	}
	return "", nil
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
