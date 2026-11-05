package usecase

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/garda-inflasi/api/internal/dashboard/domain"
	"github.com/garda-inflasi/api/pkg/logger"
)

// wib: hari dashboard dihitung menurut waktu Aceh.
var wib = time.FixedZone("WIB", 7*60*60)

type UseCase struct {
	repo domain.Repository
	log  *logger.Logger
	now  func() time.Time
}

func New(repo domain.Repository, log *logger.Logger) *UseCase {
	return &UseCase{
		repo: repo,
		log:  log,
		now:  time.Now,
	}
}

// ── Operasional ───────────────────────────────────────────────────────

// definisiKpi memuat label dan tujuan tautan tiap kartu, berurutan seperti di dashboard.
var definisiKpi = []struct {
	id, label, format, href, keterangan string
}{
	{domain.IndikatorEntri, "Entri harga hari ini", "angka", "/data/harga", "dari 7 hari sebelumnya"},
	{domain.IndikatorValidasi, "Tingkat validasi", "persen", "/data/harga?status=DIAJUKAN", "poin dari pekan lalu"},
	{domain.IndikatorWaspada, "Komoditas di atas ambang", "angka", "/ews", "komoditas sejak kemarin"},
	{domain.IndikatorCS, "Utilisasi cold storage", "persen", "/cold-storage", "poin dari pekan lalu"},
}

const (
	// Ambang capaian target untuk warna kartu.
	capaianNormal  = 100
	capaianWaspada = 80
	// jendelaDelta adalah selisih hari pembanding delta.
	jendelaDelta = 7
)

// GetKpiOperasional merakit kartu KPI dari deret harian dan target.
func (uc *UseCase) GetKpiOperasional(ctx context.Context) ([]domain.Kpi, error) {
	deret, err := uc.repo.DeretKpi(ctx)
	if err != nil {
		uc.log.Error("dashboard: get kpi operasional failed", map[string]interface{}{"error": err.Error()})
		return nil, fmt.Errorf("get kpi operasional: %w", err)
	}
	targets, err := uc.repo.TargetKpi(ctx, uc.now().In(wib))
	if err != nil {
		uc.log.Error("dashboard: get target kpi failed", map[string]interface{}{"error": err.Error()})
		return nil, fmt.Errorf("get target kpi: %w", err)
	}
	return RakitKpi(deret, targets), nil
}

// RakitKpi menyusun kartu KPI. Indikator tanpa deret dilewati.
func RakitKpi(deret []domain.TitikKpi, targets []domain.TargetKpi) []domain.Kpi {
	perIndikator := map[string][]domain.TitikKpi{}
	for _, t := range deret {
		perIndikator[t.Indikator] = append(perIndikator[t.Indikator], t)
	}
	targetOf := map[string]domain.TargetKpi{}
	for _, t := range targets {
		targetOf[t.Indikator] = t
	}

	var kpis []domain.Kpi
	for _, def := range definisiKpi {
		titik := perIndikator[def.id]
		if len(titik) == 0 {
			continue
		}
		sort.Slice(titik, func(i, j int) bool { return titik[i].Tanggal.Before(titik[j].Tanggal) })
		nilai := make([]float64, len(titik))
		for i, t := range titik {
			nilai[i] = t.Nilai
		}

		tone := "data"
		if target, ada := targetOf[def.id]; ada {
			tone = toneDariCapaian(capaian(nilai[len(nilai)-1], target))
		}
		kpis = append(kpis, domain.Kpi{
			ID: def.id, Label: def.label, Nilai: bulatkan(nilai[len(nilai)-1]), Format: def.format,
			Delta: bulatkan(delta(def.id, nilai)), KeteranganDelta: def.keterangan,
			Tren: bulatkanSemua(nilai), Tone: tone, Href: def.href,
		})
	}
	return kpis
}

// delta menghitung perubahan kartu: entri dalam persen (7 hari terakhir terhadap 7 hari sebelumnya),
// waspada terhadap kemarin, dan indikator persentase dalam poin terhadap sepekan lalu.
func delta(indikator string, nilai []float64) float64 {
	n := len(nilai)
	switch indikator {
	case domain.IndikatorEntri:
		if n < 2*jendelaDelta {
			return 0
		}
		terakhir, sebelumnya := jumlah(nilai[n-jendelaDelta:]), jumlah(nilai[n-2*jendelaDelta:n-jendelaDelta])
		if sebelumnya == 0 {
			return 0
		}
		return (terakhir - sebelumnya) / sebelumnya * 100
	case domain.IndikatorWaspada:
		if n < 2 {
			return 0
		}
		return nilai[n-1] - nilai[n-2]
	default:
		if n <= jendelaDelta {
			return 0
		}
		return nilai[n-1] - nilai[n-1-jendelaDelta]
	}
}

// capaian mengembalikan persen pencapaian target; arah TURUN dibalik agar nilai kecil lebih baik.
func capaian(nilai float64, target domain.TargetKpi) float64 {
	if target.Arah == domain.ArahTurun {
		if nilai <= 0 {
			return capaianNormal
		}
		return target.Target / nilai * 100
	}
	return nilai / target.Target * 100
}

func toneDariCapaian(pct float64) string {
	switch {
	case pct >= capaianNormal:
		return "normal"
	case pct >= capaianWaspada:
		return "waspada"
	default:
		return "defisit"
	}
}

func jumlah(v []float64) (total float64) {
	for _, x := range v {
		total += x
	}
	return total
}

func bulatkan(v float64) float64 { return math.Round(v*10) / 10 }

func bulatkanSemua(v []float64) []float64 {
	out := make([]float64, len(v))
	for i, x := range v {
		out[i] = bulatkan(x)
	}
	return out
}

func (uc *UseCase) GetHeatmap(ctx context.Context) (*domain.HeatmapData, error) {
	data, err := uc.repo.Heatmap(ctx)
	if err != nil {
		uc.log.Error("dashboard: get heatmap failed", map[string]interface{}{"error": err.Error()})
		return nil, fmt.Errorf("get heatmap: %w", err)
	}
	return data, nil
}

// GetAktivitasLapangan menghitung capaian entri hari ini (waktu Aceh) per kabupaten.
func (uc *UseCase) GetAktivitasLapangan(ctx context.Context) ([]domain.AktivitasKabupaten, error) {
	sekarang := uc.now().In(wib)
	awal := time.Date(sekarang.Year(), sekarang.Month(), sekarang.Day(), 0, 0, 0, 0, wib)

	items, err := uc.repo.AktivitasLapangan(ctx, awal, awal.AddDate(0, 0, 1))
	if err != nil {
		uc.log.Error("dashboard: get aktivitas lapangan failed", map[string]interface{}{"error": err.Error()})
		return nil, fmt.Errorf("get aktivitas lapangan: %w", err)
	}
	return items, nil
}

// ── Validasi silang harga dan cuaca ───────────────────────────────────

const (
	// jendelaPembanding: pasangan harga dicari sampai sekian hari ke belakang.
	jendelaPembanding = 7
	// maksPembanding: jumlah pasangan dengan selisih terbesar yang ditampilkan.
	maksPembanding = 20
	// jendelaCuaca: cuaca terbaru dicari sampai sekian hari ke belakang (prakiraan hari esok tidak dipakai).
	jendelaCuaca = 3
)

// hariIni adalah tanggal kalender WIB hari ini sebagai nilai DATE (tengah malam UTC).
func (uc *UseCase) hariIni() time.Time {
	t := uc.now().In(wib)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// GetPembandingHarga membandingkan harga lapangan dengan harga eksternal (hasil ingestion):
// pasangan terbaru per komoditas dan kabupaten/kota dalam 7 hari terakhir, diurutkan menurut
// selisih absolut terbesar. Pasangan dan Sepakat dihitung atas semua pasangan.
func (uc *UseCase) GetPembandingHarga(ctx context.Context) (*domain.Pembanding, error) {
	items, err := uc.repo.PembandingHarga(ctx, uc.hariIni().AddDate(0, 0, -jendelaPembanding))
	if err != nil {
		uc.log.Error("dashboard: get pembanding harga failed", map[string]interface{}{"error": err.Error()})
		return nil, fmt.Errorf("get pembanding harga: %w", err)
	}
	res := &domain.Pembanding{Pasangan: len(items)}
	for _, it := range items {
		if it.Sepakat() {
			res.Sepakat++
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		a, b := math.Abs(items[i].SelisihPct), math.Abs(items[j].SelisihPct)
		if a != b {
			return a > b
		}
		if items[i].KabupatenKode != items[j].KabupatenKode {
			return items[i].KabupatenKode < items[j].KabupatenKode
		}
		return items[i].KomoditasKode < items[j].KomoditasKode
	})
	res.Items = items[:min(len(items), maksPembanding)]
	return res, nil
}

// GetCuaca mengembalikan cuaca harian terbaru tiap kabupaten/kota (hari ini atau sampai 3 hari
// ke belakang), yang anomali (hujan lebat) dan curah hujan terbesar lebih dulu.
func (uc *UseCase) GetCuaca(ctx context.Context) ([]domain.CuacaKabupaten, error) {
	hari := uc.hariIni()
	items, err := uc.repo.CuacaTerbaru(ctx, hari.AddDate(0, 0, -jendelaCuaca), hari)
	if err != nil {
		uc.log.Error("dashboard: get cuaca failed", map[string]interface{}{"error": err.Error()})
		return nil, fmt.Errorf("get cuaca: %w", err)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Anomali != items[j].Anomali {
			return items[i].Anomali
		}
		if items[i].CurahHujanMm != items[j].CurahHujanMm {
			return items[i].CurahHujanMm > items[j].CurahHujanMm
		}
		return items[i].KabupatenKode < items[j].KabupatenKode
	})
	return items, nil
}
