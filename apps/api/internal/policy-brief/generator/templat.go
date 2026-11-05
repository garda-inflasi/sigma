// Package generator berisi penyusun narasi policy brief: templat deterministik
// (cadangan yang selalu tersedia) dan klien LLM Claude (aktif hanya bila kunci API ada).
// Keduanya hanya menulis narasi dari Konteks, dan menyebut angka lewat placeholder {{ID}}.
package generator

import (
	"context"
	"fmt"
	"strings"

	"github.com/garda-inflasi/api/internal/policy-brief/domain"
)

// NamaTemplat adalah nama Generator templat.
const NamaTemplat = "templat"

// Templat menyusun narasi dengan aturan tetap: data yang sama selalu menghasilkan narasi yang sama.
type Templat struct{}

var _ domain.Generator = Templat{}

func (Templat) Nama() string { return NamaTemplat }

func ph(id string) string { return "{{" + id + "}}" }

func (Templat) Susun(_ context.Context, k domain.Konteks) (*domain.Narasi, error) {
	ada := func(id string) bool { _, ok := k.Cari(id); return ok }
	idx := func(kelompok string, n int, atribut string) string { return ph(domain.IDFakta(kelompok, n, atribut)) }
	adaIdx := func(kelompok string, n int, atribut string) bool { return ada(domain.IDFakta(kelompok, n, atribut)) }
	naik := func(kelompok string, n int) bool {
		f, ok := k.Cari(domain.IDFakta(kelompok, n, "ARAH"))
		return ok && f.Nilai == "naik"
	}

	n := &domain.Narasi{Meta: domain.MetaNarasi{Penyusun: domain.PenyusunTemplat}}

	// Ringkasan.
	ringkasan := []string{fmt.Sprintf("Policy brief %s untuk %s disusun otomatis dari data yang tersedia di sistem.", ph(domain.FaktaPeriode), ph(domain.FaktaWilayah))}
	if ada(domain.FaktaInflasiYoy) {
		kalimat := fmt.Sprintf("Inflasi tahunan %s tercatat %s (bulanan %s)", ph(domain.FaktaInflasiPeriode), ph(domain.FaktaInflasiYoy), ph(domain.FaktaInflasiMtm))
		if ada(domain.FaktaStatusSasaran) {
			kalimat += ", " + ph(domain.FaktaStatusSasaran)
		}
		ringkasan = append(ringkasan, kalimat+".")
	} else {
		ringkasan = append(ringkasan, "Data inflasi resmi belum tersedia untuk periode ini.")
	}
	if adaIdx(domain.KelompokHarga, 1, "KOMODITAS") {
		ringkasan = append(ringkasan, fmt.Sprintf("Perubahan harga terbesar bulan ini terjadi pada %s (%s dibanding bulan sebelumnya).",
			idx(domain.KelompokHarga, 1, "KOMODITAS"), idx(domain.KelompokHarga, 1, "PERUBAHAN")))
	}
	if ada(domain.FaktaEwsTotal) {
		ringkasan = append(ringkasan, fmt.Sprintf("Sistem peringatan dini mencatat %s peringatan pada periode ini.", ph(domain.FaktaEwsTotal)))
	}
	if ada(domain.FaktaCatatanData) {
		ringkasan = append(ringkasan, fmt.Sprintf("Catatan: %s.", ph(domain.FaktaCatatanData)))
	}
	n.Ringkasan = strings.Join(ringkasan, " ")

	// Temuan.
	if ada(domain.FaktaInflasiYoy) {
		kalimat := fmt.Sprintf("Inflasi tahunan %s sebesar %s dan inflasi sejak awal tahun %s", ph(domain.FaktaInflasiPeriode), ph(domain.FaktaInflasiYoy), ph(domain.FaktaInflasiYtd))
		if ada(domain.FaktaTargetBawah) {
			kalimat += fmt.Sprintf(", dengan sasaran inflasi %s sampai %s", ph(domain.FaktaTargetBawah), ph(domain.FaktaTargetAtas))
		}
		n.Temuan = append(n.Temuan, kalimat+".")
	}
	for i := 1; adaIdx(domain.KelompokAndil, i, "KOMODITAS"); i++ {
		n.Temuan = append(n.Temuan, fmt.Sprintf("%s memberi andil %s terhadap inflasi.", idx(domain.KelompokAndil, i, "KOMODITAS"), idx(domain.KelompokAndil, i, "NILAI")))
	}
	for i := 1; adaIdx(domain.KelompokHarga, i, "KOMODITAS"); i++ {
		n.Temuan = append(n.Temuan, fmt.Sprintf("Harga rata-rata %s %s %s dibanding bulan sebelumnya, menjadi %s.",
			idx(domain.KelompokHarga, i, "KOMODITAS"), idx(domain.KelompokHarga, i, "ARAH"), idx(domain.KelompokHarga, i, "PERUBAHAN"), idx(domain.KelompokHarga, i, "RATA")))
	}
	if ada(domain.FaktaEwsTotal) {
		n.Temuan = append(n.Temuan, fmt.Sprintf("Dari %s peringatan dini pada periode ini, %s berstatus kritis, %s siaga, dan %s waspada.",
			ph(domain.FaktaEwsTotal), ph(domain.FaktaEwsKritis), ph(domain.FaktaEwsSiaga), ph(domain.FaktaEwsWaspada)))
	}
	if len(n.Temuan) == 0 {
		n.Temuan = []string{"Data pada periode ini belum cukup untuk menyusun temuan; lengkapi data inflasi dan harga terlebih dahulu."}
	}

	// Risiko.
	if f, ok := k.Cari(domain.FaktaStatusSasaran); ok && !strings.Contains(f.Nilai, "dalam koridor") {
		n.Risiko = append(n.Risiko, fmt.Sprintf("Inflasi tahunan %s, sehingga tekanan harga pangan perlu dikendalikan.", ph(domain.FaktaStatusSasaran)))
	}
	for i := 1; adaIdx(domain.KelompokProyeksi, i, "KOMODITAS"); i++ {
		n.Risiko = append(n.Risiko, fmt.Sprintf("Harga %s diproyeksikan %s %s dalam %s (%s).",
			idx(domain.KelompokProyeksi, i, "KOMODITAS"), idx(domain.KelompokProyeksi, i, "ARAH"), idx(domain.KelompokProyeksi, i, "PERUBAHAN"),
			ph(domain.FaktaProyeksiHoriz), ph(domain.FaktaProyeksiAsal)))
	}
	for i := 1; adaIdx(domain.KelompokEws, i, "LEVEL"); i++ {
		n.Risiko = append(n.Risiko, fmt.Sprintf("Peringatan tingkat %s untuk %s di %s.", idx(domain.KelompokEws, i, "LEVEL"), idx(domain.KelompokEws, i, "KOMODITAS"), idx(domain.KelompokEws, i, "WILAYAH")))
	}
	if len(n.Risiko) == 0 {
		n.Risiko = []string{"Risiko belum dapat dinilai karena data proyeksi dan peringatan dini belum tersedia."}
	}

	// Rekomendasi.
	for i := 1; adaIdx(domain.KelompokProyeksi, i, "KOMODITAS"); i++ {
		if naik(domain.KelompokProyeksi, i) {
			n.RekomendasiPemda = append(n.RekomendasiPemda, fmt.Sprintf("Siapkan operasi pasar dan pasokan alternatif untuk %s.", idx(domain.KelompokProyeksi, i, "KOMODITAS")))
			n.RekomendasiTPID = append(n.RekomendasiTPID, fmt.Sprintf("Bahas langkah pengendalian harga %s dalam rapat koordinasi TPID.", idx(domain.KelompokProyeksi, i, "KOMODITAS")))
		}
	}
	for i := 1; adaIdx(domain.KelompokHarga, i, "KOMODITAS"); i++ {
		if naik(domain.KelompokHarga, i) {
			n.RekomendasiPemda = append(n.RekomendasiPemda, fmt.Sprintf("Perkuat pemantauan harga %s di pasar sentinel.", idx(domain.KelompokHarga, i, "KOMODITAS")))
		}
	}
	n.RekomendasiPemda = append(n.RekomendasiPemda, "Tinjau draf ini bersama data resmi terbaru sebelum menetapkan kebijakan.")
	n.RekomendasiTPID = append(n.RekomendasiTPID, "Verifikasi angka pada lampiran dengan rilis resmi BPS dan Bank Indonesia sebelum draf diajukan.")

	return n, nil
}
