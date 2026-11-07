package notifikasi

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/audit/audittest"
	"github.com/garda-inflasi/api/internal/ews/domain"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var t0 = time.Date(2026, 11, 5, 8, 0, 0, 0, time.UTC)

type tandaan struct {
	id     int64
	status string
	galat  string
	jadwal time.Time
	pada   time.Time
}

// fakeRepo adalah NotifikasiRepository dalam memori.
type fakeRepo struct {
	mu         sync.Mutex
	antre      []idx.ID
	antreErr   error
	klaim      []domain.Pesan
	klaimErr   error
	ditandai   []tandaan
	tandaiErr  error
	prefs      []domain.PreferensiNotifikasi
	tersimpan  []domain.PreferensiNotifikasi
	eskalasi   []idx.ID
	batasEsk   time.Duration
	minimumEsk string
	langganan  []domain.LanggananPush
	dihapus    bool
}

func (f *fakeRepo) Antrekan(_ context.Context, alertID idx.ID, _ []string, _ time.Time) (int, error) {
	f.antre = append(f.antre, alertID)
	return 1, f.antreErr
}
func (f *fakeRepo) Klaim(context.Context, time.Time, int, time.Duration) ([]domain.Pesan, error) {
	return f.klaim, f.klaimErr
}
func (f *fakeRepo) Tandai(_ context.Context, id int64, status, galat string, jadwal, now time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ditandai = append(f.ditandai, tandaan{id, status, galat, jadwal, now})
	return f.tandaiErr
}

func (f *fakeRepo) jumlahDitandai() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.ditandai)
}
func (f *fakeRepo) ListPreferensi(context.Context, string) ([]domain.PreferensiNotifikasi, error) {
	return f.prefs, nil
}
func (f *fakeRepo) SimpanPreferensi(_ context.Context, _ string, p []domain.PreferensiNotifikasi) error {
	f.tersimpan = p
	return nil
}
func (f *fakeRepo) SimpanLangganan(_ context.Context, l domain.LanggananPush) error {
	f.langganan = append(f.langganan, l)
	return nil
}
func (f *fakeRepo) HapusLangganan(context.Context, string, string) (bool, error) {
	return f.dihapus, nil
}
func (f *fakeRepo) ListLangganan(context.Context, string) ([]domain.LanggananPush, error) {
	return f.langganan, nil
}
func (f *fakeRepo) HapusLanggananKadaluwarsa(context.Context, string) error { return nil }
func (f *fakeRepo) EskalasiOtomatis(_ context.Context, _ time.Time, batas time.Duration, minimum string) ([]idx.ID, error) {
	f.batasEsk, f.minimumEsk = batas, minimum
	return f.eskalasi, nil
}

// fakePengirim mencatat pesan dan mengembalikan galat yang diprogram.
type fakePengirim struct {
	kanal string
	err   error
	got   []domain.Pesan
}

func (f *fakePengirim) Kanal() string { return f.kanal }
func (f *fakePengirim) Kirim(_ context.Context, p domain.Pesan) error {
	f.got = append(f.got, p)
	return f.err
}

func newService(repo *fakeRepo, cfg Config, senders ...domain.Pengirim) (*Service, *audittest.Recorder) {
	rec := &audittest.Recorder{}
	s := New(repo, []string{"OPS_KOORDINATOR"}, cfg, rec, logger.NewDefault(), senders...)
	s.now = func() time.Time { return t0 }
	return s, rec
}

func pesan(id int64, kanal string, percobaan int) domain.Pesan {
	return domain.Pesan{NotifikasiID: id, Kanal: kanal, Percobaan: percobaan, PenggunaID: "u", Tujuan: "u@x.id", AlertID: 12, Severity: "KRITIS", Judul: "Judul", Isi: "Isi"}
}

func TestKirim_DeliversThroughAdapterAndMarksSent(t *testing.T) {
	email := &fakePengirim{kanal: domain.KanalEmail}
	repo := &fakeRepo{klaim: []domain.Pesan{pesan(1, domain.KanalEmail, 0)}}
	s, _ := newService(repo, Config{}, email)

	r, err := s.Kirim(t.Context())

	require.NoError(t, err)
	assert.Equal(t, RingkasanKirim{Terkirim: 1}, r)
	require.Len(t, email.got, 1)
	assert.Equal(t, "/ews?id=12", email.got[0].TautanPath, "tautan memakai ID desimal; tautan lama berkode tetap dilayani alias API")
	assert.Equal(t, []tandaan{{id: 1, status: domain.NotifTerkirim, jadwal: t0.Add(time.Minute), pada: t0}}, repo.ditandai)
}

func TestKirim_ChannelWithoutAdapterIsOnlyRecorded(t *testing.T) {
	repo := &fakeRepo{klaim: []domain.Pesan{pesan(1, domain.KanalEmail, 0), pesan(2, domain.KanalPush, 0)}}
	s, _ := newService(repo, Config{}) // tidak ada adaptor terkonfigurasi

	r, err := s.Kirim(t.Context())

	require.NoError(t, err)
	assert.Equal(t, RingkasanKirim{Dicatat: 2}, r)
	for _, m := range repo.ditandai {
		assert.Equal(t, domain.NotifDicatat, m.status)
		assert.Contains(t, m.galat, "belum dikonfigurasi")
	}
}

func TestKirim_NoDestinationIsRecordedWithoutRetry(t *testing.T) {
	push := &fakePengirim{kanal: domain.KanalPush, err: errors.Join(domain.ErrTanpaTujuan, errors.New("belum ada langganan"))}
	repo := &fakeRepo{klaim: []domain.Pesan{pesan(1, domain.KanalPush, 0)}}
	s, _ := newService(repo, Config{}, push)

	r, _ := s.Kirim(t.Context())

	assert.Equal(t, RingkasanKirim{Dicatat: 1}, r)
	assert.Equal(t, domain.NotifDicatat, repo.ditandai[0].status)
}

func TestKirim_TransientFailureRetriesWithBackoffThenFails(t *testing.T) {
	email := &fakePengirim{kanal: domain.KanalEmail, err: errors.New("smtp putus")}
	repo := &fakeRepo{klaim: []domain.Pesan{pesan(1, domain.KanalEmail, 0), pesan(2, domain.KanalEmail, 2), pesan(3, domain.KanalEmail, 4)}}
	s, _ := newService(repo, Config{MaxPercobaan: 5}, email)

	r, err := s.Kirim(t.Context())

	require.NoError(t, err)
	assert.Equal(t, RingkasanKirim{Diulang: 2, Gagal: 1}, r)
	assert.Equal(t, domain.NotifMenunggu, repo.ditandai[0].status)
	assert.Equal(t, t0.Add(time.Minute), repo.ditandai[0].jadwal, "percobaan ke-1: 1 menit")
	assert.Equal(t, t0.Add(4*time.Minute), repo.ditandai[1].jadwal, "percobaan ke-3: 4 menit")
	assert.Equal(t, domain.NotifGagal, repo.ditandai[2].status, "percobaan ke-5 habis")
	assert.Equal(t, "smtp putus", repo.ditandai[2].galat)
}

func TestKirim_ClaimErrorIsReturned(t *testing.T) {
	s, _ := newService(&fakeRepo{klaimErr: assert.AnError}, Config{})
	_, err := s.Kirim(t.Context())
	assert.ErrorIs(t, err, assert.AnError)
}

func TestKirim_SaveErrorSkipsCounting(t *testing.T) {
	email := &fakePengirim{kanal: domain.KanalEmail}
	repo := &fakeRepo{klaim: []domain.Pesan{pesan(1, domain.KanalEmail, 0)}, tandaiErr: assert.AnError}
	s, _ := newService(repo, Config{}, email)

	r, err := s.Kirim(t.Context())

	require.NoError(t, err)
	assert.Equal(t, RingkasanKirim{}, r)
}

func TestBackoff(t *testing.T) {
	assert.Equal(t, time.Minute, backoff(1))
	assert.Equal(t, 2*time.Minute, backoff(2))
	assert.Equal(t, 32*time.Minute, backoff(6))
	assert.Equal(t, time.Hour, backoff(7), "dibatasi 1 jam")
	assert.Equal(t, time.Hour, backoff(50))
	assert.Equal(t, time.Minute, backoff(0))
}

func TestAlertDitulis_EnqueuesAndWakesDispatcher(t *testing.T) {
	repo := &fakeRepo{}
	s, _ := newService(repo, Config{})

	s.AlertDitulis(t.Context(), domain.Alert{EwsAlert: domain.EwsAlert{ID: 2000}}, domain.HasilDibuat)

	assert.Equal(t, []idx.ID{2000}, repo.antre)
	select {
	case <-s.wake:
	default:
		t.Fatal("pekerja pengiriman harus dibangunkan")
	}

	repo.antreErr = assert.AnError
	s.AlertDitulis(t.Context(), domain.Alert{EwsAlert: domain.EwsAlert{ID: 2001}}, domain.HasilDibuat) // galat hanya dilog
}

func TestEskalasi(t *testing.T) {
	t.Run("mati bila batas 0", func(t *testing.T) {
		repo := &fakeRepo{eskalasi: []idx.ID{1}}
		s, rec := newService(repo, Config{})

		ids, err := s.Eskalasi(t.Context())

		require.NoError(t, err)
		assert.Empty(t, ids)
		assert.Empty(t, rec.Entries())
		assert.Zero(t, repo.batasEsk, "repository tidak dipanggil")
	})
	t.Run("mengeskalasi dan mengaudit tiap alert", func(t *testing.T) {
		repo := &fakeRepo{eskalasi: []idx.ID{1, 2}}
		s, rec := newService(repo, Config{EskalasiSetelah: 45 * time.Minute})

		ids, err := s.Eskalasi(t.Context())

		require.NoError(t, err)
		assert.Equal(t, []idx.ID{1, 2}, ids)
		assert.Equal(t, 45*time.Minute, repo.batasEsk)
		assert.Equal(t, domain.SeveritySiaga, repo.minimumEsk)
		entries := rec.Entries()
		require.Len(t, entries, 2)
		assert.Equal(t, "ESKALASI_OTOMATIS", entries[0].Aksi)
		assert.Equal(t, "2", entries[1].EntitasID, "audit mencatat ID alert desimal")
		assert.Equal(t, 45, entries[0].Metadata["setelah_menit"])
	})
}

func TestPreferensi_DefaultsAndSaved(t *testing.T) {
	repo := &fakeRepo{prefs: []domain.PreferensiNotifikasi{{PenggunaID: "u", Kanal: domain.KanalPush, SeverityMin: domain.SeverityKritis, Aktif: false}}}
	s, _ := newService(repo, Config{}, &fakePengirim{kanal: domain.KanalEmail})

	got, err := s.DaftarPreferensi(t.Context(), "u")

	require.NoError(t, err)
	assert.Equal(t, []Preferensi{
		{Kanal: "EMAIL", SeverityMin: "SIAGA", Aktif: true, Tersedia: true},
		{Kanal: "PUSH", SeverityMin: "KRITIS", Aktif: false, Tersedia: false},
	}, got)
}

func TestSimpanPreferensi(t *testing.T) {
	t.Run("valid disimpan dan diaudit", func(t *testing.T) {
		repo := &fakeRepo{}
		s, rec := newService(repo, Config{})

		err := s.SimpanPreferensi(t.Context(), "u", []Preferensi{{Kanal: "EMAIL", SeverityMin: "WASPADA", Aktif: true}})

		require.NoError(t, err)
		require.Len(t, repo.tersimpan, 1)
		assert.Equal(t, "u", repo.tersimpan[0].PenggunaID)
		entry, _ := rec.Last()
		assert.Equal(t, "UBAH_PREFERENSI_NOTIFIKASI", entry.Aksi)
		assert.Equal(t, "u", entry.AktorID)
	})
	t.Run("kanal dan tingkat tak valid ditolak sebelum menyimpan", func(t *testing.T) {
		repo := &fakeRepo{}
		s, rec := newService(repo, Config{})

		err := s.SimpanPreferensi(t.Context(), "u", []Preferensi{{Kanal: "SMS", SeverityMin: "SIAGA"}})
		assert.ErrorIs(t, err, ErrKanalTidakValid)
		err = s.SimpanPreferensi(t.Context(), "u", []Preferensi{{Kanal: "PUSH", SeverityMin: "GENTING"}})
		assert.ErrorIs(t, err, ErrSeverityTidakValid)

		assert.Nil(t, repo.tersimpan)
		assert.Empty(t, rec.Entries())
	})
}

func TestPushSubscription(t *testing.T) {
	repo := &fakeRepo{}
	s, rec := newService(repo, Config{})

	require.NoError(t, s.DaftarPush(t.Context(), "u", domain.LanggananPush{Endpoint: "https://push.example/abc", P256dh: "k", Auth: "a"}))
	assert.Equal(t, "u", repo.langganan[0].PenggunaID)

	ada, err := s.BatalPush(t.Context(), "u", "https://push.example/abc")
	require.NoError(t, err)
	assert.False(t, ada)
	repo.dihapus = true
	ada, err = s.BatalPush(t.Context(), "u", "https://push.example/abc")
	require.NoError(t, err)
	assert.True(t, ada)

	var aksi []string
	for _, e := range rec.Entries() {
		aksi = append(aksi, e.Aksi)
	}
	assert.Equal(t, []string{"DAFTAR_PUSH", "BATAL_PUSH"}, aksi, "pembatalan yang tidak menemukan apa pun tidak diaudit")
}

func TestRun_DisabledReturnsImmediatelyAndEnabledStopsOnCancel(t *testing.T) {
	s, _ := newService(&fakeRepo{}, Config{Interval: 0})
	done := make(chan struct{})
	go func() { s.Run(t.Context()); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("interval 0 harus mematikan Run")
	}

	repo := &fakeRepo{klaim: []domain.Pesan{pesan(1, domain.KanalEmail, 0)}}
	s, _ = newService(repo, Config{Interval: time.Hour}, &fakePengirim{kanal: domain.KanalEmail})
	ctx, cancel := context.WithCancel(t.Context())
	done = make(chan struct{})
	go func() { s.Run(ctx); close(done) }()
	require.Eventually(t, func() bool { return repo.jumlahDitandai() == 1 }, 2*time.Second, 10*time.Millisecond, "putaran pertama berjalan segera")
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run harus berhenti setelah ctx dibatalkan")
	}
}
