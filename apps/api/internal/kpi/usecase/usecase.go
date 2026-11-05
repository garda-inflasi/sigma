package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/kpi/domain"
	"github.com/garda-inflasi/api/pkg/logger"
)

const (
	// bulanCakupan adalah panjang riwayat yang dimuat untuk daftar KPI.
	bulanCakupan = 12
	// titikTren adalah jumlah periode terakhir yang dikirim sebagai tren.
	titikTren = 6
	// batasRiwayat adalah jumlah periode terbanyak pada riwayat satu KPI.
	batasRiwayat = 36
)

// wib adalah zona waktu batas bulan: hari kerja program ada di Aceh (UTC+7).
var wib = time.FixedZone("WIB", 7*60*60)

// Laporan merangkum satu putaran penghitungan.
type Laporan struct {
	Baru, Direvisi, Tetap int
	// TanpaDasar adalah (KPI, periode) yang datanya belum ada sehingga tidak ditulis.
	TanpaDasar int
}

type UseCase struct {
	repo  domain.Repository
	audit audit.Writer
	log   *logger.Logger
}

func New(repo domain.Repository, auditWriter audit.Writer, log *logger.Logger) *UseCase {
	return &UseCase{repo: repo, audit: auditWriter, log: log}
}

// AwalBulan mengembalikan tanggal 1 bulan t (UTC, sebagai nilai DATE).
func AwalBulan(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// Daftar menyusun semua KPI untuk periode: nilai terbaru yang tidak melewati
// periode, tren beberapa periode terakhir, dan capaian terhadap target.
func (uc *UseCase) Daftar(ctx context.Context, periode time.Time, wilayah *string) ([]domain.Indikator, error) {
	periode = AwalBulan(periode)
	defs, err := uc.repo.Definisi(ctx)
	if err != nil {
		return nil, err
	}
	targets, err := uc.repo.TargetBerlaku(ctx, periode, wilayah)
	if err != nil {
		return nil, err
	}
	nilai, err := uc.repo.NilaiBerlaku(ctx, periode.AddDate(0, -(bulanCakupan-1), 0), periode, wilayah)
	if err != nil {
		return nil, err
	}

	targetByKode := make(map[string]*domain.Target, len(targets))
	for i := range targets {
		targetByKode[targets[i].KpiKode] = &targets[i]
	}
	nilaiByKode := make(map[string][]domain.Nilai, len(defs))
	for _, n := range nilai { // urut naik menurut periode
		nilaiByKode[n.KpiKode] = append(nilaiByKode[n.KpiKode], n)
	}

	res := make([]domain.Indikator, len(defs))
	for i, def := range defs {
		res[i] = rakit(def, targetByKode[def.Kode], nilaiByKode[def.Kode])
	}
	return res, nil
}

// Riwayat mengembalikan satu KPI beserta nilai berlaku tiap periode (paling banyak 36).
func (uc *UseCase) Riwayat(ctx context.Context, kode string, periode time.Time, wilayah *string) (*domain.Indikator, []domain.Nilai, error) {
	def, err := uc.repo.DefinisiByKode(ctx, kode)
	if err != nil {
		return nil, nil, err
	}
	targets, err := uc.repo.TargetBerlaku(ctx, AwalBulan(periode), wilayah)
	if err != nil {
		return nil, nil, err
	}
	riwayat, err := uc.repo.RiwayatKPI(ctx, kode, wilayah, batasRiwayat)
	if err != nil {
		return nil, nil, err
	}
	var target *domain.Target
	for i := range targets {
		if targets[i].KpiKode == kode {
			target = &targets[i]
		}
	}
	ind := rakit(*def, target, riwayat)
	return &ind, riwayat, nil
}

func rakit(def domain.Definisi, target *domain.Target, nilai []domain.Nilai) domain.Indikator {
	ind := domain.Indikator{Definisi: def, Target: target, Otomatis: def.SumberQuery != nil}
	var terbaru *float64
	if n := len(nilai); n > 0 {
		ind.Nilai = &nilai[n-1]
		terbaru = &nilai[n-1].Nilai
		ind.Tren = nilai[max(0, n-titikTren):]
	}
	ind.Status = domain.StatusCapaian(def.ArahBaik, terbaru, target)
	return ind
}

// Hitung menghitung KPI otomatis untuk tiap bulan dari dariPeriode sampai
// sampaiPeriode (inklusif), memakai data yang ada saat ini. Idempoten: nilai yang
// sama tidak ditulis ulang; nilai yang berbeda menjadi versi baru (revisi).
// Bulan di masa depan dilewati. Kejadian yang mengubah data dicatat di audit.
func (uc *UseCase) Hitung(ctx context.Context, dariPeriode, sampaiPeriode, sekarang time.Time) (Laporan, error) {
	var lap Laporan
	defs, err := uc.repo.Definisi(ctx)
	if err != nil {
		return lap, err
	}

	for periode := AwalBulan(dariPeriode); !periode.After(AwalBulan(sampaiPeriode)); periode = periode.AddDate(0, 1, 0) {
		w := jendela(periode, sekarang)
		if w.Mulai.After(sekarang) {
			break
		}
		for _, def := range defs {
			if def.SumberQuery == nil {
				continue
			}
			nilai, err := uc.repo.Hitung(ctx, *def.SumberQuery, w)
			if err != nil {
				return lap, err
			}
			if nilai == nil {
				lap.TanpaDasar++
				continue
			}
			hasil, err := uc.repo.SimpanNilai(ctx, def.Kode, periode, *nilai, sekarang)
			if err != nil {
				return lap, err
			}
			switch hasil {
			case domain.Baru:
				lap.Baru++
			case domain.Direvisi:
				lap.Direvisi++
			default:
				lap.Tetap++
			}
		}
	}

	if lap.Baru+lap.Direvisi > 0 {
		err := uc.audit.Write(ctx, audit.Entry{
			Aksi: "HITUNG_KPI", Entitas: "kpi_nilai",
			EntitasID: fmt.Sprintf("%s..%s", AwalBulan(dariPeriode).Format("2006-01"), AwalBulan(sampaiPeriode).Format("2006-01")),
			Hasil:     audit.HasilBerhasil,
			Metadata:  map[string]any{"baru": lap.Baru, "direvisi": lap.Direvisi, "tetap": lap.Tetap, "tanpa_dasar": lap.TanpaDasar},
		})
		if err != nil {
			uc.log.Warn("kpi: gagal mencatat audit", map[string]interface{}{"error": err.Error()})
		}
	}
	return lap, nil
}

// jendela membentuk rentang bulan periode menurut WIB.
func jendela(periode, sekarang time.Time) domain.Jendela {
	mulai := time.Date(periode.Year(), periode.Month(), 1, 0, 0, 0, 0, wib)
	akhir := mulai.AddDate(0, 1, 0)
	return domain.Jendela{Mulai: mulai, Akhir: akhir, AsOf: minWaktu(akhir, sekarang)}
}

func minWaktu(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}

// CatatEkspor mencatat bahwa laporan KPI periode tertentu diekspor. Ekspor adalah
// keluarnya data, jadi pemanggil menyerahkan berkas hanya bila pencatatan berhasil.
func (uc *UseCase) CatatEkspor(ctx context.Context, aktorID string, periode time.Time, wilayah *string, baris int) error {
	meta := map[string]any{"periode": AwalBulan(periode).Format("2006-01"), "baris": baris}
	if wilayah != nil {
		meta["wilayah"] = *wilayah
	}
	return uc.audit.Write(ctx, audit.Entry{
		AktorID: aktorID, Aksi: "EKSPOR_DATA", Entitas: "kpi_nilai", EntitasID: "laporan",
		Hasil: audit.HasilBerhasil, Metadata: meta,
	})
}
