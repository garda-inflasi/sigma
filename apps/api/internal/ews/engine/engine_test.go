package engine

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

var aturanLonjakan = domain.AturanEws{
	ID: 1, Jenis: domain.JenisLonjakanHarga, AmbangWaspada: 8, AmbangSiaga: 12, AmbangKritis: 18,
	JendelaHari: 7, CooldownJam: 24,
}

func TestSeverity_ByThreshold(t *testing.T) {
	cases := map[float64]string{
		0: "", 7.99: "", 8: domain.SeverityWaspada, 11.9: domain.SeverityWaspada,
		12: domain.SeveritySiaga, 17.9: domain.SeveritySiaga, 18: domain.SeverityKritis, 40: domain.SeverityKritis,
	}
	for besaran, want := range cases {
		assert.Equal(t, want, Severity(besaran, aturanLonjakan), "besaran %v", besaran)
	}
}

func TestBesaran_StockDeclineIsNegativeValue(t *testing.T) {
	assert.EqualValues(t, 32, besaran(domain.JenisKelangkaanStok, -32), "penurunan stok dibandingkan secara mutlak")
	assert.EqualValues(t, -5, besaran(domain.JenisKelangkaanStok, 5), "stok yang bertambah tidak pernah melampaui ambang")
	assert.EqualValues(t, 21.4, besaran(domain.JenisLonjakanHarga, 21.4))
}

func TestBaru_MessagesPerJenis(t *testing.T) {
	kode := "CBM-KRT"
	p := domain.Pengukuran{
		KomoditasKode: &kode, KomoditasNama: "Cabai merah keriting", KabupatenKode: "1171", KabupatenNama: "KOTA BANDA ACEH",
		Nilai: 21.44, Terbaru: 66500, SatuanKomoditas: "kg",
	}

	lonjakan := baru(aturanLonjakan, p, domain.SeverityKritis)
	assert.Equal(t, "Lonjakan harga cabai merah keriting", lonjakan.Judul)
	assert.Equal(t, "Harga konsumen naik 21,4% di atas rata-rata 7 hari di Kota Banda Aceh. Harga rata-rata terbaru Rp66.500/kg.", lonjakan.Pesan)
	assert.Equal(t, "LONJAKAN_HARGA|CBM-KRT|1171", lonjakan.DedupKey)
	assert.Equal(t, "%", lonjakan.SatuanNilai)
	assert.Equal(t, 24, lonjakan.CooldownJam)
	assert.Equal(t, 91, lonjakan.SkorDampak, "kritis 80 + int(21,44/18*10) = 91")

	stok := baru(domain.AturanEws{ID: 2, Jenis: domain.JenisKelangkaanStok, AmbangWaspada: 25, AmbangSiaga: 40, AmbangKritis: 55, JendelaHari: 5},
		domain.Pengukuran{KomoditasKode: &kode, KomoditasNama: "Cabai merah keriting", KabupatenKode: "1108", KabupatenNama: "KOTA LANGSA", Nilai: -32, Terbaru: 12.5},
		domain.SeverityWaspada)
	assert.Equal(t, "Stok cabai merah keriting menipis", stok.Judul)
	assert.Equal(t, "Stok layak simpan turun 32,0% dalam 5 hari di Kota Langsa; tersisa 12,5 ton.", stok.Pesan)
	assert.EqualValues(t, -32, stok.Nilai, "nilai tersimpan negatif seperti seed")

	cuaca := baru(domain.AturanEws{ID: 3, Jenis: domain.JenisAnomaliCuaca, AmbangWaspada: 100, AmbangSiaga: 140, AmbangKritis: 200, JendelaHari: 10},
		domain.Pengukuran{KabupatenKode: "1106", KabupatenNama: "ACEH TENGAH", Nilai: 148, Terbaru: 148}, domain.SeveritySiaga)
	assert.Equal(t, "mm", cuaca.SatuanNilai)
	assert.Equal(t, "Curah hujan kumulatif 148 mm dalam 10 hari di Aceh Tengah.", cuaca.Pesan)
	assert.Nil(t, cuaca.KomoditasKode)
	assert.Equal(t, "ANOMALI_CUACA|-|1106", cuaca.DedupKey)
}

func TestSkor_StaysBelowOneHundred(t *testing.T) {
	assert.Equal(t, 45, skor(9, domain.SeverityWaspada, aturanLonjakan))
	assert.Equal(t, 99, skor(500, domain.SeverityKritis, aturanLonjakan))
}

func TestAngka_IndonesianFormat(t *testing.T) {
	assert.Equal(t, "1.234.567", angka(1234567, 0))
	assert.Equal(t, "12,3", angka(12.34, 1))
	assert.Equal(t, "0,0", angka(0, 1))
	assert.Equal(t, "999", angka(-999, 0), "tanda dibuang; arah ditulis di kalimat")
}

// fakeRepo adalah EngineRepository dalam memori yang meniru dedup dan peredaman.
type fakeRepo struct {
	mu       sync.Mutex
	aturan   []domain.AturanEws
	ukuran   map[idx.ID][]domain.Pengukuran
	ukurErr  map[idx.ID]error
	aktif    map[string]domain.Alert // dedup_key -> alert aktif
	ditulis  []domain.NewAlert
	terapErr error
}

func (f *fakeRepo) AturanAktif(context.Context) ([]domain.AturanEws, error) { return f.aturan, nil }

func (f *fakeRepo) Ukur(_ context.Context, a domain.AturanEws, _ time.Time) ([]domain.Pengukuran, error) {
	return f.ukuran[a.ID], f.ukurErr[a.ID]
}

func (f *fakeRepo) Terapkan(_ context.Context, in domain.NewAlert, _ time.Time) (domain.HasilTulis, *domain.Alert, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.terapErr != nil {
		return "", nil, f.terapErr
	}
	f.ditulis = append(f.ditulis, in)
	if lama, ok := f.aktif[in.DedupKey]; ok {
		if domain.SeverityRank(in.Severity) <= domain.SeverityRank(lama.Severity) {
			return domain.HasilDilewati, nil, nil
		}
		lama.Severity = in.Severity
		f.aktif[in.DedupKey] = lama
		return domain.HasilDinaikkan, &lama, nil
	}
	id := idx.ID(len(f.aktif) + 1)
	kode := "EWS-" + in.DedupKey
	alert := domain.Alert{EwsAlert: domain.EwsAlert{ID: id, Kode: &kode, Severity: in.Severity, Jenis: in.Jenis, Nilai: in.Nilai}}
	f.aktif[in.DedupKey] = alert
	return domain.HasilDibuat, &alert, nil
}

type observerSpy struct {
	alerts []domain.Alert
	hasil  []domain.HasilTulis
}

func (o *observerSpy) AlertDitulis(_ context.Context, a domain.Alert, h domain.HasilTulis) {
	o.alerts = append(o.alerts, a)
	o.hasil = append(o.hasil, h)
}

func pengukuran(kab string, nilai float64) domain.Pengukuran {
	kode := "CBM-KRT"
	return domain.Pengukuran{KomoditasKode: &kode, KomoditasNama: "Cabai", KabupatenKode: kab, KabupatenNama: "KAB " + kab, Nilai: nilai, Terbaru: 60000, SatuanKomoditas: "kg"}
}

func newEngine(repo *fakeRepo, observers ...domain.Observer) (*Engine, *audittest.Recorder) {
	rec := &audittest.Recorder{}
	e := New(repo, rec, logger.NewDefault(), observers...)
	e.now = func() time.Time { return time.Date(2026, 11, 5, 8, 0, 0, 0, time.UTC) }
	return e, rec
}

func TestEvaluasi_CreatesRaisesAndSkips(t *testing.T) {
	repo := &fakeRepo{
		aturan: []domain.AturanEws{aturanLonjakan},
		ukuran: map[idx.ID][]domain.Pengukuran{aturanLonjakan.ID: {
			pengukuran("1171", 21), pengukuran("1108", 9), pengukuran("1106", 3), // kritis, waspada, di bawah ambang
		}},
		aktif: map[string]domain.Alert{},
	}
	spy := &observerSpy{}
	e, rec := newEngine(repo, spy)

	first, err := e.Evaluasi(t.Context())
	require.NoError(t, err)
	assert.Equal(t, domain.RingkasanEvaluasi{Aturan: 1, Diukur: 3, Dibuat: 2}, first)
	assert.Len(t, spy.alerts, 2)

	// Putaran kedua dengan data sama: tidak ada alert ganda.
	second, err := e.Evaluasi(t.Context())
	require.NoError(t, err)
	assert.Equal(t, domain.RingkasanEvaluasi{Aturan: 1, Diukur: 3, Dilewati: 2}, second)
	assert.Len(t, spy.alerts, 2, "observer hanya dipanggil untuk alert baru atau naik tingkat")

	// Kondisi memburuk: 1108 naik dari waspada ke siaga.
	repo.ukuran[aturanLonjakan.ID][1] = pengukuran("1108", 13)
	third, err := e.Evaluasi(t.Context())
	require.NoError(t, err)
	assert.Equal(t, domain.RingkasanEvaluasi{Aturan: 1, Diukur: 3, Dinaikkan: 1, Dilewati: 1}, third)
	require.Len(t, spy.alerts, 3)
	assert.Equal(t, domain.HasilDinaikkan, spy.hasil[2])
	assert.Equal(t, domain.SeveritySiaga, spy.alerts[2].Severity)

	var aksi []string
	for i, entry := range rec.Entries() {
		aksi = append(aksi, entry.Aksi)
		assert.Empty(t, entry.AktorID, "aksi mesin tercatat tanpa aktor pengguna")
		assert.Equal(t, "ews_alert", entry.Entitas)
		assert.Equal(t, spy.alerts[i].ID.String(), entry.EntitasID, "audit mencatat ID alert (desimal), bukan kode tampilan")
	}
	assert.Equal(t, []string{"BUAT_OTOMATIS", "BUAT_OTOMATIS", "NAIK_TINGKAT"}, aksi)
}

func TestEvaluasi_FailuresAreCountedAndDoNotStopOtherRules(t *testing.T) {
	rusak := domain.AturanEws{ID: 9, Jenis: domain.JenisLonjakanHarga, AmbangWaspada: 1, AmbangSiaga: 2, AmbangKritis: 3}
	repo := &fakeRepo{
		aturan:  []domain.AturanEws{rusak, aturanLonjakan},
		ukuran:  map[idx.ID][]domain.Pengukuran{aturanLonjakan.ID: {pengukuran("1171", 21)}},
		ukurErr: map[idx.ID]error{rusak.ID: errors.New("db putus")},
		aktif:   map[string]domain.Alert{},
	}
	e, _ := newEngine(repo)

	ringkasan, err := e.Evaluasi(t.Context())

	require.NoError(t, err)
	assert.Equal(t, domain.RingkasanEvaluasi{Aturan: 2, Diukur: 1, Dibuat: 1, Galat: 1}, ringkasan)
}

func TestEvaluasi_WriteErrorIsCounted(t *testing.T) {
	repo := &fakeRepo{
		aturan: []domain.AturanEws{aturanLonjakan}, ukuran: map[idx.ID][]domain.Pengukuran{aturanLonjakan.ID: {pengukuran("1171", 21)}},
		aktif: map[string]domain.Alert{}, terapErr: errors.New("deadlock"),
	}
	e, _ := newEngine(repo)

	ringkasan, err := e.Evaluasi(t.Context())

	require.NoError(t, err)
	assert.Equal(t, 1, ringkasan.Galat)
	assert.Zero(t, ringkasan.Dibuat)
}

func TestRun_DisabledIntervalReturnsImmediately(t *testing.T) {
	e, _ := newEngine(&fakeRepo{})
	done := make(chan struct{})
	go func() { e.Run(t.Context(), 0); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run dengan interval 0 harus langsung kembali")
	}
}

func TestRun_EvaluatesImmediatelyAndStopsOnCancel(t *testing.T) {
	repo := &fakeRepo{aturan: []domain.AturanEws{aturanLonjakan}, ukuran: map[idx.ID][]domain.Pengukuran{aturanLonjakan.ID: {pengukuran("1171", 21)}}, aktif: map[string]domain.Alert{}}
	e, _ := newEngine(repo)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { e.Run(ctx, time.Hour); close(done) }()

	require.Eventually(t, func() bool { repo.mu.Lock(); defer repo.mu.Unlock(); return len(repo.ditulis) == 1 }, 2*time.Second, 10*time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run harus berhenti setelah ctx dibatalkan")
	}
}
