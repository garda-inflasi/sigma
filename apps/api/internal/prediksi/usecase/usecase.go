package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/garda-inflasi/api/internal/prediksi/domain"
	"gorm.io/gorm"
)

var ErrNotFound = errors.New("proyeksi tidak ditemukan")

const (
	// DefaultKabupaten dan DefaultHorizon dipakai bila klien tidak menyebutkannya:
	// Kota Banda Aceh dan proyeksi 14 hari.
	DefaultKabupaten = "1171"
	DefaultHorizon   = 14

	// riwayatHari adalah panjang riwayat harga aktual yang menyertai proyeksi.
	riwayatHari = 30

	// Ambang perubahan (persen) yang menentukan rekomendasi: di atas ambangNaik
	// harga dinilai menekan, di bawah ambangTurun dinilai surplus.
	ambangNaik  = 5.0
	ambangTurun = -3.0
)

type UseCase struct {
	repo domain.Repository
}

func New(repo domain.Repository) *UseCase {
	return &UseCase{repo: repo}
}

// Ringkasan mengembalikan proyeksi terbaru tiap komoditas yang punya model produksi.
func (uc *UseCase) Ringkasan(ctx context.Context, kabupaten string, horizon int) ([]domain.Ringkasan, error) {
	items, err := uc.repo.ListTerbaru(ctx, kabupaten, horizon)
	if err != nil {
		return nil, err
	}
	res := make([]domain.Ringkasan, len(items))
	for i, p := range items {
		res[i] = domain.Ringkasan{
			Proyeksi:     p,
			PerubahanPct: persenTerhadapDasar(p.Nilai, p.HargaDasar),
			BawahPct:     persenTerhadapDasar(p.Bawah, p.HargaDasar),
			AtasPct:      persenTerhadapDasar(p.Atas, p.HargaDasar),
		}
	}
	return res, nil
}

// Model mengembalikan registri model.
func (uc *UseCase) Model(ctx context.Context) ([]domain.ModelView, error) {
	return uc.repo.ListModel(ctx)
}

// Detail menyusun proyeksi lengkap satu komoditas: jalur proyeksi, riwayat harga
// aktual (bila ada), faktor, narasi, dan rekomendasi. Narasi dan rekomendasi
// dibuat dari templat aturan, bukan dari LLM.
func (uc *UseCase) Detail(ctx context.Context, kode, kabupaten string, horizon int) (*domain.Detail, error) {
	p, err := uc.repo.GetTerbaru(ctx, kode, kabupaten, horizon)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	var faktor []domain.Faktor
	if err := json.Unmarshal([]byte(p.Faktor), &faktor); err != nil {
		return nil, fmt.Errorf("faktor proyeksi %s tidak terbaca: %w", p.ID, err)
	}

	jalur, err := uc.repo.Jalur(ctx, *p, p.HorizonHari)
	if err != nil {
		return nil, err
	}
	dasar := p.TanggalDasar()
	riwayat, err := uc.repo.Riwayat(ctx, kode, kabupaten, dasar.AddDate(0, 0, -riwayatHari), dasar)
	if err != nil {
		return nil, err
	}

	pct := persenTerhadapDasar(p.Nilai, p.HargaDasar)
	return &domain.Detail{
		Proyeksi:     *p,
		PerubahanPct: pct,
		BawahPct:     persenTerhadapDasar(p.Bawah, p.HargaDasar),
		AtasPct:      persenTerhadapDasar(p.Atas, p.HargaDasar),
		Riwayat:      riwayat,
		Jalur:        jalur,
		Faktor:       faktor,
		Narasi:       narasi(*p, pct, faktor),
		Rekomendasi:  rekomendasi(*p, pct),
	}, nil
}

// persenTerhadapDasar menghitung perubahan nilai terhadap harga dasar, satu desimal.
func persenTerhadapDasar(nilai, dasar int) float64 {
	return math.Round((float64(nilai-dasar)/float64(dasar))*1000) / 10
}

func koma(v float64) string {
	return strings.Replace(fmt.Sprintf("%.1f", v), ".", ",", 1)
}

func narasi(p domain.ProyeksiView, pct float64, faktor []domain.Faktor) string {
	arah := "naik"
	if pct < 0 {
		arah = "turun"
	}
	teks := fmt.Sprintf("%s di %s diproyeksi %s %s%% dalam %d hari.", p.KomoditasNama, kabupatenLabel(p.KabupatenNama), arah, koma(math.Abs(pct)), p.HorizonHari)
	if len(faktor) == 0 {
		return teks
	}
	urut := append([]domain.Faktor(nil), faktor...)
	sort.SliceStable(urut, func(i, j int) bool { return math.Abs(urut[i].Kontribusi) > math.Abs(urut[j].Kontribusi) })
	return fmt.Sprintf("%s Faktor paling berpengaruh: %s (%s).", teks, strings.ToLower(urut[0].Nama), strings.ToLower(urut[0].Detail))
}

// kabupatenLabel mengubah nama kabupaten dari master ("KOTA BANDA ACEH") ke tulisan biasa.
func kabupatenLabel(nama string) string {
	kata := strings.Fields(strings.ToLower(nama))
	for i, k := range kata {
		kata[i] = strings.ToUpper(k[:1]) + k[1:]
	}
	return strings.Join(kata, " ")
}

func rekomendasi(p domain.ProyeksiView, pct float64) []string {
	kab := kabupatenLabel(p.KabupatenNama)
	switch {
	case pct > ambangNaik:
		return []string{
			fmt.Sprintf("Siapkan operasi pasar %s di %s", strings.ToLower(p.KomoditasNama), kab),
			"Petakan pasokan alternatif dari sentra terdekat",
			"Ajukan dukungan subsidi ongkos angkut ke Kanwil DJPb",
		}
	case pct < ambangTurun:
		return []string{
			"Arahkan surplus ke cold storage terdekat",
			"Hubungkan agregator dengan pembeli antardaerah",
		}
	default:
		return []string{"Lanjutkan pemantauan rutin", "Verifikasi silang dengan rilis BPS mingguan"}
	}
}
