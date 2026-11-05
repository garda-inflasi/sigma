// Package usecase berisi logika bisnis modul intervensi.
package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/intervensi/domain"
	"github.com/garda-inflasi/api/pkg/logger"
	"gorm.io/gorm"
)

var (
	ErrNotFound         = domain.ErrNotFound
	ErrInvalidReference = domain.ErrInvalidReference
	// ErrInvalidTahap: tahap tidak terdefinisi untuk siklus kartu.
	ErrInvalidTahap = errors.New("tahap tidak valid untuk siklus ini")
)

// ForbiddenError: pemanggil tidak memiliki izin yang disyaratkan tahap tujuan.
type ForbiddenError struct{ Izin string }

func (e *ForbiddenError) Error() string {
	return "tahap tujuan memerlukan izin " + e.Izin
}

type UseCase struct {
	repo  domain.Repository
	audit audit.Writer
	log   *logger.Logger
}

func New(repo domain.Repository, auditWriter audit.Writer, log *logger.Logger) *UseCase {
	return &UseCase{repo: repo, audit: auditWriter, log: log}
}

func (uc *UseCase) List(ctx context.Context, q domain.ListQuery) ([]domain.Kartu, error) {
	cards, err := uc.repo.List(ctx, q)
	if err != nil {
		uc.log.Error("intervensi: daftar gagal", map[string]interface{}{"error": err.Error()})
		return nil, err
	}
	return cards, nil
}

func (uc *UseCase) ListTahap(ctx context.Context) ([]domain.TahapDef, error) {
	defs, err := uc.repo.ListTahap(ctx)
	if err != nil {
		uc.log.Error("intervensi: daftar tahap gagal", map[string]interface{}{"error": err.Error()})
		return nil, err
	}
	return defs, nil
}

// Create membuat kartu pada tahap pertama siklusnya dan mencatatnya di audit.
func (uc *UseCase) Create(ctx context.Context, in domain.NewIntervensi) (*domain.Intervensi, error) {
	defs, err := uc.repo.ListTahap(ctx)
	if err != nil {
		return nil, err
	}
	first, ok := tahapAwal(defs, in.Siklus)
	if !ok {
		return nil, ErrInvalidTahap
	}

	created, err := uc.repo.Create(ctx, in, first.KodeTahap)
	if err != nil {
		if !errors.Is(err, ErrInvalidReference) {
			uc.log.Error("intervensi: buat gagal", map[string]interface{}{"error": err.Error()})
		}
		return nil, err
	}

	uc.writeAudit(ctx, audit.Entry{
		AktorID: in.DibuatOlehID, Aksi: "BUAT", Entitas: "intervensi", EntitasID: created.ID, Hasil: audit.HasilBerhasil,
		Sesudah: map[string]any{"siklus": created.Siklus, "tahap": created.Tahap, "judul": created.Judul, "prioritas": created.Prioritas},
	})
	return created, nil
}

// Move memindahkan kartu ke tahap dan posisi baru. Tahap harus terdefinisi untuk
// siklus kartu, dan hasPermission harus memenuhi izin yang disyaratkan tahap itu.
func (uc *UseCase) Move(ctx context.Context, id, tahap string, index int, actorID string, hasPermission func(string) bool) error {
	card, err := uc.repo.GetByID(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	defs, err := uc.repo.ListTahap(ctx)
	if err != nil {
		return err
	}
	target, final, ok := cariTahap(defs, card.Siklus, tahap)
	if !ok {
		return ErrInvalidTahap
	}
	if !hasPermission(target.IzinDibutuhkan) {
		uc.writeAudit(ctx, audit.Entry{
			AktorID: actorID, Aksi: "PINDAH_TAHAP", Entitas: "intervensi", EntitasID: id, Hasil: audit.HasilDitolak,
			Metadata: map[string]any{"tahap": tahap, "izin": target.IzinDibutuhkan},
		})
		return &ForbiddenError{Izin: target.IzinDibutuhkan}
	}

	if err := uc.repo.Move(ctx, id, tahap, index, final); err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrNotFound
		}
		uc.log.Error("intervensi: pindah tahap gagal", map[string]interface{}{"id": id, "tahap": tahap, "error": err.Error()})
		return fmt.Errorf("pindah tahap: %w", err)
	}

	uc.writeAudit(ctx, audit.Entry{
		AktorID: actorID, Aksi: "PINDAH_TAHAP", Entitas: "intervensi", EntitasID: id, Hasil: audit.HasilBerhasil,
		Sebelum: map[string]any{"tahap": card.Tahap, "urutan": card.Urutan},
		Sesudah: map[string]any{"tahap": tahap, "urutan": index},
	})
	return nil
}

func (uc *UseCase) writeAudit(ctx context.Context, entry audit.Entry) {
	if err := uc.audit.Write(ctx, entry); err != nil {
		uc.log.Warn("intervensi: gagal mencatat audit", map[string]interface{}{"aksi": entry.Aksi, "error": err.Error()})
	}
}

// tahapAwal adalah tahap berurutan terkecil pada siklus.
func tahapAwal(defs []domain.TahapDef, siklus string) (domain.TahapDef, bool) {
	var first domain.TahapDef
	found := false
	for _, d := range defs {
		if d.Siklus == siklus && (!found || d.Urutan < first.Urutan) {
			first, found = d, true
		}
	}
	return first, found
}

// cariTahap mengembalikan definisi tahap kode pada siklus dan apakah itu tahap
// terakhir (penutup) siklus.
func cariTahap(defs []domain.TahapDef, siklus, kode string) (def domain.TahapDef, final, ok bool) {
	last := 0
	for _, d := range defs {
		if d.Siklus != siklus {
			continue
		}
		last = max(last, d.Urutan)
		if d.KodeTahap == kode {
			def, ok = d, true
		}
	}
	return def, ok && def.Urutan == last, ok
}
