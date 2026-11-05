package workflow

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/audit/audittest"
	ewsdomain "github.com/garda-inflasi/api/internal/ews/domain"
	"github.com/garda-inflasi/api/internal/intervensi/domain"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var t0 = time.Date(2026, 11, 5, 8, 0, 0, 0, time.UTC)

// fakeRepo mengimplementasi domain.WorkflowRepository seperlunya.
type fakeRepo struct {
	domain.WorkflowRepository
	kartu     []domain.KartuOtomatis
	dibuat    *domain.Intervensi
	buatErr   error
	lewat     []domain.KartuMelewatiSLA
	lewatErr  error
	eskalasi  map[string]bool // id -> berhasil
	alasan    map[string]string
	eskalasiE error
}

func (f *fakeRepo) BuatKartuOtomatis(_ context.Context, in domain.KartuOtomatis) (*domain.Intervensi, error) {
	f.kartu = append(f.kartu, in)
	return f.dibuat, f.buatErr
}

func (f *fakeRepo) KartuMelewatiSLA(context.Context, time.Time) ([]domain.KartuMelewatiSLA, error) {
	return f.lewat, f.lewatErr
}

func (f *fakeRepo) EskalasiSLA(_ context.Context, id, alasan string) (bool, error) {
	if f.alasan == nil {
		f.alasan = map[string]string{}
	}
	f.alasan[id] = alasan
	return f.eskalasi[id], f.eskalasiE
}

func service(repo *fakeRepo, cfg Config) (*Service, *audittest.Recorder) {
	rec := &audittest.Recorder{}
	s := New(repo, rec, cfg, logger.NewDefault())
	s.now = func() time.Time { return t0 }
	return s, rec
}

func alertContoh(jenis, severity string) ewsdomain.Alert {
	kode := "CBM-KRT"
	return ewsdomain.Alert{
		EwsAlert: ewsdomain.EwsAlert{
			ID: "EWS-2000", Jenis: jenis, Severity: severity, Judul: "Lonjakan harga cabai merah keriting",
			Pesan: "Harga naik 20%.", KomoditasKode: &kode, KabupatenKode: "1171",
		},
		KabupatenNama: "KOTA BANDA ACEH",
	}
}

func TestSiklusUntukDanPrioritasUntuk(t *testing.T) {
	for jenis, want := range map[string]string{
		ewsdomain.JenisLonjakanHarga: domain.SiklusTekanan, ewsdomain.JenisKelangkaanStok: domain.SiklusTekanan,
		ewsdomain.JenisAnomaliCuaca: domain.SiklusTekanan, ewsdomain.JenisOverSupply: domain.SiklusSurplus,
	} {
		got, ok := SiklusUntuk(jenis)
		assert.True(t, ok, jenis)
		assert.Equal(t, want, got, jenis)
	}
	_, ok := SiklusUntuk("ANOMALI_DATA")
	assert.False(t, ok, "anomali data bukan pemicu intervensi")

	assert.Equal(t, "TINGGI", PrioritasUntuk(ewsdomain.SeverityKritis))
	assert.Equal(t, "SEDANG", PrioritasUntuk(ewsdomain.SeveritySiaga))
	assert.Equal(t, "RENDAH", PrioritasUntuk(ewsdomain.SeverityWaspada))
}

func TestAlertDitulis_CreatesCardForSeriousAlerts(t *testing.T) {
	repo := &fakeRepo{dibuat: &domain.Intervensi{ID: "INT-1000", Siklus: "TEKANAN", Tahap: "t-ews", Judul: "x", Prioritas: "TINGGI"}}
	s, rec := service(repo, Config{KartuOtomatisMin: ewsdomain.SeveritySiaga})

	s.AlertDitulis(t.Context(), alertContoh(ewsdomain.JenisLonjakanHarga, ewsdomain.SeverityKritis), ewsdomain.HasilDibuat)

	require.Len(t, repo.kartu, 1)
	k := repo.kartu[0]
	assert.Equal(t, "EWS-2000", k.AlertID)
	assert.Equal(t, domain.SiklusTekanan, k.Siklus)
	assert.Equal(t, "Lonjakan harga cabai merah keriting — Kota Banda Aceh", k.Judul)
	assert.Equal(t, "Harga naik 20%.", k.Deskripsi)
	assert.Equal(t, "CBM-KRT", k.KomoditasKode)
	assert.Equal(t, "1171", k.KabupatenKode)
	assert.Equal(t, "TINGGI", k.Prioritas)
	assert.Equal(t, []string{"MANAJER_PROGRAM"}, k.PeranPenanggungJawab)
	assert.Equal(t, t0, k.Sekarang)

	entry, ok := rec.Last()
	require.True(t, ok)
	assert.Equal(t, "BUAT_OTOMATIS", entry.Aksi)
	assert.Empty(t, entry.AktorID, "dibuat sistem")
	assert.Equal(t, "INT-1000", entry.EntitasID)
	assert.Equal(t, "EWS-2000", entry.Metadata["ews_alert_id"])
}

func TestAlertDitulis_SkipsWhatDoesNotQualify(t *testing.T) {
	cases := map[string]struct {
		cfg   Config
		alert ewsdomain.Alert
	}{
		"di bawah tingkat minimum": {Config{KartuOtomatisMin: ewsdomain.SeveritySiaga}, alertContoh(ewsdomain.JenisLonjakanHarga, ewsdomain.SeverityWaspada)},
		"fitur dimatikan":          {Config{}, alertContoh(ewsdomain.JenisLonjakanHarga, ewsdomain.SeverityKritis)},
		"jenis bukan pemicu":       {Config{KartuOtomatisMin: ewsdomain.SeveritySiaga}, alertContoh("ANOMALI_DATA", ewsdomain.SeverityKritis)},
		"tanpa komoditas (kartu butuh)": {Config{KartuOtomatisMin: ewsdomain.SeveritySiaga}, func() ewsdomain.Alert {
			a := alertContoh(ewsdomain.JenisAnomaliCuaca, ewsdomain.SeverityKritis)
			a.KomoditasKode = nil
			return a
		}()},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			repo := &fakeRepo{}
			s, rec := service(repo, tc.cfg)
			s.AlertDitulis(t.Context(), tc.alert, ewsdomain.HasilDibuat)
			assert.Empty(t, repo.kartu)
			assert.Empty(t, rec.Entries())
		})
	}
}

func TestAlertDitulis_ExistingCardOrFailureWritesNoAudit(t *testing.T) {
	repo := &fakeRepo{} // dibuat nil: alert sudah punya kartu
	s, rec := service(repo, Config{KartuOtomatisMin: ewsdomain.SeveritySiaga})
	s.AlertDitulis(t.Context(), alertContoh(ewsdomain.JenisLonjakanHarga, ewsdomain.SeveritySiaga), ewsdomain.HasilDinaikkan)
	assert.Len(t, repo.kartu, 1, "alert yang naik tingkat tetap dicek: mungkin belum punya kartu")
	assert.Empty(t, rec.Entries())

	repo = &fakeRepo{buatErr: errors.New("db putus")}
	s, rec = service(repo, Config{KartuOtomatisMin: ewsdomain.SeveritySiaga})
	s.AlertDitulis(t.Context(), alertContoh(ewsdomain.JenisLonjakanHarga, ewsdomain.SeveritySiaga), ewsdomain.HasilDibuat)
	assert.Empty(t, rec.Entries())
}

func TestAlertDitulis_TitleIsTruncatedToColumnLength(t *testing.T) {
	repo := &fakeRepo{}
	s, _ := service(repo, Config{KartuOtomatisMin: ewsdomain.SeveritySiaga})
	alert := alertContoh(ewsdomain.JenisLonjakanHarga, ewsdomain.SeveritySiaga)
	alert.Judul = string(make([]byte, 400))

	s.AlertDitulis(t.Context(), alert, ewsdomain.HasilDibuat)

	require.Len(t, repo.kartu, 1)
	assert.Len(t, repo.kartu[0].Judul, maksJudul)
}

func TestCekSLA_EscalatesOverdueCardsAndAudits(t *testing.T) {
	repo := &fakeRepo{
		lewat: []domain.KartuMelewatiSLA{
			{ID: "INT-1", TahapKode: "t-ews", Label: "EWS terpicu", SLAHari: 1},
			{ID: "INT-2", TahapKode: "t-verifikasi", Label: "Verifikasi cepat BI/BPS", SLAHari: 2},
			{ID: "INT-3", TahapKode: "t-logistik", Label: "Logistik", SLAHari: 3},
		},
		eskalasi: map[string]bool{"INT-1": true, "INT-2": false, "INT-3": true}, // INT-2 sudah tidak memenuhi syarat
	}
	s, rec := service(repo, Config{SLAInterval: time.Hour})

	n, err := s.CekSLA(t.Context())

	require.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Equal(t, `Melewati SLA tahap "EWS terpicu" (1 hari)`, repo.alasan["INT-1"])
	entries := rec.Entries()
	require.Len(t, entries, 2)
	assert.Equal(t, "ESKALASI_SLA", entries[0].Aksi)
	assert.Equal(t, "INT-1", entries[0].EntitasID)
	assert.Equal(t, "INT-3", entries[1].EntitasID)
}

func TestCekSLA_FailureOnOneCardDoesNotStopTheOthers(t *testing.T) {
	repo := &fakeRepo{lewat: []domain.KartuMelewatiSLA{{ID: "INT-1", Label: "A", SLAHari: 1}}, eskalasiE: assert.AnError}
	s, rec := service(repo, Config{})

	n, err := s.CekSLA(t.Context())

	require.NoError(t, err)
	assert.Zero(t, n)
	assert.Empty(t, rec.Entries())

	repo.lewatErr = assert.AnError
	_, err = s.CekSLA(t.Context())
	assert.ErrorIs(t, err, assert.AnError)
}

func TestRun(t *testing.T) {
	s, _ := service(&fakeRepo{}, Config{SLAInterval: 0})
	done := make(chan struct{})
	go func() { s.Run(t.Context()); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("interval 0 mematikan Run")
	}

	repo := &fakeRepo{lewat: []domain.KartuMelewatiSLA{{ID: "INT-1", Label: "A", SLAHari: 1}}, eskalasi: map[string]bool{"INT-1": true}}
	s, rec := service(repo, Config{SLAInterval: time.Hour})
	ctx, cancel := context.WithCancel(t.Context())
	done = make(chan struct{})
	go func() { s.Run(ctx); close(done) }()
	require.Eventually(t, func() bool { return len(rec.Entries()) == 1 }, 2*time.Second, 10*time.Millisecond, "pemeriksaan pertama segera")
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run harus berhenti setelah ctx dibatalkan")
	}
}

func TestConfigFromEnv(t *testing.T) {
	t.Setenv("INTERVENSI_SLA_INTERVAL", "")
	t.Setenv("INTERVENSI_KARTU_OTOMATIS_MIN", "")
	cfg, err := ConfigFromEnv()
	require.NoError(t, err)
	assert.Equal(t, Config{KartuOtomatisMin: "SIAGA"}, cfg, "bawaan: kartu otomatis dari SIAGA, SLA mati")

	t.Setenv("INTERVENSI_SLA_INTERVAL", "30m")
	t.Setenv("INTERVENSI_KARTU_OTOMATIS_MIN", "kritis")
	cfg, err = ConfigFromEnv()
	require.NoError(t, err)
	assert.Equal(t, Config{SLAInterval: 30 * time.Minute, KartuOtomatisMin: "KRITIS"}, cfg)

	t.Setenv("INTERVENSI_SLA_INTERVAL", "off")
	t.Setenv("INTERVENSI_KARTU_OTOMATIS_MIN", "off")
	cfg, err = ConfigFromEnv()
	require.NoError(t, err)
	assert.Equal(t, Config{}, cfg)

	t.Setenv("INTERVENSI_SLA_INTERVAL", "sering")
	t.Setenv("INTERVENSI_KARTU_OTOMATIS_MIN", "GENTING")
	cfg, err = ConfigFromEnv()
	require.Error(t, err)
	assert.ErrorContains(t, err, "INTERVENSI_SLA_INTERVAL")
	assert.ErrorContains(t, err, "INTERVENSI_KARTU_OTOMATIS_MIN")
	assert.Equal(t, Config{KartuOtomatisMin: "SIAGA"}, cfg, "nilai tak valid memakai bawaan")
}

func TestKapital(t *testing.T) {
	assert.Equal(t, "Kota Banda Aceh", kapital("KOTA BANDA ACEH"))
	assert.Equal(t, "", kapital(""))
}
