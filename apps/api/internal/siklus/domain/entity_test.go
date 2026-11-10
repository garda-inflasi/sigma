package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func alert(jumlah int) AturanSiklus {
	jenis, minimal := "LONJAKAN_HARGA", "SIAGA"
	return AturanSiklus{Indikator: IndikatorAlertEws, JenisEws: &jenis, SeverityMin: &minimal, JumlahMin: &jumlah}
}

func stok(ambang float64) AturanSiklus {
	return AturanSiklus{Indikator: IndikatorUtilisasiStok, Ambang: &ambang}
}

// Batas inklusif: tepat di jumlah minimum atau ambang sudah memicu; satu di bawahnya tidak.
func TestAturanSiklus_MemicuDiBatas(t *testing.T) {
	tests := []struct {
		nama   string
		aturan AturanSiklus
		p      Pengukuran
		memicu bool
	}{
		{"alert: satu di bawah jumlah minimum", alert(2), Pengukuran{Terukur: true, Nilai: 1}, false},
		{"alert: tepat jumlah minimum", alert(2), Pengukuran{Terukur: true, Nilai: 2}, true},
		{"alert: di atas jumlah minimum", alert(2), Pengukuran{Terukur: true, Nilai: 3}, true},
		{"alert: nol alert", alert(1), Pengukuran{Terukur: true, Nilai: 0}, false},
		{"stok: sedikit di bawah ambang", stok(90), Pengukuran{Terukur: true, Nilai: 89.999999}, false},
		{"stok: tepat ambang", stok(90), Pengukuran{Terukur: true, Nilai: 90}, true},
		{"stok: di atas ambang", stok(90), Pengukuran{Terukur: true, Nilai: 100}, true},
		{"tak terukur tidak memicu walau nilainya besar", stok(90), Pengukuran{Terukur: false, Nilai: 100}, false},
		{"alert tak terukur tidak memicu", alert(1), Pengukuran{Terukur: false, Nilai: 5}, false},
		{"alert tanpa jumlah minimum tidak memicu (aturan rusak)", AturanSiklus{Indikator: IndikatorAlertEws}, Pengukuran{Terukur: true, Nilai: 5}, false},
		{"stok tanpa ambang tidak memicu (aturan rusak)", AturanSiklus{Indikator: IndikatorUtilisasiStok}, Pengukuran{Terukur: true, Nilai: 99}, false},
		{"indikator tak dikenal tidak memicu", AturanSiklus{Indikator: "LAIN"}, Pengukuran{Terukur: true, Nilai: 99}, false},
	}
	for _, tc := range tests {
		t.Run(tc.nama, func(t *testing.T) {
			assert.Equal(t, tc.memicu, tc.aturan.Memicu(tc.p))
		})
	}
}

func TestRankSiklus_TekananSebelumSurplusSebelumNormal(t *testing.T) {
	assert.Less(t, RankSiklus(SiklusTekanan), RankSiklus(SiklusSurplus))
	assert.Less(t, RankSiklus(SiklusSurplus), RankSiklus(SiklusNormal))
	assert.Equal(t, RankSiklus(SiklusNormal), RankSiklus("TAK-DIKENAL"))
}

func TestFormatAngka(t *testing.T) {
	for masuk, harap := range map[float64]string{0: "0", 3: "3", 90: "90", 85.6: "85,6", 91.25: "91,2", 99.96: "100", -2.5: "-2,5"} {
		assert.Equal(t, harap, FormatAngka(masuk), "%v", masuk)
	}
}
