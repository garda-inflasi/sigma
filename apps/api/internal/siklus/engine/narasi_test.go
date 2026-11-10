package engine

import (
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/siklus/domain"
	"github.com/stretchr/testify/assert"
)

func TestDeskripsi_BentukKalimatPerIndikator(t *testing.T) {
	duaAlert := aturanAlert(1, "X", domain.SiklusTekanan, "LONJAKAN_HARGA", "SIAGA", 2, 10, 48)

	assert.Equal(t, "3 alert LONJAKAN_HARGA ≥ SIAGA", deskripsi(ukur(lonjakan, 3)))
	assert.Equal(t, "2 alert LONJAKAN_HARGA ≥ SIAGA (minimal 2)", deskripsi(ukur(duaAlert, 2)), "jumlah minimum di atas satu disebut")
	assert.Equal(t, "utilisasi stok 91,2% ≥ 90%", deskripsi(ukur(utilisasi, 91.2)))
	assert.Equal(t, "utilisasi stok 90% ≥ 90%", deskripsi(ukur(utilisasi, 90)), "angka bulat tanpa koma")
}

func TestDaftar_DanDaftarSiklus(t *testing.T) {
	hasil := []domain.HasilAturan{ukur(lonjakan, 3), ukur(kelangkaan, 1), ukur(utilisasi, 95)}

	assert.Equal(t, "3 alert LONJAKAN_HARGA ≥ SIAGA; 1 alert KELANGKAAN_STOK ≥ SIAGA; utilisasi stok 95% ≥ 90%", daftar(hasil))
	assert.Equal(t, "TEKANAN dan SURPLUS", daftarSiklus(hasil), "siklus berbeda disebut sekali")
	assert.Equal(t, "TEKANAN", daftarSiklus(hasil[:2]))
}

func TestRingkasanGanti_SemuaSebab(t *testing.T) {
	pemicu := []domain.HasilAturan{ukur(lonjakan, 3)}
	ditekan := []domain.HasilAturan{ukur(utilisasi, 95)}
	kalimatTekan := " Aturan SURPLUS ikut memicu (utilisasi stok 95% ≥ 90%) tetapi tidak diutamakan karena prioritasnya lebih rendah."

	assert.Equal(t, "Siklus awal NORMAL: tidak ada aturan yang memicu.", ringkasanGanti(domain.SiklusNormal, sebabAwal, "", nil, nil, 0))
	assert.Equal(t, "Siklus awal TEKANAN: 3 alert LONJAKAN_HARGA ≥ SIAGA.", ringkasanGanti(domain.SiklusTekanan, sebabAwal, "", pemicu, nil, 0))
	assert.Equal(t, "Berpindah dari NORMAL ke TEKANAN: 3 alert LONJAKAN_HARGA ≥ SIAGA."+kalimatTekan,
		ringkasanGanti(domain.SiklusTekanan, sebabMasuk, domain.SiklusNormal, pemicu, ditekan, 0))
	assert.Equal(t, "Berpindah dari SURPLUS ke TEKANAN: aturan TEKANAN berprioritas lebih tinggi memicu (3 alert LONJAKAN_HARGA ≥ SIAGA).",
		ringkasanGanti(domain.SiklusTekanan, sebabGeser, domain.SiklusSurplus, pemicu, nil, 0))
	assert.Equal(t, "Kembali ke NORMAL: tidak ada aturan TEKANAN yang memicu selama 48 jam.",
		ringkasanGanti(domain.SiklusNormal, sebabPulih, domain.SiklusTekanan, nil, nil, 48))
	assert.Equal(t, "Berpindah dari TEKANAN ke SURPLUS: tidak ada aturan TEKANAN yang memicu selama 48 jam, sedangkan aturan SURPLUS memicu (utilisasi stok 95% ≥ 90%).",
		ringkasanGanti(domain.SiklusSurplus, sebabPulih, domain.SiklusTekanan, ditekan, nil, 48))
}

func TestRingkasanTetapPulih_MembulatkanSisaKeAtas(t *testing.T) {
	assert.Contains(t, ringkasanTetapPulih(domain.SiklusTekanan, 48, 90*time.Minute, nil), "sisa sekitar 2 jam")
	assert.Contains(t, ringkasanTetapPulih(domain.SiklusTekanan, 48, time.Nanosecond, nil), "sisa sekitar 1 jam")
	assert.Contains(t, ringkasanTetapPulih(domain.SiklusTekanan, 48, -time.Hour, nil), "sisa sekitar 0 jam", "tidak pernah negatif")
}

func TestPemicuAlasan_AmbangDanSatuanMenurutIndikator(t *testing.T) {
	h := ukur(lonjakan, 4)
	h.Pengukuran.Rujukan = []string{"EWS-1000", "EWS-1001"}
	got := pemicuAlasan(h)
	assert.Equal(t, domain.PemicuAlasan{
		Aturan: "LONJAKAN", Siklus: domain.SiklusTekanan, Indikator: domain.IndikatorAlertEws, Nilai: 4, Ambang: 1, Satuan: "alert",
		Prioritas: 10, Rujukan: []string{"EWS-1000", "EWS-1001"},
	}, got)

	h = ukur(utilisasi, 92.5)
	h.Pengukuran.Rincian = "925 dari 1000 ton"
	got = pemicuAlasan(h)
	assert.Equal(t, domain.PemicuAlasan{
		Aturan: "UTILISASI", Siklus: domain.SiklusSurplus, Indikator: domain.IndikatorUtilisasiStok, Nilai: 92.5, Ambang: 90, Satuan: "%",
		Prioritas: 20, Rincian: "925 dari 1000 ton",
	}, got)
}
