package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	hargadomain "github.com/garda-inflasi/api/internal/harga/domain"
	"github.com/garda-inflasi/api/internal/lapangan/domain"
	"github.com/garda-inflasi/api/pkg/idx"
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
	repo       domain.Repository
	audit      audit.Writer
	log        *logger.Logger
	now        func() time.Time
	storageDir string
}

func New(repo domain.Repository, auditWriter audit.Writer, log *logger.Logger) *UseCase {
	return &UseCase{
		repo:       repo,
		audit:      auditWriter,
		log:        log,
		now:        time.Now,
		storageDir: "./storage/lapangan-foto",
	}
}

// WithStorageDir mengatur direktori penyimpanan berkas foto lapangan.
func (uc *UseCase) WithStorageDir(dir string) *UseCase {
	if strings.TrimSpace(dir) != "" {
		uc.storageDir = strings.TrimSpace(dir)
	}
	return uc
}

func (uc *UseCase) tampil(items ...domain.TugasView) []Tugas {
	now := uc.now()
	out := make([]Tugas, len(items))
	for i, item := range items {
		out[i] = Tugas{TugasView: item, StatusTampil: item.StatusEfektif(now)}
	}
	return out
}

// Cakupan menurunkan jangkauan baris pemanggil dari peran dan kabupaten tugasnya di DB (02-B).
// Pemegang tugas.kelola atau dashboard.operasional memperoleh cakupan perannya; selain itu hanya miliknya sendiri.
func (uc *UseCase) Cakupan(ctx context.Context, p Pemanggil) (hargadomain.Cakupan, error) {
	if !p.Semua {
		return hargadomain.CakupanMilik(p.ID), nil
	}
	peran, kab, ditemukan, err := uc.repo.PenggunaCakupan(ctx, p.ID)
	if err != nil {
		return hargadomain.Cakupan{}, fmt.Errorf("baca cakupan pengguna: %w", err)
	}
	if !ditemukan {
		return hargadomain.Cakupan{}, nil
	}
	return hargadomain.CakupanPeran(peran, kab, p.ID), nil
}

// ListSaya mengembalikan tugas sesuai cakupan baris pemanggil.
func (uc *UseCase) ListSaya(ctx context.Context, p Pemanggil) ([]Tugas, error) {
	cakupan, err := uc.Cakupan(ctx, p)
	if err != nil {
		uc.log.Error("lapangan: baca cakupan gagal", map[string]interface{}{"error": err.Error()})
		return nil, fmt.Errorf("list tugas: %w", err)
	}
	items, err := uc.repo.ListSaya(ctx, cakupan)
	if err != nil {
		uc.log.Error("lapangan: list tugas gagal", map[string]interface{}{"error": err.Error()})
		return nil, fmt.Errorf("list tugas: %w", err)
	}
	return uc.tampil(items...), nil
}

// GetByID mengembalikan satu tugas menurut ID BIGINT atau kode lama (TGS-...) sesuai cakupan baris pemanggil.
func (uc *UseCase) GetByID(ctx context.Context, ref idx.RujukanID, p Pemanggil) (*Tugas, error) {
	cakupan, err := uc.Cakupan(ctx, p)
	if err != nil {
		uc.log.Error("lapangan: baca cakupan gagal", map[string]interface{}{"error": err.Error()})
		return nil, fmt.Errorf("get tugas: %w", err)
	}
	item, err := uc.repo.GetByID(ctx, ref, cakupan)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, ErrNotFound
		}
		uc.log.Error("lapangan: get tugas gagal", map[string]interface{}{"id": ref.ID.String(), "kode": ref.Kode, "error": err.Error()})
		return nil, fmt.Errorf("get tugas: %w", err)
	}
	return &uc.tampil(*item)[0], nil
}

// Detail adalah satu tugas beserta rincian tiap komoditasnya (urutan sesuai tugas).
type Detail struct {
	Tugas
	Rincian []domain.RincianKomoditas
}

// GetDetail seperti GetByID, ditambah nama dan status terisi tiap komoditas tugas.
func (uc *UseCase) GetDetail(ctx context.Context, ref idx.RujukanID, p Pemanggil) (*Detail, error) {
	tugas, err := uc.GetByID(ctx, ref, p)
	if err != nil {
		return nil, err
	}
	ada, err := uc.repo.Rincian(ctx, tugas.ID, tugas.Komoditas)
	if err != nil {
		uc.log.Error("lapangan: rincian tugas gagal", map[string]interface{}{"id": tugas.ID.String(), "error": err.Error()})
		return nil, fmt.Errorf("rincian tugas: %w", err)
	}
	byKode := make(map[string]domain.RincianKomoditas, len(ada))
	for _, r := range ada {
		byKode[r.Kode] = r
	}
	rincian := make([]domain.RincianKomoditas, len(tugas.Komoditas))
	for i, kode := range tugas.Komoditas {
		// Kode yang sudah tak ada di master tetap tampil dengan kodenya sebagai nama.
		r, ok := byKode[kode]
		if !ok {
			r = domain.RincianKomoditas{Kode: kode, Nama: kode}
		}
		rincian[i] = r
	}
	return &Detail{Tugas: *tugas, Rincian: rincian}, nil
}

// HasilBulk melaporkan tugas yang diproses dan yang dilewati.
type HasilBulk struct {
	Berhasil int
	Gagal    int
}

// BulkAction menjalankan aksi massal. refs adalah ID BIGINT atau kode lama (TGS-...): antrean dan tampilan
// yang dibuat sebelum migrasi 000111 masih memegang kode lama sebagai ID, dan tetap dilayani. tandai_selesai hanya
// menutup tugas yang targetnya sudah terpenuhi dan dicatat di audit trail; unduh_data_acuan hanya menghitung
// tugas yang boleh diunduh pemanggil. Berhasil dan Gagal dihitung per rujukan.
func (uc *UseCase) BulkAction(ctx context.Context, refs []idx.RujukanID, aksi string, p Pemanggil) (HasilBulk, error) {
	action := domain.BulkAction(aksi)
	if !action.Valid() {
		return HasilBulk{}, ErrInvalidAction
	}
	unik := dedup(refs)
	if len(unik) == 0 || len(unik) > MaxBulkIDs {
		return HasilBulk{}, fmt.Errorf("%w: jumlah id harus 1 sampai %d", ErrValidasi, MaxBulkIDs)
	}

	cakupan, err := uc.Cakupan(ctx, p)
	if err != nil {
		uc.log.Error("lapangan: baca cakupan gagal", map[string]interface{}{"error": err.Error()})
		return HasilBulk{}, fmt.Errorf("bulk %s: %w", aksi, err)
	}

	if action == domain.BulkUnduhDataAcuan {
		found, err := uc.repo.Aksesibel(ctx, unik, cakupan)
		if err != nil {
			return HasilBulk{}, fmt.Errorf("bulk %s: %w", aksi, err)
		}
		return hitungHasil(unik, found), nil
	}

	changed, err := uc.repo.TandaiSelesai(ctx, unik, cakupan, uc.now())
	if err != nil {
		uc.log.Error("lapangan: tandai selesai gagal", map[string]interface{}{"count": len(unik), "error": err.Error()})
		return HasilBulk{}, fmt.Errorf("bulk %s: %w", aksi, err)
	}

	hasil := hitungHasil(unik, changed)
	if len(changed) == 0 {
		return hasil, nil
	}
	ids := make([]idx.ID, len(changed))
	for i, c := range changed {
		ids[i] = c.ID
	}
	if err := uc.audit.Write(ctx, audit.Entry{
		AktorID: p.ID, Aksi: "TUGAS_" + strings.ToUpper(aksi), Entitas: "tugas_lapangan",
		EntitasID: fmt.Sprintf("%d tugas", len(changed)), Hasil: audit.HasilBerhasil,
		Metadata: map[string]any{"ids": ids, "gagal": hasil.Gagal},
	}); err != nil {
		uc.log.Warn("lapangan: gagal mencatat audit", map[string]interface{}{"aksi": aksi, "error": err.Error()})
	}
	return hasil, nil
}

// hitungHasil menghitung rujukan yang menemukan baris di antara found (berhasil) dan yang tidak (gagal).
func hitungHasil(refs []idx.RujukanID, found []domain.TugasRef) HasilBulk {
	var hasil HasilBulk
	for _, ref := range refs {
		cocok := slices.ContainsFunc(found, func(t domain.TugasRef) bool { return t.Cocok(ref) })
		if cocok {
			hasil.Berhasil++
		} else {
			hasil.Gagal++
		}
	}
	return hasil
}

// dedup membuang rujukan kosong dan ganda dengan mempertahankan urutan.
func dedup(refs []idx.RujukanID) []idx.RujukanID {
	seen := make(map[idx.RujukanID]struct{}, len(refs))
	out := make([]idx.RujukanID, 0, len(refs))
	for _, ref := range refs {
		if _, dup := seen[ref]; dup || (ref.ID == 0 && ref.Kode == "") {
			continue
		}
		seen[ref] = struct{}{}
		out = append(out, ref)
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
