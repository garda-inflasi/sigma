package engine

import (
	"math/rand"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/siklus/domain"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var t0 = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

// Aturan bawaan (cermin db/seeds/reference/14_siklus_aturan.sql): TEKANAN prioritas 10 pulih 48 jam, SURPLUS prioritas
// 20 pulih 72 jam.
func aturanAlert(id idx.ID, kode, siklus, jenis, minimal string, jumlah, prioritas, pulih int) domain.AturanSiklus {
	return domain.AturanSiklus{
		ID: id, Kode: kode, Siklus: siklus, Indikator: domain.IndikatorAlertEws, JenisEws: &jenis, SeverityMin: &minimal,
		JumlahMin: &jumlah, Prioritas: prioritas, PulihJam: pulih, Aktif: true,
	}
}

func aturanStok(id idx.ID, kode, siklus string, ambang float64, prioritas, pulih int) domain.AturanSiklus {
	return domain.AturanSiklus{
		ID: id, Kode: kode, Siklus: siklus, Indikator: domain.IndikatorUtilisasiStok, Ambang: &ambang,
		Prioritas: prioritas, PulihJam: pulih, Aktif: true,
	}
}

var (
	lonjakan   = aturanAlert(1, "LONJAKAN", domain.SiklusTekanan, "LONJAKAN_HARGA", "SIAGA", 1, 10, 48)
	kelangkaan = aturanAlert(2, "KELANGKAAN", domain.SiklusTekanan, "KELANGKAAN_STOK", "SIAGA", 1, 10, 48)
	cuaca      = aturanAlert(3, "CUACA", domain.SiklusTekanan, "ANOMALI_CUACA", "SIAGA", 1, 10, 48)
	overSupply = aturanAlert(4, "OVERSUPPLY", domain.SiklusSurplus, "OVER_SUPPLY", "SIAGA", 1, 20, 72)
	utilisasi  = aturanStok(5, "UTILISASI", domain.SiklusSurplus, 90, 20, 72)
)

// ukur menyusun hasil ukur aturan dengan nilai tertentu; Memicu mengikuti aturan (batas inklusif).
func ukur(a domain.AturanSiklus, nilai float64) domain.HasilAturan {
	p := domain.Pengukuran{Terukur: true, Nilai: nilai}
	return domain.HasilAturan{Aturan: a, Pengukuran: p, Memicu: a.Memicu(p)}
}

// semua mengukur kelima aturan bawaan; nilai yang tidak disebut bernilai nol.
func semua(nilai map[string]float64) []domain.HasilAturan {
	var out []domain.HasilAturan
	for _, a := range []domain.AturanSiklus{lonjakan, kelangkaan, cuaca, overSupply, utilisasi} {
		out = append(out, ukur(a, nilai[a.Kode]))
	}
	return out
}

func aktif(siklus string, mulai time.Time, pulihSejak *time.Time) *domain.SiklusEkonomi {
	return &domain.SiklusEkonomi{ID: 7, WilayahKode: "11", Siklus: siklus, MulaiPada: mulai, PulihSejak: pulihSejak}
}

func waktu(t time.Time) *time.Time { return &t }

func kodePemicu(items []domain.PemicuAlasan) []string {
	out := []string{}
	for _, p := range items {
		out = append(out, p.Aturan)
	}
	return out
}

// Tabel keputusan: tiap baris adalah satu keadaan (siklus aktif, penanda pulih, hasil ukur) dan keputusan yang
// diharapkan. Waktu sekarang selalu t0.
func TestPutuskan_Tabel(t *testing.T) {
	jam := func(n float64) time.Duration { return time.Duration(n * float64(time.Hour)) }

	tests := []struct {
		nama       string
		aktif      *domain.SiklusEkonomi
		hasil      []domain.HasilAturan
		siklus     string
		ganti      bool
		pulihSejak *time.Time // dipakai bila !ganti
		pemicu     []string   // kode aturan alasan (hanya bila ganti)
		ditekan    []string
		ringkasan  string // awalan kalimat (bila tidak kosong)
	}{
		// --- masuk dari NORMAL atau tanpa siklus aktif ---
		{nama: "basis data kosong tanpa pemicu membuka NORMAL", aktif: nil, hasil: semua(nil),
			siklus: domain.SiklusNormal, ganti: true, pemicu: []string{}, ditekan: []string{},
			ringkasan: "Siklus awal NORMAL: tidak ada aturan yang memicu."},
		{nama: "basis data kosong dengan pemicu langsung membuka TEKANAN", aktif: nil, hasil: semua(map[string]float64{"LONJAKAN": 2}),
			siklus: domain.SiklusTekanan, ganti: true, pemicu: []string{"LONJAKAN"}, ditekan: []string{},
			ringkasan: "Siklus awal TEKANAN: 2 alert LONJAKAN_HARGA ≥ SIAGA."},
		{nama: "NORMAL tanpa pemicu tetap NORMAL", aktif: aktif(domain.SiklusNormal, t0.Add(-jam(5)), nil), hasil: semua(nil),
			siklus: domain.SiklusNormal, pulihSejak: nil},
		{nama: "NORMAL dengan satu alert lonjakan SIAGA (tepat jumlah minimum) menjadi TEKANAN",
			aktif: aktif(domain.SiklusNormal, t0.Add(-jam(5)), nil), hasil: semua(map[string]float64{"LONJAKAN": 1}),
			siklus: domain.SiklusTekanan, ganti: true, pemicu: []string{"LONJAKAN"}, ditekan: []string{},
			ringkasan: "Berpindah dari NORMAL ke TEKANAN: 1 alert LONJAKAN_HARGA ≥ SIAGA."},
		{nama: "tiap aturan TEKANAN memicu sendiri: kelangkaan stok", aktif: aktif(domain.SiklusNormal, t0.Add(-jam(5)), nil),
			hasil: semua(map[string]float64{"KELANGKAAN": 1}), siklus: domain.SiklusTekanan, ganti: true, pemicu: []string{"KELANGKAAN"}, ditekan: []string{}},
		{nama: "tiap aturan TEKANAN memicu sendiri: anomali cuaca", aktif: aktif(domain.SiklusNormal, t0.Add(-jam(5)), nil),
			hasil: semua(map[string]float64{"CUACA": 3}), siklus: domain.SiklusTekanan, ganti: true, pemicu: []string{"CUACA"}, ditekan: []string{}},
		{nama: "over-supply memicu SURPLUS", aktif: aktif(domain.SiklusNormal, t0.Add(-jam(5)), nil),
			hasil: semua(map[string]float64{"OVERSUPPLY": 1}), siklus: domain.SiklusSurplus, ganti: true, pemicu: []string{"OVERSUPPLY"}, ditekan: []string{}},
		{nama: "utilisasi tepat di ambang (90) memicu SURPLUS", aktif: aktif(domain.SiklusNormal, t0.Add(-jam(5)), nil),
			hasil: semua(map[string]float64{"UTILISASI": 90}), siklus: domain.SiklusSurplus, ganti: true, pemicu: []string{"UTILISASI"}, ditekan: []string{},
			ringkasan: "Berpindah dari NORMAL ke SURPLUS: utilisasi stok 90% ≥ 90%."},
		{nama: "utilisasi sedikit di bawah ambang tidak memicu", aktif: aktif(domain.SiklusNormal, t0.Add(-jam(5)), nil),
			hasil: semua(map[string]float64{"UTILISASI": 89.99}), siklus: domain.SiklusNormal},
		{nama: "utilisasi di atas ambang memicu", aktif: aktif(domain.SiklusNormal, t0.Add(-jam(5)), nil),
			hasil: semua(map[string]float64{"UTILISASI": 90.01}), siklus: domain.SiklusSurplus, ganti: true, pemicu: []string{"UTILISASI"}, ditekan: []string{}},

		// --- prioritas bila TEKANAN dan SURPLUS memicu bersamaan ---
		{nama: "TEKANAN dan SURPLUS bersamaan: TEKANAN menang, SURPLUS tercatat ditekan", aktif: aktif(domain.SiklusNormal, t0.Add(-jam(5)), nil),
			hasil:  semua(map[string]float64{"LONJAKAN": 3, "KELANGKAAN": 1, "UTILISASI": 95}),
			siklus: domain.SiklusTekanan, ganti: true, pemicu: []string{"LONJAKAN", "KELANGKAAN"}, ditekan: []string{"UTILISASI"},
			ringkasan: "Berpindah dari NORMAL ke TEKANAN: 3 alert LONJAKAN_HARGA ≥ SIAGA; 1 alert KELANGKAAN_STOK ≥ SIAGA. " +
				"Aturan SURPLUS ikut memicu (utilisasi stok 95% ≥ 90%) tetapi tidak diutamakan karena prioritasnya lebih rendah."},
		{nama: "prioritas dibalik oleh operator: SURPLUS menang atas TEKANAN",
			aktif: aktif(domain.SiklusNormal, t0.Add(-jam(5)), nil),
			hasil: []domain.HasilAturan{
				ukur(aturanAlert(1, "LONJAKAN", domain.SiklusTekanan, "LONJAKAN_HARGA", "SIAGA", 1, 30, 48), 1),
				ukur(aturanStok(5, "UTILISASI", domain.SiklusSurplus, 90, 5, 72), 95),
			},
			siklus: domain.SiklusSurplus, ganti: true, pemicu: []string{"UTILISASI"}, ditekan: []string{"LONJAKAN"}},
		{nama: "prioritas sama: TEKANAN menang (pemutus seri menurut siklus)",
			aktif: aktif(domain.SiklusNormal, t0.Add(-jam(5)), nil),
			hasil: []domain.HasilAturan{
				ukur(aturanStok(2, "UTILISASI", domain.SiklusSurplus, 90, 10, 72), 95),
				ukur(aturanAlert(9, "LONJAKAN", domain.SiklusTekanan, "LONJAKAN_HARGA", "SIAGA", 1, 10, 48), 1),
			},
			siklus: domain.SiklusTekanan, ganti: true, pemicu: []string{"LONJAKAN"}, ditekan: []string{"UTILISASI"}},

		// --- aturan yang diabaikan ---
		{nama: "aturan nonaktif diabaikan walau nilainya melampaui ambang", aktif: aktif(domain.SiklusNormal, t0.Add(-jam(5)), nil),
			hasil: func() []domain.HasilAturan {
				mati := lonjakan
				mati.Aktif = false
				return []domain.HasilAturan{ukur(mati, 9)}
			}(),
			siklus: domain.SiklusNormal},
		{nama: "pengukuran tak terukur (tanpa data) tidak memicu", aktif: aktif(domain.SiklusNormal, t0.Add(-jam(5)), nil),
			hasil:  []domain.HasilAturan{{Aturan: utilisasi, Pengukuran: domain.Pengukuran{Terukur: false, Nilai: 100}}},
			siklus: domain.SiklusNormal},
		{nama: "aturan bersiklus NORMAL bukan pemicu dan diabaikan", aktif: aktif(domain.SiklusNormal, t0.Add(-jam(5)), nil),
			hasil: func() []domain.HasilAturan {
				salah := lonjakan
				salah.Siklus = domain.SiklusNormal
				return []domain.HasilAturan{ukur(salah, 9)}
			}(),
			siklus: domain.SiklusNormal},

		// --- siklus berjalan yang masih dipicu ---
		{nama: "TEKANAN masih dipicu tetap TEKANAN", aktif: aktif(domain.SiklusTekanan, t0.Add(-jam(30)), nil),
			hasil: semua(map[string]float64{"LONJAKAN": 4}), siklus: domain.SiklusTekanan, pulihSejak: nil,
			ringkasan: "Siklus tetap TEKANAN: 4 alert LONJAKAN_HARGA ≥ SIAGA."},
		{nama: "TEKANAN masih dipicu menghapus penanda pulih yang tersisa", aktif: aktif(domain.SiklusTekanan, t0.Add(-jam(30)), waktu(t0.Add(-jam(20)))),
			hasil: semua(map[string]float64{"CUACA": 1}), siklus: domain.SiklusTekanan, pulihSejak: nil},
		{nama: "TEKANAN dipicu, SURPLUS juga memicu: tetap TEKANAN dan SURPLUS tidak merebut", aktif: aktif(domain.SiklusTekanan, t0.Add(-jam(30)), nil),
			hasil: semua(map[string]float64{"LONJAKAN": 1, "OVERSUPPLY": 5}), siklus: domain.SiklusTekanan, pulihSejak: nil,
			ringkasan: "Siklus tetap TEKANAN: 1 alert LONJAKAN_HARGA ≥ SIAGA. " +
				"Aturan SURPLUS ikut memicu (5 alert OVER_SUPPLY ≥ SIAGA) tetapi tidak diutamakan karena prioritasnya lebih rendah."},
		{nama: "SURPLUS dipicu lalu TEKANAN memicu: TEKANAN merebut saat itu juga (prioritas lebih tinggi)",
			aktif: aktif(domain.SiklusSurplus, t0.Add(-jam(30)), nil), hasil: semua(map[string]float64{"OVERSUPPLY": 2, "LONJAKAN": 1}),
			siklus: domain.SiklusTekanan, ganti: true, pemicu: []string{"LONJAKAN"}, ditekan: []string{"OVERSUPPLY"},
			ringkasan: "Berpindah dari SURPLUS ke TEKANAN: aturan TEKANAN berprioritas lebih tinggi memicu (1 alert LONJAKAN_HARGA ≥ SIAGA). " +
				"Aturan SURPLUS ikut memicu (2 alert OVER_SUPPLY ≥ SIAGA) tetapi tidak diutamakan karena prioritasnya lebih rendah."},

		// --- masa pulih (histeresis) ---
		{nama: "pemicu hilang: masa pulih dimulai sekarang, siklus belum berubah", aktif: aktif(domain.SiklusTekanan, t0.Add(-jam(30)), nil),
			hasil: semua(nil), siklus: domain.SiklusTekanan, pulihSejak: waktu(t0),
			ringkasan: "Siklus tetap TEKANAN: tidak ada aturan TEKANAN yang memicu; masa pulih 48 jam, sisa sekitar 48 jam."},
		{nama: "di tengah masa pulih penanda tidak berubah", aktif: aktif(domain.SiklusTekanan, t0.Add(-jam(80)), waktu(t0.Add(-jam(36)))),
			hasil: semua(nil), siklus: domain.SiklusTekanan, pulihSejak: waktu(t0.Add(-jam(36))),
			ringkasan: "Siklus tetap TEKANAN: tidak ada aturan TEKANAN yang memicu; masa pulih 48 jam, sisa sekitar 12 jam."},
		{nama: "satu nanodetik sebelum masa pulih habis masih TEKANAN", aktif: aktif(domain.SiklusTekanan, t0.Add(-jam(80)), waktu(t0.Add(-jam(48)).Add(time.Nanosecond))),
			hasil: semua(nil), siklus: domain.SiklusTekanan, pulihSejak: waktu(t0.Add(-jam(48)).Add(time.Nanosecond))},
		{nama: "tepat saat masa pulih habis kembali ke NORMAL", aktif: aktif(domain.SiklusTekanan, t0.Add(-jam(80)), waktu(t0.Add(-jam(48)))),
			hasil: semua(nil), siklus: domain.SiklusNormal, ganti: true, pemicu: []string{}, ditekan: []string{},
			ringkasan: "Kembali ke NORMAL: tidak ada aturan TEKANAN yang memicu selama 48 jam."},
		{nama: "lewat masa pulih kembali ke NORMAL", aktif: aktif(domain.SiklusTekanan, t0.Add(-jam(200)), waktu(t0.Add(-jam(100)))),
			hasil: semua(nil), siklus: domain.SiklusNormal, ganti: true, pemicu: []string{}, ditekan: []string{}},
		{nama: "pemicu kembali di tengah masa pulih membatalkan pemulihan", aktif: aktif(domain.SiklusTekanan, t0.Add(-jam(80)), waktu(t0.Add(-jam(40)))),
			hasil: semua(map[string]float64{"LONJAKAN": 1}), siklus: domain.SiklusTekanan, pulihSejak: nil},
		{nama: "SURPLUS memakai masa pulih 72 jam: 60 jam belum cukup", aktif: aktif(domain.SiklusSurplus, t0.Add(-jam(200)), waktu(t0.Add(-jam(60)))),
			hasil: semua(nil), siklus: domain.SiklusSurplus, pulihSejak: waktu(t0.Add(-jam(60)))},
		{nama: "SURPLUS lepas ke NORMAL setelah 72 jam", aktif: aktif(domain.SiklusSurplus, t0.Add(-jam(200)), waktu(t0.Add(-jam(72)))),
			hasil: semua(nil), siklus: domain.SiklusNormal, ganti: true, pemicu: []string{}, ditekan: []string{},
			ringkasan: "Kembali ke NORMAL: tidak ada aturan SURPLUS yang memicu selama 72 jam."},
		{nama: "penanda pulih di masa depan (jam mundur) diperlakukan sebagai sekarang", aktif: aktif(domain.SiklusTekanan, t0.Add(-jam(80)), waktu(t0.Add(jam(10)))),
			hasil: semua(nil), siklus: domain.SiklusTekanan, pulihSejak: waktu(t0)},
		{nama: "NORMAL membuang penanda pulih yang tersisa", aktif: aktif(domain.SiklusNormal, t0.Add(-jam(5)), waktu(t0.Add(-jam(1)))),
			hasil: semua(nil), siklus: domain.SiklusNormal, pulihSejak: nil},

		// --- pergantian siklus selama masa pulih ---
		{nama: "TEKANAN pulih, SURPLUS memicu (prioritas lebih rendah): menunggu masa pulih habis",
			aktif: aktif(domain.SiklusTekanan, t0.Add(-jam(80)), waktu(t0.Add(-jam(10)))), hasil: semua(map[string]float64{"UTILISASI": 95}),
			siklus: domain.SiklusTekanan, pulihSejak: waktu(t0.Add(-jam(10))),
			ringkasan: "Siklus tetap TEKANAN: tidak ada aturan TEKANAN yang memicu; masa pulih 48 jam, sisa sekitar 38 jam. " +
				"Aturan SURPLUS memicu (utilisasi stok 95% ≥ 90%) dan menunggu masa pulih selesai."},
		{nama: "TEKANAN pulih penuh, SURPLUS memicu: langsung ke SURPLUS tanpa lewat NORMAL",
			aktif: aktif(domain.SiklusTekanan, t0.Add(-jam(80)), waktu(t0.Add(-jam(48)))), hasil: semua(map[string]float64{"UTILISASI": 95}),
			siklus: domain.SiklusSurplus, ganti: true, pemicu: []string{"UTILISASI"}, ditekan: []string{},
			ringkasan: "Berpindah dari TEKANAN ke SURPLUS: tidak ada aturan TEKANAN yang memicu selama 48 jam, sedangkan aturan SURPLUS memicu (utilisasi stok 95% ≥ 90%)."},
		{nama: "SURPLUS pulih, TEKANAN memicu (prioritas lebih tinggi): merebut segera tanpa menunggu",
			aktif: aktif(domain.SiklusSurplus, t0.Add(-jam(80)), waktu(t0.Add(-jam(2)))), hasil: semua(map[string]float64{"LONJAKAN": 1}),
			siklus: domain.SiklusTekanan, ganti: true, pemicu: []string{"LONJAKAN"}, ditekan: []string{}},

		// --- siklus aktif tanpa aturan yang membenarkannya ---
		{nama: "siklus aktif tanpa aturan aktif sama sekali dilepas ke NORMAL (masa pulih nol)",
			aktif: aktif(domain.SiklusSurplus, t0.Add(-jam(5)), nil), hasil: []domain.HasilAturan{ukur(lonjakan, 0)},
			siklus: domain.SiklusNormal, ganti: true, pemicu: []string{}, ditekan: []string{}},
		{nama: "siklus aktif tanpa aturan sendiri diganti segera oleh pemicu siklus lain",
			aktif: aktif(domain.SiklusSurplus, t0.Add(-jam(5)), nil), hasil: []domain.HasilAturan{ukur(lonjakan, 2)},
			siklus: domain.SiklusTekanan, ganti: true, pemicu: []string{"LONJAKAN"}, ditekan: []string{}},
		{nama: "masa pulih nol melepas segera begitu pemicu hilang",
			aktif:  aktif(domain.SiklusTekanan, t0.Add(-jam(5)), nil),
			hasil:  []domain.HasilAturan{ukur(aturanAlert(1, "LONJAKAN", domain.SiklusTekanan, "LONJAKAN_HARGA", "SIAGA", 1, 10, 0), 0)},
			siklus: domain.SiklusNormal, ganti: true, pemicu: []string{}, ditekan: []string{}},
	}

	for _, tc := range tests {
		t.Run(tc.nama, func(t *testing.T) {
			got := Putuskan(Kondisi{Sekarang: t0, Aktif: tc.aktif, Hasil: tc.hasil})

			assert.Equal(t, tc.siklus, got.Siklus, "siklus")
			assert.Equal(t, tc.ganti, got.Ganti, "ganti")
			if tc.ganti {
				assert.Nil(t, got.PulihSejak, "siklus baru dimulai tanpa penanda pulih")
				assert.Equal(t, tc.pemicu, kodePemicu(got.Alasan.Pemicu), "pemicu")
				assert.Equal(t, tc.ditekan, kodePemicu(got.Alasan.Ditekan), "ditekan")
			} else if tc.pulihSejak == nil {
				assert.Nil(t, got.PulihSejak, "penanda pulih")
			} else {
				require.NotNil(t, got.PulihSejak, "penanda pulih")
				assert.True(t, tc.pulihSejak.Equal(*got.PulihSejak), "penanda pulih: harapan %s, hasil %s", tc.pulihSejak, got.PulihSejak)
			}
			assert.NotEmpty(t, got.Alasan.Ringkasan, "selalu ada kalimat penjelas")
			if tc.ringkasan != "" {
				assert.Equal(t, tc.ringkasan, got.Alasan.Ringkasan)
			}
			assert.NotNil(t, got.Alasan.Pemicu, "daftar JSON tidak boleh null")
			assert.NotNil(t, got.Alasan.Ditekan, "daftar JSON tidak boleh null")
		})
	}
}

// Masa pulih siklus adalah pulih_jam terbesar di antara aturan aktifnya, bukan yang terkecil atau milik aturan yang
// kebetulan terakhir memicu.
func TestPutuskan_MasaPulihMemakaiPulihJamTerbesarSiklusItu(t *testing.T) {
	cepat := aturanAlert(1, "CEPAT", domain.SiklusTekanan, "LONJAKAN_HARGA", "SIAGA", 1, 10, 24)
	lambat := aturanAlert(2, "LAMBAT", domain.SiklusTekanan, "KELANGKAAN_STOK", "SIAGA", 1, 10, 72)
	hasil := []domain.HasilAturan{ukur(cepat, 0), ukur(lambat, 0)}

	for _, c := range []struct {
		lalu   time.Duration
		siklus string
		ganti  bool
	}{
		{25 * time.Hour, domain.SiklusTekanan, false}, // 24 jam aturan cepat sudah lewat, tetapi 72 jam belum
		{71 * time.Hour, domain.SiklusTekanan, false},
		{72 * time.Hour, domain.SiklusNormal, true},
	} {
		got := Putuskan(Kondisi{Sekarang: t0, Aktif: aktif(domain.SiklusTekanan, t0.Add(-200*time.Hour), waktu(t0.Add(-c.lalu))), Hasil: hasil})
		assert.Equal(t, c.siklus, got.Siklus, "pemicu hilang %s", c.lalu)
		assert.Equal(t, c.ganti, got.Ganti, "pemicu hilang %s", c.lalu)
	}
}

// Hasil tidak boleh bergantung pada urutan aturan masukan, dan masukan tidak boleh diubah.
func TestPutuskan_DeterministikTerhadapUrutanMasukan(t *testing.T) {
	hasil := semua(map[string]float64{"LONJAKAN": 2, "KELANGKAAN": 1, "OVERSUPPLY": 1, "UTILISASI": 99})
	asli := append([]domain.HasilAturan(nil), hasil...)
	acuan := Putuskan(Kondisi{Sekarang: t0, Aktif: aktif(domain.SiklusNormal, t0.Add(-time.Hour), nil), Hasil: hasil})
	require.Equal(t, domain.SiklusTekanan, acuan.Siklus)

	rng := rand.New(rand.NewSource(1))
	for range 50 {
		acak := append([]domain.HasilAturan(nil), hasil...)
		rng.Shuffle(len(acak), func(i, j int) { acak[i], acak[j] = acak[j], acak[i] })
		got := Putuskan(Kondisi{Sekarang: t0, Aktif: aktif(domain.SiklusNormal, t0.Add(-time.Hour), nil), Hasil: acak})
		assert.Equal(t, acuan, got)
	}
	assert.Equal(t, asli, hasil, "masukan tidak boleh diubah")
}

// Skenario berurutan dengan keadaan yang diteruskan antar evaluasi (seperti basis data): satu kali tekanan, pemicu
// hilang sebentar lalu muncul lagi, kemudian hilang untuk selamanya. Siklus hanya berganti dua kali.
func TestPutuskan_TidakBerkedipSelamaPemicuBerkedip(t *testing.T) {
	now := t0
	var siklus *domain.SiklusEkonomi
	var pergantian []string

	langkah := func(lonjakanAda bool, maju time.Duration) {
		now = now.Add(maju)
		nilai := map[string]float64{}
		if lonjakanAda {
			nilai["LONJAKAN"] = 1
		}
		k := Putuskan(Kondisi{Sekarang: now, Aktif: siklus, Hasil: semua(nilai)})
		if k.Ganti {
			siklus = &domain.SiklusEkonomi{ID: 1, Siklus: k.Siklus, MulaiPada: now}
			pergantian = append(pergantian, k.Siklus)
			return
		}
		siklus.PulihSejak = k.PulihSejak
	}

	langkah(false, 0)        // basis data kosong -> NORMAL
	langkah(true, time.Hour) // pemicu muncul -> TEKANAN
	for range 10 {           // pemicu berkedip tiap 6 jam selama 2,5 hari: tidak pernah 48 jam tanpa pemicu
		langkah(false, 6*time.Hour)
		langkah(true, 6*time.Hour)
	}
	langkah(false, time.Hour)    // pemicu hilang: masa pulih mulai
	langkah(false, 47*time.Hour) // 47 jam kemudian masih TEKANAN
	assert.Equal(t, []string{domain.SiklusNormal, domain.SiklusTekanan}, pergantian)
	langkah(false, time.Hour+time.Minute) // lewat 48 jam: NORMAL

	assert.Equal(t, []string{domain.SiklusNormal, domain.SiklusTekanan, domain.SiklusNormal}, pergantian)
}
