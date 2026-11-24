package integration

import (
	"net/http"
	"strings"
	"testing"

	"github.com/garda-inflasi/api/internal/dashboard/domain"
	"github.com/garda-inflasi/api/pkg/seed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Row scope (02-B) pada seluruh /v1/dashboard/operasional/*: Koordinator dan Agregator hanya melihat kabupaten tugasnya,
// peran tingkat provinsi ke atas seluruh Aceh. Filter di luar cakupan menghasilkan hasil kosong (bukan 403, bukan seluruh
// Aceh), dan kabupaten dibaca dari basis data pada tiap permintaan, bukan dari klaim token.

const (
	emailAgregator = "agregator.data@gardainflasi.id"
	// Demo: Koordinator dan Agregator bertugas di Kota Banda Aceh (1171).
	kabTugas = kabBanda
)

type aktivitas struct {
	KabupatenKode string `json:"kabupaten_kode"`
	JumlahPetugas int    `json:"jumlah_petugas"`
	Entri         int    `json:"entri"`
	Target        int    `json:"target"`
}

func (s *suite) aktivitasDari(t *testing.T, email string) []aktivitas {
	t.Helper()
	w := s.call(t, http.MethodGet, "/v1/dashboard/operasional/aktivitas-lapangan", email, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	return data[[]aktivitas](t, w)
}

func (s *suite) pembandingDari(t *testing.T, email string) pembanding {
	t.Helper()
	w := s.call(t, http.MethodGet, "/v1/dashboard/operasional/pembanding-harga", email, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	return data[pembanding](t, w)
}

func (s *suite) cuacaDari(t *testing.T, email string) []cuacaRow {
	t.Helper()
	w := s.call(t, http.MethodGet, "/v1/dashboard/operasional/cuaca", email, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	return data[[]cuacaRow](t, w)
}

// pindahKabupaten mengubah kabupaten tugas pengguna di basis data (seperti PATCH /v1/users/:id) tanpa login ulang dan
// memulihkannya saat tes selesai. Token yang sudah terbit tetap membawa klaim `kab` lama: cakupan harus mengikuti
// basis data, bukan klaim itu.
func (s *suite) pindahKabupaten(t *testing.T, email, kabupaten string) {
	t.Helper()
	var asal *string
	require.NoError(t, s.db.Raw("SELECT kabupaten_kode FROM pengguna WHERE email = ?", email).Scan(&asal).Error)
	var nilai any
	if kabupaten != "" {
		nilai = kabupaten
	}
	require.NoError(t, s.db.Exec("UPDATE pengguna SET kabupaten_kode = ? WHERE email = ?", nilai, email).Error)
	t.Cleanup(func() { s.db.Exec("UPDATE pengguna SET kabupaten_kode = ? WHERE email = ?", asal, email) })
}

// kodeKabupaten mengembalikan kode kabupaten pada heatmap, urut seperti respons.
func kodeKabupaten(h petaPanas) []string {
	out := make([]string, len(h.Kabupaten))
	for i, k := range h.Kabupaten {
		out[i] = k.Kode
	}
	return out
}

func TestCakupan_KoordinatorDanAgregatorHanyaKabupatenTugasnyaDiKelimaEndpoint(t *testing.T) {
	s := ingestionSuite(t)
	hari := hariIni()

	// Fixture pembanding: seluruh Aceh punya pasangan di 4 kabupaten, termasuk 1171.
	prov := s.pembandingDari(t, emailBerhak)
	var pasangan1171 int
	for _, it := range prov.Items {
		if it.KabupatenKode == kabTugas {
			pasangan1171++
		}
	}
	require.Positive(t, pasangan1171, "fixture: 1171 punya pasangan harga lapangan dan eksternal")
	provKab := map[string]bool{}
	for _, it := range prov.Items {
		provKab[it.KabupatenKode] = true
	}
	require.Greater(t, len(provKab), 1, "fixture: seluruh Aceh punya pasangan di lebih dari satu kabupaten")

	for _, email := range []string{emailKoordinator, emailAgregator} {
		t.Run(email, func(t *testing.T) {
			t.Run("kpi sama dengan tampilan seluruh Aceh yang difilter ke kabupaten tugas, bernada netral", func(t *testing.T) {
				scoped := s.kpiDari(t, email, "")
				filtered := s.kpiDari(t, emailBerhak, "?kabupaten_kode="+kabTugas)
				require.Len(t, scoped, 4)
				for _, id := range []string{"entri", "validasi", "waspada", "cs"} {
					a, b := kartuByID(t, scoped, id), kartuByID(t, filtered, id)
					assert.Equal(t, b.Tren, a.Tren, id)
					assert.Equal(t, b.Nilai, a.Nilai, id)
					assert.Equal(t, b.Delta, a.Delta, id)
					assert.Equal(t, "data", a.Tone, "target provinsi tidak dipakai untuk satu kabupaten: %s", id)
				}
				samaDenganTabelDasar(t, "koordinator", scoped, s.dariTabelDasar(t, kabTugas, "", hari.AddDate(0, 0, -13), hari))
				// Bukan seluruh Aceh: entri seluruh Aceh jauh lebih banyak daripada satu kabupaten.
				assert.Less(t, kartuByID(t, scoped, "entri").Nilai, kartuByID(t, s.kpiDari(t, emailBerhak, ""), "entri").Nilai)
			})

			t.Run("heatmap hanya sel kabupaten tugas", func(t *testing.T) {
				h := s.heatmapDari(t, email, "")
				require.NotEmpty(t, h.Sel)
				assert.Equal(t, []string{kabTugas}, kodeKabupaten(h))
				for _, c := range h.Sel {
					assert.Equal(t, kabTugas, c.KabupatenKode)
				}
				assert.Equal(t, sel(s.heatmapDari(t, emailBerhak, "?kabupaten_kode="+kabTugas)), sel(h))
			})

			t.Run("aktivitas lapangan satu baris", func(t *testing.T) {
				items := s.aktivitasDari(t, email)
				require.Len(t, items, 1)
				assert.Equal(t, kabTugas, items[0].KabupatenKode)
				assert.Equal(t, 1, items[0].JumlahPetugas)
				assert.Equal(t, 14, items[0].Target)
				for _, a := range s.aktivitasDari(t, emailBerhak) {
					if a.KabupatenKode == kabTugas {
						assert.Equal(t, a, items[0], "angkanya sama dengan baris kabupaten itu pada tampilan seluruh Aceh")
					}
				}
			})

			t.Run("pembanding harga hanya pasangan kabupaten tugas", func(t *testing.T) {
				p := s.pembandingDari(t, email)
				require.NotEmpty(t, p.Items)
				for _, it := range p.Items {
					assert.Equal(t, kabTugas, it.KabupatenKode)
				}
				assert.Equal(t, pasangan1171, p.Pasangan, "Pasangan dan Sepakat dihitung atas pasangan dalam cakupan saja")
				assert.LessOrEqual(t, p.Sepakat, p.Pasangan)
				assert.Less(t, p.Pasangan, prov.Pasangan)
			})

			t.Run("cuaca hanya kabupaten tugas", func(t *testing.T) {
				items := s.cuacaDari(t, email)
				require.Len(t, items, 1)
				assert.Equal(t, kabTugas, items[0].KabupatenKode)
			})
		})
	}
}

func TestCakupan_PeranTingkatProvinsiMelihatSeluruhAceh(t *testing.T) {
	s := ingestionSuite(t)

	// 10 kabupaten berdata lapangan ditambah satu yang hanya berharga eksternal (1110).
	for _, email := range []string{emailBerhak, emailManajerData} {
		t.Run(email, func(t *testing.T) {
			h := s.heatmapDari(t, email, "")
			assert.Len(t, h.Kabupaten, 11, "semua kabupaten yang punya sel")
			assert.Len(t, s.aktivitasDari(t, email), 10)
			assert.Len(t, s.cuacaDari(t, email), 23)
			assert.Greater(t, s.pembandingDari(t, email).Pasangan, 4)
			kpis := s.kpiDari(t, email, "")
			assert.NotEqual(t, "data", kartuByID(t, kpis, "validasi").Tone, "tingkat provinsi memakai target")
		})
	}
}

// Filter kabupaten di luar cakupan menghasilkan hasil kosong, bukan 403 dan bukan data seluruh Aceh.
func TestCakupan_FilterKabupatenDiLuarCakupanKosong(t *testing.T) {
	s := ingestionSuite(t)

	for _, email := range []string{emailKoordinator, emailAgregator} {
		kpis := s.kpiDari(t, email, "?kabupaten_kode="+kabSelatan)
		require.Len(t, kpis, 4)
		for _, k := range kpis {
			assert.Zero(t, k.Nilai, "%s %s: 1108 punya data, tetapi di luar cakupan", email, k.ID)
			assert.Equal(t, seragamNol(14), k.Tren, "%s %s", email, k.ID)
		}
		h := s.heatmapDari(t, email, "?kabupaten_kode="+kabSelatan)
		assert.Empty(t, h.Sel, email)
		assert.Empty(t, h.Kabupaten, email)
		assert.Empty(t, s.heatmapDari(t, email, "?kabupaten_kode="+kabSelatan+"&sampai="+tanggal(hariIni())).Sel, "jalur langsung juga")

		// Filter yang sama dengan kabupaten tugas (dan filter lain di dalamnya) tetap bekerja.
		assert.NotEmpty(t, s.heatmapDari(t, email, "?kabupaten_kode="+kabTugas).Sel, email)
		assert.Len(t, s.heatmapDari(t, email, "?kabupaten_kode="+kabTugas+"&komoditas_kode=CBM-KRT").Sel, 1, email)
	}
}

func seragamNol(n int) []float64 { return make([]float64, n) }

// Kabupaten tugas dibaca dari basis data pada tiap permintaan: setelah admin memindahkan Koordinator, token lama
// (klaim kab lama, masih berlaku sampai 15 menit) langsung tidak membuka kabupaten lamanya, dan hasil yang sudah
// di-cache untuk kabupaten lama tidak tersaji kepadanya.
func TestCakupan_KabupatenDibacaDariBasisDataBukanKlaimToken(t *testing.T) {
	s := ingestionSuite(t)
	s.login(t, emailKoordinator) // token terbit dengan kabupaten 1171

	// Isi cache untuk cakupan lama pada kelima endpoint.
	assert.Equal(t, []string{kabBanda}, kodeKabupaten(s.heatmapDari(t, emailKoordinator, "")))
	require.Equal(t, kabBanda, s.aktivitasDari(t, emailKoordinator)[0].KabupatenKode)
	require.Equal(t, kabBanda, s.cuacaDari(t, emailKoordinator)[0].KabupatenKode)
	entriBanda := kartuByID(t, s.kpiDari(t, emailKoordinator, ""), "entri").Nilai
	require.Equal(t, kabBanda, s.pembandingDari(t, emailKoordinator).Items[0].KabupatenKode)

	t.Run("dipindah ke kabupaten lain tanpa login ulang", func(t *testing.T) {
		s.pindahKabupaten(t, emailKoordinator, kabSelatan)

		assert.Equal(t, []string{kabSelatan}, kodeKabupaten(s.heatmapDari(t, emailKoordinator, "")))
		items := s.aktivitasDari(t, emailKoordinator)
		require.Len(t, items, 1)
		assert.Equal(t, kabSelatan, items[0].KabupatenKode)
		cuaca := s.cuacaDari(t, emailKoordinator)
		require.Len(t, cuaca, 1)
		assert.Equal(t, kabSelatan, cuaca[0].KabupatenKode)
		for _, it := range s.pembandingDari(t, emailKoordinator).Items {
			assert.Equal(t, kabSelatan, it.KabupatenKode)
		}
		kpis := s.kpiDari(t, emailKoordinator, "")
		samaDenganTabelDasar(t, "pindah", kpis, s.dariTabelDasar(t, kabSelatan, "", hariIni().AddDate(0, 0, -13), hariIni()))
		assert.NotEqual(t, entriBanda, kartuByID(t, kpis, "entri").Nilai, "angka kabupaten lama tidak bocor lewat cache")
		// Filter ke kabupaten lamanya kini di luar cakupan.
		assert.Empty(t, s.heatmapDari(t, emailKoordinator, "?kabupaten_kode="+kabBanda).Sel)
	})

	t.Run("tanpa kabupaten tugas: tertutup, bukan seluruh Aceh", func(t *testing.T) {
		s.pindahKabupaten(t, emailKoordinator, "")

		for _, k := range s.kpiDari(t, emailKoordinator, "") {
			assert.Zero(t, k.Nilai, k.ID)
			assert.Equal(t, "data", k.Tone, k.ID)
		}
		assert.Empty(t, s.heatmapDari(t, emailKoordinator, "").Sel)
		assert.Empty(t, s.aktivitasDari(t, emailKoordinator))
		assert.Empty(t, s.cuacaDari(t, emailKoordinator))
		p := s.pembandingDari(t, emailKoordinator)
		assert.Zero(t, p.Pasangan)
		assert.Empty(t, p.Items)
	})

	t.Run("kembali ke kabupaten semula", func(t *testing.T) {
		assert.Equal(t, []string{kabBanda}, kodeKabupaten(s.heatmapDari(t, emailKoordinator, "")))
	})
}

// Satu URL, dua pemanggil bergantian: hasil cache satu cakupan tidak boleh tersaji ke cakupan lain (cacat keamanan).
func TestCakupan_HasilCacheTidakTersajiKeCakupanLain(t *testing.T) {
	s := ingestionSuite(t)

	// Angka yang benar untuk tiap cakupan, dari tabel dasar: entri CBM-KRT hari ini seluruh Aceh dan hanya Banda Aceh.
	hari := hariIni()
	entriProvinsi := s.dariTabelDasar(t, "", "CBM-KRT", hari, hari)["entri"][0]
	entriBanda := s.dariTabelDasar(t, kabBanda, "CBM-KRT", hari, hari)["entri"][0]
	require.Greater(t, entriProvinsi, entriBanda, "fixture: seluruh Aceh punya lebih banyak entri daripada satu kabupaten")
	require.Positive(t, entriBanda)

	// Urutan sengaja berganti-ganti, dan tiap pasangan memanggil URL persis sama.
	for range 3 {
		for _, email := range []string{emailBerhak, emailKoordinator, emailManajerData, emailAgregator} {
			provinsi := email == emailBerhak || email == emailManajerData

			assert.Len(t, s.aktivitasDari(t, email), map[bool]int{true: 10, false: 1}[provinsi], "aktivitas %s", email)
			assert.Len(t, s.cuacaDari(t, email), map[bool]int{true: 23, false: 1}[provinsi], "cuaca %s", email)
			assert.Len(t, s.heatmapDari(t, email, "?komoditas_kode=CBM-KRT").Sel, map[bool]int{true: 11, false: 1}[provinsi], "heatmap %s", email)
			assert.Equal(t, map[bool]float64{true: entriProvinsi, false: entriBanda}[provinsi],
				kartuByID(t, s.kpiDari(t, email, "?komoditas_kode=CBM-KRT"), "entri").Nilai, "kpi %s", email)
			p := s.pembandingDari(t, email)
			if provinsi {
				assert.Greater(t, p.Pasangan, 4, "pembanding %s", email)
			} else {
				assert.Equal(t, 4, p.Pasangan, "pembanding %s", email)
			}
		}
	}
}

// Basis data kosong: Koordinator tidak mendapat galat di kelima endpoint.
func TestCakupan_DatabaseKosongTidakGalat(t *testing.T) {
	s := newSuite(t, "dashboard_cakupan_kosong", false)

	for _, email := range []string{emailKoordinator, emailAgregator, emailBerhak} {
		kpis := s.kpiDari(t, email, "")
		require.Len(t, kpis, 4, email)
		for _, k := range kpis {
			assert.Zero(t, k.Nilai, "%s %s", email, k.ID)
		}
		assert.Empty(t, s.heatmapDari(t, email, "").Sel, email)
		assert.Empty(t, s.cuacaDari(t, email), email)
		assert.Zero(t, s.pembandingDari(t, email).Pasangan, email)
		// Aktivitas menampilkan kabupaten berpetugas walau belum ada entri: capaian nol dari target.
		for _, a := range s.aktivitasDari(t, email) {
			assert.Zero(t, a.Entri, "%s %s", email, a.KabupatenKode)
			assert.Equal(t, 14, a.Target, "%s %s", email, a.KabupatenKode)
		}
	}
	assert.Len(t, s.aktivitasDari(t, emailKoordinator), 1, "hanya kabupaten tugas")
	assert.Len(t, s.aktivitasDari(t, emailBerhak), 10)
}

// Setiap peran yang boleh membuka dashboard operasional punya aturan cakupan yang jelas: provinsi ke atas melihat seluruh
// Aceh, Koordinator dan Agregator satu kabupaten. Peran baru yang lupa diklasifikasikan membuat tes ini gagal.
func TestCakupan_SetiapPeranBerizinPunyaCakupanYangJelas(t *testing.T) {
	s := ingestionSuite(t)

	var perperan = map[string]string{} // peran -> email demo aktif tanpa MFA
	for _, akun := range seed.BuildDemoAkun() {
		if _, ada := perperan[akun.Peran]; !ada && akun.Status != "NONAKTIF" && !akun.MfaAktif {
			perperan[akun.Peran] = akun.Email
		}
	}
	require.Len(t, perperan, 14, "satu akun demo untuk tiap peran")

	var dibuka, ditolak int
	for peran, email := range perperan {
		w := s.call(t, http.MethodGet, "/v1/dashboard/operasional/heatmap", email, nil)
		if w.Code == http.StatusForbidden {
			ditolak++
			continue // tanpa dashboard.operasional (Mahasiswa, admin sistem)
		}
		dibuka++
		require.Equal(t, http.StatusOK, w.Code, peran)
		h := data[petaPanas](t, w)

		var pengguna struct {
			ID            string
			KabupatenKode *string
		}
		require.NoError(t, s.db.Raw("SELECT id::text AS id, kabupaten_kode FROM pengguna WHERE email = ?", email).Scan(&pengguna).Error)
		kab := ""
		if pengguna.KabupatenKode != nil {
			kab = *pengguna.KabupatenKode
		}
		cakupan := domain.CakupanPeran(peran, kab, pengguna.ID)
		switch {
		case cakupan.Semua:
			assert.Len(t, h.Kabupaten, 11, "%s melihat seluruh Aceh", peran)
		case cakupan.KabupatenKode != "":
			assert.Equal(t, []string{cakupan.KabupatenKode}, kodeKabupaten(h), "%s melihat satu kabupaten", peran)
		default:
			assert.Empty(t, h.Sel, "%s tertutup", peran)
		}
		// Peran operasional bertugas di Banda Aceh; peran lain tingkat provinsi.
		if peran == "OPS_KOORDINATOR" || peran == "OPS_AGREGATOR" {
			assert.Equal(t, []string{kabBanda}, kodeKabupaten(h), peran)
		} else {
			assert.True(t, cakupan.Semua, "%s: peran tingkat provinsi memegang seluruh Aceh", peran)
		}
	}
	assert.Equal(t, 13, dibuka, "4 strategis, 6 manajerial, Koordinator, Agregator, dan admin sistem memegang dashboard.operasional")
	assert.Equal(t, 1, ditolak, "hanya Mahasiswa yang tidak memegangnya")
}

// Penjaga: rute baru di /v1/dashboard/operasional/* harus ikut cakupan baris (handler.cakupan, usecase menerima
// domain.Cakupan, repository memakai kondisiKabupaten atau kondisiAsalAtauTujuan, kunci cache memuat cakupan) dan
// didaftarkan di tes ini. Tes gagal begitu ada rute yang belum tercakup, supaya endpoint baru tidak pernah terbuka ke
// seluruh Aceh dengan tidak sengaja.
func TestCakupan_SetiapRuteOperasionalTerdaftarDiTesCakupan(t *testing.T) {
	s := sharedSuite(t)

	var rute []string
	for _, r := range s.router.Routes() {
		if strings.HasPrefix(r.Path, "/v1/dashboard/operasional/") {
			rute = append(rute, r.Method+" "+r.Path)
		}
	}

	assert.ElementsMatch(t, []string{
		"GET /v1/dashboard/operasional/kpi",
		"GET /v1/dashboard/operasional/heatmap",
		"GET /v1/dashboard/operasional/aktivitas-lapangan",
		"GET /v1/dashboard/operasional/pembanding-harga",
		"GET /v1/dashboard/operasional/cuaca",
		// Ringkasan 08-B: cakupannya diuji di petugas_test.go, stok_test.go, produsen_test.go, dan logistik_test.go.
		"GET /v1/dashboard/operasional/aktivitas-petugas",
		"GET /v1/dashboard/operasional/stok",
		"GET /v1/dashboard/operasional/produsen",
		"GET /v1/dashboard/operasional/logistik",
	}, rute, "rute dashboard operasional baru wajib memakai handler.cakupan dan ditambahkan ke cakupan_test.go dan smoke_dashboard_test.go")
}
