package usecase

import (
	"fmt"
	"math"
	"time"

	"github.com/garda-inflasi/api/internal/dashboard/domain"
)

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

// TingkatAceh melaporkan apakah tampilan KPI adalah seluruh Aceh tanpa filter kabupaten maupun komoditas, satu-satunya
// tingkat yang punya target: target_kpi tidak punya kolom wilayah, jadi targetnya target provinsi. Di luar itu nada
// kartu netral ("data"), supaya tidak menyesatkan:
//   - kartu hitungan (waspada) tidak sebanding: hitungan satu kabupaten atau satu komoditas selalu lebih kecil dari
//     hitungan provinsi, jadi kabupaten yang buruk bisa tampak "normal" terhadap target provinsi;
//   - utilisasi cold storage dengan filter komoditas adalah porsi kapasitas yang diisi satu komoditas, yang pasti jauh
//     di bawah target utilisasi seluruh stok (selalu "defisit");
//   - kabupaten tanpa fasilitas cold storage akan tampil "defisit" padahal memang tidak punya fasilitas.
//
// Karena itu persentase (validasi, cs) pun tidak diberi nada di tingkat kabupaten. Bila kelak target per wilayah ada,
// ubah fungsi ini saja.
func TingkatAceh(cakupan domain.Cakupan, f domain.Filter) bool {
	return cakupan.Semua && f.KabupatenKode == "" && f.KomoditasKode == ""
}

// RakitKpi menyusun empat kartu KPI untuk periode f dari titik harian pembilang/penyebut (satu titik per indikator
// dan hari; hari tanpa titik dianggap nol). Selalu menghasilkan keempat kartu, berurutan seperti definisiKpi: basis
// data kosong atau irisan kosong menghasilkan kartu bernilai nol, bukan daftar kosong.
//
//   - nilai adalah nilai pada tanggal acuan f.Sampai; tren memuat nilai harian f.Dari..f.Sampai.
//   - delta selalu dibandingkan terhadap tanggal acuan (bukan terhadap f.Dari), jadi tidak berubah bila periode tren
//     dipendekkan; titik sampai 13 hari sebelum tanggal acuan (dan 6 hari sebelum itu untuk jendela validasi) harus
//     ikut dibaca pemanggil.
//   - persentase dihitung dari pembilang dan penyebut yang sudah dijumlahkan, bukan rata-rata persentase.
//   - nada dari targets; targets kosong berarti nada "data" (lihat TingkatAceh).
func RakitKpi(titik []domain.TitikKpi, f domain.Filter, hariIni time.Time, targets []domain.TargetKpi) []domain.Kpi {
	perIndikator := map[string]map[string]domain.TitikKpi{}
	for _, t := range titik {
		m := perIndikator[t.Indikator]
		if m == nil {
			m = map[string]domain.TitikKpi{}
			perIndikator[t.Indikator] = m
		}
		m[t.Tanggal.Format(domain.TanggalLayout)] = t
	}
	targetOf := map[string]domain.TargetKpi{}
	for _, t := range targets {
		targetOf[t.Indikator] = t
	}

	// Nilai dihitung sejak mulai: awal tren, atau 13 hari sebelum tanggal acuan bila itu lebih awal (untuk delta).
	mulai := f.Dari
	if banding := f.Sampai.AddDate(0, 0, -domain.BandingHari); banding.Before(mulai) {
		mulai = banding
	}
	awalTren := domain.HariAntara(mulai, f.Dari) - 1

	kpis := make([]domain.Kpi, 0, len(definisiKpi))
	for _, def := range definisiKpi {
		semua := nilaiHarian(def.id, perIndikator[def.id], mulai, f.Sampai)
		terakhir := semua[len(semua)-1]

		tone := "data"
		if target, ada := targetOf[def.id]; ada {
			tone = toneDariCapaian(capaian(terakhir, target))
		}
		label := def.label
		if def.id == domain.IndikatorEntri && !f.Sampai.Equal(hariIni) {
			label = "Entri harga " + tanggalID(f.Sampai)
		}
		kpis = append(kpis, domain.Kpi{
			ID: def.id, Label: label, Nilai: bulatkan(terakhir), Format: def.format,
			Delta: bulatkan(delta(def.id, semua)), KeteranganDelta: def.keterangan,
			Tren: bulatkanSemua(semua[awalTren:]), Tone: tone, Href: def.href,
		})
	}
	return kpis
}

// nilaiHarian menghitung nilai kartu tiap hari dari awal sampai akhir (inklusif) dari titik per tanggal. Jendela
// validasi membaca titik sampai 6 hari sebelum awal, jadi hari-hari pertamanya tidak terpotong.
func nilaiHarian(indikator string, perHari map[string]domain.TitikKpi, awal, akhir time.Time) []float64 {
	titik := func(hari time.Time) domain.TitikKpi { return perHari[hari.Format(domain.TanggalLayout)] }

	nilai := make([]float64, domain.HariAntara(awal, akhir))
	for i := range nilai {
		hari := awal.AddDate(0, 0, i)
		switch indikator {
		case domain.IndikatorValidasi:
			var valid, total float64
			for k := range domain.JendelaValidasiHari {
				t := titik(hari.AddDate(0, 0, -k))
				valid += t.Pembilang
				total += t.Penyebut
			}
			nilai[i] = persen(valid, total)
		case domain.IndikatorCS:
			t := titik(hari)
			nilai[i] = persen(t.Pembilang, t.Penyebut)
		default:
			nilai[i] = titik(hari).Pembilang
		}
	}
	return nilai
}

// persen menghitung 100 x pembilang / penyebut; tanpa penyebut (tak ada entri atau kapasitas) hasilnya nol.
func persen(pembilang, penyebut float64) float64 {
	if penyebut <= 0 {
		return 0
	}
	return 100 * pembilang / penyebut
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

// bulatkan membulatkan ke satu desimal. Hasil nol selalu +0: math.Round(-0.4) menghasilkan -0, yang ditulis JSON
// sebagai "-0" padahal artinya sama dengan 0.
func bulatkan(v float64) float64 {
	if r := math.Round(v*10) / 10; r != 0 {
		return r
	}
	return 0
}

func bulatkanSemua(v []float64) []float64 {
	out := make([]float64, len(v))
	for i, x := range v {
		out[i] = bulatkan(x)
	}
	return out
}

var bulanSingkat = [...]string{"Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}

// tanggalID menulis tanggal seperti "5 Nov 2026".
func tanggalID(t time.Time) string {
	return fmt.Sprintf("%d %s %d", t.Day(), bulanSingkat[t.Month()-1], t.Year())
}
