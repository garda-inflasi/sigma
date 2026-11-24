package usecase

import (
	"context"
	"sort"
	"time"

	"github.com/garda-inflasi/api/internal/ml/domain"
)

// Kategori kesegaran satu sumber, dari run crawl terakhir non-BACKFILL.
const (
	KategoriSegar     = "SEGAR"
	KategoriBasi      = "BASI"
	KategoriGagal     = "GAGAL"
	KategoriDilewati  = "DILEWATI"
	KategoriBelumAda  = "BELUM_ADA_RUN_TERJADWAL"
	KategoriTakDikena = "TIDAK_DIKENAL"
)

// zonaWIB adalah zona tanggal bisnis (crawl_date dan tanggal bisnis Mongo).
var zonaWIB = time.FixedZone("WIB", 7*3600)

// EntriKesegaran adalah keadaan satu (sumber, dataset).
type EntriKesegaran struct {
	Sumber, Dataset string
	Kategori        string
	// Status adalah status run terjadwal terakhir (FRESH, STALE_CARRIED_FORWARD, ERROR); nil bila belum ada.
	Status            *string
	Pesan             string
	LastAvailableDate *time.Time
	// UmurHari dihitung dari last_available_date terhadap tanggal WIB hari ini; nil bila tidak diketahui.
	UmurHari  *int
	Terjadwal *domain.CrawlRun
	Backfill  *domain.CrawlRun
	// MenurutSiklus adalah kesegaran yang dipakai siklus training terbaru (source_freshness), nil bila tidak ada.
	MenurutSiklus *domain.Kesegaran
}

// HasilKesegaran adalah kesegaran seluruh sumber.
type HasilKesegaran struct {
	DihitungPada time.Time
	Siklus       *domain.Siklus
	Entri        []EntriKesegaran
	// Sinyal adalah sinyal SUMBER_DATA siklus terbaru, termasuk INFO (adapter sengaja dilewati) yang tidak
	// pernah menjadi alert.
	Sinyal []domain.SinyalEws
	// KeadaanSiklus menjelaskan bila siklus atau sinyal tidak tersedia (crawl_runs tetap dibaca).
	KeadaanSiklus Keadaan
}

// Kategori menentukan kategori run terjadwal terakhir.
func Kategori(run *domain.CrawlRun) string {
	switch {
	case run == nil:
		return KategoriBelumAda
	case domain.Dilewati(run.Pesan):
		return KategoriDilewati
	case run.Status == domain.SumberError:
		return KategoriGagal
	case run.Status == domain.SumberStale:
		return KategoriBasi
	case run.Status == domain.SumberFresh:
		return KategoriSegar
	}
	return KategoriTakDikena
}

// UmurHari menghitung selisih hari kalender WIB antara tanggal bisnis lad (tengah malam UTC dari tanggal WIB) dan now.
// Tanggal di masa depan (dataset prakiraan) berumur 0: datanya mencakup hari ini.
func UmurHari(lad *time.Time, now time.Time) *int {
	if lad == nil {
		return nil
	}
	l := lad.UTC()
	hari := time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, time.UTC)
	n := now.In(zonaWIB)
	kini := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
	umur := max(0, int(kini.Sub(hari).Hours()/24))
	return &umur
}

// Kesegaran menggabungkan run crawl terakhir (dataunseen.crawl_runs; BACKFILL diabaikan untuk penilaian) dengan
// kesegaran dan sinyal SUMBER_DATA siklus training terbaru. Siklus yang belum ada tidak menyembunyikan crawl_runs.
func (s *Service) Kesegaran(ctx context.Context) (*HasilKesegaran, Keadaan) {
	runs, k := s.RunSumber(ctx)
	if !k.Tersedia {
		return nil, k
	}
	now := s.now()
	hasil := &HasilKesegaran{DihitungPada: now, Sinyal: []domain.SinyalEws{}}

	snap, ks := s.Snapshot(ctx, "")
	hasil.KeadaanSiklus = ks
	menurutSiklus := map[[2]string]domain.Kesegaran{}
	if ks.Tersedia {
		hasil.Siklus = &snap.Siklus
		for _, f := range snap.Siklus.SourceFreshness {
			menurutSiklus[[2]string{f.Sumber, f.Dataset}] = f
		}
		for _, sg := range snap.Ews {
			if sg.Jenis == domain.SinyalSumberData && sg.Aktif {
				hasil.Sinyal = append(hasil.Sinyal, sg)
			}
		}
	}

	dilihat := map[[2]string]bool{}
	for _, r := range runs {
		kunci := [2]string{r.Sumber, r.Dataset}
		dilihat[kunci] = true
		e := EntriKesegaran{Sumber: r.Sumber, Dataset: r.Dataset, Kategori: Kategori(r.Terjadwal), Terjadwal: r.Terjadwal, Backfill: r.Backfill}
		if r.Terjadwal != nil {
			st := r.Terjadwal.Status
			e.Status, e.Pesan, e.LastAvailableDate = &st, r.Terjadwal.Pesan, r.Terjadwal.LastAvailableDate
			e.UmurHari = UmurHari(e.LastAvailableDate, now)
		}
		if f, ok := menurutSiklus[kunci]; ok {
			f := f
			e.MenurutSiklus = &f
		}
		hasil.Entri = append(hasil.Entri, e)
	}
	// Sumber yang tercatat siklus tetapi tidak punya run lagi (mis. koleksi crawl_runs dibersihkan) tetap dilaporkan.
	for kunci, f := range menurutSiklus {
		if dilihat[kunci] {
			continue
		}
		f := f
		hasil.Entri = append(hasil.Entri, EntriKesegaran{Sumber: f.Sumber, Dataset: f.Dataset, Kategori: KategoriBelumAda, MenurutSiklus: &f})
	}
	sort.SliceStable(hasil.Entri, func(i, j int) bool {
		if hasil.Entri[i].Sumber != hasil.Entri[j].Sumber {
			return hasil.Entri[i].Sumber < hasil.Entri[j].Sumber
		}
		return hasil.Entri[i].Dataset < hasil.Entri[j].Dataset
	})
	return hasil, tersedia
}
