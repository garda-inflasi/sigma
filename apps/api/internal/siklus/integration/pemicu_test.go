package integration

import (
	"context"
	"testing"
	"time"

	ewsjobs "github.com/garda-inflasi/api/internal/ews/jobs"
	"github.com/garda-inflasi/api/internal/siklus/domain"
	siklusjobs "github.com/garda-inflasi/api/internal/siklus/jobs"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Rule engine EWS sungguhan menulis alert dari harga yang melonjak; Pemicu (pengamat EWS) membuat siklus dievaluasi ulang
// beberapa saat kemudian, tanpa menunggu jadwal, dan pergantiannya diaudit sebagai dipicu alert.
func TestPemicu_AlertRuleEngineEwsMenggantiSiklusTanpaMenungguJadwal(t *testing.T) {
	s := newSuite(t, "siklus_it_pemicu")
	log := logger.NewDefault()
	ews := ewsjobs.Shared(s.db, log)
	pemicu := siklusjobs.NewPemicu(s.engine, domain.WilayahBawaan, 50*time.Millisecond, log)
	ews.Engine.AddObserver(pemicu)
	ctx, cancel := context.WithCancel(t.Context())
	selesai := make(chan struct{})
	go func() { pemicu.Run(ctx); close(selesai) }()
	t.Cleanup(func() {
		cancel()
		<-selesai
	})

	s.evaluasi(t) // NORMAL awal
	require.Equal(t, "NORMAL", s.aktif(t).Siklus)

	// Cabai Banda Aceh: Rp50.000 tiga hari lalu, Rp60.000 kini (+20%) = KRITIS pada aturan lonjakan harga.
	now := time.Now() // rule engine EWS mengukur menurut jam aplikasi, jadi data uji memakai jam yang sama
	s.hargaKonsumen(t, "H1", "CBM-KRT", "PSR-1171-01", 50000, now.Add(-72*time.Hour))
	s.hargaKonsumen(t, "H2", "CBM-KRT", "PSR-1171-01", 60000, now.Add(-2*time.Hour))
	ringkasan, err := ews.Engine.Evaluasi(t.Context())
	require.NoError(t, err)
	require.Equal(t, 1, ringkasan.Dibuat, "rule engine EWS membuat satu alert lonjakan harga")

	require.Eventually(t, func() bool {
		var siklus string
		_ = s.db.Raw(`SELECT siklus::text FROM siklus_ekonomi WHERE wilayah_kode = '11' AND akhir_pada IS NULL`).Scan(&siklus).Error
		return siklus == "TEKANAN"
	}, 15*time.Second, 50*time.Millisecond, "siklus harus berganti ke TEKANAN setelah alert EWS ditulis, tanpa evaluasi terjadwal")

	assert.Equal(t, []string{"NORMAL", "TEKANAN"}, s.urutanSiklus(t))
	ganti := s.audit(t, "GANTI_SIKLUS")
	require.Len(t, ganti, 2)
	require.NotNil(t, ganti[1].Metadata)
	assert.Contains(t, *ganti[1].Metadata, `"pemicu": "alert"`)
	assert.Nil(t, ganti[1].AktorID, "dilakukan sistem")
	assert.Contains(t, *s.aktif(t).Alasan, "SKL-TKN-LONJAKAN-HARGA")
}

// hargaKonsumen menyisipkan satu entri harga konsumen tervalidasi yang dicatat pada waktu dicatat.
func (s *suite) hargaKonsumen(t *testing.T, kode, komoditas, pasar string, harga int, dicatat time.Time) {
	t.Helper()
	require.NoError(t, s.db.Exec(`INSERT INTO entri_harga (kode, komoditas_kode, pasar_id, tipe, harga, satuan, status, dicatat_pada)
		VALUES (?, ?, ?, 'KONSUMEN', ?, 'kg', 'DIVALIDASI', ?)`, kode, komoditas, pasar, harga, dicatat).Error)
}
