package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/policy-brief/domain"
	"github.com/garda-inflasi/api/pkg/logger"
)

var (
	ErrPeriodeTidakValid = errors.New("periode harus berformat YYYY-MM")
	ErrPeriodeSudahAda   = errors.New("brief untuk periode ini sudah ada")
	ErrAksiTidakValid    = errors.New("aksi tidak valid")
)

// Penanda draf otomatis: Generate belum memakai LLM atau model (Fase F, 15-B).
const (
	catatanDrafOtomatis = "Draf otomatis non-LLM dari templat deterministik"
	halamanKerangka     = 1
)

type UseCase struct {
	repo  domain.Repository
	audit audit.Writer
	log   *logger.Logger
}

func New(repo domain.Repository, auditWriter audit.Writer, log *logger.Logger) *UseCase {
	return &UseCase{repo: repo, audit: auditWriter, log: log}
}

func (uc *UseCase) List(ctx context.Context) ([]domain.BriefView, error) {
	return uc.repo.List(ctx)
}

// Generate menyimpan draf brief untuk periode (YYYY-MM) atas nama penyusun.
// Isinya kerangka dari templat deterministik, berlabel jelas sebagai draf otomatis
// non-LLM; penyusunan nyata dengan data dan LLM dikerjakan di 15-B.
func (uc *UseCase) Generate(ctx context.Context, penyusunID, periode string) (*domain.BriefView, error) {
	tanggal, err := time.Parse(domain.FormatPeriode, periode)
	if err != nil {
		return nil, ErrPeriodeTidakValid
	}
	label := domain.LabelPeriode(tanggal)

	pb := &domain.PolicyBrief{
		ID:      domain.BriefID(tanggal),
		Periode: label,
		Judul:   fmt.Sprintf("Policy brief %s (draf otomatis)", label),
		Status:  domain.StatusDraf,
		Ringkasan: fmt.Sprintf("%s: kerangka policy brief periode %s. Angka, temuan, dan rekomendasi belum diisi; "+
			"wajib dilengkapi dan ditelaah manusia sebelum diajukan.", catatanDrafOtomatis, label),
		Rekomendasi:   "[]",
		Halaman:       halamanKerangka,
		DisusunOlehID: penyusunID,
	}

	dibuat, err := uc.repo.Create(ctx, pb)
	if err != nil {
		return nil, err
	}
	if !dibuat {
		return nil, ErrPeriodeSudahAda
	}

	uc.catatAudit(ctx, penyusunID, pb)

	return uc.repo.Get(ctx, pb.ID)
}

func (uc *UseCase) catatAudit(ctx context.Context, penyusunID string, pb *domain.PolicyBrief) {
	err := uc.audit.Write(ctx, audit.Entry{
		AktorID: penyusunID, Aksi: "GENERATE_BRIEF", Entitas: "policy_brief", EntitasID: pb.ID,
		Hasil:    audit.HasilBerhasil,
		Sesudah:  map[string]any{"status": pb.Status, "periode": pb.Periode},
		Metadata: map[string]any{"penyusun": "templat non-LLM"},
	})
	if err != nil {
		uc.log.Warn("policy-brief: gagal mencatat audit", map[string]interface{}{"id": pb.ID, "error": err.Error()})
	}
}

// BulkResult adalah hasil aksi massal: yang berpindah status dan yang dilewati.
type BulkResult struct {
	Berhasil int
	Gagal    int
}

// transisiAksi memetakan aksi ke status asal dan tujuannya.
var transisiAksi = map[string][2]string{
	domain.AksiTelaah:  {domain.StatusDraf, domain.StatusTelaah},
	domain.AksiSetujui: {domain.StatusTelaah, domain.StatusDisetujui},
}

// BulkAction memajukan status brief. Brief yang tidak ada atau bukan di status asal
// dihitung gagal; setiap perubahan dicatat di audit.
func (uc *UseCase) BulkAction(ctx context.Context, ids []string, aksi, aktorID string) (BulkResult, error) {
	rute, ok := transisiAksi[aksi]
	if !ok {
		return BulkResult{}, ErrAksiTidakValid
	}

	var hasil BulkResult
	for _, id := range ids {
		berubah, err := uc.repo.Transisi(ctx, id, rute[0], rute[1])
		if err != nil {
			return hasil, err
		}
		if !berubah {
			hasil.Gagal++
			continue
		}
		hasil.Berhasil++
		uc.catatTransisi(ctx, aktorID, id, aksi, rute)
	}
	return hasil, nil
}

func (uc *UseCase) catatTransisi(ctx context.Context, aktorID, id, aksi string, rute [2]string) {
	err := uc.audit.Write(ctx, audit.Entry{
		AktorID: aktorID, Aksi: "BRIEF_" + strings.ToUpper(aksi), Entitas: "policy_brief", EntitasID: id,
		Hasil:   audit.HasilBerhasil,
		Sebelum: map[string]any{"status": rute[0]},
		Sesudah: map[string]any{"status": rute[1]},
	})
	if err != nil {
		uc.log.Warn("policy-brief: gagal mencatat audit", map[string]interface{}{"id": id, "error": err.Error()})
	}
}
