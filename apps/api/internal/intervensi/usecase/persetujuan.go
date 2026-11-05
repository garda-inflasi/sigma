package usecase

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/intervensi/domain"
	"gorm.io/gorm"
)

var (
	ErrKartuSelesai        = domain.ErrKartuSelesai
	ErrTanpaPersetujuan    = domain.ErrTanpaPersetujuan
	ErrPersetujuanTerbuka  = domain.ErrPersetujuanTerbuka
	ErrPersetujuanNotFound = domain.ErrPersetujuanNotFound
	ErrPersetujuanSelesai  = domain.ErrPersetujuanSelesai
	// ErrPemisahanTugas: pemutus tidak boleh sama dengan pengaju, dan satu orang tidak memutus dua langkah.
	ErrPemisahanTugas = errors.New("pengaju atau pemutus langkah sebelumnya tidak boleh memutus langkah ini")
	// ErrCatatanWajib: penolakan harus disertai alasan.
	ErrCatatanWajib = errors.New("penolakan wajib disertai catatan")
)

// ListAlur mengembalikan definisi alur persetujuan beserta langkahnya.
func (uc *UseCase) ListAlur(ctx context.Context) ([]domain.Alur, error) {
	alur, err := uc.wf.ListAlur(ctx)
	if err != nil {
		uc.log.Error("intervensi: daftar alur gagal", map[string]interface{}{"error": err.Error()})
		return nil, err
	}
	return alur, nil
}

// ListPersetujuan mengembalikan permintaan persetujuan (terbaru dahulu); status kosong = semua.
func (uc *UseCase) ListPersetujuan(ctx context.Context, status string) ([]domain.Persetujuan, error) {
	items, err := uc.wf.ListPersetujuan(ctx, status)
	if err != nil {
		uc.log.Error("intervensi: daftar persetujuan gagal", map[string]interface{}{"error": err.Error()})
		return nil, err
	}
	return items, nil
}

// Ajukan meminta persetujuan memindahkan kartu ke tahap persetujuan atau tahap sesudahnya. Rantai
// persetujuan terdiri dari alur tiap tahap persetujuan yang dilalui; setelah semua langkah disetujui
// kartu dipindahkan otomatis. Hanya satu permintaan terbuka per kartu.
func (uc *UseCase) Ajukan(ctx context.Context, kartuID, tahapTujuan, catatan, actorID string) (*domain.Persetujuan, error) {
	card, err := uc.repo.GetByID(ctx, kartuID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	defs, err := uc.repo.ListTahap(ctx)
	if err != nil {
		return nil, err
	}
	if _, _, ok := cariTahap(defs, card.Siklus, tahapTujuan); !ok {
		return nil, ErrInvalidTahap
	}

	p, err := uc.wf.Ajukan(ctx, domain.AjukanInput{KartuID: kartuID, Jenis: domain.JenisPindahTahap, TahapTujuan: tahapTujuan, Oleh: actorID, Catatan: catatan})
	if err != nil {
		if !errors.Is(err, ErrNotFound) && !errors.Is(err, ErrKartuSelesai) && !errors.Is(err, ErrTanpaPersetujuan) &&
			!errors.Is(err, ErrPersetujuanTerbuka) && !errors.Is(err, ErrInvalidTahap) {
			uc.log.Error("intervensi: ajukan persetujuan gagal", map[string]interface{}{"id": kartuID, "error": err.Error()})
		}
		return nil, err
	}
	uc.writeAudit(ctx, audit.Entry{
		AktorID: actorID, Aksi: "AJUKAN_PERSETUJUAN", Entitas: "intervensi", EntitasID: kartuID, Hasil: audit.HasilBerhasil,
		Metadata: map[string]any{"persetujuan": p.ID, "tahap_tujuan": tahapTujuan, "langkah": len(p.Langkah)},
	})
	return p, nil
}

// Putus memutuskan langkah persetujuan yang sedang menunggu. hasPermission harus memenuhi izin langkah itu;
// pengaju dan pemutus langkah sebelumnya tidak boleh memutus (pemisahan tugas). Keputusan, termasuk
// pemindahan kartu setelah langkah terakhir, dicatat di audit.
func (uc *UseCase) Putus(ctx context.Context, id int64, keputusan, catatan, actorID string, hasPermission func(string) bool) (*domain.Persetujuan, error) {
	catatan = strings.TrimSpace(catatan)
	if keputusan == domain.KeputusanTolak && catatan == "" {
		return nil, ErrCatatanWajib
	}
	p, err := uc.wf.GetPersetujuan(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.Status != domain.PersetujuanMenunggu {
		return nil, ErrPersetujuanSelesai
	}

	izin := p.IzinLangkahKini()
	if !hasPermission(izin) {
		uc.writeAudit(ctx, audit.Entry{
			AktorID: actorID, Aksi: "PUTUS_PERSETUJUAN", Entitas: "intervensi", EntitasID: p.IntervensiID, Hasil: audit.HasilDitolak,
			Metadata: map[string]any{"persetujuan": id, "izin": izin},
		})
		return nil, &ForbiddenError{Izin: izin}
	}
	if sudahTerlibat(p, actorID) {
		return nil, ErrPemisahanTugas
	}

	hasil, err := uc.wf.Putus(ctx, domain.PutusInput{PersetujuanID: id, LangkahKini: p.LangkahKini, Keputusan: keputusan, Oleh: actorID, Catatan: catatan})
	if err != nil {
		if !errors.Is(err, ErrPersetujuanSelesai) && !errors.Is(err, ErrKartuSelesai) && !errors.Is(err, ErrPersetujuanNotFound) {
			uc.log.Error("intervensi: putus persetujuan gagal", map[string]interface{}{"id": id, "error": err.Error()})
			return nil, fmt.Errorf("putus persetujuan: %w", err)
		}
		return nil, err
	}
	uc.writeAudit(ctx, audit.Entry{
		AktorID: actorID, Aksi: "PUTUS_PERSETUJUAN", Entitas: "intervensi", EntitasID: p.IntervensiID, Hasil: audit.HasilBerhasil,
		Metadata: map[string]any{
			"persetujuan": id, "langkah": p.LangkahKini, "keputusan": keputusan, "status": hasil.Status, "jenis": p.Jenis, "tahap_tujuan": p.TahapTujuan,
		},
	})
	return hasil, nil
}

// sudahTerlibat: aktor adalah pengaju, atau sudah memutus langkah lain permintaan ini.
func sudahTerlibat(p *domain.Persetujuan, actorID string) bool {
	if p.DiajukanOleh != nil && *p.DiajukanOleh == actorID {
		return true
	}
	return slices.ContainsFunc(p.Langkah, func(l domain.LangkahPersetujuan) bool { return l.Oleh != nil && *l.Oleh == actorID })
}
