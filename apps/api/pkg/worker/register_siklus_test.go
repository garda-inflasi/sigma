package worker

import (
	"context"
	"testing"
	"time"

	ewsjobs "github.com/garda-inflasi/api/internal/ews/jobs"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	siklusjobs "github.com/garda-inflasi/api/internal/siklus/jobs"
	"github.com/garda-inflasi/api/pkg/seed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Job terjadwal siklus_evaluasi dan pemicu setelah alert EWS terpasang pada komponen bersama proses worker: job
// menetapkan siklus dari data nyata, dan alert yang ditulis rule engine EWS (yang dipakai worker) memicu evaluasi ulang
// tanpa menunggu jadwal berikutnya.
func TestSiklusEvaluasi_JobTerjadwalDanPemicuTerpasang(t *testing.T) {
	db := pg.NewDatabase(t, "worker_siklus")
	pgtest.MigrateUp(t, db)
	require.NoError(t, seed.SeedReference(t.Context(), db))
	require.NoError(t, seed.SeedFiles(t.Context(), db, "demo/01_auth_users.sql"))
	d := deps{db: db, log: quiet(), getenv: env(nil)}

	aktif := func() (siklus string, otomatis bool) {
		var row struct {
			Siklus   string
			Otomatis bool
		}
		require.NoError(t, db.Raw(`SELECT siklus::text AS siklus, otomatis FROM siklus_ekonomi WHERE wilayah_kode = '11' AND akhir_pada IS NULL`).Scan(&row).Error)
		return row.Siklus, row.Otomatis
	}

	// Job terjadwal (dijalankan seperti `worker run siklus_evaluasi`): tanpa alert, NORMAL awal dibuka mesin.
	s, err := buildScheduler(d)
	require.NoError(t, err)
	ran, err := s.RunOnce(t.Context(), "siklus_evaluasi")
	require.NoError(t, err)
	require.True(t, ran)
	siklus, otomatis := aktif()
	assert.Equal(t, "NORMAL", siklus)
	assert.True(t, otomatis)

	// buildDaemons memasang pengamat siklus ke rule engine EWS bersama. Pemicu yang sama dijalankan langsung (bukan lewat
	// daemon) agar tes tidak ikut menyalakan evaluasi EWS terjadwal dan pengiriman notifikasi.
	_, err = buildDaemons(d)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	selesai := make(chan struct{})
	go func() { siklusjobs.Shared(db, d.log).Pemicu.Run(ctx); close(selesai) }()
	t.Cleanup(func() {
		cancel()
		<-selesai
	})

	now := time.Now() // rule engine EWS mengukur menurut jam aplikasi
	for kode, row := range map[string]struct {
		harga int
		lalu  time.Duration
	}{"W1": {50000, 72 * time.Hour}, "W2": {60000, 2 * time.Hour}} {
		require.NoError(t, db.Exec(`INSERT INTO entri_harga (kode, komoditas_kode, pasar_id, tipe, harga, satuan, status, dicatat_pada)
			VALUES (?, 'CBM-KRT', 'PSR-1171-01', 'KONSUMEN', ?, 'kg', 'DIVALIDASI', ?)`, kode, row.harga, now.Add(-row.lalu)).Error)
	}
	ringkasan, err := ewsjobs.Shared(db, d.log).Engine.Evaluasi(t.Context())
	require.NoError(t, err)
	require.Equal(t, 1, ringkasan.Dibuat)

	require.Eventually(t, func() bool {
		s, _ := aktif()
		return s == "TEKANAN"
	}, 30*time.Second, 100*time.Millisecond, "siklus harus menjadi TEKANAN setelah alert EWS ditulis (jeda gabungan %s)", siklusjobs.JedaPemicu)
}
