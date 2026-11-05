package usecase

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/garda-inflasi/api/internal/eksekutif/domain"
)

const (
	// bulanSeri adalah panjang seri tren di hero eksekutif.
	bulanSeri = 12
	// jumlahPenyumbang adalah jumlah komoditas penyumbang yang ditampilkan.
	jumlahPenyumbang = 6
)

// Urutan baku agar respons stabil; juga prioritas saat jumlah kartu aktif seri.
var (
	severities = []string{"INFO", "WASPADA", "SIAGA", "KRITIS"}
	siklusAll  = []string{"normal", "tekanan", "surplus"}
	// siklusPrioritas memutus seri siklus dominan: tekanan didahulukan karena paling mendesak.
	siklusPrioritas = []string{"tekanan", "surplus", "normal"}
)

type UseCase struct {
	repo domain.Repository
}

func New(repo domain.Repository) *UseCase {
	return &UseCase{repo: repo}
}

// Ringkasan menyusun layar eksekutif. Ruang fiskal hanya dimuat bila
// lihatFiskal (izin fiskal.lihat); tanpa izin itu bloknya nil.
func (uc *UseCase) Ringkasan(ctx context.Context, lihatFiskal bool) (*domain.Ringkasan, error) {
	res := &domain.Ringkasan{}

	inflasi, err := uc.inflasi(ctx)
	if err != nil {
		return nil, err
	}
	res.Inflasi = inflasi

	neraca, err := uc.neraca(ctx)
	if err != nil {
		return nil, err
	}
	res.Neraca = neraca

	if lihatFiskal {
		if res.Fiskal, err = uc.fiskal(ctx); err != nil {
			return nil, err
		}
	}

	if res.EWS, err = uc.ews(ctx); err != nil {
		return nil, err
	}
	if res.Intervensi, res.SiklusDominan, err = uc.intervensi(ctx); err != nil {
		return nil, err
	}

	res.Demo = (inflasi != nil && inflasi.Sumber == domain.SumberDemo) ||
		(neraca != nil && neraca.Sumber == domain.SumberDemo) ||
		(res.Fiskal != nil && res.Fiskal.Sumber == domain.SumberDemo)
	return res, nil
}

// inflasi memuat bulan terbaru, seri 12 bulan, penyumbang, dan koridor sasaran.
// Tanpa target berlaku, koridor tidak bisa digambar, jadi blok inflasi tidak
// ditampilkan; seeder referensi menjamin target tersedia.
func (uc *UseCase) inflasi(ctx context.Context) (*domain.InflasiRingkas, error) {
	terbaru, err := uc.repo.InflasiTerbaru(ctx, domain.WilayahProvinsi)
	if err != nil || terbaru == nil {
		return nil, err
	}
	target, err := uc.repo.TargetBerlaku(ctx, terbaru.Periode.Year())
	if err != nil || target == nil {
		return nil, err
	}
	demo := terbaru.Sumber == domain.SumberDemo
	seri, err := uc.repo.SeriInflasi(ctx, domain.WilayahProvinsi, terbaru.Periode, bulanSeri, demo)
	if err != nil {
		return nil, err
	}
	penyumbang, err := uc.repo.Penyumbang(ctx, domain.WilayahProvinsi, terbaru.Periode, jumlahPenyumbang, demo)
	if err != nil {
		return nil, err
	}
	// Terbesar ke terkecil: kontribusi menaikkan inflasi di atas, menurunkan di bawah.
	slices.SortStableFunc(penyumbang, func(a, b domain.Penyumbang) int {
		switch {
		case a.Andil > b.Andil:
			return -1
		case a.Andil < b.Andil:
			return 1
		}
		return strings.Compare(a.Kode, b.Kode)
	})

	titik := make([]domain.TitikSeri, len(seri))
	for i, s := range seri {
		titik[i] = domain.TitikSeri{Periode: s.Periode, Yoy: s.InflasiYoy}
	}
	return &domain.InflasiRingkas{
		Periode: terbaru.Periode, Sumber: terbaru.Sumber, DiimporPada: terbaru.DiimporPada,
		IHK: terbaru.IHK, Yoy: terbaru.InflasiYoy, Mtm: terbaru.InflasiMtm, Ytd: terbaru.InflasiYtd,
		Sasaran:    domain.Sasaran{Tahun: target.Tahun, Bawah: target.BatasBawah, Atas: target.BatasAtas},
		Seri:       titik,
		Penyumbang: penyumbang,
	}, nil
}

func (uc *UseCase) neraca(ctx context.Context) (*domain.NeracaRingkas, error) {
	items, err := uc.repo.NeracaTerbaru(ctx, domain.WilayahProvinsi)
	if err != nil || len(items) == 0 {
		return nil, err
	}
	sumber := items[0].Sumber
	for _, item := range items {
		if item.Sumber == domain.SumberDemo {
			sumber = domain.SumberDemo
		}
	}
	return &domain.NeracaRingkas{Periode: items[0].Periode, Sumber: sumber, Items: items}, nil
}

func (uc *UseCase) fiskal(ctx context.Context) (*domain.FiskalRingkas, error) {
	rows, err := uc.repo.RuangFiskalTerbaru(ctx)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	res := &domain.FiskalRingkas{Sumber: rows[0].Sumber, PerTanggal: rows[0].PerTanggal, Instrumen: rows}
	for _, row := range rows {
		res.Pagu += row.Pagu
		res.Realisasi += row.Realisasi
		if row.Sumber == domain.SumberDemo {
			res.Sumber = domain.SumberDemo
		}
	}
	res.Skor, res.Status = domain.SkorFiskal(res.Pagu, res.Realisasi)
	return res, nil
}

// ews melengkapi semua severity dengan 0 agar bentuk respons tetap.
func (uc *UseCase) ews(ctx context.Context) (map[string]int, error) {
	counts, err := uc.repo.HitungEWS(ctx)
	if err != nil {
		return nil, err
	}
	res := make(map[string]int, len(severities))
	for _, s := range severities {
		res[s] = counts[s]
	}
	return res, nil
}

func (uc *UseCase) intervensi(ctx context.Context) (map[string]domain.IntervensiRingkas, string, error) {
	rows, err := uc.repo.HitungIntervensi(ctx)
	if err != nil {
		return nil, "", err
	}
	res := make(map[string]domain.IntervensiRingkas, len(siklusAll))
	for _, s := range siklusAll {
		res[s] = domain.IntervensiRingkas{}
	}
	for _, row := range rows {
		if _, known := res[row.Siklus]; !known {
			return nil, "", fmt.Errorf("siklus intervensi tak dikenal: %q", row.Siklus)
		}
		res[row.Siklus] = domain.IntervensiRingkas{Aktif: row.Aktif, Selesai: row.Selesai}
	}

	dominan := "normal"
	terbanyak := 0
	for _, s := range siklusPrioritas {
		if res[s].Aktif > terbanyak {
			dominan, terbanyak = s, res[s].Aktif
		}
	}
	return res, dominan, nil
}
