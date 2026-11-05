// Package usecase berisi logika bisnis modul intervensi.
package usecase

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/intervensi/domain"
	"github.com/garda-inflasi/api/pkg/logger"
	"gorm.io/gorm"
)

var (
	ErrNotFound         = domain.ErrNotFound
	ErrInvalidReference = domain.ErrInvalidReference
	ErrVersiUsang       = domain.ErrVersiUsang
	// ErrInvalidTahap: tahap tidak terdefinisi untuk siklus kartu.
	ErrInvalidTahap = domain.ErrTahapTidakValid
	// ErrInvalidAction: aksi massal tidak dikenal.
	ErrInvalidAction = errors.New("aksi tidak valid")
)

// ForbiddenError: pemanggil tidak memiliki izin yang disyaratkan. Tahap kosong berarti
// tahap tujuan; terisi berarti tahap yang dilewati (label tahap) yang tidak boleh dilompati.
type ForbiddenError struct {
	Izin  string
	Tahap string
}

func (e *ForbiddenError) Error() string {
	if e.Tahap != "" {
		return fmt.Sprintf("melewati tahap %q memerlukan izin %s", e.Tahap, e.Izin)
	}
	return "tahap tujuan memerlukan izin " + e.Izin
}

// BulkResult adalah hasil aksi massal per kartu (sesuai urutan id yang diminta).
type BulkResult struct {
	Berhasil, Gagal int
	Hasil           []domain.HasilKartu
}

type UseCase struct {
	repo  domain.Repository
	wf    domain.WorkflowRepository
	audit audit.Writer
	log   *logger.Logger
}

func New(repo domain.Repository, wf domain.WorkflowRepository, auditWriter audit.Writer, log *logger.Logger) *UseCase {
	return &UseCase{repo: repo, wf: wf, audit: auditWriter, log: log}
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

// Move memindahkan kartu ke tahap dan posisi baru dan mengembalikan versi baru kartu.
// versi harus sama dengan versi yang dibaca klien (ErrVersiUsang bila tidak). Tahap harus
// terdefinisi untuk siklus kartu, hasPermission harus memenuhi izin tahap tujuan, dan
// pemindahan maju tidak boleh melompati tahap persetujuan yang izinnya tidak dimiliki
// (persetujuan berjenjang).
func (uc *UseCase) Move(ctx context.Context, id, tahap string, index, versi int, actorID string, hasPermission func(string) bool) (int, error) {
	card, err := uc.repo.GetByID(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	defs, err := uc.repo.ListTahap(ctx)
	if err != nil {
		return 0, err
	}
	target, final, ok := cariTahap(defs, card.Siklus, tahap)
	if !ok {
		return 0, ErrInvalidTahap
	}
	if forbidden := izinPemindahan(defs, card, target, hasPermission); forbidden != nil {
		uc.writeAudit(ctx, audit.Entry{
			AktorID: actorID, Aksi: "PINDAH_TAHAP", Entitas: "intervensi", EntitasID: id, Hasil: audit.HasilDitolak,
			Metadata: map[string]any{"tahap": tahap, "izin": forbidden.Izin, "dilewati": forbidden.Tahap},
		})
		return 0, forbidden
	}

	versiBaru, err := uc.repo.Move(ctx, id, tahap, index, final, versi, actorID)
	if err != nil {
		if errors.Is(err, ErrNotFound) || errors.Is(err, ErrVersiUsang) {
			return 0, err
		}
		uc.log.Error("intervensi: pindah tahap gagal", map[string]interface{}{"id": id, "tahap": tahap, "error": err.Error()})
		return 0, fmt.Errorf("pindah tahap: %w", err)
	}

	uc.writeAudit(ctx, audit.Entry{
		AktorID: actorID, Aksi: "PINDAH_TAHAP", Entitas: "intervensi", EntitasID: id, Hasil: audit.HasilBerhasil,
		Sebelum: map[string]any{"tahap": card.Tahap, "urutan": card.Urutan, "versi": versi},
		Sesudah: map[string]any{"tahap": tahap, "urutan": index, "versi": versiBaru},
	})
	return versiBaru, nil
}

// izinPemindahan memeriksa izin tahap tujuan, lalu izin tiap tahap persetujuan yang
// dilompati pada pemindahan maju. Nil bila boleh.
func izinPemindahan(defs []domain.TahapDef, card *domain.Intervensi, target domain.TahapDef, hasPermission func(string) bool) *ForbiddenError {
	if !hasPermission(target.IzinDibutuhkan) {
		return &ForbiddenError{Izin: target.IzinDibutuhkan}
	}
	var asal domain.TahapDef
	for _, d := range defs {
		if d.Siklus == card.Siklus && d.KodeTahap == card.Tahap {
			asal = d
		}
	}
	for _, d := range defs {
		dilewati := d.Siklus == card.Siklus && d.Urutan > asal.Urutan && d.Urutan < target.Urutan
		if dilewati && d.IzinDibutuhkan != domain.IzinBawaan && !hasPermission(d.IzinDibutuhkan) {
			return &ForbiddenError{Izin: d.IzinDibutuhkan, Tahap: d.Label}
		}
	}
	return nil
}

// BulkAksi menerapkan aksi massal ("tugaskan" ke pemanggil, atau "eskalasi" ke Dewan
// Pengarah) dan mengembalikan hasil per kartu. Aksi yang mengubah kartu dicatat di audit.
func (uc *UseCase) BulkAksi(ctx context.Context, aksi string, ids []string, actorID string) (BulkResult, error) {
	domainAksi := domain.BulkAksi(aksi)
	if domainAksi != domain.BulkTugaskan && domainAksi != domain.BulkEskalasi {
		return BulkResult{}, ErrInvalidAction
	}
	ids = unique(ids)
	hasil, err := uc.repo.BulkAksi(ctx, domainAksi, ids, actorID)
	if err != nil {
		uc.log.Error("intervensi: aksi massal gagal", map[string]interface{}{"aksi": aksi, "jumlah": len(ids), "error": err.Error()})
		return BulkResult{}, err
	}
	res := BulkResult{Hasil: hasil}
	var berubah []string
	for _, h := range hasil {
		if h.Berhasil {
			res.Berhasil++
			berubah = append(berubah, h.ID)
		} else {
			res.Gagal++
		}
	}
	if len(berubah) > 0 {
		uc.writeAudit(ctx, audit.Entry{
			AktorID: actorID, Aksi: "BULK_" + strings.ToUpper(aksi), Entitas: "intervensi",
			EntitasID: fmt.Sprintf("%d kartu", len(berubah)), Hasil: audit.HasilBerhasil,
			Metadata: map[string]any{"ids": berubah, "diminta": len(ids)},
		})
	}
	return res, nil
}

// ListRiwayat mengembalikan peristiwa satu kartu, terlama dahulu.
func (uc *UseCase) ListRiwayat(ctx context.Context, id string) ([]domain.Riwayat, error) {
	if _, err := uc.repo.GetByID(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	items, err := uc.repo.ListRiwayat(ctx, id)
	if err != nil {
		uc.log.Error("intervensi: riwayat gagal", map[string]interface{}{"id": id, "error": err.Error()})
		return nil, err
	}
	return items, nil
}

// unique membuang id ganda dengan tetap menjaga urutan.
func unique(ids []string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
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
