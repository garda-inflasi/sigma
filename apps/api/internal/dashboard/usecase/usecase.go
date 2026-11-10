package usecase

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/garda-inflasi/api/internal/dashboard/cache"
	"github.com/garda-inflasi/api/internal/dashboard/domain"
	"github.com/garda-inflasi/api/pkg/logger"
)

// ErrValidasi dibungkus dengan penjelasan untuk pemanggil; cocokkan dengan errors.Is.
var ErrValidasi = domain.ErrValidasi

// wib: hari dashboard dihitung menurut waktu Aceh.
var wib = time.FixedZone("WIB", 7*60*60)

// UseCase melayani endpoint dashboard operasional. Semua endpoint agregat menerima domain.Cakupan pemanggil
// (UseCase.Cakupan) dan hanya mengembalikan data dalam cakupan itu; cakupan nol tertutup (hasil kosong). Hasil
// agregasi berat disimpan di cache dengan kunci yang memuat cakupan, filter, dan hari (lihat Kunci).
//
// Untuk menambah endpoint dashboard baru: terima domain.Cakupan di metode usecase dan repository, terjemahkan ke SQL
// lewat kondisiKabupaten (atau kondisiAsalAtauTujuan untuk baris yang punya dua ujung wilayah) di repository (cakupan nol
// menjadi FALSE), kembalikan hasil kosong untuk Cakupan.Tertutup dan irisan kosong tanpa menyentuh basis data, dan
// bungkus pemuatan dengan cache.Ambil memakai kunciCakupan, kunciSaringan, atau kunciFilter. Contoh yang lengkap:
// petugas.go, stok.go, produsen.go, dan logistik.go (ringkasan operasional 08-B).
type UseCase struct {
	repo  domain.Repository
	log   *logger.Logger
	now   func() time.Time
	cache *cache.Cache[Kunci]
}

func New(repo domain.Repository, log *logger.Logger, opts ...Option) *UseCase {
	co := cache.Options{TTL: TTLHasil, MaksEntri: MaksEntriHasil}
	for _, opt := range opts {
		opt(&co)
	}
	return &UseCase{
		repo:  repo,
		log:   log,
		now:   time.Now,
		cache: cache.New[Kunci](co),
	}
}

// Cakupan menurunkan jangkauan data pemanggil dari perannya dan kabupaten tugasnya (02-B), dibaca dari basis data
// pada tiap permintaan, bukan dari klaim token: klaim `kab` hanya untuk tampilan dan bisa basi sampai access token
// habis setelah admin mengubah kabupaten seseorang. Klasifikasi peran adalah harga/domain.CakupanPeran (sumber
// tunggal). Gagal-tutup: galat basis data dikembalikan sebagai galat (tidak pernah dijawab dengan cakupan yang lebih
// luas), dan akun yang tidak ada mendapat cakupan tertutup. Hasilnya tidak di-cache, supaya perubahan kabupaten
// langsung berlaku.
func (uc *UseCase) Cakupan(ctx context.Context, penggunaID string) (domain.Cakupan, error) {
	pengguna, err := uc.repo.PenggunaCakupan(ctx, penggunaID)
	if err != nil {
		if errors.Is(err, domain.ErrPenggunaTidakAda) {
			return domain.Cakupan{}, nil
		}
		uc.log.Error("dashboard: baca cakupan pengguna failed", map[string]interface{}{"error": err.Error()})
		return domain.Cakupan{}, fmt.Errorf("baca cakupan pengguna: %w", err)
	}
	return domain.CakupanPeran(pengguna.Peran, pengguna.KabupatenKode, penggunaID), nil
}

// hariIni adalah tanggal kalender WIB hari ini sebagai nilai DATE (tengah malam UTC).
func (uc *UseCase) hariIni() time.Time {
	t := uc.now().In(wib)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// ── Operasional ───────────────────────────────────────────────────────

// GetKpiOperasional merakit kartu KPI dari fakta harian dan target untuk periode, kabupaten, dan komoditas yang
// diminta, dalam cakupan pemanggil. Parameter yang tidak valid dijawab ErrValidasi. Arti tiap indikator di bawah
// filter dan nada kartu: lihat RakitKpi, TingkatAceh, dan migrasi 000124.
//
// Cakupan tertutup atau filter kabupaten di luar cakupan menghasilkan kartu bernilai nol (seperti basis data
// kosong), tanpa menyentuh basis data.
func (uc *UseCase) GetKpiOperasional(ctx context.Context, cakupan domain.Cakupan, req domain.PermintaanFilter) ([]domain.Kpi, error) {
	hari := uc.hariIni()
	f, err := domain.NormalisasiFilter(req, hari)
	if err != nil {
		return nil, err
	}
	if _, ada := cakupan.Irisi(f.KabupatenKode); !ada {
		return RakitKpi(nil, f, hari, nil), nil
	}
	return cache.Ambil(ctx, uc.cache, kunciFilter("kpi", cakupan, f, hari), func(ctx context.Context) ([]domain.Kpi, error) {
		return uc.muatKpi(ctx, cakupan, f, hari)
	})
}

func (uc *UseCase) muatKpi(ctx context.Context, cakupan domain.Cakupan, f domain.Filter, hari time.Time) ([]domain.Kpi, error) {
	// Fakta dibaca sejak 6 hari sebelum titik pertama yang dihitung (jendela validasi), dan titik pertama itu sendiri
	// paling lambat 13 hari sebelum tanggal acuan (pembanding delta), meski tren yang diminta lebih pendek.
	mulai := f.Dari
	if banding := f.Sampai.AddDate(0, 0, -domain.BandingHari); banding.Before(mulai) {
		mulai = banding
	}
	titik, err := uc.repo.DeretKpi(ctx, domain.DeretKpiQuery{
		Cakupan: cakupan, KabupatenKode: f.KabupatenKode, KomoditasKode: f.KomoditasKode,
		Awal: mulai.AddDate(0, 0, -(domain.JendelaValidasiHari - 1)), Akhir: f.Sampai,
	})
	if err != nil {
		uc.log.Error("dashboard: get kpi operasional failed", map[string]interface{}{"error": err.Error()})
		return nil, fmt.Errorf("get kpi operasional: %w", err)
	}

	// Target hanya berlaku untuk seluruh Aceh tanpa filter (TingkatAceh); selain itu nada kartu netral.
	var targets []domain.TargetKpi
	if TingkatAceh(cakupan, f) {
		targets, err = uc.repo.TargetKpi(ctx, f.Sampai)
		if err != nil {
			uc.log.Error("dashboard: get target kpi failed", map[string]interface{}{"error": err.Error()})
			return nil, fmt.Errorf("get target kpi: %w", err)
		}
	}
	return RakitKpi(titik, f, hari, targets), nil
}

// GetHeatmap menyusun sel peta panas per tanggal acuan (sampai; bawaan hari ini) untuk kabupaten dan komoditas yang
// diminta, dalam cakupan pemanggil. Tanpa dari dan sampai sel dibaca dari mv_heatmap_harga (cepat); dengan salah satunya
// dihitung langsung dari agregat harian untuk tanggal acuan itu, termasuk sel dari harga eksternal. Cakupan tertutup
// atau filter di luar cakupan menghasilkan heatmap kosong tanpa menyentuh basis data.
func (uc *UseCase) GetHeatmap(ctx context.Context, cakupan domain.Cakupan, req domain.PermintaanFilter) (*domain.HeatmapData, error) {
	hari := uc.hariIni()
	f, err := domain.NormalisasiFilter(req, hari)
	if err != nil {
		return nil, err
	}
	if _, ada := cakupan.Irisi(f.KabupatenKode); !ada {
		return &domain.HeatmapData{}, nil
	}
	return cache.Ambil(ctx, uc.cache, kunciFilter("heatmap", cakupan, f, hari), func(ctx context.Context) (*domain.HeatmapData, error) {
		data, err := uc.repo.Heatmap(ctx, domain.HeatmapQuery{
			Cakupan: cakupan, KabupatenKode: f.KabupatenKode, KomoditasKode: f.KomoditasKode,
			Acuan: f.Sampai, Langsung: f.Periode, Dari: f.Dari,
		})
		if err != nil {
			uc.log.Error("dashboard: get heatmap failed", map[string]interface{}{"error": err.Error()})
			return nil, fmt.Errorf("get heatmap: %w", err)
		}
		return data, nil
	})
}

// GetAktivitasLapangan menghitung capaian entri hari ini (waktu Aceh) per kabupaten dalam cakupan pemanggil.
func (uc *UseCase) GetAktivitasLapangan(ctx context.Context, cakupan domain.Cakupan) ([]domain.AktivitasKabupaten, error) {
	if cakupan.Tertutup() {
		return []domain.AktivitasKabupaten{}, nil
	}
	// Jam dibaca sekali: awal hari dan kunci cache harus berasal dari hari yang sama, juga di sekitar tengah malam.
	hari := uc.hariIni()
	awal := time.Date(hari.Year(), hari.Month(), hari.Day(), 0, 0, 0, 0, wib)

	return cache.Ambil(ctx, uc.cache, kunciCakupan("aktivitas-lapangan", cakupan, hari), func(ctx context.Context) ([]domain.AktivitasKabupaten, error) {
		items, err := uc.repo.AktivitasLapangan(ctx, awal, awal.AddDate(0, 0, 1), cakupan)
		if err != nil {
			uc.log.Error("dashboard: get aktivitas lapangan failed", map[string]interface{}{"error": err.Error()})
			return nil, fmt.Errorf("get aktivitas lapangan: %w", err)
		}
		return items, nil
	})
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

// GetPembandingHarga membandingkan harga lapangan dengan harga eksternal (hasil ingestion):
// pasangan terbaru per komoditas dan kabupaten/kota dalam 7 hari terakhir, diurutkan menurut
// selisih absolut terbesar. Pasangan dan Sepakat dihitung atas semua pasangan dalam cakupan pemanggil.
func (uc *UseCase) GetPembandingHarga(ctx context.Context, cakupan domain.Cakupan) (*domain.Pembanding, error) {
	if cakupan.Tertutup() {
		return &domain.Pembanding{}, nil
	}
	hari := uc.hariIni()
	return cache.Ambil(ctx, uc.cache, kunciCakupan("pembanding-harga", cakupan, hari), func(ctx context.Context) (*domain.Pembanding, error) {
		items, err := uc.repo.PembandingHarga(ctx, hari.AddDate(0, 0, -jendelaPembanding), cakupan)
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
	})
}

// GetCuaca mengembalikan cuaca harian terbaru tiap kabupaten/kota dalam cakupan pemanggil (hari ini atau sampai 3 hari
// ke belakang), yang anomali (hujan lebat) dan curah hujan terbesar lebih dulu.
func (uc *UseCase) GetCuaca(ctx context.Context, cakupan domain.Cakupan) ([]domain.CuacaKabupaten, error) {
	if cakupan.Tertutup() {
		return []domain.CuacaKabupaten{}, nil
	}
	hari := uc.hariIni()
	return cache.Ambil(ctx, uc.cache, kunciCakupan("cuaca", cakupan, hari), func(ctx context.Context) ([]domain.CuacaKabupaten, error) {
		items, err := uc.repo.CuacaTerbaru(ctx, hari.AddDate(0, 0, -jendelaCuaca), hari, cakupan)
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
	})
}
