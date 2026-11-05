package usecase

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/garda-inflasi/api/internal/policy-brief/domain"
)

const (
	// maksKomoditas membatasi jumlah komoditas per kelompok fakta (andil, harga, proyeksi, alert).
	maksKomoditas = 3
	// batasStabilPct: perubahan harga/proyeksi di bawah ini (persen, mutlak) disebut stabil.
	batasStabilPct = 0.05

	wilayahBrief = "Provinsi Aceh"

	sumberDemo  = "DEMO"
	sumberModel = "MODEL"
)

// BangunKonteks mengubah data mentah menjadi fakta terformat. Murni dan deterministik:
// data yang sama selalu menghasilkan fakta yang sama dengan urutan yang sama.
func BangunKonteks(d domain.DataPeriode) domain.Konteks {
	k := domain.Konteks{Periode: d.Periode}
	tambah := func(id, label, nilai, sumber string) {
		k.Fakta = append(k.Fakta, domain.Fakta{ID: id, Label: label, Nilai: nilai, Sumber: sumber})
	}

	tambah(domain.FaktaPeriode, "Periode brief", domain.LabelPeriode(d.Periode), "")
	tambah(domain.FaktaWilayah, "Wilayah", wilayahBrief, "")

	demo := false
	catatDemo := func(sumber string) {
		demo = demo || strings.Contains(sumber, sumberDemo)
	}

	if in := d.Inflasi; in != nil {
		sumber := "inflasi_bulanan (" + in.Sumber + ")"
		catatDemo(sumber)
		tambah(domain.FaktaInflasiPeriode, "Bulan data inflasi", domain.LabelPeriode(in.Periode), sumber)
		tambah(domain.FaktaInflasiMtm, "Inflasi bulanan (month-to-month)", persen(in.Mtm), sumber)
		tambah(domain.FaktaInflasiYoy, "Inflasi tahunan (year-on-year)", persen(in.Yoy), sumber)
		tambah(domain.FaktaInflasiYtd, "Inflasi sejak awal tahun (year-to-date)", persen(in.Ytd), sumber)
		if t := d.Target; t != nil {
			tambah(domain.FaktaTargetTahun, "Tahun sasaran inflasi", strconv.Itoa(t.Tahun), "target_inflasi")
			tambah(domain.FaktaTargetBawah, "Batas bawah sasaran inflasi", persen(t.Bawah), "target_inflasi")
			tambah(domain.FaktaTargetAtas, "Batas atas sasaran inflasi", persen(t.Atas), "target_inflasi")
			tambah(domain.FaktaStatusSasaran, "Posisi inflasi tahunan terhadap sasaran", posisiSasaran(in.Yoy, *t), "target_inflasi dan inflasi_bulanan")
		}
	}

	for i, a := range top(d.Andil, maksKomoditas) {
		sumber := "andil_inflasi (" + a.Sumber + ")"
		catatDemo(sumber)
		n := i + 1
		tambah(domain.IDFakta(domain.KelompokAndil, n, "KOMODITAS"), fmt.Sprintf("Komoditas penyumbang inflasi nomor %d", n), a.Komoditas, sumber)
		tambah(domain.IDFakta(domain.KelompokAndil, n, "NILAI"), fmt.Sprintf("Andil %s terhadap inflasi", a.Komoditas), poinPersen(a.Andil), sumber)
	}

	const sumberHarga = "entri_harga (rata-rata bulanan harga konsumen tervalidasi)"
	for i, h := range top(urutkanPerubahan(d.Harga, func(h domain.HargaData) float64 { return h.PerubahanPct }), maksKomoditas) {
		n := i + 1
		tambah(domain.IDFakta(domain.KelompokHarga, n, "KOMODITAS"), fmt.Sprintf("Komoditas dengan perubahan harga terbesar nomor %d", n), h.Komoditas, sumberHarga)
		tambah(domain.IDFakta(domain.KelompokHarga, n, "RATA"), fmt.Sprintf("Harga rata-rata %s bulan ini", h.Komoditas), rupiah(h.RataIni), sumberHarga)
		tambah(domain.IDFakta(domain.KelompokHarga, n, "PERUBAHAN"), fmt.Sprintf("Perubahan harga %s dibanding bulan sebelumnya", h.Komoditas), persenBertanda(h.PerubahanPct), sumberHarga)
		tambah(domain.IDFakta(domain.KelompokHarga, n, "ARAH"), fmt.Sprintf("Arah harga %s", h.Komoditas), arah(h.PerubahanPct), sumberHarga)
	}

	if e := d.Ews; e.Total > 0 {
		const sumber = "ews_alert"
		tambah(domain.FaktaEwsTotal, "Jumlah peringatan dini pada periode", strconv.Itoa(e.Total), sumber)
		tambah(domain.FaktaEwsKritis, "Peringatan tingkat kritis", strconv.Itoa(e.Kritis), sumber)
		tambah(domain.FaktaEwsSiaga, "Peringatan tingkat siaga", strconv.Itoa(e.Siaga), sumber)
		tambah(domain.FaktaEwsWaspada, "Peringatan tingkat waspada", strconv.Itoa(e.Waspada), sumber)
		for i, a := range top(e.Teratas, maksKomoditas) {
			n := i + 1
			tambah(domain.IDFakta(domain.KelompokEws, n, "KOMODITAS"), fmt.Sprintf("Komoditas pada peringatan terparah nomor %d", n), judulKomoditas(a.Komoditas), sumber)
			tambah(domain.IDFakta(domain.KelompokEws, n, "WILAYAH"), fmt.Sprintf("Wilayah peringatan terparah nomor %d", n), kapital(a.Wilayah), sumber)
			tambah(domain.IDFakta(domain.KelompokEws, n, "LEVEL"), fmt.Sprintf("Tingkat peringatan terparah nomor %d", n), strings.ToLower(a.Level), sumber)
		}
	}

	proyeksi := top(urutkanPerubahan(d.Proyeksi, func(p domain.ProyeksiData) float64 { return p.PerubahanPct }), maksKomoditas)
	semuaModel := true
	for _, p := range proyeksi {
		semuaModel = semuaModel && p.Sumber == sumberModel
	}
	for i, p := range proyeksi {
		sumber := "prediksi_harga (model produksi, " + p.Sumber + ")"
		catatDemo(sumber)
		n := i + 1
		if n == 1 {
			tambah(domain.FaktaProyeksiHoriz, "Horizon proyeksi", fmt.Sprintf("%d hari", p.HorizonHari), sumber)
			tambah(domain.FaktaProyeksiAsal, "Asal proyeksi", asalProyeksi(semuaModel), sumber)
		}
		tambah(domain.IDFakta(domain.KelompokProyeksi, n, "KOMODITAS"), fmt.Sprintf("Komoditas dengan proyeksi perubahan terbesar nomor %d", n), p.Komoditas, sumber)
		tambah(domain.IDFakta(domain.KelompokProyeksi, n, "PERUBAHAN"), fmt.Sprintf("Proyeksi perubahan harga %s", p.Komoditas), persenBertanda(p.PerubahanPct), sumber)
		tambah(domain.IDFakta(domain.KelompokProyeksi, n, "ARAH"), fmt.Sprintf("Arah proyeksi harga %s", p.Komoditas), arah(p.PerubahanPct), sumber)
	}

	if demo {
		tambah(domain.FaktaCatatanData, "Catatan keandalan data", "sebagian angka berasal dari data demo dan bukan data resmi", "sistem")
	}
	return k
}

func top[T any](items []T, n int) []T {
	if len(items) > n {
		return items[:n]
	}
	return items
}

// urutkanPerubahan mengembalikan salinan items berurut |perubahan| terbesar dulu (stabil menurut urutan masuk).
func urutkanPerubahan[T any](items []T, nilai func(T) float64) []T {
	out := append([]T(nil), items...)
	sort.SliceStable(out, func(i, j int) bool { return math.Abs(nilai(out[i])) > math.Abs(nilai(out[j])) })
	return out
}

func posisiSasaran(yoy float64, t domain.TargetData) string {
	switch {
	case yoy > t.Atas:
		return "berada di atas batas atas sasaran inflasi"
	case yoy < t.Bawah:
		return "berada di bawah batas bawah sasaran inflasi"
	default:
		return "berada dalam koridor sasaran inflasi"
	}
}

// asalProyeksi menyebut asal proyeksi; satu saja proyeksi demo membuat seluruhnya disebut demo.
func asalProyeksi(semuaModel bool) string {
	if semuaModel {
		return "model hasil pelatihan"
	}
	return "data demo, bukan keluaran model sungguhan"
}

func arah(pct float64) string {
	switch {
	case math.Abs(pct) < batasStabilPct:
		return "stabil"
	case pct > 0:
		return "naik"
	default:
		return "turun"
	}
}

// koma menulis v dengan desimal koma dan sebanyak-banyaknya `desimal` angka di belakangnya (nol di ujung dibuang).
func koma(v float64, desimal int) string {
	teks := strconv.FormatFloat(math.Round(v*math.Pow10(desimal))/math.Pow10(desimal), 'f', -1, 64)
	if teks == "-0" {
		teks = "0"
	}
	return strings.Replace(teks, ".", ",", 1)
}

func persen(v float64) string { return koma(v, 2) + "%" }

func poinPersen(v float64) string { return koma(v, 2) + " poin persen" }

func persenBertanda(v float64) string {
	teks := koma(v, 1)
	if v > 0 && teks != "0" {
		teks = "+" + teks
	}
	return teks + "%"
}

// rupiah menulis n sebagai "Rp50.000" (pemisah ribuan titik).
func rupiah(n int) string {
	digit := strconv.Itoa(n)
	var b strings.Builder
	for i, r := range digit {
		if i > 0 && (len(digit)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	return "Rp" + b.String()
}

// kapital mengubah nama dari master ("KOTA BANDA ACEH") menjadi tulisan biasa ("Kota Banda Aceh").
func kapital(nama string) string {
	kata := strings.Fields(strings.ToLower(nama))
	for i, k := range kata {
		kata[i] = strings.ToUpper(k[:1]) + k[1:]
	}
	return strings.Join(kata, " ")
}

// judulKomoditas menjaga nama komoditas apa adanya; alert tanpa komoditas (nil/kosong) disebut umum.
func judulKomoditas(nama string) string {
	if nama == "" {
		return "komoditas umum"
	}
	return nama
}
