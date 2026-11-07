package usecase

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/cold-storage/domain"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/garda-inflasi/api/pkg/logger"
)

const (
	// MaxTelemetriBatch membatasi pembacaan dalam satu kiriman sensor/gateway.
	MaxTelemetriBatch = 500
	// DefaultJamTelemetri dan MaxJamTelemetri adalah rentang riwayat suhu pada detail fasilitas.
	DefaultJamTelemetri = 48
	MaxJamTelemetri     = 720
	// MaxTitikTelemetri membatasi jumlah titik riwayat yang dikembalikan (yang terbaru).
	MaxTitikTelemetri = 1000
	// DetailTransaksiLimit adalah jumlah transaksi terbaru pada detail fasilitas.
	DetailTransaksiLimit = 20
	// maxWaktuMasaDepan menoleransi selisih jam sensor terhadap server.
	maxWaktuMasaDepan = 5 * time.Minute
	// maxVolumeTon menolak volume yang jelas salah ketik.
	maxVolumeTon = 1_000_000
)

// InputError adalah galat validasi masukan; pesannya aman ditampilkan ke pengguna.
type InputError struct{ Msg string }

func (e *InputError) Error() string { return e.Msg }

func invalid(format string, args ...any) error {
	return &InputError{Msg: fmt.Sprintf(format, args...)}
}

// Operasi menangani telemetri dan transaksi stok (13-B).
type Operasi struct {
	repo  domain.OperasiRepository
	log   *logger.Logger
	audit audit.Writer
	now   func() time.Time
}

func NewOperasi(repo domain.OperasiRepository, log *logger.Logger, auditWriter audit.Writer) *Operasi {
	return &Operasi{repo: repo, log: log, audit: auditWriter, now: time.Now}
}

// WithClock mengganti sumber waktu; dipakai pengujian.
func (o *Operasi) WithClock(now func() time.Time) *Operasi {
	o.now = now
	return o
}

func (o *Operasi) record(ctx context.Context, entry audit.Entry) {
	if err := o.audit.Write(ctx, entry); err != nil {
		o.log.Warn("cold-storage: failed to record audit log", map[string]interface{}{"aksi": entry.Aksi, "error": err.Error()})
	}
}

// idFasilitas menentukan UUID fasilitas dari rujukan: UUID dipakai apa adanya (fasilitas yang tidak ada akan
// dilaporkan ErrFasilitasTidakAda oleh pembacaan atau penguncian berikutnya), kode dicari lebih dulu.
func (o *Operasi) idFasilitas(ctx context.Context, ref idx.RujukanUUID) (string, error) {
	if ref.ID != "" {
		return ref.ID, nil
	}
	if ref.Kode == "" {
		return "", domain.ErrFasilitasTidakAda
	}
	return o.repo.IDFasilitasDariKode(ctx, ref.Kode)
}

// ── Detail fasilitas ──────────────────────────────────────────────────

// TitikSuhu adalah satu pembacaan pada riwayat suhu.
type TitikSuhu struct {
	Waktu         time.Time
	SuhuC         float64
	KelembapanPct *float64
	Sumber        string
	DiLuarRentang bool
}

// Detail adalah data halaman detail satu fasilitas.
type Detail struct {
	Fasilitas domain.FasilitasView
	Telemetri []TitikSuhu
	Stok      []domain.StokView
	Transaksi []domain.TransaksiView
}

// Detail mengambil fasilitas, riwayat suhu `jam` jam terakhir (terlama dulu),
// lot tersimpan, dan transaksi terbaru. jam <= 0 memakai DefaultJamTelemetri.
func (o *Operasi) Detail(ctx context.Context, ref idx.RujukanUUID, jam int) (*Detail, error) {
	if jam <= 0 {
		jam = DefaultJamTelemetri
	}
	jam = min(jam, MaxJamTelemetri)

	id, err := o.idFasilitas(ctx, ref)
	if err != nil {
		return nil, err
	}
	f, err := o.repo.GetFasilitas(ctx, id)
	if err != nil {
		return nil, err
	}
	isiTurunan(f)

	sejak := o.now().Add(-time.Duration(jam) * time.Hour)
	rows, err := o.repo.ListTelemetri(ctx, id, sejak, MaxTitikTelemetri)
	if err != nil {
		return nil, err
	}
	stok, err := o.repo.StokFasilitas(ctx, id)
	if err != nil {
		return nil, err
	}
	trx, err := o.repo.ListTransaksi(ctx, id, DetailTransaksiLimit)
	if err != nil {
		return nil, err
	}

	now := o.now()
	for i := range stok {
		stok[i].VolumeTon = round1(stok[i].VolumeTon)
		stok[i].SisaMasaSimpanHari = sisaHari(now, stok[i].KedaluwarsaPada)
	}

	// Repositori mengembalikan terbaru lebih dulu; grafik membutuhkan terlama dulu.
	titik := make([]TitikSuhu, len(rows))
	for i, row := range rows {
		titik[len(rows)-1-i] = TitikSuhu{
			Waktu: row.Waktu, SuhuC: row.SuhuC, KelembapanPct: row.KelembapanPct, Sumber: row.Sumber,
			DiLuarRentang: domain.DiLuarRentang(row.SuhuC, f.BatasMinC, f.BatasMaksC),
		}
	}
	return &Detail{Fasilitas: *f, Telemetri: titik, Stok: stok, Transaksi: trx}, nil
}

// ── Telemetri ─────────────────────────────────────────────────────────

// Pembacaan adalah satu pembacaan sensor yang dikirim. FasilitasID boleh UUID atau kode fasilitas (CS-01).
type Pembacaan struct {
	FasilitasID   string
	Waktu         time.Time
	SuhuC         float64
	KelembapanPct *float64
	Sumber        string
}

// IngestInput adalah satu kiriman telemetri.
type IngestInput struct {
	Pembacaan []Pembacaan
	ActorID   string
}

// AlarmSuhu adalah pembacaan yang keluar dari batas alarm fasilitasnya.
type AlarmSuhu struct {
	FasilitasID string
	Waktu       time.Time
	SuhuC       float64
	BatasMinC   float64
	BatasMaksC  float64
}

// IngestOutcome adalah hasil satu kiriman telemetri.
type IngestOutcome struct {
	Diterima      int
	Duplikat      int
	DiLuarRentang []AlarmSuhu
}

// Ingest memvalidasi dan menyimpan pembacaan. Pembacaan yang sudah ada
// (fasilitas + waktu sama) dilewati sehingga kiriman ulang aman.
func (o *Operasi) Ingest(ctx context.Context, in IngestInput) (IngestOutcome, error) {
	if len(in.Pembacaan) == 0 {
		return IngestOutcome{}, invalid("tidak ada pembacaan")
	}
	if len(in.Pembacaan) > MaxTelemetriBatch {
		return IngestOutcome{}, invalid("maksimal %d pembacaan per kiriman", MaxTelemetriBatch)
	}

	batas := map[string]domain.FasilitasView{} // menurut UUID
	rujukan := map[string]string{}             // teks isian -> UUID
	rows := make([]domain.TelemetriSuhu, 0, len(in.Pembacaan))
	var alarm []AlarmSuhu
	batasWaktu := o.now().Add(maxWaktuMasaDepan)

	for i, p := range in.Pembacaan {
		teks := strings.TrimSpace(p.FasilitasID)
		if teks == "" {
			return IngestOutcome{}, invalid("pembacaan %d: fasilitas_id wajib diisi", i+1)
		}
		if p.SuhuC < -80 || p.SuhuC > 60 || math.IsNaN(p.SuhuC) {
			return IngestOutcome{}, invalid("pembacaan %d: suhu_c harus antara -80 dan 60", i+1)
		}
		if p.KelembapanPct != nil && (*p.KelembapanPct < 0 || *p.KelembapanPct > 100) {
			return IngestOutcome{}, invalid("pembacaan %d: kelembapan_pct harus antara 0 dan 100", i+1)
		}
		if p.Waktu.IsZero() || p.Waktu.After(batasWaktu) {
			return IngestOutcome{}, invalid("pembacaan %d: waktu kosong atau di masa depan", i+1)
		}
		sumber := p.Sumber
		switch sumber {
		case "":
			sumber = domain.SumberSensor
		case domain.SumberSensor, domain.SumberManual:
		default:
			return IngestOutcome{}, invalid("pembacaan %d: sumber harus SENSOR atau MANUAL", i+1)
		}

		id, ok := rujukan[teks]
		if !ok {
			var err error
			id, err = o.idFasilitas(ctx, domain.RujukanFasilitas(teks))
			if err != nil {
				if errors.Is(err, domain.ErrFasilitasTidakAda) {
					return IngestOutcome{}, fmt.Errorf("pembacaan %d (%s): %w", i+1, teks, err)
				}
				return IngestOutcome{}, err
			}
			rujukan[teks] = id
		}
		f, ok := batas[id]
		if !ok {
			found, err := o.repo.GetFasilitas(ctx, id)
			if err != nil {
				if errors.Is(err, domain.ErrFasilitasTidakAda) {
					return IngestOutcome{}, fmt.Errorf("pembacaan %d (%s): %w", i+1, teks, err)
				}
				return IngestOutcome{}, err
			}
			if found.Status == domain.StatusFasilitasRencana {
				return IngestOutcome{}, fmt.Errorf("pembacaan %d (%s): %w", i+1, teks, domain.ErrFasilitasTidakAktif)
			}
			isiTurunan(found)
			f = *found
			id = f.ID // bentuk kanonik dari basis data
			batas[id] = f
			rujukan[teks] = id
		}

		rows = append(rows, domain.TelemetriSuhu{FasilitasID: id, Waktu: p.Waktu.UTC(), SuhuC: p.SuhuC, KelembapanPct: p.KelembapanPct, Sumber: sumber})
		if domain.DiLuarRentang(p.SuhuC, f.BatasMinC, f.BatasMaksC) {
			alarm = append(alarm, AlarmSuhu{FasilitasID: id, Waktu: p.Waktu.UTC(), SuhuC: p.SuhuC, BatasMinC: f.BatasMinC, BatasMaksC: f.BatasMaksC})
		}
	}

	inserted, err := o.repo.SimpanTelemetri(ctx, rows)
	if err != nil {
		o.log.Error("cold-storage: failed to store telemetry", map[string]interface{}{"error": err.Error()})
		return IngestOutcome{}, err
	}
	out := IngestOutcome{Diterima: int(inserted), Duplikat: len(rows) - int(inserted), DiLuarRentang: alarm}

	ids := make([]string, 0, len(batas))
	for id := range batas {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	// entitas_id berisi UUID bila hanya satu fasilitas (riwayat per fasilitas); kiriman banyak fasilitas dicatat
	// sebagai ringkasan dan daftar UUID-nya ada di metadata (kolom entitas_id terbatas 255 karakter).
	entitasID := fmt.Sprintf("%d fasilitas", len(ids))
	if len(ids) == 1 {
		entitasID = ids[0]
	}
	o.record(ctx, audit.Entry{
		AktorID: in.ActorID, Aksi: "INGEST_TELEMETRI", Entitas: "cold_storage", EntitasID: entitasID,
		Hasil:    audit.HasilBerhasil,
		Metadata: map[string]any{"fasilitas_ids": ids, "diterima": out.Diterima, "duplikat": out.Duplikat, "di_luar_rentang": len(alarm)},
	})
	return out, nil
}

// ── Transaksi stok ────────────────────────────────────────────────────

// TransaksiRequest adalah permintaan satu transaksi stok.
type TransaksiRequest struct {
	Jenis string
	// LotID adalah ID BIGINT lot (KELUAR dan PINDAH); nol berarti tidak diisi.
	LotID idx.ID
	// FasilitasID dan FasilitasTujuanID boleh UUID atau kode fasilitas (CS-01).
	FasilitasID       string
	FasilitasTujuanID string
	KomoditasKode     string
	Pemilik           string
	Alasan            string
	VolumeTon         float64
	Semua             bool
	KedaluwarsaPada   *time.Time
	ActorID           string
}

// Transaksi mencatat masuk/keluar/pindah stok secara atomik dan menuliskannya
// ke audit trail. Galat domain (stok tak cukup, kapasitas penuh, dst.) diteruskan.
func (o *Operasi) Transaksi(ctx context.Context, req TransaksiRequest) (domain.TransaksiHasil, error) {
	in, err := o.susunTransaksi(ctx, req)
	if err != nil {
		return domain.TransaksiHasil{}, err
	}

	hasil, err := o.repo.CatatTransaksi(ctx, in)
	if err != nil {
		if !isDomainError(err) {
			o.log.Error("cold-storage: transaction failed", map[string]interface{}{"jenis": req.Jenis, "error": err.Error()})
		}
		return domain.TransaksiHasil{}, err
	}

	meta := map[string]any{"transaksi_id": hasil.TransaksiID.String(), "volume_ton": hasil.VolumeTon, "alasan": in.Alasan}
	if hasil.LotBaruID != nil {
		meta["lot_baru_id"] = hasil.LotBaruID.String()
	}
	var sebelum any
	if hasil.Sebelum != nil {
		sebelum = hasil.Sebelum
	}
	o.record(ctx, audit.Entry{
		AktorID: req.ActorID, Aksi: "TRANSAKSI_STOK_" + hasil.Jenis, Entitas: "stok_cold_storage", EntitasID: hasil.LotID.String(),
		Hasil: audit.HasilBerhasil, Sebelum: sebelum, Sesudah: hasil.Sesudah, Metadata: meta,
	})
	return hasil, nil
}

// susunTransaksi memvalidasi permintaan dan membentuk masukan repositori. ID transaksi dan lot baru tidak dibuat
// di sini: basis data yang memberikannya.
func (o *Operasi) susunTransaksi(ctx context.Context, req TransaksiRequest) (domain.TransaksiInput, error) {
	now := o.now()
	in := domain.TransaksiInput{
		Jenis: req.Jenis, Waktu: now, OlehID: req.ActorID,
		Alasan:        strings.TrimSpace(req.Alasan),
		LotID:         req.LotID,
		KomoditasKode: strings.TrimSpace(req.KomoditasKode), Pemilik: strings.TrimSpace(req.Pemilik),
		VolumeTon: req.VolumeTon, Semua: req.Semua, KedaluwarsaPada: req.KedaluwarsaPada,
	}
	fasilitas := strings.TrimSpace(req.FasilitasID)
	tujuan := strings.TrimSpace(req.FasilitasTujuanID)

	if !in.Semua || req.Jenis == domain.TransaksiMasuk {
		if math.IsNaN(in.VolumeTon) || in.VolumeTon <= 0 || in.VolumeTon > maxVolumeTon {
			return in, invalid("volume_ton harus lebih dari 0 dan paling banyak %d", maxVolumeTon)
		}
	}

	switch req.Jenis {
	case domain.TransaksiMasuk:
		if fasilitas == "" || in.KomoditasKode == "" || in.Pemilik == "" {
			return in, invalid("fasilitas_id, komoditas_kode, dan pemilik wajib diisi untuk MASUK")
		}
		if in.KedaluwarsaPada != nil && !in.KedaluwarsaPada.After(now) {
			return in, invalid("kedaluwarsa_pada harus di masa depan")
		}
		in.Semua = false
		in.LotID = 0 // lot baru: ID dari basis data
		id, err := o.idFasilitas(ctx, domain.RujukanFasilitas(fasilitas))
		if err != nil {
			return in, err
		}
		in.FasilitasID = id
	case domain.TransaksiKeluar:
		if in.LotID == 0 {
			return in, invalid("lot_id wajib diisi untuk KELUAR")
		}
		if in.Alasan == "" {
			return in, invalid("alasan wajib diisi untuk KELUAR")
		}
	case domain.TransaksiPindah:
		if in.LotID == 0 || tujuan == "" {
			return in, invalid("lot_id dan fasilitas_tujuan_id wajib diisi untuk PINDAH")
		}
		id, err := o.idFasilitas(ctx, domain.RujukanFasilitas(tujuan))
		if err != nil {
			return in, err
		}
		in.FasilitasTujuanID = id
	default:
		return in, invalid("jenis harus MASUK, KELUAR, atau PINDAH")
	}
	return in, nil
}

// isDomainError membedakan penolakan bisnis yang wajar dari kegagalan sistem.
func isDomainError(err error) bool {
	for _, target := range []error{
		domain.ErrFasilitasTidakAda, domain.ErrFasilitasTidakAktif, domain.ErrLotTidakAda, domain.ErrLotHabis,
		domain.ErrKomoditasTidakAda, domain.ErrStokTidakCukup, domain.ErrKapasitasTidakCukup,
		domain.ErrKapasitasDiBawahStok, domain.ErrFasilitasSama, domain.ErrBatasSuhuTidakValid,
	} {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}

// ── Fasilitas ─────────────────────────────────────────────────────────

// UbahFasilitas menerapkan perubahan parsial (kapasitas, batas alarm, status).
func (o *Operasi) UbahFasilitas(ctx context.Context, ref idx.RujukanUUID, patch domain.FasilitasPatch, actorID string) (*domain.FasilitasView, error) {
	if patch.KapasitasTon == nil && patch.SuhuMinC == nil && patch.SuhuMaksC == nil && patch.Status == nil {
		return nil, invalid("tidak ada perubahan")
	}
	if patch.KapasitasTon != nil && *patch.KapasitasTon <= 0 {
		return nil, invalid("kapasitas_ton harus lebih dari 0")
	}
	if patch.Status != nil {
		switch *patch.Status {
		case domain.StatusFasilitasOperasional, domain.StatusFasilitasPerawatan, domain.StatusFasilitasRencana:
		default:
			return nil, invalid("status harus OPERASIONAL, PERAWATAN, atau RENCANA")
		}
	}

	id, err := o.idFasilitas(ctx, ref)
	if err != nil {
		return nil, err
	}
	before, after, err := o.repo.UpdateFasilitas(ctx, id, patch)
	if err != nil {
		if !isDomainError(err) {
			o.log.Error("cold-storage: failed to update facility", map[string]interface{}{"id": id, "error": err.Error()})
		}
		return nil, err
	}
	isiTurunan(&after)

	o.record(ctx, audit.Entry{
		AktorID: actorID, Aksi: "UBAH_FASILITAS_COLD_STORAGE", Entitas: "cold_storage", EntitasID: id, Hasil: audit.HasilBerhasil,
		Sebelum: fasilitasSnapshot(before), Sesudah: fasilitasSnapshot(after),
	})
	return &after, nil
}

func fasilitasSnapshot(f domain.FasilitasView) map[string]any {
	return map[string]any{"kapasitas_ton": f.KapasitasTon, "suhu_min_c": f.SuhuMinC, "suhu_maks_c": f.SuhuMaksC, "status": f.Status}
}

// ── Aksi massal yang mengubah stok ────────────────────────────────────

// IsMutatingBulk melaporkan apakah aksi massal ditangani modul ini (mengubah stok).
func IsMutatingBulk(action string) bool {
	switch domain.BulkAction(action) {
	case domain.BulkTandaiPrioritasJual, domain.BulkBuang:
		return true
	}
	return false
}

// alasanBuang dicatat pada transaksi KELUAR hasil aksi massal "buang".
const alasanBuang = "Dibuang (aksi massal)"

// BulkMutasi menjalankan aksi massal yang mengubah stok dan mengembalikan hasil per lot.
func (o *Operasi) BulkMutasi(ctx context.Context, in BulkInput) (BulkOutcome, error) {
	if !IsMutatingBulk(in.Action) {
		return BulkOutcome{}, ErrInvalidAction
	}
	ids := uniqueNonEmpty(in.IDs)
	if len(ids) == 0 {
		return BulkOutcome{}, ErrNoIDs
	}
	if len(ids) > MaxBulkIDs {
		return BulkOutcome{}, ErrTooManyIDs
	}

	var baris []domain.BarisBulk
	switch domain.BulkAction(in.Action) {
	case domain.BulkTandaiPrioritasJual:
		ok, err := o.repo.TandaiPrioritasJual(ctx, ids)
		if err != nil {
			o.log.Error("cold-storage: bulk action failed", map[string]interface{}{"action": in.Action, "error": err.Error()})
			return BulkOutcome{}, fmt.Errorf("bulk action: %w", err)
		}
		baris = gabungBaris(ids, ok)
	case domain.BulkBuang:
		baris = o.buangLot(ctx, ids, in.ActorID)
	}

	out := BulkOutcome{Baris: baris}
	var transaksi []string
	for _, b := range baris {
		if b.Berhasil {
			out.Berhasil++
			out.VolumeTon += b.VolumeTon
			if b.TransaksiID != 0 {
				transaksi = append(transaksi, b.TransaksiID.String())
			}
		} else {
			out.Gagal++
		}
	}
	out.VolumeTon = round1(out.VolumeTon)

	hasil := audit.HasilBerhasil
	if out.Berhasil == 0 {
		hasil = audit.HasilGagal
	}
	meta := map[string]any{"ids": ids, "gagal": out.Gagal, "volume_ton": out.VolumeTon}
	if len(transaksi) > 0 {
		meta["transaksi_ids"] = transaksi
	}
	o.record(ctx, audit.Entry{
		AktorID: in.ActorID, Aksi: "BULK_" + in.Action, Entitas: "stok_cold_storage",
		EntitasID: fmt.Sprintf("%d lot", out.Berhasil), Hasil: hasil, Metadata: meta,
	})
	return out, nil
}

// buangLot mengeluarkan seluruh sisa tiap lot (satu transaksi atomik per lot),
// sehingga kegagalan satu lot tidak membatalkan yang lain.
func (o *Operasi) buangLot(ctx context.Context, ids []idx.ID, actorID string) []domain.BarisBulk {
	baris := make([]domain.BarisBulk, len(ids))
	for i, id := range ids {
		baris[i] = domain.BarisBulk{ID: id}
		hasil, err := o.repo.CatatTransaksi(ctx, domain.TransaksiInput{
			Jenis: domain.TransaksiKeluar, Waktu: o.now(), OlehID: actorID,
			Alasan: alasanBuang, LotID: id, Semua: true,
		})
		switch {
		case err == nil:
			baris[i].Berhasil, baris[i].VolumeTon, baris[i].TransaksiID = true, hasil.VolumeTon, hasil.TransaksiID
		case errors.Is(err, domain.ErrLotTidakAda), errors.Is(err, domain.ErrLotHabis):
			baris[i].Pesan = "Lot tidak ditemukan atau sudah habis"
		default:
			o.log.Error("cold-storage: discard lot failed", map[string]interface{}{"lot": id.String(), "error": err.Error()})
			baris[i].Pesan = "Gagal memproses lot"
		}
	}
	return baris
}

// gabungBaris menyusun hasil per lot menurut urutan permintaan dari lot yang berhasil.
func gabungBaris(ids []idx.ID, ok []domain.BarisBulk) []domain.BarisBulk {
	byID := make(map[idx.ID]domain.BarisBulk, len(ok))
	for _, b := range ok {
		byID[b.ID] = b
	}
	out := make([]domain.BarisBulk, len(ids))
	for i, id := range ids {
		if b, hit := byID[id]; hit {
			out[i] = b
		} else {
			out[i] = domain.BarisBulk{ID: id, Pesan: "Lot tidak ditemukan atau sudah habis"}
		}
	}
	return out
}
