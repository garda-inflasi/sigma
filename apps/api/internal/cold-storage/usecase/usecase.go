package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/garda-inflasi/api/internal/cold-storage/domain"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/google/uuid"
)

var (
	ErrInvalidAction = errors.New("aksi tidak valid")
	ErrNoIDs         = errors.New("tidak ada lot yang dipilih")
	ErrTooManyIDs    = errors.New("terlalu banyak lot dipilih")
)

const (
	// DefaultLimit dan MaxLimit membatasi ukuran halaman daftar stok.
	DefaultLimit = 10
	MaxLimit     = 100
	// MaxBulkIDs membatasi jumlah lot dalam satu aksi massal.
	MaxBulkIDs = 500
)

type UseCase struct {
	repo domain.Repository
	log  *logger.Logger
	now  func() time.Time
}

func New(repo domain.Repository, log *logger.Logger) *UseCase {
	return &UseCase{repo: repo, log: log, now: time.Now}
}

// WithClock mengganti sumber waktu; dipakai pengujian agar sisa masa simpan deterministik.
func (uc *UseCase) WithClock(now func() time.Time) *UseCase {
	uc.now = now
	return uc
}

// Overview adalah data satu layar: kartu fasilitas, satu halaman stok, dan ringkasan.
type Overview struct {
	Fasilitas []domain.FasilitasView
	Stok      []domain.StokView

	Page       int
	Limit      int
	Total      int64
	TotalPages int
	// TotalVolumeTon menjumlahkan seluruh hasil filter, bukan hanya halaman ini.
	TotalVolumeTon float64

	// Ringkasan mencakup seluruh stok dan tidak terpengaruh filter tabel.
	Ringkasan domain.RingkasanStok
}

// GetAll mengambil fasilitas, satu halaman stok menurut q, dan ringkasan stok.
func (uc *UseCase) GetAll(ctx context.Context, q domain.StokQuery) (*Overview, error) {
	q = normalizeQuery(q)

	fasilitas, err := uc.repo.ListFasilitas(ctx)
	if err != nil {
		uc.log.Error("cold-storage: failed to get fasilitas", map[string]interface{}{"error": err.Error()})
		return nil, fmt.Errorf("list fasilitas: %w", err)
	}

	page, err := uc.repo.ListStok(ctx, q)
	if err != nil {
		uc.log.Error("cold-storage: failed to get stok", map[string]interface{}{"error": err.Error()})
		return nil, fmt.Errorf("list stok: %w", err)
	}

	ringkasan, err := uc.repo.RingkasanStok(ctx, domain.RisikoTinggiAmbang)
	if err != nil {
		uc.log.Error("cold-storage: failed to summarize stok", map[string]interface{}{"error": err.Error()})
		return nil, fmt.Errorf("ringkasan stok: %w", err)
	}

	for i := range fasilitas {
		f := &fasilitas[i]
		f.TerisiTon = round1(f.TerisiTon)
		if f.KapasitasTon > 0 {
			f.UtilisasiPersen = round1(f.TerisiTon / float64(f.KapasitasTon) * 100)
		}
	}

	now := uc.now()
	for i := range page.Items {
		s := &page.Items[i]
		s.VolumeTon = round1(s.VolumeTon)
		s.SisaMasaSimpanHari = sisaHari(now, s.KedaluwarsaPada)
	}

	ringkasan.TotalVolumeTon = round1(ringkasan.TotalVolumeTon)
	ringkasan.RisikoTinggiTon = round1(ringkasan.RisikoTinggiTon)

	return &Overview{
		Fasilitas:      fasilitas,
		Stok:           page.Items,
		Page:           q.Page,
		Limit:          q.Limit,
		Total:          page.Total,
		TotalPages:     totalPages(page.Total, q.Limit),
		TotalVolumeTon: round1(page.TotalVolumeTon),
		Ringkasan:      ringkasan,
	}, nil
}

// BulkInput adalah permintaan aksi massal beserta identitas pelakunya.
type BulkInput struct {
	Action  string
	IDs     []string
	ActorID string
	IP      string
}

// BulkOutcome adalah hasil aksi massal.
type BulkOutcome struct {
	Berhasil  int
	Gagal     int
	VolumeTon float64
}

// BulkAction memproses aksi massal pada lot stok dan mencatatnya di audit trail.
func (uc *UseCase) BulkAction(ctx context.Context, in BulkInput) (BulkOutcome, error) {
	action := domain.BulkAction(in.Action)
	switch action {
	case domain.BulkRencanakanDistribusi, domain.BulkRekomendasiOlahan:
		// valid
	default:
		return BulkOutcome{}, ErrInvalidAction
	}

	ids := uniqueNonEmpty(in.IDs)
	if len(ids) == 0 {
		return BulkOutcome{}, ErrNoIDs
	}
	if len(ids) > MaxBulkIDs {
		return BulkOutcome{}, ErrTooManyIDs
	}

	res, err := uc.repo.BulkAction(ctx, ids, action)
	if err != nil {
		uc.log.Error("cold-storage: bulk action failed", map[string]interface{}{"action": in.Action, "error": err.Error()})
		return BulkOutcome{}, fmt.Errorf("bulk action: %w", err)
	}

	out := BulkOutcome{
		Berhasil:  int(res.Berhasil),
		Gagal:     len(ids) - int(res.Berhasil),
		VolumeTon: round1(res.VolumeTon),
	}
	uc.recordAudit(ctx, in, ids, out)
	return out, nil
}

// recordAudit mencatat aksi massal. Kegagalan mencatat tidak membatalkan aksi:
// aksi ini bersifat advisori dan sudah diproses.
func (uc *UseCase) recordAudit(ctx context.Context, in BulkInput, ids []string, out BulkOutcome) {
	meta, _ := json.Marshal(map[string]interface{}{
		"ids":        ids,
		"gagal":      out.Gagal,
		"volume_ton": out.VolumeTon,
	})
	metaStr := string(meta)

	hasil := "BERHASIL"
	if out.Berhasil == 0 {
		hasil = "GAGAL"
	}

	entry := &domain.AuditLog{
		ID:        uuid.New().String(),
		Waktu:     uc.now(),
		Aksi:      "BULK_" + in.Action,
		Entitas:   "stok_cold_storage",
		EntitasID: fmt.Sprintf("%d lot", out.Berhasil),
		Hasil:     hasil,
		Metadata:  &metaStr,
	}
	if in.ActorID != "" {
		entry.AktorID = &in.ActorID
	}
	if in.IP != "" {
		entry.IP = &in.IP
	}

	if err := uc.repo.CreateAuditLog(ctx, entry); err != nil {
		uc.log.Warn("cold-storage: failed to record audit log", map[string]interface{}{"error": err.Error()})
	}
}

// normalizeQuery menerapkan nilai bawaan dan batas pada kriteria daftar stok.
func normalizeQuery(q domain.StokQuery) domain.StokQuery {
	q.Search = strings.TrimSpace(q.Search)
	q.ColdStorageID = strings.TrimSpace(q.ColdStorageID)
	if q.Sort == "" {
		q.Sort = domain.SortSisaMasaSimpan
	}
	if q.Page < 1 {
		q.Page = 1
	}
	if q.Limit < 1 {
		q.Limit = DefaultLimit
	}
	if q.Limit > MaxLimit {
		q.Limit = MaxLimit
	}
	return q
}

// sisaHari menghitung sisa masa simpan dalam hari, dibulatkan ke atas agar lot
// yang kedaluwarsa dalam hitungan jam masih terbaca "1 hari" dan bukan "0".
// Lot yang sudah lewat batas bernilai 0.
func sisaHari(now, kedaluwarsa time.Time) int {
	d := kedaluwarsa.Sub(now)
	if d <= 0 {
		return 0
	}
	return int(math.Ceil(d.Hours() / 24))
}

func totalPages(total int64, limit int) int {
	if total <= 0 || limit <= 0 {
		return 0
	}
	return int((total + int64(limit) - 1) / int64(limit))
}

// round1 membulatkan ke satu desimal, cukup untuk volume ton dan persentase.
func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

func uniqueNonEmpty(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, id := range in {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
