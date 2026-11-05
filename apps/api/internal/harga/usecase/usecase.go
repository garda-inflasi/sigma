package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/harga/domain"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/google/uuid"
)

var (
	ErrNotFound      = domain.ErrNotFound
	ErrInvalidAction = errors.New("aksi tidak valid")
	// ErrValidasi dibungkus dengan penjelasan untuk pemanggil; cocokkan dengan errors.Is.
	ErrValidasi = errors.New("data tidak valid")
)

// Batas halaman daftar dan jendela waktu.
const (
	DefaultLimit = 100
	MaxLimit     = 1000
	MaxBulkIDs   = 1000

	DefaultRingkasanHari = 7
	MaxHari              = 365
	DefaultTrenHari      = 45

	// toleransiWaktu adalah selisih jam perangkat petugas yang masih diterima untuk waktu pencatatan.
	toleransiWaktu = 5 * time.Minute
)

// AgregatRefresher menyegarkan agregat dashboard setelah status entri berubah.
// Trigger tidak menunggu dan tidak pernah menggagalkan aksi.
type AgregatRefresher interface {
	Trigger()
}

type UseCase struct {
	repo    domain.Repository
	audit   audit.Writer
	refresh AgregatRefresher
	log     *logger.Logger
	now     func() time.Time
}

func New(repo domain.Repository, auditWriter audit.Writer, refresh AgregatRefresher, log *logger.Logger) *UseCase {
	return &UseCase{repo: repo, audit: auditWriter, refresh: refresh, log: log, now: time.Now}
}

// ListHarga mengembalikan satu halaman entri beserta jumlah seluruh hasil filter.
func (uc *UseCase) ListHarga(ctx context.Context, filter domain.ListFilter) ([]domain.EntriView, int64, error) {
	if filter.Status != "" && !domain.StatusValid(filter.Status) {
		return nil, 0, fmt.Errorf("%w: status %q", ErrValidasi, filter.Status)
	}
	if filter.Limit <= 0 {
		filter.Limit = DefaultLimit
	}
	filter.Limit = min(filter.Limit, MaxLimit)
	filter.Page = max(filter.Page, 1)

	items, total, err := uc.repo.List(ctx, filter)
	if err != nil {
		uc.log.Error("list harga failed", map[string]interface{}{"error": err.Error()})
		return nil, 0, fmt.Errorf("list harga: %w", err)
	}
	return items, total, nil
}

// GetHargaByID mengembalikan satu entri.
func (uc *UseCase) GetHargaByID(ctx context.Context, id string) (*domain.EntriView, error) {
	entri, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, ErrNotFound
		}
		uc.log.Error("get harga failed", map[string]interface{}{"id": id, "error": err.Error()})
		return nil, fmt.Errorf("get harga: %w", err)
	}
	return entri, nil
}

// Ringkasan menghitung entri per status pada hari terakhir.
func (uc *UseCase) Ringkasan(ctx context.Context, hari int) (domain.Ringkasan, error) {
	if hari <= 0 {
		hari = DefaultRingkasanHari
	}
	sejak := uc.now().AddDate(0, 0, -min(hari, MaxHari))
	ringkasan, err := uc.repo.Ringkasan(ctx, sejak)
	if err != nil {
		uc.log.Error("ringkasan harga failed", map[string]interface{}{"error": err.Error()})
		return domain.Ringkasan{}, fmt.Errorf("ringkasan harga: %w", err)
	}
	return ringkasan, nil
}

// GetTren mengembalikan tren harga komoditas di satu kabupaten.
func (uc *UseCase) GetTren(ctx context.Context, komoditasKode, kabupatenKode string, hari int, interval string) ([]domain.TitikTren, error) {
	if komoditasKode == "" || kabupatenKode == "" {
		return nil, fmt.Errorf("%w: komoditas dan wilayah wajib diisi", ErrValidasi)
	}
	switch interval {
	case "":
		interval = domain.IntervalHarian
	case domain.IntervalHarian, domain.IntervalMingguan:
	default:
		return nil, fmt.Errorf("%w: interval %q", ErrValidasi, interval)
	}
	if hari <= 0 {
		hari = DefaultTrenHari
	}

	sejak := uc.now().AddDate(0, 0, -min(hari, MaxHari))
	tren, err := uc.repo.Tren(ctx, komoditasKode, kabupatenKode, sejak, interval)
	if err != nil {
		uc.log.Error("get tren failed", map[string]interface{}{"komoditas": komoditasKode, "wilayah": kabupatenKode, "error": err.Error()})
		return nil, fmt.Errorf("get tren: %w", err)
	}
	return tren, nil
}

// CreateInput adalah isian entri baru dari petugas.
type CreateInput struct {
	KomoditasKode string
	PasarID       string
	Tipe          string
	Harga         int
	Kualitas      string
	Catatan       *string
	DicatatPada   *time.Time
}

// CreateHarga menyimpan entri berstatus DIAJUKAN atas nama petugas dan mencatatnya di audit trail.
func (uc *UseCase) CreateHarga(ctx context.Context, in CreateInput, petugasID string) (*domain.EntriHarga, error) {
	now := uc.now()
	dicatat := now
	if in.DicatatPada != nil {
		if in.DicatatPada.After(now.Add(toleransiWaktu)) {
			return nil, fmt.Errorf("%w: waktu pencatatan di masa depan", ErrValidasi)
		}
		dicatat = *in.DicatatPada
	}

	satuan, err := uc.repo.SatuanKomoditas(ctx, in.KomoditasKode, in.PasarID)
	if err != nil {
		if errors.Is(err, domain.ErrReferensiTidakAda) {
			return nil, fmt.Errorf("%w: %w", ErrValidasi, err)
		}
		return nil, fmt.Errorf("cari referensi harga: %w", err)
	}

	entri := &domain.EntriHarga{
		ID:            uuid.NewString(),
		KomoditasKode: in.KomoditasKode,
		PasarID:       in.PasarID,
		Tipe:          in.Tipe,
		Harga:         in.Harga,
		Satuan:        satuan,
		Kualitas:      in.Kualitas,
		Sumber:        "LAPANGAN",
		Status:        domain.StatusDiajukan,
		Catatan:       in.Catatan,
		DicatatPada:   dicatat,
		PetugasID:     &petugasID,
		DibuatPada:    now,
		DiupdatePada:  now,
	}
	if entri.Kualitas == "" {
		entri.Kualitas = "MEDIUM"
	}
	entri.DalamGeofence = true

	if err := uc.repo.Create(ctx, entri); err != nil {
		uc.log.Error("create harga failed", map[string]interface{}{"error": err.Error()})
		return nil, fmt.Errorf("create harga: %w", err)
	}

	uc.catat(ctx, audit.Entry{
		AktorID: petugasID, Aksi: "HARGA_INPUT", Entitas: "entri_harga", EntitasID: entri.ID,
		Hasil: audit.HasilBerhasil, Sesudah: entri,
	})
	return entri, nil
}

// HasilBulk melaporkan jumlah entri yang berubah dan yang dilewati.
type HasilBulk struct {
	Berhasil int
	Gagal    int
}

// BulkAction menjalankan aksi massal. Entri yang statusnya tidak memenuhi syarat
// aksi, atau yang dicatat oleh pelaku sendiri (pemisahan tugas), dilewati dan dihitung gagal.
func (uc *UseCase) BulkAction(ctx context.Context, ids []string, aksi, aktorID string) (HasilBulk, error) {
	action := domain.BulkAction(aksi)
	if !action.Valid() {
		return HasilBulk{}, ErrInvalidAction
	}
	unik := dedup(ids)
	if len(unik) == 0 || len(unik) > MaxBulkIDs {
		return HasilBulk{}, fmt.Errorf("%w: jumlah id harus 1 sampai %d", ErrValidasi, MaxBulkIDs)
	}

	changed, err := uc.repo.Transisi(ctx, unik, action, aktorID, uc.now())
	if err != nil {
		uc.log.Error("bulk action failed", map[string]interface{}{"action": aksi, "count": len(unik), "error": err.Error()})
		return HasilBulk{}, fmt.Errorf("bulk action: %w", err)
	}

	hasil := HasilBulk{Berhasil: len(changed), Gagal: len(unik) - len(changed)}
	if len(changed) == 0 {
		return hasil, nil
	}
	uc.refresh.Trigger()
	uc.catat(ctx, audit.Entry{
		AktorID: aktorID, Aksi: "HARGA_" + strings.ToUpper(aksi), Entitas: "entri_harga",
		EntitasID: fmt.Sprintf("%d entri", hasil.Berhasil), Hasil: audit.HasilBerhasil,
		Metadata: map[string]any{"ids": changed, "gagal": hasil.Gagal},
	})
	return hasil, nil
}

// catat menulis audit; kegagalan mencatat tidak membatalkan aksi yang sudah tersimpan.
func (uc *UseCase) catat(ctx context.Context, entry audit.Entry) {
	if err := uc.audit.Write(ctx, entry); err != nil {
		uc.log.Warn("harga: gagal mencatat audit", map[string]interface{}{"aksi": entry.Aksi, "error": err.Error()})
	}
}

func dedup(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, dup := seen[id]; dup || id == "" {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
