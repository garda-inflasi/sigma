package usecase

import (
	"context"
	"sync"
	"time"

	"github.com/garda-inflasi/api/internal/dashboard/domain"
)

// fakeRepo adalah repository palsu yang mencatat setiap panggilan (jumlah dan argumen terakhir). Jawabannya diatur
// lewat field fungsi; yang tidak diatur mengembalikan hasil kosong.
type fakeRepo struct {
	mu sync.Mutex
	n  map[string]int

	pengguna    domain.PenggunaCakupan
	penggunaErr error

	deretKpi   func(domain.DeretKpiQuery) ([]domain.TitikKpi, error)
	targets    []domain.TargetKpi
	targetErr  error
	heatmap    func(domain.HeatmapQuery) (*domain.HeatmapData, error)
	pembanding func(sejak time.Time, c domain.Cakupan) ([]domain.PembandingHarga, error)
	cuaca      func(sejak, sampai time.Time, c domain.Cakupan) ([]domain.CuacaKabupaten, error)
	aktivitas  func(awal, akhir time.Time, c domain.Cakupan) ([]domain.AktivitasKabupaten, error)
	petugas    func(domain.AktivitasPetugasQuery) ([]domain.AktivitasPetugas, error)
	stok       func(domain.StokQuery) (domain.FaktaStok, error)
	produsen   func(domain.ProdusenQuery) ([]domain.ProdusenFakta, error)
	logistik   func(domain.LogistikQuery) (domain.LogistikFakta, error)

	// Argumen panggilan terakhir.
	qKpi      domain.DeretKpiQuery
	qTarget   time.Time
	qHeatmap  domain.HeatmapQuery
	qSejak    time.Time
	qSampai   time.Time
	qAwal     time.Time
	qAkhir    time.Time
	qCakupan  domain.Cakupan
	qPengguna string
	qPetugas  domain.AktivitasPetugasQuery
	qStok     domain.StokQuery
	qProdusen domain.ProdusenQuery
	qLogistik domain.LogistikQuery
}

func (f *fakeRepo) catat(nama string) {
	if f.n == nil {
		f.n = map[string]int{}
	}
	f.n[nama]++
}

// panggilan mengembalikan berapa kali metode nama dipanggil.
func (f *fakeRepo) panggilan(nama string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.n[nama]
}

// totalPanggilanData adalah jumlah panggilan ke semua metode data (selain PenggunaCakupan).
func (f *fakeRepo) totalPanggilanData() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	total := 0
	for nama, n := range f.n {
		if nama != "PenggunaCakupan" {
			total += n
		}
	}
	return total
}

func (f *fakeRepo) PenggunaCakupan(_ context.Context, penggunaID string) (domain.PenggunaCakupan, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.catat("PenggunaCakupan")
	f.qPengguna = penggunaID
	return f.pengguna, f.penggunaErr
}

func (f *fakeRepo) DeretKpi(_ context.Context, q domain.DeretKpiQuery) ([]domain.TitikKpi, error) {
	f.mu.Lock()
	f.catat("DeretKpi")
	f.qKpi = q
	fn := f.deretKpi
	f.mu.Unlock()
	if fn == nil {
		return nil, nil
	}
	return fn(q)
}

func (f *fakeRepo) TargetKpi(_ context.Context, tanggal time.Time) ([]domain.TargetKpi, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.catat("TargetKpi")
	f.qTarget = tanggal
	return f.targets, f.targetErr
}

func (f *fakeRepo) Heatmap(_ context.Context, q domain.HeatmapQuery) (*domain.HeatmapData, error) {
	f.mu.Lock()
	f.catat("Heatmap")
	f.qHeatmap = q
	fn := f.heatmap
	f.mu.Unlock()
	if fn == nil {
		return &domain.HeatmapData{}, nil
	}
	return fn(q)
}

func (f *fakeRepo) PembandingHarga(_ context.Context, sejak time.Time, c domain.Cakupan) ([]domain.PembandingHarga, error) {
	f.mu.Lock()
	f.catat("PembandingHarga")
	f.qSejak, f.qCakupan = sejak, c
	fn := f.pembanding
	f.mu.Unlock()
	if fn == nil {
		return nil, nil
	}
	return fn(sejak, c)
}

func (f *fakeRepo) CuacaTerbaru(_ context.Context, sejak, sampai time.Time, c domain.Cakupan) ([]domain.CuacaKabupaten, error) {
	f.mu.Lock()
	f.catat("CuacaTerbaru")
	f.qSejak, f.qSampai, f.qCakupan = sejak, sampai, c
	fn := f.cuaca
	f.mu.Unlock()
	if fn == nil {
		return nil, nil
	}
	return fn(sejak, sampai, c)
}

func (f *fakeRepo) AktivitasLapangan(_ context.Context, awal, akhir time.Time, c domain.Cakupan) ([]domain.AktivitasKabupaten, error) {
	f.mu.Lock()
	f.catat("AktivitasLapangan")
	f.qAwal, f.qAkhir, f.qCakupan = awal, akhir, c
	fn := f.aktivitas
	f.mu.Unlock()
	if fn == nil {
		return nil, nil
	}
	return fn(awal, akhir, c)
}

func (f *fakeRepo) AktivitasPetugas(_ context.Context, q domain.AktivitasPetugasQuery) ([]domain.AktivitasPetugas, error) {
	f.mu.Lock()
	f.catat("AktivitasPetugas")
	f.qPetugas = q
	fn := f.petugas
	f.mu.Unlock()
	if fn == nil {
		return nil, nil
	}
	return fn(q)
}

func (f *fakeRepo) FaktaStok(_ context.Context, q domain.StokQuery) (domain.FaktaStok, error) {
	f.mu.Lock()
	f.catat("FaktaStok")
	f.qStok = q
	fn := f.stok
	f.mu.Unlock()
	if fn == nil {
		return domain.FaktaStok{}, nil
	}
	return fn(q)
}

func (f *fakeRepo) Produsen(_ context.Context, q domain.ProdusenQuery) ([]domain.ProdusenFakta, error) {
	f.mu.Lock()
	f.catat("Produsen")
	f.qProdusen = q
	fn := f.produsen
	f.mu.Unlock()
	if fn == nil {
		return nil, nil
	}
	return fn(q)
}

func (f *fakeRepo) Logistik(_ context.Context, q domain.LogistikQuery) (domain.LogistikFakta, error) {
	f.mu.Lock()
	f.catat("Logistik")
	f.qLogistik = q
	fn := f.logistik
	f.mu.Unlock()
	if fn == nil {
		return domain.LogistikFakta{}, nil
	}
	return fn(q)
}
