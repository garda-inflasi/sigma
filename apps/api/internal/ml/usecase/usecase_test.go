package usecase

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/ml/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// repoPalsu adalah Repository dalam memori; galat diisi untuk mensimulasikan Mongo mati.
type repoPalsu struct {
	siklus     map[string]domain.Siklus
	galat      error
	tunda      time.Duration
	nTerbaru   atomic.Int32
	nSnapshot  atomic.Int32
	runs       []domain.RunSumber
	snapshotEw []domain.SinyalEws
}

func (r *repoPalsu) tunggu(ctx context.Context) error {
	if r.tunda > 0 {
		select {
		case <-time.After(r.tunda):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return r.galat
}

func (r *repoPalsu) SiklusTerbaru(ctx context.Context) (*domain.Siklus, error) {
	r.nTerbaru.Add(1)
	if err := r.tunggu(ctx); err != nil {
		return nil, err
	}
	var best *domain.Siklus
	for _, s := range r.siklus {
		s := s
		if domain.Final(s.Status) && (best == nil || s.CycleTimestamp.After(best.CycleTimestamp)) {
			best = &s
		}
	}
	if best == nil {
		return nil, domain.ErrTidakDitemukan
	}
	return best, nil
}

func (r *repoPalsu) Siklus(ctx context.Context, id string) (*domain.Siklus, error) {
	if err := r.tunggu(ctx); err != nil {
		return nil, err
	}
	s, ok := r.siklus[id]
	if !ok {
		return nil, domain.ErrTidakDitemukan
	}
	return &s, nil
}

func (r *repoPalsu) ListSiklus(ctx context.Context, q domain.ListSiklusQuery) ([]domain.Siklus, int64, error) {
	return nil, 0, r.tunggu(ctx)
}

func (r *repoPalsu) Snapshot(ctx context.Context, s domain.Siklus) (*domain.Snapshot, error) {
	r.nSnapshot.Add(1)
	if err := r.tunggu(ctx); err != nil {
		return nil, err
	}
	return &domain.Snapshot{Siklus: s, Ews: r.snapshotEw}, nil
}

func (r *repoPalsu) Forecast(ctx context.Context, c, k, w string) (*domain.Forecast, error) {
	return nil, domain.ErrTidakDitemukan
}

func (r *repoPalsu) RunSumber(ctx context.Context) ([]domain.RunSumber, error) {
	return r.runs, r.tunggu(ctx)
}

func (r *repoPalsu) Ping(ctx context.Context) error { return r.galat }

func siklusUji() map[string]domain.Siklus {
	return map[string]domain.Siklus{
		"C20261124T08Z": {CycleID: "C20261124T08Z", Status: domain.StatusBerhasil, CycleTimestamp: time.Date(2026, 11, 24, 8, 0, 0, 0, time.UTC)},
		"C20261124T09Z": {CycleID: "C20261124T09Z", Status: domain.StatusSebagian, CycleTimestamp: time.Date(2026, 11, 24, 9, 0, 0, 0, time.UTC)},
		"C20261124T10Z": {CycleID: "C20261124T10Z", Status: domain.StatusBerjalan, CycleTimestamp: time.Date(2026, 11, 24, 10, 0, 0, 0, time.UTC)},
	}
}

func TestService_TanpaKonfigurasi(t *testing.T) {
	svc := New(nil, "", nil)
	assert.False(t, svc.Dikonfigurasi())
	_, k := svc.Snapshot(context.Background(), "")
	assert.Equal(t, Keadaan{Alasan: AlasanTidakDikonfigurasi}, k)
	_, k = New(nil, AlasanKonfigurasiInvalid, nil).Snapshot(context.Background(), "")
	assert.Equal(t, AlasanKonfigurasiInvalid, k.Alasan)
}

func TestService_PemilihanSiklus(t *testing.T) {
	repo := &repoPalsu{siklus: siklusUji()}
	svc := New(repo, "", nil)
	ctx := context.Background()

	snap, k := svc.Snapshot(ctx, "")
	require.True(t, k.Tersedia)
	assert.Equal(t, "C20261124T09Z", snap.Siklus.CycleID, "terbaru yang final")

	snap, k = svc.Snapshot(ctx, "C20261124T08Z")
	require.True(t, k.Tersedia)
	assert.Equal(t, "C20261124T08Z", snap.Siklus.CycleID)

	for id, alasan := range map[string]string{
		"C20261124T10Z": AlasanSiklusBelumFinal, "C20200101T00Z": AlasanSiklusTidakDitemukan,
		"c20261124t09z": AlasanCycleIDTidakValid, "2026-11-24": AlasanCycleIDTidakValid,
	} {
		_, k = svc.Snapshot(ctx, id)
		assert.Equal(t, Keadaan{Alasan: alasan}, k, id)
	}

	kosong := New(&repoPalsu{siklus: map[string]domain.Siklus{}}, "", nil)
	_, k = kosong.Snapshot(ctx, "")
	assert.Equal(t, AlasanBelumAdaSiklus, k.Alasan)
}

func TestService_CacheSnapshotPerSiklus(t *testing.T) {
	repo := &repoPalsu{siklus: siklusUji()}
	svc := New(repo, "", nil)
	for i := 0; i < 5; i++ {
		_, k := svc.Snapshot(context.Background(), "")
		require.True(t, k.Tersedia)
	}
	assert.EqualValues(t, 1, repo.nTerbaru.Load(), "siklus terbaru di-cache")
	assert.EqualValues(t, 1, repo.nSnapshot.Load(), "snapshot siklus append-only di-cache")

	// Setelah TTL, siklus terbaru dibaca ulang tetapi snapshot siklus yang sama tidak.
	t0 := time.Now()
	svc.SetJam(func() time.Time { return t0.Add(time.Minute) })
	_, _ = svc.Snapshot(context.Background(), "")
	assert.EqualValues(t, 2, repo.nTerbaru.Load())
	assert.EqualValues(t, 1, repo.nSnapshot.Load())
}

func TestService_PemutusSaatMongoMati(t *testing.T) {
	repo := &repoPalsu{siklus: siklusUji(), galat: errors.New("server selection timeout")}
	svc := New(repo, "", nil)
	now := time.Now()
	svc.SetJam(func() time.Time { return now })

	_, k := svc.Snapshot(context.Background(), "")
	assert.Equal(t, AlasanMongoTidakTersedia, k.Alasan)
	_, k = svc.Snapshot(context.Background(), "")
	assert.Equal(t, AlasanMongoTidakTersedia, k.Alasan)
	assert.EqualValues(t, 1, repo.nTerbaru.Load(), "pemutus terbuka: Mongo tidak dicoba lagi")

	// Setelah jeda pemutus, Mongo yang pulih kembali dipakai.
	repo.galat = nil
	svc.SetJam(func() time.Time { return now.Add(jedaPutus + time.Second) })
	_, k = svc.Snapshot(context.Background(), "")
	assert.True(t, k.Tersedia)
}

// Anggaran request yang habis tidak membuka pemutus: pemuatan latar tetap selesai dan di-cache.
func TestService_LambatTidakMembukaPemutus(t *testing.T) {
	repo := &repoPalsu{siklus: siklusUji(), tunda: 150 * time.Millisecond}
	svc := New(repo, "", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, k := svc.Snapshot(ctx, "")
	assert.Equal(t, AlasanMongoLambat, k.Alasan)
	assert.Eventually(t, func() bool {
		_, k := svc.Snapshot(context.Background(), "")
		return k.Tersedia
	}, 2*time.Second, 20*time.Millisecond)
}

func TestKategoriDanUmur(t *testing.T) {
	lad := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	// 24 Nov 18:00 UTC = 25 Nov 01:00 WIB.
	umur := UmurHari(&lad, time.Date(2026, 11, 24, 18, 0, 0, 0, time.UTC))
	require.NotNil(t, umur)
	assert.Equal(t, 5, *umur)
	assert.Nil(t, UmurHari(nil, time.Now()))
	depan := time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, 0, *UmurHari(&depan, time.Date(2026, 11, 24, 1, 0, 0, 0, time.UTC)), "prakiraan di masa depan tidak berumur negatif")

	assert.Equal(t, KategoriBelumAda, Kategori(nil))
	assert.Equal(t, KategoriDilewati, Kategori(&domain.CrawlRun{Status: "ERROR", Pesan: "DILEWATI: robots"}))
	assert.Equal(t, KategoriGagal, Kategori(&domain.CrawlRun{Status: "ERROR", Pesan: "HTTP 503"}))
	assert.Equal(t, KategoriBasi, Kategori(&domain.CrawlRun{Status: "STALE_CARRIED_FORWARD"}))
	assert.Equal(t, KategoriSegar, Kategori(&domain.CrawlRun{Status: "FRESH"}))
}

func TestKesegaran_TanpaSiklusTetapMelaporkanCrawl(t *testing.T) {
	repo := &repoPalsu{siklus: map[string]domain.Siklus{}, runs: []domain.RunSumber{
		{Sumber: "PIHPS", Dataset: "harga", Terjadwal: &domain.CrawlRun{Status: "ERROR", Pesan: "HTTP 500"}},
	}}
	h, k := New(repo, "", nil).Kesegaran(context.Background())
	require.True(t, k.Tersedia)
	assert.Nil(t, h.Siklus)
	assert.Equal(t, AlasanBelumAdaSiklus, h.KeadaanSiklus.Alasan)
	require.Len(t, h.Entri, 1)
	assert.Equal(t, KategoriGagal, h.Entri[0].Kategori)
}
