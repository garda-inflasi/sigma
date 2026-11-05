package usecase

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/ingestion/cron"
	ingest "github.com/garda-inflasi/api/internal/ingestion/domain"
	"github.com/garda-inflasi/api/internal/ingestion/engine"
	"github.com/garda-inflasi/api/internal/sumber-data/domain"
	"github.com/garda-inflasi/api/pkg/logger"
)

var (
	ErrNotFound      = domain.ErrNotFound
	ErrInvalidAction = errors.New("aksi tidak valid")
	// ErrTanpaKonektor: sumber ada tetapi belum punya konektor ingestion.
	ErrTanpaKonektor = ingest.ErrTanpaKonektor
	// ErrDijeda dan ErrSudahBerjalan: sumber tidak bisa dijalankan saat ini.
	ErrDijeda        = ingest.ErrDijeda
	ErrSudahBerjalan = ingest.ErrSudahBerjalan
)

// Berjalan adalah eksekusi yang sudah tercatat dan siap dijalankan (engine.Run).
type Berjalan interface {
	Eksekusi() ingest.EksekusiSumber
	Jalankan(ctx context.Context) (*ingest.EksekusiSumber, error)
}

// Ingestion adalah bagian engine ingestion yang dipakai katalog sumber.
type Ingestion interface {
	// Punya benar bila sumber punya konektor.
	Punya(sumberID string) bool
	Mulai(ctx context.Context, sumberID, pemicu string, aktorID *string) (Berjalan, error)
	Eksekusi(ctx context.Context, sumberID string, limit int) ([]ingest.EksekusiSumber, error)
	SampelMentah(ctx context.Context, sumberID string, limit int) ([]ingest.SampelMentah, error)
	Pipeline(ctx context.Context) (ingest.Pipeline, error)
	TerakhirSukses(ctx context.Context) (map[string]time.Time, error)
}

// Pipeline melengkapi hitungan lapisan dan mutu dari repository dengan ketepatan waktu.
type Pipeline struct {
	ingest.Pipeline
	// KetepatanWaktu: persen sumber ber-konektor (tidak dijeda) yang eksekusi suksesnya terakhir
	// tidak lebih tua dari dua kali selang jadwalnya. Nil bila tidak ada sumber yang dinilai.
	KetepatanWaktu *float64
}

// Pipeline menghitung isi lapisan bronze/silver/gold dan mutu data dari eksekusi nyata.
func (uc *UseCase) Pipeline(ctx context.Context, sekarang time.Time) (Pipeline, error) {
	p, err := uc.ingest.Pipeline(ctx)
	if err != nil {
		return Pipeline{}, err
	}
	sumber, err := uc.repo.List(ctx)
	if err != nil {
		return Pipeline{}, fmt.Errorf("pipeline: baca katalog: %w", err)
	}
	sukses, err := uc.ingest.TerakhirSukses(ctx)
	if err != nil {
		return Pipeline{}, err
	}

	dinilai, tepat := 0, 0
	for _, s := range sumber {
		if !uc.ingest.Punya(s.ID) || s.Status == domain.StatusDijeda {
			continue
		}
		jadwal, err := cron.Parse(s.JadwalCron)
		if err != nil {
			uc.log.Warn("sumber-data: jadwal tidak valid, dilewati dari ketepatan waktu", map[string]interface{}{"sumber": s.ID, "error": err.Error()})
			continue
		}
		dinilai++
		if terakhir, ada := sukses[s.ID]; ada && sekarang.Sub(terakhir) <= 2*jadwal.Selang(sekarang) {
			tepat++
		}
	}
	res := Pipeline{Pipeline: p}
	if dinilai > 0 {
		res.KetepatanWaktu = new(math.Round(1000*float64(tepat)/float64(dinilai)) / 10)
	}
	return res, nil
}

// BulkResult: Berhasil adalah jumlah sumber yang berubah; Gagal sisanya dari id yang diminta.
type BulkResult struct {
	Berhasil, Gagal int
}

type UseCase struct {
	repo   domain.Repository
	ingest Ingestion
	audit  audit.Writer
	log    *logger.Logger
}

func New(repo domain.Repository, ingestion Ingestion, auditWriter audit.Writer, log *logger.Logger) *UseCase {
	return &UseCase{repo: repo, ingest: ingestion, audit: auditWriter, log: log}
}

// BisaDijalankan benar bila sumber punya konektor ingestion.
func (uc *UseCase) BisaDijalankan(sumberID string) bool { return uc.ingest.Punya(sumberID) }

// Eksekusi mengembalikan riwayat eksekusi sumber, terbaru lebih dulu.
func (uc *UseCase) Eksekusi(ctx context.Context, sumberID string, limit int) ([]ingest.EksekusiSumber, error) {
	if _, err := uc.repo.GetByID(ctx, sumberID); err != nil {
		return nil, err
	}
	return uc.ingest.Eksekusi(ctx, sumberID, limit)
}

// Sampel mengembalikan cuplikan payload mentah (bronze) terbaru sumber.
func (uc *UseCase) Sampel(ctx context.Context, sumberID string, limit int) ([]ingest.SampelMentah, error) {
	if _, err := uc.repo.GetByID(ctx, sumberID); err != nil {
		return nil, err
	}
	return uc.ingest.SampelMentah(ctx, sumberID, limit)
}

// Jalankan memulai ingestion sumber sekarang dan segera mengembalikan eksekusi
// yang tercatat BERJALAN; pengambilan data berlanjut di latar belakang dan
// hasilnya dibaca lewat Eksekusi. Aksi dicatat di audit trail.
func (uc *UseCase) Jalankan(ctx context.Context, sumberID, actorID string) (ingest.EksekusiSumber, error) {
	if _, err := uc.repo.GetByID(ctx, sumberID); err != nil {
		return ingest.EksekusiSumber{}, err
	}
	run, err := uc.ingest.Mulai(ctx, sumberID, ingest.PemicuManual, &actorID)
	if err != nil {
		uc.catatJalankan(ctx, sumberID, actorID, audit.HasilDitolak, map[string]any{"alasan": err.Error()})
		return ingest.EksekusiSumber{}, err
	}
	e := run.Eksekusi()
	uc.catatJalankan(ctx, sumberID, actorID, audit.HasilBerhasil, map[string]any{"eksekusi_id": e.ID})

	// Lepas dari konteks permintaan: klien tidak menunggu run selesai.
	bg, cancel := context.WithTimeout(context.WithoutCancel(ctx), engine.BatasWaktu)
	go func() {
		defer cancel()
		if _, err := run.Jalankan(bg); err != nil {
			uc.log.Error("sumber-data: gagal mencatat hasil eksekusi", map[string]interface{}{"sumber": sumberID, "eksekusi": e.ID, "error": err.Error()})
		}
	}()
	return e, nil
}

func (uc *UseCase) catatJalankan(ctx context.Context, sumberID, actorID, hasil string, metadata map[string]any) {
	err := uc.audit.Write(ctx, audit.Entry{
		AktorID: actorID, Aksi: "JALANKAN_SUMBER", Entitas: "sumber_data", EntitasID: sumberID,
		Hasil: hasil, Metadata: metadata,
	})
	if err != nil {
		uc.log.Warn("sumber-data: gagal mencatat audit", map[string]interface{}{"aksi": "JALANKAN_SUMBER", "error": err.Error()})
	}
}

func (uc *UseCase) List(ctx context.Context) ([]domain.SumberData, error) {
	return uc.repo.List(ctx)
}

func (uc *UseCase) GetByID(ctx context.Context, id string) (*domain.SumberData, error) {
	return uc.repo.GetByID(ctx, id)
}

// BulkAction menjeda atau melanjutkan sumber dan mencatatnya di audit trail.
// Id ganda dihitung sekali.
func (uc *UseCase) BulkAction(ctx context.Context, ids []string, action, actorID string) (BulkResult, error) {
	domainAction := domain.BulkAction(action)
	if domainAction != domain.BulkJeda && domainAction != domain.BulkLanjutkan {
		return BulkResult{}, ErrInvalidAction
	}

	ids = slices.Compact(slices.Sorted(slices.Values(ids)))
	changed, err := uc.repo.BulkAction(ctx, ids, domainAction)
	if err != nil {
		return BulkResult{}, fmt.Errorf("bulk sumber data: %w", err)
	}

	err = uc.audit.Write(ctx, audit.Entry{
		AktorID: actorID, Aksi: "BULK_" + action, Entitas: "sumber_data", EntitasID: fmt.Sprintf("%d sumber", len(changed)),
		Hasil: audit.HasilBerhasil, Metadata: map[string]any{"diminta": ids, "berubah": changed},
	})
	if err != nil {
		uc.log.Warn("sumber-data: gagal mencatat audit", map[string]interface{}{"aksi": action, "error": err.Error()})
	}
	return BulkResult{Berhasil: len(changed), Gagal: len(ids) - len(changed)}, nil
}
