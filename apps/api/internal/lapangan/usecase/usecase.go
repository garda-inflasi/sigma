package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/lapangan/domain"
	"github.com/garda-inflasi/api/pkg/logger"
)

var (
	ErrNotFound      = domain.ErrNotFound
	ErrInvalidAction = errors.New("aksi tidak valid")
	// ErrValidasi dibungkus dengan penjelasan untuk pemanggil; cocokkan dengan errors.Is.
	ErrValidasi = errors.New("data tidak valid")
)

// MaxBulkIDs membatasi jumlah id pada satu aksi massal.
const MaxBulkIDs = 500

// Parameter data acuan offline.
const (
	// ToleransiAcuan adalah selisih dari median yang masih dianggap wajar (±25%).
	ToleransiAcuan = 0.25
	// HariAcuan adalah jendela median harga resmi.
	HariAcuan = 7
	// MinSampelAcuan adalah jumlah entri resmi minimal agar median dipercaya.
	MinSampelAcuan = 3
	// pembulatanAcuan membulatkan batas rentang wajar ke ratusan rupiah.
	pembulatanAcuan = 100
)

// Tugas adalah tugas yang siap ditampilkan: status sudah dihitung terhadap waktu sekarang.
type Tugas struct {
	domain.TugasView
	StatusTampil string
}

// Pemanggil menjelaskan siapa yang meminta; Semua true bila ia boleh mengelola tugas petugas lain.
type Pemanggil struct {
	ID    string
	Semua bool
}

type UseCase struct {
	repo  domain.Repository
	audit audit.Writer
	log   *logger.Logger
	now   func() time.Time
}

func New(repo domain.Repository, auditWriter audit.Writer, log *logger.Logger) *UseCase {
	return &UseCase{repo: repo, audit: auditWriter, log: log, now: time.Now}
}

func (uc *UseCase) tampil(items ...domain.TugasView) []Tugas {
	now := uc.now()
	out := make([]Tugas, len(items))
	for i, item := range items {
		out[i] = Tugas{TugasView: item, StatusTampil: item.StatusEfektif(now)}
	}
	return out
}

// ListSaya mengembalikan tugas milik pemanggil.
func (uc *UseCase) ListSaya(ctx context.Context, petugasID string) ([]Tugas, error) {
	items, err := uc.repo.ListSaya(ctx, petugasID)
	if err != nil {
		uc.log.Error("lapangan: list tugas gagal", map[string]interface{}{"error": err.Error()})
		return nil, fmt.Errorf("list tugas: %w", err)
	}
	return uc.tampil(items...), nil
}

// GetByID mengembalikan satu tugas; tugas milik petugas lain tidak terlihat kecuali pemanggil boleh mengelola.
func (uc *UseCase) GetByID(ctx context.Context, id string, p Pemanggil) (*Tugas, error) {
	item, err := uc.repo.GetByID(ctx, id, p.ID, p.Semua)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, ErrNotFound
		}
		uc.log.Error("lapangan: get tugas gagal", map[string]interface{}{"id": id, "error": err.Error()})
		return nil, fmt.Errorf("get tugas: %w", err)
	}
	return &uc.tampil(*item)[0], nil
}

// HasilBulk melaporkan tugas yang diproses dan yang dilewati.
type HasilBulk struct {
	Berhasil int
	Gagal    int
}

// BulkAction menjalankan aksi massal. tandai_selesai hanya menutup tugas yang
// targetnya sudah terpenuhi dan dicatat di audit trail; unduh_data_acuan hanya
// menghitung tugas yang boleh diunduh pemanggil.
func (uc *UseCase) BulkAction(ctx context.Context, ids []string, aksi string, p Pemanggil) (HasilBulk, error) {
	action := domain.BulkAction(aksi)
	if !action.Valid() {
		return HasilBulk{}, ErrInvalidAction
	}
	unik := dedup(ids)
	if len(unik) == 0 || len(unik) > MaxBulkIDs {
		return HasilBulk{}, fmt.Errorf("%w: jumlah id harus 1 sampai %d", ErrValidasi, MaxBulkIDs)
	}

	if action == domain.BulkUnduhDataAcuan {
		found, err := uc.repo.Aksesibel(ctx, unik, p.ID, p.Semua)
		if err != nil {
			return HasilBulk{}, fmt.Errorf("bulk %s: %w", aksi, err)
		}
		return HasilBulk{Berhasil: len(found), Gagal: len(unik) - len(found)}, nil
	}

	changed, err := uc.repo.TandaiSelesai(ctx, unik, p.ID, p.Semua, uc.now())
	if err != nil {
		uc.log.Error("lapangan: tandai selesai gagal", map[string]interface{}{"count": len(unik), "error": err.Error()})
		return HasilBulk{}, fmt.Errorf("bulk %s: %w", aksi, err)
	}

	hasil := HasilBulk{Berhasil: len(changed), Gagal: len(unik) - len(changed)}
	if len(changed) == 0 {
		return hasil, nil
	}
	if err := uc.audit.Write(ctx, audit.Entry{
		AktorID: p.ID, Aksi: "TUGAS_" + strings.ToUpper(aksi), Entitas: "tugas_lapangan",
		EntitasID: fmt.Sprintf("%d tugas", hasil.Berhasil), Hasil: audit.HasilBerhasil,
		Metadata: map[string]any{"ids": changed, "gagal": hasil.Gagal},
	}); err != nil {
		uc.log.Warn("lapangan: gagal mencatat audit", map[string]interface{}{"aksi": aksi, "error": err.Error()})
	}
	return hasil, nil
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

// HargaAcuan adalah median dan rentang wajar harga satu komoditas dan tipe di satu kabupaten.
type HargaAcuan struct {
	KomoditasKode string `json:"komoditas_kode"`
	KabupatenKode string `json:"kabupaten_kode"`
	Tipe          string `json:"tipe"`
	Median        int    `json:"median"`
	Bawah         int    `json:"bawah"`
	Atas          int    `json:"atas"`
	Sampel        int    `json:"sampel"`
}

// DataAcuan adalah paket unduhan offline petugas lapangan beserta versinya. Versi
// berubah hanya bila isinya berubah, sehingga klien dapat bertanya "ada yang baru?".
type DataAcuan struct {
	Versi      string
	Toleransi  float64
	Komoditas  []domain.AcuanKomoditas
	Pasar      []domain.AcuanPasar
	HargaAcuan []HargaAcuan
}

// Acuan menyusun data acuan offline: komoditas, pasar beserta geofence, dan median
// harga resmi tujuh hari terakhir dengan rentang wajar.
func (uc *UseCase) Acuan(ctx context.Context) (DataAcuan, error) {
	sejak := uc.now().AddDate(0, 0, -HariAcuan)
	raw, err := uc.repo.Acuan(ctx, sejak, MinSampelAcuan)
	if err != nil {
		uc.log.Error("lapangan: baca data acuan gagal", map[string]interface{}{"error": err.Error()})
		return DataAcuan{}, fmt.Errorf("data acuan: %w", err)
	}

	harga := make([]HargaAcuan, len(raw.Median))
	for i, m := range raw.Median {
		harga[i] = HargaAcuan{
			KomoditasKode: m.KomoditasKode, KabupatenKode: m.KabupatenKode, Tipe: m.Tipe, Sampel: m.Sampel,
			Median: int(math.Round(m.Median)),
			Bawah:  bulatkan(m.Median * (1 - ToleransiAcuan)),
			Atas:   bulatkan(m.Median * (1 + ToleransiAcuan)),
		}
	}
	data := DataAcuan{Toleransi: ToleransiAcuan, Komoditas: raw.Komoditas, Pasar: raw.Pasar, HargaAcuan: harga}

	isi, err := json.Marshal(data)
	if err != nil {
		return DataAcuan{}, fmt.Errorf("versi data acuan: %w", err)
	}
	ringkas := sha256.Sum256(isi)
	data.Versi = hex.EncodeToString(ringkas[:8])
	return data, nil
}

func bulatkan(v float64) int {
	return int(math.Round(v/pembulatanAcuan)) * pembulatanAcuan
}
