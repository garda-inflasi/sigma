package integration

import (
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/ingestion/connector/pihps"
	"github.com/garda-inflasi/api/internal/ingestion/domain"
	"github.com/garda-inflasi/api/internal/ingestion/scheduler"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SRC-BI dijadwalkan "0 10 * * *" (WIB) di katalog.
var pukul10WIB = time.Date(2026, 11, 5, 10, 0, 0, 0, time.FixedZone("WIB", 7*3600))

func newPenjadwal(s *setup, sekarang *time.Time) *scheduler.Penjadwal {
	p := scheduler.New(s.repo, s.runner, logger.NewDefault())
	p.Sekarang = func() time.Time { return *sekarang }
	return p
}

func TestPenjadwal_MenjalankanSumberPadaMenitJadwalSekaliSaja(t *testing.T) {
	s := newSetup(t, "ingest_sched")
	sekarang := pukul10WIB.UTC() // zona dinormalkan penjadwal ke WIB
	p := newPenjadwal(s, &sekarang)

	assert.Equal(t, 1, p.Periksa(t.Context()), "hanya SRC-BI yang punya konektor dan jatuh tempo")
	p.Tunggu()
	assert.Zero(t, p.Periksa(t.Context()), "menit yang sama tidak diproses dua kali")

	var e domain.EksekusiSumber
	require.NoError(t, s.db.First(&e).Error)
	assert.Equal(t, domain.PemicuJadwal, e.Pemicu)
	assert.Equal(t, domain.StatusSukses, e.Status)
	assert.Nil(t, e.AktorID)
	assert.EqualValues(t, 14, s.hitung(t, "harga_eksternal"))

	sekarang = sekarang.Add(time.Minute)
	assert.Zero(t, p.Periksa(t.Context()), "di luar menit jadwal tidak ada yang berjalan")
}

func TestPenjadwal_MelewatiSumberDijedaDanJadwalRusak(t *testing.T) {
	s := newSetup(t, "ingest_sched_lewati")
	sekarang := pukul10WIB
	p := newPenjadwal(s, &sekarang)

	require.NoError(t, s.db.Exec(`UPDATE sumber_data SET status = 'DIJEDA' WHERE id = ?`, pihps.SumberID).Error)
	assert.Zero(t, p.Periksa(t.Context()), "sumber dijeda tidak dijadwalkan")

	require.NoError(t, s.db.Exec(`UPDATE sumber_data SET status = 'BELUM_AKTIF', jadwal_cron = 'bukan cron' WHERE id = ?`, pihps.SumberID).Error)
	assert.Zero(t, p.Periksa(t.Context()), "jadwal rusak dilewati tanpa menghentikan penjadwal")
	assert.Zero(t, s.hitung(t, "eksekusi_sumber"))
}

func TestPenjadwal_TidakMenumpukBilaMasihBerjalan(t *testing.T) {
	s := newSetup(t, "ingest_sched_tumpuk")
	sekarang := pukul10WIB
	p := newPenjadwal(s, &sekarang)

	// Run manual sebelumnya belum selesai ketika jadwal tiba.
	_, err := s.runner.Mulai(t.Context(), pihps.SumberID, domain.PemicuManual, nil)
	require.NoError(t, err)

	assert.Zero(t, p.Periksa(t.Context()))
	assert.EqualValues(t, 1, s.hitung(t, "eksekusi_sumber"))
}
