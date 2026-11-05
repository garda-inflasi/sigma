package generator_test

import (
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/policy-brief/domain"
	"github.com/garda-inflasi/api/internal/policy-brief/generator"
	"github.com/garda-inflasi/api/internal/policy-brief/usecase"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func awalBulan(m time.Month) time.Time { return time.Date(2026, m, 1, 0, 0, 0, 0, time.UTC) }

// dataPenuh mengisi setiap bagian sampai batas jumlah komoditas, supaya templat teruji pada jumlah butir terbanyak.
func dataPenuh() domain.DataPeriode {
	return domain.DataPeriode{
		Periode: awalBulan(time.November),
		Inflasi: &domain.InflasiData{Periode: awalBulan(time.October), Mtm: 0.41, Yoy: 3.9, Ytd: 2.13, Sumber: "DEMO"},
		Target:  &domain.TargetData{Tahun: 2026, Bawah: 1.5, Atas: 3.5},
		Andil: []domain.AndilData{
			{Komoditas: "Cabai merah keriting", Andil: 0.18, Sumber: "DEMO"},
			{Komoditas: "Bawang merah", Andil: 0.06, Sumber: "DEMO"},
			{Komoditas: "Ikan tongkol", Andil: 0.04, Sumber: "DEMO"},
		},
		Harga: []domain.HargaData{
			{Komoditas: "Cabai merah keriting", RataIni: 56000, RataLalu: 50000, PerubahanPct: 12},
			{Komoditas: "Bawang merah", RataIni: 42000, RataLalu: 40000, PerubahanPct: 5},
			{Komoditas: "Ikan tongkol", RataIni: 38000, RataLalu: 36000, PerubahanPct: 5.5},
		},
		Ews: domain.EwsData{Total: 17, Kritis: 2, Siaga: 5, Waspada: 10, Teratas: []domain.AlertData{
			{Komoditas: "Cabai merah keriting", Wilayah: "KABUPATEN ACEH TENGAH", Level: "KRITIS"},
			{Komoditas: "Bawang merah", Wilayah: "KOTA BANDA ACEH", Level: "SIAGA"},
			{Komoditas: "Ikan tongkol", Wilayah: "KABUPATEN ACEH UTARA", Level: "SIAGA"},
		}},
		Proyeksi: []domain.ProyeksiData{
			{Komoditas: "Cabai merah keriting", HorizonHari: 14, PerubahanPct: 14.2, Sumber: "DEMO"},
			{Komoditas: "Bawang merah", HorizonHari: 14, PerubahanPct: 7.1, Sumber: "DEMO"},
			{Komoditas: "Ikan tongkol", HorizonHari: 14, PerubahanPct: 3.1, Sumber: "DEMO"},
		},
	}
}

func TestTemplat_NamaDanMetadata(t *testing.T) {
	var g domain.Generator = generator.Templat{}

	n, err := g.Susun(t.Context(), usecase.BangunKonteks(dataPenuh()))

	require.NoError(t, err)
	assert.Equal(t, "templat", g.Nama())
	assert.Equal(t, domain.MetaNarasi{Penyusun: domain.PenyusunTemplat}, n.Meta, "templat tidak punya model atau prompt")
}

func TestTemplat_DeterministikUntukDataSama(t *testing.T) {
	k := usecase.BangunKonteks(dataPenuh())

	a, errA := generator.Templat{}.Susun(t.Context(), k)
	b, errB := generator.Templat{}.Susun(t.Context(), k)

	require.NoError(t, errA)
	require.NoError(t, errB)
	assert.Equal(t, a, b)
}

// Hasil templat harus lolos aturan anti-halusinasi (tanpa angka bebas, placeholder valid, jumlah butir
// dalam batas) pada setiap kombinasi data, termasuk data kosong dan data terpenuh.
func TestTemplat_LolosRenderUntukBerbagaiData(t *testing.T) {
	kosong := domain.DataPeriode{Periode: awalBulan(time.December)}
	tanpaInflasi := dataPenuh()
	tanpaInflasi.Inflasi, tanpaInflasi.Target, tanpaInflasi.Andil = nil, nil, nil
	tanpaTarget := dataPenuh()
	tanpaTarget.Target = nil
	diAtasSasaran := dataPenuh()
	diAtasSasaran.Inflasi.Yoy = 4.2
	hanyaHarga := domain.DataPeriode{Periode: awalBulan(time.October), Harga: dataPenuh().Harga[:1]}
	stabil := dataPenuh()
	stabil.Harga[0].PerubahanPct, stabil.Proyeksi[0].PerubahanPct = 0, 0

	kasus := map[string]domain.DataPeriode{
		"penuh": dataPenuh(), "kosong": kosong, "tanpa inflasi": tanpaInflasi, "tanpa target": tanpaTarget,
		"di atas sasaran": diAtasSasaran, "hanya harga": hanyaHarga, "perubahan nol": stabil,
	}
	for nama, data := range kasus {
		t.Run(nama, func(t *testing.T) {
			k := usecase.BangunKonteks(data)
			n, err := generator.Templat{}.Susun(t.Context(), k)
			require.NoError(t, err)

			konten, err := usecase.Render(n, k, "")

			require.NoError(t, err)
			assert.Equal(t, domain.LabelDrafTemplat, konten.Label)
			for _, kode := range []string{domain.BagianRingkasan, domain.BagianTemuan, domain.BagianRisiko, domain.BagianRekomendasiPemda, domain.BagianRekomendasiTPID} {
				assert.NotEmpty(t, konten.Butir(kode), kode)
			}
		})
	}
}

func TestTemplat_IsiMengikutiDataYangAda(t *testing.T) {
	render := func(d domain.DataPeriode) *domain.Konten {
		k := usecase.BangunKonteks(d)
		n, err := generator.Templat{}.Susun(t.Context(), k)
		require.NoError(t, err)
		konten, err := usecase.Render(n, k, "")
		require.NoError(t, err)
		return konten
	}

	t.Run("data kosong dinyatakan belum tersedia, bukan dikarang", func(t *testing.T) {
		konten := render(domain.DataPeriode{Periode: awalBulan(time.December)})

		assert.Contains(t, konten.Butir(domain.BagianRingkasan)[0], "Data inflasi resmi belum tersedia")
		assert.Contains(t, konten.Butir(domain.BagianTemuan)[0], "belum cukup")
		assert.Contains(t, konten.Butir(domain.BagianRisiko)[0], "belum dapat dinilai")
		assert.Empty(t, konten.Lampiran, "tanpa data tidak ada sitasi")
	})

	t.Run("data penuh memuat angka dari fakta dan sitasi sumbernya", func(t *testing.T) {
		konten := render(dataPenuh())

		ringkasan := konten.Butir(domain.BagianRingkasan)[0]
		assert.Contains(t, ringkasan, "Inflasi tahunan Oktober 2026 tercatat 3,9%")
		assert.Contains(t, ringkasan, "berada di atas batas atas sasaran inflasi", "perbandingan dengan sasaran dihitung server, bukan oleh LLM")
		assert.Contains(t, ringkasan, "sebagian angka berasal dari data demo")
		assert.Contains(t, konten.Butir(domain.BagianRisiko)[0], "tekanan harga pangan")
		assert.Contains(t, konten.Butir(domain.BagianRekomendasiPemda)[0], "operasi pasar")

		sumber := map[string]string{}
		for _, f := range konten.Lampiran {
			sumber[f.ID] = f.Sumber
		}
		assert.Equal(t, "inflasi_bulanan (DEMO)", sumber["INFLASI_YOY"])
		assert.Equal(t, "ews_alert", sumber["EWS_TOTAL"])
	})

	t.Run("jumlah butir terbanyak masih dalam batas", func(t *testing.T) {
		konten := render(dataPenuh())
		for _, b := range konten.Bagian {
			assert.LessOrEqual(t, len(b.Butir), 8, b.Kode)
		}
	})
}
