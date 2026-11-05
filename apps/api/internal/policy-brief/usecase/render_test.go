package usecase

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/policy-brief/domain"
	"github.com/garda-inflasi/api/internal/policy-brief/generator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func narasiValid() *domain.Narasi {
	return &domain.Narasi{
		Ringkasan:        "Inflasi tahunan {{INFLASI_PERIODE}} tercatat {{INFLASI_YOY}}.",
		Temuan:           []string{"{{ANDIL_1_KOMODITAS}} memberi andil {{ANDIL_1_NILAI}}."},
		Risiko:           []string{"Harga {{HARGA_1_KOMODITAS}} {{HARGA_1_ARAH}}."},
		RekomendasiPemda: []string{"Siapkan operasi pasar untuk {{HARGA_1_KOMODITAS}}."},
		RekomendasiTPID:  []string{"Bahas dalam rapat TPID."},
		Meta:             domain.MetaNarasi{Penyusun: domain.PenyusunLLM},
	}
}

func TestRender_MenggantiPlaceholderDenganNilaiFakta(t *testing.T) {
	k := BangunKonteks(dataLengkap())

	konten, err := Render(narasiValid(), k, "")

	require.NoError(t, err)
	assert.Equal(t, []string{"Inflasi tahunan Oktober 2026 tercatat 2,84%."}, konten.Butir(domain.BagianRingkasan))
	assert.Equal(t, []string{"Cabai merah keriting memberi andil 0,18 poin persen."}, konten.Butir(domain.BagianTemuan))
	assert.Equal(t, []string{"Harga Cabai merah keriting naik."}, konten.Butir(domain.BagianRisiko))
	assert.Equal(t, domain.LabelDrafAI, konten.Label)
	assert.Equal(t, domain.PenyusunLLM, konten.Penyusun)
}

func TestRender_UrutanBagianTetap(t *testing.T) {
	konten, err := Render(narasiValid(), BangunKonteks(dataLengkap()), "")

	require.NoError(t, err)
	var kode []string
	for _, b := range konten.Bagian {
		kode = append(kode, b.Kode)
		assert.NotEmpty(t, b.Judul)
	}
	assert.Equal(t, []string{"ringkasan", "temuan", "risiko", "rekomendasi_pemda", "rekomendasi_tpid"}, kode)
}

func TestRender_LampiranMemuatFaktaYangDipakaiBerurutDenganSumber(t *testing.T) {
	konten, err := Render(narasiValid(), BangunKonteks(dataLengkap()), "")

	require.NoError(t, err)
	var id []string
	for _, f := range konten.Lampiran {
		id = append(id, f.ID)
		assert.NotEmpty(t, f.Sumber, f.ID)
		assert.NotEmpty(t, f.Nilai, f.ID)
	}
	assert.Equal(t, []string{"INFLASI_PERIODE", "INFLASI_YOY", "ANDIL_1_KOMODITAS", "ANDIL_1_NILAI", "HARGA_1_KOMODITAS", "HARGA_1_ARAH"}, id,
		"hanya fakta yang dipakai, berurut kemunculan pertama, tanpa duplikat")
}

func TestRender_FaktaPenandaTidakMasukLampiran(t *testing.T) {
	n := narasiValid()
	n.Ringkasan = "Brief {{PERIODE}} untuk {{WILAYAH}}."

	konten, err := Render(n, BangunKonteks(dataLengkap()), "")

	require.NoError(t, err)
	assert.Equal(t, []string{"Brief November 2026 untuk Provinsi Aceh."}, konten.Butir(domain.BagianRingkasan))
	for _, f := range konten.Lampiran {
		assert.NotEqual(t, domain.FaktaPeriode, f.ID)
	}
}

func TestRender_MenolakAngkaDiLuarPlaceholder(t *testing.T) {
	kasus := map[string]func(*domain.Narasi){
		"digit di ringkasan":         func(n *domain.Narasi) { n.Ringkasan = "Inflasi {{INFLASI_YOY}} naik 3 persen." },
		"tahun ditulis sendiri":      func(n *domain.Narasi) { n.Ringkasan = "Brief tahun 2026." },
		"angka di temuan":            func(n *domain.Narasi) { n.Temuan = []string{"Harga naik 12%."} },
		"angka di rekomendasi":       func(n *domain.Narasi) { n.RekomendasiPemda = []string{"Siapkan 5 operasi pasar."} },
		"angka desimal berkoma":      func(n *domain.Narasi) { n.Risiko = []string{"Inflasi 2,84%."} },
		"digit non-ASCII":            func(n *domain.Narasi) { n.Risiko = []string{"Inflasi ٣ persen."} },
		"angka menempel placeholder": func(n *domain.Narasi) { n.Temuan = []string{"{{INFLASI_YOY}}5"} },
	}
	for nama, ubah := range kasus {
		t.Run(nama, func(t *testing.T) {
			n := narasiValid()
			ubah(n)

			_, err := Render(n, BangunKonteks(dataLengkap()), "")

			require.ErrorIs(t, err, ErrNarasiTidakValid)
			assert.Contains(t, err.Error(), "angka ditulis di luar placeholder")
		})
	}
}

func TestRender_MenolakPlaceholderTidakDikenalDanRusak(t *testing.T) {
	kasus := map[string]struct {
		teks, pesan string
	}{
		"id tidak ada":         {"Nilai {{TIDAK_ADA}}.", "tidak ada di daftar fakta"},
		"huruf kecil":          {"Nilai {{inflasi_yoy}}.", "tidak ada di daftar fakta"},
		"kurung tidak lengkap": {"Nilai {{INFLASI_YOY.", "placeholder rusak"},
		"kurung tutup saja":    {"Nilai INFLASI_YOY}}.", "placeholder rusak"},
	}
	for nama, k := range kasus {
		t.Run(nama, func(t *testing.T) {
			n := narasiValid()
			n.Temuan = []string{k.teks}

			_, err := Render(n, BangunKonteks(dataLengkap()), "")

			require.ErrorIs(t, err, ErrNarasiTidakValid)
			assert.Contains(t, err.Error(), k.pesan)
		})
	}
}

func TestRender_MenerimaSpasiDalamKurung(t *testing.T) {
	n := narasiValid()
	n.Temuan = []string{"Inflasi {{ INFLASI_YOY }}."}

	konten, err := Render(n, BangunKonteks(dataLengkap()), "")

	require.NoError(t, err)
	assert.Equal(t, []string{"Inflasi 2,84%."}, konten.Butir(domain.BagianTemuan))
}

func TestRender_MenolakBentukYangSalah(t *testing.T) {
	kasus := map[string]func(*domain.Narasi){
		"ringkasan kosong":          func(n *domain.Narasi) { n.Ringkasan = "  " },
		"ringkasan terlalu panjang": func(n *domain.Narasi) { n.Ringkasan = strings.Repeat("a", maksPanjangRingkasan+1) },
		"temuan kosong":             func(n *domain.Narasi) { n.Temuan = nil },
		"risiko kosong":             func(n *domain.Narasi) { n.Risiko = []string{} },
		"rekomendasi pemda kosong":  func(n *domain.Narasi) { n.RekomendasiPemda = nil },
		"rekomendasi tpid kosong":   func(n *domain.Narasi) { n.RekomendasiTPID = nil },
		"butir kosong":              func(n *domain.Narasi) { n.Temuan = []string{"ada", " "} },
		"terlalu banyak butir": func(n *domain.Narasi) {
			n.Temuan = make([]string, maksButirPerBagian+1)
			for i := range n.Temuan {
				n.Temuan[i] = "butir"
			}
		},
		"butir terlalu panjang": func(n *domain.Narasi) { n.Risiko = []string{strings.Repeat("a", maksPanjangButir+1)} },
	}
	for nama, ubah := range kasus {
		t.Run(nama, func(t *testing.T) {
			n := narasiValid()
			ubah(n)

			_, err := Render(n, BangunKonteks(dataLengkap()), "")

			assert.ErrorIs(t, err, ErrNarasiTidakValid)
		})
	}
}

func TestRender_LabelMengikutiPenyusunDanCatatanDisimpan(t *testing.T) {
	n := narasiValid()
	n.Meta.Penyusun = domain.PenyusunTemplat

	konten, err := Render(n, BangunKonteks(dataLengkap()), "LLM gagal; memakai templat.")

	require.NoError(t, err)
	assert.Equal(t, domain.LabelDrafTemplat, konten.Label)
	assert.Equal(t, "LLM gagal; memakai templat.", konten.Catatan)
}

// tokenAngka mengambil token angka (digit, boleh bertitik/berkoma di tengah) dari teks.
var tokenAngka = regexp.MustCompile(`[0-9]+(?:[.,][0-9]+)*`)

// TestRender_AngkaDiNarasiCocokDenganDataSumber memeriksa dari ujung ke ujung bahwa setiap angka
// pada narasi final keluaran templat berasal dari nilai sebuah fakta, bukan ditulis bebas.
func TestRender_AngkaDiNarasiCocokDenganDataSumber(t *testing.T) {
	data := map[string]domain.DataPeriode{"lengkap": dataLengkap(), "kosong": {Periode: bulanUji(2026, time.December)}}
	for nama, d := range data {
		t.Run(nama, func(t *testing.T) {
			k := BangunKonteks(d)
			narasi, err := generator.Templat{}.Susun(t.Context(), k)
			require.NoError(t, err)

			konten, err := Render(narasi, k, "")
			require.NoError(t, err)

			var nilai, teks []string
			for _, f := range k.Fakta {
				nilai = append(nilai, f.Nilai)
			}
			for _, b := range konten.Bagian {
				teks = append(teks, b.Butir...)
			}
			angkaDitemukan := 0
			for _, butir := range teks {
				for _, angka := range tokenAngka.FindAllString(butir, -1) {
					angkaDitemukan++
					assert.True(t, adaDalam(nilai, angka), "angka %q pada %q tidak ada di nilai fakta mana pun", angka, butir)
				}
			}
			if nama == "lengkap" {
				assert.NotZero(t, angkaDitemukan, "narasi lengkap memuat angka dari fakta")
			}
		})
	}
}

func adaDalam(nilai []string, angka string) bool {
	for _, n := range nilai {
		if strings.Contains(n, angka) {
			return true
		}
	}
	return false
}
