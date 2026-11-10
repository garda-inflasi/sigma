package refresher

import (
	"slices"
	"testing"
)

// Views berurutan menurut ketergantungan: mv_kpi_operasional membaca agregat_harga_harian (kolom waspada), dan
// mv_heatmap_harga serta gold_harga_harian juga turunan agregat, jadi agregat harus disegarkan lebih dulu.
func TestViews_AgregatLebihDuluDariTurunannya(t *testing.T) {
	indeks := func(nama string) int {
		i := slices.Index(Views, nama)
		if i < 0 {
			t.Fatalf("view %s tidak ada di refresher.Views", nama)
		}
		return i
	}
	agregat := indeks("agregat_harga_harian")
	for _, turunan := range []string{"mv_heatmap_harga", "mv_kpi_operasional", "gold_harga_harian"} {
		if indeks(turunan) <= agregat {
			t.Errorf("%s harus disegarkan sesudah agregat_harga_harian", turunan)
		}
	}
}

// Epoch naik tiap penyegaran; tanpa basis data tidak ada yang bisa disegarkan, jadi hanya nilai awal dan kenaikan
// manual yang diperiksa di sini (penyegaran sungguhan diuji di dashboard/integration).
func TestEpoch_MulaiDariNolDanTunggalPerRefresher(t *testing.T) {
	a, b := &Refresher{}, &Refresher{}
	if a.Epoch() != 0 {
		t.Fatalf("epoch awal = %d, mau 0", a.Epoch())
	}
	a.epoch.Add(1)
	if a.Epoch() != 1 || b.Epoch() != 0 {
		t.Fatalf("epoch per refresher: a=%d b=%d", a.Epoch(), b.Epoch())
	}
}
