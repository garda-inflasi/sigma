package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/dashboard/refresher"
	"github.com/garda-inflasi/api/internal/dashboard/usecase"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Cache hasil agregasi: hasil disimpan selama TTL, dan dibatalkan begitu refresher selesai menyegarkan agregat.

func entriHariIni(t *testing.T, s *suite, email, kab string) int {
	t.Helper()
	for _, a := range s.aktivitasDari(t, email) {
		if a.KabupatenKode == kab {
			return a.Entri
		}
	}
	require.Failf(t, "kabupaten tidak ada di aktivitas lapangan", kab)
	return 0
}

// aktivitas-lapangan membaca tabel dasar langsung (bukan view), jadi perubahan data hanya terlihat bila cache dibuang:
// di sinilah invalidasi oleh penyegaran terbukti.
func TestCache_HasilDisimpanLaluDibatalkanSaatPenyegaran(t *testing.T) {
	s := newSuite(t, "dashboard_cache_invalidasi", true)
	h := hariIni()
	sebelum := entriHariIni(t, s, emailBerhak, kabBanda)

	// Entri baru hari ini di Banda Aceh, langsung ke tabel (tanpa melewati API).
	s.sisipEntri(t, h, pasarBanda, "TMT", "DIAJUKAN", 5000, 1)

	assert.Equal(t, sebelum, entriHariIni(t, s, emailBerhak, kabBanda),
		"dalam TTL hasil tersimpan disajikan: perhitungan tidak diulang")

	s.segarkan(t) // refresher selesai: epoch naik

	assert.Equal(t, sebelum+1, entriHariIni(t, s, emailBerhak, kabBanda),
		"sesudah penyegaran data baru langsung tampil, tanpa menunggu TTL")
}

// Alur sebenarnya: validasi dan publikasi harga lewat API memicu penyegaran di latar belakang; dashboard yang sudah
// ter-cache harus menampilkan hasilnya segera setelah penyegaran selesai, jauh sebelum TTL (20 detik) habis.
func TestCache_PublikasiHargaMenampilkanDataBaruTanpaMenungguTTL(t *testing.T) {
	s := newSuite(t, "dashboard_cache_publikasi", true)
	// Penyegaran latar belakang harus selesai sebelum basis data uji dibuang.
	t.Cleanup(func() {
		assert.Eventually(t, func() bool {
			var aktif int64
			_ = s.db.Raw("SELECT COUNT(*) FROM pg_stat_activity WHERE (query ILIKE 'REFRESH MATERIALIZED VIEW%' OR query ILIKE 'SELECT garda_segarkan_mv%') AND state = 'active'").Scan(&aktif).Error
			return aktif == 0
		}, 20*time.Second, 100*time.Millisecond)
	})

	awal := kartuByID(t, s.kpiDari(t, emailBerhak, ""), "validasi").Nilai // mengisi cache
	require.Equal(t, awal, kartuByID(t, s.kpiDari(t, emailBerhak, ""), "validasi").Nilai)

	var ids []string
	require.NoError(t, s.db.Raw("SELECT id FROM entri_harga WHERE status = 'DIAJUKAN' AND petugas_id <> garda_uuid_stabil('pengguna', 'koordinator_lapangan') LIMIT 400").Scan(&ids).Error)
	require.NotEmpty(t, ids)
	valid := s.call(t, http.MethodPost, "/v1/harga/bulk", emailKoordinator, map[string]any{"action": "validasi", "ids": ids})
	require.Equal(t, http.StatusOK, valid.Code, valid.Body.String())
	pub := s.call(t, http.MethodPost, "/v1/harga/bulk", emailManajerData, map[string]any{"action": "publikasi", "ids": ids})
	require.Equal(t, http.StatusOK, pub.Code, pub.Body.String())

	// Batas 15 detik < TTL 20 detik: lolos hanya bila cache dibatalkan penyegaran, bukan karena TTL habis. Penyegaran
	// data demo selesai dalam ~1 detik; sisanya margin untuk mesin yang sedang sibuk.
	const batas = 15 * time.Second
	require.Less(t, batas, usecase.TTLHasil)
	require.Eventually(t, func() bool {
		return kartuByID(t, s.kpiDari(t, emailBerhak, ""), "validasi").Nilai > awal
	}, batas, 100*time.Millisecond, "tingkat validasi harus naik setelah entri divalidasi dan diterbitkan")
}

func TestRefresher_EpochNaikSetiapPenyegaranTermasukYangGagal(t *testing.T) {
	s := newSuite(t, "dashboard_epoch", false)
	r := refresher.Shared(s.db, logger.NewDefault())

	e0 := r.Epoch()
	require.NoError(t, r.Refresh(t.Context()))
	e1 := r.Epoch()
	assert.Greater(t, e1, e0, "penyegaran selesai menaikkan epoch")

	ctx, batal := context.WithCancel(t.Context())
	batal()
	require.Error(t, r.Refresh(ctx), "konteks batal: penyegaran gagal")
	assert.Greater(t, r.Epoch(), e1, "penyegaran yang gagal pun menaikkan epoch: view yang sempat disegarkan sudah berubah")

	assert.Same(t, r, refresher.Shared(s.db, logger.NewDefault()), "satu refresher per koneksi: epoch dipakai bersama modul dashboard")
}
