package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/harga/domain"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/garda-inflasi/api/pkg/logger"
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

// Cakupan menurunkan jangkauan baris pemanggil dari perannya dan kabupaten tugasnya (02-B), dibaca dari basis data
// pada tiap permintaan, bukan dari klaim token: klaim `kab` hanya untuk tampilan dan bisa basi sampai access token
// habis setelah admin mengubah kabupaten seseorang. Gagal-tutup: galat basis data dikembalikan sebagai galat
// (tidak pernah dijawab dengan cakupan yang lebih luas), dan akun yang tidak ada mendapat cakupan tertutup.
func (uc *UseCase) Cakupan(ctx context.Context, penggunaID string) (domain.Cakupan, error) {
	pengguna, err := uc.repo.PenggunaCakupan(ctx, penggunaID)
	if err != nil {
		if errors.Is(err, domain.ErrPenggunaTidakAda) {
			return domain.Cakupan{}, nil
		}
		uc.log.Error("baca cakupan pengguna failed", map[string]interface{}{"error": err.Error()})
		return domain.Cakupan{}, fmt.Errorf("baca cakupan pengguna: %w", err)
	}
	return domain.CakupanPeran(pengguna.Peran, pengguna.KabupatenKode, penggunaID), nil
}

// CakupanBaca adalah cakupan untuk membuka satu entri (detail, riwayat, versi, lineage). Pemegang harga.lihat
// memakai cakupan perannya; tanpa harga.lihat (mis. petugas yang hanya punya harga.input) hanya entri yang ia
// catat sendiri, apa pun perannya.
func (uc *UseCase) CakupanBaca(ctx context.Context, penggunaID string, lihat bool) (domain.Cakupan, error) {
	if !lihat {
		return domain.CakupanMilik(penggunaID), nil
	}
	return uc.Cakupan(ctx, penggunaID)
}

// ListHarga mengembalikan satu halaman entri beserta jumlah seluruh hasil filter. Hanya entri dalam
// filter.Cakupan yang dimuat; cakupan nol (tertutup) menghasilkan daftar kosong.
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

// GetHargaByID mengembalikan satu entri menurut ID BIGINT atau kode lama (alias tautan lama) bila masuk cakupan.
// Entri di luar cakupan dijawab ErrNotFound, sama dengan entri yang tidak ada (jawabannya tidak membocorkan
// keberadaan entri).
func (uc *UseCase) GetHargaByID(ctx context.Context, ref idx.RujukanID, cakupan domain.Cakupan) (*domain.EntriView, error) {
	entri, err := uc.repo.GetByID(ctx, ref)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, ErrNotFound
		}
		uc.log.Error("get harga failed", map[string]interface{}{"id": ref.ID.String(), "kode": ref.Kode, "error": err.Error()})
		return nil, fmt.Errorf("get harga: %w", err)
	}
	if !cakupan.Mengizinkan(entri.KabupatenKode, entri.PetugasID) {
		return nil, ErrNotFound
	}
	return entri, nil
}

// Ringkasan menghitung entri per status pada hari terakhir, hanya yang masuk cakupan.
func (uc *UseCase) Ringkasan(ctx context.Context, hari int, cakupan domain.Cakupan) (domain.Ringkasan, error) {
	if hari <= 0 {
		hari = DefaultRingkasanHari
	}
	sejak := uc.now().AddDate(0, 0, -min(hari, MaxHari))
	ringkasan, err := uc.repo.Ringkasan(ctx, sejak, cakupan)
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
	KodePedagang  *string
	Catatan       *string
	DicatatPada   *time.Time
	Lat, Lng      *float64
	// IdempotencyKey kosong berarti kiriman tanpa jaminan idempotensi.
	IdempotencyKey string
	// Pertahankan menyimpan entri walau bentrok dengan entri petugas lain.
	Pertahankan bool
}

// HasilKirim adalah entri yang tersimpan; Diulang true bila kiriman ini mengulang
// kunci idempotensi yang sudah tersimpan.
type HasilKirim struct {
	Entri   *domain.EntriHarga
	Diulang bool
}

// BentrokError dikembalikan bila petugas lain sudah mengirim harga berbeda untuk
// pasar, komoditas, dan tipe yang sama; Server memuat versi pembandingnya.
type BentrokError struct {
	Server domain.Bentrok
}

func (e *BentrokError) Error() string {
	return "harga bentrok dengan entri petugas lain"
}

// CreateHarga menyimpan entri berstatus DIAJUKAN atas nama petugas dan mencatatnya di audit trail.
// Kiriman ulang dengan kunci idempotensi yang sama mengembalikan entri semula tanpa menyimpan atau
// mencatat ulang; entri yang bentrok dengan petugas lain ditolak dengan *BentrokError.
func (uc *UseCase) CreateHarga(ctx context.Context, in CreateInput, petugasID string) (HasilKirim, error) {
	now := uc.now()
	dicatat := now
	if in.DicatatPada != nil {
		if in.DicatatPada.After(now.Add(toleransiWaktu)) {
			return HasilKirim{}, fmt.Errorf("%w: waktu pencatatan di masa depan", ErrValidasi)
		}
		dicatat = *in.DicatatPada
	}
	if (in.Lat == nil) != (in.Lng == nil) {
		return HasilKirim{}, fmt.Errorf("%w: lat dan lng harus diisi bersamaan", ErrValidasi)
	}

	satuan, err := uc.repo.SatuanKomoditas(ctx, in.KomoditasKode, in.PasarID)
	if err != nil {
		if errors.Is(err, domain.ErrReferensiTidakAda) {
			return HasilKirim{}, fmt.Errorf("%w: %w", ErrValidasi, err)
		}
		return HasilKirim{}, fmt.Errorf("cari referensi harga: %w", err)
	}

	// ID tidak diisi: basis data yang memberinya (BIGINT identity) dan GORM membacanya lewat RETURNING saat Simpan.
	// Kode dibiarkan NULL: UI memakai kode ?? id, dan baris baru tidak punya kode tampilan.
	entri := &domain.EntriHarga{
		KomoditasKode: in.KomoditasKode,
		PasarID:       in.PasarID,
		Tipe:          in.Tipe,
		Harga:         in.Harga,
		Satuan:        satuan,
		Kualitas:      in.Kualitas,
		Sumber:        "LAPANGAN",
		Status:        domain.StatusDiajukan,
		KodePedagang:  in.KodePedagang,
		Catatan:       in.Catatan,
		Lat:           in.Lat,
		Lng:           in.Lng,
		DicatatPada:   dicatat,
		PetugasID:     &petugasID,
		DibuatPada:    now,
		DiupdatePada:  now,
	}
	if in.IdempotencyKey != "" {
		entri.IdempotencyKey = &in.IdempotencyKey
	}
	if entri.Kualitas == "" {
		entri.Kualitas = "MEDIUM"
	}

	konteks, err := uc.repo.KonteksValidasi(ctx, entri)
	if err != nil {
		if errors.Is(err, domain.ErrReferensiTidakAda) {
			return HasilKirim{}, fmt.Errorf("%w: %w", ErrValidasi, err)
		}
		return HasilKirim{}, fmt.Errorf("konteks validasi harga: %w", err)
	}
	cek := domain.Validasi(*entri, konteks)
	entri.DeviasiPct, entri.SkorAnomali, entri.DalamGeofence, entri.FlagValidasi = cek.DeviasiPct, cek.SkorAnomali, cek.DalamGeofence, cek.Flag

	hasil, err := uc.repo.Simpan(ctx, domain.Pengiriman{Entri: entri, Pertahankan: in.Pertahankan})
	if err != nil {
		uc.log.Error("create harga failed", map[string]interface{}{"error": err.Error()})
		return HasilKirim{}, fmt.Errorf("create harga: %w", err)
	}
	if hasil.Bentrok != nil {
		return HasilKirim{}, &BentrokError{Server: *hasil.Bentrok}
	}
	if hasil.Diulang {
		return HasilKirim{Entri: hasil.Entri, Diulang: true}, nil
	}

	uc.catat(ctx, audit.Entry{
		AktorID: petugasID, Aksi: "HARGA_INPUT", Entitas: "entri_harga", EntitasID: entri.ID.String(),
		Hasil: audit.HasilBerhasil, Sesudah: entri,
	})
	return HasilKirim{Entri: entri}, nil
}

// Batas panjang alasan tertulis.
const (
	MinAlasan = 3
	MaxAlasan = 1000
)

// RincianBulk menjelaskan satu entri yang dilewati pada aksi massal.
type RincianBulk struct {
	ID    idx.ID
	Sebab string
}

// HasilBulk melaporkan jumlah entri yang berubah dan yang dilewati beserta sebabnya.
type HasilBulk struct {
	Berhasil int
	Gagal    int
	// Rincian memuat tiap entri yang dilewati, urut seperti permintaan.
	Rincian []RincianBulk
}

// BulkAction menjalankan aksi massal sesuai state machine domain.Evaluasi. Entri yang tidak ditemukan,
// di luar cakupan aktor (dilaporkan tidak_ditemukan, sama dengan id yang tidak ada), statusnya tidak memenuhi
// syarat aksi, atau yang melanggar pemisahan tugas dilewati dan dilaporkan di Rincian. Alasan wajib untuk tolak
// dan tampil bagi penginput. Pemeriksaan izin aktor ada di delivery.
func (uc *UseCase) BulkAction(ctx context.Context, ids []idx.ID, aksi, aktorID, alasan string, cakupan domain.Cakupan) (HasilBulk, error) {
	action := domain.BulkAction(aksi)
	if !action.Valid() {
		return HasilBulk{}, ErrInvalidAction
	}
	unik := dedup(ids)
	if len(unik) == 0 || len(unik) > MaxBulkIDs {
		return HasilBulk{}, fmt.Errorf("%w: jumlah id harus 1 sampai %d", ErrValidasi, MaxBulkIDs)
	}
	alasan = strings.TrimSpace(alasan)
	if len(alasan) > MaxAlasan || (action.AlasanWajib() && len(alasan) < MinAlasan) {
		return HasilBulk{}, fmt.Errorf("%w: alasan %s", ErrValidasi, syaratAlasan(action))
	}

	hasil, err := uc.repo.Transisi(ctx, unik, action, aktorID, alasan, uc.now(), cakupan)
	if err != nil {
		uc.log.Error("bulk action failed", map[string]interface{}{"action": aksi, "count": len(unik), "error": err.Error()})
		return HasilBulk{}, fmt.Errorf("bulk action: %w", err)
	}

	var out HasilBulk
	var berubah []idx.ID
	for _, h := range hasil {
		if h.Diterapkan() {
			berubah = append(berubah, h.ID)
			continue
		}
		out.Rincian = append(out.Rincian, RincianBulk{ID: h.ID, Sebab: h.Sebab})
	}
	out.Berhasil, out.Gagal = len(berubah), len(out.Rincian)
	if out.Berhasil == 0 {
		return out, nil
	}

	uc.refresh.Trigger()
	metadata := map[string]any{"ids": berubah, "gagal": out.Gagal}
	if alasan != "" {
		metadata["alasan"] = alasan
	}
	uc.catat(ctx, audit.Entry{
		AktorID: aktorID, Aksi: "HARGA_" + strings.ToUpper(aksi), Entitas: "entri_harga",
		EntitasID: fmt.Sprintf("%d entri", out.Berhasil), Hasil: audit.HasilBerhasil, Metadata: metadata,
	})
	return out, nil
}

func syaratAlasan(action domain.BulkAction) string {
	if action.AlasanWajib() {
		return fmt.Sprintf("wajib diisi (%d sampai %d karakter)", MinAlasan, MaxAlasan)
	}
	return fmt.Sprintf("maksimal %d karakter", MaxAlasan)
}

// Riwayat mengembalikan perpindahan status entri (ID BIGINT atau kode lama) bila entri masuk cakupan; di luar
// cakupan dijawab ErrNotFound, sama dengan entri yang tidak ada.
func (uc *UseCase) Riwayat(ctx context.Context, ref idx.RujukanID, cakupan domain.Cakupan) ([]domain.RiwayatView, error) {
	entri, err := uc.GetHargaByID(ctx, ref, cakupan)
	if err != nil {
		return nil, err
	}
	items, err := uc.repo.Riwayat(ctx, entri.ID)
	if err != nil {
		uc.log.Error("riwayat harga failed", map[string]interface{}{"id": entri.ID.String(), "error": err.Error()})
		return nil, fmt.Errorf("riwayat harga: %w", err)
	}
	return items, nil
}

// Versi mengembalikan seluruh versi isi entri, yang terlama lebih dulu. Aturan akses sama dengan Riwayat.
func (uc *UseCase) Versi(ctx context.Context, ref idx.RujukanID, cakupan domain.Cakupan) ([]domain.VersiView, error) {
	entri, err := uc.GetHargaByID(ctx, ref, cakupan)
	if err != nil {
		return nil, err
	}
	items, err := uc.repo.Versi(ctx, entri.ID)
	if err != nil {
		uc.log.Error("versi harga failed", map[string]interface{}{"id": entri.ID.String(), "error": err.Error()})
		return nil, fmt.Errorf("versi harga: %w", err)
	}
	return items, nil
}

// Lineage mengembalikan jejak asal entri harga (PRD §1.4): pencatat, validasi otomatis, validasi, publikasi,
// penolakan, versi, riwayat status, dan rujukan audit. Aturan akses sama dengan Riwayat.
func (uc *UseCase) Lineage(ctx context.Context, ref idx.RujukanID, cakupan domain.Cakupan) (domain.Lineage, error) {
	entri, err := uc.GetHargaByID(ctx, ref, cakupan)
	if err != nil {
		return domain.Lineage{}, err
	}
	lineage, err := uc.repo.Lineage(ctx, *entri)
	if err != nil {
		uc.log.Error("lineage harga failed", map[string]interface{}{"id": entri.ID.String(), "error": err.Error()})
		return domain.Lineage{}, fmt.Errorf("lineage harga: %w", err)
	}
	return lineage, nil
}

// catat menulis audit; kegagalan mencatat tidak membatalkan aksi yang sudah tersimpan.
func (uc *UseCase) catat(ctx context.Context, entry audit.Entry) {
	if err := uc.audit.Write(ctx, entry); err != nil {
		uc.log.Warn("harga: gagal mencatat audit", map[string]interface{}{"aksi": entry.Aksi, "error": err.Error()})
	}
}

func dedup(ids []idx.ID) []idx.ID {
	seen := make(map[idx.ID]struct{}, len(ids))
	out := make([]idx.ID, 0, len(ids))
	for _, id := range ids {
		if _, dup := seen[id]; dup || id == 0 {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
